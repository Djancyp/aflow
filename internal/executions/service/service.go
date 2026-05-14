package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/djan/aflow/internal/observability/metrics"
	"github.com/djan/aflow/internal/queue/jobs"
	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
)

var (
	ErrNotFound          = errors.New("execution not found")
	ErrWorkflowNotFound  = errors.New("workflow not found")
	ErrWorkflowInactive  = errors.New("workflow is not active")
	ErrNoPublishedVersion = errors.New("workflow has no published version")
	ErrCancelForbidden   = errors.New("execution cannot be cancelled in current state")
)

type ExecutionStatus string

const (
	StatusQueued    ExecutionStatus = "queued"
	StatusRunning   ExecutionStatus = "running"
	StatusSuccess   ExecutionStatus = "success"
	StatusFailed    ExecutionStatus = "failed"
	StatusCancelled ExecutionStatus = "cancelled"
)

type Execution struct {
	ID                string
	WorkspaceID       string
	WorkflowID        string
	WorkflowVersionID string
	Status            ExecutionStatus
	TriggerSource     *string
	StartedAt         *time.Time
	FinishedAt        *time.Time
	Input             json.RawMessage
	Output            json.RawMessage
	Error             *string
	CreatedAt         time.Time
}

type ExecutionLog struct {
	ID          int64
	ExecutionID string
	NodeID      *string
	Level       string
	Message     string
	Metadata    json.RawMessage
	CreatedAt   time.Time
}

type PrepareInput struct {
	WorkspaceID   string
	WorkflowID    string
	TriggerSource string
	Input         json.RawMessage
}

type WriteLogInput struct {
	ExecutionID string
	NodeID      *string
	Level       string
	Message     string
	Metadata    json.RawMessage
}

// Repository is the storage contract for execution operations.
type Repository interface {
	PrepareExecution(ctx context.Context, in PrepareInput) (*Execution, error)
	GetExecution(ctx context.Context, workspaceID, id string) (*Execution, error)
	GetExecutionInternal(ctx context.Context, id string) (*Execution, error)
	ListByWorkspace(ctx context.Context, workspaceID string, limit, offset int32) ([]*Execution, error)
	ListByWorkflow(ctx context.Context, workspaceID, workflowID string) ([]*Execution, error)
	CancelExecution(ctx context.Context, workspaceID, id string) (*Execution, error)
	GetLogs(ctx context.Context, executionID string) ([]*ExecutionLog, error)

	// Used by the runtime executor.
	StartExecution(ctx context.Context, id string) error
	FinishExecution(ctx context.Context, id string, status ExecutionStatus, output json.RawMessage, errMsg string) error
	WriteLog(ctx context.Context, in WriteLogInput) error
	GetVersionDefinition(ctx context.Context, versionID string) (json.RawMessage, error)
}

// Service implements execution business logic.
type Service struct {
	repo        Repository
	riverClient *river.Client[pgx.Tx]
}

func New(repo Repository, riverClient *river.Client[pgx.Tx]) *Service {
	return &Service{repo: repo, riverClient: riverClient}
}

func (s *Service) Execute(ctx context.Context, workspaceID, workflowID string, input json.RawMessage) (*Execution, error) {
	exec, err := s.repo.PrepareExecution(ctx, PrepareInput{
		WorkspaceID:   workspaceID,
		WorkflowID:    workflowID,
		TriggerSource: "api",
		Input:         input,
	})
	if err != nil {
		return nil, err
	}

	if _, err := s.riverClient.Insert(ctx, jobs.WorkflowExecuteArgs{
		ExecutionID: exec.ID,
	}, nil); err != nil {
		return nil, fmt.Errorf("enqueue execution: %w", err)
	}

	metrics.QueueJobsEnqueued.WithLabelValues("workflow.execute").Inc()
	return exec, nil
}

func (s *Service) Get(ctx context.Context, workspaceID, id string) (*Execution, error) {
	return s.repo.GetExecution(ctx, workspaceID, id)
}

func (s *Service) ListByWorkspace(ctx context.Context, workspaceID string, limit, offset int32) ([]*Execution, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	return s.repo.ListByWorkspace(ctx, workspaceID, limit, offset)
}

func (s *Service) ListByWorkflow(ctx context.Context, workspaceID, workflowID string) ([]*Execution, error) {
	return s.repo.ListByWorkflow(ctx, workspaceID, workflowID)
}

// ExecuteWithTrigger creates and enqueues an execution with an explicit trigger source.
// Used by webhook and cron subsystems.
func (s *Service) ExecuteWithTrigger(ctx context.Context, workspaceID, workflowID, trigger string, input json.RawMessage) (*Execution, error) {
	exec, err := s.repo.PrepareExecution(ctx, PrepareInput{
		WorkspaceID:   workspaceID,
		WorkflowID:    workflowID,
		TriggerSource: trigger,
		Input:         input,
	})
	if err != nil {
		return nil, err
	}
	if _, err := s.riverClient.Insert(ctx, jobs.WorkflowExecuteArgs{ExecutionID: exec.ID}, nil); err != nil {
		return nil, fmt.Errorf("enqueue execution: %w", err)
	}
	metrics.QueueJobsEnqueued.WithLabelValues("workflow.execute").Inc()
	return exec, nil
}

func (s *Service) Cancel(ctx context.Context, workspaceID, id string) (*Execution, error) {
	exec, err := s.repo.CancelExecution(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}
	return exec, nil
}

func (s *Service) Retry(ctx context.Context, workspaceID, id string) (*Execution, error) {
	orig, err := s.repo.GetExecution(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}

	newExec, err := s.repo.PrepareExecution(ctx, PrepareInput{
		WorkspaceID:   workspaceID,
		WorkflowID:    orig.WorkflowID,
		TriggerSource: "retry",
		Input:         orig.Input,
	})
	if err != nil {
		return nil, err
	}

	if _, err := s.riverClient.Insert(ctx, jobs.WorkflowExecuteArgs{
		ExecutionID: newExec.ID,
	}, nil); err != nil {
		return nil, fmt.Errorf("enqueue retry: %w", err)
	}
	metrics.QueueJobsEnqueued.WithLabelValues("workflow.execute").Inc()

	return newExec, nil
}

func (s *Service) GetLogs(ctx context.Context, workspaceID, executionID string) ([]*ExecutionLog, error) {
	// Verify ownership first.
	if _, err := s.repo.GetExecution(ctx, workspaceID, executionID); err != nil {
		return nil, err
	}
	return s.repo.GetLogs(ctx, executionID)
}
