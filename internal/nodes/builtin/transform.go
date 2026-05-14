package builtin

import (
	"context"
	"fmt"

	"github.com/djan/aflow/internal/nodes/interfaces"
)

// TransformNode maps selected fields from input to a new output object.
// Config: {"fields": {"output_key": "path.to.input.field"}}
// Dot-path notation; missing paths produce null.
// Output: the mapped object.
type TransformNode struct{}

func (n *TransformNode) Metadata() interfaces.NodeMetadata {
	return interfaces.NodeMetadata{
		Type:        "transform",
		Name:        "Transform",
		Description: "Maps input fields to a new output object using dot-path notation",
		Version:     "1.0.0",
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
