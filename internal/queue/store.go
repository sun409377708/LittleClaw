package queue

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"littleclaw/internal/types"
)

type Store struct {
	dir           string
	deadLetterDir string
	backoffBase   time.Duration
	backoffMax    time.Duration
	mu            sync.Mutex
}

func NewStore(dir string) *Store {
	return NewStoreWithOptions(Options{Dir: dir})
}

func NewStoreWithOptions(opts Options) *Store {
	if strings.TrimSpace(opts.Dir) == "" {
		opts.Dir = "queue_jobs"
	}
	if strings.TrimSpace(opts.DeadLetterDir) == "" {
		opts.DeadLetterDir = "queue_dead"
	}
	if opts.BackoffBase <= 0 {
		opts.BackoffBase = 2 * time.Second
	}
	if opts.BackoffMax <= 0 {
		opts.BackoffMax = 30 * time.Second
	}
	if opts.BackoffMax < opts.BackoffBase {
		opts.BackoffMax = opts.BackoffBase
	}
	return &Store{
		dir:           opts.Dir,
		deadLetterDir: opts.DeadLetterDir,
		backoffBase:   opts.BackoffBase,
		backoffMax:    opts.BackoffMax,
	}
}

func (s *Store) Submit(req SubmitRequest) (*Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if req.Target == "" {
		req.Target = JobTargetTask
	}
	if req.Skill == "" {
		req.Skill = "default"
	}
	if req.MaxAttempts <= 0 {
		req.MaxAttempts = 1
	}
	if req.Target == JobTargetTask && strings.TrimSpace(req.Task) == "" {
		return nil, fmt.Errorf("task job requires task")
	}
	if req.Target == JobTargetWorkflow && strings.TrimSpace(req.Workflow) == "" {
		return nil, fmt.Errorf("workflow job requires workflow")
	}

	now := time.Now().UTC()
	job := &Job{
		ID:            types.NewID("job"),
		Target:        req.Target,
		Task:          req.Task,
		Workflow:      req.Workflow,
		Skill:         req.Skill,
		Planner:       req.Planner,
		Status:        JobStatusPending,
		Priority:      req.Priority,
		MaxAttempts:   req.MaxAttempts,
		CreatedAt:     now,
		UpdatedAt:     now,
		NextAttemptAt: &now,
	}
	if err := s.writeJob(job); err != nil {
		return nil, err
	}
	return job, nil
}

func (s *Store) ClaimNext(workerID string) (*Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	jobs, err := s.loadJobs()
	if err != nil {
		return nil, err
	}
	candidates := make([]*Job, 0)
	for _, job := range jobs {
		if job.Status != JobStatusPending {
			continue
		}
		if job.Attempts >= job.MaxAttempts {
			continue
		}
		if job.NextAttemptAt != nil && job.NextAttemptAt.After(time.Now().UTC()) {
			continue
		}
		candidates = append(candidates, job)
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Priority == candidates[j].Priority {
			return candidates[i].CreatedAt.Before(candidates[j].CreatedAt)
		}
		return candidates[i].Priority > candidates[j].Priority
	})
	if len(candidates) == 0 {
		return nil, nil
	}

	job := candidates[0]
	now := time.Now().UTC()
	job.Status = JobStatusRunning
	job.Attempts++
	job.UpdatedAt = now
	job.StartedAt = &now
	job.NextAttemptAt = nil
	job.WorkerID = workerID
	job.LastError = ""
	if err := s.writeJob(job); err != nil {
		return nil, err
	}
	return job, nil
}

func (s *Store) Complete(jobID, output, taskRunID, workflowRunID string) (*Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, err := s.loadJobByID(jobID)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	job.Status = JobStatusCompleted
	job.Output = output
	job.TaskRunID = taskRunID
	job.WorkflowRunID = workflowRunID
	job.UpdatedAt = now
	job.FinishedAt = &now
	job.LastError = ""
	if err := s.writeJob(job); err != nil {
		return nil, err
	}
	return job, nil
}

func (s *Store) Fail(jobID, errText string) (*Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, err := s.loadJobByID(jobID)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	job.LastError = errText
	job.UpdatedAt = now
	job.Output = ""
	if job.Attempts < job.MaxAttempts {
		nextAttempt := now.Add(s.backoffForAttempt(job.Attempts))
		job.Status = JobStatusPending
		job.FinishedAt = nil
		job.NextAttemptAt = &nextAttempt
		job.WorkerID = ""
		if err := s.writeJob(job); err != nil {
			return nil, err
		}
		return job, nil
	}

	job.Status = JobStatusFailed
	job.FinishedAt = &now
	job.NextAttemptAt = nil
	if path, err := s.writeDeadLetter(job); err == nil {
		job.DeadLetteredAt = &now
		job.DeadLetterPath = path
	} else {
		job.LastError = fmt.Sprintf("%s (dead-letter write failed: %v)", errText, err)
	}
	if err := s.writeJob(job); err != nil {
		return nil, err
	}
	return job, nil
}

func (s *Store) Get(jobID string) (*Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadJobByID(jobID)
}

func (s *Store) List(limit int) ([]*Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	jobs, err := s.loadJobs()
	if err != nil {
		return nil, err
	}
	sort.SliceStable(jobs, func(i, j int) bool {
		return jobs[i].CreatedAt.After(jobs[j].CreatedAt)
	})
	if limit <= 0 || limit > len(jobs) {
		limit = len(jobs)
	}
	return jobs[:limit], nil
}

func (s *Store) loadJobs() ([]*Job, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read queue dir: %w", err)
	}
	jobs := make([]*Job, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		job, err := s.loadJobFromPath(filepath.Join(s.dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	return jobs, nil
}

func (s *Store) loadJobByID(jobID string) (*Job, error) {
	return s.loadJobFromPath(filepath.Join(s.dir, jobID+".json"))
}

func (s *Store) loadJobFromPath(path string) (*Job, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read queue job: %w", err)
	}
	var job Job
	if err := json.Unmarshal(data, &job); err != nil {
		return nil, fmt.Errorf("parse queue job: %w", err)
	}
	return &job, nil
}

func (s *Store) ListDeadLetters(limit int) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entries, err := os.ReadDir(s.deadLetterDir)
	if err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		paths = append(paths, filepath.Join(s.deadLetterDir, entry.Name()))
	}
	sort.Sort(sort.Reverse(sort.StringSlice(paths)))
	if limit <= 0 || limit > len(paths) {
		limit = len(paths)
	}
	return paths[:limit], nil
}

func (s *Store) writeDeadLetter(job *Job) (string, error) {
	if err := os.MkdirAll(s.deadLetterDir, 0o755); err != nil {
		return "", fmt.Errorf("create dead-letter dir: %w", err)
	}
	payload, err := json.MarshalIndent(job, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal dead-letter job: %w", err)
	}
	path := filepath.Join(s.deadLetterDir, job.ID+".json")
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		return "", fmt.Errorf("write dead-letter job: %w", err)
	}
	return path, nil
}

func (s *Store) backoffForAttempt(attempt int) time.Duration {
	if attempt <= 1 {
		return s.backoffBase
	}
	backoff := s.backoffBase
	for i := 1; i < attempt; i++ {
		backoff *= 2
		if backoff >= s.backoffMax {
			return s.backoffMax
		}
	}
	if backoff > s.backoffMax {
		return s.backoffMax
	}
	return backoff
}

func (s *Store) writeJob(job *Job) error {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return fmt.Errorf("create queue dir: %w", err)
	}
	payload, err := json.MarshalIndent(job, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal queue job: %w", err)
	}
	path := filepath.Join(s.dir, job.ID+".json")
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		return fmt.Errorf("write queue job: %w", err)
	}
	return nil
}
