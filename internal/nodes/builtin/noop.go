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
		Description: "Passes input through unchanged. Useful as a placeholder node.",
		Version:     "1.0.0",
		Category:    "utility",
		InputSchema: map[string]any{
			"type":        "object",
			"description": "Any input — passed through unchanged",
		},
		OutputSchema: map[string]any{
			"description": "Same as input",
		},
	}
}

func (n *NoOpNode) Execute(_ context.Context, _ interfaces.ExecutionContext, input any) (any, error) {
	return input, nil
}
