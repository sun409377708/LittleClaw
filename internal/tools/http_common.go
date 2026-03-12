package tools

import (
	"fmt"
	"net/url"
	"strings"
)

func validateHTTPURLInput(toolName string, input map[string]any) error {
	rawURL, ok := input["url"].(string)
	if !ok || strings.TrimSpace(rawURL) == "" {
		return fmt.Errorf("%s requires a non-empty url", toolName)
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid url %q: %w", rawURL, err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("url scheme must be http or https")
	}
	if parsed.Host == "" {
		return fmt.Errorf("url host is empty")
	}
	return nil
}
