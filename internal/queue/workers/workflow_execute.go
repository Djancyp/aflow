package workers

import (
	"context"
	"log/slog"

	"github.com/djan/aflow/internal/queue/jobs"
	"github.com/djan/aflow/internal/runtime/executor"
	"github.com/riverqueue/river"
)

// WorkflowExecuteWorker processes workflow.execute jobs.
type WorkflowExecuteWorker struct {
	river.WorkerDefaults[jobs.WorkflowExecuteArgs]
	executor *executor.Executor
}

func NewWorkflowExecuteWorker(exec *executor.Executor) *WorkflowExecuteWorker {
	return &WorkflowExecuteWorker{executor: exec}
}

func (w *WorkflowExecuteWorker) Work(ctx context.Context, job *river.Job[jobs.WorkflowExecuteArgs]) error {
	slog.Info("executing workflow", "execution_id", job.Args.ExecutionID, "attempt", job.Attempt)

	if err := w.executor.RunExecution(ctx, job.Args.ExecutionID); err != nil {
		slog.Error("execution failed", "execution_id", job.Args.ExecutionID, "err", err)
		return err
	}

	slog.Info("execution completed", "execution_id", job.Args.ExecutionID)
	return nil
}
