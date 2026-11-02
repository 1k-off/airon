package tool

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/1k-off/airon/internal/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubTool struct {
	name        string
	description string
	parameters  map[string]any
	execute     func(ctx context.Context, args json.RawMessage) (string, error)
}

func (s *stubTool) Name() string        { return s.name }
func (s *stubTool) Description() string { return s.description }
func (s *stubTool) Parameters() map[string]any {
	return s.parameters
}
func (s *stubTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	return s.execute(ctx, args)
}

func newStubTool(name, description string, parameters map[string]any) *stubTool {
	return &stubTool{
		name:        name,
		description: description,
		parameters:  parameters,
		execute: func(ctx context.Context, args json.RawMessage) (string, error) {
			return "result from " + name, nil
		},
	}
}

func TestNewRegistry_registersToolsSuccessfully(t *testing.T) {
	toolA := newStubTool("alpha", "first tool", map[string]any{"type": "object"})
	toolB := newStubTool("beta", "second tool", map[string]any{"type": "object", "properties": map[string]any{}})

	reg, err := NewRegistry(toolA, toolB)
	require.NoError(t, err)
	require.NotNil(t, reg)

	tools := reg.List()
	require.Len(t, tools, 2)
	assert.Equal(t, "alpha", tools[0].Name())
	assert.Equal(t, "beta", tools[1].Name())
}

func TestNewRegistry_duplicateNameReturnsError(t *testing.T) {
	toolA := newStubTool("dup", "first", nil)
	toolB := newStubTool("dup", "second", nil)

	_, err := NewRegistry(toolA, toolB)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate")
	assert.Contains(t, err.Error(), "dup")
}

func TestRegistry_Get_findsTool(t *testing.T) {
	toolA := newStubTool("alpha", "first tool", nil)
	reg, err := NewRegistry(toolA)
	require.NoError(t, err)

	got, ok := reg.Get("alpha")
	require.True(t, ok)
	assert.Equal(t, "alpha", got.Name())

	_, ok = reg.Get("missing")
	assert.False(t, ok)
}

func TestRegistry_Execute_dispatchesToCorrectTool(t *testing.T) {
	var receivedArgs json.RawMessage

	toolA := newStubTool("alpha", "first tool", nil)
	toolA.execute = func(ctx context.Context, args json.RawMessage) (string, error) {
		receivedArgs = args
		return "alpha-result", nil
	}
	toolB := newStubTool("beta", "second tool", nil)
	toolB.execute = func(ctx context.Context, args json.RawMessage) (string, error) {
		return "beta-result", nil
	}

	reg, err := NewRegistry(toolA, toolB)
	require.NoError(t, err)

	result, err := reg.Execute(context.Background(), provider.ToolCall{
		ID:        "call-1",
		Name:      "alpha",
		Arguments: `{"task":"analyze"}`,
	})
	require.NoError(t, err)
	assert.Equal(t, "alpha-result", result)
	assert.JSONEq(t, `{"task":"analyze"}`, string(receivedArgs))

	result, err = reg.Execute(context.Background(), provider.ToolCall{
		ID:        "call-2",
		Name:      "beta",
		Arguments: `{}`,
	})
	require.NoError(t, err)
	assert.Equal(t, "beta-result", result)
}

func TestRegistry_Definitions_exportsSchemas(t *testing.T) {
	paramsA := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"task": map[string]any{"type": "string"},
		},
		"required": []any{"task"},
	}
	paramsB := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"message": map[string]any{"type": "string"},
		},
	}

	toolA := newStubTool("skill_researcher", "Research topics", paramsA)
	toolB := newStubTool("agent_analyst", "Data analyst", paramsB)

	reg, err := NewRegistry(toolA, toolB)
	require.NoError(t, err)

	defs := reg.Definitions()
	require.Len(t, defs, 2)

	assert.Equal(t, provider.ToolDefinition{
		Name:        "skill_researcher",
		Description: "Research topics",
		Parameters:  paramsA,
	}, defs[0])
	assert.Equal(t, provider.ToolDefinition{
		Name:        "agent_analyst",
		Description: "Data analyst",
		Parameters:  paramsB,
	}, defs[1])
}
