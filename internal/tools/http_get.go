package tools

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type HTTPGetTool struct {
	client *http.Client
}

func NewHTTPGetTool() *HTTPGetTool {
	return &HTTPGetTool{
		client: &http.Client{
			Timeout: 20 * time.Second,
		},
	}
}

func (t *HTTPGetTool) Name() string {
	return "http.get"
}

func (t *HTTPGetTool) Description() string {
	return "Fetch text content from an http or https URL."
}

func (t *HTTPGetTool) InputSchema() string {
	return `{"url":"http-or-https-url"}`
}

func (t *HTTPGetTool) Validate(input map[string]any) error {
	return validateHTTPURLInput("http.get", input)
}

func (t *HTTPGetTool) Execute(ctx context.Context, input map[string]any) (string, error) {
	rawURL := input["url"].(string)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("http get %q: %w", rawURL, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return "", fmt.Errorf("read response body: %w", err)
	}
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("http get %q returned %s: %s", rawURL, resp.Status, strings.TrimSpace(string(body)))
	}

	return strings.TrimSpace(string(body)), nil
}
