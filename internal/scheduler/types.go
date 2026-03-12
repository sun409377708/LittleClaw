package scheduler

import (
	"time"

	"littleclaw/internal/types"
)

type Definition struct {
	Name           string `json:"name"`
	Description    string `json:"description"`
	Every          string `json:"every,omitempty"`
	Cron           string `json:"cron,omitempty"`
	Dispatch       string `json:"dispatch,omitempty"`
	Target         string `json:"target"`
	Skill          string `json:"skill,omitempty"`
	Task           string `json:"task,omitempty"`
	Workflow       string `json:"workflow,omitempty"`
	Concurrency    string `json:"concurrency,omitempty"`
	RunImmediately bool   `json:"run_immediately"`
	Enabled        bool   `json:"enabled"`
}

type Info struct {
	Name           string `json:"name"`
	Description    string `json:"description"`
	Every          string `json:"every,omitempty"`
	Cron           string `json:"cron,omitempty"`
	Dispatch       string `json:"dispatch"`
	Target         string `json:"target"`
	Concurrency    string `json:"concurrency"`
	RunImmediately bool   `json:"run_immediately"`
	Enabled        bool   `json:"enabled"`
}

type RunResult struct {
	RunID         string          `json:"run_id"`
	ScheduleName  string          `json:"schedule_name"`
	Status        types.RunStatus `json:"status"`
	StartedAt     time.Time       `json:"started_at"`
	FinishedAt    time.Time       `json:"finished_at"`
	Every         string          `json:"every,omitempty"`
	Cron          string          `json:"cron,omitempty"`
	Dispatch      string          `json:"dispatch,omitempty"`
	Target        string          `json:"target"`
	Skill         string          `json:"skill,omitempty"`
	Task          string          `json:"task,omitempty"`
	Workflow      string          `json:"workflow,omitempty"`
	Concurrency   string          `json:"concurrency,omitempty"`
	JobID         string          `json:"job_id,omitempty"`
	TaskRunID     string          `json:"task_run_id,omitempty"`
	WorkflowRunID string          `json:"workflow_run_id,omitempty"`
	Output        string          `json:"output"`
}
