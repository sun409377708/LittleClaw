package workflow

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
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
		return nil, fmt.Errorf("read workflows dir %q: %w", dir, err)
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
	return Definition{}, fmt.Errorf("workflow %q is not registered in %q", name, dir)
}

func LoadFile(path string) (Definition, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Definition{}, fmt.Errorf("read workflow file %q: %w", path, err)
	}

	def, err := parseWorkflowYAML(string(data))
	if err != nil {
		return Definition{}, fmt.Errorf("parse workflow file %q: %w", path, err)
	}
	if strings.TrimSpace(def.Name) == "" {
		return Definition{}, fmt.Errorf("workflow file %q is missing name", path)
	}
	if len(def.Steps) == 0 {
		return Definition{}, fmt.Errorf("workflow file %q must define at least one step", path)
	}
	for i, step := range def.Steps {
		if strings.TrimSpace(step.Type) == "" {
			def.Steps[i].Type = "agent"
		}
		if strings.TrimSpace(step.Name) == "" {
			def.Steps[i].Name = fmt.Sprintf("step-%d", i+1)
		}
		switch def.Steps[i].Type {
		case "agent":
			if strings.TrimSpace(step.Skill) == "" {
				def.Steps[i].Skill = "default"
			}
			if strings.TrimSpace(step.Task) == "" {
				return Definition{}, fmt.Errorf("workflow file %q has agent step %d without task", path, i+1)
			}
		case "tool":
			if strings.TrimSpace(step.Tool) == "" {
				return Definition{}, fmt.Errorf("workflow file %q has tool step %d without tool", path, i+1)
			}
			if strings.TrimSpace(step.InputJSON) != "" && len(step.ToolInput) > 0 {
				return Definition{}, fmt.Errorf("workflow file %q has tool step %d with both input_json and input block; use only one", path, i+1)
			}
			if strings.TrimSpace(step.InputJSON) != "" {
				var parsed map[string]any
				if err := json.Unmarshal([]byte(step.InputJSON), &parsed); err != nil {
					return Definition{}, fmt.Errorf("workflow file %q has invalid input_json at step %d: %w", path, i+1, err)
				}
				def.Steps[i].ToolInput = parsed
			}
			if len(def.Steps[i].ToolInput) == 0 {
				return Definition{}, fmt.Errorf("workflow file %q has tool step %d without input", path, i+1)
			}
		case "approval":
			if strings.TrimSpace(step.Prompt) == "" {
				return Definition{}, fmt.Errorf("workflow file %q has approval step %d without prompt", path, i+1)
			}
		default:
			return Definition{}, fmt.Errorf("workflow file %q has unsupported step type %q at step %d", path, def.Steps[i].Type, i+1)
		}
		if strings.TrimSpace(def.Steps[i].WhenStatus) != "" && strings.TrimSpace(def.Steps[i].WhenStep) == "" {
			return Definition{}, fmt.Errorf("workflow file %q has step %d with when_status but no when_step", path, i+1)
		}
		if strings.TrimSpace(def.Steps[i].OnFailure) == "" {
			def.Steps[i].OnFailure = "abort"
		}
		switch def.Steps[i].OnFailure {
		case "abort", "continue":
		default:
			return Definition{}, fmt.Errorf("workflow file %q has step %d with unsupported on_failure %q", path, i+1, def.Steps[i].OnFailure)
		}
	}
	return def, nil
}

func parseWorkflowYAML(raw string) (Definition, error) {
	var def Definition
	var inSteps bool
	var current *Step
	var stepSection string
	var nestedInputKey string

	flushStep := func() {
		if current != nil {
			def.Steps = append(def.Steps, *current)
			current = nil
		}
		stepSection = ""
		nestedInputKey = ""
	}

	for _, rawLine := range strings.Split(raw, "\n") {
		indent := countIndent(rawLine)
		trimmed := strings.TrimSpace(rawLine)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if trimmed == "steps:" {
			inSteps = true
			continue
		}

		if !inSteps {
			key, value, err := parseKeyValue(trimmed)
			if err != nil {
				return Definition{}, err
			}
			switch key {
			case "name":
				def.Name = value
			case "description":
				def.Description = value
			default:
				return Definition{}, fmt.Errorf("unsupported workflow key %q", key)
			}
			continue
		}

		if indent == 2 && strings.HasPrefix(trimmed, "- ") {
			flushStep()
			current = &Step{ToolInput: make(map[string]any)}
			rest := strings.TrimSpace(strings.TrimPrefix(trimmed, "- "))
			if rest != "" {
				key, value, err := parseKeyValue(rest)
				if err != nil {
					return Definition{}, err
				}
				if err := assignWorkflowStep(current, key, value); err != nil {
					return Definition{}, err
				}
			}
			continue
		}
		if current == nil {
			return Definition{}, fmt.Errorf("workflow step content found before any step declaration: %q", trimmed)
		}

		if indent == 4 {
			if trimmed == "input:" {
				stepSection = "input"
				nestedInputKey = ""
				continue
			}
			stepSection = ""
			nestedInputKey = ""
			key, value, err := parseKeyValue(trimmed)
			if err != nil {
				return Definition{}, err
			}
			if err := assignWorkflowStep(current, key, value); err != nil {
				return Definition{}, err
			}
			continue
		}

		if stepSection == "input" && indent == 6 {
			if strings.HasSuffix(trimmed, ":") {
				nestedInputKey = strings.TrimSuffix(trimmed, ":")
				current.ToolInput[nestedInputKey] = map[string]any{}
				continue
			}
			nestedInputKey = ""
			key, value, err := parseKeyValue(trimmed)
			if err != nil {
				return Definition{}, err
			}
			current.ToolInput[key] = parseScalar(value)
			continue
		}

		if stepSection == "input" && indent == 8 && nestedInputKey != "" {
			key, value, err := parseKeyValue(trimmed)
			if err != nil {
				return Definition{}, err
			}
			target, ok := current.ToolInput[nestedInputKey].(map[string]any)
			if !ok {
				return Definition{}, fmt.Errorf("workflow input section %q is not a map", nestedInputKey)
			}
			target[key] = parseScalar(value)
			continue
		}

		return Definition{}, fmt.Errorf("unsupported workflow structure near %q", trimmed)
	}
	flushStep()

	return def, nil
}

func parseKeyValue(line string) (string, string, error) {
	parts := strings.SplitN(line, ":", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid line %q", line)
	}
	key := strings.TrimSpace(parts[0])
	value := stripWrappingQuotes(strings.TrimSpace(parts[1]))
	return key, value, nil
}

func assignWorkflowStep(step *Step, key, value string) error {
	switch key {
	case "type":
		step.Type = value
	case "name":
		step.Name = value
	case "skill":
		step.Skill = value
	case "task":
		step.Task = value
	case "tool":
		step.Tool = value
	case "prompt":
		step.Prompt = value
	case "input_json":
		step.InputJSON = value
	case "when_step":
		step.WhenStep = value
	case "when_status":
		step.WhenStatus = value
	case "on_failure":
		step.OnFailure = value
	default:
		return fmt.Errorf("unsupported workflow step key %q", key)
	}
	return nil
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

func countIndent(line string) int {
	count := 0
	for _, ch := range line {
		if ch == ' ' {
			count++
			continue
		}
		break
	}
	return count
}

func parseScalar(value string) any {
	value = stripWrappingQuotes(strings.TrimSpace(value))
	if value == "" {
		return ""
	}
	if strings.EqualFold(value, "true") {
		return true
	}
	if strings.EqualFold(value, "false") {
		return false
	}
	if i, err := strconv.Atoi(value); err == nil {
		return i
	}
	if f, err := strconv.ParseFloat(value, 64); err == nil {
		return f
	}
	if (strings.HasPrefix(value, "{") && strings.HasSuffix(value, "}")) || (strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]")) {
		var parsed any
		if err := json.Unmarshal([]byte(value), &parsed); err == nil {
			return parsed
		}
	}
	return value
}
