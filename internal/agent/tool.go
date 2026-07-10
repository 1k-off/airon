package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/1node/airon/internal/depth"
	"github.com/1node/airon/internal/provider"
	"github.com/1node/airon/internal/tool"
)

const defaultMaxToolRounds = 10

type agentTool struct {
	id            string
	description   string
	systemPrompt  string
	model         string
	toolDefs      []provider.ToolDefinition
	toolsByName   map[string]tool.Tool
	provider      provider.Provider
	maxDepth      int
	maxToolRounds int
}

func (a *agentTool) Name() string {
	return a.id
}

func (a *agentTool) Description() string {
	return a.description
}

func (a *agentTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"message": map[string]any{
				"type":        "string",
				"description": "Message to send to the agent",
			},
		},
		"required": []any{"message"},
	}
}

func (a *agentTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	if depth.FromContext(ctx) >= a.maxDepth {
		return "", fmt.Errorf("max call depth exceeded: %d", a.maxDepth)
	}

	var payload struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(args, &payload); err != nil {
		return "", fmt.Errorf("decode args: %w", err)
	}
	if payload.Message == "" {
		return "", fmt.Errorf("message is required")
	}

	subCtx := depth.WithIncrement(ctx)

	messages := make([]provider.Message, 0, 2)
	if a.systemPrompt != "" {
		messages = append(messages, provider.Message{
			Role:    provider.RoleSystem,
			Content: a.systemPrompt,
		})
	}
	messages = append(messages, provider.Message{
		Role:    provider.RoleUser,
		Content: payload.Message,
	})

	for round := 1; round <= a.maxToolRounds; round++ {
		resp, err := a.provider.Complete(subCtx, provider.CompletionRequest{
			Model:    a.model,
			Messages: messages,
			Tools:    a.toolDefs,
		})
		if err != nil {
			return "", fmt.Errorf("provider complete: %w", err)
		}

		if len(resp.ToolCalls) == 0 {
			return resp.Content, nil
		}

		messages = append(messages, provider.Message{
			Role:      provider.RoleAssistant,
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		})

		for _, call := range resp.ToolCalls {
			target, ok := a.toolsByName[call.Name]
			var result string
			if !ok {
				result = fmt.Sprintf("error: agent tool %q is not allowed", call.Name)
			} else {
				var execErr error
				result, execErr = target.Execute(depth.WithIncrement(subCtx), json.RawMessage(call.Arguments))
				if execErr != nil {
					result = fmt.Sprintf("error: %v", execErr)
				}
			}

			messages = append(messages, provider.Message{
				Role:       provider.RoleTool,
				Content:    result,
				ToolCallID: call.ID,
			})
		}
	}

	return "", fmt.Errorf("max tool rounds exceeded: %d", a.maxToolRounds)
}

