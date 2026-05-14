package engine

import (
	"encoding/json"
	"fmt"
)

// WorkflowDefinition is the parsed DAG stored in workflow_versions.definition.
type WorkflowDefinition struct {
	Nodes []NodeConfig `json:"nodes"`
	Edges []Edge       `json:"edges"`
}

type NodeConfig struct {
	ID     string         `json:"id"`
	Type   string         `json:"type"`
	Config map[string]any `json:"config"`
	Retry  *RetryConfig   `json:"retry,omitempty"`
}

type Edge struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type RetryConfig struct {
	MaxAttempts int `json:"max_attempts"`
	DelayMS     int `json:"delay_ms"`
}

// ParseDefinition unmarshals a raw JSON definition into a WorkflowDefinition.
func ParseDefinition(raw json.RawMessage) (*WorkflowDefinition, error) {
	var def WorkflowDefinition
	if err := json.Unmarshal(raw, &def); err != nil {
		return nil, fmt.Errorf("parse workflow definition: %w", err)
	}
	return &def, nil
}

// TopologicalSort returns nodes in execution order using Kahn's algorithm.
// Returns an error if the graph contains a cycle.
func TopologicalSort(def *WorkflowDefinition) ([]NodeConfig, error) {
	inDegree := make(map[string]int, len(def.Nodes))
	adj := make(map[string][]string, len(def.Nodes))
	nodeMap := make(map[string]NodeConfig, len(def.Nodes))

	for _, n := range def.Nodes {
		inDegree[n.ID] = 0
		nodeMap[n.ID] = n
	}

	for _, e := range def.Edges {
		if _, ok := nodeMap[e.From]; !ok {
			return nil, fmt.Errorf("edge references unknown node %q", e.From)
		}
		if _, ok := nodeMap[e.To]; !ok {
			return nil, fmt.Errorf("edge references unknown node %q", e.To)
		}
		adj[e.From] = append(adj[e.From], e.To)
		inDegree[e.To]++
	}

	queue := make([]string, 0, len(def.Nodes))
	for id, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, id)
		}
	}

	sorted := make([]NodeConfig, 0, len(def.Nodes))
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		sorted = append(sorted, nodeMap[cur])

		for _, next := range adj[cur] {
			inDegree[next]--
			if inDegree[next] == 0 {
				queue = append(queue, next)
			}
		}
	}

	if len(sorted) != len(def.Nodes) {
		return nil, fmt.Errorf("workflow definition contains a cycle")
	}

	return sorted, nil
}

// FindTriggerNode returns the first node whose type starts with "trigger.",
// or nil if the workflow has no explicit trigger node.
func FindTriggerNode(def *WorkflowDefinition) *NodeConfig {
	for i := range def.Nodes {
		if len(def.Nodes[i].Type) > 8 && def.Nodes[i].Type[:8] == "trigger." {
			return &def.Nodes[i]
		}
	}
	return nil
}

// ParentOutputs returns the combined outputs from all parent nodes for a given node.
// The result is keyed by parent node ID.
func ParentOutputs(nodeID string, edges []Edge, outputs map[string]any) map[string]any {
	parents := make(map[string]any)
	for _, e := range edges {
		if e.To == nodeID {
			if out, ok := outputs[e.From]; ok {
				parents[e.From] = out
			}
		}
	}
	return parents
}
