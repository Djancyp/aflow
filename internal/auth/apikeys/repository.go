// Package apikeys implements API key storage and lookup for auth middleware.
package apikeys

import (
	"context"
	"errors"
	"fmt"

	"github.com/djan/aflow/database/sqlc"
	apimw "github.com/djan/aflow/internal/api/middleware"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("API key not found")

// Repository handles API key persistence.
type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// WorkspaceByKeyHash implements apimw.APIKeyLookup.
func (r *Repository) WorkspaceByKeyHash(ctx context.Context, hash string) (string, error) {
	q := db.New(r.pool)
	row, err := q.GetAPIKeyByHash(ctx, hash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("lookup api key: %w", err)
	}
	return row.WorkspaceID, nil
}

// Create stores a new API key (caller provides the raw key; we hash it).
func (r *Repository) Create(ctx context.Context, workspaceID, name, rawKey string) error {
	hash := apimw.HashAPIKey(rawKey)
	q := db.New(r.pool)
	_, err := q.CreateAPIKey(ctx, db.CreateAPIKeyParams{
		ID:          pgtype.UUID{Bytes: uuid.New(), Valid: true},
		WorkspaceID: workspaceID,
		Name:        name,
		KeyHash:     hash,
	})
	return err
}
