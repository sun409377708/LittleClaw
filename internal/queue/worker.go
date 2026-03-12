package queue

import (
	"context"
	"fmt"
	"time"

	"littleclaw/internal/types"
	"littleclaw/internal/workflow"
)

type RuntimeRunner interface {
	Run(ctx context.Context, task types.Task) (*types.RunResult, error)
}

type WorkflowRunner interface {
	Run(ctx context.Context, task types.Task) (*types.RunResult, error)
	ExecuteTool(ctx context.Context, name string, input map[string]any) (string, error)
}

func ExecuteJob(ctx context.Context, store *Store, runner WorkflowRunner, workflowDir, memoryDir, workerID string, job *Job) (*Job, error) {
	if store == nil || runner == nil || job == nil {
		return nil, fmt.Errorf("worker execute job requires store, runner, and job")
	}

	switch job.Target {
	case JobTargetTask:
		run, err := runner.Run(ctx, types.Task{
			ID:          types.NewID("task"),
			Input:       job.Task,
			Skill:       job.Skill,
			RequestedAt: time.Now().UTC(),
		})
		if err != nil {
			updated, failErr := store.Fail(job.ID, err.Error())
			if failErr != nil {
				return nil, failErr
			}
			return updated, nil
		}
		if run.Status == types.RunStatusFailed {
			updated, failErr := store.Fail(job.ID, run.Output)
			if failErr != nil {
				return nil, failErr
			}
			return updated, nil
		}
		updated, err := store.Complete(job.ID, run.Output, run.RunID, "")
		return updated, err
	case JobTargetWorkflow:
		def, err := workflow.LoadByName(workflowDir, job.Workflow)
		if err != nil {
			updated, failErr := store.Fail(job.ID, err.Error())
			if failErr != nil {
				return nil, failErr
			}
			return updated, nil
		}
		run, err := workflow.Execute(ctx, runner, def)
		if err != nil {
			updated, failErr := store.Fail(job.ID, err.Error())
			if failErr != nil {
				return nil, failErr
			}
			return updated, nil
		}
		if err := workflow.PersistRunWithMemoryDir(run, memoryDir); err != nil {
			updated, failErr := store.Fail(job.ID, err.Error())
			if failErr != nil {
				return nil, failErr
			}
			return updated, nil
		}
		if run.Status == types.RunStatusFailed {
			updated, failErr := store.Fail(job.ID, run.Output)
			if failErr != nil {
				return nil, failErr
			}
			return updated, nil
		}
		updated, err := store.Complete(job.ID, run.Output, "", run.RunID)
		return updated, err
	default:
		err := fmt.Errorf("unsupported job target %q", job.Target)
		updated, failErr := store.Fail(job.ID, err.Error())
		if failErr != nil {
			return nil, failErr
		}
		return updated, nil
	}
}
