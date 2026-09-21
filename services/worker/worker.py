"""
The main worker loop.

Lifecycle per job:
  1. BRPOP from Redis → parse job payload
  2. Validate against DB (ghost message guard)
  3. Mark job RUNNING
  4. Fetch file from MinIO
  5. Run Gemini extraction
  6. Write extraction_runs + extraction_fields
  7. Advance document + job status (COMPLETED | REVIEW_REQUIRED | RETRY | FAILED)

Retry policy:
  - On transient failure: status → RETRY, re-enqueue to the BACK of the queue
  - After max_attempts: status → FAILED, document → FAILED_PERMANENTLY
  - Ghost messages (job not found in DB): discarded silently

REVIEW_REQUIRED trigger:
  - Any field with validation_status = FAILED → document needs human review
"""

from __future__ import annotations

import io
import json
import logging
import signal
import sys
import time
from typing import Any

import psycopg
import redis as redis_lib
from minio import Minio
from minio.error import S3Error

from config import Config
import db as store
from extractor import Extractor
import validator

logger = logging.getLogger(__name__)

# ---------------------------------------------------------------------------
# Worker
# ---------------------------------------------------------------------------

class Worker:
    def __init__(self, cfg: Config) -> None:
        self._cfg = cfg
        self._running = True

        # Redis client
        self._redis = redis_lib.from_url(cfg.redis_url, decode_responses=True)

        # Postgres connection (reconnects on error)
        self._conn: psycopg.Connection | None = None

        # MinIO client
        self._minio = Minio(
            cfg.minio_endpoint,
            access_key=cfg.minio_access_key,
            secret_key=cfg.minio_secret_key,
            secure=cfg.minio_use_ssl,
        )

        # Gemini extractor (no fixed model — uses CANDIDATE_MODELS fallback list)
        self._extractor = Extractor(api_key=cfg.gemini_api_key)

    # -----------------------------------------------------------------------
    # Main loop
    # -----------------------------------------------------------------------

    def run(self) -> None:
        logger.info("Worker started — listening on %s", self._cfg.queue_key)

        # Graceful shutdown on SIGTERM / SIGINT
        signal.signal(signal.SIGTERM, self._handle_signal)
        signal.signal(signal.SIGINT, self._handle_signal)

        while self._running:
            try:
                self._tick()
            except Exception:  # noqa: BLE001
                logger.exception("Unexpected error in main loop — sleeping 5s before retry")
                time.sleep(5)

        logger.info("Worker shutdown complete")

    def _handle_signal(self, signum: int, _frame: Any) -> None:
        logger.info("Received signal %d — draining current job then stopping", signum)
        self._running = False

    # -----------------------------------------------------------------------
    # Single tick — blocks up to brpop_timeout seconds on Redis
    # -----------------------------------------------------------------------

    def _tick(self) -> None:
        try:
            result = self._redis.brpop(self._cfg.queue_key, timeout=self._cfg.brpop_timeout)
        except (redis_lib.exceptions.TimeoutError, TimeoutError):
            return  # timeout waiting for work — normal idle behavior

        if result is None:
            return  # timeout — loop again

        _, raw = result
        try:
            payload = json.loads(raw)
        except json.JSONDecodeError:
            logger.error("Received malformed JSON from queue — discarding: %s", raw[:200])
            return

        self._process(payload)

    # -----------------------------------------------------------------------
    # Process a single job
    # -----------------------------------------------------------------------

    def _process(self, payload: dict) -> None:
        job_id = payload.get("job_id")
        doc_id = payload.get("document_id")
        storage_key = payload.get("storage_key")
        mime_type = payload.get("mime_type")

        if not all([job_id, doc_id, storage_key, mime_type]):
            logger.error("Payload missing required fields — discarding: %s", payload)
            return

        log = logger.getChild(f"job={job_id[:8]}")
        log.info("Processing doc=%s key=%s", doc_id[:8], storage_key)

        conn = self._db_conn()

        # 1. Ghost-message guard — job must exist in DB
        job = store.get_job(conn, job_id)
        if job is None:
            log.warning("Job not found in DB — likely a ghost message, discarding")
            return

        if job["status"] not in ("QUEUED", "RETRY"):
            log.info("Job status=%s — already processed or in flight, skipping", job["status"])
            return

        attempt = job.get("attempt_count", 0) + 1  # after mark_running it will be this value
        permanent_failure = attempt >= self._cfg.max_attempts

        # 2. Mark RUNNING
        store.mark_job_running(conn, job_id)
        store.advance_document_status(conn, doc_id, "PROCESSING")

        # 3. Fetch file from MinIO
        try:
            file_bytes = self._fetch_file(storage_key)
        except Exception as exc:  # noqa: BLE001
            log.error("MinIO fetch failed: %s", exc)
            self._fail(conn, job_id, doc_id, str(exc), permanent_failure, payload)
            return

        # 4. Run extraction
        run_id: str | None = None
        try:
            result, model_used, latency_ms, tokens_used = self._extractor.extract(file_bytes, mime_type)
            log.info("Extraction done — model=%s fields=%d latency=%dms tokens=%d",
                     model_used, len(result.fields), latency_ms, tokens_used)

            # 5. Write extraction_run
            run_id = store.insert_extraction_run(
                conn,
                doc_id=doc_id,
                job_id=job_id,
                model=model_used,
                model_version="",
                prompt_version=self._cfg.prompt_version,
                status="COMPLETED",
                latency_ms=latency_ms,
                tokens_used=tokens_used,
                error=None,
            )

            # 6. Apply regional validation rules and write extraction_fields
            validated_fields = []
            fields_dict: dict[str, str | None] = {}
            for f in result.fields:
                computed_conf, status = validator.validate_field(f.field_name, f.value, f.confidence)
                validated_fields.append({
                    "field_name": f.field_name,
                    "value": f.value,
                    "confidence": f.confidence,
                    "computed_confidence": computed_conf,
                    "validation_status": status,
                    "page_number": f.page_number,
                })
                fields_dict[f.field_name] = f.value

            # Arithmetic checks on financial amounts
            math_warnings = validator.validate_document_math(fields_dict)
            if math_warnings:
                for w in math_warnings:
                    log.warning("Regional validation warning: %s", w)

            store.insert_extraction_fields(
                conn,
                run_id,
                validated_fields,
            )

            # 7. Decide final document status
            has_failed_fields = any(f["validation_status"] == "FAILED" for f in validated_fields)
            if math_warnings:
                # Discrepancy in totals requires human review
                has_failed_fields = True

            final_doc_status = "REVIEW_REQUIRED" if has_failed_fields else "COMPLETED"

            store.mark_job_completed(conn, job_id)
            store.advance_document_status(conn, doc_id, final_doc_status)
            log.info("Job COMPLETED — doc_status=%s (math_warnings=%d)", final_doc_status, len(math_warnings))

        except ValueError as exc:
            # Malformed model output — record the run as FAILED
            log.error("Extraction parse error: %s", exc)
            if run_id is None:
                run_id = store.insert_extraction_run(
                    conn,
                    doc_id=doc_id,
                    job_id=job_id,
                    model=self._cfg.gemini_model,
                    model_version="",
                    prompt_version=self._cfg.prompt_version,
                    status="FAILED",
                    latency_ms=0,
                    tokens_used=0,
                    error=str(exc)[:2000],
                )
            self._fail(conn, job_id, doc_id, str(exc), permanent_failure, payload)

        except Exception as exc:  # noqa: BLE001
            log.error("Extraction API error (attempt %d/%d): %s",
                      attempt, self._cfg.max_attempts, exc)
            self._fail(conn, job_id, doc_id, str(exc), permanent_failure, payload)

    # -----------------------------------------------------------------------
    # Failure handling
    # -----------------------------------------------------------------------

    def _fail(
        self,
        conn: psycopg.Connection,
        job_id: str,
        doc_id: str,
        error: str,
        permanent: bool,
        payload: dict,
    ) -> None:
        if permanent:
            logger.error("Job exceeded max attempts — marking FAILED permanently: %s", job_id)
            store.mark_job_failed(conn, job_id, error, permanent=True)
            store.advance_document_status(conn, doc_id, "FAILED_PERMANENTLY", error)
        else:
            logger.warning("Job RETRY — re-enqueuing: %s", job_id)
            store.mark_job_failed(conn, job_id, error, permanent=False)
            store.advance_document_status(conn, doc_id, "QUEUED")
            # Re-enqueue to the BACK of the queue (LPUSH = FIFO when consuming with BRPOP)
            self._redis.lpush(self._cfg.queue_key, json.dumps(payload))

    # -----------------------------------------------------------------------
    # Helpers
    # -----------------------------------------------------------------------

    def _fetch_file(self, storage_key: str) -> bytes:
        """Download file bytes from MinIO."""
        response = self._minio.get_object(self._cfg.minio_bucket, storage_key)
        try:
            return response.read()
        finally:
            response.close()
            response.release_conn()

    def _db_conn(self) -> psycopg.Connection:
        """Return the DB connection, reconnecting if it was closed."""
        if self._conn is None or self._conn.closed:
            logger.info("(Re)connecting to Postgres")
            self._conn = store.connect(self._cfg)
        return self._conn
