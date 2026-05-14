package builtin

import (
	"context"
	"fmt"
	"time"

	"github.com/djan/aflow/internal/nodes/interfaces"
)

// CronTriggerNode fires on a schedule and emits trigger metadata downstream.
// Config: {"schedule": "*/5 * * * *"}  (standard 5-field cron expression)
// Output: {"triggered_at": "<RFC3339>", "schedule": "<expression>"}
type CronTriggerNode struct{}

func (n *CronTriggerNode) Metadata() interfaces.NodeMetadata {
	return interfaces.NodeMetadata{
		Type:        "trigger.cron",
		Name:        "Cron Trigger",
		Description: "Triggered on a cron schedule. Emits trigger metadata downstream.",
		Version:     "1.0.0",
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
