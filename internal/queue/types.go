package queue

import "time"

type JobStatus string

const (
	JobStatusPending   JobStatus = "pending"
	JobStatusRunning   JobStatus = "running"
	JobStatusCompleted JobStatus = "completed"
	JobStatusFailed    JobStatus = "failed"
)

type JobTarget string

const (
	JobTargetTask     JobTarget = "task"
	JobTargetWorkflow JobTarget = "workflow"
)

type Job struct {
	ID             string     `json:"id"`
	Target         JobTarget  `json:"target"`
	Task           string     `json:"task,omitempty"`
	Workflow       string     `json:"workflow,omitempty"`
	Skill          string     `json:"skill,omitempty"`
	Planner        string     `json:"planner,omitempty"`
	Status         JobStatus  `json:"status"`
	Priority       int        `json:"priority"`
	MaxAttempts    int        `json:"max_attempts"`
	Attempts       int        `json:"attempts"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	StartedAt      *time.Time `json:"started_at,omitempty"`
	FinishedAt     *time.Time `json:"finished_at,omitempty"`
	NextAttemptAt  *time.Time `json:"next_attempt_at,omitempty"`
	DeadLetteredAt *time.Time `json:"dead_lettered_at,omitempty"`
	DeadLetterPath string     `json:"dead_letter_path,omitempty"`
	WorkerID       string     `json:"worker_id,omitempty"`
	TaskRunID      string     `json:"task_run_id,omitempty"`
	WorkflowRunID  string     `json:"workflow_run_id,omitempty"`
	LastError      string     `json:"last_error,omitempty"`
	Output         string     `json:"output,omitempty"`
}

type SubmitRequest struct {
	Target      JobTarget
	Task        string
	Workflow    string
	Skill       string
	Planner     string
	Priority    int
	MaxAttempts int
}
