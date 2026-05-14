// Package tools registers all aflow MCP tools onto an MCP server.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	execsvc "github.com/djan/aflow/internal/executions/service"
	mcpserver "github.com/djan/aflow/internal/mcp/server"
	wfsvc "github.com/djan/aflow/internal/workflows/service"
)

// Register adds all aflow tools to the server.
func Register(s *mcpserver.Server, wf *wfsvc.Service, exec *execsvc.Service) {
	s.Register(listWorkflows(wf))
	s.Register(executeWorkflow(exec, wf))
	s.Register(getExecution(exec))
	s.Register(getExecutionLogs(exec))
}

// listWorkflows lists active, published workflows in the workspace.
func listWorkflows(svc *wfsvc.Service) *mcpserver.Tool {
	return &mcpserver.Tool{
		Name:        "list_workflows",
		Description: "List active, published workflows available in the workspace.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		Handler: func(ctx context.Context, workspaceID string, _ map[string]any) (*mcpserver.ToolResult, error) {
			workflows, err := svc.List(ctx, workspaceID)
			if err != nil {
				return nil, fmt.Errorf("list workflows: %w", err)
			}

			if len(workflows) == 0 {
				return mcpserver.Text("No active workflows found in this workspace."), nil
			}

			var sb strings.Builder
			fmt.Fprintf(&sb, "Found %d workflow(s):\n\n", len(workflows))
			for _, wf := range workflows {
				active := ""
				if wf.Active {
					active = " [active]"
				}
				desc := ""
				if wf.Description != nil && *wf.Description != "" {
					desc = " — " + *wf.Description
				}
				fmt.Fprintf(&sb, "• %s%s%s\n  id: %s\n  version: %d\n\n",
					wf.Name, active, desc, wf.ID, wf.LatestVersion)
			}
			return mcpserver.Text(sb.String()), nil
		},
	}
}

// executeWorkflow triggers a workflow execution and returns the execution ID.
func executeWorkflow(exec *execsvc.Service, wf *wfsvc.Service) *mcpserver.Tool {
	return &mcpserver.Tool{
		Name:        "execute_workflow",
		Description: "Trigger a workflow execution. Returns the execution ID and initial status.",
		InputSchema: map[string]any{
			"type":     "object",
			"required": []string{"workflow_id"},
			"properties": map[string]any{
				"workflow_id": map[string]any{
					"type":        "string",
					"description": "UUID of the workflow to execute",
				},
				"input": map[string]any{
					"type":        "object",
					"description": "Optional JSON input passed to the workflow",
				},
			},
		},
		Handler: func(ctx context.Context, workspaceID string, args map[string]any) (*mcpserver.ToolResult, error) {
			workflowID, ok := args["workflow_id"].(string)
			if !ok || workflowID == "" {
				return nil, fmt.Errorf("workflow_id is required")
			}

			var inputRaw json.RawMessage
			if input, ok := args["input"]; ok {
				b, err := json.Marshal(input)
				if err != nil {
					return nil, fmt.Errorf("marshal input: %w", err)
				}
				inputRaw = b
			} else {
				inputRaw = json.RawMessage("{}")
			}

			execution, err := exec.Execute(ctx, workspaceID, workflowID, inputRaw)
			if err != nil {
				return nil, fmt.Errorf("execute workflow: %w", err)
			}

			text := fmt.Sprintf(
				"Execution started.\n\nexecution_id: %s\nworkflow_id:  %s\nstatus:       %s\ncreated_at:   %s",
				execution.ID, execution.WorkflowID, execution.Status,
				execution.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
			)
			return mcpserver.Text(text), nil
		},
	}
}

// getExecution fetches the status and metadata of a single execution.
func getExecution(svc *execsvc.Service) *mcpserver.Tool {
	return &mcpserver.Tool{
		Name:        "get_execution",
		Description: "Get the status and details of a workflow execution.",
		InputSchema: map[string]any{
			"type":     "object",
			"required": []string{"execution_id"},
			"properties": map[string]any{
				"execution_id": map[string]any{
					"type":        "string",
					"description": "UUID of the execution to inspect",
				},
			},
		},
		Handler: func(ctx context.Context, workspaceID string, args map[string]any) (*mcpserver.ToolResult, error) {
			executionID, ok := args["execution_id"].(string)
			if !ok || executionID == "" {
				return nil, fmt.Errorf("execution_id is required")
			}

			e, err := svc.Get(ctx, workspaceID, executionID)
			if err != nil {
				return nil, fmt.Errorf("get execution: %w", err)
			}

			var sb strings.Builder
			fmt.Fprintf(&sb, "execution_id:  %s\nworkflow_id:   %s\nversion_id:    %s\nstatus:        %s\n",
				e.ID, e.WorkflowID, e.WorkflowVersionID, e.Status)

			if e.TriggerSource != nil {
				fmt.Fprintf(&sb, "trigger:       %s\n", *e.TriggerSource)
			}
			if e.StartedAt != nil {
				fmt.Fprintf(&sb, "started_at:    %s\n", e.StartedAt.UTC().Format("2006-01-02T15:04:05Z"))
			}
			if e.FinishedAt != nil {
				fmt.Fprintf(&sb, "finished_at:   %s\n", e.FinishedAt.UTC().Format("2006-01-02T15:04:05Z"))
			}
			if e.Error != nil {
				fmt.Fprintf(&sb, "error:         %s\n", *e.Error)
			}
			if len(e.Output) > 0 && string(e.Output) != "null" {
				fmt.Fprintf(&sb, "output:        %s\n", string(e.Output))
			}

			return mcpserver.Text(sb.String()), nil
		},
	}
}

// getExecutionLogs retrieves structured logs for an execution.
func getExecutionLogs(svc *execsvc.Service) *mcpserver.Tool {
	return &mcpserver.Tool{
		Name:        "get_execution_logs",
		Description: "Get structured logs for a workflow execution, in chronological order.",
		InputSchema: map[string]any{
			"type":     "object",
			"required": []string{"execution_id"},
			"properties": map[string]any{
				"execution_id": map[string]any{
					"type":        "string",
					"description": "UUID of the execution",
				},
			},
		},
		Handler: func(ctx context.Context, workspaceID string, args map[string]any) (*mcpserver.ToolResult, error) {
			executionID, ok := args["execution_id"].(string)
			if !ok || executionID == "" {
				return nil, fmt.Errorf("execution_id is required")
			}

			logs, err := svc.GetLogs(ctx, workspaceID, executionID)
			if err != nil {
				return nil, fmt.Errorf("get logs: %w", err)
			}

			if len(logs) == 0 {
				return mcpserver.Text("No logs found for this execution."), nil
			}

			var sb strings.Builder
			fmt.Fprintf(&sb, "%d log entries for execution %s:\n\n", len(logs), executionID)
			for _, l := range logs {
				ts := l.CreatedAt.UTC().Format("15:04:05.000")
				nodeLabel := ""
				if l.NodeID != nil {
					nodeLabel = "[" + *l.NodeID + "] "
				}
				fmt.Fprintf(&sb, "%s %-5s %s%s\n", ts, strings.ToUpper(l.Level), nodeLabel, l.Message)
			}

			return mcpserver.Text(sb.String()), nil
		},
	}
}
