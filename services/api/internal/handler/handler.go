package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/doc-intel/api/internal/db"
	"github.com/doc-intel/api/internal/document"
	apimiddleware "github.com/doc-intel/api/internal/middleware"
)

// Handler holds all HTTP handler dependencies.
// Constructed once in main and reused across requests.
type Handler struct {
	svc      *document.Service
	userRepo *db.UserRepository
	pool     *pgxpool.Pool
	redis    *redis.Client
}

// New constructs a Handler. All fields are required.
func New(
	svc *document.Service,
	userRepo *db.UserRepository,
	pool *pgxpool.Pool,
	redisClient *redis.Client,
) *Handler {
	return &Handler{
		svc:      svc,
		userRepo: userRepo,
		pool:     pool,
		redis:    redisClient,
	}
}

// Routes builds and returns the full HTTP router.
//
// Route structure:
//
//	GET  /v1/health                    → no auth
//	POST /v1/documents                 → auth required
//	GET  /v1/documents                 → auth required
//	GET  /v1/documents/{id}            → auth required
//	GET  /v1/documents/{id}/status     → auth required
//	GET  /v1/jobs/{id}                 → auth required
func (h *Handler) Routes() http.Handler {
	r := chi.NewRouter()

	// Global middleware — applied to all routes
	r.Use(middleware.RequestID) // injects X-Request-Id header
	r.Use(middleware.Logger)    // structured request log per request
	r.Use(middleware.Recoverer) // catch panics, return 500 instead of crashing

	// Public routes — no auth
	r.Get("/v1/health", h.Health)

	// Protected routes — API key auth
	r.Group(func(r chi.Router) {
		r.Use(apimiddleware.Auth(h.userRepo))

		r.Post("/v1/documents", h.UploadDocument)
		r.Get("/v1/documents", h.ListDocuments)
		r.Get("/v1/documents/{id}", h.GetDocument)
		r.Get("/v1/documents/{id}/status", h.GetDocumentStatus)
		r.Get("/v1/jobs/{id}", h.GetJob)
	})

	return r
}
