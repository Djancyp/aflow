package middleware

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

type contextKey string

const (
	WorkspaceIDKey contextKey = "workspace_id"
	UserIDKey      contextKey = "user_id"
)

// WorkspaceRequired extracts X-Workspace-ID from the request header and
// stores it in the Echo context. Rejects requests that omit the header.
func WorkspaceRequired(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		wsID := c.Request().Header.Get("X-Workspace-ID")
		if wsID == "" {
			return c.JSON(http.StatusBadRequest, map[string]any{
				"type":   "https://aflow.dev/errors/missing-workspace",
				"title":  "Missing Workspace",
				"status": http.StatusBadRequest,
				"detail": "X-Workspace-ID header is required",
			})
		}
		c.Set(string(WorkspaceIDKey), wsID)
		c.Set(string(UserIDKey), c.Request().Header.Get("X-User-ID"))
		return next(c)
	}
}

// WorkspaceID retrieves the workspace ID stored by WorkspaceRequired.
func WorkspaceID(c echo.Context) string {
	v, _ := c.Get(string(WorkspaceIDKey)).(string)
	return v
}
