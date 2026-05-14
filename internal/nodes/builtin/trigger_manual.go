package builtin

import (
	"context"

	"github.com/djan/aflow/internal/nodes/interfaces"
)

// ManualTriggerNode is a no-op trigger that documents manual API invocation.
// The execution input flows through as this node's output.
type ManualTriggerNode struct{}

func (n *ManualTriggerNode) Metadata() interfaces.NodeMetadata {
	return interfaces.NodeMetadata{
		Type:        "trigger.manual",
		Name:        "Manual Trigger",
		Description: "Triggered via the REST API or MCP. Passes the execution input downstream.",
		Version:     "1.0.0",
	}
}

func (n *ManualTriggerNode) Execute(_ context.Context, _ interfaces.ExecutionContext, input any) (any, error) {
	return input, nil
}
