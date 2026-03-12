package runtime

import (
	"strings"

	"littleclaw/internal/memory"
	"littleclaw/internal/types"
)

func buildTaskMemoryEntry(task types.Task, run *types.RunResult) memory.Entry {
	summary := strings.TrimSpace(run.Output)
	if summary == "" {
		summary = "task completed without output"
	}

	tags := []string{task.Skill}
	seenTools := make(map[string]struct{})
	for _, step := range run.Steps {
		if step.ToolName == "" {
			continue
		}
		if _, ok := seenTools[step.ToolName]; ok {
			continue
		}
		seenTools[step.ToolName] = struct{}{}
		tags = append(tags, step.ToolName)
	}

	return memory.Entry{
		ID:         types.NewID("memory"),
		Kind:       "task",
		SourceID:   run.RunID,
		ParentID:   task.ID,
		Subject:    task.Input,
		Summary:    truncateSummary(summary),
		Status:     run.Status,
		StartedAt:  run.StartedAt,
		FinishedAt: run.FinishedAt,
		Tags:       tags,
		Metadata: map[string]any{
			"skill": task.Skill,
			"steps": len(run.Steps),
		},
	}
}

func truncateSummary(raw string) string {
	return memory.TruncateText(raw, 400)
}
