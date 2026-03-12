package types

import (
	"fmt"
	"time"
)

type Task struct {
	ID          string    `json:"id"`
	Input       string    `json:"input"`
	Skill       string    `json:"skill"`
	RequestedAt time.Time `json:"requested_at"`
}

type Step struct {
	Index       int            `json:"index"`
	Thought     string         `json:"thought,omitempty"`
	ActionType  string         `json:"action_type"`
	ToolName    string         `json:"tool_name,omitempty"`
	ToolInput   map[string]any `json:"tool_input,omitempty"`
	Observation string         `json:"observation,omitempty"`
	Error       string         `json:"error,omitempty"`
	Timestamp   time.Time      `json:"timestamp"`
}

type RunStatus string

const (
	RunStatusCompleted       RunStatus = "completed"
	RunStatusFailed          RunStatus = "failed"
	RunStatusSkipped         RunStatus = "skipped"
	RunStatusWaitingApproval RunStatus = "waiting_approval"
)

type RunResult struct {
	RunID      string    `json:"run_id"`
	TaskID     string    `json:"task_id"`
	Status     RunStatus `json:"status"`
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at"`
	Steps      []Step    `json:"steps"`
	Output     string    `json:"output"`
}

func NewID(prefix string) string {
	return fmt.Sprintf("%s_%d", prefix, time.Now().UTC().UnixNano())
}
