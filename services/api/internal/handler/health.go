package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// Health handles GET /v1/health
//
// Pings both Postgres and Redis with a 3-second timeout.
// Returns 200 if both are healthy, 503 if either is unreachable.
// No auth required — used by load balancers and Docker healthchecks.
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	type serviceStatus struct {
		OK    bool   `json:"ok"`
		Error string `json:"error,omitempty"`
	}

	type healthResponse struct {
		Status   string                   `json:"status"`
		Services map[string]serviceStatus `json:"services"`
	}

	services := map[string]serviceStatus{}
	allOK := true

	// Ping Postgres
	if err := h.pool.Ping(ctx); err != nil {
		services["postgres"] = serviceStatus{OK: false, Error: err.Error()}
		allOK = false
	} else {
		services["postgres"] = serviceStatus{OK: true}
	}

	// Ping Redis
	if err := h.redis.Ping(ctx).Err(); err != nil {
		services["redis"] = serviceStatus{OK: false, Error: err.Error()}
		allOK = false
	} else {
		services["redis"] = serviceStatus{OK: true}
	}

	overall := "ok"
	statusCode := http.StatusOK
	if !allOK {
		overall = "degraded"
		statusCode = http.StatusServiceUnavailable
	}

	writeJSON(w, statusCode, healthResponse{
		Status:   overall,
		Services: services,
	})
}

// healthDeps carries dependencies needed only for health checks.
// Kept separate from the main Handler to keep the constructor clean.
type healthDeps struct {
	pool  *pgxpool.Pool
	redis *redis.Client
}
