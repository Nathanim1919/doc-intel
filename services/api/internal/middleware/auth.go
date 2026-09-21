package middleware

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"strings"

	"github.com/doc-intel/api/internal/db"
	"github.com/doc-intel/api/pkg/models"
)

// contextKey is a package-level type to avoid context key collisions.
type contextKey string

const userContextKey contextKey = "authenticated_user"

// UserFromContext retrieves the authenticated user injected by Auth middleware.
// Returns nil if the request was not authenticated (should not happen on
// protected routes — the middleware rejects unauthenticated requests first).
func UserFromContext(ctx context.Context) *models.User {
	u, _ := ctx.Value(userContextKey).(*models.User)
	return u
}

// Auth returns a middleware that enforces API key authentication.
//
// Protocol: Authorization: Bearer <raw_api_key>
//
// The raw key is SHA-256 hashed and looked up in the users table.
// We hash on every request rather than storing the raw key — the raw key
// is shown once on creation and must never touch persistent storage.
//
// 401 if the header is missing, malformed, or the key is not in the DB.
// On success, the authenticated *models.User is injected into the context.
func Auth(userRepo *db.UserRepository) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rawKey, ok := extractBearerToken(r)
			if !ok {
				writeUnauthorized(w, "missing or malformed Authorization header")
				return
			}

			hash := hashAPIKey(rawKey)

			user, err := userRepo.FindByAPIKeyHash(r.Context(), hash)
			if err != nil {
				// Don't distinguish between invalid key and DB error to the client.
				// Leaking DB errors as 500 here would reveal auth infrastructure.
				writeUnauthorized(w, "invalid API key")
				return
			}

			ctx := context.WithValue(r.Context(), userContextKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// extractBearerToken parses "Authorization: Bearer <token>" and returns the token.
func extractBearerToken(r *http.Request) (string, bool) {
	header := r.Header.Get("Authorization")
	if header == "" {
		return "", false
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return "", false
	}
	return parts[1], true
}

// hashAPIKey returns the lowercase hex SHA-256 of the raw key.
func hashAPIKey(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("%x", sum)
}

func writeUnauthorized(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("WWW-Authenticate", `Bearer realm="doc-intel"`)
	w.WriteHeader(http.StatusUnauthorized)
	fmt.Fprintf(w, `{"error":{"code":"UNAUTHORIZED","message":%q}}`, msg)
}
