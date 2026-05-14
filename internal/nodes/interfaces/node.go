package interfaces

import "context"

// Node is the plugin contract every built-in and external node must implement.
type Node interface {
	Execute(ctx context.Context, ec ExecutionContext, input any) (any, error)
	Metadata() NodeMetadata
}

type NodeMetadata struct {
	Type        string
	Name        string
	Description string
	Version     string
}

// ExecutionContext carries per-execution state into a node.
type ExecutionContext struct {
	WorkspaceID string
	ExecutionID string
	NodeID      string
	Config      map[string]any
}
