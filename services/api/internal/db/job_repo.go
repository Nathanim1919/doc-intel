package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/doc-intel/api/pkg/models"
)

// JobRepository provides database operations for processing jobs.
// Accepts any DBTX — pool for reads, pgx.Tx for transactional writes.
type JobRepository struct {
	db DBTX
}

// NewJobRepository constructs a JobRepository.
func NewJobRepository(db DBTX) *JobRepository {
	return &JobRepository{db: db}
}

// WithTx returns a new repository scoped to the given transaction.
func (r *JobRepository) WithTx(tx pgx.Tx) *JobRepository {
	return &JobRepository{db: tx}
}

// CreateJob inserts a new processing_job in QUEUED status.
// If job.ID is nil, a new UUID is generated. The caller may pre-set the ID so
// it can be embedded in the Redis payload before the transaction commits.
func (r *JobRepository) CreateJob(ctx context.Context, job *models.ProcessingJob) (*models.ProcessingJob, error) {
	const q = `
		INSERT INTO processing_jobs (id, document_id, status, attempt)
		VALUES ($1, $2, $3, 0)
		RETURNING id, document_id, status, attempt, started_at, completed_at, error, created_at
	`
	if job.ID == uuid.Nil {
		job.ID = uuid.New()
	}

	row := r.db.QueryRow(ctx, q, job.ID, job.DocumentID, job.Status)
	result, err := scanJob(row)
	if err != nil {
		return nil, fmt.Errorf("db: create job: %w", err)
	}
	return result, nil
}

// GetByDocumentID retrieves the most recent job for a document.
func (r *JobRepository) GetByDocumentID(ctx context.Context, documentID uuid.UUID) (*models.ProcessingJob, error) {
	const q = `
		SELECT id, document_id, status, attempt, started_at, completed_at, error, created_at
		FROM processing_jobs
		WHERE document_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`
	row := r.db.QueryRow(ctx, q, documentID)
	job, err := scanJob(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("db: get job by document: %w", err)
	}
	return job, nil
}

// GetByID retrieves a job by its primary key.
func (r *JobRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.ProcessingJob, error) {
	const q = `
		SELECT id, document_id, status, attempt, started_at, completed_at, error, created_at
		FROM processing_jobs
		WHERE id = $1
	`
	row := r.db.QueryRow(ctx, q, id)
	job, err := scanJob(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("db: get job: %w", err)
	}
	return job, nil
}

func scanJob(row pgx.Row) (*models.ProcessingJob, error) {
	job := &models.ProcessingJob{}
	err := row.Scan(
		&job.ID, &job.DocumentID, &job.Status, &job.Attempt,
		&job.StartedAt, &job.CompletedAt, &job.Error, &job.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return job, nil
}
