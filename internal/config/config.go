package config

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
)

type Config struct {
	App       AppConfig       `yaml:"app"`
	API       APIConfig       `yaml:"api"`
	LLM       LLMConfig       `yaml:"llm"`
	Approvals ApprovalsConfig `yaml:"approvals"`
	Memory    MemoryConfig    `yaml:"memory"`
	Queue     QueueConfig     `yaml:"queue"`
	Skills    SkillsConfig    `yaml:"skills"`
	Workflows WorkflowsConfig `yaml:"workflows"`
	Schedules SchedulesConfig `yaml:"schedules"`
}

type AppConfig struct {
	Name     string `yaml:"name"`
	LogLevel string `yaml:"log_level"`
}

type APIConfig struct {
	Listen       string `yaml:"listen"`
	ReadTimeout  string `yaml:"read_timeout"`
	WriteTimeout string `yaml:"write_timeout"`
}

type LLMConfig struct {
	PlannerMode string `yaml:"planner_mode"`
	Enabled     bool   `yaml:"enabled"`
	Provider    string `yaml:"provider"`
	BaseURL     string `yaml:"base_url"`
	Model       string `yaml:"model"`
	APIKeyEnv   string `yaml:"api_key_env"`
	HTTPTimeout string `yaml:"http_timeout"`
}

type ApprovalsConfig struct {
	Dir string `yaml:"dir"`
}

type MemoryConfig struct {
	Dir            string `yaml:"dir"`
	RetrievalLimit int    `yaml:"retrieval_limit"`
}

type QueueConfig struct {
	Dir           string `yaml:"dir"`
	DeadLetterDir string `yaml:"dead_letter_dir"`
	LockFile      string `yaml:"lock_file"`
	Poll          string `yaml:"poll"`
	BackoffBase   string `yaml:"backoff_base"`
	BackoffMax    string `yaml:"backoff_max"`
	Concurrency   int    `yaml:"concurrency"`
}

type SkillsConfig struct {
	Dir string `yaml:"dir"`
}

type WorkflowsConfig struct {
	Dir string `yaml:"dir"`
}

type SchedulesConfig struct {
	Dir      string `yaml:"dir"`
	LockFile string `yaml:"lock_file"`
}

func Load(path string) (*Config, error) {
	cfg := defaultConfig()
	if path == "" {
		return cfg, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	if err := parseSimpleYAML(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func defaultConfig() *Config {
	return &Config{
		App: AppConfig{
			Name:     "littleclaw",
			LogLevel: "info",
		},
		API: APIConfig{
			Listen:       ":8080",
			ReadTimeout:  "15s",
			WriteTimeout: "30s",
		},
		LLM: LLMConfig{
			PlannerMode: "auto",
			Enabled:     false,
			Provider:    "minimax",
			BaseURL:     "https://api.minimax.chat/v1/text/chatcompletion_v2",
			Model:       "MiniMax-M2.5",
			APIKeyEnv:   "MINIMAX_API_KEY",
			HTTPTimeout: "30s",
		},
		Approvals: ApprovalsConfig{
			Dir: "approvals",
		},
		Memory: MemoryConfig{
			Dir:            "memories",
			RetrievalLimit: 3,
		},
		Queue: QueueConfig{
			Dir:           "queue_jobs",
			DeadLetterDir: "queue_dead",
			LockFile:      ".littleclaw/worker.lock",
			Poll:          "1s",
			BackoffBase:   "2s",
			BackoffMax:    "30s",
			Concurrency:   1,
		},
		Skills: SkillsConfig{
			Dir: "skills",
		},
		Workflows: WorkflowsConfig{
			Dir: "workflows",
		},
		Schedules: SchedulesConfig{
			Dir:      "schedules",
			LockFile: ".littleclaw/scheduler.lock",
		},
	}
}

func (c *Config) Validate() error {
	if c == nil {
		return errors.New("config is nil")
	}
	if c.App.Name == "" {
		return errors.New("app.name must not be empty")
	}
	if c.App.LogLevel == "" {
		c.App.LogLevel = "info"
	}
	if c.API.Listen == "" {
		c.API.Listen = ":8080"
	}
	if c.API.ReadTimeout == "" {
		c.API.ReadTimeout = "15s"
	}
	if c.API.WriteTimeout == "" {
		c.API.WriteTimeout = "30s"
	}
	if c.LLM.Provider == "" {
		c.LLM.Provider = "minimax"
	}
	if c.LLM.PlannerMode == "" {
		c.LLM.PlannerMode = "auto"
	}
	if c.LLM.APIKeyEnv == "" {
		c.LLM.APIKeyEnv = "MINIMAX_API_KEY"
	}
	if c.LLM.HTTPTimeout == "" {
		c.LLM.HTTPTimeout = "30s"
	}
	if c.Approvals.Dir == "" {
		c.Approvals.Dir = "approvals"
	}
	if c.Memory.Dir == "" {
		c.Memory.Dir = "memories"
	}
	if c.Memory.RetrievalLimit <= 0 {
		c.Memory.RetrievalLimit = 3
	}
	if c.Queue.Dir == "" {
		c.Queue.Dir = "queue_jobs"
	}
	if c.Queue.DeadLetterDir == "" {
		c.Queue.DeadLetterDir = "queue_dead"
	}
	if c.Queue.LockFile == "" {
		c.Queue.LockFile = ".littleclaw/worker.lock"
	}
	if c.Queue.Poll == "" {
		c.Queue.Poll = "1s"
	}
	if c.Queue.BackoffBase == "" {
		c.Queue.BackoffBase = "2s"
	}
	if c.Queue.BackoffMax == "" {
		c.Queue.BackoffMax = "30s"
	}
	if c.Queue.Concurrency <= 0 {
		c.Queue.Concurrency = 1
	}
	if c.Skills.Dir == "" {
		c.Skills.Dir = "skills"
	}
	if c.Workflows.Dir == "" {
		c.Workflows.Dir = "workflows"
	}
	if c.Schedules.Dir == "" {
		c.Schedules.Dir = "schedules"
	}
	if c.Schedules.LockFile == "" {
		c.Schedules.LockFile = ".littleclaw/scheduler.lock"
	}
	return nil
}

type contextKey string

const configKey contextKey = "littleclaw-config"

func WithConfig(ctx context.Context, cfg *Config) context.Context {
	return context.WithValue(ctx, configKey, cfg)
}

func FromContext(ctx context.Context) (*Config, bool) {
	cfg, ok := ctx.Value(configKey).(*Config)
	return cfg, ok
}

func parseSimpleYAML(data []byte, cfg *Config) error {
	var section string

	for _, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasSuffix(line, ":") {
			section = strings.TrimSuffix(line, ":")
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid line %q", line)
		}

		key := strings.TrimSpace(parts[0])
		value := strings.Trim(strings.TrimSpace(parts[1]), `"'`)

		switch section + "." + key {
		case "app.name":
			cfg.App.Name = value
		case "app.log_level":
			cfg.App.LogLevel = value
		case "api.listen":
			cfg.API.Listen = value
		case "api.read_timeout":
			cfg.API.ReadTimeout = value
		case "api.write_timeout":
			cfg.API.WriteTimeout = value
		case "llm.enabled":
			cfg.LLM.Enabled = strings.EqualFold(value, "true")
		case "llm.planner_mode":
			cfg.LLM.PlannerMode = value
		case "llm.provider":
			cfg.LLM.Provider = value
		case "llm.base_url":
			cfg.LLM.BaseURL = value
		case "llm.model":
			cfg.LLM.Model = value
		case "llm.api_key_env":
			cfg.LLM.APIKeyEnv = value
		case "llm.http_timeout":
			cfg.LLM.HTTPTimeout = value
		case "approvals.dir":
			cfg.Approvals.Dir = value
		case "memory.dir":
			cfg.Memory.Dir = value
		case "memory.retrieval_limit":
			cfg.Memory.RetrievalLimit = atoiDefault(value, cfg.Memory.RetrievalLimit)
		case "queue.dir":
			cfg.Queue.Dir = value
		case "queue.dead_letter_dir":
			cfg.Queue.DeadLetterDir = value
		case "queue.lock_file":
			cfg.Queue.LockFile = value
		case "queue.poll":
			cfg.Queue.Poll = value
		case "queue.backoff_base":
			cfg.Queue.BackoffBase = value
		case "queue.backoff_max":
			cfg.Queue.BackoffMax = value
		case "queue.concurrency":
			cfg.Queue.Concurrency = atoiDefault(value, cfg.Queue.Concurrency)
		case "skills.dir":
			cfg.Skills.Dir = value
		case "workflows.dir":
			cfg.Workflows.Dir = value
		case "schedules.dir":
			cfg.Schedules.Dir = value
		case "schedules.lock_file":
			cfg.Schedules.LockFile = value
		default:
			return fmt.Errorf("unsupported key %q", section+"."+key)
		}
	}

	return nil
}

func atoiDefault(raw string, fallback int) int {
	value := strings.TrimSpace(raw)
	if value == "" {
		return fallback
	}
	var result int
	for _, ch := range value {
		if ch < '0' || ch > '9' {
			return fallback
		}
		result = result*10 + int(ch-'0')
	}
	if result <= 0 {
		return fallback
	}
	return result
}
