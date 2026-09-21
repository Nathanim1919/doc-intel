package handler

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/doc-intel/api/internal/db"
	"github.com/doc-intel/api/internal/document"
	"github.com/doc-intel/api/internal/middleware"
	"github.com/doc-intel/api/pkg/models"
)

const (
	maxUploadBytes = 10 << 20 // 10 MB
)

// allowedMIMETypes is the Phase 1 allowlist.
// Validated against the sniffed content type, not the client-supplied header.
var allowedMIMETypes = map[string]bool{
	"application/pdf": true,
	"image/png":       true,
	"image/jpeg":      true,
	"image/tiff":      true,
}

// uploadResponse is returned on successful document upload.
type uploadResponse struct {
	DocumentID  string `json:"document_id"`
	JobID       string `json:"job_id,omitempty"`
	Status      string `json:"status"`
	IsDuplicate bool   `json:"is_duplicate"`
}

// documentResponse is the full document representation.
type documentResponse struct {
	ID         string `json:"id"`
	Filename   string `json:"filename"`
	MimeType   string `json:"mime_type"`
	SizeBytes  int64  `json:"size_bytes"`
	Status     string `json:"status"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

// statusResponse is the lightweight status-check representation.
type statusResponse struct {
	DocumentID string `json:"document_id"`
	Status     string `json:"status"`
	UpdatedAt  string `json:"updated_at"`
}

// UploadDocument handles POST /v1/documents
//
// Flow:
//  1. Parse multipart form (max 10 MB)
//  2. Read and sniff the file content for MIME type
//  3. Validate MIME type against allowlist
//  4. Sanitize filename (basename only, no path traversal)
//  5. Call service.Upload
//  6. Return 200 if duplicate, 202 if new
func (h *Handler) UploadDocument(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
		writeError(w, http.StatusRequestEntityTooLarge, "FILE_TOO_LARGE",
			"file exceeds the 10 MB limit or request is not multipart")
		return
	}
	defer r.MultipartForm.RemoveAll() //nolint:errcheck

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "MISSING_FILE",
			"field 'file' is required")
		return
	}
	defer file.Close()

	// Read entire file into a buffer so we can:
	//   a) sniff MIME type from the first 512 bytes
	//   b) pass the full bytes to the service (which hashes and uploads)
	// ParseMultipartForm already enforced the 10 MB limit above.
	data, err := io.ReadAll(file)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "READ_ERROR",
			"failed to read uploaded file")
		return
	}

	// Sniff MIME type from content — more reliable than Content-Type header.
	sniffed := http.DetectContentType(data)
	mimeType := normalizeMIME(sniffed)
	if !allowedMIMETypes[mimeType] {
		writeError(w, http.StatusUnsupportedMediaType, "UNSUPPORTED_FILE_TYPE",
			"only PDF, PNG, JPEG, and TIFF files are accepted")
		return
	}

	// Sanitize filename — take only the base name to prevent path traversal.
	filename := filepath.Base(header.Filename)
	if filename == "." || filename == "/" {
		filename = "upload"
	}

	user := middleware.UserFromContext(r.Context())

	result, err := h.svc.Upload(r.Context(), document.UploadInput{
		OwnerID:   user.ID,
		Filename:  filename,
		MimeType:  mimeType,
		SizeBytes: int64(len(data)),
		Content:   bytes.NewReader(data),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "UPLOAD_FAILED", err.Error())
		return
	}

	resp := uploadResponse{
		DocumentID:  result.Document.ID.String(),
		Status:      result.Document.Status,
		IsDuplicate: result.IsDupe,
	}
	if result.Job != nil {
		resp.JobID = result.Job.ID.String()
	}

	// 200 if duplicate (idempotent — same file already exists), 202 if new.
	status := http.StatusAccepted
	if result.IsDupe {
		status = http.StatusOK
	}
	writeJSON(w, status, resp)
}

// GetDocument handles GET /v1/documents/{id}
func (h *Handler) GetDocument(w http.ResponseWriter, r *http.Request) {
	docID, ok := parseUUID(w, chi.URLParam(r, "id"))
	if !ok {
		return
	}

	doc, err := h.svc.GetByID(r.Context(), docID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "document not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to retrieve document")
		return
	}

	writeJSON(w, http.StatusOK, toDocumentResponse(doc))
}

// GetDocumentStatus handles GET /v1/documents/{id}/status
// Lightweight — returns only the current status without the full document.
func (h *Handler) GetDocumentStatus(w http.ResponseWriter, r *http.Request) {
	docID, ok := parseUUID(w, chi.URLParam(r, "id"))
	if !ok {
		return
	}

	doc, err := h.svc.GetByID(r.Context(), docID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "document not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to retrieve document")
		return
	}

	writeJSON(w, http.StatusOK, statusResponse{
		DocumentID: doc.ID.String(),
		Status:     doc.Status,
		UpdatedAt:  doc.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	})
}

// GetDocumentResults handles GET /v1/documents/{id}/results
// Returns the latest extraction run metadata and all extracted fields for the document.
func (h *Handler) GetDocumentResults(w http.ResponseWriter, r *http.Request) {
	docID, ok := parseUUID(w, chi.URLParam(r, "id"))
	if !ok {
		return
	}

	results, err := h.svc.GetExtractionResults(r.Context(), docID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "no extraction results found for document")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to retrieve extraction results")
		return
	}

	writeJSON(w, http.StatusOK, toExtractionResultsResponse(results))
}

// GetDocumentContent handles GET /v1/documents/{id}/content
// Streams the raw stored document (PDF or image) directly with the proper Content-Type.
func (h *Handler) GetDocumentContent(w http.ResponseWriter, r *http.Request) {
	docID, ok := parseUUID(w, chi.URLParam(r, "id"))
	if !ok {
		return
	}

	reader, contentType, err := h.svc.GetDocumentContent(r.Context(), docID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "document not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to read document content")
		return
	}
	defer reader.Close()

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "public, max-age=3600")
	if _, err := io.Copy(w, reader); err != nil {
		_ = err
	}
}

// ListDocuments handles GET /v1/documents?status=QUEUED
func (h *Handler) ListDocuments(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	statusFilter := r.URL.Query().Get("status")

	docs, err := h.svc.ListByOwner(r.Context(), user.ID, statusFilter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list documents")
		return
	}

	resp := make([]documentResponse, len(docs))
	for i, d := range docs {
		resp[i] = toDocumentResponse(d)
	}
	writeJSON(w, http.StatusOK, map[string]any{"documents": resp, "count": len(resp)})
}

// GetJob handles GET /v1/jobs/{id}
func (h *Handler) GetJob(w http.ResponseWriter, r *http.Request) {
	jobID, ok := parseUUID(w, chi.URLParam(r, "id"))
	if !ok {
		return
	}

	job, err := h.svc.GetJobByID(r.Context(), jobID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "job not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to retrieve job")
		return
	}

	writeJSON(w, http.StatusOK, job)
}

// --- helpers ----------------------------------------------------------------

func parseUUID(w http.ResponseWriter, raw string) (uuid.UUID, bool) {
	id, err := uuid.Parse(raw)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_ID", "id must be a valid UUID")
		return uuid.Nil, false
	}
	return id, true
}

func toDocumentResponse(d *models.Document) documentResponse {
	return documentResponse{
		ID:        d.ID.String(),
		Filename:  d.Filename,
		MimeType:  d.MimeType,
		SizeBytes: d.SizeBytes,
		Status:    d.Status,
		CreatedAt: d.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt: d.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

type extractionRunResponse struct {
	ID            string  `json:"id"`
	JobID         string  `json:"job_id"`
	Model         string  `json:"model"`
	ModelVersion  string  `json:"model_version,omitempty"`
	PromptVersion string  `json:"prompt_version"`
	Status        string  `json:"status"`
	LatencyMs     *int    `json:"latency_ms,omitempty"`
	TokensUsed    *int    `json:"tokens_used,omitempty"`
	Error         *string `json:"error,omitempty"`
	CreatedAt     string  `json:"created_at"`
}

type extractionFieldResponse struct {
	ID                 string   `json:"id"`
	FieldName          string   `json:"field_name"`
	Value              *string  `json:"value,omitempty"`
	RawConfidence      *float64 `json:"raw_confidence,omitempty"`
	ComputedConfidence *float64 `json:"computed_confidence,omitempty"`
	ValidationStatus   string   `json:"validation_status"`
	PageNumber         *int     `json:"page_number,omitempty"`
}

type extractionResultsResponse struct {
	DocumentID string                    `json:"document_id"`
	Run        extractionRunResponse     `json:"run"`
	Fields     []extractionFieldResponse `json:"fields"`
}

func toExtractionResultsResponse(res *document.ExtractionResults) extractionResultsResponse {
	run := extractionRunResponse{
		ID:            res.Run.ID.String(),
		JobID:         res.Run.JobID.String(),
		Model:         res.Run.Model,
		ModelVersion:  res.Run.ModelVersion,
		PromptVersion: res.Run.PromptVersion,
		Status:        res.Run.Status,
		LatencyMs:     res.Run.LatencyMs,
		TokensUsed:    res.Run.TokensUsed,
		Error:         res.Run.Error,
		CreatedAt:     res.Run.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}

	fields := make([]extractionFieldResponse, len(res.Fields))
	for i, f := range res.Fields {
		fields[i] = extractionFieldResponse{
			ID:                 f.ID.String(),
			FieldName:          f.FieldName,
			Value:              f.Value,
			RawConfidence:      f.RawConfidence,
			ComputedConfidence: f.ComputedConfidence,
			ValidationStatus:   f.ValidationStatus,
			PageNumber:         f.PageNumber,
		}
	}

	return extractionResultsResponse{
		DocumentID: res.DocumentID.String(),
		Run:        run,
		Fields:     fields,
	}
}

// normalizeMIME strips parameters like "; charset=utf-8" from a MIME type.
func normalizeMIME(mime string) string {
	if idx := strings.Index(mime, ";"); idx != -1 {
		return strings.TrimSpace(mime[:idx])
	}
	return strings.TrimSpace(mime)
}
