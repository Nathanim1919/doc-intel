package models

import (
	"time"

	"github.com/google/uuid"
)

// User is the auth principal. One user belongs to one organization.
// The raw API key is shown once on creation and never stored.
// Only the SHA-256 hex hash is persisted here.
type User struct {
	ID             uuid.UUID `json:"id"`
	OrganizationID uuid.UUID `json:"organization_id"`
	Email          string    `json:"email"`
	APIKeyHash     string    `json:"-"` // never serialised to JSON
	CreatedAt      time.Time `json:"created_at"`
}
