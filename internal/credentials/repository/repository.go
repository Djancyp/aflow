package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/djan/aflow/database/sqlc"
	"github.com/djan/aflow/internal/credentials/service"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PGRepository implements service.Repository for credentials.
type PGRepository struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *PGRepository {
	return &PGRepository{pool: pool}
}

func (r *PGRepository) Create(ctx context.Context, in service.CreateInput, encryptedData []byte) (*service.Credential, error) {
	q := db.New(r.pool)
	row, err := q.CreateCredential(ctx, db.CreateCredentialParams{
		ID:            pgtype.UUID{Bytes: uuid.New(), Valid: true},
		WorkspaceID:   in.WorkspaceID,
		Name:          in.Name,
		Type:          in.Type,
		EncryptedData: encryptedData,
	})
	if err != nil {
		return nil, fmt.Errorf("create credential: %w", err)
	}
	c := toCred(row)
	return &c, nil
}

func (r *PGRepository) Get(ctx context.Context, workspaceID, id string) (*service.Credential, error) {
	credID, err := parseUUID(id)
	if err != nil {
		return nil, service.ErrNotFound
	}
	q := db.New(r.pool)
	row, err := q.GetCredential(ctx, db.GetCredentialParams{ID: credID, WorkspaceID: workspaceID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, service.ErrNotFound
		}
		return nil, fmt.Errorf("get credential: %w", err)
	}
	c := toCred(row)
	return &c, nil
}

func (r *PGRepository) List(ctx context.Context, workspaceID string) ([]*service.Credential, error) {
	q := db.New(r.pool)
	rows, err := q.ListCredentials(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list credentials: %w", err)
	}
	out := make([]*service.Credential, len(rows))
	for i, row := range rows {
		c := toCred(row)
		out[i] = &c
	}
	return out, nil
}

func (r *PGRepository) Delete(ctx context.Context, workspaceID, id string) error {
	credID, err := parseUUID(id)
	if err != nil {
		return service.ErrNotFound
	}
	q := db.New(r.pool)
	return q.DeleteCredential(ctx, db.DeleteCredentialParams{ID: credID, WorkspaceID: workspaceID})
}

func toCred(r db.Credential) service.Credential {
	return service.Credential{
		ID:            uuidStr(r.ID),
		WorkspaceID:   r.WorkspaceID,
		Name:          r.Name,
		Type:          r.Type,
		EncryptedData: r.EncryptedData,
		CreatedAt:     tsTime(r.CreatedAt),
	}
}

func parseUUID(s string) (pgtype.UUID, error) {
	u, err := uuid.Parse(s)
	if err != nil {
		return pgtype.UUID{}, err
	}
	return pgtype.UUID{Bytes: u, Valid: true}, nil
}

func uuidStr(u pgtype.UUID) string {
	return uuid.UUID(u.Bytes).String()
}

func tsTime(ts pgtype.Timestamp) time.Time {
	if !ts.Valid {
		return time.Time{}
	}
	return ts.Time
}
