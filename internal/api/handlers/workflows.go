package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/djan/aflow/internal/api/middleware"
	"github.com/djan/aflow/internal/workflows/service"
	"github.com/labstack/echo/v4"
)

// WorkflowHandler handles HTTP requests for workflow resources.
type WorkflowHandler struct {
	svc *service.Service
}

func NewWorkflowHandler(svc *service.Service) *WorkflowHandler {
	return &WorkflowHandler{svc: svc}
}

// --- request / response DTOs ---

type createWorkflowReq struct {
	Name        string          `json:"name"        example:"Email Processor"`
	Description string          `json:"description" example:"Processes incoming emails"`
	Definition  json.RawMessage `json:"definition"  swaggertype:"object"`
}

type updateWorkflowReq struct {
	Name        string `json:"name"        example:"Email Processor v2"`
	Description string `json:"description" example:"Updated description"`
}

type workflowResp struct {
	ID            string  `json:"id"             example:"550e8400-e29b-41d4-a716-446655440000"`
	WorkspaceID   string  `json:"workspace_id"   example:"ws_prod"`
	Name          string  `json:"name"           example:"Email Processor"`
	Description   *string `json:"description,omitempty"`
	Active        bool    `json:"active"         example:"true"`
	LatestVersion int32   `json:"latest_version" example:"1"`
	CreatedAt     string  `json:"created_at"     example:"2025-01-01T00:00:00Z"`
	UpdatedAt     string  `json:"updated_at"     example:"2025-01-01T00:00:00Z"`
}

type versionResp struct {
	ID         string          `json:"id"          example:"550e8400-e29b-41d4-a716-446655440001"`
	WorkflowID string          `json:"workflow_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Version    int32           `json:"version"     example:"1"`
	Definition json.RawMessage `json:"definition"  swaggertype:"object"`
	Published  bool            `json:"published"   example:"true"`
	CreatedAt  string          `json:"created_at"  example:"2025-01-01T00:00:00Z"`
}

type createWorkflowResp struct {
	Workflow *workflowResp `json:"workflow"`
	Version  *versionResp  `json:"version"`
}

type publishWorkflowResp struct {
	Workflow   *workflowResp `json:"workflow"`
	Version    *versionResp  `json:"version"`
	WebhookURL string        `json:"webhook_url,omitempty" example:"/webhooks/uuid?secret=whsec_..."`
}

// --- handlers ---

// Create handles POST /v1/workflows
//
//	@Summary     Create workflow
//	@Description Creates a new workflow with an initial version (definition required).
//	@Tags        Workflows
//	@Accept      json
//	@Produce     json
//	@Param       X-Workspace-ID header   string            true "Workspace ID"
//	@Param       request        body     createWorkflowReq true "Workflow definition"
//	@Success     201            {object} createWorkflowResp
//	@Failure     400            {object} ProblemDetail
//	@Failure     401            {object} ProblemDetail
//	@Failure     500            {object} ProblemDetail
//	@Security    BearerAuth
//	@Router      /v1/workflows [post]
func (h *WorkflowHandler) Create(c echo.Context) error {
	var req createWorkflowReq
	if err := c.Bind(&req); err != nil {
		return problem(c, http.StatusBadRequest, "invalid-request", "Invalid request body", err.Error())
	}

	wsID := middleware.WorkspaceID(c)
	var desc *string
	if req.Description != "" {
		desc = &req.Description
	}

	wf, ver, err := h.svc.Create(c.Request().Context(), service.CreateInput{
		WorkspaceID: wsID,
		Name:        req.Name,
		Description: desc,
		Definition:  req.Definition,
	})
	if err != nil {
		return serviceErr(c, err)
	}

	return c.JSON(http.StatusCreated, createWorkflowResp{
		Workflow: toWorkflowResp(wf),
		Version:  toVersionResp(ver),
	})
}

// List handles GET /v1/workflows
//
//	@Summary     List workflows
//	@Description Returns all workflows in the workspace.
//	@Tags        Workflows
//	@Produce     json
//	@Param       X-Workspace-ID header   string true "Workspace ID"
//	@Success     200            {object} map[string]interface{}
//	@Failure     401            {object} ProblemDetail
//	@Security    BearerAuth
//	@Router      /v1/workflows [get]
func (h *WorkflowHandler) List(c echo.Context) error {
	wfs, err := h.svc.List(c.Request().Context(), middleware.WorkspaceID(c))
	if err != nil {
		return serviceErr(c, err)
	}
	resp := make([]*workflowResp, len(wfs))
	for i, wf := range wfs {
		resp[i] = toWorkflowResp(wf)
	}
	return c.JSON(http.StatusOK, map[string]any{"data": resp, "count": len(resp)})
}

// Get handles GET /v1/workflows/:id
//
//	@Summary     Get workflow
//	@Description Returns a single workflow by ID.
//	@Tags        Workflows
//	@Produce     json
//	@Param       X-Workspace-ID header   string true  "Workspace ID"
//	@Param       id             path     string true  "Workflow UUID"
//	@Success     200            {object} workflowResp
//	@Failure     401,404        {object} ProblemDetail
//	@Security    BearerAuth
//	@Router      /v1/workflows/{id} [get]
func (h *WorkflowHandler) Get(c echo.Context) error {
	wf, err := h.svc.Get(c.Request().Context(), middleware.WorkspaceID(c), c.Param("id"))
	if err != nil {
		return serviceErr(c, err)
	}
	return c.JSON(http.StatusOK, toWorkflowResp(wf))
}

// Update handles PUT /v1/workflows/:id
//
//	@Summary     Update workflow
//	@Description Updates name and description. Does not change the definition.
//	@Tags        Workflows
//	@Accept      json
//	@Produce     json
//	@Param       X-Workspace-ID header   string           true "Workspace ID"
//	@Param       id             path     string           true "Workflow UUID"
//	@Param       request        body     updateWorkflowReq true "Fields to update"
//	@Success     200            {object} workflowResp
//	@Failure     400,401,404    {object} ProblemDetail
//	@Security    BearerAuth
//	@Router      /v1/workflows/{id} [put]
func (h *WorkflowHandler) Update(c echo.Context) error {
	var req updateWorkflowReq
	if err := c.Bind(&req); err != nil {
		return problem(c, http.StatusBadRequest, "invalid-request", "Invalid request body", err.Error())
	}

	wsID := middleware.WorkspaceID(c)
	var desc *string
	if req.Description != "" {
		desc = &req.Description
	}

	wf, err := h.svc.Update(c.Request().Context(), service.UpdateInput{
		WorkspaceID: wsID,
		ID:          c.Param("id"),
		Name:        req.Name,
		Description: desc,
	})
	if err != nil {
		return serviceErr(c, err)
	}
	return c.JSON(http.StatusOK, toWorkflowResp(wf))
}

// Delete handles DELETE /v1/workflows/:id
//
//	@Summary     Delete workflow
//	@Description Permanently deletes a workflow and all its versions and executions.
//	@Tags        Workflows
//	@Param       X-Workspace-ID header string true "Workspace ID"
//	@Param       id             path   string true "Workflow UUID"
//	@Success     204
//	@Failure     401,404 {object} ProblemDetail
//	@Security    BearerAuth
//	@Router      /v1/workflows/{id} [delete]
func (h *WorkflowHandler) Delete(c echo.Context) error {
	if err := h.svc.Delete(c.Request().Context(), middleware.WorkspaceID(c), c.Param("id")); err != nil {
		return serviceErr(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

// Deactivate handles POST /v1/workflows/:id/deactivate
//
//	@Summary     Deactivate workflow
//	@Description Sets active=false. Stops cron chains and rejects webhook triggers.
//	@Tags        Workflows
//	@Produce     json
//	@Param       X-Workspace-ID header   string true "Workspace ID"
//	@Param       id             path     string true "Workflow UUID"
//	@Success     200            {object} workflowResp
//	@Failure     401,404        {object} ProblemDetail
//	@Security    BearerAuth
//	@Router      /v1/workflows/{id}/deactivate [post]
func (h *WorkflowHandler) Deactivate(c echo.Context) error {
	wf, err := h.svc.Deactivate(c.Request().Context(), middleware.WorkspaceID(c), c.Param("id"))
	if err != nil {
		return serviceErr(c, err)
	}
	return c.JSON(http.StatusOK, toWorkflowResp(wf))
}

// Publish handles POST /v1/workflows/:id/publish
//
//	@Summary     Publish workflow
//	@Description Validates the definition, marks the latest version published, sets active=true, and activates its trigger (cron or webhook).
//	@Tags        Workflows
//	@Produce     json
//	@Param       X-Workspace-ID header   string true "Workspace ID"
//	@Param       id             path     string true "Workflow UUID"
//	@Success     200            {object} publishWorkflowResp
//	@Failure     400,401,404,409 {object} ProblemDetail
//	@Security    BearerAuth
//	@Router      /v1/workflows/{id}/publish [post]
func (h *WorkflowHandler) Publish(c echo.Context) error {
	wf, ver, err := h.svc.Publish(c.Request().Context(), middleware.WorkspaceID(c), c.Param("id"))
	if err != nil {
		return serviceErr(c, err)
	}

	resp := &publishWorkflowResp{
		Workflow: toWorkflowResp(wf),
		Version:  toVersionResp(ver),
	}
	if wf.WebhookSecret != nil && *wf.WebhookSecret != "" {
		resp.WebhookURL = "/webhooks/" + wf.ID + "?secret=" + *wf.WebhookSecret
	}
	return c.JSON(http.StatusOK, resp)
}

// ListVersions handles GET /v1/workflows/:id/versions
//
//	@Summary     List workflow versions
//	@Description Returns all immutable versions of a workflow, newest first.
//	@Tags        Workflows
//	@Produce     json
//	@Param       X-Workspace-ID header   string true "Workspace ID"
//	@Param       id             path     string true "Workflow UUID"
//	@Success     200            {object} map[string]interface{}
//	@Failure     401,404        {object} ProblemDetail
//	@Security    BearerAuth
//	@Router      /v1/workflows/{id}/versions [get]
func (h *WorkflowHandler) ListVersions(c echo.Context) error {
	vers, err := h.svc.ListVersions(c.Request().Context(), middleware.WorkspaceID(c), c.Param("id"))
	if err != nil {
		return serviceErr(c, err)
	}
	resp := make([]*versionResp, len(vers))
	for i, v := range vers {
		resp[i] = toVersionResp(v)
	}
	return c.JSON(http.StatusOK, map[string]any{"data": resp, "count": len(resp)})
}

// --- helpers ---

func toWorkflowResp(wf *service.Workflow) *workflowResp {
	return &workflowResp{
		ID:            wf.ID,
		WorkspaceID:   wf.WorkspaceID,
		Name:          wf.Name,
		Description:   wf.Description,
		Active:        wf.Active,
		LatestVersion: wf.LatestVersion,
		CreatedAt:     wf.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		UpdatedAt:     wf.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
}

func toVersionResp(v *service.WorkflowVersion) *versionResp {
	return &versionResp{
		ID:         v.ID,
		WorkflowID: v.WorkflowID,
		Version:    v.Version,
		Definition: v.Definition,
		Published:  v.Published,
		CreatedAt:  v.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
}

func serviceErr(c echo.Context, err error) error {
	if errors.Is(err, service.ErrNotFound) {
		return problem(c, http.StatusNotFound, "not-found", "Workflow not found", err.Error())
	}
	if errors.Is(err, service.ErrForbidden) {
		return problem(c, http.StatusForbidden, "forbidden", "Access denied", err.Error())
	}
	return problem(c, http.StatusInternalServerError, "internal-error", "Internal server error", "An unexpected error occurred")
}

func problem(c echo.Context, status int, slug, title, detail string) error {
	return c.JSON(status, map[string]any{
		"type":   "https://aflow.dev/errors/" + slug,
		"title":  title,
		"status": status,
		"detail": detail,
	})
}
