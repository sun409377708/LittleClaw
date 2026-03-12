package tools

import (
	"context"
	"fmt"
	"os"
	"strings"
)

type FileReadTool struct{}

func NewFileReadTool() *FileReadTool {
	return &FileReadTool{}
}

func (t *FileReadTool) Name() string {
	return "file.read"
}

func (t *FileReadTool) Description() string {
	return "Read a UTF-8 text file from disk."
}

func (t *FileReadTool) InputSchema() string {
	return `{"path":"relative-or-absolute-path-within-workspace"}`
}

func (t *FileReadTool) Validate(input map[string]any) error {
	path, ok := input["path"].(string)
	if !ok || strings.TrimSpace(path) == "" {
		return fmt.Errorf("file.read requires a non-empty path")
	}

	_, err := ensureWithinWorkspace(path)
	return err
}

func (t *FileReadTool) Execute(_ context.Context, input map[string]any) (string, error) {
	safePath, err := ensureWithinWorkspace(input["path"].(string))
	if err != nil {
		return "", err
	}

	data, err := os.ReadFile(safePath)
	if err != nil {
		return "", fmt.Errorf("read file %q: %w", safePath, err)
	}

	return string(data), nil
}
