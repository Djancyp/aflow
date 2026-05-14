package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/djan/aflow/internal/runtime/engine"
)

var (
	ErrNotFound  = errors.New("workflow not found")
	ErrForbidden = errors.New("workflow belongs to different workspace")
)

// Workflow is the domain representation of a workflow.
type Workflow struct {
	ID            string
	WorkspaceID   string
	Name          string
	Description   *string
	Active        bool
	LatestVersion int32
	WebhookSecret *string // set when a trigger.webhook node is published
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// WorkflowVersion is the domain representation of an immutable workflow version.
type WorkflowVersion struct {
	ID         string
	WorkflowID string
	Version    int32
	Definition json.RawMessage
	Published  bool
	CreatedAt  time.Time
}

// CreateInput carries data for workflow creation.
type CreateInput struct {
	WorkspaceID string
	Name        string
	Description *string
	Definition  json.RawMessage
}

// UpdateInput carries data for a metadata update (name/description only).
type UpdateInput struct {
	WorkspaceID string
	ID          string
	Name        string
	Description *string
}

// TriggerScheduler handles async trigger activation after a workflow is published.
// Implementations are in internal/runtime/scheduler.
type TriggerScheduler interface {
	ScheduleCron(ctx context.Context, workspaceID, workflowID, schedule string) error
}

// Repository is the storage contract the service depends on.
type Repository interface {
	// CreateWithVersion creates a workflow and its first version atomically.
	CreateWithVersion(ctx context.Context, in CreateInput) (*Workflow, *WorkflowVersion, error)

	GetByID(ctx context.Context, workspaceID, id string) (*Workflow, error)
	List(ctx context.Context, workspaceID string) ([]*Workflow, error)
	Update(ctx context.Context, in UpdateInput) (*Workflow, error)
	Delete(ctx context.Context, workspaceID, id string) error

	// Publish marks the latest version as published and activates the workflow.
	Publish(ctx context.Context, workspaceID, id string) (*Workflow, *WorkflowVersion, error)

	Deactivate(ctx context.Context, workspaceID, id string) (*Workflow, error)

	// SetWebhookSecret writes a generated webhook secret onto the workflow record.
	SetWebhookSecret(ctx context.Context, workspaceID, id, secret string) (*Workflow, error)

	// GetByWebhookSecret looks up an active workflow by ID and webhook secret.
	GetByWebhookSecret(ctx context.Context, workflowID, secret string) (*Workflow, error)

	ListVersions(ctx context.Context, workspaceID, workflowID string) ([]*WorkflowVersion, error)
}

// Service implements workflow business logic.
type Service struct {
	repo      Repository
	scheduler TriggerScheduler // may be nil
}

func New(repo Repository) *Service {
	return &Service{repo: repo}
}

// WithScheduler attaches a trigger scheduler (used for cron activation on publish).
func (s *Service) WithScheduler(ts TriggerScheduler) *Service {
	s.scheduler = ts
	return s
}

func (s *Service) Create(ctx context.Context, in CreateInput) (*Workflow, *WorkflowVersion, error) {
	if in.Name == "" {
		return nil, nil, errors.New("name is required")
	}
	if len(in.Definition) == 0 {
		return nil, nil, errors.New("definition is required")
	}
	return s.repo.CreateWithVersion(ctx, in)
}

func (s *Service) Get(ctx context.Context, workspaceID, id string) (*Workflow, error) {
	return s.repo.GetByID(ctx, workspaceID, id)
}

func (s *Service) List(ctx context.Context, workspaceID string) ([]*Workflow, error) {
	return s.repo.List(ctx, workspaceID)
}

func (s *Service) Update(ctx context.Context, in UpdateInput) (*Workflow, error) {
	if in.Name == "" {
		return nil, errors.New("name is required")
	}
	return s.repo.Update(ctx, in)
}

func (s *Service) Delete(ctx context.Context, workspaceID, id string) error {
	return s.repo.Delete(ctx, workspaceID, id)
}

// Publish validates the definition, publishes the workflow, and activates its trigger.
func (s *Service) Publish(ctx context.Context, workspaceID, id string) (*Workflow, *WorkflowVersion, error) {
	wf, err := s.repo.GetByID(ctx, workspaceID, id)
	if err != nil {
		return nil, nil, err
	}

	versions, err := s.repo.ListVersions(ctx, workspaceID, wf.ID)
	if err != nil {
		return nil, nil, err
	}
	if len(versions) == 0 {
		return nil, nil, errors.New("workflow has no versions to publish")
	}

	latest := versions[0]
	def, err := engine.ParseDefinition(latest.Definition)
	if err != nil {
		return nil, nil, errors.New("workflow definition is invalid JSON: " + err.Error())
	}
	if _, err := engine.TopologicalSort(def); err != nil {
		return nil, nil, errors.New("workflow definition has invalid DAG: " + err.Error())
	}

	publishedWf, publishedVer, err := s.repo.Publish(ctx, workspaceID, id)
	if err != nil {
		return nil, nil, err
	}

	// Activate triggers.
	triggerNode := engine.FindTriggerNode(def)
	if triggerNode != nil {
		switch triggerNode.Type {

		case "trigger.webhook":
			// Generate a fresh webhook secret if not already set.
			if publishedWf.WebhookSecret == nil || *publishedWf.WebhookSecret == "" {
				secret := generateSecret()
				updated, err := s.repo.SetWebhookSecret(ctx, workspaceID, id, secret)
				if err != nil {
					slog.Error("failed to set webhook secret", "workflow_id", id, "err", err)
				} else {
					publishedWf = updated
				}
			}

		case "trigger.cron":
			schedule, _ := triggerNode.Config["schedule"].(string)
			if schedule != "" && s.scheduler != nil {
				if err := s.scheduler.ScheduleCron(ctx, publishedWf.WorkspaceID, publishedWf.ID, schedule); err != nil {
					slog.Error("failed to schedule cron", "workflow_id", id, "err", err)
				}
			}
		}
	}

	return publishedWf, publishedVer, nil
}

func (s *Service) Deactivate(ctx context.Context, workspaceID, id string) (*Workflow, error) {
	return s.repo.Deactivate(ctx, workspaceID, id)
}

func (s *Service) GetByWebhookSecret(ctx context.Context, workflowID, secret string) (*Workflow, error) {
	return s.repo.GetByWebhookSecret(ctx, workflowID, secret)
}

func (s *Service) ListVersions(ctx context.Context, workspaceID, workflowID string) ([]*WorkflowVersion, error) {
	return s.repo.ListVersions(ctx, workspaceID, workflowID)
}

func generateSecret() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return "whsec_" + hex.EncodeToString(b)
}
