package builtin

import (
	"context"

	"github.com/djan/aflow/internal/nodes/interfaces"
)

// WebhookTriggerNode receives an inbound HTTP payload and passes it downstream.
// The execution input (set from the webhook request body) flows through as output.
// Config (optional): {"method": "POST", "content_type": "application/json"}
type WebhookTriggerNode struct{}

func (n *WebhookTriggerNode) Metadata() interfaces.NodeMetadata {
	return interfaces.NodeMetadata{
		Type:        "trigger.webhook",
		Name:        "Webhook Trigger",
		Description: "Triggered by an inbound HTTP request. Request body becomes execution input.",
		Version:     "1.0.0",
	}
}

func (n *WebhookTriggerNode) Execute(_ context.Context, _ interfaces.ExecutionContext, input any) (any, error) {
	return input, nil
}
