// Package server implements a minimal MCP server over JSON-RPC 2.0.
// Spec: https://spec.modelcontextprotocol.io
package server

import (
	"context"
	"encoding/json"
	"fmt"
)

const protocolVersion = "2024-11-05"

// --- JSON-RPC 2.0 wire types ---

type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// --- MCP tool types ---

// ToolResult is what a tool handler returns; content is sent back to the caller.
type ToolResult struct {
	Content []ContentBlock `json:"content"`
	IsError bool           `json:"isError,omitempty"`
}

type ContentBlock struct {
	Type string `json:"type"` // "text"
	Text string `json:"text"`
}

// ToolHandler executes a tool in the context of a workspace.
type ToolHandler func(ctx context.Context, workspaceID string, args map[string]any) (*ToolResult, error)

// Tool describes a single MCP tool.
type Tool struct {
	Name        string
	Description string
	InputSchema map[string]any
	Handler     ToolHandler
}

// --- Server ---

// Server is a stateless MCP JSON-RPC dispatcher.
type Server struct {
	name    string
	version string
	tools   map[string]*Tool
	ordered []*Tool // preserves registration order for tools/list
}

func New(name, version string) *Server {
	return &Server{
		name:    name,
		version: version,
		tools:   make(map[string]*Tool),
	}
}

func (s *Server) Register(t *Tool) {
	s.tools[t.Name] = t
	s.ordered = append(s.ordered, t)
}

// Dispatch handles one JSON-RPC request and returns a response.
func (s *Server) Dispatch(ctx context.Context, workspaceID string, req Request) Response {
	base := Response{JSONRPC: "2.0", ID: req.ID}

	switch req.Method {
	case "initialize":
		base.Result = s.handleInitialize()
	case "notifications/initialized":
		// No-op notification — client sends after initialize, no response needed.
		return Response{} // empty response signals "drop this"
	case "ping":
		base.Result = map[string]any{}
	case "tools/list":
		base.Result = s.handleToolsList()
	case "tools/call":
		result, err := s.handleToolsCall(ctx, workspaceID, req.Params)
		if err != nil {
			base.Error = &RPCError{Code: -32603, Message: err.Error()}
		} else {
			base.Result = result
		}
	default:
		base.Error = &RPCError{Code: -32601, Message: fmt.Sprintf("method not found: %s", req.Method)}
	}

	return base
}

func (s *Server) handleInitialize() any {
	return map[string]any{
		"protocolVersion": protocolVersion,
		"capabilities":    map[string]any{"tools": map[string]any{}},
		"serverInfo":      map[string]any{"name": s.name, "version": s.version},
	}
}

func (s *Server) handleToolsList() any {
	list := make([]map[string]any, 0, len(s.ordered))
	for _, t := range s.ordered {
		list = append(list, map[string]any{
			"name":        t.Name,
			"description": t.Description,
			"inputSchema": t.InputSchema,
		})
	}
	return map[string]any{"tools": list}
}

type callParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

func (s *Server) handleToolsCall(ctx context.Context, workspaceID string, raw json.RawMessage) (*ToolResult, error) {
	var p callParams
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	tool, ok := s.tools[p.Name]
	if !ok {
		return nil, fmt.Errorf("unknown tool: %s", p.Name)
	}

	if p.Arguments == nil {
		p.Arguments = map[string]any{}
	}

	result, err := tool.Handler(ctx, workspaceID, p.Arguments)
	if err != nil {
		return &ToolResult{
			IsError: true,
			Content: []ContentBlock{{Type: "text", Text: err.Error()}},
		}, nil
	}
	return result, nil
}

// Text is a convenience constructor for a single-text ToolResult.
func Text(s string) *ToolResult {
	return &ToolResult{Content: []ContentBlock{{Type: "text", Text: s}}}
}
