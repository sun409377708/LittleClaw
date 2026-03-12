package workflow

import (
	"context"
	"fmt"
	"time"

	"littleclaw/internal/approval"
	"littleclaw/internal/types"
)

type Runner interface {
	Run(ctx context.Context, task types.Task) (*types.RunResult, error)
	ExecuteTool(ctx context.Context, name string, input map[string]any) (string, error)
}

func Execute(ctx context.Context, runner Runner, def Definition) (*RunResult, error) {
	result := &RunResult{
		RunID:        types.NewID("workflow"),
		WorkflowName: def.Name,
		Status:       types.RunStatusCompleted,
		StartedAt:    time.Now().UTC(),
		StepRuns:     make([]WorkflowStepRun, 0, len(def.Steps)),
	}
	return executeInto(ctx, runner, def, result, 0)
}

func Resume(ctx context.Context, runner Runner, def Definition, existing *RunResult) (*RunResult, error) {
	if existing == nil {
		return nil, fmt.Errorf("workflow run is nil")
	}
	startIndex := len(existing.StepRuns)
	if existing.Status == types.RunStatusWaitingApproval && startIndex > 0 {
		last := existing.StepRuns[startIndex-1]
		if last.Type == "approval" {
			startIndex--
			existing.StepRuns = existing.StepRuns[:startIndex]
		}
	}
	existing.Status = types.RunStatusCompleted
	existing.Output = ""
	return executeInto(ctx, runner, def, existing, startIndex)
}

func executeInto(ctx context.Context, runner Runner, def Definition, result *RunResult, startIndex int) (*RunResult, error) {
	if runner == nil {
		return nil, fmt.Errorf("workflow runner is nil")
	}
	if result == nil {
		return nil, fmt.Errorf("workflow result is nil")
	}
	continuedFailure := false

	for i := startIndex; i < len(def.Steps); i++ {
		step := def.Steps[i]
		resolved, err := resolveStep(step, result.StepRuns)
		if err != nil {
			result.Status = types.RunStatusFailed
			result.Output = err.Error()
			result.StepRuns = append(result.StepRuns, WorkflowStepRun{
				Type:       defaultStepType(step.Type),
				Name:       step.Name,
				Skill:      step.Skill,
				Task:       step.Task,
				Tool:       step.Tool,
				ToolInput:  step.ToolInput,
				Prompt:     step.Prompt,
				WhenStep:   step.WhenStep,
				WhenStatus: step.WhenStatus,
				OnFailure:  step.OnFailure,
				Status:     types.RunStatusFailed,
				Output:     err.Error(),
			})
			result.FinishedAt = time.Now().UTC()
			return result, nil
		}

		if shouldSkip, reason := shouldSkipStep(resolved, result.StepRuns); shouldSkip {
			result.StepRuns = append(result.StepRuns, WorkflowStepRun{
				Type:       defaultStepType(resolved.Type),
				Name:       resolved.Name,
				Skill:      resolved.Skill,
				Task:       resolved.Task,
				Tool:       resolved.Tool,
				ToolInput:  resolved.ToolInput,
				Prompt:     resolved.Prompt,
				WhenStep:   resolved.WhenStep,
				WhenStatus: resolved.WhenStatus,
				OnFailure:  resolved.OnFailure,
				Status:     types.RunStatusSkipped,
				Output:     reason,
			})
			continue
		}

		switch resolved.Type {
		case "", "agent":
			run, err := runner.Run(ctx, types.Task{
				ID:          types.NewID("task"),
				Input:       resolved.Task,
				Skill:       resolved.Skill,
				RequestedAt: time.Now().UTC(),
			})
			if err != nil {
				result.Output = err.Error()
				result.StepRuns = append(result.StepRuns, WorkflowStepRun{
					Type:       "agent",
					Name:       resolved.Name,
					Skill:      resolved.Skill,
					Task:       resolved.Task,
					WhenStep:   resolved.WhenStep,
					WhenStatus: resolved.WhenStatus,
					OnFailure:  resolved.OnFailure,
					Status:     types.RunStatusFailed,
					Output:     err.Error(),
				})
				if resolved.OnFailure != "continue" {
					result.Status = types.RunStatusFailed
					result.FinishedAt = time.Now().UTC()
					return result, nil
				}
				continuedFailure = true
				continue
			}

			result.StepRuns = append(result.StepRuns, WorkflowStepRun{
				Type:       "agent",
				Name:       resolved.Name,
				Skill:      resolved.Skill,
				Task:       resolved.Task,
				WhenStep:   resolved.WhenStep,
				WhenStatus: resolved.WhenStatus,
				OnFailure:  resolved.OnFailure,
				Status:     run.Status,
				RunID:      run.RunID,
				Output:     run.Output,
			})
			if run.Status == types.RunStatusFailed {
				result.Output = run.Output
				if resolved.OnFailure != "continue" {
					result.Status = types.RunStatusFailed
					result.FinishedAt = time.Now().UTC()
					return result, nil
				}
				continuedFailure = true
				continue
			}
			continuedFailure = false
			result.Output = run.Output
		case "tool":
			output, err := runner.ExecuteTool(ctx, resolved.Tool, resolved.ToolInput)
			stepRun := WorkflowStepRun{
				Type:       "tool",
				Name:       resolved.Name,
				Tool:       resolved.Tool,
				ToolInput:  resolved.ToolInput,
				WhenStep:   resolved.WhenStep,
				WhenStatus: resolved.WhenStatus,
				OnFailure:  resolved.OnFailure,
				Status:     types.RunStatusCompleted,
				Output:     output,
			}
			if err != nil {
				stepRun.Status = types.RunStatusFailed
				stepRun.Output = err.Error()
				result.StepRuns = append(result.StepRuns, stepRun)
				result.Output = err.Error()
				if resolved.OnFailure != "continue" {
					result.Status = types.RunStatusFailed
					result.FinishedAt = time.Now().UTC()
					return result, nil
				}
				continuedFailure = true
				continue
			}
			result.StepRuns = append(result.StepRuns, stepRun)
			continuedFailure = false
			result.Output = output
		case "approval":
			req, err := resolveApproval(ctx, result.RunID, resolved.Name)
			if err != nil {
				result.Status = types.RunStatusFailed
				result.Output = err.Error()
				result.FinishedAt = time.Now().UTC()
				return result, nil
			}
			if req == nil {
				req, err = ensureApproval(ctx, result.RunID, result.WorkflowName, resolved)
				if err != nil {
					result.Status = types.RunStatusFailed
					result.Output = err.Error()
					result.FinishedAt = time.Now().UTC()
					return result, nil
				}
			}
			stepRun := approvalStepRun(resolved, req)
			result.StepRuns = append(result.StepRuns, stepRun)
			switch req.Status {
			case approval.StatusApproved:
				result.Output = stepRun.Output
				continue
			case approval.StatusRejected:
				result.Status = types.RunStatusFailed
				result.Output = stepRun.Output
				result.FinishedAt = time.Now().UTC()
				return result, nil
			default:
				result.Status = types.RunStatusWaitingApproval
				result.Output = stepRun.Output
				result.FinishedAt = time.Now().UTC()
				return result, nil
			}
		default:
			result.Status = types.RunStatusFailed
			result.Output = fmt.Sprintf("unsupported workflow step type %q", resolved.Type)
			result.StepRuns = append(result.StepRuns, WorkflowStepRun{
				Type:       resolved.Type,
				Name:       resolved.Name,
				WhenStep:   resolved.WhenStep,
				WhenStatus: resolved.WhenStatus,
				OnFailure:  resolved.OnFailure,
				Status:     types.RunStatusFailed,
				Output:     result.Output,
			})
			if resolved.OnFailure != "continue" {
				result.FinishedAt = time.Now().UTC()
				return result, nil
			}
		}
	}

	if continuedFailure {
		result.Status = types.RunStatusFailed
	}
	result.FinishedAt = time.Now().UTC()
	return result, nil
}

func ToInfos(defs []Definition) []Info {
	infos := make([]Info, 0, len(defs))
	for _, def := range defs {
		infos = append(infos, Info{
			Name:        def.Name,
			Description: def.Description,
			StepCount:   len(def.Steps),
		})
	}
	return infos
}

func shouldSkipStep(step Step, previous []WorkflowStepRun) (bool, string) {
	if step.WhenStep == "" && step.WhenStatus == "" {
		return false, ""
	}
	for _, prior := range previous {
		if prior.Name != step.WhenStep {
			continue
		}
		if string(prior.Status) == step.WhenStatus {
			return false, ""
		}
		return true, fmt.Sprintf("skipped because step %q status is %q, expected %q", step.WhenStep, prior.Status, step.WhenStatus)
	}
	return true, fmt.Sprintf("skipped because dependency step %q was not found", step.WhenStep)
}

func defaultStepType(stepType string) string {
	if stepType == "" {
		return "agent"
	}
	return stepType
}
