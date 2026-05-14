package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/djan/aflow/database/sqlc"
	"github.com/djan/aflow/internal/datatables/service"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PGRepository implements service.Repository for data tables.
type PGRepository struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *PGRepository {
	return &PGRepository{pool: pool}
}

func (r *PGRepository) CreateTable(ctx context.Context, in service.CreateTableInput) (*service.Table, error) {
	schema := in.Schema
	if len(schema) == 0 {
		schema = json.RawMessage(`{"columns":[]}`)
	}
	q := db.New(r.pool)
	row, err := q.CreateDataTable(ctx, db.CreateDataTableParams{
		ID:          pgtype.UUID{Bytes: uuid.New(), Valid: true},
		WorkspaceID: in.WorkspaceID,
		Name:        in.Name,
		Schema:      []byte(schema),
	})
	if err != nil {
		return nil, fmt.Errorf("create table: %w", err)
	}
	t := toTable(row)
	return &t, nil
}

func (r *PGRepository) GetTable(ctx context.Context, workspaceID, id string) (*service.Table, error) {
	tblID, err := parseUUID(id)
	if err != nil {
		return nil, service.ErrNotFound
	}
	q := db.New(r.pool)
	row, err := q.GetDataTable(ctx, db.GetDataTableParams{ID: tblID, WorkspaceID: workspaceID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, service.ErrNotFound
		}
		return nil, fmt.Errorf("get table: %w", err)
	}
	t := toTable(row)
	return &t, nil
}

func (r *PGRepository) ListTables(ctx context.Context, workspaceID string) ([]*service.Table, error) {
	q := db.New(r.pool)
	rows, err := q.ListDataTables(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list tables: %w", err)
	}
	out := make([]*service.Table, len(rows))
	for i, row := range rows {
		t := toTable(row)
		out[i] = &t
	}
	return out, nil
}

func (r *PGRepository) DeleteTable(ctx context.Context, workspaceID, id string) error {
	tblID, err := parseUUID(id)
	if err != nil {
		return service.ErrNotFound
	}
	q := db.New(r.pool)
	return q.DeleteDataTable(ctx, db.DeleteDataTableParams{ID: tblID, WorkspaceID: workspaceID})
}

func (r *PGRepository) InsertRow(ctx context.Context, in service.InsertRowInput) (*service.Row, error) {
	tblID, err := parseUUID(in.TableID)
	if err != nil {
		return nil, service.ErrNotFound
	}
	q := db.New(r.pool)
	row, err := q.InsertDataTableRow(ctx, db.InsertDataTableRowParams{
		ID:          pgtype.UUID{Bytes: uuid.New(), Valid: true},
		TableID:     tblID,
		WorkspaceID: in.WorkspaceID,
		Data:        []byte(in.Data),
	})
	if err != nil {
		return nil, fmt.Errorf("insert row: %w", err)
	}
	rv := toRow(row)
	return &rv, nil
}

func (r *PGRepository) ListRows(ctx context.Context, workspaceID, tableID string) ([]*service.Row, error) {
	tblID, err := parseUUID(tableID)
	if err != nil {
		return nil, service.ErrNotFound
	}
	q := db.New(r.pool)
	rows, err := q.ListDataTableRows(ctx, db.ListDataTableRowsParams{TableID: tblID, WorkspaceID: workspaceID})
	if err != nil {
		return nil, fmt.Errorf("list rows: %w", err)
	}
	out := make([]*service.Row, len(rows))
	for i, row := range rows {
		rv := toRow(row)
		out[i] = &rv
	}
	return out, nil
}

func (r *PGRepository) UpdateRow(ctx context.Context, workspaceID, tableID, rowID string, data json.RawMessage) (*service.Row, error) {
	tblID, err := parseUUID(tableID)
	if err != nil {
		return nil, service.ErrNotFound
	}
	rowUUID, err := parseUUID(rowID)
	if err != nil {
		return nil, service.ErrRowNotFound
	}
	q := db.New(r.pool)
	row, err := q.UpdateDataTableRow(ctx, db.UpdateDataTableRowParams{
		ID:          rowUUID,
		TableID:     tblID,
		WorkspaceID: workspaceID,
		Data:        []byte(data),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, service.ErrRowNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("update row: %w", err)
	}
	rv := toRow(row)
	return &rv, nil
}

func (r *PGRepository) DeleteRow(ctx context.Context, workspaceID, tableID, rowID string) error {
	tblID, err := parseUUID(tableID)
	if err != nil {
		return service.ErrNotFound
	}
	rowUUID, err := parseUUID(rowID)
	if err != nil {
		return service.ErrRowNotFound
	}
	q := db.New(r.pool)
	return q.DeleteDataTableRow(ctx, db.DeleteDataTableRowParams{
		ID:          rowUUID,
		TableID:     tblID,
		WorkspaceID: workspaceID,
	})
}

// --- converters ---

func toTable(r db.DataTable) service.Table {
	return service.Table{
		ID:          uuidStr(r.ID),
		WorkspaceID: r.WorkspaceID,
		Name:        r.Name,
		Schema:      json.RawMessage(r.Schema),
		CreatedAt:   tsTime(r.CreatedAt),
	}
}

func toRow(r db.DataTableRow) service.Row {
	return service.Row{
		ID:          uuidStr(r.ID),
		TableID:     uuidStr(r.TableID),
		WorkspaceID: r.WorkspaceID,
		Data:        json.RawMessage(r.Data),
		CreatedAt:   tsTime(r.CreatedAt),
		UpdatedAt:   tsTime(r.UpdatedAt),
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
