package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type FileWriteTool struct{}

func NewFileWriteTool() *FileWriteTool {
	return &FileWriteTool{}
}

func (t *FileWriteTool) Name() string {
	return "file.write"
}

func (t *FileWriteTool) Description() string {
	return "Write UTF-8 text content to disk, creating parent directories when needed."
}

func (t *FileWriteTool) InputSchema() string {
	return `{"path":"relative-or-absolute-path-within-workspace","content":"string"}`
}

func (t *FileWriteTool) Validate(input map[string]any) error {
	path, ok := input["path"].(string)
	if !ok || strings.TrimSpace(path) == "" {
		return fmt.Errorf("file.write requires a non-empty path")
	}

	if _, ok := input["content"].(string); !ok {
		return fmt.Errorf("file.write requires string content")
	}

	_, err := ensureWithinWorkspace(path)
	return err
}

func (t *FileWriteTool) Execute(_ context.Context, input map[string]any) (string, error) {
	safePath, err := ensureWithinWorkspace(input["path"].(string))
	if err != nil {
		return "", err
	}
	content := input["content"].(string)

	dir := filepath.Dir(safePath)
	if dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", fmt.Errorf("create parent directory for %q: %w", safePath, err)
		}
	}

	if err := os.WriteFile(safePath, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("write file %q: %w", safePath, err)
	}

	return fmt.Sprintf("wrote %d bytes to %s", len(content), safePath), nil
}
