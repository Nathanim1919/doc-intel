"""
Database helpers for the worker.

Intentionally minimal — the worker only writes results and advances
status.  All reads go through the API service.

Uses psycopg[binary] (psycopg v3).  Connection is a plain sync connection
(not a pool) because the worker is single-threaded.
"""

from __future__ import annotations

import uuid
from typing import TYPE_CHECKING

import psycopg
from psycopg.rows import dict_row

if TYPE_CHECKING:
    from config import Config


def connect(cfg: "Config") -> psycopg.Connection:
    """Open a synchronous Postgres connection."""
    return psycopg.connect(cfg.postgres_dsn, row_factory=dict_row)


# ---------------------------------------------------------------------------
# Job / document status reads
# ---------------------------------------------------------------------------

def get_job(conn: psycopg.Connection, job_id: str) -> dict | None:
    """Return the processing_jobs row, or None if not found."""
    with conn.cursor() as cur:
        cur.execute(
            "SELECT id, document_id, status, attempt_count FROM processing_jobs WHERE id = %s",  # noqa: E501
            (job_id,),
        )
        return cur.fetchone()


# ---------------------------------------------------------------------------
# Status advances
# ---------------------------------------------------------------------------

def mark_job_running(conn: psycopg.Connection, job_id: str) -> None:
    """QUEUED | RETRY → RUNNING, bump attempt_count, set started_at on first attempt."""
    with conn.cursor() as cur:
        cur.execute(
            """
            UPDATE processing_jobs
               SET status        = 'RUNNING',
                   attempt_count = attempt_count + 1,
                   started_at    = COALESCE(started_at, NOW()),
                   updated_at    = NOW()
             WHERE id = %s
            """,
            (job_id,),
        )
    conn.commit()


def mark_job_completed(conn: psycopg.Connection, job_id: str) -> None:
    with conn.cursor() as cur:
        cur.execute(
            "UPDATE processing_jobs SET status = 'COMPLETED', updated_at = NOW() WHERE id = %s",
            (job_id,),
        )
    conn.commit()


def mark_job_failed(conn: psycopg.Connection, job_id: str, error: str, permanent: bool) -> None:
    status = "FAILED" if permanent else "RETRY"
    with conn.cursor() as cur:
        cur.execute(
            """
            UPDATE processing_jobs
               SET status     = %s,
                   error      = %s,
                   updated_at = NOW()
             WHERE id = %s
            """,
            (status, error[:2000], job_id),
        )
    conn.commit()


def advance_document_status(
    conn: psycopg.Connection,
    doc_id: str,
    new_status: str,
    error: str | None = None,
) -> None:
    """Advance documents.status; optionally set last_error."""
    with conn.cursor() as cur:
        cur.execute(
            """
            UPDATE documents
               SET status     = %s,
                   last_error = %s,
                   updated_at = NOW()
             WHERE id = %s
            """,
            (new_status, error, doc_id),
        )
    conn.commit()


# ---------------------------------------------------------------------------
# Extraction result writes
# ---------------------------------------------------------------------------

def insert_extraction_run(
    conn: psycopg.Connection,
    *,
    doc_id: str,
    job_id: str,
    model: str,
    model_version: str,
    prompt_version: str,
    status: str,
    latency_ms: int,
    tokens_used: int,
    error: str | None,
) -> str:
    """Insert an extraction_runs row and return its UUID."""
    run_id = str(uuid.uuid4())
    with conn.cursor() as cur:
        cur.execute(
            """
            INSERT INTO extraction_runs
                (id, document_id, job_id, model, model_version, prompt_version,
                 status, latency_ms, tokens_used, error)
            VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s)
            """,
            (
                run_id, doc_id, job_id, model, model_version,
                prompt_version, status, latency_ms, tokens_used, error,
            ),
        )
    conn.commit()
    return run_id


def insert_extraction_fields(
    conn: psycopg.Connection,
    run_id: str,
    fields: list[dict],
) -> None:
    """Bulk-insert extraction_fields rows."""
    if not fields:
        return

    import validator

    rows = []
    for f in fields:
        # If computed_confidence and validation_status are precomputed, use them
        raw_conf = f.get("confidence", 0.0)
        computed_conf = f.get("computed_confidence")
        status = f.get("validation_status")

        if computed_conf is None or status is None:
            computed_conf, status = validator.validate_field(f["field_name"], f.get("value"), raw_conf)

        rows.append(
            (
                str(uuid.uuid4()),
                run_id,
                f["field_name"],
                f.get("value"),
                raw_conf,
                computed_conf,
                status,
                f.get("page_number"),
            )
        )

    with conn.cursor() as cur:
        cur.executemany(
            """
            INSERT INTO extraction_fields
                (id, extraction_run_id, field_name, value,
                 raw_confidence, computed_confidence, validation_status, page_number)
            VALUES (%s, %s, %s, %s, %s, %s, %s, %s)
            """,
            rows,
        )
    conn.commit()


def _validation_status(confidence: float) -> str:
    """Threshold-based validation fallback."""
    if confidence >= 0.85:
        return "PASSED"
    if confidence >= 0.50:
        return "SKIPPED"
    return "FAILED"
