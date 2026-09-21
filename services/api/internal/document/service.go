package document

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/doc-intel/api/internal/db"
	"github.com/doc-intel/api/internal/queue"
	"github.com/doc-intel/api/internal/storage"
	"github.com/doc-intel/api/pkg/models"
)

// ErrDuplicate is returned by Upload when the same file already exists for this owner.
// The caller should return the existing document rather than an error to the client.
var ErrDuplicate = errors.New("document: duplicate upload")

// Service orchestrates the full document lifecycle.
// It owns transactions — repos are given a tx handle when atomicity is needed.
//
// Dependency layout:
//
//	Handler → Service → (DB repos, ObjectStorage, Queue)
//
// The handler never touches the pool, repos, or Redis directly.
// This keeps the handler thin and testable (mock the Service interface).
type Service struct {
	pool    *pgxpool.Pool
	docRepo *db.DocumentRepository
	jobRepo *db.JobRepository
	store   storage.ObjectStorage
	queue   queue.Producer
}

// New constructs a Service. All dependencies are required.
func New(
	pool *pgxpool.Pool,
	docRepo *db.DocumentRepository,
	jobRepo *db.JobRepository,
	store storage.ObjectStorage,
	q queue.Producer,
) *Service {
	return &Service{
		pool:    pool,
		docRepo: docRepo,
		jobRepo: jobRepo,
		store:   store,
		queue:   q,
	}
}

// UploadInput carries the validated data from the HTTP handler.
// By the time this reaches the service, MIME type has been validated,
// file size is within limits, and filename has been sanitized.
type UploadInput struct {
	OwnerID     uuid.UUID
	Filename    string
	MimeType    string
	SizeBytes   int64
	Content     io.Reader
}

// UploadResult is returned on successful upload.
type UploadResult struct {
	Document *models.Document
	Job      *models.ProcessingJob
	IsDupe   bool // true if this is a duplicate — existing document returned
}

// Upload is the core upload flow. Order of operations:
//
//  1. Hash the file content (SHA-256)
//  2. Check for existing document with same owner + hash (idempotency)
//  3. Upload file to object storage
//  4. BEGIN transaction:
//     a. INSERT document (status=UPLOADED)
//     b. INSERT processing_job (status=QUEUED)
//     c. UPDATE document status UPLOADED→QUEUED
//  5. COMMIT
//  6. LPUSH job payload to Redis
//
// Object storage upload happens BEFORE the transaction. This is intentional:
// if the DB transaction fails, we have an orphaned object in storage — which
// is fine. It will be unreferenced and can be cleaned up by a background job.
// The alternative (upload inside the transaction) risks holding the transaction
// open for the duration of a network upload, which is worse.
//
// Redis enqueue happens AFTER commit. If it fails, the document is in QUEUED
// status in the DB but not in Redis. A background reconciler (Phase 2) will
// pick up QUEUED documents with no active job and re-enqueue them.
func (s *Service) Upload(ctx context.Context, input UploadInput) (*UploadResult, error) {
	// 1. Hash the content
	hash, content, err := hashContent(input.Content)
	if err != nil {
		return nil, fmt.Errorf("document: hash: %w", err)
	}

	// 2. Idempotency check — same owner + same file hash
	existing, err := s.docRepo.FindByHashAndOwner(ctx, input.OwnerID, hash)
	if err != nil && !errors.Is(err, db.ErrNotFound) {
		return nil, fmt.Errorf("document: idempotency check: %w", err)
	}
	if existing != nil {
		job, _ := s.jobRepo.GetByDocumentID(ctx, existing.ID)
		return &UploadResult{Document: existing, Job: job, IsDupe: true}, nil
	}

	// 3. Compute storage key and upload to object storage
	docID := uuid.New()
	storageKey := storage.StorageKey(docID, input.Filename)

	if err := s.store.Upload(ctx, storageKey, content, input.SizeBytes, input.MimeType); err != nil {
		return nil, fmt.Errorf("document: store upload: %w", err)
	}

	// 4. Atomic DB transaction: create document + job + advance status
	doc := &models.Document{
		ID:         docID,
		OwnerID:    input.OwnerID,
		Filename:   input.Filename,
		MimeType:   input.MimeType,
		SizeBytes:  input.SizeBytes,
		StorageKey: storageKey,
		FileHash:   hash,
		Status:     models.StatusUploaded,
	}

	jobID := uuid.New() // pre-generate so we can embed in the queue payload

	doc, job, err := s.createDocumentAndJob(ctx, doc, jobID)
	if err != nil {
		return nil, fmt.Errorf("document: create: %w", err)
	}

	// 5. Enqueue — after commit. Safe to fail; reconciler will retry.
	payload := models.QueuePayload{
		JobID:      job.ID,
		DocumentID: doc.ID,
		StorageKey: doc.StorageKey,
		MimeType:   doc.MimeType,
	}
	if err := s.queue.Enqueue(ctx, payload); err != nil {
		// Log but don't fail the request. The document is committed.
		// The reconciler (Phase 2) will detect QUEUED documents with no
		// active Redis entry and re-enqueue them.
		// TODO: replace with structured logger
		fmt.Printf("warn: enqueue failed for job %s: %v\n", job.ID, err)
	}

	return &UploadResult{Document: doc, Job: job, IsDupe: false}, nil
}

// createDocumentAndJob runs the three DB operations inside a single transaction.
// This is the transactional unit you identified: doc insert + job insert + status advance.
// If any step fails, the entire transaction rolls back — no orphaned rows.
func (s *Service) createDocumentAndJob(ctx context.Context, doc *models.Document, jobID uuid.UUID) (*models.Document, *models.ProcessingJob, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("begin tx: %w", err)
	}
	// Rollback is a no-op if Commit already succeeded.
	defer tx.Rollback(ctx) //nolint:errcheck

	docRepo := s.docRepo.WithTx(tx)
	jobRepo := s.jobRepo.WithTx(tx)

	// a. Insert document at UPLOADED
	created, err := docRepo.CreateDocument(ctx, doc)
	if err != nil {
		return nil, nil, fmt.Errorf("insert document: %w", err)
	}

	// b. Insert job at QUEUED
	job, err := jobRepo.CreateJob(ctx, &models.ProcessingJob{
		ID:         jobID,
		DocumentID: created.ID,
		Status:     models.JobStatusQueued,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("insert job: %w", err)
	}

	// c. Advance document status UPLOADED → QUEUED
	if err := docRepo.UpdateStatus(ctx, created.ID, models.StatusUploaded, models.StatusQueued); err != nil {
		return nil, nil, fmt.Errorf("advance status: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, nil, fmt.Errorf("commit: %w", err)
	}

	created.Status = models.StatusQueued
	return created, job, nil
}

// GetByID retrieves a document. Returns db.ErrNotFound if missing.
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*models.Document, error) {
	return s.docRepo.GetByID(ctx, id)
}

// ListByOwner returns documents for ownerID. optStatus filters by status.
func (s *Service) ListByOwner(ctx context.Context, ownerID uuid.UUID, optStatus string) ([]*models.Document, error) {
	return s.docRepo.ListByOwner(ctx, ownerID, optStatus)
}

// GetJobByDocument returns the latest job for a document.
func (s *Service) GetJobByDocument(ctx context.Context, docID uuid.UUID) (*models.ProcessingJob, error) {
	return s.jobRepo.GetByDocumentID(ctx, docID)
}

// --- helpers ----------------------------------------------------------------

// hashContent reads all of r into a SHA-256 hash and returns both the hex
// digest and a new reader over the same bytes — because io.Reader is consumed
// once, we must buffer it before the storage upload.
func hashContent(r io.Reader) (hexHash string, reReader io.Reader, err error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return "", nil, err
	}
	sum := sha256.Sum256(data)
	return fmt.Sprintf("%x", sum), io.NopCloser(
		// Return a new bytes reader so the caller can stream to MinIO
		newBytesReader(data),
	), nil
}

// bytesReader wraps a byte slice to implement io.Reader.
// We avoid importing bytes here — simple enough to inline.
type bytesReader struct {
	data []byte
	pos  int
}

func newBytesReader(data []byte) *bytesReader {
	return &bytesReader{data: data}
}

func (b *bytesReader) Read(p []byte) (n int, err error) {
	if b.pos >= len(b.data) {
		return 0, io.EOF
	}
	n = copy(p, b.data[b.pos:])
	b.pos += n
	return n, nil
}
