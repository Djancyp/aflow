package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	mcpserver "github.com/djan/aflow/internal/mcp/server"
	ntsvc "github.com/djan/aflow/internal/nodetypes/service"
)

// listNodeTypes returns the unified node type catalog (built-in + custom).
func listNodeTypes(svc *ntsvc.Service) *mcpserver.Tool {
	return &mcpserver.Tool{
		Name:        "list_node_types",
		Description: "List all available node types (built-in and custom HTTP-action nodes). Use this to discover what nodes can be used in workflow definitions. Supports optional search and category filtering.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"q":        map[string]any{"type": "string", "description": "Search term (matches name or description)"},
				"category": map[string]any{"type": "string", "description": "Filter by category: trigger, core, logic, utility, communication, ai, data, custom"},
			},
		},
		Handler: func(ctx context.Context, workspaceID string, args map[string]any) (*mcpserver.ToolResult, error) {
			q, _ := args["q"].(string)
			category, _ := args["category"].(string)

			entries, err := svc.Catalog(ctx, workspaceID, q, category)
			if err != nil {
				return nil, fmt.Errorf("catalog: %w", err)
			}
			if len(entries) == 0 {
				return mcpserver.Text("No node types found matching your criteria."), nil
			}

			var sb strings.Builder
			fmt.Fprintf(&sb, "%d node type(s) available:\n\n", len(entries))
			for _, e := range entries {
				paramHint := ""
				if e.InputSchema != nil {
					if props, ok := e.InputSchema["properties"].(map[string]any); ok && len(props) > 0 {
						keys := make([]string, 0, len(props))
						for k := range props {
							keys = append(keys, k)
						}
						paramHint = " [config: " + strings.Join(keys, ", ") + "]"
					}
				}
				fmt.Fprintf(&sb, "• %s  [%s / %s]\n  id: %s\n  %s%s\n\n",
					e.Name, e.Category, e.Kind, e.ID, e.Description, paramHint)
			}
			return mcpserver.Text(sb.String()), nil
		},
	}
}

// createNodeType creates a new custom HTTP-action node type via MCP.
func createNodeType(svc *ntsvc.Service) *mcpserver.Tool {
	return &mcpserver.Tool{
		Name:        "create_node_type",
		Description: "Create a new custom HTTP-action node type in this workspace. Once created, use its ID as the 'type' field in workflow node definitions.",
		InputSchema: map[string]any{
			"type":     "object",
			"required": []string{"name", "base_url"},
			"properties": map[string]any{
				"name":             map[string]any{"type": "string", "description": "Human-readable node name"},
				"description":      map[string]any{"type": "string"},
				"category":         map[string]any{"type": "string", "description": "Category: communication, ai, data, utility, custom", "default": "custom"},
				"base_url":         map[string]any{"type": "string", "description": "Base URL of the target service"},
				"endpoint":         map[string]any{"type": "string", "description": "Path appended to base_url", "default": "/"},
				"method":           map[string]any{"type": "string", "enum": []string{"GET", "POST", "PUT", "PATCH", "DELETE"}, "default": "POST"},
				"content_type":     map[string]any{"type": "string", "default": "application/json"},
				"headers_template": map[string]any{"type": "object", "description": "Headers with {{credential.field}} or {{input.field}} templates"},
				"body_template":    map[string]any{"type": "string", "description": "JSON body string with {{input.field}} templates"},
				"input_schema":     map[string]any{"type": "object", "description": "JSON Schema draft-07 describing required config keys"},
				"output_schema":    map[string]any{"type": "object", "description": "JSON Schema describing node output shape"},
				"credential_id":    map[string]any{"type": "string", "description": "UUID of credential to decrypt at runtime"},
			},
		},
		Handler: func(ctx context.Context, workspaceID string, args map[string]any) (*mcpserver.ToolResult, error) {
			in := ntsvc.CreateInput{
				WorkspaceID: workspaceID,
				Name:        str(args, "name"),
				Category:    strDefault(args, "category", "custom"),
				BaseURL:     str(args, "base_url"),
				Endpoint:    strDefault(args, "endpoint", "/"),
				Method:      strDefault(args, "method", "POST"),
				ContentType: strDefault(args, "content_type", "application/json"),
			}
			if d := str(args, "description"); d != "" {
				in.Description = &d
			}
			if bt := str(args, "body_template"); bt != "" {
				in.BodyTemplate = &bt
			}
			if ci := str(args, "credential_id"); ci != "" {
				in.CredentialID = &ci
			}
			if m, ok := args["headers_template"].(map[string]any); ok {
				in.HeadersTemplate = m
			}
			if m, ok := args["input_schema"].(map[string]any); ok {
				in.InputSchema = m
			}
			if m, ok := args["output_schema"].(map[string]any); ok {
				in.OutputSchema = m
			}

			nt, err := svc.Create(ctx, in)
			if err != nil {
				return nil, fmt.Errorf("create node type: %w", err)
			}
			text := fmt.Sprintf("Node type created.\n\nid:       %s\nname:     %s\ncategory: %s\nkind:     http-action\n\nUse 'type': '%s' in workflow node definitions.",
				nt.ID, nt.Name, nt.Category, nt.ID)
			return mcpserver.Text(text), nil
		},
	}
}

// updateNodeType updates an existing custom node type.
func updateNodeType(svc *ntsvc.Service) *mcpserver.Tool {
	return &mcpserver.Tool{
		Name:        "update_node_type",
		Description: "Update an existing custom HTTP-action node type. All fields are replaced. Changes affect future executions only.",
		InputSchema: map[string]any{
			"type":     "object",
			"required": []string{"id", "name", "base_url"},
			"properties": map[string]any{
				"id":               map[string]any{"type": "string", "description": "Node type UUID to update"},
				"name":             map[string]any{"type": "string"},
				"description":      map[string]any{"type": "string"},
				"category":         map[string]any{"type": "string"},
				"base_url":         map[string]any{"type": "string"},
				"endpoint":         map[string]any{"type": "string"},
				"method":           map[string]any{"type": "string"},
				"content_type":     map[string]any{"type": "string"},
				"headers_template": map[string]any{"type": "object"},
				"body_template":    map[string]any{"type": "string"},
				"input_schema":     map[string]any{"type": "object"},
				"output_schema":    map[string]any{"type": "object"},
				"credential_id":    map[string]any{"type": "string"},
			},
		},
		Handler: func(ctx context.Context, workspaceID string, args map[string]any) (*mcpserver.ToolResult, error) {
			id := str(args, "id")
			if id == "" {
				return nil, fmt.Errorf("id is required")
			}
			in := ntsvc.UpdateInput{
				ID: id,
				CreateInput: ntsvc.CreateInput{
					WorkspaceID: workspaceID,
					Name:        str(args, "name"),
					Category:    strDefault(args, "category", "custom"),
					BaseURL:     str(args, "base_url"),
					Endpoint:    strDefault(args, "endpoint", "/"),
					Method:      strDefault(args, "method", "POST"),
					ContentType: strDefault(args, "content_type", "application/json"),
				},
			}
			if d := str(args, "description"); d != "" {
				in.Description = &d
			}
			if bt := str(args, "body_template"); bt != "" {
				in.BodyTemplate = &bt
			}
			if ci := str(args, "credential_id"); ci != "" {
				in.CredentialID = &ci
			}
			if m, ok := args["headers_template"].(map[string]any); ok {
				in.HeadersTemplate = m
			}
			if m, ok := args["input_schema"].(map[string]any); ok {
				in.InputSchema = m
			}
			if m, ok := args["output_schema"].(map[string]any); ok {
				in.OutputSchema = m
			}

			nt, err := svc.Update(ctx, in)
			if err != nil {
				return nil, fmt.Errorf("update node type: %w", err)
			}
			return mcpserver.Text(fmt.Sprintf("Node type updated.\n\nid:   %s\nname: %s", nt.ID, nt.Name)), nil
		},
	}
}

// deleteNodeType removes a custom node type.
func deleteNodeType(svc *ntsvc.Service) *mcpserver.Tool {
	return &mcpserver.Tool{
		Name:        "delete_node_type",
		Description: "Delete a custom node type. Workflows referencing this ID will fail at execution after deletion.",
		InputSchema: map[string]any{
			"type":     "object",
			"required": []string{"id"},
			"properties": map[string]any{
				"id": map[string]any{"type": "string", "description": "Node type UUID to delete"},
			},
		},
		Handler: func(ctx context.Context, workspaceID string, args map[string]any) (*mcpserver.ToolResult, error) {
			id := str(args, "id")
			if id == "" {
				return nil, fmt.Errorf("id is required")
			}
			if err := svc.Delete(ctx, workspaceID, id); err != nil {
				return nil, fmt.Errorf("delete node type: %w", err)
			}
			return mcpserver.Text(fmt.Sprintf("Node type %s deleted.", id)), nil
		},
	}
}

// helpers
func str(args map[string]any, key string) string {
	v, _ := args[key].(string)
	return v
}
func strDefault(args map[string]any, key, def string) string {
	if v := str(args, key); v != "" {
		return v
	}
	return def
}

// getNodeType returns the full schema for one node type by ID.
func getNodeType(svc *ntsvc.Service) *mcpserver.Tool {
	return &mcpserver.Tool{
		Name:        "get_node_type",
		Description: "Get the full JSON Schema and configuration for a specific node type by ID. Use this before generating a workflow definition to know exactly which config fields are required and what the node outputs.",
		InputSchema: map[string]any{
			"type":     "object",
			"required": []string{"id"},
			"properties": map[string]any{
				"id": map[string]any{
					"type":        "string",
					"description": "Node type ID: built-in string (e.g. 'http-request') or custom UUID",
				},
			},
		},
		Handler: func(ctx context.Context, workspaceID string, args map[string]any) (*mcpserver.ToolResult, error) {
			id, ok := args["id"].(string)
			if !ok || id == "" {
				return nil, fmt.Errorf("id is required")
			}

			entries, err := svc.Catalog(ctx, workspaceID, "", "")
			if err != nil {
				return nil, err
			}

			for _, e := range entries {
				if e.ID != id {
					continue
				}
				inputJSON, _ := json.MarshalIndent(e.InputSchema, "", "  ")
				outputJSON, _ := json.MarshalIndent(e.OutputSchema, "", "  ")

				text := fmt.Sprintf(
					"Node Type: %s\nID:          %s\nKind:        %s\nCategory:    %s\nVersion:     %s\nDescription: %s\n\nInput Schema (config keys for workflow definition):\n%s\n\nOutput Schema (what downstream nodes receive):\n%s",
					e.Name, e.ID, e.Kind, e.Category, e.Version, e.Description,
					string(inputJSON), string(outputJSON),
				)
				return mcpserver.Text(text), nil
			}

			return mcpserver.Text(fmt.Sprintf("Node type %q not found in catalog.", id)), nil
		},
	}
}
