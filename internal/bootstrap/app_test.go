package bootstrap

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/1node/airon/internal/provider"
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
	response := m.responses[0]
	m.responses = m.responses[1:]
	return response, nil
}

func TestNewAppWithProvider_WiresRuntimeAndRuns(t *testing.T) {
	root := t.TempDir()
	skillDir := filepath.Join(root, "skills", "researcher")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(`---
name: researcher
description: Research topics
---
You are a research skill.
`), 0o644))

	cfgPath := filepath.Join(root, "airon.yaml")
	cfg := `version: "1"
provider:
  type: openai
  model: gpt-4o-mini
  api_key: test-api-key
workspace:
  path: ./workspace
runtime:
  max_tool_rounds: 2
  max_call_depth: 3
skills:
  - path: ./skills/researcher
agents:
  - id: analyst
    description: Analyst
    system_prompt: You are an analyst.
    tools: [fs_list]
mcp_servers: []
prompts:
  - id: draft
    content: "Write about {{input}}"
    tools: [researcher]
`
	require.NoError(t, os.WriteFile(cfgPath, []byte(cfg), 0o644))

	mock := &mockProvider{
		responses: []provider.CompletionResponse{
			{Content: "Draft complete"},
		},
	}
	app, err := NewAppWithProvider(cfgPath, map[string]string{"input": "Go testing"}, mock)
	require.NoError(t, err)
	require.NotNil(t, app.registry)
	require.NotNil(t, app.workspace)

	_, ok := app.registry.Get("fs_read")
	assert.True(t, ok)
	_, ok = app.registry.Get("researcher")
	assert.True(t, ok)
	_, ok = app.registry.Get("analyst")
	assert.True(t, ok)

	runResult, err := app.Run(context.Background())
	require.NoError(t, err)
	require.Len(t, runResult.Steps, 1)
	assert.Equal(t, "Draft complete", runResult.Steps[0].Output)
	require.Len(t, mock.requests, 1)

	logPath := filepath.Join(app.workspace.Root, "logs", runResult.RunID+".json")
	_, err = os.Stat(logPath)
	require.NoError(t, err)
}
