package registry

import (
	"fmt"
	"sync"

	"github.com/djan/aflow/internal/nodes/interfaces"
)

// Registry holds all registered node types.
type Registry struct {
	mu    sync.RWMutex
	nodes map[string]interfaces.Node
}

func New() *Registry {
	return &Registry{nodes: make(map[string]interfaces.Node)}
}

func (r *Registry) Register(node interfaces.Node) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.nodes[node.Metadata().Type] = node
}

func (r *Registry) Get(nodeType string) (interfaces.Node, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	n, ok := r.nodes[nodeType]
	if !ok {
		return nil, fmt.Errorf("node type %q not registered", nodeType)
	}
	return n, nil
}

func (r *Registry) List() []interfaces.NodeMetadata {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]interfaces.NodeMetadata, 0, len(r.nodes))
	for _, n := range r.nodes {
		out = append(out, n.Metadata())
	}
	return out
}
