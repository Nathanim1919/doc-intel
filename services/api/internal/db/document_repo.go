package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/doc-intel/api/pkg/models"
)

// ErrNotFound is returned when a queried row does not exist.
var ErrNotFound = errors.New("db: not found")

// ErrInvalidTransition is returned when UpdateStatus is called with a
// fromStatus that doesn't match the row's current status.
var ErrInvalidTransition = errors.New("db: invalid status transition")

// DocumentRepository provides all database operations for documents.
// It accepts any DBTX — pool for reads, pgx.Tx for transactional writes.
type DocumentRepository struct {
	db DBTX
}

// NewDocumentRepository constructs a DocumentRepository backed by the pool.
// Use WithTx to get a transaction-scoped copy.
func NewDocumentRepository(db DBTX) *DocumentRepository {
	return &DocumentRepository{db: db}
}

// WithTx returns a new repository scoped to the given transaction.
// The original repo is unaffected — this is safe to call multiple times.
func (r *DocumentRepository) WithTx(tx pgx.Tx) *DocumentRepository {
	return &DocumentRepository{db: tx}
}

// FindByHashAndOwner looks up an existing document by owner + SHA-256 hash.
// This is the idempotency check on upload — same user uploading the same file
// returns the existing document_id rather than creating a duplicate.
func (r *DocumentRepository) FindByHashAndOwner(ctx context.Context, ownerID uuid.UUID, fileHash string) (*models.Document, error) {
	const q = `
		SELECT id, owner_id, filename, mime_type, size_bytes, storage_key, file_hash, status, created_at, updated_at
		FROM documents
		WHERE owner_id = $1 AND file_hash = $2
		LIMIT 1
	`
	return r.scanDocument(ctx, q, ownerID, fileHash)
}

// GetByID retrieves a document by its primary key.
func (r *DocumentRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Document, error) {
	const q = `
		SELECT id, owner_id, filename, mime_type, size_bytes, storage_key, file_hash, status, created_at, updated_at
		FROM documents
		WHERE id = $1
	`
	return r.scanDocument(ctx, q, id)
}

// CreateDocument inserts a new document record in UPLOADED status.
// The caller should set doc.ID before calling — we use it for the storage key
// before the row is committed, avoiding a round-trip.
func (r *DocumentRepository) CreateDocument(ctx context.Context, doc *models.Document) (*models.Document, error) {
	const q = `
		INSERT INTO documents (id, owner_id, filename, mime_type, size_bytes, storage_key, file_hash, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, owner_id, filename, mime_type, size_bytes, storage_key, file_hash, status, created_at, updated_at
	`
	if doc.ID == uuid.Nil {
		doc.ID = uuid.New()
	}
	return r.scanDocument(ctx, q,
		doc.ID, doc.OwnerID, doc.Filename, doc.MimeType,
		doc.SizeBytes, doc.StorageKey, doc.FileHash, doc.Status,
	)
}

// UpdateStatus performs a guarded atomic status transition.
// The UPDATE only fires if the row currently has fromStatus.
// Returns ErrInvalidTransition if the status doesn't match, ErrNotFound if the row is missing.
//
// The guard (WHERE status = fromStatus) makes this safe under concurrent access:
// two goroutines racing to advance the same document will serialize via Postgres
// row-level locking. The second one will see 0 rows affected after the first commits.
func (r *DocumentRepository) UpdateStatus(ctx context.Context, id uuid.UUID, fromStatus, toStatus string) error {
	const q = `
		UPDATE documents
		SET status = $1, updated_at = NOW()
		WHERE id = $2 AND status = $3
	`
	tag, err := r.db.Exec(ctx, q, toStatus, id, fromStatus)
	if err != nil {
		return fmt.Errorf("db: update status: %w", err)
	}

	switch tag.RowsAffected() {
	case 1:
		return nil
	case 0:
		exists, err := r.exists(ctx, id)
		if err != nil {
			return fmt.Errorf("db: update status check: %w", err)
		}
		if !exists {
			return ErrNotFound
		}
		return ErrInvalidTransition
	default:
		return fmt.Errorf("db: update status: unexpected rows affected: %d", tag.RowsAffected())
	}
}

// ListByOwner returns all documents owned by ownerID, newest first.
// optStatus filters by status when non-empty.
func (r *DocumentRepository) ListByOwner(ctx context.Context, ownerID uuid.UUID, optStatus string) ([]*models.Document, error) {
	const baseQ = `
		SELECT id, owner_id, filename, mime_type, size_bytes, storage_key, file_hash, status, created_at, updated_at
		FROM documents
		WHERE owner_id = $1
	`
	var q string
	var args []any

	if optStatus != "" {
		q = baseQ + " AND status = $2 ORDER BY created_at DESC"
		args = []any{ownerID, optStatus}
	} else {
		q = baseQ + " ORDER BY created_at DESC"
		args = []any{ownerID}
	}

	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("db: list documents: %w", err)
	}
	defer rows.Close()

	var docs []*models.Document
	for rows.Next() {
		doc := &models.Document{}
		if err := scanRow(rows, doc); err != nil {
			return nil, fmt.Errorf("db: list scan: %w", err)
		}
		docs = append(docs, doc)
	}
	return docs, rows.Err()
}

// --- helpers ----------------------------------------------------------------

func (r *DocumentRepository) scanDocument(ctx context.Context, q string, args ...any) (*models.Document, error) {
	row := r.db.QueryRow(ctx, q, args...)
	doc := &models.Document{}
	if err := scanRow(row, doc); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("db: scan document: %w", err)
	}
	return doc, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanRow(r rowScanner, doc *models.Document) error {
	return r.Scan(
		&doc.ID, &doc.OwnerID, &doc.Filename, &doc.MimeType, &doc.SizeBytes,
		&doc.StorageKey, &doc.FileHash, &doc.Status, &doc.CreatedAt, &doc.UpdatedAt,
	)
}

func (r *DocumentRepository) exists(ctx context.Context, id uuid.UUID) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM documents WHERE id = $1)", id).Scan(&exists)
	return exists, err
}

// WithPool returns a repo backed by a pool — for reads outside a transaction.
// Useful when the service holds a pool and needs to do a non-transactional read.
func WithPool(pool *pgxpool.Pool) *DocumentRepository {
	return NewDocumentRepository(pool)
}
