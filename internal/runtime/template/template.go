// Package template resolves {{namespace.path}} placeholders in strings and JSON.
//
// Two namespaces are supported:
//   - {{input.field.nested}}   — resolved from the node's input data
//   - {{credential.field}}     — resolved from the decrypted credential JSON
package template

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// Namespaces maps namespace name → data (map or json.RawMessage).
type Namespaces map[string]any

var re = regexp.MustCompile(`\{\{(\w+)\.([^}]+)\}\}`)

// Resolve replaces all {{ns.path}} tokens in s with values from ns.
// Missing paths are replaced with an empty string.
func Resolve(s string, ns Namespaces) string {
	return re.ReplaceAllStringFunc(s, func(m string) string {
		subs := re.FindStringSubmatch(m)
		if len(subs) < 3 {
			return m
		}
		namespace, path := subs[1], subs[2]
		data, ok := ns[namespace]
		if !ok {
			return ""
		}
		val := walkPath(data, path)
		if val == nil {
			return ""
		}
		return stringify(val)
	})
}

// ResolveHeaders applies string substitution to every value in headers map.
func ResolveHeaders(headers map[string]any, ns Namespaces) map[string]string {
	out := make(map[string]string, len(headers))
	for k, v := range headers {
		out[k] = Resolve(fmt.Sprintf("%v", v), ns)
	}
	return out
}

// ResolveBody resolves a body template.
// If the template is valid JSON, uses type-preserving substitution so that
// a string value like "{{input.count}}" where count is an integer becomes
// an integer in the output, not the string "42".
// Falls back to plain string substitution for non-JSON bodies.
func ResolveBody(body string, ns Namespaces) string {
	var parsed any
	if err := json.Unmarshal([]byte(body), &parsed); err == nil {
		resolved := resolveValue(parsed, ns)
		b, err := json.Marshal(resolved)
		if err == nil {
			return string(b)
		}
	}
	return Resolve(body, ns)
}

// resolveValue recursively substitutes templates inside a decoded JSON value.
func resolveValue(v any, ns Namespaces) any {
	switch val := v.(type) {
	case string:
		// If the string is exactly one template token, return the typed value.
		if isExact(val) {
			if resolved := resolveExact(val, ns); resolved != nil {
				return resolved
			}
			return ""
		}
		return Resolve(val, ns)
	case map[string]any:
		out := make(map[string]any, len(val))
		for k, vv := range val {
			out[k] = resolveValue(vv, ns)
		}
		return out
	case []any:
		out := make([]any, len(val))
		for i, vv := range val {
			out[i] = resolveValue(vv, ns)
		}
		return out
	default:
		return v
	}
}

// isExact returns true when s is exactly one {{ns.path}} token.
func isExact(s string) bool {
	return re.MatchString(s) && re.ReplaceAllString(s, "") == ""
}

// resolveExact returns the typed value for an exact {{ns.path}} token.
func resolveExact(s string, ns Namespaces) any {
	subs := re.FindStringSubmatch(s)
	if len(subs) < 3 {
		return nil
	}
	namespace, path := subs[1], subs[2]
	data, ok := ns[namespace]
	if !ok {
		return nil
	}
	return walkPath(data, path)
}

// walkPath traverses a nested structure following dot-separated path segments.
func walkPath(data any, path string) any {
	parts := strings.SplitN(path, ".", 2)
	key := parts[0]

	var obj map[string]any
	switch v := data.(type) {
	case map[string]any:
		obj = v
	case json.RawMessage:
		if err := json.Unmarshal(v, &obj); err != nil {
			return nil
		}
	default:
		// Try marshalling to map as last resort.
		b, err := json.Marshal(v)
		if err != nil {
			return nil
		}
		if err := json.Unmarshal(b, &obj); err != nil {
			return nil
		}
	}

	val, ok := obj[key]
	if !ok {
		return nil
	}
	if len(parts) == 1 {
		return val
	}
	return walkPath(val, parts[1])
}

func stringify(v any) string {
	switch val := v.(type) {
	case string:
		return val
	case bool:
		if val {
			return "true"
		}
		return "false"
	case json.Number:
		return val.String()
	default:
		b, _ := json.Marshal(v)
		return string(b)
	}
}
