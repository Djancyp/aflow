package builtin

import (
	"context"
	"fmt"
	"strings"

	"github.com/djan/aflow/internal/nodes/interfaces"
)

// ConditionNode evaluates a boolean condition against the input and routes accordingly.
// Config: {"field": "body.status", "operator": "eq", "value": "ok"}
// Operators: eq, neq, contains, exists, gt, lt
// Output: {"matched": bool, "input": <original input>}
type ConditionNode struct{}

func (n *ConditionNode) Metadata() interfaces.NodeMetadata {
	return interfaces.NodeMetadata{
		Type:        "condition",
		Name:        "Condition",
		Description: "Evaluates a boolean condition and emits {matched, input}. Downstream nodes can branch on matched.",
		Version:     "1.0.0",
		Category:    "logic",
		InputSchema: map[string]any{
			"type":     "object",
			"required": []string{"field", "operator"},
			"properties": map[string]any{
				"field":    map[string]any{"type": "string", "description": "Dot-path to the field to test (e.g. body.status)"},
				"operator": map[string]any{"type": "string", "enum": []string{"eq", "neq", "gt", "lt", "contains", "exists"}, "description": "Comparison operator"},
				"value":    map[string]any{"description": "Value to compare against (not required for 'exists')"},
			},
		},
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"matched": map[string]any{"type": "boolean", "description": "True when condition is satisfied"},
				"input":   map[string]any{"description": "The original input, passed through unchanged"},
			},
		},
	}
}

func (n *ConditionNode) Execute(_ context.Context, ec interfaces.ExecutionContext, input any) (any, error) {
	field, _ := ec.Config["field"].(string)
	operator, _ := ec.Config["operator"].(string)
	expected := ec.Config["value"]

	if field == "" || operator == "" {
		return nil, fmt.Errorf("condition: 'field' and 'operator' are required")
	}

	actual, _ := resolveDotPath(input, field)
	matched, err := evaluate(actual, operator, expected)
	if err != nil {
		return nil, fmt.Errorf("condition: %w", err)
	}

	return map[string]any{"matched": matched, "input": input}, nil
}

func evaluate(actual any, operator string, expected any) (bool, error) {
	switch operator {
	case "exists":
		return actual != nil, nil
	case "eq":
		return fmt.Sprintf("%v", actual) == fmt.Sprintf("%v", expected), nil
	case "neq":
		return fmt.Sprintf("%v", actual) != fmt.Sprintf("%v", expected), nil
	case "contains":
		return strings.Contains(fmt.Sprintf("%v", actual), fmt.Sprintf("%v", expected)), nil
	case "gt":
		return toFloat(actual) > toFloat(expected), nil
	case "lt":
		return toFloat(actual) < toFloat(expected), nil
	default:
		return false, fmt.Errorf("unknown operator: %q", operator)
	}
}

func toFloat(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	case int64:
		return float64(n)
	}
	return 0
}
