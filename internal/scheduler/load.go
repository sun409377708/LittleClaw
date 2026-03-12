package scheduler

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func LoadAll(dir string) ([]Definition, error) {
	if strings.TrimSpace(dir) == "" {
		return nil, nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read schedules dir %q: %w", dir, err)
	}

	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext != ".yaml" && ext != ".yml" {
			continue
		}
		paths = append(paths, filepath.Join(dir, entry.Name()))
	}
	sort.Strings(paths)

	defs := make([]Definition, 0, len(paths))
	for _, path := range paths {
		def, err := LoadFile(path)
		if err != nil {
			return nil, err
		}
		defs = append(defs, def)
	}
	return defs, nil
}

func LoadByName(dir, name string) (Definition, error) {
	defs, err := LoadAll(dir)
	if err != nil {
		return Definition{}, err
	}
	for _, def := range defs {
		if def.Name == name {
			return def, nil
		}
	}
	return Definition{}, fmt.Errorf("schedule %q is not registered in %q", name, dir)
}

func LoadFile(path string) (Definition, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Definition{}, fmt.Errorf("read schedule file %q: %w", path, err)
	}

	def, err := parseScheduleYAML(string(data))
	if err != nil {
		return Definition{}, fmt.Errorf("parse schedule file %q: %w", path, err)
	}
	if strings.TrimSpace(def.Name) == "" {
		return Definition{}, fmt.Errorf("schedule file %q is missing name", path)
	}
	if strings.TrimSpace(def.Every) == "" && strings.TrimSpace(def.Cron) == "" {
		return Definition{}, fmt.Errorf("schedule file %q must define every or cron", path)
	}
	if strings.TrimSpace(def.Every) != "" && strings.TrimSpace(def.Cron) != "" {
		return Definition{}, fmt.Errorf("schedule file %q cannot define both every and cron", path)
	}
	if strings.TrimSpace(def.Every) != "" {
		if _, err := parsePositiveDuration(def.Every); err != nil {
			return Definition{}, fmt.Errorf("schedule file %q has invalid every: %w", path, err)
		}
	}
	if strings.TrimSpace(def.Cron) != "" {
		if _, err := ParseCron(def.Cron); err != nil {
			return Definition{}, fmt.Errorf("schedule file %q has invalid cron: %w", path, err)
		}
	}
	if strings.TrimSpace(def.Target) == "" {
		switch {
		case strings.TrimSpace(def.Task) != "":
			def.Target = "task"
		case strings.TrimSpace(def.Workflow) != "":
			def.Target = "workflow"
		default:
			return Definition{}, fmt.Errorf("schedule file %q must define either task or workflow", path)
		}
	}
	if strings.TrimSpace(def.Dispatch) == "" {
		def.Dispatch = "direct"
	}
	switch def.Dispatch {
	case "direct", "queue":
	default:
		return Definition{}, fmt.Errorf("schedule file %q has unsupported dispatch %q", path, def.Dispatch)
	}
	if strings.TrimSpace(def.Concurrency) == "" {
		def.Concurrency = "skip"
	}
	switch def.Concurrency {
	case "skip", "allow":
	default:
		return Definition{}, fmt.Errorf("schedule file %q has unsupported concurrency %q", path, def.Concurrency)
	}
	switch def.Target {
	case "task":
		if strings.TrimSpace(def.Skill) == "" {
			def.Skill = "default"
		}
		if strings.TrimSpace(def.Task) == "" {
			return Definition{}, fmt.Errorf("schedule file %q target task requires task", path)
		}
	case "workflow":
		if strings.TrimSpace(def.Workflow) == "" {
			return Definition{}, fmt.Errorf("schedule file %q target workflow requires workflow", path)
		}
	default:
		return Definition{}, fmt.Errorf("schedule file %q has unsupported target %q", path, def.Target)
	}
	return def, nil
}

func ToInfos(defs []Definition) []Info {
	infos := make([]Info, 0, len(defs))
	for _, def := range defs {
		infos = append(infos, Info{
			Name:           def.Name,
			Description:    def.Description,
			Every:          def.Every,
			Cron:           def.Cron,
			Dispatch:       def.Dispatch,
			Target:         def.Target,
			Concurrency:    def.Concurrency,
			RunImmediately: def.RunImmediately,
			Enabled:        def.Enabled,
		})
	}
	return infos
}

func parseScheduleYAML(raw string) (Definition, error) {
	def := Definition{
		Enabled:        true,
		Dispatch:       "direct",
		Concurrency:    "skip",
		RunImmediately: true,
	}
	for _, rawLine := range strings.Split(raw, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			return Definition{}, fmt.Errorf("invalid line %q", line)
		}
		key := strings.TrimSpace(parts[0])
		value := stripWrappingQuotes(strings.TrimSpace(parts[1]))
		switch key {
		case "name":
			def.Name = value
		case "description":
			def.Description = value
		case "every":
			def.Every = value
		case "cron":
			def.Cron = value
		case "dispatch":
			def.Dispatch = value
		case "target":
			def.Target = value
		case "skill":
			def.Skill = value
		case "task":
			def.Task = value
		case "workflow":
			def.Workflow = value
		case "concurrency":
			def.Concurrency = value
		case "run_immediately":
			def.RunImmediately = strings.EqualFold(value, "true")
		case "enabled":
			def.Enabled = strings.EqualFold(value, "true")
		default:
			return Definition{}, fmt.Errorf("unsupported schedule key %q", key)
		}
	}
	return def, nil
}

func stripWrappingQuotes(value string) string {
	if len(value) >= 2 {
		if strings.HasPrefix(value, `"`) && strings.HasSuffix(value, `"`) {
			return value[1 : len(value)-1]
		}
		if strings.HasPrefix(value, `'`) && strings.HasSuffix(value, `'`) {
			return value[1 : len(value)-1]
		}
	}
	return value
}
