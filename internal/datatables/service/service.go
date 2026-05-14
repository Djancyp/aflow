package service

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

var (
	ErrNotFound    = errors.New("table not found")
	ErrRowNotFound = errors.New("row not found")
)

type Table struct {
	ID          string
	WorkspaceID string
	Name        string
	Schema      json.RawMessage
	CreatedAt   time.Time
}

type Row struct {
	ID          string
	TableID     string
	WorkspaceID string
	Data        json.RawMessage
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type CreateTableInput struct {
	WorkspaceID string
	Name        string
	Schema      json.RawMessage
}

type InsertRowInput struct {
	WorkspaceID string
	TableID     string
	Data        json.RawMessage
}

// Repository is the storage contract for data tables.
type Repository interface {
	CreateTable(ctx context.Context, in CreateTableInput) (*Table, error)
	GetTable(ctx context.Context, workspaceID, id string) (*Table, error)
	ListTables(ctx context.Context, workspaceID string) ([]*Table, error)
	DeleteTable(ctx context.Context, workspaceID, id string) error
	InsertRow(ctx context.Context, in InsertRowInput) (*Row, error)
	ListRows(ctx context.Context, workspaceID, tableID string) ([]*Row, error)
	DeleteRow(ctx context.Context, workspaceID, tableID, rowID string) error
}

// Service implements data table business logic.
type Service struct {
	repo Repository
}

func New(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) CreateTable(ctx context.Context, in CreateTableInput) (*Table, error) {
	if in.Name == "" {
		return nil, errors.New("name is required")
	}
	if len(in.Schema) == 0 {
		in.Schema = json.RawMessage(`{"columns":[]}`)
	}
	return s.repo.CreateTable(ctx, in)
}

func (s *Service) GetTable(ctx context.Context, workspaceID, id string) (*Table, error) {
	return s.repo.GetTable(ctx, workspaceID, id)
}

func (s *Service) ListTables(ctx context.Context, workspaceID string) ([]*Table, error) {
	return s.repo.ListTables(ctx, workspaceID)
}

func (s *Service) DeleteTable(ctx context.Context, workspaceID, id string) error {
	return s.repo.DeleteTable(ctx, workspaceID, id)
}

func (s *Service) InsertRow(ctx context.Context, workspaceID, tableID string, data json.RawMessage) (*Row, error) {
	// Verify table exists and belongs to workspace.
	if _, err := s.repo.GetTable(ctx, workspaceID, tableID); err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, errors.New("row data is required")
	}
	return s.repo.InsertRow(ctx, InsertRowInput{
		WorkspaceID: workspaceID,
		TableID:     tableID,
		Data:        data,
	})
}

func (s *Service) ListRows(ctx context.Context, workspaceID, tableID string) ([]*Row, error) {
	if _, err := s.repo.GetTable(ctx, workspaceID, tableID); err != nil {
		return nil, err
	}
	return s.repo.ListRows(ctx, workspaceID, tableID)
}

func (s *Service) DeleteRow(ctx context.Context, workspaceID, tableID, rowID string) error {
	if _, err := s.repo.GetTable(ctx, workspaceID, tableID); err != nil {
		return err
	}
	return s.repo.DeleteRow(ctx, workspaceID, tableID, rowID)
}
