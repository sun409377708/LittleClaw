package runtime

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"littleclaw/internal/config"
	"littleclaw/internal/llm"
	"littleclaw/internal/memory"
	"littleclaw/internal/planner"
	"littleclaw/internal/skills"
	"littleclaw/internal/tools"
	"littleclaw/internal/types"
)

type Options struct {
	Logger   *slog.Logger
	MaxSteps int
	Timeout  time.Duration
}

type Runtime struct {
	logger         *slog.Logger
	maxSteps       int
	timeout        time.Duration
	planner        planner.Planner
	skills         *skills.Registry
	tools          *tools.Registry
	memoryDir      string
	memoryRetrieve int
}

type PlannerMode string

const (
	PlannerModeAuto PlannerMode = "auto"
	PlannerModeRule PlannerMode = "rule"
	PlannerModeLLM  PlannerMode = "llm"
)

func New(opts Options) *Runtime {
	registry := tools.NewRegistry()
	registry.MustRegister(tools.NewShellTool())
	registry.MustRegister(tools.NewFileReadTool())
	registry.MustRegister(tools.NewFileWriteTool())
	registry.MustRegister(tools.NewFileAppendTool())
	registry.MustRegister(tools.NewFileListTool())
	registry.MustRegister(tools.NewHTTPGetTool())
	registry.MustRegister(tools.NewHTTPPostTool())

	return &Runtime{
		logger:         opts.Logger,
		maxSteps:       opts.MaxSteps,
		timeout:        opts.Timeout,
		planner:        planner.NewRuleBased(),
		skills:         skills.NewRegistry(),
		tools:          registry,
		memoryDir:      "memories",
		memoryRetrieve: 3,
	}
}

func (r *Runtime) ToolInfos() []tools.Info {
	return r.tools.Infos()
}

func (r *Runtime) SkillInfos() []skills.Info {
	return r.skills.Infos()
}

func (r *Runtime) ExecuteTool(ctx context.Context, name string, input map[string]any) (string, error) {
	return r.tools.Execute(ctx, name, input)
}

func NewWithConfig(opts Options, cfg *config.Config, mode PlannerMode) *Runtime {
	rt := New(opts)
	if cfg == nil {
		return rt
	}
	if registry, err := skills.NewRegistryFromDir(cfg.Skills.Dir); err == nil {
		rt.skills = registry
	} else if opts.Logger != nil {
		opts.Logger.Warn("skill registry load failed; using built-in skills",
			slog.String("dir", cfg.Skills.Dir),
			slog.String("error", err.Error()),
		)
	}
	rt.memoryDir = cfg.Memory.Dir
	rt.memoryRetrieve = cfg.Memory.RetrievalLimit
	if mode == "" {
		mode = PlannerMode(cfg.LLM.PlannerMode)
	}
	switch mode {
	case PlannerModeRule:
		return rt
	case PlannerModeAuto, PlannerModeLLM:
	default:
		if opts.Logger != nil {
			opts.Logger.Warn("invalid planner mode; using rule-based planner",
				slog.String("planner_mode", string(mode)),
			)
		}
		return rt
	}
	if !cfg.LLM.Enabled {
		if mode == PlannerModeLLM {
			if opts.Logger != nil {
				opts.Logger.Warn("llm planner requested but llm is disabled; using rule-based planner")
			}
		}
		return rt
	}

	timeout, err := time.ParseDuration(cfg.LLM.HTTPTimeout)
	if err != nil {
		timeout = 30 * time.Second
	}

	client, err := llm.NewMinimaxClient(llm.Config{
		Provider:    cfg.LLM.Provider,
		BaseURL:     cfg.LLM.BaseURL,
		Model:       cfg.LLM.Model,
		APIKeyEnv:   cfg.LLM.APIKeyEnv,
		Enabled:     cfg.LLM.Enabled,
		HTTPTimeout: timeout,
	})
	if err != nil {
		if mode == PlannerModeLLM {
			if opts.Logger != nil {
				opts.Logger.Warn("llm planner requested but initialization failed; using rule-based planner",
					slog.String("error", err.Error()),
				)
			}
			return rt
		}
		if opts.Logger != nil {
			opts.Logger.Warn("llm planner disabled; using rule-based planner",
				slog.String("error", err.Error()),
			)
		}
		return rt
	}

	rt.planner = planner.NewLLM(client)
	return rt
}

func (r *Runtime) Run(ctx context.Context, task types.Task) (*types.RunResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	store := memory.New()
	skill, err := r.skills.Resolve(task.Skill)
	if err != nil {
		return nil, err
	}
	run := &types.RunResult{
		RunID:     types.NewID("run"),
		TaskID:    task.ID,
		StartedAt: time.Now().UTC(),
		Status:    types.RunStatusCompleted,
	}
	relevantMemories := make([]planner.MemoryNote, 0, r.memoryRetrieve)
	if matches, err := memory.Search(r.memoryDir, task.Input, r.memoryRetrieve); err == nil {
		for _, match := range matches {
			relevantMemories = append(relevantMemories, planner.MemoryNote{
				Kind:    match.Entry.Kind,
				Subject: match.Entry.Subject,
				Summary: match.Entry.Summary,
				Status:  string(match.Entry.Status),
				Tags:    append([]string(nil), match.Entry.Tags...),
			})
		}
	} else if r.logger != nil && !strings.Contains(err.Error(), "no such file or directory") {
		r.logger.WarnContext(ctx, "memory search failed", slog.String("error", err.Error()))
	}

	maxSteps := r.maxSteps
	if maxSteps <= 0 {
		maxSteps = skill.MaxSteps
	}
	if skill.MaxSteps > 0 && (maxSteps == 0 || skill.MaxSteps < maxSteps) {
		maxSteps = skill.MaxSteps
	}
	if maxSteps <= 0 {
		maxSteps = 1
	}

	for stepIndex := 1; stepIndex <= maxSteps; stepIndex++ {
		shouldStop := false
		decision, err := r.planner.Decide(ctx, planner.State{
			Task:         task,
			Steps:        store.Steps(),
			AllowedTools: append([]string(nil), skill.AllowedTools...),
			SkillPrompt:  skill.SystemPrompt,
			Memories:     append([]planner.MemoryNote(nil), relevantMemories...),
		})
		if err != nil {
			run.Status = types.RunStatusFailed
			run.Output = fmt.Sprintf("planner error: %v", err)
			break
		}
		if err := decision.Validate(); err != nil {
			run.Status = types.RunStatusFailed
			run.Output = fmt.Sprintf("invalid planner decision: %v", err)
			store.Append(types.Step{
				Index:      stepIndex,
				Thought:    decision.Thought,
				ActionType: "planner_error",
				Error:      err.Error(),
				Timestamp:  time.Now().UTC(),
			})
			break
		}

		switch decision.Type {
		case planner.DecisionTool:
			if !containsTool(skill.AllowedTools, decision.ToolName) {
				run.Status = types.RunStatusFailed
				run.Output = fmt.Sprintf("tool %s is not allowed for skill %s", decision.ToolName, skill.Name)
				store.Append(types.Step{
					Index:      stepIndex,
					Thought:    decision.Thought,
					ActionType: "tool_denied",
					ToolName:   decision.ToolName,
					ToolInput:  decision.ToolInput,
					Error:      run.Output,
					Timestamp:  time.Now().UTC(),
				})
				shouldStop = true
				break
			}
			observation, execErr := r.tools.Execute(ctx, decision.ToolName, decision.ToolInput)
			step := types.Step{
				Index:       stepIndex,
				Thought:     decision.Thought,
				ActionType:  "tool",
				ToolName:    decision.ToolName,
				ToolInput:   decision.ToolInput,
				Observation: observation,
				Timestamp:   time.Now().UTC(),
			}
			if execErr != nil {
				step.Error = execErr.Error()
				run.Status = types.RunStatusFailed
				run.Output = fmt.Sprintf("tool %s failed: %v", decision.ToolName, execErr)
				shouldStop = true
			}
			store.Append(step)
		case planner.DecisionFinal:
			store.Append(types.Step{
				Index:      stepIndex,
				Thought:    decision.Thought,
				ActionType: "final",
				Timestamp:  time.Now().UTC(),
			})
			run.Output = decision.FinalOutput
			if run.Status != types.RunStatusFailed {
				run.Status = types.RunStatusCompleted
			}
			shouldStop = true
		default:
			run.Status = types.RunStatusFailed
			run.Output = fmt.Sprintf("unsupported decision type: %s", decision.Type)
			shouldStop = true
		}

		if shouldStop {
			break
		}
	}

	if run.Output == "" && run.Status == types.RunStatusCompleted {
		run.Output = fmt.Sprintf("task finished without explicit output: %s", task.Input)
	}

	run.Steps = store.Steps()
	run.FinishedAt = time.Now().UTC()
	if err := persistRun(run); err != nil && r.logger != nil {
		r.logger.WarnContext(ctx, "persist run failed",
			slog.String("run_id", run.RunID),
			slog.String("error", err.Error()),
		)
	}
	if err := memory.PersistEntry(r.memoryDir, buildTaskMemoryEntry(task, run)); err != nil && r.logger != nil {
		r.logger.WarnContext(ctx, "persist memory failed",
			slog.String("run_id", run.RunID),
			slog.String("error", err.Error()),
		)
	}

	if r.logger != nil {
		r.logger.InfoContext(ctx, "run completed",
			slog.String("run_id", run.RunID),
			slog.String("task_id", task.ID),
			slog.Int("steps", len(run.Steps)),
			slog.String("status", string(run.Status)),
		)
	}

	return run, nil
}

func containsTool(allowed []string, name string) bool {
	for _, candidate := range allowed {
		if candidate == name {
			return true
		}
	}
	return false
}
