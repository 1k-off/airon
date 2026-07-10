package executor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/1node/airon/internal/config"
	"github.com/1node/airon/internal/provider"
	"github.com/1node/airon/internal/tool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type failingOutputWriter struct {
	err error
}

func (f *failingOutputWriter) WriteStepOutput(_, _ string) error {
	return f.err
}

type errorProvider struct {
	err error
}

func (e *errorProvider) Complete(context.Context, provider.CompletionRequest) (provider.CompletionResponse, error) {
	return provider.CompletionResponse{}, e.err
}

type brokenTool struct{}

func (b *brokenTool) Name() string        { return "broken" }
func (b *brokenTool) Description() string { return "broken" }
func (b *brokenTool) Parameters() map[string]any {
	return map[string]any{"type": "object"}
}
func (b *brokenTool) Execute(context.Context, json.RawMessage) (string, error) {
	return "", errors.New("tool exploded")
}

func TestExecutor_Run_saveOutputsFailure(t *testing.T) {
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
		Workspace:     &failingOutputWriter{err: fmt.Errorf("disk full")},
	})

	_, err = exec.Run(context.Background(), RunInput{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "disk full")
}

func TestExecutor_Run_providerCompleteError(t *testing.T) {
	reg, err := tool.NewRegistry()
	require.NoError(t, err)

	exec := NewExecutor(ExecutorConfig{
		Provider:      &errorProvider{err: errors.New("provider down")},
		Registry:      reg,
		Prompts:       []config.Prompt{{ID: "step1", Content: "go"}},
		MaxToolRounds: 2,
	})

	_, err = exec.Run(context.Background(), RunInput{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "provider complete")
}

func TestExecutor_Run_systemPromptRenderError(t *testing.T) {
	mp := &mockProvider{
		responses: []provider.CompletionResponse{{Content: "never"}},
	}
	reg, err := tool.NewRegistry()
	require.NoError(t, err)

	exec := NewExecutor(ExecutorConfig{
		Provider:      mp,
		Registry:      reg,
		Prompts:       []config.Prompt{{ID: "step1", System: "Hi {{missing}}", Content: "go"}},
		MaxToolRounds: 2,
	})

	_, err = exec.Run(context.Background(), RunInput{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing template variable")
	assert.Empty(t, mp.requests)
}

func TestExecutor_Run_sortPromptsCycleFails(t *testing.T) {
	reg, err := tool.NewRegistry()
	require.NoError(t, err)

	exec := NewExecutor(ExecutorConfig{
		Provider: &mockProvider{},
		Registry: reg,
		Prompts: []config.Prompt{
			{ID: "a", DependsOn: []string{"b"}, Content: "a"},
			{ID: "b", DependsOn: []string{"a"}, Content: "b"},
		},
		MaxToolRounds: 2,
	})

	_, err = exec.Run(context.Background(), RunInput{})
	require.Error(t, err)
}

func TestExecutor_stepTools_unknownToolFails(t *testing.T) {
	reg, err := tool.NewRegistry(&echoTool{})
	require.NoError(t, err)

	exec := NewExecutor(ExecutorConfig{
		Provider:      &mockProvider{},
		Registry:      reg,
		Prompts:       []config.Prompt{{ID: "step1", Content: "go", Tools: []string{"missing"}}},
		MaxToolRounds: 2,
	})

	_, err = exec.Run(context.Background(), RunInput{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown tool")
}

func TestExecutor_stepTools_nilRegistryWithToolsFails(t *testing.T) {
	exec := NewExecutor(ExecutorConfig{
		Provider:      &mockProvider{},
		Registry:      nil,
		Prompts:       []config.Prompt{{ID: "step1", Content: "go", Tools: []string{"echo"}}},
		MaxToolRounds: 2,
	})

	_, err := exec.Run(context.Background(), RunInput{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tool registry is required")
}

func TestExecutor_Run_toolExecuteErrorIsCaptured(t *testing.T) {
	mp := &mockProvider{
		responses: []provider.CompletionResponse{
			{
				ToolCalls: []provider.ToolCall{
					{ID: "call-1", Name: "broken", Arguments: `{}`},
				},
			},
			{Content: "recovered"},
		},
	}
	reg, err := tool.NewRegistry(&brokenTool{})
	require.NoError(t, err)

	exec := NewExecutor(ExecutorConfig{
		Provider:      mp,
		Registry:      reg,
		Prompts:       []config.Prompt{{ID: "step1", Content: "go", Tools: []string{"broken"}}},
		MaxToolRounds: 3,
	})

	result, err := exec.Run(context.Background(), RunInput{})
	require.NoError(t, err)
	require.Len(t, result.Steps[0].ToolCalls, 1)
	assert.Contains(t, result.Steps[0].ToolCalls[0].Result, "tool exploded")
}

func TestNewExecutor_defaultsMaxToolRounds(t *testing.T) {
	exec := NewExecutor(ExecutorConfig{MaxToolRounds: 0})
	assert.Equal(t, 10, exec.maxToolRounds)
}

func TestWorkspaceOutputWriter_nilManagerFails(t *testing.T) {
	writer := NewWorkspaceOutputWriter(nil)
	err := writer.WriteStepOutput("draft", "content")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not configured")
}

func TestExecutor_Run_rendersSystemPrompt(t *testing.T) {
	mp := &mockProvider{
		responses: []provider.CompletionResponse{{Content: "done"}},
	}
	reg, err := tool.NewRegistry()
	require.NoError(t, err)

	exec := NewExecutor(ExecutorConfig{
		Provider:      mp,
		Registry:      reg,
		Prompts:       []config.Prompt{{ID: "step1", System: "You are helpful", Content: "go"}},
		MaxToolRounds: 2,
	})

	_, err = exec.Run(context.Background(), RunInput{})
	require.NoError(t, err)
	require.Len(t, mp.requests, 1)
	require.Len(t, mp.requests[0].Messages, 2)
	assert.Equal(t, provider.RoleSystem, mp.requests[0].Messages[0].Role)
	assert.Equal(t, "You are helpful", mp.requests[0].Messages[0].Content)
}

func TestExecutor_Run_skipsSaveOutputsForEmptyOutput(t *testing.T) {
	mp := &mockProvider{
		responses: []provider.CompletionResponse{{Content: ""}},
	}
	reg, err := tool.NewRegistry()
	require.NoError(t, err)

	writer := &failingOutputWriter{err: errors.New("should not be called")}
	exec := NewExecutor(ExecutorConfig{
		Provider:      mp,
		Registry:      reg,
		Prompts:       []config.Prompt{{ID: "draft", Content: "write"}},
		MaxToolRounds: 2,
		SaveOutputs:   true,
		Workspace:     writer,
	})

	_, err = exec.Run(context.Background(), RunInput{})
	require.NoError(t, err)
}
