package scheduler

import (
	"fmt"
	"strings"

	"littleclaw/internal/memory"
	"littleclaw/internal/types"
)

func (r *RunResult) MemoryEntry() memory.Entry {
	subject := r.ScheduleName
	if r.Target == "task" && strings.TrimSpace(r.Task) != "" {
		subject = r.Task
	}
	if r.Target == "workflow" && strings.TrimSpace(r.Workflow) != "" {
		subject = r.Workflow
	}

	return memory.Entry{
		ID:         types.NewID("memory"),
		Kind:       "schedule",
		SourceID:   r.RunID,
		Subject:    subject,
		Summary:    truncateScheduleSummary(r),
		Status:     r.Status,
		StartedAt:  r.StartedAt,
		FinishedAt: r.FinishedAt,
		Tags: []string{
			r.ScheduleName,
			r.Target,
			r.Dispatch,
			r.Concurrency,
		},
		Metadata: map[string]any{
			"schedule_name": r.ScheduleName,
		},
	}
}

func truncateScheduleSummary(r *RunResult) string {
	if strings.TrimSpace(r.Output) != "" {
		return truncateScheduleText(r.Output)
	}
	return truncateScheduleText(fmt.Sprintf("schedule %s finished with target %s", r.ScheduleName, r.Target))
}

func truncateScheduleText(raw string) string {
	return memory.TruncateText(raw, 400)
}
