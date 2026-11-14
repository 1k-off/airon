package agent

import (
	"fmt"
	"os"

	"github.com/1k-off/airon/internal/config"
	"github.com/1k-off/airon/internal/provider"
	"github.com/1k-off/airon/internal/tool"
)

func NewAgentTool(
	cfg config.Agent,
	registry *tool.Registry,
	provider provider.Provider,
	defaultModel string,
	maxDepth, maxToolRounds int,
) (tool.Tool, error) {
	if provider == nil {
		return nil, fmt.Errorf("provider is required")
	}

	if cfg.ID == "" {
		return nil, fmt.Errorf("agent id is required")
	}

	systemPrompt, err := resolveSystemPrompt(cfg.SystemPrompt)
	if err != nil {
		return nil, fmt.Errorf("resolve system prompt: %w", err)
	}

	toolsByName, toolDefs, err := resolveScopedTools(cfg, registry)
	if err != nil {
		return nil, err
	}

	agentModel := defaultModel
	if cfg.Model != "" {
		agentModel = cfg.Model
	}

	rounds := maxToolRounds
	if cfg.MaxToolRounds > 0 {
		rounds = cfg.MaxToolRounds
	}
	if rounds <= 0 {
		rounds = defaultMaxToolRounds
	}

	return &agentTool{
		id:            cfg.ID,
		description:   cfg.Description,
		systemPrompt:  systemPrompt,
		model:         agentModel,
		toolDefs:      toolDefs,
		toolsByName:   toolsByName,
		provider:      provider,
		maxDepth:      maxDepth,
		maxToolRounds: rounds,
	}, nil
}

func LoadAgents(
	agents []config.Agent,
	registry *tool.Registry,
	provider provider.Provider,
	defaultModel string,
	maxDepth, maxToolRounds int,
) ([]tool.Tool, error) {
	loaded := make([]tool.Tool, 0, len(agents))
	for _, cfg := range agents {
		t, err := NewAgentTool(cfg, registry, provider, defaultModel, maxDepth, maxToolRounds)
		if err != nil {
			return nil, fmt.Errorf("load agent %s: %w", cfg.ID, err)
		}
		loaded = append(loaded, t)
	}
	return loaded, nil
}

func resolveScopedTools(cfg config.Agent, registry *tool.Registry) (map[string]tool.Tool, []provider.ToolDefinition, error) {
	if len(cfg.Tools) == 0 {
		return map[string]tool.Tool{}, nil, nil
	}
	if registry == nil {
		return nil, nil, fmt.Errorf("tool registry is required")
	}

	toolsByName := make(map[string]tool.Tool, len(cfg.Tools))
	defs := make([]provider.ToolDefinition, 0, len(cfg.Tools))
	for _, name := range cfg.Tools {
		if name == cfg.ID {
			return nil, nil, fmt.Errorf("agent cannot reference itself")
		}
		t, ok := registry.Get(name)
		if !ok {
			return nil, nil, fmt.Errorf("unknown tool: %s", name)
		}
		toolsByName[name] = t
		defs = append(defs, provider.ToolDefinition{
			Name:        t.Name(),
			Description: t.Description(),
			Parameters:  t.Parameters(),
		})
	}
	return toolsByName, defs, nil
}

func resolveSystemPrompt(value string) (string, error) {
	if value == "" {
		return "", nil
	}

	info, err := os.Stat(value)
	if err != nil {
		if os.IsNotExist(err) {
			return value, nil
		}
		return "", err
	}
	if info.IsDir() {
		return "", fmt.Errorf("system prompt path is a directory: %s", value)
	}

	data, err := os.ReadFile(value)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

