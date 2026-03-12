package tools

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

type ShellTool struct{}

func NewShellTool() *ShellTool {
	return &ShellTool{}
}

func (t *ShellTool) Name() string {
	return "shell"
}

func (t *ShellTool) Description() string {
	return "Execute a shell command in the current workspace."
}

func (t *ShellTool) InputSchema() string {
	return `{"command":"string"}`
}

func (t *ShellTool) Validate(input map[string]any) error {
	command, ok := input["command"].(string)
	if !ok || strings.TrimSpace(command) == "" {
		return fmt.Errorf("shell tool requires a non-empty command")
	}
	if err := isShellCommandAllowed(command); err != nil {
		return err
	}
	return nil
}

func (t *ShellTool) Execute(ctx context.Context, input map[string]any) (string, error) {
	command := input["command"].(string)

	cmd := exec.CommandContext(ctx, "zsh", "-lc", command)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("command failed: %w; output: %s", err, strings.TrimSpace(string(output)))
	}

	return strings.TrimSpace(string(output)), nil
}
