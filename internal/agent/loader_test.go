package agent

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/1node/airon/internal/config"
	"github.com/1node/airon/internal/depth"
	"github.com/1node/airon/internal/provider"
	"github.com/1node/airon/internal/tool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type queueProvider struct {
	requests  []provider.CompletionRequest
	responses []provider.CompletionResponse
	err       error
}

func (m *queueProvider) Complete(_ context.Context, req provider.CompletionRequest) (provider.CompletionResponse, error) {
	m.requests = append(m.requests, req)
	if m.err != nil {
		return provider.CompletionResponse{}, m.err
	}
	if len(m.responses) == 0 {
		return provider.CompletionResponse{}, errors.New("no queued response")
	}
	resp := m.responses[0]
	m.responses = m.responses[1:]
	return resp, nil
}

type stubTool struct {
	name    string
	execute func(ctx context.Context, args json.RawMessage) (string, error)
}

func (s *stubTool) Name() string        { return s.name }
func (s *stubTool) Description() string { return "stub " + s.name }
func (s *stubTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
	}
}
func (s *stubTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	return s.execute(ctx, args)
}

func TestNewAgentTool_selfReferenceReturnsError(t *testing.T) {
	registry, err := tool.NewRegistry()
	require.NoError(t, err)

	_, err = NewAgentTool(config.Agent{
		ID:           "analyst",
		SystemPrompt: "inline prompt",
		Tools:        []string{"analyst"},
	}, registry, &queueProvider{}, "gpt-default", 3, 4)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot reference itself")
}

func TestAgentTool_Execute_usesScopedToolSubset(t *testing.T) {
	allowedCalled := 0
	allowed := &stubTool{
		name: "allowed_tool",
		execute: func(_ context.Context, args json.RawMessage) (string, error) {
			allowedCalled++
			assert.JSONEq(t, `{"x":"1"}`, string(args))
			return "allowed-result", nil
		},
	}
	disallowed := &stubTool{
		name: "disallowed_tool",
		execute: func(_ context.Context, _ json.RawMessage) (string, error) {
			t.Fatal("disallowed tool must never be executed")
			return "", nil
		},
	}
	registry, err := tool.NewRegistry(allowed, disallowed)
	require.NoError(t, err)

	mockProvider := &queueProvider{
		responses: []provider.CompletionResponse{
			{
				Content: "calling allowed tool",
				ToolCalls: []provider.ToolCall{
					{ID: "call-1", Name: "allowed_tool", Arguments: `{"x":"1"}`},
				},
			},
			{
				Content: "final agent answer",
			},
		},
	}

	agentTool, err := NewAgentTool(config.Agent{
		ID:            "analyst",
		Description:   "Data analyst",
		SystemPrompt:  "You are an analyst.",
		Model:         "gpt-agent",
		Tools:         []string{"allowed_tool"},
		MaxToolRounds: 3,
	}, registry, mockProvider, "gpt-default", 4, 6)
	require.NoError(t, err)

	out, err := agentTool.Execute(context.Background(), json.RawMessage(`{"message":"Analyze this"}`))
	require.NoError(t, err)
	assert.Equal(t, "final agent answer", out)
	assert.Equal(t, 1, allowedCalled)
	require.Len(t, mockProvider.requests, 2)
	assert.Equal(t, "gpt-agent", mockProvider.requests[0].Model)
	require.Len(t, mockProvider.requests[0].Tools, 1)
	assert.Equal(t, "allowed_tool", mockProvider.requests[0].Tools[0].Name)
	require.Len(t, mockProvider.requests[1].Messages, 4)
	assert.Equal(t, provider.RoleAssistant, mockProvider.requests[1].Messages[2].Role)
	assert.Equal(t, provider.RoleTool, mockProvider.requests[1].Messages[3].Role)
	assert.Equal(t, "allowed-result", mockProvider.requests[1].Messages[3].Content)
}

func TestAgentTool_Execute_enforcesMaxCallDepth(t *testing.T) {
	registry, err := tool.NewRegistry()
	require.NoError(t, err)

	mockProvider := &queueProvider{
		responses: []provider.CompletionResponse{
			{Content: "should not be used"},
		},
	}

	agentTool, err := NewAgentTool(config.Agent{
		ID:           "analyst",
		SystemPrompt: "inline prompt",
	}, registry, mockProvider, "gpt-default", 2, 3)
	require.NoError(t, err)

	ctx := depth.WithIncrement(depth.WithIncrement(context.Background()))
	_, err = agentTool.Execute(ctx, json.RawMessage(`{"message":"hello"}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "max call depth exceeded")
	assert.Len(t, mockProvider.requests, 0)
}

func TestLoadAgents_buildsToolsAndDefaultsModel(t *testing.T) {
	registry, err := tool.NewRegistry()
	require.NoError(t, err)

	mockProvider := &queueProvider{
		responses: []provider.CompletionResponse{
			{Content: "done"},
		},
	}

	tools, err := LoadAgents([]config.Agent{
		{
			ID:           "analyst",
			Description:  "desc",
			SystemPrompt: "inline prompt",
		},
	}, registry, mockProvider, "gpt-default", 3, 4)
	require.NoError(t, err)
	require.Len(t, tools, 1)

	out, err := tools[0].Execute(context.Background(), json.RawMessage(`{"message":"hello"}`))
	require.NoError(t, err)
	assert.Equal(t, "done", out)
	require.Len(t, mockProvider.requests, 1)
	assert.Equal(t, "gpt-default", mockProvider.requests[0].Model)
}
