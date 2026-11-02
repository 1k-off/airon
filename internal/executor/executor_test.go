package executor

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/1k-off/airon/internal/config"
	"github.com/1k-off/airon/internal/depth"
	"github.com/1k-off/airon/internal/provider"
	"github.com/1k-off/airon/internal/skill"
	"github.com/1k-off/airon/internal/tool"
	"github.com/1k-off/airon/internal/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockProvider struct {
	requests  []provider.CompletionRequest
	responses []provider.CompletionResponse
}

func (m *mockProvider) Complete(ctx context.Context, req provider.CompletionRequest) (provider.CompletionResponse, error) {
	m.requests = append(m.requests, req)
	if len(m.responses) == 0 {
		return provider.CompletionResponse{}, nil
	}
	resp := m.responses[0]
	m.responses = m.responses[1:]
	return resp, nil
}

type echoTool struct{}

func (e *echoTool) Name() string        { return "echo" }
func (e *echoTool) Description() string { return "Echo text" }
func (e *echoTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"text": map[string]any{"type": "string"},
		},
		"required": []any{"text"},
	}
}
func (e *echoTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var payload struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(args, &payload); err != nil {
		return "", err
	}
	return "ECHO:" + payload.Text, nil
}

func TestExecutor_Run_twoStepWithoutTools(t *testing.T) {
	mp := &mockProvider{
		responses: []provider.CompletionResponse{
			{Content: "first-output"},
			{Content: "second-output"},
		},
	}
	reg, err := tool.NewRegistry()
	require.NoError(t, err)

	exec := NewExecutor(ExecutorConfig{
		Provider:      mp,
		Registry:      reg,
		Prompts:       []config.Prompt{{ID: "step1", Content: "First {{input}}"}, {ID: "step2", DependsOn: []string{"step1"}, Content: "Second {{step1}}"}},
		MaxToolRounds: 3,
	})

	result, err := exec.Run(context.Background(), RunInput{
		Variables: map[string]string{"input": "hello"},
	})
	require.NoError(t, err)
	require.Len(t, result.Steps, 2)
	assert.Equal(t, "first-output", result.Steps[0].Output)
	assert.Equal(t, "second-output", result.Steps[1].Output)

	require.Len(t, mp.requests, 2)
	assert.Equal(t, "First hello", mp.requests[0].Messages[0].Content)
	assert.Equal(t, "Second first-output", mp.requests[1].Messages[0].Content)
}

func TestExecutor_Run_toolCallLoopThenFinalText(t *testing.T) {
	mp := &mockProvider{
		responses: []provider.CompletionResponse{
			{
				Content: "calling tool",
				ToolCalls: []provider.ToolCall{
					{ID: "call-1", Name: "echo", Arguments: `{"text":"hello"}`},
				},
			},
			{Content: "final-output"},
		},
	}
	reg, err := tool.NewRegistry(&echoTool{})
	require.NoError(t, err)

	exec := NewExecutor(ExecutorConfig{
		Provider:      mp,
		Registry:      reg,
		Prompts:       []config.Prompt{{ID: "step1", Content: "go", Tools: []string{"echo"}}},
		MaxToolRounds: 3,
	})

	result, err := exec.Run(context.Background(), RunInput{})
	require.NoError(t, err)
	require.Len(t, result.Steps, 1)
	assert.Equal(t, "final-output", result.Steps[0].Output)
	assert.Equal(t, 2, result.Steps[0].Rounds)

	require.Len(t, mp.requests, 2)
	require.Len(t, mp.requests[1].Messages, 3)
	assert.Equal(t, provider.RoleAssistant, mp.requests[1].Messages[1].Role)
	assert.Equal(t, provider.RoleTool, mp.requests[1].Messages[2].Role)
	assert.Equal(t, "call-1", mp.requests[1].Messages[2].ToolCallID)
	assert.Equal(t, "ECHO:hello", mp.requests[1].Messages[2].Content)
}

func TestExecutor_Run_maxToolRoundsExceeded(t *testing.T) {
	mp := &mockProvider{
		responses: []provider.CompletionResponse{
			{
				ToolCalls: []provider.ToolCall{
					{ID: "call-1", Name: "echo", Arguments: `{"text":"hello"}`},
				},
			},
		},
	}
	reg, err := tool.NewRegistry(&echoTool{})
	require.NoError(t, err)

	exec := NewExecutor(ExecutorConfig{
		Provider:      mp,
		Registry:      reg,
		Prompts:       []config.Prompt{{ID: "step1", Content: "go", Tools: []string{"echo"}}},
		MaxToolRounds: 1,
	})

	_, err = exec.Run(context.Background(), RunInput{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "max tool rounds")
}

func TestExecutor_Run_rendersInputAndStepVariables(t *testing.T) {
	mp := &mockProvider{
		responses: []provider.CompletionResponse{
			{Content: "alpha"},
			{Content: "beta"},
		},
	}
	reg, err := tool.NewRegistry()
	require.NoError(t, err)

	exec := NewExecutor(ExecutorConfig{
		Provider:      mp,
		Registry:      reg,
		Prompts:       []config.Prompt{{ID: "a", Content: "Input={{input}}"}, {ID: "b", DependsOn: []string{"a"}, Content: "FromA={{a}}"}},
		MaxToolRounds: 2,
	})

	_, err = exec.Run(context.Background(), RunInput{Variables: map[string]string{"input": "xyz"}})
	require.NoError(t, err)
	require.Len(t, mp.requests, 2)
	assert.Equal(t, "Input=xyz", mp.requests[0].Messages[0].Content)
	assert.Equal(t, "FromA=alpha", mp.requests[1].Messages[0].Content)
}

func TestExecutor_Run_missingTemplateVariableFailsBeforeProviderCall(t *testing.T) {
	mp := &mockProvider{
		responses: []provider.CompletionResponse{{Content: "never"}},
	}
	reg, err := tool.NewRegistry()
	require.NoError(t, err)

	exec := NewExecutor(ExecutorConfig{
		Provider:      mp,
		Registry:      reg,
		Prompts:       []config.Prompt{{ID: "step1", Content: "Missing {{unknown}}"}},
		MaxToolRounds: 2,
	})

	_, err = exec.Run(context.Background(), RunInput{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing template variable")
	assert.Empty(t, mp.requests)
}

func TestExecutor_Run_rejectsDisallowedToolCall(t *testing.T) {
	mp := &mockProvider{
		responses: []provider.CompletionResponse{
			{
				ToolCalls: []provider.ToolCall{
					{ID: "call-1", Name: "secret", Arguments: `{}`},
				},
			},
			{Content: "recovered"},
		},
	}
	reg, err := tool.NewRegistry(&echoTool{}, &secretTool{})
	require.NoError(t, err)

	exec := NewExecutor(ExecutorConfig{
		Provider:      mp,
		Registry:      reg,
		Prompts:       []config.Prompt{{ID: "step1", Content: "go", Tools: []string{"echo"}}},
		MaxToolRounds: 3,
	})

	result, err := exec.Run(context.Background(), RunInput{})
	require.NoError(t, err)
	assert.Equal(t, "recovered", result.Steps[0].Output)
	require.Len(t, mp.requests, 2)
	assert.Equal(t, provider.RoleTool, mp.requests[1].Messages[2].Role)
	assert.Contains(t, mp.requests[1].Messages[2].Content, "not allowed")
}

type secretTool struct{}

func (s *secretTool) Name() string        { return "secret" }
func (s *secretTool) Description() string { return "secret" }
func (s *secretTool) Parameters() map[string]any {
	return map[string]any{"type": "object"}
}
func (s *secretTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	return "SECRET", nil
}

type skillProviderMock struct {
	response provider.CompletionResponse
}

func (s *skillProviderMock) Complete(ctx context.Context, req provider.CompletionRequest) (provider.CompletionResponse, error) {
	return s.response, nil
}

func TestExecutor_Run_invokesSkillTool(t *testing.T) {
	skillProvider := &skillProviderMock{
		response: provider.CompletionResponse{Content: "skill findings"},
	}
	skillTool := skill.NewSkillTool(
		"researcher",
		"Research topics",
		"You are a researcher.",
		skillProvider,
		3,
	)

	mp := &mockProvider{
		responses: []provider.CompletionResponse{
			{
				ToolCalls: []provider.ToolCall{
					{ID: "call-1", Name: "researcher", Arguments: `{"task":"Study Go"}`},
				},
			},
			{Content: "final with skill"},
		},
	}
	reg, err := tool.NewRegistry(skillTool)
	require.NoError(t, err)

	exec := NewExecutor(ExecutorConfig{
		Provider:      mp,
		Registry:      reg,
		Prompts:       []config.Prompt{{ID: "step1", Content: "go", Tools: []string{"researcher"}}},
		MaxToolRounds: 3,
	})

	result, err := exec.Run(context.Background(), RunInput{})
	require.NoError(t, err)
	assert.Equal(t, "final with skill", result.Steps[0].Output)
	require.Len(t, mp.requests, 2)
	assert.Equal(t, "skill findings", mp.requests[1].Messages[2].Content)
}

type depthCapturingTool struct {
	executeDepth int
}

func (d *depthCapturingTool) Name() string        { return "depth_probe" }
func (d *depthCapturingTool) Description() string { return "Captures execution depth" }
func (d *depthCapturingTool) Parameters() map[string]any {
	return map[string]any{"type": "object"}
}
func (d *depthCapturingTool) Execute(ctx context.Context, _ json.RawMessage) (string, error) {
	d.executeDepth = depth.FromContext(ctx)
	return "probed", nil
}

func TestExecutor_Run_recordsToolCallDepthFromContext(t *testing.T) {
	probe := &depthCapturingTool{}
	mp := &mockProvider{
		responses: []provider.CompletionResponse{
			{
				ToolCalls: []provider.ToolCall{
					{ID: "call-1", Name: "depth_probe", Arguments: `{}`},
				},
			},
			{Content: "done"},
		},
	}
	reg, err := tool.NewRegistry(probe)
	require.NoError(t, err)

	exec := NewExecutor(ExecutorConfig{
		Provider:      mp,
		Registry:      reg,
		Prompts:       []config.Prompt{{ID: "step1", Content: "go", Tools: []string{"depth_probe"}}},
		MaxToolRounds: 3,
	})

	ctx := depth.WithIncrement(context.Background())
	result, err := exec.Run(ctx, RunInput{})
	require.NoError(t, err)
	require.Len(t, result.Steps, 1)
	require.Len(t, result.Steps[0].ToolCalls, 1)
	assert.Equal(t, 1, result.Steps[0].ToolCalls[0].Depth)
	assert.Equal(t, 2, probe.executeDepth)
}

func TestExecutor_Run_saveOutputsWritesFile(t *testing.T) {
	root := t.TempDir()
	ws, err := workspace.NewManager(root)
	require.NoError(t, err)

	mp := &mockProvider{
		responses: []provider.CompletionResponse{{Content: "saved text"}},
	}
	reg, err := tool.NewRegistry()
	require.NoError(t, err)

	exec := NewExecutor(ExecutorConfig{
		Provider:      mp,
		Registry:      reg,
		Prompts:       []config.Prompt{{ID: "draft", Content: "write"}},
		MaxToolRounds: 2,
		SaveOutputs:   true,
		Workspace:     NewWorkspaceOutputWriter(ws),
	})

	_, err = exec.Run(context.Background(), RunInput{})
	require.NoError(t, err)

	data, err := ws.Read("outputs/draft.md")
	require.NoError(t, err)
	assert.Equal(t, "saved text", string(data))
}

func TestExecutor_Run_recordsUsagePerStepAndTotal(t *testing.T) {
	tracker := provider.NewUsageTracker()
	mp := &mockProvider{
		responses: []provider.CompletionResponse{
			{Content: "first", Usage: &provider.TokenUsage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15}},
			{Content: "second", Usage: &provider.TokenUsage{PromptTokens: 20, CompletionTokens: 10, TotalTokens: 30}},
		},
	}
	tracked := provider.NewTrackingProvider(mp, "gpt-4o-mini", tracker)
	reg, err := tool.NewRegistry()
	require.NoError(t, err)

	exec := NewExecutor(ExecutorConfig{
		Provider:      tracked,
		Registry:      reg,
		UsageTracker:  tracker,
		Prompts:       []config.Prompt{{ID: "step1", Content: "one"}, {ID: "step2", DependsOn: []string{"step1"}, Content: "two {{step1}}"}},
		MaxToolRounds: 2,
		Model:         "gpt-4o-mini",
	})

	result, err := exec.Run(context.Background(), RunInput{})
	require.NoError(t, err)
	require.Len(t, result.Steps, 2)
	assert.Equal(t, 15, result.Steps[0].Usage.TotalTokens)
	assert.Equal(t, 30, result.Steps[1].Usage.TotalTokens)
	assert.Equal(t, 45, result.Usage.TotalTokens)
}
