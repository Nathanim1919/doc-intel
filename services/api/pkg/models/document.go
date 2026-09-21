package models


import (
	"time"

	"github.com/google/uuid"
)


// Document lifesycle statuses
const (
	StatusUploaded         = "UPLOADED"
	StatusQueued           = "QUEUED"
	StatusProcessing       = "PROCESSING"
	StatusCompleted        = "COMPLETED"
	StatusFailed           = "FAILED"
	StatusReviewRequired   = "REVIEW_REQUIRED"
	StatusFailedPermanent  = "FAILED_PERMANENTLY"
)

// Job statuses
const (
	JobStatusQueued    = "QUEUED"
	JobStatusRunning   = "RUNNING"
	JobStatusCompleted = "COMPLETED"
	JobStatusFailed    = "FAILED"
	JobStatusRetry     = "RETRY"
)

type Document struct {
	ID         uuid.UUID `json:"id"`
	OwnerID    uuid.UUID `json:"owner_id"`
	Filename   string    `json:"filename"`
	MimeType   string    `json:"mime_type"`
	SizeBytes  int64     `json:"size_bytes"`
	StorageKey string    `json:"storage_key"`
	FileHash   string    `json:"file_hash"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type ProcessingJob struct {
	ID          uuid.UUID  `json:"id"`
	DocumentID  uuid.UUID  `json:"document_id"`
	Status      string     `json:"status"`
	Attempt     int        `json:"attempt"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	Error       *string    `json:"error,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

type ExtractionField struct {
	ID                 uuid.UUID `json:"id"`
	ExtractionRunID    uuid.UUID `json:"extraction_run_id"`
	FieldName          string    `json:"field_name"`
	Value              *string   `json:"value,omitempty"`
	RawConfidence      *float64  `json:"raw_confidence,omitempty"`
	ComputedConfidence *float64  `json:"computed_confidence,omitempty"`
	ValidationStatus   string    `json:"validation_status"`
	PageNumber         *int      `json:"page_number,omitempty"`
}

// QueuePayload is pushed to Redis for workers to consume
type QueuePayload struct {
	JobID      uuid.UUID `json:"job_id"`
	DocumentID uuid.UUID `json:"document_id"`
	StorageKey string    `json:"storage_key"`
	MimeType   string    `json:"mime_type"`
}