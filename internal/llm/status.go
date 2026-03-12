package llm

import (
	"fmt"
	"os"
	"strings"

	"littleclaw/internal/config"
)

type Status struct {
	PlannerMode   string `json:"planner_mode"`
	Enabled       bool   `json:"enabled"`
	Provider      string `json:"provider"`
	Model         string `json:"model"`
	BaseURL       string `json:"base_url"`
	APIKeyEnv     string `json:"api_key_env"`
	APIKeyPresent bool   `json:"api_key_present"`
	Ready         bool   `json:"ready"`
	Message       string `json:"message"`
}

func Inspect(cfg *config.Config, mode string) Status {
	status := Status{
		PlannerMode: mode,
		Message:     "rule-based planner active",
	}
	if cfg == nil {
		status.Message = "config is nil; rule-based planner active"
		return status
	}

	if mode == "" {
		mode = cfg.LLM.PlannerMode
		status.PlannerMode = mode
	}

	status.Enabled = cfg.LLM.Enabled
	status.Provider = cfg.LLM.Provider
	status.Model = cfg.LLM.Model
	status.BaseURL = cfg.LLM.BaseURL
	status.APIKeyEnv = cfg.LLM.APIKeyEnv
	status.APIKeyPresent = strings.TrimSpace(os.Getenv(cfg.LLM.APIKeyEnv)) != ""

	switch mode {
	case "rule":
		status.Ready = true
		status.Message = "planner forced to rule mode"
	case "llm":
		if !cfg.LLM.Enabled {
			status.Message = "planner forced to llm mode, but llm.enabled=false"
			return status
		}
		if !status.APIKeyPresent {
			status.Message = fmt.Sprintf("planner forced to llm mode, but %s is not set", cfg.LLM.APIKeyEnv)
			return status
		}
		status.Ready = true
		status.Message = "llm planner is configured and ready for request testing"
	case "auto":
		if cfg.LLM.Enabled && status.APIKeyPresent {
			status.Ready = true
			status.Message = "auto mode can use llm planner"
			return status
		}
		status.Ready = true
		status.Message = "auto mode will fall back to rule planner"
	default:
		status.Message = "unknown planner mode; runtime will fall back to rule planner"
	}

	return status
}
