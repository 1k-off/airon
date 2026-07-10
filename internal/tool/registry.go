package tool

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/1node/airon/internal/provider"
)

type Registry struct {
	tools map[string]Tool
	order []Tool
}

func NewRegistry(tools ...Tool) (*Registry, error) {
	r := &Registry{
		tools: make(map[string]Tool, len(tools)),
		order: make([]Tool, 0, len(tools)),
	}

	for _, t := range tools {
		name := t.Name()
		if _, exists := r.tools[name]; exists {
			return nil, fmt.Errorf("duplicate tool name: %s", name)
		}
		r.tools[name] = t
		r.order = append(r.order, t)
	}

	return r, nil
}

func (r *Registry) Get(name string) (Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

func (r *Registry) List() []Tool {
	result := make([]Tool, len(r.order))
	copy(result, r.order)
	return result
}

func (r *Registry) Definitions() []provider.ToolDefinition {
	defs := make([]provider.ToolDefinition, len(r.order))
	for i, t := range r.order {
		defs[i] = provider.ToolDefinition{
			Name:        t.Name(),
			Description: t.Description(),
			Parameters:  t.Parameters(),
		}
	}
	return defs
}

func (r *Registry) Execute(ctx context.Context, call provider.ToolCall) (string, error) {
	t, ok := r.tools[call.Name]
	if !ok {
		return "", fmt.Errorf("unknown tool: %s", call.Name)
	}
	return t.Execute(ctx, json.RawMessage(call.Arguments))
}
