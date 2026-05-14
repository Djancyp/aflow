package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/djan/aflow/database/sqlc"
	"github.com/djan/aflow/internal/nodetypes/service"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PGRepository implements service.Repository for node types.
type PGRepository struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *PGRepository {
	return &PGRepository{pool: pool}
}

func (r *PGRepository) Create(ctx context.Context, in service.CreateInput) (*service.NodeType, error) {
	credID := pgtype.UUID{}
	if in.CredentialID != nil {
		u, err := uuid.Parse(*in.CredentialID)
		if err == nil {
			credID = pgtype.UUID{Bytes: u, Valid: true}
		}
	}

	q := db.New(r.pool)
	row, err := q.CreateNodeType(ctx, db.CreateNodeTypeParams{
		ID:              pgtype.UUID{Bytes: uuid.New(), Valid: true},
		WorkspaceID:     in.WorkspaceID,
		Name:            in.Name,
		Description:     in.Description,
		Category:        in.Category,
		Version:         in.Version,
		BaseUrl:         in.BaseURL,
		Endpoint:        in.Endpoint,
		Method:          in.Method,
		ContentType:     in.ContentType,
		HeadersTemplate: marshalJSON(in.HeadersTemplate),
		BodyTemplate:    in.BodyTemplate,
		InputSchema:     marshalJSONRequired(in.InputSchema),
		OutputSchema:    marshalJSON(in.OutputSchema),
		CredentialID:    credID,
	})
	if err != nil {
		return nil, fmt.Errorf("create node type: %w", err)
	}
	nt := toNodeType(row)
	return &nt, nil
}

func (r *PGRepository) Get(ctx context.Context, workspaceID, id string) (*service.NodeType, error) {
	ntID, err := parseUUID(id)
	if err != nil {
		return nil, service.ErrNotFound
	}
	q := db.New(r.pool)
	row, err := q.GetNodeType(ctx, db.GetNodeTypeParams{ID: ntID, WorkspaceID: workspaceID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, service.ErrNotFound
		}
		return nil, fmt.Errorf("get node type: %w", err)
	}
	nt := toNodeType(row)
	return &nt, nil
}

func (r *PGRepository) List(ctx context.Context, workspaceID, q, category string) ([]*service.NodeType, error) {
	query := db.New(r.pool)
	rows, err := query.ListNodeTypes(ctx, db.ListNodeTypesParams{
		WorkspaceID: workspaceID,
		Column2:     q,
		Column3:     category,
	})
	if err != nil {
		return nil, fmt.Errorf("list node types: %w", err)
	}
	out := make([]*service.NodeType, len(rows))
	for i, row := range rows {
		nt := toNodeType(row)
		out[i] = &nt
	}
	return out, nil
}

func (r *PGRepository) Update(ctx context.Context, in service.UpdateInput) (*service.NodeType, error) {
	ntID, err := parseUUID(in.ID)
	if err != nil {
		return nil, service.ErrNotFound
	}
	credID := pgtype.UUID{}
	if in.CredentialID != nil {
		u, err := uuid.Parse(*in.CredentialID)
		if err == nil {
			credID = pgtype.UUID{Bytes: u, Valid: true}
		}
	}
	q := db.New(r.pool)
	row, err := q.UpdateNodeType(ctx, db.UpdateNodeTypeParams{
		ID:              ntID,
		WorkspaceID:     in.WorkspaceID,
		Name:            in.Name,
		Description:     in.Description,
		Category:        in.Category,
		BaseUrl:         in.BaseURL,
		Endpoint:        in.Endpoint,
		Method:          in.Method,
		ContentType:     in.ContentType,
		HeadersTemplate: marshalJSON(in.HeadersTemplate),
		BodyTemplate:    in.BodyTemplate,
		InputSchema:     marshalJSONRequired(in.InputSchema),
		OutputSchema:    marshalJSON(in.OutputSchema),
		CredentialID:    credID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, service.ErrNotFound
		}
		return nil, fmt.Errorf("update node type: %w", err)
	}
	nt := toNodeType(row)
	return &nt, nil
}

func (r *PGRepository) Delete(ctx context.Context, workspaceID, id string) error {
	ntID, err := parseUUID(id)
	if err != nil {
		return service.ErrNotFound
	}
	q := db.New(r.pool)
	return q.DeleteNodeType(ctx, db.DeleteNodeTypeParams{ID: ntID, WorkspaceID: workspaceID})
}

// --- converters ---

func toNodeType(r db.NodeType) service.NodeType {
	nt := service.NodeType{
		ID:          uuidStr(r.ID),
		WorkspaceID: r.WorkspaceID,
		Name:        r.Name,
		Description: r.Description,
		Category:    r.Category,
		Version:     r.Version,
		BaseURL:     r.BaseUrl,
		Endpoint:    r.Endpoint,
		Method:      r.Method,
		ContentType: r.ContentType,
		BodyTemplate: r.BodyTemplate,
		Active:      r.Active,
		CreatedAt:   tsTime(r.CreatedAt),
		UpdatedAt:   tsTime(r.UpdatedAt),
	}
	if r.CredentialID.Valid {
		s := uuidStr(r.CredentialID)
		nt.CredentialID = &s
	}
	if len(r.HeadersTemplate) > 0 {
		var m map[string]any
		_ = json.Unmarshal(r.HeadersTemplate, &m)
		nt.HeadersTemplate = m
	}
	if len(r.InputSchema) > 0 {
		var m map[string]any
		_ = json.Unmarshal(r.InputSchema, &m)
		nt.InputSchema = m
	}
	if len(r.OutputSchema) > 0 {
		var m map[string]any
		_ = json.Unmarshal(r.OutputSchema, &m)
		nt.OutputSchema = m
	}
	return nt
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

func marshalJSON(m map[string]any) []byte {
	if m == nil {
		return nil
	}
	b, _ := json.Marshal(m)
	return b
}

func marshalJSONRequired(m map[string]any) []byte {
	if m == nil {
		return []byte(`{}`)
	}
	b, _ := json.Marshal(m)
	return b
}
