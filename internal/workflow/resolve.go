package workflow

import (
	"fmt"
	"regexp"
	"strings"
)

var stepRefPattern = regexp.MustCompile(`\{\{\s*steps\.([a-zA-Z0-9_-]+)\.(output|status|run_id)\s*\}\}`)

func resolveStep(step Step, previous []WorkflowStepRun) (Step, error) {
	resolved := step

	taskValue, err := resolveValue(step.Task, previous)
	if err != nil {
		return Step{}, err
	}
	if task, ok := taskValue.(string); ok {
		resolved.Task = task
	}

	inputValue, err := resolveValue(step.ToolInput, previous)
	if err != nil {
		return Step{}, err
	}
	if inputMap, ok := inputValue.(map[string]any); ok {
		resolved.ToolInput = inputMap
	}

	return resolved, nil
}

func resolveValue(value any, previous []WorkflowStepRun) (any, error) {
	switch typed := value.(type) {
	case string:
		return resolveString(typed, previous)
	case map[string]any:
		resolved := make(map[string]any, len(typed))
		for key, child := range typed {
			next, err := resolveValue(child, previous)
			if err != nil {
				return nil, err
			}
			resolved[key] = next
		}
		return resolved, nil
	case []any:
		resolved := make([]any, 0, len(typed))
		for _, child := range typed {
			next, err := resolveValue(child, previous)
			if err != nil {
				return nil, err
			}
			resolved = append(resolved, next)
		}
		return resolved, nil
	default:
		return value, nil
	}
}

func resolveString(raw string, previous []WorkflowStepRun) (any, error) {
	matches := stepRefPattern.FindAllStringSubmatchIndex(raw, -1)
	if len(matches) == 0 {
		return raw, nil
	}

	// Preserve the original type when the full value is a single placeholder.
	if len(matches) == 1 && matches[0][0] == 0 && matches[0][1] == len(raw) {
		name := raw[matches[0][2]:matches[0][3]]
		field := raw[matches[0][4]:matches[0][5]]
		return lookupStepField(previous, name, field)
	}

	var builder strings.Builder
	last := 0
	for _, match := range matches {
		builder.WriteString(raw[last:match[0]])
		name := raw[match[2]:match[3]]
		field := raw[match[4]:match[5]]
		value, err := lookupStepField(previous, name, field)
		if err != nil {
			return nil, err
		}
		builder.WriteString(fmt.Sprint(value))
		last = match[1]
	}
	builder.WriteString(raw[last:])
	return builder.String(), nil
}

func lookupStepField(previous []WorkflowStepRun, name, field string) (any, error) {
	for _, step := range previous {
		if step.Name != name {
			continue
		}
		switch field {
		case "output":
			return step.Output, nil
		case "status":
			return string(step.Status), nil
		case "run_id":
			return step.RunID, nil
		default:
			return nil, fmt.Errorf("unsupported step field %q", field)
		}
	}
	return nil, fmt.Errorf("step reference %q was not found", name)
}
