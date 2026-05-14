// Package httpaction executes custom DB-backed HTTP-action node types.
// These are workspace-scoped node definitions stored in the node_types table.
package httpaction

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/djan/aflow/database/sqlc"
	"github.com/djan/aflow/internal/runtime/template"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

var httpClient = &http.Client{Timeout: 30 * time.Second}

// CredentialDecryptor resolves encrypted credentials by ID.
type CredentialDecryptor interface {
	Decrypt(ctx context.Context, workspaceID, credentialID string) (json.RawMessage, error)
}

// Executor runs custom HTTP-action nodes stored in the node_types table.
type Executor struct {
	pool      *pgxpool.Pool
	credDecry CredentialDecryptor
}

func New(pool *pgxpool.Pool, creds CredentialDecryptor) *Executor {
	return &Executor{pool: pool, credDecry: creds}
}

// Execute runs the HTTP-action node identified by nodeTypeID against the given input.
func (e *Executor) Execute(ctx context.Context, workspaceID, nodeTypeID string, config map[string]any, input any) (any, error) {
	nt, err := e.loadNodeType(ctx, nodeTypeID)
	if err != nil {
		return nil, fmt.Errorf("http-action %s: %w", nodeTypeID, err)
	}

	// Build namespace for template resolution.
	ns := template.Namespaces{
		"input": input,
	}

	// Decrypt bound credential if present.
	if nt.CredentialID.Valid {
		credID := uuid.UUID(nt.CredentialID.Bytes).String()
		if e.credDecry != nil {
			raw, err := e.credDecry.Decrypt(ctx, workspaceID, credID)
			if err != nil {
				return nil, fmt.Errorf("http-action %s: decrypt credential: %w", nodeTypeID, err)
			}
			var credMap map[string]any
			_ = json.Unmarshal(raw, &credMap)
			ns["credential"] = credMap
		}
	}

	// Also allow config-level overrides to be part of the input namespace.
	if len(config) > 0 {
		merged := make(map[string]any)
		if inputMap, ok := input.(map[string]any); ok {
			for k, v := range inputMap {
				merged[k] = v
			}
		}
		for k, v := range config {
			merged[k] = v
		}
		ns["input"] = merged
	}

	// Resolve URL.
	rawURL := template.Resolve(nt.BaseUrl+nt.Endpoint, ns)

	// Resolve headers.
	var headers map[string]string
	if len(nt.HeadersTemplate) > 0 {
		var headersTmpl map[string]any
		if err := json.Unmarshal(nt.HeadersTemplate, &headersTmpl); err == nil {
			headers = template.ResolveHeaders(headersTmpl, ns)
		}
	}

	// Resolve body.
	var bodyReader io.Reader
	if nt.BodyTemplate != nil && *nt.BodyTemplate != "" {
		resolved := template.ResolveBody(*nt.BodyTemplate, ns)
		bodyReader = strings.NewReader(resolved)
	}

	req, err := http.NewRequestWithContext(ctx, strings.ToUpper(nt.Method), rawURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("http-action %s: build request: %w", nodeTypeID, err)
	}

	req.Header.Set("Content-Type", nt.ContentType)
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http-action %s: %w", nodeTypeID, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return nil, fmt.Errorf("http-action %s: read response: %w", nodeTypeID, err)
	}

	var parsed any
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		parsed = string(respBody)
	}

	respHeaders := make(map[string]string, len(resp.Header))
	for k := range resp.Header {
		respHeaders[k] = resp.Header.Get(k)
	}

	return map[string]any{
		"status_code": resp.StatusCode,
		"headers":     respHeaders,
		"body":        parsed,
	}, nil
}

func (e *Executor) loadNodeType(ctx context.Context, id string) (*db.NodeType, error) {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("invalid node type ID %q", id)
	}
	q := db.New(e.pool)
	nt, err := q.GetNodeTypeByID(ctx, pgtype.UUID{Bytes: parsed, Valid: true})
	if err != nil {
		return nil, fmt.Errorf("node type not found: %w", err)
	}
	return &nt, nil
}
