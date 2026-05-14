package builtin

import (
	"strconv"
	"strings"
)

// resolveDotPath extracts a nested value from data using dot notation.
// Examples: "body.items.0.name", "status", "$.body"
// Returns (nil, false) if any path segment is missing.
func resolveDotPath(data any, path string) (any, bool) {
	path = strings.TrimPrefix(path, "$.")
	if path == "$" || path == "" {
		return data, true
	}

	parts := strings.SplitN(path, ".", 2)
	key := parts[0]
	rest := ""
	if len(parts) == 2 {
		rest = parts[1]
	}

	var next any
	switch v := data.(type) {
	case map[string]any:
		val, ok := v[key]
		if !ok {
			return nil, false
		}
		next = val
	case []any:
		idx, err := strconv.Atoi(key)
		if err != nil || idx < 0 || idx >= len(v) {
			return nil, false
		}
		next = v[idx]
	default:
		return nil, false
	}

	if rest == "" {
		return next, true
	}
	return resolveDotPath(next, rest)
}
