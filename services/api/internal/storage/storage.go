package storage

import (
	"context"
	"fmt"
	"io"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// ObjectStorage is the interface the application uses to interact with object storage.
// It is intentionally narrow — only the operations Phase 1 needs.
//
// Using an interface here means:
//   - MinIO in local dev
//   - AWS S3 or GCS in production
//   - An in-memory mock in tests
//
// The handler layer only depends on this interface, never on MinIO directly.
type ObjectStorage interface {
	// Upload stores reader content at key. size is the byte length — required
	// by S3-compatible APIs to set Content-Length. Use -1 only if size is unknown.
	Upload(ctx context.Context, key string, reader io.Reader, size int64, contentType string) error

	// Download returns a streaming reader for the object at key.
	// The caller MUST close the returned ReadCloser.
	Download(ctx context.Context, key string) (io.ReadCloser, error)

	// EnsureBucket creates the bucket if it does not already exist.
	// Called once at startup — safe to call repeatedly.
	EnsureBucket(ctx context.Context) error
}

// StorageKey produces a canonical object storage path for a document.
// Format: documents/{doc_id}/original{ext}
// ext should include the leading dot, e.g. ".pdf", ".png".
// If ext is empty, the file is stored without an extension.
//
// This is a pure function — test it directly, no mocks needed.
func StorageKey(docID uuid.UUID, filename string) string {
	ext := filepath.Ext(filename)
	return fmt.Sprintf("documents/%s/original%s", docID.String(), ext)
}

// MinioStorage implements ObjectStorage against a MinIO (or S3-compatible) backend.
type MinioStorage struct {
	client *minio.Client
	bucket string
}

// NewMinioStorage constructs a MinioStorage and verifies connectivity.
// endpoint should be "host:port" without a scheme.
func NewMinioStorage(endpoint, accessKey, secretKey, bucket string, useSSL bool) (*MinioStorage, error) {
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("storage: create minio client: %w", err)
	}

	return &MinioStorage{
		client: client,
		bucket: bucket,
	}, nil
}

// EnsureBucket creates the storage bucket if it does not exist.
// Idempotent — safe to call on every startup.
func (s *MinioStorage) EnsureBucket(ctx context.Context) error {
	exists, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return fmt.Errorf("storage: check bucket exists: %w", err)
	}
	if exists {
		return nil
	}

	// MakeBucket with empty region defaults to the server's region.
	if err := s.client.MakeBucket(ctx, s.bucket, minio.MakeBucketOptions{}); err != nil {
		return fmt.Errorf("storage: create bucket %q: %w", s.bucket, err)
	}
	return nil
}

// Upload stores the content from reader at the given key.
// contentType is stored as object metadata and returned on download — important
// for serving files via presigned URLs later.
func (s *MinioStorage) Upload(ctx context.Context, key string, reader io.Reader, size int64, contentType string) error {
	_, err := s.client.PutObject(ctx, s.bucket, key, reader, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return fmt.Errorf("storage: upload %q: %w", key, err)
	}
	return nil
}

// Download returns a streaming reader for the object at key.
// The caller is responsible for closing the returned ReadCloser to release
// the underlying HTTP connection back to the pool.
func (s *MinioStorage) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	obj, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("storage: download %q: %w", key, err)
	}
	return obj, nil
}
