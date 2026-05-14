package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/djan/aflow/database/sqlc"
	"github.com/djan/aflow/internal/executions/service"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PGRepository implements service.Repository against PostgreSQL.
type PGRepository struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *PGRepository {
	return &PGRepository{pool: pool}
}

// PrepareExecution verifies the workflow is active + published, then creates an execution.
func (r *PGRepository) PrepareExecution(ctx context.Context, in service.PrepareInput) (*service.Execution, error) {
	wfUUID, err := parseUUID(in.WorkflowID)
	if err != nil {
		return nil, service.ErrWorkflowNotFound
	}

	input := in.Input
	if len(input) == 0 {
		input = json.RawMessage("{}")
	}

	var exec service.Execution

	err = pgx.BeginTxFunc(ctx, r.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		q := db.New(tx)

		wf, err := q.GetWorkflow(ctx, db.GetWorkflowParams{ID: wfUUID, WorkspaceID: in.WorkspaceID})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return service.ErrWorkflowNotFound
			}
			return fmt.Errorf("get workflow: %w", err)
		}
		if !wf.Active {
			return service.ErrWorkflowInactive
		}

		ver, err := q.GetLatestPublishedVersion(ctx, wfUUID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return service.ErrNoPublishedVersion
			}
			return fmt.Errorf("get latest version: %w", err)
		}

		row, err := q.CreateExecution(ctx, db.CreateExecutionParams{
			ID:                pgtype.UUID{Bytes: uuid.New(), Valid: true},
			WorkspaceID:       in.WorkspaceID,
			WorkflowID:        wfUUID,
			WorkflowVersionID: ver.ID,
			TriggerSource:     &in.TriggerSource,
			Input:             []byte(input),
		})
		if err != nil {
			return fmt.Errorf("create execution: %w", err)
		}

		exec = toExecution(row)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &exec, nil
}

func (r *PGRepository) GetExecution(ctx context.Context, workspaceID, id string) (*service.Execution, error) {
	execUUID, err := parseUUID(id)
	if err != nil {
		return nil, service.ErrNotFound
	}
	q := db.New(r.pool)
	row, err := q.GetExecution(ctx, db.GetExecutionParams{ID: execUUID, WorkspaceID: workspaceID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, service.ErrNotFound
		}
		return nil, fmt.Errorf("get execution: %w", err)
	}
	e := toExecution(row)
	return &e, nil
}

func (r *PGRepository) GetExecutionInternal(ctx context.Context, id string) (*service.Execution, error) {
	execUUID, err := parseUUID(id)
	if err != nil {
		return nil, service.ErrNotFound
	}
	q := db.New(r.pool)
	row, err := q.GetExecutionInternal(ctx, execUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, service.ErrNotFound
		}
		return nil, fmt.Errorf("get execution internal: %w", err)
	}
	e := toExecution(row)
	return &e, nil
}

func (r *PGRepository) ListByWorkspace(ctx context.Context, workspaceID string, limit, offset int32) ([]*service.Execution, error) {
	q := db.New(r.pool)
	rows, err := q.ListExecutionsByWorkspace(ctx, db.ListExecutionsByWorkspaceParams{
		WorkspaceID: workspaceID,
		Limit:       limit,
		Offset:      offset,
	})
	if err != nil {
		return nil, fmt.Errorf("list executions by workspace: %w", err)
	}
	out := make([]*service.Execution, len(rows))
	for i, row := range rows {
		e := toExecution(row)
		out[i] = &e
	}
	return out, nil
}

func (r *PGRepository) ListByWorkflow(ctx context.Context, workspaceID, workflowID string) ([]*service.Execution, error) {
	wfUUID, err := parseUUID(workflowID)
	if err != nil {
		return nil, service.ErrWorkflowNotFound
	}
	q := db.New(r.pool)
	rows, err := q.ListExecutionsByWorkflow(ctx, db.ListExecutionsByWorkflowParams{
		WorkflowID:  wfUUID,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		return nil, fmt.Errorf("list executions: %w", err)
	}
	out := make([]*service.Execution, len(rows))
	for i, row := range rows {
		e := toExecution(row)
		out[i] = &e
	}
	return out, nil
}

func (r *PGRepository) CancelExecution(ctx context.Context, workspaceID, id string) (*service.Execution, error) {
	execUUID, err := parseUUID(id)
	if err != nil {
		return nil, service.ErrNotFound
	}
	q := db.New(r.pool)
	row, err := q.CancelExecution(ctx, db.CancelExecutionParams{ID: execUUID, WorkspaceID: workspaceID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, service.ErrCancelForbidden
		}
		return nil, fmt.Errorf("cancel execution: %w", err)
	}
	e := toExecution(row)
	return &e, nil
}

func (r *PGRepository) GetLogs(ctx context.Context, executionID string) ([]*service.ExecutionLog, error) {
	execUUID, err := parseUUID(executionID)
	if err != nil {
		return nil, service.ErrNotFound
	}
	q := db.New(r.pool)
	rows, err := q.ListExecutionLogs(ctx, execUUID)
	if err != nil {
		return nil, fmt.Errorf("list logs: %w", err)
	}
	out := make([]*service.ExecutionLog, len(rows))
	for i, row := range rows {
		l := toLog(row)
		out[i] = &l
	}
	return out, nil
}

func (r *PGRepository) StartExecution(ctx context.Context, id string) error {
	execUUID, err := parseUUID(id)
	if err != nil {
		return service.ErrNotFound
	}
	q := db.New(r.pool)
	return q.StartExecution(ctx, execUUID)
}

func (r *PGRepository) FinishExecution(ctx context.Context, id string, status service.ExecutionStatus, output json.RawMessage, errMsg string) error {
	execUUID, err := parseUUID(id)
	if err != nil {
		return service.ErrNotFound
	}
	var errPtr *string
	if errMsg != "" {
		errPtr = &errMsg
	}
	var outBytes []byte
	if len(output) > 0 {
		outBytes = []byte(output)
	}
	q := db.New(r.pool)
	return q.FinishExecution(ctx, db.FinishExecutionParams{
		ID:     execUUID,
		Status: string(status),
		Output: outBytes,
		Error:  errPtr,
	})
}

func (r *PGRepository) WriteLog(ctx context.Context, in service.WriteLogInput) error {
	execUUID, err := parseUUID(in.ExecutionID)
	if err != nil {
		return nil
	}
	var meta []byte
	if len(in.Metadata) > 0 {
		meta = []byte(in.Metadata)
	}
	q := db.New(r.pool)
	return q.CreateExecutionLog(ctx, db.CreateExecutionLogParams{
		ExecutionID: execUUID,
		NodeID:      in.NodeID,
		Level:       in.Level,
		Message:     in.Message,
		Metadata:    meta,
	})
}

func (r *PGRepository) GetVersionDefinition(ctx context.Context, versionID string) (json.RawMessage, error) {
	verUUID, err := parseUUID(versionID)
	if err != nil {
		return nil, service.ErrNotFound
	}
	q := db.New(r.pool)
	row, err := q.GetWorkflowVersionByID(ctx, verUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, service.ErrNotFound
		}
		return nil, fmt.Errorf("get version: %w", err)
	}
	return json.RawMessage(row.Definition), nil
}

// --- converters ---

func toExecution(r db.Execution) service.Execution {
	e := service.Execution{
		ID:                uuidStr(r.ID),
		WorkspaceID:       r.WorkspaceID,
		WorkflowID:        uuidStr(r.WorkflowID),
		WorkflowVersionID: uuidStr(r.WorkflowVersionID),
		Status:            service.ExecutionStatus(r.Status),
		TriggerSource:     r.TriggerSource,
		Input:             json.RawMessage(r.Input),
		Output:            json.RawMessage(r.Output),
		Error:             r.Error,
		CreatedAt:         tsTime(r.CreatedAt),
	}
	if r.StartedAt.Valid {
		t := r.StartedAt.Time
		e.StartedAt = &t
	}
	if r.FinishedAt.Valid {
		t := r.FinishedAt.Time
		e.FinishedAt = &t
	}
	return e
}

func toLog(r db.ExecutionLog) service.ExecutionLog {
	return service.ExecutionLog{
		ID:          r.ID,
		ExecutionID: uuidStr(r.ExecutionID),
		NodeID:      r.NodeID,
		Level:       r.Level,
		Message:     r.Message,
		Metadata:    json.RawMessage(r.Metadata),
		CreatedAt:   tsTime(r.CreatedAt),
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
