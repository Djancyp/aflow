package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/djan/aflow/internal/api/middleware"
	"github.com/djan/aflow/internal/executions/service"
	"github.com/labstack/echo/v4"
)

// ExecutionHandler handles HTTP requests for execution resources.
type ExecutionHandler struct {
	svc *service.Service
}

func NewExecutionHandler(svc *service.Service) *ExecutionHandler {
	return &ExecutionHandler{svc: svc}
}

// --- DTOs ---

type executeReq struct {
	Input json.RawMessage `json:"input" swaggertype:"object"`
}

type executionResp struct {
	ID                string          `json:"id"                          example:"550e8400-e29b-41d4-a716-446655440010"`
	WorkspaceID       string          `json:"workspace_id"                example:"ws_prod"`
	WorkflowID        string          `json:"workflow_id"                 example:"550e8400-e29b-41d4-a716-446655440000"`
	WorkflowVersionID string          `json:"workflow_version_id"         example:"550e8400-e29b-41d4-a716-446655440001"`
	Status            string          `json:"status"                      example:"queued"`
	TriggerSource     *string         `json:"trigger_source,omitempty"    example:"api"`
	StartedAt         *string         `json:"started_at,omitempty"        example:"2025-01-01T00:00:01Z"`
	FinishedAt        *string         `json:"finished_at,omitempty"       example:"2025-01-01T00:00:05Z"`
	Input             json.RawMessage `json:"input,omitempty"             swaggertype:"object"`
	Output            json.RawMessage `json:"output,omitempty"            swaggertype:"object"`
	Error             *string         `json:"error,omitempty"`
	CreatedAt         string          `json:"created_at"                  example:"2025-01-01T00:00:00Z"`
}

type executionLogResp struct {
	ID          int64           `json:"id"                   example:"1"`
	ExecutionID string          `json:"execution_id"         example:"550e8400-e29b-41d4-a716-446655440010"`
	NodeID      *string         `json:"node_id,omitempty"    example:"fetch"`
	Level       string          `json:"level"                example:"info"`
	Message     string          `json:"message"              example:"node fetch completed"`
	Metadata    json.RawMessage `json:"metadata,omitempty"   swaggertype:"object"`
	CreatedAt   string          `json:"created_at"           example:"2025-01-01T00:00:02Z"`
}

// --- handlers ---

// Execute handles POST /v1/workflows/:id/execute
//
//	@Summary     Execute workflow
//	@Description Creates an execution and enqueues it for async processing.
//	@Tags        Executions
//	@Accept      json
//	@Produce     json
//	@Param       X-Workspace-ID header   string     true  "Workspace ID"
//	@Param       id             path     string     true  "Workflow UUID"
//	@Param       request        body     executeReq false "Optional input data"
//	@Success     202            {object} executionResp
//	@Failure     401,404,409    {object} ProblemDetail
//	@Security    BearerAuth
//	@Router      /v1/workflows/{id}/execute [post]
func (h *ExecutionHandler) Execute(c echo.Context) error {
	var req executeReq
	if err := c.Bind(&req); err != nil {
		req.Input = json.RawMessage("{}")
	}
	if len(req.Input) == 0 {
		req.Input = json.RawMessage("{}")
	}

	exec, err := h.svc.Execute(
		c.Request().Context(),
		middleware.WorkspaceID(c),
		c.Param("id"),
		req.Input,
	)
	if err != nil {
		return execServiceErr(c, err)
	}
	return c.JSON(http.StatusAccepted, toExecutionResp(exec))
}

// List handles GET /v1/executions
//
//	@Summary     List executions
//	@Description Returns executions for the workspace, newest first. Supports limit/offset pagination.
//	@Tags        Executions
//	@Produce     json
//	@Param       X-Workspace-ID header   string true  "Workspace ID"
//	@Param       limit          query    int    false "Max results (default 20, max 100)"
//	@Param       offset         query    int    false "Offset for pagination"
//	@Success     200            {object} map[string]interface{}
//	@Failure     401            {object} ProblemDetail
//	@Security    BearerAuth
//	@Router      /v1/executions [get]
func (h *ExecutionHandler) List(c echo.Context) error {
	limit := int32(20)
	offset := int32(0)
	if v := c.QueryParam("limit"); v != "" {
		var n int32
		if _, err := fmt.Sscan(v, &n); err == nil {
			limit = n
		}
	}
	if v := c.QueryParam("offset"); v != "" {
		var n int32
		if _, err := fmt.Sscan(v, &n); err == nil {
			offset = n
		}
	}
	execs, err := h.svc.ListByWorkspace(c.Request().Context(), middleware.WorkspaceID(c), limit, offset)
	if err != nil {
		return execServiceErr(c, err)
	}
	resp := make([]*executionResp, len(execs))
	for i, e := range execs {
		resp[i] = toExecutionResp(e)
	}
	return c.JSON(http.StatusOK, map[string]any{"data": resp, "count": len(resp), "limit": limit, "offset": offset})
}

// ListByWorkflow handles GET /v1/workflows/:id/executions
//
//	@Summary     List executions for a workflow
//	@Description Returns all executions for a specific workflow, newest first.
//	@Tags        Executions
//	@Produce     json
//	@Param       X-Workspace-ID header   string true "Workspace ID"
//	@Param       id             path     string true "Workflow UUID"
//	@Success     200            {object} map[string]interface{}
//	@Failure     401,404        {object} ProblemDetail
//	@Security    BearerAuth
//	@Router      /v1/workflows/{id}/executions [get]
func (h *ExecutionHandler) ListByWorkflow(c echo.Context) error {
	execs, err := h.svc.ListByWorkflow(c.Request().Context(), middleware.WorkspaceID(c), c.Param("id"))
	if err != nil {
		return execServiceErr(c, err)
	}
	resp := make([]*executionResp, len(execs))
	for i, e := range execs {
		resp[i] = toExecutionResp(e)
	}
	return c.JSON(http.StatusOK, map[string]any{"data": resp, "count": len(resp)})
}

// Get handles GET /v1/executions/:id
//
//	@Summary     Get execution
//	@Description Returns the status and details of a single execution.
//	@Tags        Executions
//	@Produce     json
//	@Param       X-Workspace-ID header   string true "Workspace ID"
//	@Param       id             path     string true "Execution UUID"
//	@Success     200            {object} executionResp
//	@Failure     401,404        {object} ProblemDetail
//	@Security    BearerAuth
//	@Router      /v1/executions/{id} [get]
func (h *ExecutionHandler) Get(c echo.Context) error {
	exec, err := h.svc.Get(c.Request().Context(), middleware.WorkspaceID(c), c.Param("id"))
	if err != nil {
		return execServiceErr(c, err)
	}
	return c.JSON(http.StatusOK, toExecutionResp(exec))
}

// GetLogs handles GET /v1/executions/:id/logs
//
//	@Summary     Get execution logs
//	@Description Returns structured per-node logs for an execution, in chronological order.
//	@Tags        Executions
//	@Produce     json
//	@Param       X-Workspace-ID header   string true "Workspace ID"
//	@Param       id             path     string true "Execution UUID"
//	@Success     200            {object} map[string]interface{}
//	@Failure     401,404        {object} ProblemDetail
//	@Security    BearerAuth
//	@Router      /v1/executions/{id}/logs [get]
func (h *ExecutionHandler) GetLogs(c echo.Context) error {
	logs, err := h.svc.GetLogs(c.Request().Context(), middleware.WorkspaceID(c), c.Param("id"))
	if err != nil {
		return execServiceErr(c, err)
	}
	resp := make([]*executionLogResp, len(logs))
	for i, l := range logs {
		resp[i] = toLogResp(l)
	}
	return c.JSON(http.StatusOK, map[string]any{"data": resp, "count": len(resp)})
}

// Cancel handles POST /v1/executions/:id/cancel
//
//	@Summary     Cancel execution
//	@Description Cancels an execution that is still pending or queued.
//	@Tags        Executions
//	@Produce     json
//	@Param       X-Workspace-ID header   string true "Workspace ID"
//	@Param       id             path     string true "Execution UUID"
//	@Success     200            {object} executionResp
//	@Failure     401,404,409    {object} ProblemDetail
//	@Security    BearerAuth
//	@Router      /v1/executions/{id}/cancel [post]
func (h *ExecutionHandler) Cancel(c echo.Context) error {
	exec, err := h.svc.Cancel(c.Request().Context(), middleware.WorkspaceID(c), c.Param("id"))
	if err != nil {
		return execServiceErr(c, err)
	}
	return c.JSON(http.StatusOK, toExecutionResp(exec))
}

// Retry handles POST /v1/executions/:id/retry
//
//	@Summary     Retry execution
//	@Description Creates a new execution from the same workflow and input as the original.
//	@Tags        Executions
//	@Produce     json
//	@Param       X-Workspace-ID header   string true "Workspace ID"
//	@Param       id             path     string true "Original execution UUID"
//	@Success     202            {object} executionResp
//	@Failure     401,404,409    {object} ProblemDetail
//	@Security    BearerAuth
//	@Router      /v1/executions/{id}/retry [post]
func (h *ExecutionHandler) Retry(c echo.Context) error {
	exec, err := h.svc.Retry(c.Request().Context(), middleware.WorkspaceID(c), c.Param("id"))
	if err != nil {
		return execServiceErr(c, err)
	}
	return c.JSON(http.StatusAccepted, toExecutionResp(exec))
}

// --- helpers ---

func toExecutionResp(e *service.Execution) *executionResp {
	r := &executionResp{
		ID:                e.ID,
		WorkspaceID:       e.WorkspaceID,
		WorkflowID:        e.WorkflowID,
		WorkflowVersionID: e.WorkflowVersionID,
		Status:            string(e.Status),
		TriggerSource:     e.TriggerSource,
		Input:             e.Input,
		Output:            e.Output,
		Error:             e.Error,
		CreatedAt:         e.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
	if e.StartedAt != nil {
		s := e.StartedAt.UTC().Format("2006-01-02T15:04:05Z")
		r.StartedAt = &s
	}
	if e.FinishedAt != nil {
		s := e.FinishedAt.UTC().Format("2006-01-02T15:04:05Z")
		r.FinishedAt = &s
	}
	return r
}

func toLogResp(l *service.ExecutionLog) *executionLogResp {
	return &executionLogResp{
		ID:          l.ID,
		ExecutionID: l.ExecutionID,
		NodeID:      l.NodeID,
		Level:       l.Level,
		Message:     l.Message,
		Metadata:    l.Metadata,
		CreatedAt:   l.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
}

func execServiceErr(c echo.Context, err error) error {
	if errors.Is(err, service.ErrNotFound) {
		return problem(c, http.StatusNotFound, "not-found", "Execution not found", err.Error())
	}
	if errors.Is(err, service.ErrWorkflowNotFound) {
		return problem(c, http.StatusNotFound, "workflow-not-found", "Workflow not found", err.Error())
	}
	if errors.Is(err, service.ErrWorkflowInactive) {
		return problem(c, http.StatusConflict, "workflow-inactive", "Workflow is not active", "Publish the workflow before executing")
	}
	if errors.Is(err, service.ErrNoPublishedVersion) {
		return problem(c, http.StatusConflict, "no-published-version", "No published version", "Publish a version before executing")
	}
	if errors.Is(err, service.ErrCancelForbidden) {
		return problem(c, http.StatusConflict, "cancel-forbidden", "Cannot cancel execution", "Only pending or queued executions can be cancelled")
	}
	return problem(c, http.StatusInternalServerError, "internal-error", "Internal server error", "An unexpected error occurred")
}
