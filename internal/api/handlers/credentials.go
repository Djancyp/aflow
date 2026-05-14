package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/djan/aflow/internal/api/middleware"
	credsvc "github.com/djan/aflow/internal/credentials/service"
	"github.com/labstack/echo/v4"
)

// CredentialHandler handles HTTP requests for credential resources.
type CredentialHandler struct {
	svc *credsvc.Service
}

func NewCredentialHandler(svc *credsvc.Service) *CredentialHandler {
	return &CredentialHandler{svc: svc}
}

// --- DTOs ---

type createCredentialReq struct {
	Name string          `json:"name" example:"Slack Bot Token"`
	Type string          `json:"type" example:"api-key"`
	Data json.RawMessage `json:"data" swaggertype:"object"`
}

type credentialResp struct {
	ID          string `json:"id"           example:"550e8400-e29b-41d4-a716-446655440020"`
	WorkspaceID string `json:"workspace_id" example:"ws_prod"`
	Name        string `json:"name"         example:"Slack Bot Token"`
	Type        string `json:"type"         example:"api-key"`
	CreatedAt   string `json:"created_at"   example:"2025-01-01T00:00:00Z"`
}

// --- handlers ---

// Create handles POST /v1/credentials
//
//	@Summary     Create credential
//	@Description Encrypts and stores a credential. The raw data is never returned after this call.
//	@Tags        Credentials
//	@Accept      json
//	@Produce     json
//	@Param       X-Workspace-ID header   string               true "Workspace ID"
//	@Param       request        body     createCredentialReq  true "Credential data (encrypted at rest)"
//	@Success     201            {object} credentialResp
//	@Failure     400,401        {object} ProblemDetail
//	@Security    BearerAuth
//	@Router      /v1/credentials [post]
func (h *CredentialHandler) Create(c echo.Context) error {
	var req createCredentialReq
	if err := c.Bind(&req); err != nil {
		return problem(c, http.StatusBadRequest, "invalid-request", "Invalid request body", err.Error())
	}

	meta, err := h.svc.Create(c.Request().Context(), credsvc.CreateInput{
		WorkspaceID: middleware.WorkspaceID(c),
		Name:        req.Name,
		Type:        req.Type,
		Data:        req.Data,
	})
	if err != nil {
		return credServiceErr(c, err)
	}
	return c.JSON(http.StatusCreated, toCredResp(meta))
}

// List handles GET /v1/credentials
//
//	@Summary     List credentials
//	@Description Returns credential metadata only. Encrypted data is never returned.
//	@Tags        Credentials
//	@Produce     json
//	@Param       X-Workspace-ID header   string true "Workspace ID"
//	@Success     200            {object} map[string]interface{}
//	@Failure     401            {object} ProblemDetail
//	@Security    BearerAuth
//	@Router      /v1/credentials [get]
func (h *CredentialHandler) List(c echo.Context) error {
	metas, err := h.svc.List(c.Request().Context(), middleware.WorkspaceID(c))
	if err != nil {
		return credServiceErr(c, err)
	}
	resp := make([]*credentialResp, len(metas))
	for i, m := range metas {
		resp[i] = toCredResp(m)
	}
	return c.JSON(http.StatusOK, map[string]any{"data": resp, "count": len(resp)})
}

// Get handles GET /v1/credentials/:id
//
//	@Summary     Get credential
//	@Description Returns metadata for a single credential. Never returns encrypted data.
//	@Tags        Credentials
//	@Produce     json
//	@Param       X-Workspace-ID header   string true "Workspace ID"
//	@Param       id             path     string true "Credential UUID"
//	@Success     200            {object} credentialResp
//	@Failure     401,404        {object} ProblemDetail
//	@Security    BearerAuth
//	@Router      /v1/credentials/{id} [get]
func (h *CredentialHandler) Get(c echo.Context) error {
	meta, err := h.svc.Get(c.Request().Context(), middleware.WorkspaceID(c), c.Param("id"))
	if err != nil {
		return credServiceErr(c, err)
	}
	return c.JSON(http.StatusOK, toCredResp(meta))
}

// Delete handles DELETE /v1/credentials/:id
//
//	@Summary     Delete credential
//	@Description Permanently deletes a credential and its encrypted data.
//	@Tags        Credentials
//	@Param       X-Workspace-ID header string true "Workspace ID"
//	@Param       id             path   string true "Credential UUID"
//	@Success     204
//	@Failure     401,404 {object} ProblemDetail
//	@Security    BearerAuth
//	@Router      /v1/credentials/{id} [delete]
func (h *CredentialHandler) Delete(c echo.Context) error {
	if err := h.svc.Delete(c.Request().Context(), middleware.WorkspaceID(c), c.Param("id")); err != nil {
		return credServiceErr(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

// --- helpers ---

func toCredResp(m *credsvc.Meta) *credentialResp {
	return &credentialResp{
		ID:          m.ID,
		WorkspaceID: m.WorkspaceID,
		Name:        m.Name,
		Type:        m.Type,
		CreatedAt:   m.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
}

func credServiceErr(c echo.Context, err error) error {
	if errors.Is(err, credsvc.ErrNotFound) {
		return problem(c, http.StatusNotFound, "not-found", "Credential not found", err.Error())
	}
	return problem(c, http.StatusInternalServerError, "internal-error", "Internal server error", "An unexpected error occurred")
}
