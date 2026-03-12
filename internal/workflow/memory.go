package workflow

import (
	"fmt"
	"strings"

	"littleclaw/internal/memory"
	"littleclaw/internal/types"
)

func (r *RunResult) MemoryEntry() memory.Entry {
	tags := make([]string, 0, len(r.StepRuns))
	for _, step := range r.StepRuns {
		switch step.Type {
		case "agent":
			if step.Skill != "" {
				tags = append(tags, step.Skill)
			}
		case "tool":
			if step.Tool != "" {
				tags = append(tags, step.Tool)
			}
		}
	}
	return memory.Entry{
		ID:         types.NewID("memory"),
		Kind:       "workflow",
		SourceID:   r.RunID,
		Subject:    r.WorkflowName,
		Summary:    truncateWorkflowSummary(r),
		Status:     r.Status,
		StartedAt:  r.StartedAt,
		FinishedAt: r.FinishedAt,
		Tags:       tags,
		Metadata: map[string]any{
			"step_runs": len(r.StepRuns),
		},
	}
}

func truncateWorkflowSummary(r *RunResult) string {
	if strings.TrimSpace(r.Output) != "" {
		return truncateWorkflowText(r.Output)
	}
	return truncateWorkflowText(fmt.Sprintf("workflow %s finished with %d steps", r.WorkflowName, len(r.StepRuns)))
}

func truncateWorkflowText(raw string) string {
	return memory.TruncateText(raw, 400)
}
