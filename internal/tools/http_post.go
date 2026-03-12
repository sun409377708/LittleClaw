package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type HTTPPostTool struct {
	client *http.Client
}

func NewHTTPPostTool() *HTTPPostTool {
	return &HTTPPostTool{
		client: &http.Client{
			Timeout: 20 * time.Second,
		},
	}
}

func (t *HTTPPostTool) Name() string {
	return "http.post"
}

func (t *HTTPPostTool) Description() string {
	return "Send a POST request to an http or https URL with a string body."
}

func (t *HTTPPostTool) InputSchema() string {
	return `{"url":"http-or-https-url","body":"string","content_type":"optional-string","headers":"optional-string-map"}`
}

func (t *HTTPPostTool) Validate(input map[string]any) error {
	if err := validateHTTPURLInput("http.post", input); err != nil {
		return err
	}
	if _, ok := input["body"]; !ok {
		return fmt.Errorf("http.post requires body")
	}
	if contentType, ok := input["content_type"]; ok {
		if _, valid := contentType.(string); !valid {
			return fmt.Errorf("http.post content_type must be a string")
		}
	}
	if headers, ok := input["headers"]; ok {
		switch typed := headers.(type) {
		case map[string]any:
			for key, value := range typed {
				if strings.TrimSpace(key) == "" {
					return fmt.Errorf("http.post headers contain an empty key")
				}
				if _, valid := value.(string); !valid {
					return fmt.Errorf("http.post header %q must have a string value", key)
				}
			}
		default:
			return fmt.Errorf("http.post headers must be an object of string values")
		}
	}
	return nil
}

func (t *HTTPPostTool) Execute(ctx context.Context, input map[string]any) (string, error) {
	rawURL := input["url"].(string)
	contentType, _ := input["content_type"].(string)
	if strings.TrimSpace(contentType) == "" {
		contentType = "application/json"
	}
	body, err := serializeHTTPPostBody(input["body"], contentType)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, bytes.NewBufferString(body))
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", contentType)
	if headers, ok := input["headers"].(map[string]any); ok {
		for key, value := range headers {
			req.Header.Set(key, value.(string))
		}
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("http post %q: %w", rawURL, err)
	}
	defer resp.Body.Close()

	payload, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return "", fmt.Errorf("read response body: %w", err)
	}
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("http post %q returned %s: %s", rawURL, resp.Status, strings.TrimSpace(string(payload)))
	}
	return strings.TrimSpace(string(payload)), nil
}

func serializeHTTPPostBody(body any, contentType string) (string, error) {
	switch typed := body.(type) {
	case string:
		return typed, nil
	default:
		if strings.Contains(strings.ToLower(contentType), "json") {
			payload, err := json.Marshal(typed)
			if err != nil {
				return "", fmt.Errorf("marshal http.post body as json: %w", err)
			}
			return string(payload), nil
		}
		return "", fmt.Errorf("http.post non-string body requires a json content type")
	}
}
