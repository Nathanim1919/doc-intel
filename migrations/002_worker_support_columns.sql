-- =============================================================================
-- Migration 002: worker support columns
--
-- Adds:
--   processing_jobs.updated_at    — so the worker can track last status change
--   documents.last_error          — stores the most recent failure message
--
-- Renames:
--   processing_jobs.attempt → attempt_count  (more descriptive, matches worker code)
-- =============================================================================

-- processing_jobs: rename attempt → attempt_count
ALTER TABLE processing_jobs RENAME COLUMN attempt TO attempt_count;

-- processing_jobs: add updated_at (backfill from created_at)
ALTER TABLE processing_jobs
    ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

UPDATE processing_jobs SET updated_at = created_at;

-- documents: add last_error column (null = no error)
ALTER TABLE documents
    ADD COLUMN last_error TEXT;
