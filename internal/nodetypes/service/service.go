package service

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/djan/aflow/internal/nodes/registry"
)

var (
	ErrNotFound = errors.New("node type not found")
	ErrConflict = errors.New("node type name already exists in this workspace")
)

// NodeType represents a custom HTTP-action node definition stored in the DB.
type NodeType struct {
	ID              string
	WorkspaceID     string
	Name            string
	Description     *string
	Category        string
	Version         string
	BaseURL         string
	Endpoint        string
	Method          string
	ContentType     string
	HeadersTemplate map[string]any
	BodyTemplate    *string
	InputSchema     map[string]any
	OutputSchema    map[string]any
	CredentialID    *string
	Active          bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// CatalogEntry is the unified view of a node type (built-in or custom).
type CatalogEntry struct {
	ID           string         `json:"id"`
	Name         string         `json:"name"`
	Description  string         `json:"description,omitempty"`
	Category     string         `json:"category"`
	Version      string         `json:"version"`
	Kind         string         `json:"kind"` // "builtin" | "http-action"
	InputSchema  map[string]any `json:"input_schema,omitempty"`
	OutputSchema map[string]any `json:"output_schema,omitempty"`
}

type CreateInput struct {
	WorkspaceID     string
	Name            string
	Description     *string
	Category        string
	Version         string
	BaseURL         string
	Endpoint        string
	Method          string
	ContentType     string
	HeadersTemplate map[string]any
	BodyTemplate    *string
	InputSchema     map[string]any
	OutputSchema    map[string]any
	CredentialID    *string
}

type UpdateInput struct {
	ID          string
	WorkspaceID string
	CreateInput               // reuse same fields
}

// Repository is the storage contract.
type Repository interface {
	Create(ctx context.Context, in CreateInput) (*NodeType, error)
	Get(ctx context.Context, workspaceID, id string) (*NodeType, error)
	List(ctx context.Context, workspaceID, q, category string) ([]*NodeType, error)
	Update(ctx context.Context, in UpdateInput) (*NodeType, error)
	Delete(ctx context.Context, workspaceID, id string) error
}

// Service implements node type business logic and the unified catalog.
type Service struct {
	repo     Repository
	registry *registry.Registry
}

func New(repo Repository, reg *registry.Registry) *Service {
	return &Service{repo: repo, registry: reg}
}

func (s *Service) Create(ctx context.Context, in CreateInput) (*NodeType, error) {
	if in.Name == "" {
		return nil, errors.New("name is required")
	}
	if in.BaseURL == "" {
		return nil, errors.New("base_url is required")
	}
	if in.Method == "" {
		in.Method = "POST"
	}
	if in.ContentType == "" {
		in.ContentType = "application/json"
	}
	if in.Endpoint == "" {
		in.Endpoint = "/"
	}
	if in.Version == "" {
		in.Version = "1.0.0"
	}
	if in.InputSchema == nil {
		in.InputSchema = map[string]any{"type": "object"}
	}
	return s.repo.Create(ctx, in)
}

func (s *Service) Get(ctx context.Context, workspaceID, id string) (*NodeType, error) {
	return s.repo.Get(ctx, workspaceID, id)
}

func (s *Service) Update(ctx context.Context, in UpdateInput) (*NodeType, error) {
	if in.Name == "" {
		return nil, errors.New("name is required")
	}
	return s.repo.Update(ctx, in)
}

func (s *Service) Delete(ctx context.Context, workspaceID, id string) error {
	return s.repo.Delete(ctx, workspaceID, id)
}

// Catalog returns built-in nodes + workspace custom nodes in a unified format.
// q filters by name/description; category filters by category.
func (s *Service) Catalog(ctx context.Context, workspaceID, q, category string) ([]*CatalogEntry, error) {
	var entries []*CatalogEntry

	// Built-in nodes from registry.
	for _, meta := range s.registry.List() {
		if q != "" && !containsInsensitive(meta.Name, q) && !containsInsensitive(meta.Description, q) {
			continue
		}
		if category != "" && meta.Category != category {
			continue
		}
		e := &CatalogEntry{
			ID:           meta.Type,
			Name:         meta.Name,
			Description:  meta.Description,
			Category:     meta.Category,
			Version:      meta.Version,
			Kind:         "builtin",
			InputSchema:  meta.InputSchema,
			OutputSchema: meta.OutputSchema,
		}
		entries = append(entries, e)
	}

	// Custom HTTP-action nodes from DB.
	custom, err := s.repo.List(ctx, workspaceID, q, category)
	if err != nil {
		return nil, err
	}
	for _, nt := range custom {
		desc := ""
		if nt.Description != nil {
			desc = *nt.Description
		}
		e := &CatalogEntry{
			ID:           nt.ID,
			Name:         nt.Name,
			Description:  desc,
			Category:     nt.Category,
			Version:      nt.Version,
			Kind:         "http-action",
			InputSchema:  nt.InputSchema,
			OutputSchema: nt.OutputSchema,
		}
		entries = append(entries, e)
	}

	return entries, nil
}

func containsInsensitive(s, sub string) bool {
	if sub == "" {
		return true
	}
	sl := []byte(s)
	subl := []byte(sub)
	// simple case-fold: compare byte by byte with tolower
	for i := 0; i <= len(sl)-len(subl); i++ {
		match := true
		for j := range subl {
			if toLower(sl[i+j]) != toLower(subl[j]) {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

func toLower(b byte) byte {
	if b >= 'A' && b <= 'Z' {
		return b + 32
	}
	return b
}

// SchemaBytes marshals a map to JSON bytes for DB storage.
func SchemaBytes(m map[string]any) []byte {
	if m == nil {
		return nil
	}
	b, _ := json.Marshal(m)
	return b
}
