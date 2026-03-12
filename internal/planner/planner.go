package planner

import (
	"context"
	"fmt"
	"strings"

	"littleclaw/internal/types"
)

type DecisionType string

const (
	DecisionTool  DecisionType = "tool"
	DecisionFinal DecisionType = "final"
)

type Decision struct {
	Type         DecisionType   `json:"type"`
	Thought      string         `json:"thought"`
	FinalOutput  string         `json:"final_output,omitempty"`
	ToolName     string         `json:"tool_name,omitempty"`
	ToolInput    map[string]any `json:"tool_input,omitempty"`
	ToolInputRaw string         `json:"tool_input_raw,omitempty"`
}

type State struct {
	Task         types.Task   `json:"task"`
	Steps        []types.Step `json:"steps"`
	AllowedTools []string     `json:"allowed_tools,omitempty"`
	SkillPrompt  string       `json:"skill_prompt,omitempty"`
	Memories     []MemoryNote `json:"memories,omitempty"`
}

type MemoryNote struct {
	Kind    string   `json:"kind"`
	Subject string   `json:"subject"`
	Summary string   `json:"summary"`
	Status  string   `json:"status"`
	Tags    []string `json:"tags,omitempty"`
}

type Planner interface {
	Decide(ctx context.Context, state State) (Decision, error)
}

type RuleBasedPlanner struct{}

func NewRuleBased() *RuleBasedPlanner {
	return &RuleBasedPlanner{}
}

func (p *RuleBasedPlanner) Decide(_ context.Context, state State) (Decision, error) {
	task := state.Task.Input
	lower := strings.ToLower(task)

	if len(state.Steps) > 0 {
		last := state.Steps[len(state.Steps)-1]
		if last.Error != "" {
			return Decision{
				Type:        DecisionFinal,
				Thought:     "Stopping because the previous step failed.",
				FinalOutput: fmt.Sprintf("Task failed at step %d: %s", last.Index, last.Error),
			}, nil
		}

		if last.ActionType == "tool" {
			return Decision{
				Type:        DecisionFinal,
				Thought:     "Tool execution produced an observation; return it as the current final output.",
				FinalOutput: last.Observation,
			}, nil
		}
	}

	switch {
	case strings.Contains(lower, "追加到文件") || strings.Contains(lower, "append file") || strings.Contains(lower, "append to file"):
		path := extractPath(task)
		if path == "" {
			return Decision{
				Type:        DecisionFinal,
				Thought:     "The task asks to append to a file but no path was detected.",
				FinalOutput: "No output file path found in task. Use quotes or include a concrete path.",
			}, nil
		}
		content := extractContent(task)
		if content == "" {
			return Decision{
				Type:        DecisionFinal,
				Thought:     "The task asks to append to a file but no content block was detected.",
				FinalOutput: "No file content found in task. Use `内容:` or `content:` followed by text.",
			}, nil
		}
		return Decision{
			Type:     DecisionTool,
			Thought:  "Use the file.append tool because the task is explicitly about appending to a file.",
			ToolName: "file.append",
			ToolInput: map[string]any{
				"path":    path,
				"content": content,
			},
		}, nil
	case strings.Contains(lower, "列出文件") || strings.Contains(lower, "list files") || strings.Contains(lower, "list directory"):
		path := extractPath(task)
		if path == "" {
			path = "."
		}
		return Decision{
			Type:     DecisionTool,
			Thought:  "Use the file.list tool because the task is about listing files in a directory.",
			ToolName: "file.list",
			ToolInput: map[string]any{
				"path":      path,
				"recursive": strings.Contains(lower, "递归") || strings.Contains(lower, "recursive"),
			},
		}, nil
	case strings.Contains(lower, "post ") || strings.Contains(lower, "http post") ||
		strings.Contains(lower, "post请求") || strings.Contains(lower, "发送post"):
		targetURL := extractURL(task)
		if targetURL == "" {
			return Decision{
				Type:        DecisionFinal,
				Thought:     "The task looks like an HTTP POST but no URL was detected.",
				FinalOutput: "No URL found in task. Include a concrete http or https URL.",
			}, nil
		}
		body := extractContent(task)
		if body == "" {
			return Decision{
				Type:        DecisionFinal,
				Thought:     "The task looks like an HTTP POST but no request body was detected.",
				FinalOutput: "No request body found in task. Use `内容:` or `content:` followed by the POST body.",
			}, nil
		}
		return Decision{
			Type:     DecisionTool,
			Thought:  "Use the http.post tool because the task is about sending a POST request.",
			ToolName: "http.post",
			ToolInput: map[string]any{
				"url":          targetURL,
				"body":         body,
				"content_type": "application/json",
			},
		}, nil
	case strings.Contains(lower, "http://") || strings.Contains(lower, "https://") ||
		strings.Contains(lower, "访问网页") || strings.Contains(lower, "fetch url") ||
		strings.Contains(lower, "http get") || strings.Contains(lower, "请求接口"):
		targetURL := extractURL(task)
		if targetURL == "" {
			return Decision{
				Type:        DecisionFinal,
				Thought:     "The task looks like an HTTP fetch but no URL was detected.",
				FinalOutput: "No URL found in task. Include a concrete http or https URL.",
			}, nil
		}
		return Decision{
			Type:     DecisionTool,
			Thought:  "Use the http.get tool because the task is about fetching a URL.",
			ToolName: "http.get",
			ToolInput: map[string]any{
				"url": targetURL,
			},
		}, nil
	case strings.Contains(lower, "写入文件") || strings.Contains(lower, "write file") || strings.Contains(lower, "save file"):
		path := extractPath(task)
		if path == "" {
			return Decision{
				Type:        DecisionFinal,
				Thought:     "The task asks to write a file but no path was detected.",
				FinalOutput: "No output file path found in task. Use quotes or include a concrete path.",
			}, nil
		}
		content := extractContent(task)
		if content == "" {
			return Decision{
				Type:        DecisionFinal,
				Thought:     "The task asks to write a file but no content block was detected.",
				FinalOutput: "No file content found in task. Use `内容:` or `content:` followed by text.",
			}, nil
		}
		return Decision{
			Type:     DecisionTool,
			Thought:  "Use the file.write tool because the task is explicitly about writing a file.",
			ToolName: "file.write",
			ToolInput: map[string]any{
				"path":    path,
				"content": content,
			},
		}, nil
	case strings.Contains(lower, "读取文件") || strings.Contains(lower, "read file") || strings.Contains(lower, "open file"):
		path := extractPath(task)
		if path == "" {
			return Decision{
				Type:        DecisionFinal,
				Thought:     "The task asks to read a file but no path was detected.",
				FinalOutput: "No file path found in task. Use quotes or include a concrete path.",
			}, nil
		}
		return Decision{
			Type:     DecisionTool,
			Thought:  "Use the file.read tool because the task is explicitly about reading a file.",
			ToolName: "file.read",
			ToolInput: map[string]any{
				"path": path,
			},
		}, nil
	case strings.Contains(lower, "当前目录") || strings.Contains(lower, "目录") ||
		strings.Contains(lower, "largest") || strings.Contains(lower, "ls") ||
		strings.Contains(lower, "find") || strings.Contains(lower, "shell"):
		return Decision{
			Type:     DecisionTool,
			Thought:  "Use the shell tool for filesystem inspection tasks.",
			ToolName: "shell",
			ToolInput: map[string]any{
				"command": deriveShellCommand(task),
			},
		}, nil
	default:
		return Decision{
			Type:        DecisionFinal,
			Thought:     "No suitable tool pattern matched this task yet.",
			FinalOutput: fmt.Sprintf("Task accepted but no planner rule matched: %s", task),
		}, nil
	}
}

func extractPath(task string) string {
	for _, quote := range []string{`"`, `'`, "`"} {
		parts := strings.Split(task, quote)
		if len(parts) >= 3 && strings.TrimSpace(parts[1]) != "" {
			return strings.TrimSpace(parts[1])
		}
	}

	fields := strings.Fields(task)
	for _, field := range fields {
		if strings.HasPrefix(field, "/") || strings.HasPrefix(field, "./") || strings.HasPrefix(field, "../") {
			return strings.Trim(field, `"'`)
		}
	}
	return ""
}

func extractContent(task string) string {
	lower := strings.ToLower(task)
	markers := []string{"内容:", "content:", "正文:", "text:"}
	for _, marker := range markers {
		idx := strings.Index(lower, marker)
		if idx >= 0 {
			originalIdx := idx + len(marker)
			return strings.TrimSpace(task[originalIdx:])
		}
	}

	for _, quote := range []string{`"`, `'`, "`"} {
		parts := strings.Split(task, quote)
		if len(parts) >= 5 && strings.TrimSpace(parts[3]) != "" {
			return strings.TrimSpace(parts[3])
		}
	}
	return ""
}

func extractURL(task string) string {
	fields := strings.Fields(task)
	for _, field := range fields {
		candidate := strings.Trim(field, `"'(),[]`)
		if strings.HasPrefix(candidate, "http://") || strings.HasPrefix(candidate, "https://") {
			return candidate
		}
	}
	return ""
}

func deriveShellCommand(task string) string {
	lower := strings.ToLower(task)
	if strings.Contains(lower, "largest") || strings.Contains(task, "最大的10个文件") {
		return `find . -type f -print0 | xargs -0 du -h | sort -hr | head -10`
	}
	if strings.Contains(lower, "当前目录") || strings.Contains(lower, "目录") {
		return "ls -la"
	}
	for _, prefix := range []string{"shell:", "shell "} {
		if strings.HasPrefix(lower, prefix) {
			return strings.TrimSpace(task[len(prefix):])
		}
	}
	return task
}
