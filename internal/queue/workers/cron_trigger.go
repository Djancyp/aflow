package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/djan/aflow/internal/queue/jobs"
	"github.com/riverqueue/river"
)

// CronExecutor is the minimal interface the cron worker needs.
type CronExecutor interface {
	ExecuteWithTrigger(ctx context.Context, workspaceID, workflowID, trigger string, input json.RawMessage) (any, error)
}

// CronRescheduler inserts the next cron job after this one fires.
type CronRescheduler interface {
	ScheduleCron(ctx context.Context, workspaceID, workflowID, schedule string) error
}

// CronWorkflowChecker verifies the workflow is still active before executing.
type CronWorkflowChecker interface {
	IsActive(ctx context.Context, workspaceID, workflowID string) (bool, error)
}

// CronTriggerWorker processes cron.trigger River jobs.
type CronTriggerWorker struct {
	river.WorkerDefaults[jobs.CronTriggerArgs]
	executor    CronExecutor
	rescheduler CronRescheduler
	checker     CronWorkflowChecker
}

func NewCronTriggerWorker(exec CronExecutor, resched CronRescheduler, checker CronWorkflowChecker) *CronTriggerWorker {
	return &CronTriggerWorker{
		executor:    exec,
		rescheduler: resched,
		checker:     checker,
	}
}

func (w *CronTriggerWorker) Work(ctx context.Context, job *river.Job[jobs.CronTriggerArgs]) error {
	args := job.Args

	// Skip if workflow was deactivated since this job was scheduled.
	active, err := w.checker.IsActive(ctx, args.WorkspaceID, args.WorkflowID)
	if err != nil || !active {
		slog.Info("cron trigger skipped: workflow inactive", "workflow_id", args.WorkflowID)
		return nil // don't error; just stop the chain
	}

	// Create a new execution.
	if _, err := w.executor.ExecuteWithTrigger(ctx, args.WorkspaceID, args.WorkflowID, "cron", json.RawMessage("{}")); err != nil {
		return fmt.Errorf("cron trigger execution: %w", err)
	}

	// Schedule the next fire before returning.
	if err := w.rescheduler.ScheduleCron(ctx, args.WorkspaceID, args.WorkflowID, args.Schedule); err != nil {
		slog.Error("cron reschedule failed", "workflow_id", args.WorkflowID, "err", err)
		// Non-fatal: log but don't fail the current job.
	}

	slog.Info("cron triggered execution", "workflow_id", args.WorkflowID, "schedule", args.Schedule)
	return nil
}
