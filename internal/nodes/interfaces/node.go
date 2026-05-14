package interfaces

import "context"

// Node is the plugin contract every built-in and custom node must implement.
type Node interface {
	Execute(ctx context.Context, ec ExecutionContext, input any) (any, error)
	Metadata() NodeMetadata
}

// NodeMetadata describes a node type for the unified catalog.
// InputSchema and OutputSchema are JSON Schema draft-07 objects.
type NodeMetadata struct {
	Type         string
	Name         string
	Description  string
	Version      string
	Category     string         // e.g. trigger, core, logic, utility
	InputSchema  map[string]any // JSON Schema describing required config keys
	OutputSchema map[string]any // JSON Schema describing what the node produces
}

// ExecutionContext carries per-execution state into a node.
type ExecutionContext struct {
	WorkspaceID string
	ExecutionID string
	NodeID      string
	Config      map[string]any
}
