package builtin

import (
	"context"

	"github.com/djan/aflow/internal/nodes/interfaces"
)

// WebhookTriggerNode receives an inbound HTTP payload and passes it downstream.
type WebhookTriggerNode struct{}

func (n *WebhookTriggerNode) Metadata() interfaces.NodeMetadata {
	return interfaces.NodeMetadata{
		Type:        "trigger.webhook",
		Name:        "Webhook Trigger",
		Description: "Triggered by an inbound HTTP POST. The request body becomes the execution input. A webhook secret is auto-generated on publish.",
		Version:     "1.0.0",
		Category:    "trigger",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"method": map[string]any{
					"type":    "string",
					"enum":    []string{"POST", "GET", "PUT"},
					"default": "POST",
				},
			},
		},
		OutputSchema: map[string]any{
			"description": "The HTTP request body passed through as-is",
		},
	}
}

func (n *WebhookTriggerNode) Execute(_ context.Context, _ interfaces.ExecutionContext, input any) (any, error) {
	return input, nil
}
