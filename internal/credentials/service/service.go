package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/djan/aflow/pkg/crypto"
)

var ErrNotFound = errors.New("credential not found")

// Credential holds the full DB row including encrypted bytes (internal only).
type Credential struct {
	ID            string
	WorkspaceID   string
	Name          string
	Type          string
	EncryptedData []byte
	CreatedAt     time.Time
}

// Meta is the safe public projection — never includes raw or encrypted data.
type Meta struct {
	ID          string    `json:"id"`
	WorkspaceID string    `json:"workspace_id"`
	Name        string    `json:"name"`
	Type        string    `json:"type"`
	CreatedAt   time.Time `json:"created_at"`
}

type CreateInput struct {
	WorkspaceID string
	Name        string
	Type        string
	Data        json.RawMessage // plaintext data, encrypted before storage
}

// Repository is the storage contract for credentials.
type Repository interface {
	Create(ctx context.Context, in CreateInput, encryptedData []byte) (*Credential, error)
	Get(ctx context.Context, workspaceID, id string) (*Credential, error)
	List(ctx context.Context, workspaceID string) ([]*Credential, error)
	Delete(ctx context.Context, workspaceID, id string) error
}

// Service implements credential business logic.
type Service struct {
	repo      Repository
	encryptor *crypto.Encryptor
}

func New(repo Repository, encryptor *crypto.Encryptor) *Service {
	return &Service{repo: repo, encryptor: encryptor}
}

func (s *Service) Create(ctx context.Context, in CreateInput) (*Meta, error) {
	if in.Name == "" {
		return nil, errors.New("name is required")
	}
	if in.Type == "" {
		return nil, errors.New("type is required")
	}
	if len(in.Data) == 0 {
		return nil, errors.New("data is required")
	}

	encrypted, err := s.encryptor.Encrypt([]byte(in.Data))
	if err != nil {
		return nil, fmt.Errorf("encrypt credential: %w", err)
	}

	cred, err := s.repo.Create(ctx, in, encrypted)
	if err != nil {
		return nil, err
	}
	return toMeta(cred), nil
}

func (s *Service) Get(ctx context.Context, workspaceID, id string) (*Meta, error) {
	cred, err := s.repo.Get(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}
	return toMeta(cred), nil
}

func (s *Service) List(ctx context.Context, workspaceID string) ([]*Meta, error) {
	creds, err := s.repo.List(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	out := make([]*Meta, len(creds))
	for i, c := range creds {
		out[i] = toMeta(c)
	}
	return out, nil
}

func (s *Service) Delete(ctx context.Context, workspaceID, id string) error {
	return s.repo.Delete(ctx, workspaceID, id)
}

// Decrypt is for internal runtime use only — never called by HTTP handlers.
func (s *Service) Decrypt(ctx context.Context, workspaceID, id string) (json.RawMessage, error) {
	cred, err := s.repo.Get(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}
	plaintext, err := s.encryptor.Decrypt(cred.EncryptedData)
	if err != nil {
		return nil, fmt.Errorf("decrypt credential %s: %w", id, err)
	}
	return json.RawMessage(plaintext), nil
}

func toMeta(c *Credential) *Meta {
	return &Meta{
		ID:          c.ID,
		WorkspaceID: c.WorkspaceID,
		Name:        c.Name,
		Type:        c.Type,
		CreatedAt:   c.CreatedAt,
	}
}
