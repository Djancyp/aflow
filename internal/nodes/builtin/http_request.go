package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/djan/aflow/internal/nodes/interfaces"
)

var httpClient = &http.Client{Timeout: 30 * time.Second}

// HTTPRequestNode makes an outbound HTTP request.
// Config keys: url (required), method (default GET), headers (map), body (string).
type HTTPRequestNode struct{}

func (n *HTTPRequestNode) Metadata() interfaces.NodeMetadata {
	return interfaces.NodeMetadata{
		Type:        "http-request",
		Name:        "HTTP Request",
		Description: "Makes an HTTP request to an external URL",
		Version:     "1.0.0",
	}
}

func (n *HTTPRequestNode) Execute(ctx context.Context, ec interfaces.ExecutionContext, input any) (any, error) {
	rawURL, _ := ec.Config["url"].(string)
	if rawURL == "" {
		return nil, fmt.Errorf("http-request: config.url is required")
	}

	method := "GET"
	if m, ok := ec.Config["method"].(string); ok && m != "" {
		method = strings.ToUpper(m)
	}

	var bodyReader io.Reader
	if b, ok := ec.Config["body"].(string); ok && b != "" {
		bodyReader = strings.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, rawURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("http-request: build request: %w", err)
	}

	if headers, ok := ec.Config["headers"].(map[string]any); ok {
		for k, v := range headers {
			req.Header.Set(k, fmt.Sprintf("%v", v))
		}
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http-request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20)) // 10 MB cap
	if err != nil {
		return nil, fmt.Errorf("http-request: read response: %w", err)
	}

	var parsed any
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		parsed = string(respBody)
	}

	respHeaders := make(map[string]string, len(resp.Header))
	for k := range resp.Header {
		respHeaders[k] = resp.Header.Get(k)
	}

	return map[string]any{
		"status_code": resp.StatusCode,
		"headers":     respHeaders,
		"body":        parsed,
	}, nil
}
