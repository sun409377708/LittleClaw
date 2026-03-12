package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"littleclaw/internal/planner"
)

type Config struct {
	Provider    string
	BaseURL     string
	Model       string
	APIKeyEnv   string
	Enabled     bool
	HTTPTimeout time.Duration
}

type MinimaxClient struct {
	baseURL    string
	model      string
	apiKey     string
	httpClient *http.Client
}

func NewMinimaxClient(cfg Config) (*MinimaxClient, error) {
	if !cfg.Enabled {
		return nil, fmt.Errorf("llm client is disabled")
	}

	apiKey := os.Getenv(cfg.APIKeyEnv)
	if apiKey == "" {
		return nil, fmt.Errorf("environment variable %s is empty", cfg.APIKeyEnv)
	}

	timeout := cfg.HTTPTimeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://api.minimax.chat/v1/text/chatcompletion_v2"
	}

	model := cfg.Model
	if model == "" {
		model = "MiniMax-M2.5"
	}

	return &MinimaxClient{
		baseURL: baseURL,
		model:   model,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}, nil
}

func (c *MinimaxClient) Decide(ctx context.Context, state planner.State) (planner.Decision, error) {
	reqBody := minimaxRequest{
		Model: c.model,
		Messages: []minimaxMessage{
			{Role: "system", Content: buildPlannerSystemPrompt(state)},
			{Role: "user", Content: buildDecisionPrompt(state)},
		},
		Temperature: 0.1,
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return planner.Decision{}, fmt.Errorf("marshal minimax request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL, bytes.NewReader(payload))
	if err != nil {
		return planner.Decision{}, fmt.Errorf("build minimax request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return planner.Decision{}, fmt.Errorf("call minimax: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return planner.Decision{}, fmt.Errorf("read minimax response: %w", err)
	}
	if resp.StatusCode >= 300 {
		return planner.Decision{}, fmt.Errorf("minimax returned %s: %s", resp.Status, string(body))
	}

	var parsed minimaxResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return planner.Decision{}, fmt.Errorf("parse minimax response: %w", err)
	}

	raw := parsed.Reply()
	if raw == "" {
		return planner.Decision{}, fmt.Errorf("minimax returned empty reply")
	}
	raw = sanitizePlannerReply(raw)

	var decision planner.Decision
	if err := json.Unmarshal([]byte(raw), &decision); err != nil {
		return planner.Decision{}, fmt.Errorf("parse planner decision JSON: %w; raw: %s", err, raw)
	}
	return decision, nil
}

func buildPlannerSystemPrompt(state planner.State) string {
	allowed := state.AllowedTools
	if len(allowed) == 0 {
		allowed = []string{"file.append", "file.list", "file.read", "file.write", "http.get", "http.post", "shell"}
	}

	var b strings.Builder
	b.WriteString("You are the planner for LittleClaw.\n")
	b.WriteString("Return exactly one JSON object and nothing else.\n\n")
	b.WriteString("Schema:\n")
	b.WriteString("{\"type\":\"tool\"|\"final\",\"thought\":\"...\",\"tool_name\":\"...\",\"tool_input\":{},\"final_output\":\"...\"}\n\n")
	if strings.TrimSpace(state.SkillPrompt) != "" {
		b.WriteString("Skill guidance:\n")
		b.WriteString(state.SkillPrompt)
		b.WriteString("\n\n")
	}
	b.WriteString("Allowed tools:\n")
	for i, name := range allowed {
		if spec, ok := toolCatalogForPrompt()[name]; ok {
			fmt.Fprintf(&b, "%d. %s\n   Input schema: %s\n   %s\n", i+1, name, spec.Schema, spec.Description)
		}
	}
	b.WriteString("\nRules:\n")
	b.WriteString("- Use type=\"tool\" only when a real tool call is necessary.\n")
	b.WriteString("- Use type=\"final\" when the task is complete, blocked, or should stop after a tool result.\n")
	b.WriteString("- If a previous step already returned a useful observation, prefer type=\"final\" unless another tool call is necessary.\n")
	b.WriteString("- Never invent tool names or fields.\n")
	b.WriteString("- Never use a tool that is not listed above.\n")
	b.WriteString("- Never wrap JSON in markdown fences.\n")
	b.WriteString("- Treat memories as historical hints, not guaranteed ground truth.\n")
	return b.String()
}

func buildDecisionPrompt(state planner.State) string {
	payload, _ := json.MarshalIndent(state, "", "  ")
	return "Decide the next step from this run state:\n" + string(payload)
}

type minimaxRequest struct {
	Model       string           `json:"model"`
	Messages    []minimaxMessage `json:"messages"`
	Temperature float64          `json:"temperature,omitempty"`
}

type minimaxMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type minimaxResponse struct {
	ReplyField *string `json:"reply,omitempty"`
	Choices    []struct {
		Message *struct {
			Content string `json:"content"`
			Text    string `json:"text"`
		} `json:"message,omitempty"`
		Text string `json:"text,omitempty"`
	} `json:"choices,omitempty"`
}

func (r minimaxResponse) Reply() string {
	if r.ReplyField != nil && *r.ReplyField != "" {
		return *r.ReplyField
	}
	for _, choice := range r.Choices {
		if choice.Message != nil {
			if choice.Message.Content != "" {
				return choice.Message.Content
			}
			if choice.Message.Text != "" {
				return choice.Message.Text
			}
		}
		if choice.Text != "" {
			return choice.Text
		}
	}
	return ""
}

func sanitizePlannerReply(raw string) string {
	trimmed := strings.TrimSpace(raw)
	trimmed = strings.TrimPrefix(trimmed, "```json")
	trimmed = strings.TrimPrefix(trimmed, "```JSON")
	trimmed = strings.TrimPrefix(trimmed, "```")
	trimmed = strings.TrimSuffix(trimmed, "```")
	trimmed = strings.TrimSpace(trimmed)
	if strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}") {
		return trimmed
	}
	if extracted, ok := extractFirstJSONObject(trimmed); ok {
		return extracted
	}
	return trimmed
}

func extractFirstJSONObject(raw string) (string, bool) {
	start := strings.Index(raw, "{")
	if start < 0 {
		return "", false
	}

	depth := 0
	inString := false
	escaped := false
	for i := start; i < len(raw); i++ {
		ch := raw[i]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}

		switch ch {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return raw[start : i+1], true
			}
		}
	}
	return "", false
}

type promptToolSpec struct {
	Schema      string
	Description string
}

func toolCatalogForPrompt() map[string]promptToolSpec {
	return map[string]promptToolSpec{
		"shell": {
			Schema:      `{"command":"string"}`,
			Description: `Use only for safe read-only workspace inspection such as "ls -la" or "find . -type f". Do not use destructive commands, network commands, sudo, chmod, chown, mv, rm, git reset, or git clean.`,
		},
		"file.read": {
			Schema:      `{"path":"relative-or-absolute-path-within-workspace"}`,
			Description: "Use when the task explicitly asks to read a file.",
		},
		"file.write": {
			Schema:      `{"path":"relative-or-absolute-path-within-workspace","content":"string"}`,
			Description: "Use when the task explicitly asks to create or overwrite a text file.",
		},
		"file.append": {
			Schema:      `{"path":"relative-or-absolute-path-within-workspace","content":"string"}`,
			Description: "Use when the task explicitly asks to append text to a file or create one if needed.",
		},
		"file.list": {
			Schema:      `{"path":"relative-or-absolute-path-within-workspace","recursive":"optional-bool"}`,
			Description: "Use when the task is about listing files in a directory without needing shell.",
		},
		"http.get": {
			Schema:      `{"url":"http-or-https-url"}`,
			Description: "Use when the task explicitly asks to fetch a web page or API response.",
		},
		"http.post": {
			Schema:      `{"url":"http-or-https-url","body":"string","content_type":"optional-string","headers":"optional-string-map"}`,
			Description: "Use when the task explicitly asks to send a POST request.",
		},
	}
}
