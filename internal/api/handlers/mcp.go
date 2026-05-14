package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/djan/aflow/internal/api/middleware"
	mcpserver "github.com/djan/aflow/internal/mcp/server"
	"github.com/labstack/echo/v4"
)

// MCPHandler bridges Echo HTTP requests to the MCP JSON-RPC dispatcher.
type MCPHandler struct {
	srv *mcpserver.Server
}

func NewMCPHandler(srv *mcpserver.Server) *MCPHandler {
	return &MCPHandler{srv: srv}
}

// Handle processes a single MCP JSON-RPC request.
// POST /mcp  — workspace scoped via X-Workspace-ID header.
func (h *MCPHandler) Handle(c echo.Context) error {
	var req mcpserver.Request
	if err := json.NewDecoder(c.Request().Body).Decode(&req); err != nil {
		return c.JSON(http.StatusOK, mcpserver.Response{
			JSONRPC: "2.0",
			Error:   &mcpserver.RPCError{Code: -32700, Message: "parse error: " + err.Error()},
		})
	}

	wsID := middleware.WorkspaceID(c)
	resp := h.srv.Dispatch(c.Request().Context(), wsID, req)

	// notifications/initialized returns an empty Response — no body needed.
	if resp.JSONRPC == "" {
		return c.NoContent(http.StatusNoContent)
	}

	return c.JSON(http.StatusOK, resp)
}
