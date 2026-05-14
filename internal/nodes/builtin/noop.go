package builtin

import (
	"context"

	"github.com/djan/aflow/internal/nodes/interfaces"
)

// NoOpNode passes input through unchanged. Used as fallback for unknown node types.
type NoOpNode struct{}

func (n *NoOpNode) Metadata() interfaces.NodeMetadata {
	return interfaces.NodeMetadata{
		Type:        "no-op",
		Name:        "No-Op",
		Description: "Passes input through unchanged",
		Version:     "1.0.0",
	}
}

func (n *NoOpNode) Execute(_ context.Context, _ interfaces.ExecutionContext, input any) (any, error) {
	return input, nil
}
