package routes

import (
	"github.com/djan/aflow/internal/api/handlers"
	apimw "github.com/djan/aflow/internal/api/middleware"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	echoSwagger "github.com/swaggo/echo-swagger"
	"go.opentelemetry.io/contrib/instrumentation/github.com/labstack/echo/otelecho"
)

// Register wires all routes onto the Echo instance.
func Register(
	e *echo.Echo,
	wh *handlers.WorkflowHandler,
	eh *handlers.ExecutionHandler,
	ch *handlers.CredentialHandler,
	th *handlers.DataTableHandler,
	mh *handlers.MCPHandler,
	wbh *handlers.WebhookHandler,
	authMW echo.MiddlewareFunc,
) {
	e.Use(middleware.Recover())
	e.Use(middleware.RequestID())
	e.Use(otelecho.Middleware("aflow"))
	e.Use(apimw.HTTPMetrics)
	e.Use(apimw.RateLimiter(200, 40))

	// Open endpoints — no auth.
	e.GET("/metrics", echo.WrapHandler(promhttp.Handler()))
	e.GET("/health", handlers.Health)
	// Swagger UI — dev only, no auth required.
	e.GET("/docs/*", echoSwagger.WrapHandler)

	// Webhook trigger — auth via secret param, not Bearer token.
	e.POST("/webhooks/:workflow_id", wbh.Trigger)

	// MCP endpoint — workspace-scoped.
	e.POST("/mcp", mh.Handle, authMW, apimw.WorkspaceRequired)

	v1 := e.Group("/v1", authMW, apimw.WorkspaceRequired)

	// Workflows
	wf := v1.Group("/workflows")
	wf.POST("", wh.Create)
	wf.GET("", wh.List)
	wf.GET("/:id", wh.Get)
	wf.PUT("/:id", wh.Update)
	wf.DELETE("/:id", wh.Delete)
	wf.POST("/:id/publish", wh.Publish)
	wf.POST("/:id/deactivate", wh.Deactivate)
	wf.GET("/:id/versions", wh.ListVersions)
	wf.POST("/:id/execute", eh.Execute)
	wf.GET("/:id/executions", eh.ListByWorkflow)

	// Executions
	ex := v1.Group("/executions")
	ex.GET("", eh.List)
	ex.GET("/:id", eh.Get)
	ex.GET("/:id/logs", eh.GetLogs)
	ex.POST("/:id/cancel", eh.Cancel)
	ex.POST("/:id/retry", eh.Retry)

	// Credentials (metadata only — data never returned)
	cr := v1.Group("/credentials")
	cr.POST("", ch.Create)
	cr.GET("", ch.List)
	cr.GET("/:id", ch.Get)
	cr.DELETE("/:id", ch.Delete)

	// Data Tables
	tb := v1.Group("/tables")
	tb.POST("", th.CreateTable)
	tb.GET("", th.ListTables)
	tb.GET("/:id", th.GetTable)
	tb.DELETE("/:id", th.DeleteTable)
	tb.POST("/:id/rows", th.InsertRow)
	tb.GET("/:id/rows", th.ListRows)
	tb.DELETE("/:id/rows/:row_id", th.DeleteRow)
}
