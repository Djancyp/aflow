package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	execsvc "github.com/djan/aflow/internal/executions/service"
	"github.com/djan/aflow/internal/workflows/service"
	"github.com/labstack/echo/v4"
)

// WebhookHandler handles inbound webhook triggers for workflows.
type WebhookHandler struct {
	wfSvc   *service.Service
	execSvc *execsvc.Service
}

func NewWebhookHandler(wf *service.Service, exec *execsvc.Service) *WebhookHandler {
	return &WebhookHandler{wfSvc: wf, execSvc: exec}
}

type webhookTriggerResp struct {
	ExecutionID string `json:"execution_id" example:"550e8400-e29b-41d4-a716-446655440010"`
	Status      string `json:"status"       example:"queued"`
}

// Trigger handles POST /webhooks/:workflow_id
//
//	@Summary     Webhook trigger
//	@Description Triggers a workflow execution via webhook. Authenticate with the secret from the publish response.
//	@Tags        System
//	@Accept      json
//	@Produce     json
//	@Param       workflow_id path     string             true  "Workflow UUID"
//	@Param       secret      query    string             false "Webhook secret (alternative to X-Webhook-Secret header)"
//	@Param       body        body     map[string]interface{} false "Optional JSON payload (becomes execution input)"
//	@Success     202         {object} webhookTriggerResp
//	@Failure     401         {object} ProblemDetail
//	@Failure     409         {object} ProblemDetail
//	@Router      /webhooks/{workflow_id} [post]
func (h *WebhookHandler) Trigger(c echo.Context) error {
	workflowID := c.Param("workflow_id")
	secret := c.QueryParam("secret")
	if secret == "" {
		secret = c.Request().Header.Get("X-Webhook-Secret")
	}
	if secret == "" {
		return c.JSON(http.StatusUnauthorized, map[string]any{
			"type":   "https://aflow.dev/errors/unauthorized",
			"title":  "Webhook secret required",
			"status": http.StatusUnauthorized,
		})
	}

	wf, err := h.wfSvc.GetByWebhookSecret(c.Request().Context(), workflowID, secret)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			return c.JSON(http.StatusUnauthorized, map[string]any{
				"type":   "https://aflow.dev/errors/unauthorized",
				"title":  "Invalid webhook",
				"status": http.StatusUnauthorized,
			})
		}
		return problem(c, http.StatusInternalServerError, "internal-error", "Internal error", "An unexpected error occurred")
	}

	var input json.RawMessage
	_ = json.NewDecoder(c.Request().Body).Decode(&input)
	if len(input) == 0 {
		input = json.RawMessage("{}")
	}

	exec, err := h.execSvc.ExecuteWithTrigger(c.Request().Context(), wf.WorkspaceID, workflowID, "webhook", input)
	if err != nil {
		return problem(c, http.StatusConflict, "execution-failed", "Could not start execution", err.Error())
	}

	return c.JSON(http.StatusAccepted, webhookTriggerResp{
		ExecutionID: exec.ID,
		Status:      string(exec.Status),
	})
}
