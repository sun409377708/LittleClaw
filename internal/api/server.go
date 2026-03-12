package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"littleclaw/internal/approval"
	"littleclaw/internal/config"
	"littleclaw/internal/queue"
	"littleclaw/internal/runtime"
	"littleclaw/internal/types"
	"littleclaw/internal/workflow"
)

type Server struct {
	cfg    *config.Config
	logger *slog.Logger
	mux    *http.ServeMux
}

func NewServer(cfg *config.Config, logger *slog.Logger) (*Server, error) {
	if cfg == nil {
		return nil, fmt.Errorf("api config is nil")
	}

	s := &Server{
		cfg:    cfg,
		logger: logger,
		mux:    http.NewServeMux(),
	}
	s.routes()
	return s, nil
}

func (s *Server) Handler() http.Handler {
	return s.mux
}

func (s *Server) routes() {
	s.mux.HandleFunc("/", s.handleUI)
	s.mux.HandleFunc("/healthz", s.handleHealth)
	s.mux.HandleFunc("/v1/skills", s.handleSkills)
	s.mux.HandleFunc("/v1/tools", s.handleTools)
	s.mux.HandleFunc("/v1/workflows", s.handleWorkflows)
	s.mux.HandleFunc("/v1/workflows/runs", s.handleWorkflowRuns)
	s.mux.HandleFunc("/v1/workflows/runs/", s.handleWorkflowRunByID)
	s.mux.HandleFunc("/v1/approvals", s.handleApprovals)
	s.mux.HandleFunc("/v1/approvals/", s.handleApprovalByID)
	s.mux.HandleFunc("/v1/runs", s.handleRuns)
	s.mux.HandleFunc("/v1/runs/", s.handleRunByID)
	s.mux.HandleFunc("/v1/queue/jobs", s.handleQueueJobs)
	s.mux.HandleFunc("/v1/queue/jobs/", s.handleQueueJobByID)
	s.mux.HandleFunc("/v1/queue/dead", s.handleQueueDead)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"time":   time.Now().UTC(),
	})
}

func (s *Server) handleSkills(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	engine := runtime.NewWithConfig(runtime.Options{}, s.cfg, "")
	writeJSON(w, http.StatusOK, engine.SkillInfos())
}

func (s *Server) handleTools(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	engine := runtime.NewWithConfig(runtime.Options{}, s.cfg, "")
	writeJSON(w, http.StatusOK, engine.ToolInfos())
}

func (s *Server) handleWorkflows(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	defs, err := workflow.LoadAll(s.cfg.Workflows.Dir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, workflow.ToInfos(defs))
}

type workflowRunRequest struct {
	Name    string `json:"name"`
	Planner string `json:"planner"`
	Timeout string `json:"timeout"`
}

func (s *Server) handleWorkflowRuns(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		limit := parseLimit(r.URL.Query().Get("limit"), 20)
		paths, err := workflow.ListRuns()
		if err != nil {
			if strings.Contains(err.Error(), "no such file or directory") {
				writeJSON(w, http.StatusOK, []any{})
				return
			}
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		if limit > len(paths) {
			limit = len(paths)
		}
		runs := make([]*workflow.RunResult, 0, limit)
		for _, path := range paths[:limit] {
			run, err := workflow.LoadRun(path)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err)
				return
			}
			runs = append(runs, run)
		}
		writeJSON(w, http.StatusOK, runs)
	case http.MethodPost:
		var req workflowRunRequest
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if strings.TrimSpace(req.Name) == "" {
			writeError(w, http.StatusBadRequest, fmt.Errorf("workflow name is required"))
			return
		}
		timeout, err := parseOptionalDuration(req.Timeout, 5*time.Minute)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		def, err := workflow.LoadByName(s.cfg.Workflows.Dir, req.Name)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		ctx, cancel := context.WithTimeout(config.WithConfig(r.Context(), s.cfg), timeout)
		defer cancel()
		engine := runtime.NewWithConfig(runtime.Options{
			Logger:  s.logger,
			Timeout: timeout,
		}, s.cfg, runtime.PlannerMode(req.Planner))
		result, err := workflow.Execute(ctx, engine, def)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		if err := workflow.PersistRunWithMemoryDir(result, s.cfg.Memory.Dir); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
	default:
		writeMethodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

func (s *Server) handleWorkflowRunByID(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/v1/workflows/runs/")
	path = strings.Trim(path, "/")
	if strings.TrimSpace(path) == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("workflow run id is required"))
		return
	}

	parts := strings.Split(path, "/")
	id := parts[0]
	runPath := "workflow_runs/" + id + ".json"

	switch {
	case len(parts) == 1 && r.Method == http.MethodGet:
		run, err := workflow.LoadRun(runPath)
		if err != nil {
			writeError(w, http.StatusNotFound, err)
			return
		}
		writeJSON(w, http.StatusOK, run)
	case len(parts) == 2 && parts[1] == "resume" && r.Method == http.MethodPost:
		var req workflowRunRequest
		if err := decodeOptionalJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		timeout, err := parseOptionalDuration(req.Timeout, 5*time.Minute)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		existing, err := workflow.LoadRun(runPath)
		if err != nil {
			writeError(w, http.StatusNotFound, err)
			return
		}
		def, err := workflow.LoadByName(s.cfg.Workflows.Dir, existing.WorkflowName)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		ctx, cancel := context.WithTimeout(config.WithConfig(r.Context(), s.cfg), timeout)
		defer cancel()
		engine := runtime.NewWithConfig(runtime.Options{
			Logger:  s.logger,
			Timeout: timeout,
		}, s.cfg, runtime.PlannerMode(req.Planner))
		result, err := workflow.Resume(ctx, engine, def, existing)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		if err := workflow.PersistRunWithMemoryDir(result, s.cfg.Memory.Dir); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
	default:
		if len(parts) == 1 {
			writeMethodNotAllowed(w, http.MethodGet)
			return
		}
		writeMethodNotAllowed(w, http.MethodPost)
	}
}

type approvalDecisionRequest struct {
	Comment string `json:"comment"`
}

func (s *Server) handleApprovals(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	store := approval.NewStore(s.cfg.Approvals.Dir)
	requests, err := store.List(parseLimit(r.URL.Query().Get("limit"), 20))
	if err != nil {
		if strings.Contains(err.Error(), "no such file or directory") {
			writeJSON(w, http.StatusOK, []any{})
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, requests)
}

func (s *Server) handleApprovalByID(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/v1/approvals/")
	path = strings.Trim(path, "/")
	if strings.TrimSpace(path) == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("approval id is required"))
		return
	}
	parts := strings.Split(path, "/")
	id := parts[0]
	store := approval.NewStore(s.cfg.Approvals.Dir)

	switch {
	case len(parts) == 1 && r.Method == http.MethodGet:
		req, err := store.Get(id)
		if err != nil {
			writeError(w, http.StatusNotFound, err)
			return
		}
		writeJSON(w, http.StatusOK, req)
	case len(parts) == 2 && r.Method == http.MethodPost && (parts[1] == "approve" || parts[1] == "reject"):
		var body approvalDecisionRequest
		if err := decodeOptionalJSON(r, &body); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		status := approval.StatusApproved
		if parts[1] == "reject" {
			status = approval.StatusRejected
		}
		req, err := store.Decide(id, status, body.Comment)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, req)
	default:
		if len(parts) == 1 {
			writeMethodNotAllowed(w, http.MethodGet)
			return
		}
		writeMethodNotAllowed(w, http.MethodPost)
	}
}

type runRequest struct {
	Task     string `json:"task"`
	Skill    string `json:"skill"`
	Planner  string `json:"planner"`
	MaxSteps int    `json:"max_steps"`
	Timeout  string `json:"timeout"`
}

func (s *Server) handleRuns(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		limit := parseLimit(r.URL.Query().Get("limit"), 20)
		paths, err := runtime.ListRuns("runs")
		if err != nil {
			if strings.Contains(err.Error(), "no such file or directory") {
				writeJSON(w, http.StatusOK, []any{})
				return
			}
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		if limit > len(paths) {
			limit = len(paths)
		}
		runs := make([]*types.RunResult, 0, limit)
		for _, path := range paths[:limit] {
			run, err := runtime.LoadRun(path)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err)
				return
			}
			runs = append(runs, run)
		}
		writeJSON(w, http.StatusOK, runs)
	case http.MethodPost:
		var req runRequest
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if strings.TrimSpace(req.Task) == "" {
			writeError(w, http.StatusBadRequest, fmt.Errorf("task is required"))
			return
		}
		if strings.TrimSpace(req.Skill) == "" {
			req.Skill = "default"
		}
		timeout, err := parseOptionalDuration(req.Timeout, 2*time.Minute)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		ctx, cancel := context.WithTimeout(config.WithConfig(r.Context(), s.cfg), timeout)
		defer cancel()
		engine := runtime.NewWithConfig(runtime.Options{
			Logger:   s.logger,
			MaxSteps: req.MaxSteps,
			Timeout:  timeout,
		}, s.cfg, runtime.PlannerMode(req.Planner))
		result, err := engine.Run(ctx, types.Task{
			ID:          types.NewID("task"),
			Input:       req.Task,
			Skill:       req.Skill,
			RequestedAt: time.Now().UTC(),
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
	default:
		writeMethodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

func (s *Server) handleRunByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/v1/runs/")
	if strings.TrimSpace(id) == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("run id is required"))
		return
	}
	run, err := runtime.LoadRun("runs/" + id + ".json")
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, run)
}

type queueSubmitRequest struct {
	Target      string `json:"target"`
	Task        string `json:"task"`
	Workflow    string `json:"workflow"`
	Skill       string `json:"skill"`
	Planner     string `json:"planner"`
	Priority    int    `json:"priority"`
	MaxAttempts int    `json:"max_attempts"`
}

func (s *Server) handleQueueJobs(w http.ResponseWriter, r *http.Request) {
	store, err := queue.NewStoreFromConfig(s.cfg)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	switch r.Method {
	case http.MethodGet:
		limit := parseLimit(r.URL.Query().Get("limit"), 20)
		jobs, err := store.List(limit)
		if err != nil {
			if strings.Contains(err.Error(), "no such file or directory") {
				writeJSON(w, http.StatusOK, []any{})
				return
			}
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, jobs)
	case http.MethodPost:
		var req queueSubmitRequest
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if strings.TrimSpace(req.Target) == "" {
			switch {
			case strings.TrimSpace(req.Workflow) != "":
				req.Target = string(queue.JobTargetWorkflow)
			case strings.TrimSpace(req.Task) != "":
				req.Target = string(queue.JobTargetTask)
			}
		}
		target := queue.JobTarget(req.Target)
		job, err := store.Submit(queue.SubmitRequest{
			Target:      target,
			Task:        req.Task,
			Workflow:    req.Workflow,
			Skill:       req.Skill,
			Planner:     req.Planner,
			Priority:    req.Priority,
			MaxAttempts: req.MaxAttempts,
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, job)
	default:
		writeMethodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

func (s *Server) handleQueueJobByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/v1/queue/jobs/")
	if strings.TrimSpace(id) == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("job id is required"))
		return
	}
	store, err := queue.NewStoreFromConfig(s.cfg)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	job, err := store.Get(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func (s *Server) handleQueueDead(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	store, err := queue.NewStoreFromConfig(s.cfg)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	limit := parseLimit(r.URL.Query().Get("limit"), 20)
	paths, err := store.ListDeadLetters(limit)
	if err != nil {
		if strings.Contains(err.Error(), "no such file or directory") {
			writeJSON(w, http.StatusOK, []any{})
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, paths)
}

func decodeJSON(r *http.Request, target any) error {
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode request JSON: %w", err)
	}
	return nil
}

func decodeOptionalJSON(r *http.Request, target any) error {
	defer r.Body.Close()
	if r.Body == nil {
		return nil
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		if err == io.EOF {
			return nil
		}
		return fmt.Errorf("decode request JSON: %w", err)
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]any{
		"error": err.Error(),
	})
}

func writeMethodNotAllowed(w http.ResponseWriter, methods ...string) {
	w.Header().Set("Allow", strings.Join(methods, ", "))
	writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
}

func parseOptionalDuration(raw string, fallback time.Duration) (time.Duration, error) {
	if strings.TrimSpace(raw) == "" {
		return fallback, nil
	}
	value, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid duration %q: %w", raw, err)
	}
	if value <= 0 {
		return 0, fmt.Errorf("duration must be greater than zero")
	}
	return value, nil
}

func parseLimit(raw string, fallback int) int {
	if strings.TrimSpace(raw) == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}
