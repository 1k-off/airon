package agent

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/1k-off/airon/internal/config"
	"github.com/1k-off/airon/internal/provider"
	"github.com/1k-off/airon/internal/tool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAgentTool_validationErrors(t *testing.T) {
	registry, err := tool.NewRegistry()
	require.NoError(t, err)

	_, err = NewAgentTool(config.Agent{ID: "a"}, registry, nil, "gpt", 3, 4)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "provider is required")

	_, err = NewAgentTool(config.Agent{}, registry, &queueProvider{}, "gpt", 3, 4)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "agent id is required")

	_, err = NewAgentTool(config.Agent{
		ID:    "a",
		Tools: []string{"missing"},
	}, registry, &queueProvider{}, "gpt", 3, 4)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown tool")

	_, err = NewAgentTool(config.Agent{
		ID:    "a",
		Tools: []string{"x"},
	}, nil, &queueProvider{}, "gpt", 3, 4)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tool registry is required")
}

func TestResolveSystemPrompt_inlineAndFile(t *testing.T) {
	inline, err := resolveSystemPrompt("You are helpful.")
	require.NoError(t, err)
	assert.Equal(t, "You are helpful.", inline)

	root := t.TempDir()
	promptPath := filepath.Join(root, "prompt.txt")
	require.NoError(t, os.WriteFile(promptPath, []byte("From file"), 0o644))

	fromFile, err := resolveSystemPrompt(promptPath)
	require.NoError(t, err)
	assert.Equal(t, "From file", fromFile)

	dirPath := filepath.Join(root, "dir")
	require.NoError(t, os.Mkdir(dirPath, 0o755))
	_, err = resolveSystemPrompt(dirPath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "directory")
}

func TestLoadAgents_wrapsAgentLoadError(t *testing.T) {
	registry, err := tool.NewRegistry()
	require.NoError(t, err)

	_, err = LoadAgents([]config.Agent{
		{ID: "bad", Tools: []string{"missing"}},
	}, registry, &queueProvider{}, "gpt", 3, 4)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "load agent bad")
}

func TestAgentTool_metadataMethods(t *testing.T) {
	registry, err := tool.NewRegistry()
	require.NoError(t, err)

	agentTool, err := NewAgentTool(config.Agent{
		ID:          "meta",
		Description: "Meta agent",
	}, registry, &queueProvider{
		responses: []provider.CompletionResponse{{Content: "ok"}},
	}, "gpt-default", 3, 4)
	require.NoError(t, err)

	assert.Equal(t, "meta", agentTool.Name())
	assert.Equal(t, "Meta agent", agentTool.Description())
	params := agentTool.Parameters()
	assert.Equal(t, "object", params["type"])
}

func TestAgentTool_Execute_validationAndProviderErrors(t *testing.T) {
	registry, err := tool.NewRegistry()
	require.NoError(t, err)

	agentTool, err := NewAgentTool(config.Agent{
		ID:           "analyst",
		SystemPrompt: "prompt",
	}, registry, &queueProvider{}, "gpt-default", 3, 4)
	require.NoError(t, err)

	_, err = agentTool.Execute(context.Background(), json.RawMessage(`not-json`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decode args")

	_, err = agentTool.Execute(context.Background(), json.RawMessage(`{"message":""}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "message is required")

	agentTool, err = NewAgentTool(config.Agent{
		ID:           "analyst",
		SystemPrompt: "prompt",
	}, registry, &queueProvider{err: errors.New("provider down")}, "gpt-default", 3, 4)
	require.NoError(t, err)

	_, err = agentTool.Execute(context.Background(), json.RawMessage(`{"message":"hi"}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "provider complete")
}

func TestAgentTool_Execute_disallowedToolAndMaxRounds(t *testing.T) {
	allowed := &stubTool{
		name: "allowed_tool",
		execute: func(context.Context, json.RawMessage) (string, error) {
			return "ok", nil
		},
	}
	registry, err := tool.NewRegistry(allowed)
	require.NoError(t, err)

	agentTool, err := NewAgentTool(config.Agent{
		ID:            "analyst",
		SystemPrompt:  "prompt",
		Tools:         []string{"allowed_tool"},
		MaxToolRounds: 1,
	}, registry, &queueProvider{
		responses: []provider.CompletionResponse{
			{
				ToolCalls: []provider.ToolCall{
					{ID: "c1", Name: "secret", Arguments: `{}`},
				},
			},
		},
	}, "gpt-default", 3, 1)
	require.NoError(t, err)

	_, err = agentTool.Execute(context.Background(), json.RawMessage(`{"message":"go"}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "max tool rounds exceeded")

	agentTool, err = NewAgentTool(config.Agent{
		ID:            "analyst",
		SystemPrompt:  "prompt",
		Tools:         []string{"allowed_tool"},
		MaxToolRounds: 2,
	}, registry, &queueProvider{
		responses: []provider.CompletionResponse{
			{
				ToolCalls: []provider.ToolCall{
					{ID: "c1", Name: "secret", Arguments: `{}`},
				},
			},
			{Content: "done"},
		},
	}, "gpt-default", 3, 2)
	require.NoError(t, err)

	out, err := agentTool.Execute(context.Background(), json.RawMessage(`{"message":"go"}`))
	require.NoError(t, err)
	assert.Equal(t, "done", out)
}

func TestAgentTool_Execute_toolExecuteError(t *testing.T) {
	broken := &stubTool{
		name: "broken",
		execute: func(context.Context, json.RawMessage) (string, error) {
			return "", errors.New("boom")
		},
	}
	registry, err := tool.NewRegistry(broken)
	require.NoError(t, err)

	agentTool, err := NewAgentTool(config.Agent{
		ID:            "analyst",
		SystemPrompt:  "prompt",
		Tools:         []string{"broken"},
		MaxToolRounds: 3,
	}, registry, &queueProvider{
		responses: []provider.CompletionResponse{
			{
				ToolCalls: []provider.ToolCall{
					{ID: "c1", Name: "broken", Arguments: `{}`},
				},
			},
			{Content: "recovered"},
		},
	}, "gpt-default", 3, 3)
	require.NoError(t, err)

	out, err := agentTool.Execute(context.Background(), json.RawMessage(`{"message":"go"}`))
	require.NoError(t, err)
	assert.Equal(t, "recovered", out)
}

func TestNewAgentTool_defaultsMaxToolRounds(t *testing.T) {
	registry, err := tool.NewRegistry()
	require.NoError(t, err)

	loaded, err := NewAgentTool(config.Agent{
		ID:           "analyst",
		SystemPrompt: "prompt",
	}, registry, &queueProvider{
		responses: []provider.CompletionResponse{{Content: "ok"}},
	}, "gpt-default", 3, 0)
	require.NoError(t, err)

	at := loaded.(*agentTool)
	assert.Equal(t, defaultMaxToolRounds, at.maxToolRounds)
}
