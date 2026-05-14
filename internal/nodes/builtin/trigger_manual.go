package builtin

import (
	"context"

	"github.com/djan/aflow/internal/nodes/interfaces"
)

// ManualTriggerNode is a no-op trigger that documents manual API invocation.
type ManualTriggerNode struct{}

func (n *ManualTriggerNode) Metadata() interfaces.NodeMetadata {
	return interfaces.NodeMetadata{
		Type:        "trigger.manual",
		Name:        "Manual Trigger",
		Description: "Triggered via the REST API or MCP tool. The execution input flows through as output.",
		Version:     "1.0.0",
		Category:    "trigger",
		InputSchema: map[string]any{
			"type":        "object",
			"description": "Optional input payload sent at trigger time",
			"properties":  map[string]any{},
		},
		OutputSchema: map[string]any{
			"description": "The trigger input payload",
		},
	}
}

func (n *ManualTriggerNode) Execute(_ context.Context, _ interfaces.ExecutionContext, input any) (any, error) {
	return input, nil
}
