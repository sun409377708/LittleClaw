package workflow

import (
	"time"

	"littleclaw/internal/types"
)

type Definition struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Steps       []Step `json:"steps"`
}

type Step struct {
	Type       string         `json:"type"`
	Name       string         `json:"name"`
	Skill      string         `json:"skill,omitempty"`
	Task       string         `json:"task,omitempty"`
	Tool       string         `json:"tool,omitempty"`
	Prompt     string         `json:"prompt,omitempty"`
	InputJSON  string         `json:"input_json,omitempty"`
	ToolInput  map[string]any `json:"tool_input,omitempty"`
	WhenStep   string         `json:"when_step,omitempty"`
	WhenStatus string         `json:"when_status,omitempty"`
	OnFailure  string         `json:"on_failure,omitempty"`
}

type Info struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	StepCount   int    `json:"step_count"`
}

type RunResult struct {
	RunID        string            `json:"run_id"`
	WorkflowName string            `json:"workflow_name"`
	Status       types.RunStatus   `json:"status"`
	StartedAt    time.Time         `json:"started_at"`
	FinishedAt   time.Time         `json:"finished_at"`
	StepRuns     []WorkflowStepRun `json:"step_runs"`
	Output       string            `json:"output"`
}

type WorkflowStepRun struct {
	Type       string          `json:"type"`
	Name       string          `json:"name"`
	Skill      string          `json:"skill,omitempty"`
	Task       string          `json:"task,omitempty"`
	Tool       string          `json:"tool,omitempty"`
	ToolInput  map[string]any  `json:"tool_input,omitempty"`
	ApprovalID string          `json:"approval_id,omitempty"`
	Prompt     string          `json:"prompt,omitempty"`
	WhenStep   string          `json:"when_step,omitempty"`
	WhenStatus string          `json:"when_status,omitempty"`
	OnFailure  string          `json:"on_failure,omitempty"`
	Status     types.RunStatus `json:"status"`
	RunID      string          `json:"run_id,omitempty"`
	Output     string          `json:"output"`
}
