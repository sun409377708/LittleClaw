package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type FileListTool struct{}

func NewFileListTool() *FileListTool {
	return &FileListTool{}
}

func (t *FileListTool) Name() string {
	return "file.list"
}

func (t *FileListTool) Description() string {
	return "List files and directories under a workspace path."
}

func (t *FileListTool) InputSchema() string {
	return `{"path":"relative-or-absolute-path-within-workspace","recursive":"optional-bool"}`
}

func (t *FileListTool) Validate(input map[string]any) error {
	path, ok := input["path"].(string)
	if !ok || strings.TrimSpace(path) == "" {
		return fmt.Errorf("file.list requires a non-empty path")
	}
	if _, err := ensureWithinWorkspace(path); err != nil {
		return err
	}
	if recursive, ok := input["recursive"]; ok {
		if _, valid := recursive.(bool); !valid {
			return fmt.Errorf("file.list recursive must be a boolean")
		}
	}
	return nil
}

func (t *FileListTool) Execute(_ context.Context, input map[string]any) (string, error) {
	safePath, err := ensureWithinWorkspace(input["path"].(string))
	if err != nil {
		return "", err
	}
	recursive, _ := input["recursive"].(bool)

	info, err := os.Stat(safePath)
	if err != nil {
		return "", fmt.Errorf("stat path %q: %w", safePath, err)
	}
	if !info.IsDir() {
		return filepath.Base(safePath), nil
	}

	if recursive {
		return listRecursive(safePath)
	}
	return listShallow(safePath)
}

func listShallow(path string) (string, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return "", fmt.Errorf("read dir %q: %w", path, err)
	}

	lines := make([]string, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() {
			name += "/"
		}
		lines = append(lines, name)
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n"), nil
}

func listRecursive(root string) (string, error) {
	lines := make([]string, 0, 32)
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if d.IsDir() {
			rel += "/"
		}
		lines = append(lines, rel)
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("walk dir %q: %w", root, err)
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n"), nil
}
