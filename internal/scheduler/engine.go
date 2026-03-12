package scheduler

import (
	"context"
	"fmt"
	"time"

	"littleclaw/internal/types"
	"littleclaw/internal/workflow"
)

type Runner interface {
	Run(ctx context.Context, task types.Task) (*types.RunResult, error)
	ExecuteTool(ctx context.Context, name string, input map[string]any) (string, error)
}

func Execute(ctx context.Context, runner Runner, workflowDir, memoryDir string, def Definition) (*RunResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if runner == nil {
		return nil, fmt.Errorf("schedule runner is nil")
	}

	result := &RunResult{
		RunID:        types.NewID("schedule"),
		ScheduleName: def.Name,
		Status:       types.RunStatusCompleted,
		StartedAt:    time.Now().UTC(),
		Every:        def.Every,
		Cron:         def.Cron,
		Dispatch:     def.Dispatch,
		Target:       def.Target,
		Skill:        def.Skill,
		Task:         def.Task,
		Workflow:     def.Workflow,
		Concurrency:  def.Concurrency,
	}

	switch def.Target {
	case "task":
		run, err := runner.Run(ctx, types.Task{
			ID:          types.NewID("task"),
			Input:       def.Task,
			Skill:       def.Skill,
			RequestedAt: time.Now().UTC(),
		})
		if err != nil {
			result.Status = types.RunStatusFailed
			result.Output = err.Error()
			break
		}
		result.TaskRunID = run.RunID
		result.Status = run.Status
		result.Output = run.Output
	case "workflow":
		loaded, err := workflow.LoadByName(workflowDir, def.Workflow)
		if err != nil {
			result.Status = types.RunStatusFailed
			result.Output = err.Error()
			break
		}
		run, err := workflow.Execute(ctx, runner, loaded)
		if err != nil {
			result.Status = types.RunStatusFailed
			result.Output = err.Error()
			break
		}
		result.WorkflowRunID = run.RunID
		result.Status = run.Status
		result.Output = run.Output
		if err := workflow.PersistRunWithMemoryDir(run, memoryDir); err != nil {
			result.Status = types.RunStatusFailed
			result.Output = fmt.Sprintf("%s (workflow persist failed: %v)", run.Output, err)
		}
	default:
		result.Status = types.RunStatusFailed
		result.Output = fmt.Sprintf("unsupported schedule target %q", def.Target)
	}

	result.FinishedAt = time.Now().UTC()
	return result, nil
}

func parsePositiveDuration(raw string) (time.Duration, error) {
	value, err := time.ParseDuration(raw)
	if err != nil {
		return 0, err
	}
	if value <= 0 {
		return 0, fmt.Errorf("duration must be greater than zero")
	}
	return value, nil
}
