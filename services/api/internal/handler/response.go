package handler

import (
	"encoding/json"
	"net/http"
)

// writeJSON encodes v as JSON and writes it with the given HTTP status code.
// Sets Content-Type to application/json.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		// At this point headers are already sent — nothing to do but log.
		// In production this would go to a structured logger.
		_ = err
	}
}

// errorResponse is the standard error envelope for all API errors.
// Format: {"error": {"code": "NOT_FOUND", "message": "document not found"}}
type errorResponse struct {
	Error errorBody `json:"error"`
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// writeError writes a JSON error response with the given HTTP status.
// code should be a SCREAMING_SNAKE_CASE machine-readable string.
// message is human-readable and safe to surface to the client.
func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorResponse{
		Error: errorBody{Code: code, Message: message},
	})
}
