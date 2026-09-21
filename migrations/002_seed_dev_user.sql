-- =============================================================================
-- Migration: 002_seed_dev_user
-- Purpose: Insert a test organization + user for local development.
--
-- API Key: dev-api-key-do-not-use-in-production
-- Hash:    SHA-256 of the above, stored in api_key_hash
--
-- Usage:
--   Authorization: Bearer dev-api-key-do-not-use-in-production
--
-- !! NEVER run this migration against a production database !!
-- =============================================================================

DO $$
BEGIN
    -- Only seed if no organizations exist (idempotent)
    IF NOT EXISTS (SELECT 1 FROM organizations LIMIT 1) THEN

        INSERT INTO organizations (id, name)
        VALUES ('00000000-0000-0000-0000-000000000001', 'Dev Org');

        INSERT INTO users (id, organization_id, email, api_key_hash)
        VALUES (
            '00000000-0000-0000-0000-000000000002',
            '00000000-0000-0000-0000-000000000001',
            'dev@doc-intel.local',
            -- SHA-256 of 'dev-api-key-do-not-use-in-production'
            encode(digest('dev-api-key-do-not-use-in-production', 'sha256'), 'hex')
        );

    END IF;
END $$;
