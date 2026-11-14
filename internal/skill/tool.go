package skill

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/1k-off/airon/internal/depth"
	"github.com/1k-off/airon/internal/provider"
	"github.com/1k-off/airon/internal/tool"
)

type skillTool struct {
	name         string
	description  string
	systemPrompt string
	provider     provider.Provider
	maxDepth     int
}

type skillExecuteArgs struct {
	Task string `json:"task"`
}

func NewSkillTool(name, description, systemPrompt string, providerClient provider.Provider, maxDepth int) tool.Tool {
	return &skillTool{
		name:         name,
		description:  description,
		systemPrompt: systemPrompt,
		provider:     providerClient,
		maxDepth:     maxDepth,
	}
}

func (s *skillTool) Name() string {
	return s.name
}

func (s *skillTool) Description() string {
	return s.description
}

func (s *skillTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"task": map[string]any{
				"type":        "string",
				"description": "What the skill should do",
			},
		},
		"required": []string{"task"},
	}
}

func (s *skillTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	if s.provider == nil {
		return "", fmt.Errorf("skill provider is required")
	}

	var parsed skillExecuteArgs
	if err := json.Unmarshal(args, &parsed); err != nil {
		return "", fmt.Errorf("parse skill args: %w", err)
	}
	if parsed.Task == "" {
		return "", fmt.Errorf("task is required")
	}

	currentDepth := depth.FromContext(ctx)
	if currentDepth >= s.maxDepth {
		return "", fmt.Errorf("max call depth exceeded: depth=%d max=%d", currentDepth, s.maxDepth)
	}

	ctx = depth.WithIncrement(ctx)

	resp, err := s.provider.Complete(ctx, provider.CompletionRequest{
		Messages: []provider.Message{
			{
				Role:    provider.RoleSystem,
				Content: s.systemPrompt,
			},
			{
				Role:    provider.RoleUser,
				Content: parsed.Task,
			},
		},
	})
	if err != nil {
		return "", fmt.Errorf("provider complete: %w", err)
	}

	return resp.Content, nil
}
