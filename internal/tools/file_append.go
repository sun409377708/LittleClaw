package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type FileAppendTool struct{}

func NewFileAppendTool() *FileAppendTool {
	return &FileAppendTool{}
}

func (t *FileAppendTool) Name() string {
	return "file.append"
}

func (t *FileAppendTool) Description() string {
	return "Append UTF-8 text content to a file, creating parent directories and the file when needed."
}

func (t *FileAppendTool) InputSchema() string {
	return `{"path":"relative-or-absolute-path-within-workspace","content":"string"}`
}

func (t *FileAppendTool) Validate(input map[string]any) error {
	path, ok := input["path"].(string)
	if !ok || strings.TrimSpace(path) == "" {
		return fmt.Errorf("file.append requires a non-empty path")
	}
	if _, ok := input["content"].(string); !ok {
		return fmt.Errorf("file.append requires string content")
	}
	_, err := ensureWithinWorkspace(path)
	return err
}

func (t *FileAppendTool) Execute(_ context.Context, input map[string]any) (string, error) {
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

	f, err := os.OpenFile(safePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return "", fmt.Errorf("open file %q for append: %w", safePath, err)
	}
	defer f.Close()

	written, err := f.WriteString(content)
	if err != nil {
		return "", fmt.Errorf("append file %q: %w", safePath, err)
	}

	return fmt.Sprintf("appended %d bytes to %s", written, safePath), nil
}
