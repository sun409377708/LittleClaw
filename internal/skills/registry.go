package skills

import (
	"fmt"
	"sort"
	"strings"
)

type Skill struct {
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	SystemPrompt string   `json:"system_prompt"`
	AllowedTools []string `json:"allowed_tools"`
	MaxSteps     int      `json:"max_steps"`
}

type Info struct {
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	AllowedTools []string `json:"allowed_tools"`
	MaxSteps     int      `json:"max_steps"`
}

type Registry struct {
	skills map[string]Skill
}

func NewRegistry() *Registry {
	registry := &Registry{
		skills: make(map[string]Skill),
	}
	for _, skill := range builtInSkills() {
		registry.skills[skill.Name] = skill
	}
	return registry
}

func (r *Registry) Resolve(name string) (Skill, error) {
	if strings.TrimSpace(name) == "" {
		name = "default"
	}
	skill, ok := r.skills[name]
	if !ok {
		return Skill{}, fmt.Errorf("skill %q is not registered", name)
	}
	return skill, nil
}

func (r *Registry) Infos() []Info {
	names := make([]string, 0, len(r.skills))
	for name := range r.skills {
		names = append(names, name)
	}
	sort.Strings(names)

	infos := make([]Info, 0, len(names))
	for _, name := range names {
		skill := r.skills[name]
		infos = append(infos, Info{
			Name:         skill.Name,
			Description:  skill.Description,
			AllowedTools: append([]string(nil), skill.AllowedTools...),
			MaxSteps:     skill.MaxSteps,
		})
	}
	return infos
}

func builtInSkills() []Skill {
	return []Skill{
		{
			Name:         "default",
			Description:  "Balanced default skill for local tasks and lightweight web fetches.",
			SystemPrompt: "Prefer structured tools over shell. Use shell only when file or http tools are not sufficient.",
			AllowedTools: []string{"file.append", "file.list", "file.read", "file.write", "http.get", "http.post", "shell"},
			MaxSteps:     8,
		},
		{
			Name:         "coder",
			Description:  "Code-oriented skill with strong local workspace access.",
			SystemPrompt: "Focus on local repository inspection and file editing. Prefer file tools first, then shell when needed.",
			AllowedTools: []string{"file.append", "file.list", "file.read", "file.write", "shell"},
			MaxSteps:     10,
		},
		{
			Name:         "researcher",
			Description:  "Research-oriented skill for reading files and fetching web content.",
			SystemPrompt: "Prioritize http tools for external content and file tools for local notes. Avoid shell unless directory inspection is necessary.",
			AllowedTools: []string{"file.append", "file.list", "file.read", "file.write", "http.get", "http.post"},
			MaxSteps:     8,
		},
		{
			Name:         "ops",
			Description:  "Operations-oriented skill with shell plus read-only inspection tools.",
			SystemPrompt: "Inspect the system conservatively. Use shell for read-only diagnostics and avoid write operations unless explicitly requested.",
			AllowedTools: []string{"file.list", "file.read", "http.get", "shell"},
			MaxSteps:     8,
		},
	}
}
