package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/doc-intel/api/pkg/models"
)

// UserRepository handles all database operations for users.
type UserRepository struct {
	db DBTX
}

// NewUserRepository constructs a UserRepository.
func NewUserRepository(db DBTX) *UserRepository {
	return &UserRepository{db: db}
}

// FindByAPIKeyHash looks up a user by the SHA-256 hex hash of their API key.
// This is called on every authenticated request — keep it fast.
// The index on api_key_hash is implicit via uniqueness; add one explicitly if
// query plans show a seq scan under load.
func (r *UserRepository) FindByAPIKeyHash(ctx context.Context, keyHash string) (*models.User, error) {
	const q = `
		SELECT id, organization_id, email, api_key_hash, created_at
		FROM users
		WHERE api_key_hash = $1
		LIMIT 1
	`
	row := r.db.QueryRow(ctx, q, keyHash)
	user := &models.User{}
	err := row.Scan(
		&user.ID,
		&user.OrganizationID,
		&user.Email,
		&user.APIKeyHash,
		&user.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("db: find user by api key: %w", err)
	}
	return user, nil
}
