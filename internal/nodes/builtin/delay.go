package builtin

import (
	"context"
	"fmt"
	"time"

	"github.com/djan/aflow/internal/nodes/interfaces"
)

// DelayNode pauses execution for a configurable duration.
// Config: {"duration_ms": 1000}  (max 30 000 ms)
type DelayNode struct{}

func (n *DelayNode) Metadata() interfaces.NodeMetadata {
	return interfaces.NodeMetadata{
		Type:        "delay",
		Name:        "Delay",
		Description: "Pauses execution for a fixed duration and passes input through unchanged.",
		Version:     "1.0.0",
		Category:    "logic",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"duration_ms": map[string]any{
					"type":        "integer",
					"minimum":     0,
					"maximum":     30000,
					"default":     1000,
					"description": "Milliseconds to wait before continuing",
				},
			},
		},
		OutputSchema: map[string]any{
			"description": "The original input passed through unchanged",
		},
	}
}

func (n *DelayNode) Execute(ctx context.Context, ec interfaces.ExecutionContext, input any) (any, error) {
	ms, _ := ec.Config["duration_ms"].(float64)
	if ms <= 0 {
		return input, nil
	}
	if ms > 30_000 {
		return nil, fmt.Errorf("delay: duration_ms %v exceeds maximum of 30 000", ms)
	}
	select {
	case <-time.After(time.Duration(ms) * time.Millisecond):
		return input, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
