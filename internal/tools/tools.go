package tools

import (
	"context"
	"fmt"
	"sort"
)

type Tool interface {
	Name() string
	Description() string
	InputSchema() string
	Validate(input map[string]any) error
	Execute(ctx context.Context, input map[string]any) (string, error)
}

type Info struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	InputSchema string `json:"input_schema"`
}

type Registry struct {
	tools map[string]Tool
}

func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]Tool)}
}

func (r *Registry) Register(tool Tool) error {
	if tool == nil {
		return fmt.Errorf("tool is nil")
	}
	name := tool.Name()
	if name == "" {
		return fmt.Errorf("tool name must not be empty")
	}
	if _, exists := r.tools[name]; exists {
		return fmt.Errorf("tool %q already registered", name)
	}
	r.tools[name] = tool
	return nil
}

func (r *Registry) MustRegister(tool Tool) {
	if err := r.Register(tool); err != nil {
		panic(err)
	}
}

func (r *Registry) Execute(ctx context.Context, name string, input map[string]any) (string, error) {
	tool, ok := r.tools[name]
	if !ok {
		return "", fmt.Errorf("tool %q is not registered", name)
	}
	if err := tool.Validate(input); err != nil {
		return "", fmt.Errorf("tool %q input validation failed: %w", name, err)
	}
	return tool.Execute(ctx, input)
}

func (r *Registry) Names() []string {
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (r *Registry) Infos() []Info {
	names := r.Names()
	infos := make([]Info, 0, len(names))
	for _, name := range names {
		tool := r.tools[name]
		infos = append(infos, Info{
			Name:        tool.Name(),
			Description: tool.Description(),
			InputSchema: tool.InputSchema(),
		})
	}
	return infos
}
