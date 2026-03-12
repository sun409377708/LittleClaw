package app

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"littleclaw/internal/api"
	"littleclaw/internal/approval"
	"littleclaw/internal/config"
	"littleclaw/internal/llm"
	"littleclaw/internal/logger"
	"littleclaw/internal/memory"
	"littleclaw/internal/queue"
	"littleclaw/internal/runtime"
	"littleclaw/internal/scheduler"
	"littleclaw/internal/types"
	"littleclaw/internal/workflow"
)

func Run(args []string) error {
	if len(args) == 0 {
		printUsage()
		return nil
	}

	switch args[0] {
	case "run":
		return runCommand(args[1:])
	case "workflow":
		return workflowCommand(args[1:])
	case "workflows":
		return workflowsCommand(args[1:])
	case "approval":
		return approvalCommand(args[1:])
	case "approvals":
		return approvalsCommand(args[1:])
	case "schedule":
		return scheduleCommand(args[1:])
	case "schedules":
		return schedulesCommand(args[1:])
	case "api":
		return apiCommand(args[1:])
	case "queue":
		return queueCommand(args[1:])
	case "worker":
		return workerCommand(args[1:])
	case "memory":
		return memoryCommand(args[1:])
	case "memories":
		return memoriesCommand(args[1:])
	case "skills":
		return skillsCommand(args[1:])
	case "tools":
		return toolsCommand(args[1:])
	case "doctor":
		return doctorCommand(args[1:])
	case "runs":
		return runsCommand(args[1:])
	case "inspect":
		return inspectCommand(args[1:])
	case "replay":
		return replayCommand(args[1:])
	case "help", "--help", "-h":
		printUsage()
		return nil
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func apiCommand(args []string) error {
	if len(args) == 0 {
		return errors.New(`usage: littleclaw api serve ...`)
	}
	switch args[0] {
	case "serve":
		return apiServeCommand(args[1:])
	default:
		return fmt.Errorf("unknown api subcommand %q", args[0])
	}
}

func apiServeCommand(args []string) error {
	fs := flag.NewFlagSet("api serve", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	configPath := fs.String("config", os.Getenv("NANOCLAW_CONFIG"), "Path to a config file")
	listen := fs.String("listen", "", "HTTP listen address, defaults to config")
	readTimeout := fs.Duration("read-timeout", 0, "HTTP read timeout, defaults to config")
	writeTimeout := fs.Duration("write-timeout", 0, "HTTP write timeout, defaults to config")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New(`usage: littleclaw api serve [--config path] [--listen :8080] [--read-timeout 15s] [--write-timeout 30s]`)
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}
	if *listen == "" {
		*listen = cfg.API.Listen
	}
	if *readTimeout <= 0 {
		*readTimeout, err = time.ParseDuration(cfg.API.ReadTimeout)
		if err != nil {
			return fmt.Errorf("invalid api.read_timeout: %w", err)
		}
	}
	if *writeTimeout <= 0 {
		*writeTimeout, err = time.ParseDuration(cfg.API.WriteTimeout)
		if err != nil {
			return fmt.Errorf("invalid api.write_timeout: %w", err)
		}
	}

	log := logger.New(cfg.App.LogLevel)
	server, err := api.NewServer(cfg, log)
	if err != nil {
		return err
	}

	httpServer := &http.Server{
		Addr:         *listen,
		Handler:      server.Handler(),
		ReadTimeout:  *readTimeout,
		WriteTimeout: *writeTimeout,
	}
	log.Info("http api listening", "addr", *listen)
	return httpServer.ListenAndServe()
}

func queueCommand(args []string) error {
	if len(args) == 0 {
		return errors.New(`usage: littleclaw queue <submit|jobs|inspect> ...`)
	}
	switch args[0] {
	case "submit":
		return queueSubmitCommand(args[1:])
	case "jobs":
		return queueJobsCommand(args[1:])
	case "dead":
		return queueDeadCommand(args[1:])
	case "inspect":
		return queueInspectCommand(args[1:])
	default:
		return fmt.Errorf("unknown queue subcommand %q", args[0])
	}
}

func workerCommand(args []string) error {
	if len(args) == 0 {
		return errors.New(`usage: littleclaw worker <serve> ...`)
	}
	switch args[0] {
	case "serve":
		return workerServeCommand(args[1:])
	default:
		return fmt.Errorf("unknown worker subcommand %q", args[0])
	}
}

func queueSubmitCommand(args []string) error {
	fs := flag.NewFlagSet("queue submit", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	configPath := fs.String("config", os.Getenv("NANOCLAW_CONFIG"), "Path to a config file")
	task := fs.String("task", "", "Task text for a queued task job")
	workflowName := fs.String("workflow", "", "Workflow name for a queued workflow job")
	skill := fs.String("skill", "default", "Skill for queued task jobs")
	plannerMode := fs.String("planner", "auto", "Planner mode to record with the queued job")
	priority := fs.Int("priority", 0, "Queue priority, higher runs first")
	maxAttempts := fs.Int("max-attempts", 1, "Maximum worker attempts before the job stops being claimable")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New(`usage: littleclaw queue submit [--config path] (--task "..." | --workflow name) [--skill default] [--planner auto|rule|llm] [--priority 0] [--max-attempts 1]`)
	}
	if strings.TrimSpace(*task) == "" && strings.TrimSpace(*workflowName) == "" {
		return errors.New("queue submit requires either --task or --workflow")
	}
	if strings.TrimSpace(*task) != "" && strings.TrimSpace(*workflowName) != "" {
		return errors.New("queue submit accepts either --task or --workflow, not both")
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}
	store, err := queue.NewStoreFromConfig(cfg)
	if err != nil {
		return err
	}

	target := queue.JobTargetTask
	if strings.TrimSpace(*workflowName) != "" {
		target = queue.JobTargetWorkflow
	}
	job, err := store.Submit(queue.SubmitRequest{
		Target:      target,
		Task:        *task,
		Workflow:    *workflowName,
		Skill:       *skill,
		Planner:     *plannerMode,
		Priority:    *priority,
		MaxAttempts: *maxAttempts,
	})
	if err != nil {
		return err
	}
	payload, err := json.MarshalIndent(job, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal queue job: %w", err)
	}
	fmt.Println(string(payload))
	return nil
}

func queueJobsCommand(args []string) error {
	fs := flag.NewFlagSet("queue jobs", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	configPath := fs.String("config", os.Getenv("NANOCLAW_CONFIG"), "Path to a config file")
	limit := fs.Int("limit", 20, "Maximum number of queue jobs to print")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New(`usage: littleclaw queue jobs [--config path] [--limit 20]`)
	}
	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}
	store, err := queue.NewStoreFromConfig(cfg)
	if err != nil {
		return err
	}
	jobs, err := store.List(*limit)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Println("No queue jobs found.")
			return nil
		}
		return err
	}
	if len(jobs) == 0 {
		fmt.Println("No queue jobs found.")
		return nil
	}
	for _, job := range jobs {
		subject := job.Task
		if job.Target == queue.JobTargetWorkflow {
			subject = job.Workflow
		}
		next := "-"
		if job.NextAttemptAt != nil {
			next = job.NextAttemptAt.Format(time.RFC3339)
		}
		fmt.Printf("%s\t%s\t%s\t%d/%d\t%s\t%s\n", job.ID, job.Status, job.Target, job.Attempts, job.MaxAttempts, next, subject)
	}
	return nil
}

func queueDeadCommand(args []string) error {
	fs := flag.NewFlagSet("queue dead", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	configPath := fs.String("config", os.Getenv("NANOCLAW_CONFIG"), "Path to a config file")
	limit := fs.Int("limit", 20, "Maximum number of dead-letter jobs to print")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New(`usage: littleclaw queue dead [--config path] [--limit 20]`)
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}
	store, err := queue.NewStoreFromConfig(cfg)
	if err != nil {
		return err
	}
	paths, err := store.ListDeadLetters(*limit)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Println("No dead-letter jobs found.")
			return nil
		}
		return err
	}
	if len(paths) == 0 {
		fmt.Println("No dead-letter jobs found.")
		return nil
	}
	for _, path := range paths {
		fmt.Println(path)
	}
	return nil
}

func queueInspectCommand(args []string) error {
	fs := flag.NewFlagSet("queue inspect", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	configPath := fs.String("config", os.Getenv("NANOCLAW_CONFIG"), "Path to a config file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New(`usage: littleclaw queue inspect <job-id> [--config path]`)
	}
	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}
	store, err := queue.NewStoreFromConfig(cfg)
	if err != nil {
		return err
	}
	job, err := store.Get(fs.Arg(0))
	if err != nil {
		return err
	}
	payload, err := json.MarshalIndent(job, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal queue job: %w", err)
	}
	fmt.Println(string(payload))
	return nil
}

func workerServeCommand(args []string) error {
	fs := flag.NewFlagSet("worker serve", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	configPath := fs.String("config", os.Getenv("NANOCLAW_CONFIG"), "Path to a config file")
	plannerMode := fs.String("planner", "auto", "Planner mode for task jobs without an explicit planner")
	duration := fs.Duration("duration", 30*time.Second, "How long to keep the worker loop running")
	timeout := fs.Duration("timeout", 5*time.Minute, "Maximum runtime per claimed job")
	poll := fs.Duration("poll", 0, "How often to poll the queue; defaults to config")
	concurrency := fs.Int("concurrency", 0, "Number of in-process workers; defaults to config")
	lockPath := fs.String("lock-file", "", "Lock file used to keep a single worker serve instance")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New(`usage: littleclaw worker serve [--config path] [--planner auto|rule|llm] [--duration 30s] [--timeout 5m] [--poll 1s] [--concurrency 1]`)
	}
	if *duration <= 0 {
		return errors.New("duration must be greater than zero")
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}
	if *poll <= 0 {
		if *poll, err = time.ParseDuration(cfg.Queue.Poll); err != nil {
			return fmt.Errorf("invalid queue poll duration: %w", err)
		}
	}
	if *concurrency <= 0 {
		*concurrency = cfg.Queue.Concurrency
	}
	if *concurrency <= 0 {
		*concurrency = 1
	}
	if *lockPath == "" {
		*lockPath = cfg.Queue.LockFile
	}

	lock, err := queue.AcquireWorkerLock(*lockPath)
	if err != nil {
		return err
	}
	defer func() {
		_ = lock.Release()
	}()

	log := logger.New(cfg.App.LogLevel)
	store, err := queue.NewStoreFromConfig(cfg)
	if err != nil {
		return err
	}
	baseCtx := config.WithConfig(context.Background(), cfg)
	deadline := time.Now().Add(*duration)
	workerID := types.NewID("worker")

	type workerEvent struct {
		JobID   string `json:"job_id"`
		Status  string `json:"status"`
		Target  string `json:"target"`
		Message string `json:"message,omitempty"`
	}

	events := make([]workerEvent, 0)
	var eventsMu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, *concurrency)

	for time.Now().Before(deadline) {
		select {
		case sem <- struct{}{}:
			job, err := store.ClaimNext(workerID)
			if err != nil {
				<-sem
				return err
			}
			if job == nil {
				<-sem
				time.Sleep(*poll)
				continue
			}

			jobPlanner := *plannerMode
			if strings.TrimSpace(job.Planner) != "" {
				jobPlanner = job.Planner
			}
			engine := runtime.NewWithConfig(runtime.Options{
				Logger:  log,
				Timeout: *timeout,
			}, cfg, runtime.PlannerMode(jobPlanner))

			wg.Add(1)
			go func(job *queue.Job, engine *runtime.Runtime) {
				defer wg.Done()
				defer func() { <-sem }()

				runCtx, cancel := context.WithTimeout(baseCtx, *timeout)
				updated, err := queue.ExecuteJob(runCtx, store, engine, cfg.Workflows.Dir, cfg.Memory.Dir, workerID, job)
				cancel()

				eventsMu.Lock()
				defer eventsMu.Unlock()
				event := workerEvent{
					JobID:  job.ID,
					Status: "completed",
					Target: string(job.Target),
				}
				if err != nil {
					event.Status = "failed"
					event.Message = err.Error()
				} else if updated != nil {
					switch updated.Status {
					case queue.JobStatusPending:
						event.Status = "retried"
						if updated.NextAttemptAt != nil {
							event.Message = "next attempt at " + updated.NextAttemptAt.Format(time.RFC3339)
						}
					case queue.JobStatusFailed:
						event.Status = "failed"
						event.Message = updated.LastError
					case queue.JobStatusCompleted:
						event.Status = "completed"
					}
				}
				events = append(events, event)
			}(job, engine)
		default:
			time.Sleep(*poll)
		}
	}

	wg.Wait()
	payload, err := json.MarshalIndent(map[string]any{
		"worker_id":   workerID,
		"concurrency": *concurrency,
		"served_for":  duration.String(),
		"events":      events,
	}, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal worker serve result: %w", err)
	}
	fmt.Println(string(payload))
	return nil
}

func memoriesCommand(args []string) error {
	fs := flag.NewFlagSet("memories", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	configPath := fs.String("config", os.Getenv("NANOCLAW_CONFIG"), "Path to a config file")
	query := fs.String("query", "", "Search query for memory summaries")
	limit := fs.Int("limit", 10, "Maximum number of memories to print")
	kind := fs.String("kind", "", "Filter by memory kind: task, workflow, schedule")
	status := fs.String("status", "", "Filter by memory status")
	tagsRaw := fs.String("tags", "", "Comma-separated tags that must all be present")
	sinceRaw := fs.String("since", "", "Finished-after filter, RFC3339 or relative duration ago like 24h")
	untilRaw := fs.String("until", "", "Finished-before filter, RFC3339 or relative duration ago like 1h")
	fieldExprs := make([]string, 0)
	fs.Func("field", "Field filter key=value, supports top-level fields and metadata.<key>", func(value string) error {
		fieldExprs = append(fieldExprs, value)
		return nil
	})
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New(`usage: littleclaw memories [--config path] [--query text] [--limit 10] [--kind task|workflow|schedule] [--status completed] [--tags a,b] [--since 24h] [--until 1h] [--field metadata.skill=coder]`)
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}

	since, err := parseTimeFilter(*sinceRaw, time.Now())
	if err != nil {
		return err
	}
	until, err := parseTimeFilter(*untilRaw, time.Now())
	if err != nil {
		return err
	}
	fieldFilters, err := parseFieldFilters(fieldExprs)
	if err != nil {
		return err
	}

	matches, err := memory.SearchWithOptions(cfg.Memory.Dir, memory.SearchOptions{
		Query:     *query,
		Limit:     *limit,
		Kind:      *kind,
		Status:    *status,
		Tags:      splitCSV(*tagsRaw),
		Since:     since,
		Until:     until,
		FieldExpr: fieldFilters,
	})
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Println("No memories found.")
			return nil
		}
		return err
	}
	if len(matches) == 0 {
		fmt.Println("No memories found.")
		return nil
	}
	payload, err := json.MarshalIndent(matches, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal memories: %w", err)
	}
	fmt.Println(string(payload))
	return nil
}

func memoryCommand(args []string) error {
	if len(args) == 0 {
		return errors.New(`usage: littleclaw memory <inspect> ...`)
	}
	switch args[0] {
	case "inspect":
		return memoryInspectCommand(args[1:])
	default:
		return fmt.Errorf("unknown memory subcommand %q", args[0])
	}
}

func memoryInspectCommand(args []string) error {
	fs := flag.NewFlagSet("memory inspect", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	configPath := fs.String("config", os.Getenv("NANOCLAW_CONFIG"), "Path to a config file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New(`usage: littleclaw memory inspect <memory-id|memory-file> [--config path]`)
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}
	entry, err := memory.LoadEntry(normalizeMemoryPath(cfg.Memory.Dir, fs.Arg(0)))
	if err != nil {
		return err
	}
	payload, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal memory entry: %w", err)
	}
	fmt.Println(string(payload))
	return nil
}

func runCommand(args []string) error {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	configPath := fs.String("config", os.Getenv("NANOCLAW_CONFIG"), "Path to a config file")
	skill := fs.String("skill", "default", "Skill name to use")
	plannerMode := fs.String("planner", "auto", "Planner mode: rule, llm, or auto")
	maxSteps := fs.Int("max-steps", 8, "Maximum reasoning steps per run")
	timeout := fs.Duration("timeout", 2*time.Minute, "Maximum runtime duration")

	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New(`usage: littleclaw run [--config path] [--skill default] [--max-steps 8] [--timeout 2m] "<task>"`)
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}

	ctx := config.WithConfig(context.Background(), cfg)
	log := logger.New(cfg.App.LogLevel)

	runCtx, cancel := context.WithTimeout(ctx, *timeout)
	defer cancel()

	engine := runtime.NewWithConfig(runtime.Options{
		Logger:   log,
		MaxSteps: *maxSteps,
		Timeout:  *timeout,
	}, cfg, runtime.PlannerMode(*plannerMode))

	result, err := engine.Run(runCtx, types.Task{
		ID:          types.NewID("task"),
		Input:       fs.Arg(0),
		Skill:       *skill,
		RequestedAt: time.Now().UTC(),
	})
	if err != nil {
		return err
	}

	payload, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal result: %w", err)
	}

	fmt.Println(string(payload))
	return nil
}

func doctorCommand(args []string) error {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	configPath := fs.String("config", os.Getenv("NANOCLAW_CONFIG"), "Path to a config file")
	plannerMode := fs.String("planner", "auto", "Planner mode: rule, llm, or auto")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New(`usage: littleclaw doctor [--config path] [--planner auto|rule|llm]`)
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}

	status := llm.Inspect(cfg, string(runtime.PlannerMode(*plannerMode)))
	payload, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal doctor status: %w", err)
	}
	fmt.Println(string(payload))
	return nil
}

func workflowsCommand(args []string) error {
	fs := flag.NewFlagSet("workflows", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	configPath := fs.String("config", os.Getenv("NANOCLAW_CONFIG"), "Path to a config file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New(`usage: littleclaw workflows [--config path]`)
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}

	defs, err := workflow.LoadAll(cfg.Workflows.Dir)
	if err != nil {
		return err
	}
	payload, err := json.MarshalIndent(workflow.ToInfos(defs), "", "  ")
	if err != nil {
		return fmt.Errorf("marshal workflow infos: %w", err)
	}
	fmt.Println(string(payload))
	return nil
}

func workflowCommand(args []string) error {
	if len(args) == 0 {
		return errors.New(`usage: littleclaw workflow <run|resume|runs|inspect|replay> ...`)
	}

	switch args[0] {
	case "run":
		return workflowRunCommand(args[1:])
	case "resume":
		return workflowResumeCommand(args[1:])
	case "runs":
		return workflowRunsCommand(args[1:])
	case "inspect":
		return workflowInspectCommand(args[1:])
	case "replay":
		return workflowReplayCommand(args[1:])
	default:
		return fmt.Errorf("unknown workflow subcommand %q", args[0])
	}
}

func approvalsCommand(args []string) error {
	fs := flag.NewFlagSet("approvals", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	configPath := fs.String("config", os.Getenv("NANOCLAW_CONFIG"), "Path to a config file")
	limit := fs.Int("limit", 20, "Maximum number of approvals to print")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New(`usage: littleclaw approvals [--config path] [--limit 20]`)
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}
	store := approval.NewStore(cfg.Approvals.Dir)
	requests, err := store.List(*limit)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Println("No approvals found.")
			return nil
		}
		return err
	}
	if len(requests) == 0 {
		fmt.Println("No approvals found.")
		return nil
	}
	payload, err := json.MarshalIndent(requests, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal approvals: %w", err)
	}
	fmt.Println(string(payload))
	return nil
}

func approvalCommand(args []string) error {
	if len(args) == 0 {
		return errors.New(`usage: littleclaw approval <inspect|approve|reject> ...`)
	}

	switch args[0] {
	case "inspect":
		return approvalInspectCommand(args[1:])
	case "approve":
		return approvalApproveCommand(args[1:])
	case "reject":
		return approvalRejectCommand(args[1:])
	default:
		return fmt.Errorf("unknown approval subcommand %q", args[0])
	}
}

func schedulesCommand(args []string) error {
	fs := flag.NewFlagSet("schedules", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	configPath := fs.String("config", os.Getenv("NANOCLAW_CONFIG"), "Path to a config file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New(`usage: littleclaw schedules [--config path]`)
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}

	defs, err := scheduler.LoadAll(cfg.Schedules.Dir)
	if err != nil {
		return err
	}
	payload, err := json.MarshalIndent(scheduler.ToInfos(defs), "", "  ")
	if err != nil {
		return fmt.Errorf("marshal schedule infos: %w", err)
	}
	fmt.Println(string(payload))
	return nil
}

func scheduleCommand(args []string) error {
	if len(args) == 0 {
		return errors.New(`usage: littleclaw schedule <run|serve|runs|inspect|replay> ...`)
	}

	switch args[0] {
	case "run":
		return scheduleRunCommand(args[1:])
	case "serve":
		return scheduleServeCommand(args[1:])
	case "runs":
		return scheduleRunsCommand(args[1:])
	case "inspect":
		return scheduleInspectCommand(args[1:])
	case "replay":
		return scheduleReplayCommand(args[1:])
	default:
		return fmt.Errorf("unknown schedule subcommand %q", args[0])
	}
}

func workflowRunCommand(args []string) error {
	args = normalizeNamedRunArgs(args)

	fs := flag.NewFlagSet("workflow run", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	configPath := fs.String("config", os.Getenv("NANOCLAW_CONFIG"), "Path to a config file")
	plannerMode := fs.String("planner", "auto", "Planner mode: rule, llm, or auto")
	timeout := fs.Duration("timeout", 5*time.Minute, "Maximum workflow runtime duration")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New(`usage: littleclaw workflow run <name> [--config path] [--planner auto|rule|llm] [--timeout 5m]`)
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}
	def, err := workflow.LoadByName(cfg.Workflows.Dir, fs.Arg(0))
	if err != nil {
		return err
	}

	log := logger.New(cfg.App.LogLevel)
	runCtx, cancel := context.WithTimeout(config.WithConfig(context.Background(), cfg), *timeout)
	defer cancel()

	engine := runtime.NewWithConfig(runtime.Options{
		Logger:  log,
		Timeout: *timeout,
	}, cfg, runtime.PlannerMode(*plannerMode))

	result, err := workflow.Execute(runCtx, engine, def)
	if err != nil {
		return err
	}
	if err := workflow.PersistRunWithMemoryDir(result, cfg.Memory.Dir); err != nil {
		return err
	}
	payload, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal workflow result: %w", err)
	}
	fmt.Println(string(payload))
	return nil
}

func workflowResumeCommand(args []string) error {
	args = normalizeNamedRunArgs(args)

	fs := flag.NewFlagSet("workflow resume", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	configPath := fs.String("config", os.Getenv("NANOCLAW_CONFIG"), "Path to a config file")
	plannerMode := fs.String("planner", "auto", "Planner mode: rule, llm, or auto")
	timeout := fs.Duration("timeout", 5*time.Minute, "Maximum workflow runtime duration")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New(`usage: littleclaw workflow resume <run-id|run-file> [--config path] [--planner auto|rule|llm] [--timeout 5m]`)
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}
	existing, err := workflow.LoadRun(normalizeWorkflowRunPath(fs.Arg(0)))
	if err != nil {
		return err
	}
	def, err := workflow.LoadByName(cfg.Workflows.Dir, existing.WorkflowName)
	if err != nil {
		return err
	}

	log := logger.New(cfg.App.LogLevel)
	runCtx, cancel := context.WithTimeout(config.WithConfig(context.Background(), cfg), *timeout)
	defer cancel()

	engine := runtime.NewWithConfig(runtime.Options{
		Logger:  log,
		Timeout: *timeout,
	}, cfg, runtime.PlannerMode(*plannerMode))

	result, err := workflow.Resume(runCtx, engine, def, existing)
	if err != nil {
		return err
	}
	if err := workflow.PersistRunWithMemoryDir(result, cfg.Memory.Dir); err != nil {
		return err
	}
	payload, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal resumed workflow result: %w", err)
	}
	fmt.Println(string(payload))
	return nil
}

func scheduleRunCommand(args []string) error {
	args = normalizeNamedRunArgs(args)

	fs := flag.NewFlagSet("schedule run", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	configPath := fs.String("config", os.Getenv("NANOCLAW_CONFIG"), "Path to a config file")
	plannerMode := fs.String("planner", "auto", "Planner mode: rule, llm, or auto")
	timeout := fs.Duration("timeout", 5*time.Minute, "Maximum schedule runtime duration")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New(`usage: littleclaw schedule run <name> [--config path] [--planner auto|rule|llm] [--timeout 5m]`)
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}
	def, err := scheduler.LoadByName(cfg.Schedules.Dir, fs.Arg(0))
	if err != nil {
		return err
	}

	log := logger.New(cfg.App.LogLevel)
	baseCtx := config.WithConfig(context.Background(), cfg)
	runCtx, cancel := context.WithTimeout(baseCtx, *timeout)
	defer cancel()

	var result *scheduler.RunResult
	if def.Dispatch == "queue" {
		store, err := queue.NewStoreFromConfig(cfg)
		if err != nil {
			return err
		}
		result, err = scheduler.Enqueue(store, def, *plannerMode)
	} else {
		engine := runtime.NewWithConfig(runtime.Options{
			Logger:  log,
			Timeout: *timeout,
		}, cfg, runtime.PlannerMode(*plannerMode))
		result, err = scheduler.Execute(runCtx, engine, cfg.Workflows.Dir, cfg.Memory.Dir, def)
	}
	if err != nil {
		return err
	}
	if err := scheduler.PersistRunWithMemoryDir(result, cfg.Memory.Dir); err != nil {
		return err
	}
	payload, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal schedule result: %w", err)
	}
	fmt.Println(string(payload))
	return nil
}

func scheduleServeCommand(args []string) error {
	fs := flag.NewFlagSet("schedule serve", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	configPath := fs.String("config", os.Getenv("NANOCLAW_CONFIG"), "Path to a config file")
	plannerMode := fs.String("planner", "auto", "Planner mode: rule, llm, or auto")
	timeout := fs.Duration("timeout", 5*time.Minute, "Maximum runtime per triggered schedule")
	duration := fs.Duration("duration", 30*time.Second, "How long to keep the scheduler loop running")
	tick := fs.Duration("tick", 1*time.Second, "How often to poll for due schedules")
	lockPath := fs.String("lock-file", "", "Lock file used to keep a single scheduler serve instance")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New(`usage: littleclaw schedule serve [--config path] [--planner auto|rule|llm] [--timeout 5m] [--duration 30s] [--tick 1s]`)
	}
	if *duration <= 0 {
		return errors.New("duration must be greater than zero")
	}
	if *tick <= 0 {
		return errors.New("tick must be greater than zero")
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}
	defs, err := scheduler.LoadAll(cfg.Schedules.Dir)
	if err != nil {
		return err
	}
	if len(defs) == 0 {
		fmt.Println("No schedules found.")
		return nil
	}
	if *lockPath == "" {
		*lockPath = cfg.Schedules.LockFile
	}

	lock, err := scheduler.AcquireServeLock(*lockPath)
	if err != nil {
		return err
	}
	defer func() {
		_ = lock.Release()
	}()

	log := logger.New(cfg.App.LogLevel)
	engine := runtime.NewWithConfig(runtime.Options{
		Logger:  log,
		Timeout: *timeout,
	}, cfg, runtime.PlannerMode(*plannerMode))
	queueStore, err := queue.NewStoreFromConfig(cfg)
	if err != nil {
		return err
	}

	type scheduleState struct {
		Definition scheduler.Definition
		mu         sync.Mutex
		Running    int
		NextRun    time.Time
	}

	states := make([]scheduleState, 0, len(defs))
	now := time.Now()
	for _, def := range defs {
		nextRun, err := scheduler.InitialNextRun(def, now)
		if err != nil {
			return fmt.Errorf("schedule %q is invalid: %w", def.Name, err)
		}
		states = append(states, scheduleState{
			Definition: def,
			NextRun:    nextRun,
		})
	}

	type serveRun struct {
		ScheduleName string `json:"schedule_name"`
		RunID        string `json:"run_id"`
		Status       string `json:"status"`
	}

	type skippedRun struct {
		ScheduleName string `json:"schedule_name"`
		Reason       string `json:"reason"`
	}

	triggered := make([]serveRun, 0)
	skipped := make([]skippedRun, 0)
	var triggeredMu sync.Mutex
	var wg sync.WaitGroup
	baseCtx := config.WithConfig(context.Background(), cfg)
	deadline := time.Now().Add(*duration)

	for {
		now = time.Now()
		for i := range states {
			state := &states[i]
			state.mu.Lock()
			if !state.Definition.Enabled || state.NextRun.After(now) {
				state.mu.Unlock()
				continue
			}
			dueAt := state.NextRun
			nextRun, err := scheduler.NextRunAfter(state.Definition, dueAt)
			if err != nil {
				state.mu.Unlock()
				return fmt.Errorf("schedule %q next run calculation failed: %w", state.Definition.Name, err)
			}
			state.NextRun = nextRun

			if state.Running > 0 && state.Definition.Concurrency == "skip" {
				state.mu.Unlock()
				triggeredMu.Lock()
				skipped = append(skipped, skippedRun{
					ScheduleName: state.Definition.Name,
					Reason:       "skipped because a previous run is still active",
				})
				triggeredMu.Unlock()
				continue
			}

			state.Running++
			state.mu.Unlock()
			definition := state.Definition
			wg.Add(1)
			go func(state *scheduleState, definition scheduler.Definition) {
				defer wg.Done()
				var (
					result  *scheduler.RunResult
					execErr error
				)
				if definition.Dispatch == "queue" {
					result, execErr = scheduler.Enqueue(queueStore, definition, *plannerMode)
				} else {
					runCtx, cancel := context.WithTimeout(baseCtx, *timeout)
					result, execErr = scheduler.Execute(runCtx, engine, cfg.Workflows.Dir, cfg.Memory.Dir, definition)
					cancel()
				}

				triggeredMu.Lock()
				defer triggeredMu.Unlock()
				state.mu.Lock()
				state.Running--
				state.mu.Unlock()
				if execErr != nil {
					skipped = append(skipped, skippedRun{
						ScheduleName: definition.Name,
						Reason:       execErr.Error(),
					})
					return
				}
				if err := scheduler.PersistRunWithMemoryDir(result, cfg.Memory.Dir); err != nil {
					skipped = append(skipped, skippedRun{
						ScheduleName: definition.Name,
						Reason:       err.Error(),
					})
					return
				}
				triggered = append(triggered, serveRun{
					ScheduleName: definition.Name,
					RunID:        result.RunID,
					Status:       string(result.Status),
				})
			}(state, definition)
		}

		if !time.Now().Before(deadline) {
			break
		}

		select {
		case <-time.After(*tick):
		case <-baseCtx.Done():
			return baseCtx.Err()
		}
	}
	wg.Wait()

	payload, err := json.MarshalIndent(map[string]any{
		"served_for": duration.String(),
		"count":      len(triggered),
		"skipped":    skipped,
		"triggered":  triggered,
	}, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal scheduler serve result: %w", err)
	}
	fmt.Println(string(payload))
	return nil
}

func workflowRunsCommand(args []string) error {
	fs := flag.NewFlagSet("workflow runs", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	limit := fs.Int("limit", 20, "Maximum number of workflow runs to print")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New(`usage: littleclaw workflow runs [--limit 20]`)
	}

	paths, err := workflow.ListRuns()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Println("No workflow runs found.")
			return nil
		}
		return err
	}
	if len(paths) == 0 {
		fmt.Println("No workflow runs found.")
		return nil
	}

	max := *limit
	if max <= 0 || max > len(paths) {
		max = len(paths)
	}
	for _, path := range paths[:max] {
		run, err := workflow.LoadRun(path)
		if err != nil {
			return err
		}
		fmt.Printf("%s\t%s\t%s\t%s\t%s\n", run.RunID, run.WorkflowName, run.Status, run.StartedAt.Format(time.RFC3339), path)
	}
	return nil
}

func workflowInspectCommand(args []string) error {
	fs := flag.NewFlagSet("workflow inspect", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New(`usage: littleclaw workflow inspect <run-id|run-file>`)
	}

	run, err := workflow.LoadRun(normalizeWorkflowRunPath(fs.Arg(0)))
	if err != nil {
		return err
	}

	payload, err := json.MarshalIndent(run, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal workflow run: %w", err)
	}
	fmt.Println(string(payload))
	return nil
}

func workflowReplayCommand(args []string) error {
	fs := flag.NewFlagSet("workflow replay", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New(`usage: littleclaw workflow replay <run-id|run-file>`)
	}

	run, err := workflow.LoadRun(normalizeWorkflowRunPath(fs.Arg(0)))
	if err != nil {
		return err
	}

	fmt.Printf("Workflow %s [%s] (%s)\n", run.WorkflowName, run.RunID, run.Status)
	for i, step := range run.StepRuns {
		fmt.Printf("Step %d [%s] %s\n", i+1, step.Type, step.Name)
		if step.WhenStep != "" {
			fmt.Printf("  when: %s == %s\n", step.WhenStep, step.WhenStatus)
		}
		if step.OnFailure != "" {
			fmt.Printf("  on_failure: %s\n", step.OnFailure)
		}
		if step.Skill != "" {
			fmt.Printf("  skill: %s\n", step.Skill)
		}
		if step.Task != "" {
			fmt.Printf("  task: %s\n", step.Task)
		}
		if step.Tool != "" {
			fmt.Printf("  tool: %s\n", step.Tool)
		}
		if len(step.ToolInput) > 0 {
			inputPayload, _ := json.Marshal(step.ToolInput)
			fmt.Printf("  input: %s\n", string(inputPayload))
		}
		if step.Prompt != "" {
			fmt.Printf("  prompt: %s\n", step.Prompt)
		}
		if step.ApprovalID != "" {
			fmt.Printf("  approval_id: %s\n", step.ApprovalID)
		}
		fmt.Printf("  status: %s\n", step.Status)
		if step.RunID != "" {
			fmt.Printf("  run_id: %s\n", step.RunID)
		}
		if step.Output != "" {
			fmt.Printf("  output: %s\n", step.Output)
		}
	}
	fmt.Printf("Output: %s\n", run.Output)
	return nil
}

func approvalInspectCommand(args []string) error {
	args = normalizeNamedRunArgs(args)

	fs := flag.NewFlagSet("approval inspect", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	configPath := fs.String("config", os.Getenv("NANOCLAW_CONFIG"), "Path to a config file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New(`usage: littleclaw approval inspect <approval-id|approval-file> [--config path]`)
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}
	store := approval.NewStore(cfg.Approvals.Dir)
	req, err := store.Get(normalizeApprovalPath(cfg.Approvals.Dir, fs.Arg(0)))
	if err != nil {
		return err
	}
	payload, err := json.MarshalIndent(req, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal approval request: %w", err)
	}
	fmt.Println(string(payload))
	return nil
}

func approvalApproveCommand(args []string) error {
	return approvalDecideCommand("approval approve", args, approval.StatusApproved)
}

func approvalRejectCommand(args []string) error {
	return approvalDecideCommand("approval reject", args, approval.StatusRejected)
}

func approvalDecideCommand(name string, args []string, status approval.Status) error {
	args = normalizeNamedRunArgs(args)

	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	configPath := fs.String("config", os.Getenv("NANOCLAW_CONFIG"), "Path to a config file")
	comment := fs.String("comment", "", "Optional approval comment")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		if status == approval.StatusApproved {
			return errors.New(`usage: littleclaw approval approve <approval-id|approval-file> [--comment "..."] [--config path]`)
		}
		return errors.New(`usage: littleclaw approval reject <approval-id|approval-file> [--comment "..."] [--config path]`)
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}
	store := approval.NewStore(cfg.Approvals.Dir)
	req, err := store.Decide(normalizeApprovalPath(cfg.Approvals.Dir, fs.Arg(0)), status, *comment)
	if err != nil {
		return err
	}
	payload, err := json.MarshalIndent(req, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal approval request: %w", err)
	}
	fmt.Println(string(payload))
	return nil
}

func scheduleRunsCommand(args []string) error {
	fs := flag.NewFlagSet("schedule runs", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	limit := fs.Int("limit", 20, "Maximum number of schedule runs to print")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New(`usage: littleclaw schedule runs [--limit 20]`)
	}

	paths, err := scheduler.ListRuns()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Println("No schedule runs found.")
			return nil
		}
		return err
	}
	if len(paths) == 0 {
		fmt.Println("No schedule runs found.")
		return nil
	}

	max := *limit
	if max <= 0 || max > len(paths) {
		max = len(paths)
	}
	for _, path := range paths[:max] {
		run, err := scheduler.LoadRun(path)
		if err != nil {
			return err
		}
		fmt.Printf("%s\t%s\t%s\t%s\t%s\n", run.RunID, run.ScheduleName, run.Status, run.StartedAt.Format(time.RFC3339), path)
	}
	return nil
}

func scheduleInspectCommand(args []string) error {
	fs := flag.NewFlagSet("schedule inspect", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New(`usage: littleclaw schedule inspect <run-id|run-file>`)
	}

	run, err := scheduler.LoadRun(normalizeScheduleRunPath(fs.Arg(0)))
	if err != nil {
		return err
	}
	payload, err := json.MarshalIndent(run, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal schedule run: %w", err)
	}
	fmt.Println(string(payload))
	return nil
}

func scheduleReplayCommand(args []string) error {
	fs := flag.NewFlagSet("schedule replay", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New(`usage: littleclaw schedule replay <run-id|run-file>`)
	}

	run, err := scheduler.LoadRun(normalizeScheduleRunPath(fs.Arg(0)))
	if err != nil {
		return err
	}

	fmt.Printf("Schedule %s [%s] (%s)\n", run.ScheduleName, run.RunID, run.Status)
	fmt.Printf("  every: %s\n", run.Every)
	if run.Cron != "" {
		fmt.Printf("  cron: %s\n", run.Cron)
	}
	if run.Dispatch != "" {
		fmt.Printf("  dispatch: %s\n", run.Dispatch)
	}
	fmt.Printf("  target: %s\n", run.Target)
	if run.Concurrency != "" {
		fmt.Printf("  concurrency: %s\n", run.Concurrency)
	}
	if run.Skill != "" {
		fmt.Printf("  skill: %s\n", run.Skill)
	}
	if run.Task != "" {
		fmt.Printf("  task: %s\n", run.Task)
	}
	if run.Workflow != "" {
		fmt.Printf("  workflow: %s\n", run.Workflow)
	}
	if run.JobID != "" {
		fmt.Printf("  job_id: %s\n", run.JobID)
	}
	if run.TaskRunID != "" {
		fmt.Printf("  task_run_id: %s\n", run.TaskRunID)
	}
	if run.WorkflowRunID != "" {
		fmt.Printf("  workflow_run_id: %s\n", run.WorkflowRunID)
	}
	fmt.Printf("Output: %s\n", run.Output)
	return nil
}

func normalizeNamedRunArgs(args []string) []string {
	if len(args) == 0 {
		return args
	}

	reordered := make([]string, 0, len(args))
	positional := ""
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "-") {
			reordered = append(reordered, arg)
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				reordered = append(reordered, args[i+1])
				i++
			}
			continue
		}
		if positional == "" {
			positional = arg
			continue
		}
		reordered = append(reordered, arg)
	}
	if positional != "" {
		reordered = append(reordered, positional)
	}
	return reordered
}

func toolsCommand(args []string) error {
	fs := flag.NewFlagSet("tools", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	configPath := fs.String("config", os.Getenv("NANOCLAW_CONFIG"), "Path to a config file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New(`usage: littleclaw tools [--config path]`)
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}

	engine := runtime.NewWithConfig(runtime.Options{}, cfg, "")
	payload, err := json.MarshalIndent(engine.ToolInfos(), "", "  ")
	if err != nil {
		return fmt.Errorf("marshal tool infos: %w", err)
	}
	fmt.Println(string(payload))
	return nil
}

func skillsCommand(args []string) error {
	fs := flag.NewFlagSet("skills", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	configPath := fs.String("config", os.Getenv("NANOCLAW_CONFIG"), "Path to a config file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New(`usage: littleclaw skills [--config path]`)
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}

	engine := runtime.NewWithConfig(runtime.Options{}, cfg, "")
	payload, err := json.MarshalIndent(engine.SkillInfos(), "", "  ")
	if err != nil {
		return fmt.Errorf("marshal skill infos: %w", err)
	}
	fmt.Println(string(payload))
	return nil
}

func inspectCommand(args []string) error {
	fs := flag.NewFlagSet("inspect", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New(`usage: littleclaw inspect <run-id|run-file>`)
	}

	run, err := runtime.LoadRun(normalizeRunPath(fs.Arg(0)))
	if err != nil {
		return err
	}

	payload, err := json.MarshalIndent(run, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal run: %w", err)
	}
	fmt.Println(string(payload))
	return nil
}

func runsCommand(args []string) error {
	fs := flag.NewFlagSet("runs", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	limit := fs.Int("limit", 20, "Maximum number of runs to print")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New(`usage: littleclaw runs [--limit 20]`)
	}

	paths, err := runtime.ListRuns("runs")
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Println("No runs found.")
			return nil
		}
		return err
	}
	if len(paths) == 0 {
		fmt.Println("No runs found.")
		return nil
	}

	max := *limit
	if max <= 0 || max > len(paths) {
		max = len(paths)
	}
	for _, path := range paths[:max] {
		run, err := runtime.LoadRun(path)
		if err != nil {
			return err
		}
		fmt.Printf("%s\t%s\t%s\t%s\n", run.RunID, run.Status, run.StartedAt.Format(time.RFC3339), path)
	}
	return nil
}

func replayCommand(args []string) error {
	fs := flag.NewFlagSet("replay", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New(`usage: littleclaw replay <run-id|run-file>`)
	}

	run, err := runtime.LoadRun(normalizeRunPath(fs.Arg(0)))
	if err != nil {
		return err
	}

	fmt.Printf("Replay run %s (%s)\n", run.RunID, run.Status)
	for _, step := range run.Steps {
		fmt.Printf("Step %d [%s]\n", step.Index, step.ActionType)
		if step.Thought != "" {
			fmt.Printf("  thought: %s\n", step.Thought)
		}
		if step.ToolName != "" {
			fmt.Printf("  tool: %s\n", step.ToolName)
		}
		if len(step.ToolInput) > 0 {
			inputPayload, _ := json.Marshal(step.ToolInput)
			fmt.Printf("  input: %s\n", string(inputPayload))
		}
		if step.Observation != "" {
			fmt.Printf("  observation: %s\n", step.Observation)
		}
		if step.Error != "" {
			fmt.Printf("  error: %s\n", step.Error)
		}
	}
	fmt.Printf("Output: %s\n", run.Output)
	return nil
}

func normalizeRunPath(arg string) string {
	if filepath.Ext(arg) == ".json" {
		return arg
	}
	return filepath.Join("runs", arg+".json")
}

func normalizeWorkflowRunPath(arg string) string {
	if filepath.Ext(arg) == ".json" {
		return arg
	}
	return filepath.Join("workflow_runs", arg+".json")
}

func normalizeScheduleRunPath(arg string) string {
	if filepath.Ext(arg) == ".json" {
		return arg
	}
	return filepath.Join("schedule_runs", arg+".json")
}

func normalizeMemoryPath(dir, arg string) string {
	if filepath.Ext(arg) == ".json" {
		return arg
	}
	return filepath.Join(dir, arg+".json")
}

func normalizeApprovalPath(dir, arg string) string {
	if filepath.Ext(arg) == ".json" {
		return arg
	}
	return filepath.Join(dir, arg+".json")
}

func splitCSV(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value == "" {
			continue
		}
		values = append(values, value)
	}
	return values
}

func parseFieldFilters(values []string) (map[string]string, error) {
	if len(values) == 0 {
		return nil, nil
	}
	filters := make(map[string]string, len(values))
	for _, value := range values {
		parts := strings.SplitN(value, "=", 2)
		if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" {
			return nil, fmt.Errorf("invalid field filter %q, expected key=value", value)
		}
		filters[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
	}
	return filters, nil
}

func parseTimeFilter(raw string, now time.Time) (*time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	if ts, err := time.Parse(time.RFC3339, raw); err == nil {
		return &ts, nil
	}
	if duration, err := time.ParseDuration(raw); err == nil {
		ts := now.Add(-duration)
		return &ts, nil
	}
	return nil, fmt.Errorf("invalid time filter %q, use RFC3339 or relative duration like 24h", raw)
}

func printUsage() {
	fmt.Println("LittleClaw is a minimal AI agent runtime")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  littleclaw run [flags] \"task description\"")
	fmt.Println("  littleclaw workflows [--config path]")
	fmt.Println("  littleclaw workflow run <name> [--config path] [--planner auto|rule|llm] [--timeout 5m]")
	fmt.Println("  littleclaw workflow resume <run-id|run-file> [--config path] [--planner auto|rule|llm] [--timeout 5m]")
	fmt.Println("  littleclaw workflow runs [--limit 20]")
	fmt.Println("  littleclaw workflow inspect <run-id|run-file>")
	fmt.Println("  littleclaw workflow replay <run-id|run-file>")
	fmt.Println("  littleclaw approvals [--config path] [--limit 20]")
	fmt.Println("  littleclaw approval inspect <approval-id|approval-file> [--config path]")
	fmt.Println("  littleclaw approval approve <approval-id|approval-file> [--comment \"...\"] [--config path]")
	fmt.Println("  littleclaw approval reject <approval-id|approval-file> [--comment \"...\"] [--config path]")
	fmt.Println("  littleclaw schedules [--config path]")
	fmt.Println("  littleclaw schedule run <name> [--config path] [--planner auto|rule|llm] [--timeout 5m]")
	fmt.Println("  littleclaw schedule serve [--config path] [--planner auto|rule|llm] [--timeout 5m] [--duration 30s] [--tick 1s]")
	fmt.Println("  littleclaw schedule runs [--limit 20]")
	fmt.Println("  littleclaw schedule inspect <run-id|run-file>")
	fmt.Println("  littleclaw schedule replay <run-id|run-file>")
	fmt.Println("  littleclaw api serve [--config path] [--listen :8080] [--read-timeout 15s] [--write-timeout 30s]")
	fmt.Println("  littleclaw queue submit [--config path] (--task \"...\" | --workflow name) [--skill default] [--planner auto|rule|llm] [--priority 0] [--max-attempts 1]")
	fmt.Println("  littleclaw queue jobs [--config path] [--limit 20]")
	fmt.Println("  littleclaw queue dead [--config path] [--limit 20]")
	fmt.Println("  littleclaw queue inspect <job-id> [--config path]")
	fmt.Println("  littleclaw worker serve [--config path] [--planner auto|rule|llm] [--duration 30s] [--timeout 5m] [--poll 1s] [--concurrency 1]")
	fmt.Println("  littleclaw memories [--config path] [--query text] [--limit 10] [--kind task|workflow|schedule] [--status completed] [--tags a,b] [--since 24h] [--until 1h] [--field metadata.skill=coder]")
	fmt.Println("  littleclaw memory inspect <memory-id|memory-file> [--config path]")
	fmt.Println("  littleclaw skills [--config path]")
	fmt.Println("  littleclaw tools [--config path]")
	fmt.Println("  littleclaw doctor [--config path] [--planner auto|rule|llm]")
	fmt.Println("  littleclaw runs [--limit 20]")
	fmt.Println("  littleclaw inspect <run-id|run-file>")
	fmt.Println("  littleclaw replay <run-id|run-file>")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  run      Run a task with the LittleClaw runtime")
	fmt.Println("  workflows  List available workflows")
	fmt.Println("  workflow   Run a named workflow")
	fmt.Println("  approvals  List pending and decided approvals")
	fmt.Println("  approval   Inspect or decide an approval request")
	fmt.Println("  schedules  List available schedules")
	fmt.Println("  schedule   Run or serve named schedules")
	fmt.Println("  api       Serve the local HTTP API")
	fmt.Println("  queue     Submit, inspect, and review queued jobs")
	fmt.Println("  worker    Run the local queue worker loop")
	fmt.Println("  memories   List or search long-term memories")
	fmt.Println("  memory     Inspect a memory entry")
	fmt.Println("  skills   Show available skills and allowed tools")
	fmt.Println("  tools    Show available tools and input schemas")
	fmt.Println("  doctor   Show planner and LLM readiness")
	fmt.Println("  runs     List persisted runs")
	fmt.Println("  inspect  Print a persisted run as JSON")
	fmt.Println("  replay   Print a human-readable step replay")
}
