package workflow

import (
	"context"
	"fmt"

	"littleclaw/internal/approval"
	"littleclaw/internal/config"
	"littleclaw/internal/types"
)

func approvalsStore(ctx context.Context) *approval.Store {
	if cfg, ok := config.FromContext(ctx); ok && cfg != nil {
		return approval.NewStore(cfg.Approvals.Dir)
	}
	return approval.NewStore("approvals")
}

func ensureApproval(ctx context.Context, runID, workflowName string, step Step) (*approval.Request, error) {
	store := approvalsStore(ctx)
	return store.Ensure(runID, workflowName, step.Name, step.Prompt)
}

func resolveApproval(ctx context.Context, runID, stepName string) (*approval.Request, error) {
	store := approvalsStore(ctx)
	return store.FindByRunAndStep(runID, stepName)
}

func approvalStepRun(step Step, req *approval.Request) WorkflowStepRun {
	stepRun := WorkflowStepRun{
		Type:       "approval",
		Name:       step.Name,
		Prompt:     step.Prompt,
		WhenStep:   step.WhenStep,
		WhenStatus: step.WhenStatus,
		OnFailure:  step.OnFailure,
		Status:     types.RunStatusWaitingApproval,
		Output:     fmt.Sprintf("approval required: %s", step.Prompt),
	}
	if req != nil {
		stepRun.ApprovalID = req.ID
		switch req.Status {
		case approval.StatusApproved:
			stepRun.Status = types.RunStatusCompleted
			stepRun.Output = "approval approved"
		case approval.StatusRejected:
			stepRun.Status = types.RunStatusFailed
			stepRun.Output = "approval rejected"
		default:
			stepRun.Status = types.RunStatusWaitingApproval
		}
	}
	return stepRun
}
