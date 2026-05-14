package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/djan/aflow/database/sqlc"
	"github.com/djan/aflow/internal/workflows/service"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PGRepository implements service.Repository against PostgreSQL via sqlc.
type PGRepository struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *PGRepository {
	return &PGRepository{pool: pool}
}

// CreateWithVersion creates a workflow + its initial version in a single transaction.
func (r *PGRepository) CreateWithVersion(ctx context.Context, in service.CreateInput) (*service.Workflow, *service.WorkflowVersion, error) {
	wfID := pgtype.UUID{Bytes: uuid.New(), Valid: true}
	vID := pgtype.UUID{Bytes: uuid.New(), Valid: true}

	defBytes, err := json.Marshal(in.Definition)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal definition: %w", err)
	}

	var wf service.Workflow
	var ver service.WorkflowVersion

	err = pgx.BeginTxFunc(ctx, r.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		q := db.New(tx)

		row, err := q.CreateWorkflow(ctx, db.CreateWorkflowParams{
			ID:          wfID,
			WorkspaceID: in.WorkspaceID,
			Name:        in.Name,
			Description: in.Description,
		})
		if err != nil {
			return fmt.Errorf("create workflow: %w", err)
		}
		wf = toWorkflow(row)

		vRow, err := q.CreateWorkflowVersion(ctx, db.CreateWorkflowVersionParams{
			ID:         vID,
			WorkflowID: wfID,
			Version:    1,
			Definition: defBytes,
		})
		if err != nil {
			return fmt.Errorf("create workflow version: %w", err)
		}
		ver = toVersion(vRow)
		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	return &wf, &ver, nil
}

func (r *PGRepository) GetByID(ctx context.Context, workspaceID, id string) (*service.Workflow, error) {
	wfID, err := parseUUID(id)
	if err != nil {
		return nil, service.ErrNotFound
	}
	q := db.New(r.pool)
	row, err := q.GetWorkflow(ctx, db.GetWorkflowParams{ID: wfID, WorkspaceID: workspaceID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, service.ErrNotFound
		}
		return nil, fmt.Errorf("get workflow: %w", err)
	}
	wf := toWorkflow(row)
	return &wf, nil
}

func (r *PGRepository) List(ctx context.Context, workspaceID string) ([]*service.Workflow, error) {
	q := db.New(r.pool)
	rows, err := q.ListWorkflows(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list workflows: %w", err)
	}
	out := make([]*service.Workflow, len(rows))
	for i, row := range rows {
		wf := toWorkflow(row)
		out[i] = &wf
	}
	return out, nil
}

func (r *PGRepository) Update(ctx context.Context, in service.UpdateInput) (*service.Workflow, error) {
	wfID, err := parseUUID(in.ID)
	if err != nil {
		return nil, service.ErrNotFound
	}
	q := db.New(r.pool)
	row, err := q.UpdateWorkflow(ctx, db.UpdateWorkflowParams{
		ID:          wfID,
		WorkspaceID: in.WorkspaceID,
		Name:        in.Name,
		Description: in.Description,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, service.ErrNotFound
		}
		return nil, fmt.Errorf("update workflow: %w", err)
	}
	wf := toWorkflow(row)
	return &wf, nil
}

func (r *PGRepository) Delete(ctx context.Context, workspaceID, id string) error {
	wfID, err := parseUUID(id)
	if err != nil {
		return service.ErrNotFound
	}
	q := db.New(r.pool)
	return q.DeleteWorkflow(ctx, db.DeleteWorkflowParams{ID: wfID, WorkspaceID: workspaceID})
}

// Publish marks the latest version as published and activates the workflow atomically.
func (r *PGRepository) Publish(ctx context.Context, workspaceID, id string) (*service.Workflow, *service.WorkflowVersion, error) {
	wfID, err := parseUUID(id)
	if err != nil {
		return nil, nil, service.ErrNotFound
	}

	var wf service.Workflow
	var ver service.WorkflowVersion

	err = pgx.BeginTxFunc(ctx, r.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		q := db.New(tx)

		wfRow, err := q.GetWorkflow(ctx, db.GetWorkflowParams{ID: wfID, WorkspaceID: workspaceID})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return service.ErrNotFound
			}
			return fmt.Errorf("get workflow: %w", err)
		}

		vRow, err := q.GetWorkflowVersionByNumber(ctx, db.GetWorkflowVersionByNumberParams{
			WorkflowID: wfID,
			Version:    wfRow.LatestVersion,
		})
		if err != nil {
			return fmt.Errorf("get latest version: %w", err)
		}

		published, err := q.PublishWorkflowVersion(ctx, db.PublishWorkflowVersionParams{
			ID:         vRow.ID,
			WorkflowID: wfID,
		})
		if err != nil {
			return fmt.Errorf("publish version: %w", err)
		}

		activated, err := q.SetWorkflowActive(ctx, db.SetWorkflowActiveParams{
			ID:          wfID,
			WorkspaceID: workspaceID,
			Active:      true,
		})
		if err != nil {
			return fmt.Errorf("activate workflow: %w", err)
		}

		wf = toWorkflow(activated)
		ver = toVersion(published)
		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	return &wf, &ver, nil
}

// IsActive returns true when the workflow exists and is active. Used by cron worker.
func (r *PGRepository) IsActive(ctx context.Context, workspaceID, workflowID string) (bool, error) {
	wfID, err := parseUUID(workflowID)
	if err != nil {
		return false, nil
	}
	q := db.New(r.pool)
	wf, err := q.GetWorkflow(ctx, db.GetWorkflowParams{ID: wfID, WorkspaceID: workspaceID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return wf.Active, nil
}

func (r *PGRepository) GetByWebhookSecret(ctx context.Context, workflowID, secret string) (*service.Workflow, error) {
	wfID, err := parseUUID(workflowID)
	if err != nil {
		return nil, service.ErrNotFound
	}
	q := db.New(r.pool)
	row, err := q.GetWorkflowByWebhookSecret(ctx, db.GetWorkflowByWebhookSecretParams{
		ID:            wfID,
		WebhookSecret: &secret,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, service.ErrNotFound
		}
		return nil, fmt.Errorf("get workflow by webhook: %w", err)
	}
	wf := toWorkflow(row)
	return &wf, nil
}

func (r *PGRepository) Deactivate(ctx context.Context, workspaceID, id string) (*service.Workflow, error) {
	wfID, err := parseUUID(id)
	if err != nil {
		return nil, service.ErrNotFound
	}
	q := db.New(r.pool)
	row, err := q.SetWorkflowActive(ctx, db.SetWorkflowActiveParams{
		ID:          wfID,
		WorkspaceID: workspaceID,
		Active:      false,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, service.ErrNotFound
		}
		return nil, fmt.Errorf("deactivate workflow: %w", err)
	}
	wf := toWorkflow(row)
	return &wf, nil
}

func (r *PGRepository) ListVersions(ctx context.Context, workspaceID, workflowID string) ([]*service.WorkflowVersion, error) {
	wfID, err := parseUUID(workflowID)
	if err != nil {
		return nil, service.ErrNotFound
	}
	// Verify workspace ownership before listing versions.
	q := db.New(r.pool)
	if _, err := q.GetWorkflow(ctx, db.GetWorkflowParams{ID: wfID, WorkspaceID: workspaceID}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, service.ErrNotFound
		}
		return nil, fmt.Errorf("get workflow: %w", err)
	}
	rows, err := q.ListWorkflowVersions(ctx, wfID)
	if err != nil {
		return nil, fmt.Errorf("list versions: %w", err)
	}
	out := make([]*service.WorkflowVersion, len(rows))
	for i, row := range rows {
		v := toVersion(row)
		out[i] = &v
	}
	return out, nil
}

// --- converters ---

func (r *PGRepository) SetWebhookSecret(ctx context.Context, workspaceID, id, secret string) (*service.Workflow, error) {
	wfID, err := parseUUID(id)
	if err != nil {
		return nil, service.ErrNotFound
	}
	q := db.New(r.pool)
	row, err := q.SetWebhookSecret(ctx, db.SetWebhookSecretParams{
		ID:            wfID,
		WorkspaceID:   workspaceID,
		WebhookSecret: &secret,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, service.ErrNotFound
		}
		return nil, fmt.Errorf("set webhook secret: %w", err)
	}
	wf := toWorkflow(row)
	return &wf, nil
}

func toWorkflow(r db.Workflow) service.Workflow {
	return service.Workflow{
		ID:            uuidStr(r.ID),
		WorkspaceID:   r.WorkspaceID,
		Name:          r.Name,
		Description:   r.Description,
		Active:        r.Active,
		LatestVersion: r.LatestVersion,
		WebhookSecret: r.WebhookSecret,
		CreatedAt:     tsTime(r.CreatedAt),
		UpdatedAt:     tsTime(r.UpdatedAt),
	}
}

func toVersion(r db.WorkflowVersion) service.WorkflowVersion {
	return service.WorkflowVersion{
		ID:         uuidStr(r.ID),
		WorkflowID: uuidStr(r.WorkflowID),
		Version:    r.Version,
		Definition: json.RawMessage(r.Definition),
		Published:  r.Published,
		CreatedAt:  tsTime(r.CreatedAt),
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
