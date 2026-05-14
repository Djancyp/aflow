package builtin

import (
	"context"
	"fmt"
	"time"

	"github.com/djan/aflow/internal/nodes/interfaces"
)

// CronTriggerNode fires on a schedule and emits trigger metadata downstream.
// Config: {"schedule": "*/5 * * * *"}
type CronTriggerNode struct{}

func (n *CronTriggerNode) Metadata() interfaces.NodeMetadata {
	return interfaces.NodeMetadata{
		Type:        "trigger.cron",
		Name:        "Cron Trigger",
		Description: "Fires on a cron schedule. A River Queue job is registered on publish. The chain is self-sustaining until the workflow is deactivated.",
		Version:     "1.0.0",
		Category:    "trigger",
		InputSchema: map[string]any{
			"type":     "object",
			"required": []string{"schedule"},
			"properties": map[string]any{
				"schedule": map[string]any{
					"type":        "string",
					"description": "5-field cron expression",
					"example":     "0 9 * * 1-5",
				},
			},
		},
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"triggered_at": map[string]any{"type": "string", "format": "date-time"},
				"schedule":     map[string]any{"type": "string"},
			},
		},
	}
}

func (n *CronTriggerNode) Execute(_ context.Context, ec interfaces.ExecutionContext, _ any) (any, error) {
	schedule, _ := ec.Config["schedule"].(string)
	if schedule == "" {
		return nil, fmt.Errorf("trigger.cron: 'schedule' config is required")
	}
	return map[string]any{
		"triggered_at": time.Now().UTC().Format(time.RFC3339),
		"schedule":     schedule,
	}, nil
}
