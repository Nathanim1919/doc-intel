package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/doc-intel/api/pkg/models"
)

// ExtractionRepository provides read access to extraction_runs and extraction_fields.
type ExtractionRepository struct {
	db DBTX
}

// NewExtractionRepository constructs an ExtractionRepository.
func NewExtractionRepository(db DBTX) *ExtractionRepository {
	return &ExtractionRepository{db: db}
}

// WithTx returns a new repository scoped to the given transaction.
func (r *ExtractionRepository) WithTx(tx pgx.Tx) *ExtractionRepository {
	return &ExtractionRepository{db: tx}
}

// GetLatestRunByDocument returns the most recent extraction_run for a document.
// Returns ErrNotFound if no run exists yet (document not yet processed).
func (r *ExtractionRepository) GetLatestRunByDocument(ctx context.Context, docID uuid.UUID) (*models.ExtractionRun, error) {
	const q = `
		SELECT id, document_id, job_id, model, model_version, prompt_version,
		       status, latency_ms, tokens_used, error, created_at
		  FROM extraction_runs
		 WHERE document_id = $1
		 ORDER BY created_at DESC
		 LIMIT 1
	`
	row := r.db.QueryRow(ctx, q, docID)
	run, err := scanRun(row)
	if err != nil {
		return nil, fmt.Errorf("db: get latest run: %w", err)
	}
	return run, nil
}

// GetFieldsByRun returns all extraction_fields for a given extraction_run,
// ordered by page_number ASC NULLS LAST, then field_name.
func (r *ExtractionRepository) GetFieldsByRun(ctx context.Context, runID uuid.UUID) ([]*models.ExtractionField, error) {
	const q = `
		SELECT id, extraction_run_id, field_name, value,
		       raw_confidence, computed_confidence, validation_status, page_number
		  FROM extraction_fields
		 WHERE extraction_run_id = $1
		 ORDER BY page_number ASC NULLS LAST, field_name ASC
	`
	rows, err := r.db.Query(ctx, q, runID)
	if err != nil {
		return nil, fmt.Errorf("db: get fields by run: %w", err)
	}
	defer rows.Close()

	var fields []*models.ExtractionField
	for rows.Next() {
		f := &models.ExtractionField{}
		if err := rows.Scan(
			&f.ID, &f.ExtractionRunID, &f.FieldName, &f.Value,
			&f.RawConfidence, &f.ComputedConfidence, &f.ValidationStatus, &f.PageNumber,
		); err != nil {
			return nil, fmt.Errorf("db: scan field: %w", err)
		}
		fields = append(fields, f)
	}
	return fields, rows.Err()
}

// UpdateField modifies the extracted value and validation_status of a specific field.
func (r *ExtractionRepository) UpdateField(ctx context.Context, fieldID uuid.UUID, value string, status string) (*models.ExtractionField, error) {
	const q = `
		UPDATE extraction_fields
		   SET value = $2,
		       validation_status = $3
		 WHERE id = $1
		RETURNING id, extraction_run_id, field_name, value,
		          raw_confidence, computed_confidence, validation_status, page_number
	`
	row := r.db.QueryRow(ctx, q, fieldID, value, status)
	f := &models.ExtractionField{}
	if err := row.Scan(
		&f.ID, &f.ExtractionRunID, &f.FieldName, &f.Value,
		&f.RawConfidence, &f.ComputedConfidence, &f.ValidationStatus, &f.PageNumber,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("db: update field: %w", err)
	}
	return f, nil
}

// CountUnapprovedFieldsByDocument counts how many fields in the document's latest run are in FAILED status.
func (r *ExtractionRepository) CountUnapprovedFieldsByDocument(ctx context.Context, docID uuid.UUID) (int, error) {
	const q = `
		SELECT COUNT(*)
		  FROM extraction_fields f
		  JOIN extraction_runs r ON f.extraction_run_id = r.id
		 WHERE r.document_id = $1
		   AND f.validation_status = 'FAILED'
	`
	var count int
	if err := r.db.QueryRow(ctx, q, docID).Scan(&count); err != nil {
		return 0, fmt.Errorf("db: count unapproved fields: %w", err)
	}
	return count, nil
}

// --- scan helpers -----------------------------------------------------------

func scanRun(row pgx.Row) (*models.ExtractionRun, error) {
	r := &models.ExtractionRun{}
	err := row.Scan(
		&r.ID, &r.DocumentID, &r.JobID, &r.Model, &r.ModelVersion,
		&r.PromptVersion, &r.Status, &r.LatencyMs, &r.TokensUsed,
		&r.Error, &r.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return r, nil
}
