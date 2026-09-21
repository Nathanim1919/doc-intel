-- =============================================================================
-- Migration: 001_initial_schema
-- Description: Core schema for doc-intel MVP
--
-- Tables:
--   organizations    → tenancy root, every resource is scoped to one
--   users            → auth principal, belongs to one org
--   documents        → uploaded file metadata + lifecycle status machine
--   processing_jobs  → async job tracking (one per document upload)
--   extraction_runs  → per-model-invocation metadata (model, prompt, latency)
--   extraction_fields → individual field values extracted from a single run
--
-- Status machines:
--   document.status:
--     UPLOADED → QUEUED → PROCESSING → COMPLETED
--                                    → FAILED
--                                    → REVIEW_REQUIRED
--     QUEUED   → PROCESSING → RETRY  → PROCESSING (up to max attempts)
--                                    → FAILED_PERMANENTLY
--
--   processing_job.status:
--     QUEUED → RUNNING → COMPLETED | FAILED | RETRY
--
--   extraction_run.status:
--     RUNNING → COMPLETED | FAILED
--
--   extraction_field.validation_status:
--     PASSED | FAILED | SKIPPED
-- =============================================================================

-- Enable pgcrypto for gen_random_uuid() — required for UUID PKs
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- =============================================================================
-- TENANCY ROOT
-- An organization is the top-level ownership boundary.
-- Every document, user, and result is scoped to one org.
-- In the MVP this is simple, but this is the hook for multi-tenancy later.
-- =============================================================================
CREATE TABLE organizations (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    name       TEXT        NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- =============================================================================
-- AUTH PRINCIPAL
-- Users belong to exactly one organization.
-- api_key_hash stores a bcrypt/SHA-256 hash of the raw API key — never the key
-- itself. The raw key is shown once on creation and never stored.
-- email is globally unique so it can be used as a lookup key for login flows.
-- =============================================================================
CREATE TABLE users (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID        NOT NULL REFERENCES organizations(id) ON DELETE RESTRICT,
    email           TEXT        NOT NULL UNIQUE,
    api_key_hash    TEXT        NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_users_org ON users(organization_id);

-- =============================================================================
-- DOCUMENT METADATA + LIFECYCLE STATE MACHINE
--
-- storage_key: the path in MinIO, e.g. documents/{id}/original.pdf
--   We store this here rather than computing it from the ID because the format
--   might change, and we want a stable reference we can migrate.
--
-- file_hash: SHA-256 of the raw bytes. Used for idempotency — if the same user
--   uploads the same file twice, we return the existing document_id rather than
--   creating a duplicate. Also useful for deduplication across users later.
--
-- status: TEXT instead of ENUM intentionally — adding new states doesn't
--   require a schema migration. Enforce valid values at the application layer.
--
-- mime_type: stored as-is from Content-Type header after validation. We
--   allowlist on ingestion (PDF, PNG, JPEG, TIFF only for v1).
--
-- size_bytes: stored at upload time so we can enforce per-org quotas later
--   without hitting object storage.
--
-- updated_at: maintained by a trigger (see bottom of file). Every status
--   transition updates this column automatically.
-- =============================================================================
CREATE TABLE documents (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id    UUID        NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    filename    TEXT        NOT NULL,
    mime_type   TEXT        NOT NULL,
    size_bytes  BIGINT      NOT NULL,
    storage_key TEXT        NOT NULL UNIQUE,
    file_hash   TEXT        NOT NULL,
    status      TEXT        NOT NULL DEFAULT 'UPLOADED',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Queries: "list my documents" → owner_id
-- Queries: "show me all QUEUED docs for the worker to pick up" → status
-- Queries: "is this exact file already uploaded?" → owner_id + file_hash
CREATE INDEX idx_documents_owner     ON documents(owner_id);
CREATE INDEX idx_documents_status    ON documents(status);
CREATE INDEX idx_documents_file_hash ON documents(owner_id, file_hash);

-- =============================================================================
-- ASYNC JOB TRACKING
--
-- One processing_job is created per document upload. If the job fails and is
-- retried, the attempt counter increments on the SAME row — we don't create a
-- new row. This gives us a clean history of how many times we tried.
--
-- started_at / completed_at: nullable because they're set at runtime.
--   started_at NULL means the job hasn't been picked up yet.
--   completed_at NULL means it hasn't finished (or failed).
--
-- error: last error message if status = FAILED | FAILED_PERMANENTLY.
--   Overwritten on each attempt so only the most recent error is stored.
--   Detailed per-attempt error history lives in extraction_runs.
--
-- We intentionally separate this from documents because the job is an
-- execution concern, not a data concern. A document can exist without a job
-- (e.g., manually inserted for testing). And in the future, you might
-- re-process a document → a second job on the same document_id.
-- =============================================================================
CREATE TABLE processing_jobs (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    document_id  UUID        NOT NULL REFERENCES documents(id) ON DELETE RESTRICT,
    status       TEXT        NOT NULL DEFAULT 'QUEUED',
    attempt      INT         NOT NULL DEFAULT 0,
    started_at   TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    error        TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Queries: "get the job for this document" → document_id
-- Queries: "show all QUEUED jobs" → status (worker polling fallback)
CREATE INDEX idx_jobs_document ON processing_jobs(document_id);
CREATE INDEX idx_jobs_status   ON processing_jobs(status);

-- =============================================================================
-- EXTRACTION RUN METADATA
--
-- One extraction_run per model invocation per job. If we call GPT-4 once and
-- Gemini once for the same document, that's two rows. This is the audit trail
-- for what model produced what output.
--
-- model + model_version: e.g. "gemini", "2.0-flash". Stored separately because
--   you'll want to filter by model family when evaluating results later.
--
-- prompt_version: e.g. "v1.2". When you update your extraction prompt, bump
--   this. Now you can query "which results were produced with prompt v1.0?"
--   and compare them against v1.2. This is what makes your system evaluatable.
--
-- latency_ms: time from API call start to response complete, measured by the
--   worker. Not the full job time — just the model invocation cost.
--
-- tokens_used: if the model API reports it (most do). Crucial for cost tracking.
--   NULL if the model doesn't report it.
-- =============================================================================
CREATE TABLE extraction_runs (
    id             UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    document_id    UUID        NOT NULL REFERENCES documents(id) ON DELETE RESTRICT,
    job_id         UUID        NOT NULL REFERENCES processing_jobs(id) ON DELETE RESTRICT,
    model          TEXT        NOT NULL,
    model_version  TEXT        NOT NULL,
    prompt_version TEXT        NOT NULL,
    status         TEXT        NOT NULL,
    latency_ms     INT,
    tokens_used    INT,
    error          TEXT,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Queries: "all runs for this document" → document_id
-- Queries: "all runs for this job" → job_id
CREATE INDEX idx_runs_document ON extraction_runs(document_id);
CREATE INDEX idx_runs_job      ON extraction_runs(job_id);

-- =============================================================================
-- EXTRACTION FIELDS — THE ACTUAL OUTPUT
--
-- One row per field extracted per run. If your prompt asks for 8 fields,
-- you get 8 rows in extraction_fields for that run.
--
-- field_name: the key from your schema, e.g. "invoice_number", "total_amount"
--
-- value: TEXT for everything. This is intentional — don't store typed values
--   here. Let the application layer cast/validate. The DB doesn't know that
--   "total_amount" should be a NUMERIC. Storing TEXT keeps the schema stable
--   when your extraction schema evolves.
--
-- raw_confidence: model's own confidence score, if it reports one (0.0–1.0).
--   NULL if the model doesn't provide it.
--
-- computed_confidence: your own confidence score, computed by your validation
--   pipeline (e.g. regex match, cross-field consistency checks). This is what
--   you actually use for routing to REVIEW_REQUIRED.
--
-- validation_status:
--   PASSED  → field passed all validation rules
--   FAILED  → field failed validation (wrong format, out of range, etc.)
--   SKIPPED → field not applicable for this document type
--
-- page_number: which page the value was found on. Nullable because not all
--   models report this. Useful for UX (highlight the source).
-- =============================================================================
CREATE TABLE extraction_fields (
    id                  UUID   PRIMARY KEY DEFAULT gen_random_uuid(),
    extraction_run_id   UUID   NOT NULL REFERENCES extraction_runs(id) ON DELETE RESTRICT,
    field_name          TEXT   NOT NULL,
    value               TEXT,
    raw_confidence      FLOAT,
    computed_confidence FLOAT,
    validation_status   TEXT   NOT NULL,
    page_number         INT
);

-- Queries: "give me all fields for this run" → extraction_run_id
-- Queries: "find all FAILED fields across runs" → validation_status (analytics)
CREATE INDEX idx_fields_run    ON extraction_fields(extraction_run_id);
CREATE INDEX idx_fields_status ON extraction_fields(validation_status);

-- =============================================================================
-- TRIGGER: auto-update documents.updated_at on every UPDATE
-- Without this, updated_at would only change if the application explicitly
-- sets it — easy to forget. This makes it automatic and reliable.
-- =============================================================================
CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_documents_updated_at
    BEFORE UPDATE ON documents
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();
