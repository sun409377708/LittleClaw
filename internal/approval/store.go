package approval

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"littleclaw/internal/types"
)

type Status string

const (
	StatusPending  Status = "pending"
	StatusApproved Status = "approved"
	StatusRejected Status = "rejected"
)

type Request struct {
	ID            string     `json:"id"`
	WorkflowRunID string     `json:"workflow_run_id"`
	WorkflowName  string     `json:"workflow_name"`
	StepName      string     `json:"step_name"`
	Prompt        string     `json:"prompt"`
	Status        Status     `json:"status"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	DecidedAt     *time.Time `json:"decided_at,omitempty"`
	Comment       string     `json:"comment,omitempty"`
}

type Store struct {
	dir string
}

func NewStore(dir string) *Store {
	if strings.TrimSpace(dir) == "" {
		dir = "approvals"
	}
	return &Store{dir: dir}
}

func (s *Store) Ensure(runID, workflowName, stepName, prompt string) (*Request, error) {
	if existing, _ := s.FindByRunAndStep(runID, stepName); existing != nil {
		return existing, nil
	}
	now := time.Now().UTC()
	req := &Request{
		ID:            types.NewID("approval"),
		WorkflowRunID: runID,
		WorkflowName:  workflowName,
		StepName:      stepName,
		Prompt:        prompt,
		Status:        StatusPending,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := s.write(req); err != nil {
		return nil, err
	}
	return req, nil
}

func (s *Store) Get(id string) (*Request, error) {
	path := id
	if filepath.Ext(path) != ".json" {
		path = filepath.Join(s.dir, id+".json")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read approval request: %w", err)
	}
	var req Request
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, fmt.Errorf("parse approval request: %w", err)
	}
	return &req, nil
}

func (s *Store) FindByRunAndStep(runID, stepName string) (*Request, error) {
	requests, err := s.List(0)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	for _, req := range requests {
		if req.WorkflowRunID == runID && req.StepName == stepName {
			return req, nil
		}
	}
	return nil, nil
}

func (s *Store) Decide(id string, status Status, comment string) (*Request, error) {
	req, err := s.Get(id)
	if err != nil {
		return nil, err
	}
	if req.Status != StatusPending {
		return nil, fmt.Errorf("approval %s is already %s", id, req.Status)
	}
	now := time.Now().UTC()
	req.Status = status
	req.Comment = comment
	req.UpdatedAt = now
	req.DecidedAt = &now
	if err := s.write(req); err != nil {
		return nil, err
	}
	return req, nil
}

func (s *Store) List(limit int) ([]*Request, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		paths = append(paths, filepath.Join(s.dir, entry.Name()))
	}
	sort.Sort(sort.Reverse(sort.StringSlice(paths)))
	if limit > 0 && limit < len(paths) {
		paths = paths[:limit]
	}
	requests := make([]*Request, 0, len(paths))
	for _, path := range paths {
		req, err := s.Get(path)
		if err != nil {
			return nil, err
		}
		requests = append(requests, req)
	}
	return requests, nil
}

func (s *Store) write(req *Request) error {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return fmt.Errorf("create approvals dir: %w", err)
	}
	payload, err := json.MarshalIndent(req, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal approval request: %w", err)
	}
	path := filepath.Join(s.dir, req.ID+".json")
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		return fmt.Errorf("write approval request: %w", err)
	}
	return nil
}
