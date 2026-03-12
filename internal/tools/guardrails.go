package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func workspaceRoot() (string, error) {
	root, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}
	return filepath.Clean(root), nil
}

func ensureWithinWorkspace(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("path is empty")
	}

	root, err := workspaceRoot()
	if err != nil {
		return "", err
	}

	candidate := path
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(root, candidate)
	}
	candidate = filepath.Clean(candidate)

	rel, err := filepath.Rel(root, candidate)
	if err != nil {
		return "", fmt.Errorf("resolve path %q: %w", path, err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q escapes workspace root %q", path, root)
	}

	return candidate, nil
}

func isShellCommandAllowed(command string) error {
	disallowed := []string{
		"rm ",
		"rm -",
		"sudo ",
		"mv ",
		"chmod ",
		"chown ",
		"git reset",
		"git clean",
		"dd ",
		"> /",
		"curl ",
		"wget ",
	}

	lower := strings.ToLower(strings.TrimSpace(command))
	for _, token := range disallowed {
		if strings.Contains(lower, token) {
			return fmt.Errorf("shell command contains blocked token %q", strings.TrimSpace(token))
		}
	}
	return nil
}
