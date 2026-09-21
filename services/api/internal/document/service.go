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
	pool           *pgxpool.Pool
	docRepo        *db.DocumentRepository
	jobRepo        *db.JobRepository
	extractionRepo *db.ExtractionRepository
	store          storage.ObjectStorage
	queue          queue.Producer
}

// New constructs a Service. All dependencies are required.
func New(
	pool *pgxpool.Pool,
	docRepo *db.DocumentRepository,
	jobRepo *db.JobRepository,
	extractionRepo *db.ExtractionRepository,
	store storage.ObjectStorage,
	q queue.Producer,
) *Service {
	return &Service{
		pool:           pool,
		docRepo:        docRepo,
		jobRepo:        jobRepo,
		extractionRepo: extractionRepo,
		store:          store,
		queue:          q,
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

	// 4. Pre-generate IDs so we can enqueue before the DB transaction.
	jobID := uuid.New()

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

	// 5. Enqueue BEFORE committing to DB.
	//
	// Why: if we enqueue after commit and Redis fails, the document is stuck
	// in QUEUED forever (no worker picks it up). Flipping the order means:
	//   - Redis fails  → we return 500 before touching the DB. Client retries
	//                    clean; no stale state exists anywhere.
	//   - DB fails     → ghost message sits in Redis. Worker pops it, queries
	//                    DB by job_id, finds nothing, discards it. Zero harm.
	//   - Both succeed → normal path.
	//
	// This works because the worker ALWAYS validates against the DB before
	// doing any real work. A ghost Redis message is a no-op.
	payload := models.QueuePayload{
		JobID:      jobID,
		DocumentID: docID,
		StorageKey: storageKey,
		MimeType:   input.MimeType,
	}
	if err := s.queue.Enqueue(ctx, payload); err != nil {
		return nil, fmt.Errorf("document: enqueue: %w", err)
	}

	// 6. Atomic DB transaction: create document + job + advance status.
	//    If this fails, the Redis message is a ghost — harmless (see above).
	doc, job, err := s.createDocumentAndJob(ctx, doc, jobID)
	if err != nil {
		return nil, fmt.Errorf("document: create: %w", err)
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

// GetJobByID retrieves a job by its primary key.
func (s *Service) GetJobByID(ctx context.Context, id uuid.UUID) (*models.ProcessingJob, error) {
	return s.jobRepo.GetByID(ctx, id)
}

// ExtractionResults bundles the latest run metadata and extracted fields for a document.
type ExtractionResults struct {
	DocumentID uuid.UUID                 `json:"document_id"`
	Run        *models.ExtractionRun     `json:"run"`
	Fields     []*models.ExtractionField `json:"fields"`
}

// GetExtractionResults fetches the latest extraction run and its fields for a document.
// Returns db.ErrNotFound if the document has not yet completed extraction.
func (s *Service) GetExtractionResults(ctx context.Context, docID uuid.UUID) (*ExtractionResults, error) {
	run, err := s.extractionRepo.GetLatestRunByDocument(ctx, docID)
	if err != nil {
		return nil, err
	}

	fields, err := s.extractionRepo.GetFieldsByRun(ctx, run.ID)
	if err != nil {
		return nil, fmt.Errorf("document: get fields for run: %w", err)
	}

	return &ExtractionResults{
		DocumentID: docID,
		Run:        run,
		Fields:     fields,
	}, nil
}

// GetDocumentContent retrieves the raw file reader and MIME type for a document.
// The caller is responsible for closing the returned io.ReadCloser.
func (s *Service) GetDocumentContent(ctx context.Context, docID uuid.UUID) (io.ReadCloser, string, error) {
	doc, err := s.docRepo.GetByID(ctx, docID)
	if err != nil {
		return nil, "", err
	}

	reader, err := s.store.Download(ctx, doc.StorageKey)
	if err != nil {
		return nil, "", fmt.Errorf("document: download from storage: %w", err)
	}

	return reader, doc.MimeType, nil
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
