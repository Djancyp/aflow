package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/djan/aflow/internal/api/middleware"
	ntsvc "github.com/djan/aflow/internal/nodetypes/service"
	"github.com/labstack/echo/v4"
)

// NodeTypeHandler handles CRUD and catalog for node types.
type NodeTypeHandler struct {
	svc *ntsvc.Service
}

func NewNodeTypeHandler(svc *ntsvc.Service) *NodeTypeHandler {
	return &NodeTypeHandler{svc: svc}
}

// --- DTOs ---

type createNodeTypeReq struct {
	Name            string          `json:"name"             example:"Send Slack Message"`
	Description     string          `json:"description"      example:"Posts a message to a Slack channel"`
	Category        string          `json:"category"         example:"communication"`
	BaseURL         string          `json:"base_url"         example:"https://slack.com/api"`
	Endpoint        string          `json:"endpoint"         example:"/chat.postMessage"`
	Method          string          `json:"method"           example:"POST"`
	ContentType     string          `json:"content_type"     example:"application/json"`
	HeadersTemplate json.RawMessage `json:"headers_template" swaggertype:"object"`
	BodyTemplate    *string         `json:"body_template"    example:"{\"channel\":\"{{input.channel}}\",\"text\":\"{{input.text}}\"}"`
	InputSchema     json.RawMessage `json:"input_schema"     swaggertype:"object"`
	OutputSchema    json.RawMessage `json:"output_schema"    swaggertype:"object"`
	CredentialID    *string         `json:"credential_id"    example:"550e8400-e29b-41d4-a716-446655440020"`
}

type nodeTypeResp struct {
	ID              string          `json:"id"               example:"550e8400-e29b-41d4-a716-446655440050"`
	WorkspaceID     string          `json:"workspace_id"     example:"ws_prod"`
	Name            string          `json:"name"             example:"Send Slack Message"`
	Description     *string         `json:"description,omitempty"`
	Category        string          `json:"category"         example:"communication"`
	Version         string          `json:"version"          example:"1.0.0"`
	BaseURL         string          `json:"base_url"         example:"https://slack.com/api"`
	Endpoint        string          `json:"endpoint"         example:"/chat.postMessage"`
	Method          string          `json:"method"           example:"POST"`
	ContentType     string          `json:"content_type"     example:"application/json"`
	HeadersTemplate map[string]any  `json:"headers_template,omitempty"`
	BodyTemplate    *string         `json:"body_template,omitempty"`
	InputSchema     map[string]any  `json:"input_schema,omitempty"`
	OutputSchema    map[string]any  `json:"output_schema,omitempty"`
	CredentialID    *string         `json:"credential_id,omitempty"`
	CreatedAt       string          `json:"created_at"       example:"2025-01-01T00:00:00Z"`
	UpdatedAt       string          `json:"updated_at"       example:"2025-01-01T00:00:00Z"`
}

type catalogEntryResp struct {
	ID           string         `json:"id"                    example:"http-request"`
	Name         string         `json:"name"                  example:"HTTP Request"`
	Description  string         `json:"description,omitempty" example:"Makes an outbound HTTP request"`
	Category     string         `json:"category"              example:"core"`
	Version      string         `json:"version"               example:"1.0.0"`
	Kind         string         `json:"kind"                  example:"builtin"`
	InputSchema  map[string]any `json:"input_schema,omitempty"`
	OutputSchema map[string]any `json:"output_schema,omitempty"`
}

// --- handlers ---

// Catalog handles GET /v1/node-types
//
//	@Summary     List node type catalog
//	@Description Returns all available node types (built-in + custom) with their JSON Schemas. Supports ?q= search and ?category= filter.
//	@Tags        Node Types
//	@Produce     json
//	@Param       X-Workspace-ID header   string true  "Workspace ID"
//	@Param       q              query    string false "Search by name or description"
//	@Param       category       query    string false "Filter by category (trigger, core, logic, utility, communication, ai, data, custom...)"
//	@Success     200            {object} map[string]interface{}
//	@Failure     401            {object} ProblemDetail
//	@Security    BearerAuth
//	@Router      /v1/node-types [get]
func (h *NodeTypeHandler) Catalog(c echo.Context) error {
	entries, err := h.svc.Catalog(
		c.Request().Context(),
		middleware.WorkspaceID(c),
		c.QueryParam("q"),
		c.QueryParam("category"),
	)
	if err != nil {
		return nodeTypeErr(c, err)
	}
	resp := make([]*catalogEntryResp, len(entries))
	for i, e := range entries {
		resp[i] = toCatalogResp(e)
	}
	return c.JSON(http.StatusOK, map[string]any{"data": resp, "count": len(resp)})
}

// Create handles POST /v1/node-types
//
//	@Summary     Create custom node type
//	@Description Defines a new HTTP-action node type for this workspace. Once created, its ID can be used as a node 'type' in workflow definitions.
//	@Tags        Node Types
//	@Accept      json
//	@Produce     json
//	@Param       X-Workspace-ID header   string            true "Workspace ID"
//	@Param       request        body     createNodeTypeReq true "Node type definition"
//	@Success     201            {object} nodeTypeResp
//	@Failure     400,401        {object} ProblemDetail
//	@Security    BearerAuth
//	@Router      /v1/node-types [post]
func (h *NodeTypeHandler) Create(c echo.Context) error {
	var req createNodeTypeReq
	if err := c.Bind(&req); err != nil {
		return problem(c, http.StatusBadRequest, "invalid-request", "Invalid request body", err.Error())
	}

	in := ntsvc.CreateInput{
		WorkspaceID:  middleware.WorkspaceID(c),
		Name:         req.Name,
		Category:     req.Category,
		BaseURL:      req.BaseURL,
		Endpoint:     req.Endpoint,
		Method:       req.Method,
		ContentType:  req.ContentType,
		BodyTemplate: req.BodyTemplate,
		CredentialID: req.CredentialID,
	}
	if req.Description != "" {
		in.Description = &req.Description
	}
	if len(req.HeadersTemplate) > 0 {
		var m map[string]any
		_ = json.Unmarshal(req.HeadersTemplate, &m)
		in.HeadersTemplate = m
	}
	if len(req.InputSchema) > 0 {
		var m map[string]any
		_ = json.Unmarshal(req.InputSchema, &m)
		in.InputSchema = m
	}
	if len(req.OutputSchema) > 0 {
		var m map[string]any
		_ = json.Unmarshal(req.OutputSchema, &m)
		in.OutputSchema = m
	}

	nt, err := h.svc.Create(c.Request().Context(), in)
	if err != nil {
		return nodeTypeErr(c, err)
	}
	return c.JSON(http.StatusCreated, toNodeTypeResp(nt))
}

// Get handles GET /v1/node-types/:id
//
//	@Summary     Get node type
//	@Description Returns a single custom node type with full configuration.
//	@Tags        Node Types
//	@Produce     json
//	@Param       X-Workspace-ID header   string true "Workspace ID"
//	@Param       id             path     string true "Node type UUID"
//	@Success     200            {object} nodeTypeResp
//	@Failure     401,404        {object} ProblemDetail
//	@Security    BearerAuth
//	@Router      /v1/node-types/{id} [get]
func (h *NodeTypeHandler) Get(c echo.Context) error {
	nt, err := h.svc.Get(c.Request().Context(), middleware.WorkspaceID(c), c.Param("id"))
	if err != nil {
		return nodeTypeErr(c, err)
	}
	return c.JSON(http.StatusOK, toNodeTypeResp(nt))
}

// Update handles PUT /v1/node-types/:id
//
//	@Summary     Update node type
//	@Description Updates a custom node type definition. Changes affect future executions only.
//	@Tags        Node Types
//	@Accept      json
//	@Produce     json
//	@Param       X-Workspace-ID header   string            true "Workspace ID"
//	@Param       id             path     string            true "Node type UUID"
//	@Param       request        body     createNodeTypeReq true "Updated node type definition"
//	@Success     200            {object} nodeTypeResp
//	@Failure     400,401,404    {object} ProblemDetail
//	@Security    BearerAuth
//	@Router      /v1/node-types/{id} [put]
func (h *NodeTypeHandler) Update(c echo.Context) error {
	var req createNodeTypeReq
	if err := c.Bind(&req); err != nil {
		return problem(c, http.StatusBadRequest, "invalid-request", "Invalid request body", err.Error())
	}

	in := ntsvc.UpdateInput{
		ID:          c.Param("id"),
		CreateInput: ntsvc.CreateInput{
			WorkspaceID:  middleware.WorkspaceID(c),
			Name:         req.Name,
			Category:     req.Category,
			BaseURL:      req.BaseURL,
			Endpoint:     req.Endpoint,
			Method:       req.Method,
			ContentType:  req.ContentType,
			BodyTemplate: req.BodyTemplate,
			CredentialID: req.CredentialID,
		},
	}
	if req.Description != "" {
		in.Description = &req.Description
	}
	if len(req.HeadersTemplate) > 0 {
		var m map[string]any
		_ = json.Unmarshal(req.HeadersTemplate, &m)
		in.HeadersTemplate = m
	}
	if len(req.InputSchema) > 0 {
		var m map[string]any
		_ = json.Unmarshal(req.InputSchema, &m)
		in.InputSchema = m
	}
	if len(req.OutputSchema) > 0 {
		var m map[string]any
		_ = json.Unmarshal(req.OutputSchema, &m)
		in.OutputSchema = m
	}

	nt, err := h.svc.Update(c.Request().Context(), in)
	if err != nil {
		return nodeTypeErr(c, err)
	}
	return c.JSON(http.StatusOK, toNodeTypeResp(nt))
}

// Delete handles DELETE /v1/node-types/:id
//
//	@Summary     Delete node type
//	@Description Permanently deletes a custom node type. Existing workflow definitions referencing it will fail at execution.
//	@Tags        Node Types
//	@Param       X-Workspace-ID header string true "Workspace ID"
//	@Param       id             path   string true "Node type UUID"
//	@Success     204
//	@Failure     401,404 {object} ProblemDetail
//	@Security    BearerAuth
//	@Router      /v1/node-types/{id} [delete]
func (h *NodeTypeHandler) Delete(c echo.Context) error {
	if err := h.svc.Delete(c.Request().Context(), middleware.WorkspaceID(c), c.Param("id")); err != nil {
		return nodeTypeErr(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

// --- helpers ---

func toNodeTypeResp(nt *ntsvc.NodeType) *nodeTypeResp {
	return &nodeTypeResp{
		ID:              nt.ID,
		WorkspaceID:     nt.WorkspaceID,
		Name:            nt.Name,
		Description:     nt.Description,
		Category:        nt.Category,
		Version:         nt.Version,
		BaseURL:         nt.BaseURL,
		Endpoint:        nt.Endpoint,
		Method:          nt.Method,
		ContentType:     nt.ContentType,
		HeadersTemplate: nt.HeadersTemplate,
		BodyTemplate:    nt.BodyTemplate,
		InputSchema:     nt.InputSchema,
		OutputSchema:    nt.OutputSchema,
		CredentialID:    nt.CredentialID,
		CreatedAt:       nt.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		UpdatedAt:       nt.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
}

func toCatalogResp(e *ntsvc.CatalogEntry) *catalogEntryResp {
	return &catalogEntryResp{
		ID:           e.ID,
		Name:         e.Name,
		Description:  e.Description,
		Category:     e.Category,
		Version:      e.Version,
		Kind:         e.Kind,
		InputSchema:  e.InputSchema,
		OutputSchema: e.OutputSchema,
	}
}

func nodeTypeErr(c echo.Context, err error) error {
	if errors.Is(err, ntsvc.ErrNotFound) {
		return problem(c, http.StatusNotFound, "not-found", "Node type not found", err.Error())
	}
	return problem(c, http.StatusInternalServerError, "internal-error", "Internal server error", "An unexpected error occurred")
}
