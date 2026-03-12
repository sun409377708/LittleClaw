package scheduler

import (
	"fmt"
	"time"

	"littleclaw/internal/queue"
	"littleclaw/internal/types"
)

func Enqueue(store *queue.Store, def Definition, plannerMode string) (*RunResult, error) {
	if store == nil {
		return nil, fmt.Errorf("schedule queue store is nil")
	}

	req := queue.SubmitRequest{
		Planner: plannerMode,
		Skill:   def.Skill,
	}
	switch def.Target {
	case "task":
		req.Target = queue.JobTargetTask
		req.Task = def.Task
	case "workflow":
		req.Target = queue.JobTargetWorkflow
		req.Workflow = def.Workflow
	default:
		return nil, fmt.Errorf("unsupported schedule target %q", def.Target)
	}

	job, err := store.Submit(req)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	result := &RunResult{
		RunID:        types.NewID("schedule"),
		ScheduleName: def.Name,
		Status:       types.RunStatusCompleted,
		StartedAt:    now,
		FinishedAt:   now,
		Every:        def.Every,
		Cron:         def.Cron,
		Dispatch:     def.Dispatch,
		Target:       def.Target,
		Skill:        def.Skill,
		Task:         def.Task,
		Workflow:     def.Workflow,
		Concurrency:  def.Concurrency,
		JobID:        job.ID,
		Output:       fmt.Sprintf("enqueued job %s for %s", job.ID, def.Target),
	}
	return result, nil
}
