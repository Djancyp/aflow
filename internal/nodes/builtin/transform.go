package builtin

import (
	"context"
	"fmt"

	"github.com/djan/aflow/internal/nodes/interfaces"
)

// TransformNode maps selected fields from input to a new output object.
// Config: {"fields": {"output_key": "path.to.input.field"}}
// Dot-path notation; missing paths produce null.
type TransformNode struct{}

func (n *TransformNode) Metadata() interfaces.NodeMetadata {
	return interfaces.NodeMetadata{
		Type:        "transform",
		Name:        "Transform",
		Description: "Maps input fields to a new output object using dot-path notation. Use to reshape data between nodes.",
		Version:     "1.0.0",
		Category:    "logic",
		InputSchema: map[string]any{
			"type":     "object",
			"required": []string{"fields"},
			"properties": map[string]any{
				"fields": map[string]any{
					"type":                 "object",
					"description":          "Map of output key names to input dot-paths",
					"additionalProperties": map[string]any{"type": "string"},
					"example":              map[string]any{"user_email": "body.email", "total": "body.results.count"},
				},
			},
		},
		OutputSchema: map[string]any{
			"type":        "object",
			"description": "Object containing the mapped fields",
		},
	}
}

func (n *TransformNode) Execute(_ context.Context, ec interfaces.ExecutionContext, input any) (any, error) {
	rawFields, ok := ec.Config["fields"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("transform: 'fields' config must be an object")
	}

	out := make(map[string]any, len(rawFields))
	for outputKey, rawPath := range rawFields {
		path, _ := rawPath.(string)
		if path == "" {
			out[outputKey] = nil
			continue
		}
		val, _ := resolveDotPath(input, path)
		out[outputKey] = val
	}
	return out, nil
}
