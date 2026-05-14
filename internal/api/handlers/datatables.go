package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/djan/aflow/internal/api/middleware"
	tbsvc "github.com/djan/aflow/internal/datatables/service"
	"github.com/labstack/echo/v4"
)

// DataTableHandler handles HTTP requests for data table resources.
type DataTableHandler struct {
	svc *tbsvc.Service
}

func NewDataTableHandler(svc *tbsvc.Service) *DataTableHandler {
	return &DataTableHandler{svc: svc}
}

// --- DTOs ---

type createTableReq struct {
	Name   string          `json:"name"   example:"contacts"`
	Schema json.RawMessage `json:"schema" swaggertype:"object"`
}

type insertRowReq struct {
	Data json.RawMessage `json:"data" swaggertype:"object"`
}

type tableResp struct {
	ID          string          `json:"id"           example:"550e8400-e29b-41d4-a716-446655440030"`
	WorkspaceID string          `json:"workspace_id" example:"ws_prod"`
	Name        string          `json:"name"         example:"contacts"`
	Schema      json.RawMessage `json:"schema"       swaggertype:"object"`
	CreatedAt   string          `json:"created_at"   example:"2025-01-01T00:00:00Z"`
}

type rowResp struct {
	ID          string          `json:"id"           example:"550e8400-e29b-41d4-a716-446655440031"`
	TableID     string          `json:"table_id"     example:"550e8400-e29b-41d4-a716-446655440030"`
	WorkspaceID string          `json:"workspace_id" example:"ws_prod"`
	Data        json.RawMessage `json:"data"         swaggertype:"object"`
	CreatedAt   string          `json:"created_at"   example:"2025-01-01T00:00:00Z"`
	UpdatedAt   string          `json:"updated_at"   example:"2025-01-01T00:00:00Z"`
}

// --- handlers ---

// CreateTable handles POST /v1/tables
//
//	@Summary     Create table
//	@Description Creates a data table with an optional JSON schema.
//	@Tags        Data Tables
//	@Accept      json
//	@Produce     json
//	@Param       X-Workspace-ID header   string         true "Workspace ID"
//	@Param       request        body     createTableReq true "Table definition"
//	@Success     201            {object} tableResp
//	@Failure     400,401        {object} ProblemDetail
//	@Security    BearerAuth
//	@Router      /v1/tables [post]
func (h *DataTableHandler) CreateTable(c echo.Context) error {
	var req createTableReq
	if err := c.Bind(&req); err != nil {
		return problem(c, http.StatusBadRequest, "invalid-request", "Invalid request body", err.Error())
	}

	tbl, err := h.svc.CreateTable(c.Request().Context(), tbsvc.CreateTableInput{
		WorkspaceID: middleware.WorkspaceID(c),
		Name:        req.Name,
		Schema:      req.Schema,
	})
	if err != nil {
		return tableServiceErr(c, err)
	}
	return c.JSON(http.StatusCreated, toTableResp(tbl))
}

// ListTables handles GET /v1/tables
//
//	@Summary     List tables
//	@Description Returns all data tables in the workspace.
//	@Tags        Data Tables
//	@Produce     json
//	@Param       X-Workspace-ID header   string true "Workspace ID"
//	@Success     200            {object} map[string]interface{}
//	@Failure     401            {object} ProblemDetail
//	@Security    BearerAuth
//	@Router      /v1/tables [get]
func (h *DataTableHandler) ListTables(c echo.Context) error {
	tbls, err := h.svc.ListTables(c.Request().Context(), middleware.WorkspaceID(c))
	if err != nil {
		return tableServiceErr(c, err)
	}
	resp := make([]*tableResp, len(tbls))
	for i, t := range tbls {
		resp[i] = toTableResp(t)
	}
	return c.JSON(http.StatusOK, map[string]any{"data": resp, "count": len(resp)})
}

// GetTable handles GET /v1/tables/:id
//
//	@Summary     Get table
//	@Description Returns a single data table with its schema.
//	@Tags        Data Tables
//	@Produce     json
//	@Param       X-Workspace-ID header   string true "Workspace ID"
//	@Param       id             path     string true "Table UUID"
//	@Success     200            {object} tableResp
//	@Failure     401,404        {object} ProblemDetail
//	@Security    BearerAuth
//	@Router      /v1/tables/{id} [get]
func (h *DataTableHandler) GetTable(c echo.Context) error {
	tbl, err := h.svc.GetTable(c.Request().Context(), middleware.WorkspaceID(c), c.Param("id"))
	if err != nil {
		return tableServiceErr(c, err)
	}
	return c.JSON(http.StatusOK, toTableResp(tbl))
}

// DeleteTable handles DELETE /v1/tables/:id
//
//	@Summary     Delete table
//	@Description Permanently deletes a table and all its rows.
//	@Tags        Data Tables
//	@Param       X-Workspace-ID header string true "Workspace ID"
//	@Param       id             path   string true "Table UUID"
//	@Success     204
//	@Failure     401,404 {object} ProblemDetail
//	@Security    BearerAuth
//	@Router      /v1/tables/{id} [delete]
func (h *DataTableHandler) DeleteTable(c echo.Context) error {
	if err := h.svc.DeleteTable(c.Request().Context(), middleware.WorkspaceID(c), c.Param("id")); err != nil {
		return tableServiceErr(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

// InsertRow handles POST /v1/tables/:id/rows
//
//	@Summary     Insert row
//	@Description Inserts a JSON row into a data table.
//	@Tags        Data Tables
//	@Accept      json
//	@Produce     json
//	@Param       X-Workspace-ID header   string       true "Workspace ID"
//	@Param       id             path     string       true "Table UUID"
//	@Param       request        body     insertRowReq true "Row data"
//	@Success     201            {object} rowResp
//	@Failure     400,401,404    {object} ProblemDetail
//	@Security    BearerAuth
//	@Router      /v1/tables/{id}/rows [post]
func (h *DataTableHandler) InsertRow(c echo.Context) error {
	var req insertRowReq
	if err := c.Bind(&req); err != nil {
		return problem(c, http.StatusBadRequest, "invalid-request", "Invalid request body", err.Error())
	}

	row, err := h.svc.InsertRow(c.Request().Context(), middleware.WorkspaceID(c), c.Param("id"), req.Data)
	if err != nil {
		return tableServiceErr(c, err)
	}
	return c.JSON(http.StatusCreated, toRowResp(row))
}

// ListRows handles GET /v1/tables/:id/rows
//
//	@Summary     List rows
//	@Description Returns up to 1000 rows from the table, newest first.
//	@Tags        Data Tables
//	@Produce     json
//	@Param       X-Workspace-ID header   string true "Workspace ID"
//	@Param       id             path     string true "Table UUID"
//	@Success     200            {object} map[string]interface{}
//	@Failure     401,404        {object} ProblemDetail
//	@Security    BearerAuth
//	@Router      /v1/tables/{id}/rows [get]
func (h *DataTableHandler) ListRows(c echo.Context) error {
	rows, err := h.svc.ListRows(c.Request().Context(), middleware.WorkspaceID(c), c.Param("id"))
	if err != nil {
		return tableServiceErr(c, err)
	}
	resp := make([]*rowResp, len(rows))
	for i, r := range rows {
		resp[i] = toRowResp(r)
	}
	return c.JSON(http.StatusOK, map[string]any{"data": resp, "count": len(resp)})
}

// DeleteRow handles DELETE /v1/tables/:id/rows/:row_id
//
//	@Summary     Delete row
//	@Description Permanently deletes a single row.
//	@Tags        Data Tables
//	@Param       X-Workspace-ID header string true "Workspace ID"
//	@Param       id             path   string true "Table UUID"
//	@Param       row_id         path   string true "Row UUID"
//	@Success     204
//	@Failure     401,404 {object} ProblemDetail
//	@Security    BearerAuth
//	@Router      /v1/tables/{id}/rows/{row_id} [delete]
func (h *DataTableHandler) DeleteRow(c echo.Context) error {
	if err := h.svc.DeleteRow(c.Request().Context(), middleware.WorkspaceID(c), c.Param("id"), c.Param("row_id")); err != nil {
		return tableServiceErr(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

// --- helpers ---

func toTableResp(t *tbsvc.Table) *tableResp {
	return &tableResp{
		ID:          t.ID,
		WorkspaceID: t.WorkspaceID,
		Name:        t.Name,
		Schema:      t.Schema,
		CreatedAt:   t.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
}

func toRowResp(r *tbsvc.Row) *rowResp {
	return &rowResp{
		ID:          r.ID,
		TableID:     r.TableID,
		WorkspaceID: r.WorkspaceID,
		Data:        r.Data,
		CreatedAt:   r.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		UpdatedAt:   r.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
}

func tableServiceErr(c echo.Context, err error) error {
	if errors.Is(err, tbsvc.ErrNotFound) {
		return problem(c, http.StatusNotFound, "not-found", "Table not found", err.Error())
	}
	if errors.Is(err, tbsvc.ErrRowNotFound) {
		return problem(c, http.StatusNotFound, "row-not-found", "Row not found", err.Error())
	}
	return problem(c, http.StatusInternalServerError, "internal-error", "Internal server error", "An unexpected error occurred")
}
