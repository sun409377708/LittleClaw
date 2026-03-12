package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func NewRegistryFromDir(dir string) (*Registry, error) {
	registry := NewRegistry()
	if strings.TrimSpace(dir) == "" {
		return registry, nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return registry, nil
		}
		return nil, fmt.Errorf("read skills dir %q: %w", dir, err)
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

	for _, path := range paths {
		skill, err := loadSkillFile(path)
		if err != nil {
			return nil, err
		}
		registry.skills[skill.Name] = skill
	}
	return registry, nil
}

func loadSkillFile(path string) (Skill, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Skill{}, fmt.Errorf("read skill file %q: %w", path, err)
	}

	skill, err := parseSkillYAML(string(data))
	if err != nil {
		return Skill{}, fmt.Errorf("parse skill file %q: %w", path, err)
	}
	if strings.TrimSpace(skill.Name) == "" {
		return Skill{}, fmt.Errorf("skill file %q is missing name", path)
	}
	if len(skill.AllowedTools) == 0 {
		return Skill{}, fmt.Errorf("skill file %q must define at least one allowed tool", path)
	}
	if skill.MaxSteps <= 0 {
		skill.MaxSteps = 8
	}
	return skill, nil
}

func parseSkillYAML(raw string) (Skill, error) {
	var skill Skill
	var section string

	for _, line := range strings.Split(raw, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasSuffix(trimmed, ":") {
			section = strings.TrimSuffix(trimmed, ":")
			continue
		}

		if section == "allowed_tools" && strings.HasPrefix(trimmed, "- ") {
			skill.AllowedTools = append(skill.AllowedTools, strings.TrimSpace(strings.TrimPrefix(trimmed, "- ")))
			continue
		}

		parts := strings.SplitN(trimmed, ":", 2)
		if len(parts) != 2 {
			return Skill{}, fmt.Errorf("invalid line %q", trimmed)
		}

		key := strings.TrimSpace(parts[0])
		value := strings.Trim(strings.TrimSpace(parts[1]), `"'`)

		switch section + "." + key {
		case ".name":
			skill.Name = value
		case ".description":
			skill.Description = value
		case ".system_prompt":
			skill.SystemPrompt = value
		case ".max_steps":
			var parsed int
			if _, err := fmt.Sscanf(value, "%d", &parsed); err != nil {
				return Skill{}, fmt.Errorf("invalid max_steps %q", value)
			}
			skill.MaxSteps = parsed
		default:
			return Skill{}, fmt.Errorf("unsupported key %q", section+"."+key)
		}
	}

	return skill, nil
}
