package engine_test

import (
	"encoding/json"
	"testing"

	"github.com/djan/aflow/internal/runtime/engine"
)

func def(nodes []engine.NodeConfig, edges []engine.Edge) *engine.WorkflowDefinition {
	return &engine.WorkflowDefinition{Nodes: nodes, Edges: edges}
}

func node(id, typ string) engine.NodeConfig {
	return engine.NodeConfig{ID: id, Type: typ}
}

// TestTopologicalSort_Linear verifies a straight pipeline A→B→C.
func TestTopologicalSort_Linear(t *testing.T) {
	d := def(
		[]engine.NodeConfig{node("a", "http-request"), node("b", "transform"), node("c", "no-op")},
		[]engine.Edge{{From: "a", To: "b"}, {From: "b", To: "c"}},
	)
	order, err := engine.TopologicalSort(d)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(order) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(order))
	}
	if order[0].ID != "a" || order[1].ID != "b" || order[2].ID != "c" {
		t.Errorf("wrong order: %v %v %v", order[0].ID, order[1].ID, order[2].ID)
	}
}

// TestTopologicalSort_Single verifies a single node with no edges.
func TestTopologicalSort_Single(t *testing.T) {
	d := def([]engine.NodeConfig{node("only", "no-op")}, nil)
	order, err := engine.TopologicalSort(d)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(order) != 1 || order[0].ID != "only" {
		t.Errorf("expected single node 'only', got %v", order)
	}
}

// TestTopologicalSort_Diamond verifies fan-out + fan-in (A→B, A→C, B→D, C→D).
func TestTopologicalSort_Diamond(t *testing.T) {
	d := def(
		[]engine.NodeConfig{node("a", "no-op"), node("b", "no-op"), node("c", "no-op"), node("d", "no-op")},
		[]engine.Edge{{From: "a", To: "b"}, {From: "a", To: "c"}, {From: "b", To: "d"}, {From: "c", To: "d"}},
	)
	order, err := engine.TopologicalSort(d)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(order) != 4 {
		t.Fatalf("expected 4 nodes, got %d", len(order))
	}
	// a must be first, d must be last.
	if order[0].ID != "a" {
		t.Errorf("expected 'a' first, got %q", order[0].ID)
	}
	if order[3].ID != "d" {
		t.Errorf("expected 'd' last, got %q", order[3].ID)
	}
}

// TestTopologicalSort_Cycle detects a cycle (A→B→A).
func TestTopologicalSort_Cycle(t *testing.T) {
	d := def(
		[]engine.NodeConfig{node("a", "no-op"), node("b", "no-op")},
		[]engine.Edge{{From: "a", To: "b"}, {From: "b", To: "a"}},
	)
	_, err := engine.TopologicalSort(d)
	if err == nil {
		t.Fatal("expected cycle error, got nil")
	}
}

// TestTopologicalSort_UnknownNode rejects edges that reference missing nodes.
func TestTopologicalSort_UnknownNode(t *testing.T) {
	d := def(
		[]engine.NodeConfig{node("a", "no-op")},
		[]engine.Edge{{From: "a", To: "ghost"}},
	)
	_, err := engine.TopologicalSort(d)
	if err == nil {
		t.Fatal("expected error for unknown node, got nil")
	}
}

// TestParseDefinition_Valid unmarshals a well-formed definition.
func TestParseDefinition_Valid(t *testing.T) {
	raw := json.RawMessage(`{
		"nodes": [{"id":"n1","type":"http-request","config":{"url":"https://x.com"}}],
		"edges": []
	}`)
	def, err := engine.ParseDefinition(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(def.Nodes) != 1 || def.Nodes[0].ID != "n1" {
		t.Errorf("wrong parse result: %+v", def)
	}
}

// TestParseDefinition_Invalid rejects malformed JSON.
func TestParseDefinition_Invalid(t *testing.T) {
	_, err := engine.ParseDefinition(json.RawMessage(`{bad json}`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

// TestParentOutputs_SingleParent returns the parent's output keyed by parent ID.
func TestParentOutputs_SingleParent(t *testing.T) {
	edges := []engine.Edge{{From: "a", To: "b"}}
	outputs := map[string]any{"a": map[string]any{"key": "val"}}

	parents := engine.ParentOutputs("b", edges, outputs)
	if len(parents) != 1 {
		t.Fatalf("expected 1 parent, got %d", len(parents))
	}
	if _, ok := parents["a"]; !ok {
		t.Error("expected parent 'a' in result")
	}
}

// TestParentOutputs_NoParents returns empty map for root nodes.
func TestParentOutputs_NoParents(t *testing.T) {
	edges := []engine.Edge{{From: "a", To: "b"}}
	outputs := map[string]any{"a": "data"}

	parents := engine.ParentOutputs("a", edges, outputs)
	if len(parents) != 0 {
		t.Errorf("expected empty map for root, got %v", parents)
	}
}

// TestParentOutputs_MultipleParents collects all parent outputs.
func TestParentOutputs_MultipleParents(t *testing.T) {
	edges := []engine.Edge{{From: "a", To: "c"}, {From: "b", To: "c"}}
	outputs := map[string]any{"a": "outA", "b": "outB"}

	parents := engine.ParentOutputs("c", edges, outputs)
	if len(parents) != 2 {
		t.Fatalf("expected 2 parents, got %d", len(parents))
	}
}
