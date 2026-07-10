package bootstrap

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/1node/airon/internal/provider"
	"github.com/modelcontextprotocol/go-sdk/mcp"
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

func newBootstrapMCPServer(t *testing.T) *httptest.Server {
	t.Helper()

	server := mcp.NewServer(&mcp.Implementation{Name: "mock-mcp", Version: "1.0.0"}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "echo", Description: "Echo input"}, func(_ context.Context, _ *mcp.CallToolRequest, params *struct {
		Message string `json:"message"`
	}) (*mcp.CallToolResult, any, error) {
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: params.Message}}}, nil, nil
	})

	handler := mcp.NewStreamableHTTPHandler(func(_ *http.Request) *mcp.Server { return server }, nil)
	return httptest.NewServer(handler)
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
	require.NoError(t, app.Close())
}

func TestNewAppWithOptions_enablesStreaming(t *testing.T) {
	root := t.TempDir()
	cfgPath := filepath.Join(root, "airon.yaml")
	cfg := `version: "1"
provider:
  type: openai
  model: gpt-4o-mini
  api_key: test-api-key
workspace:
  path: ./workspace
runtime:
  stream: true
skills: []
agents: []
mcp_servers: []
prompts:
  - id: draft
    content: "Hello"
`
	require.NoError(t, os.WriteFile(cfgPath, []byte(cfg), 0o644))

	app, err := NewAppWithProvider(cfgPath, nil, &mockProvider{
		responses: []provider.CompletionResponse{{Content: "ok"}},
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, app.Close())
	})

	_, err = app.Run(context.Background())
	require.NoError(t, err)
}

func TestNewAppWithProvider_unknownPromptToolFails(t *testing.T) {
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
skills:
  - path: ./skills/researcher
agents: []
mcp_servers: []
prompts:
  - id: draft
    content: "Write about {{input}}"
    tools: [missing_tool]
`
	require.NoError(t, os.WriteFile(cfgPath, []byte(cfg), 0o644))

	_, err := NewAppWithProvider(cfgPath, nil, &mockProvider{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `unknown tool "missing_tool"`)
}

func TestNewAppWithProvider_connectsMCPServer(t *testing.T) {
	mockServer := newBootstrapMCPServer(t)
	t.Cleanup(mockServer.Close)

	root := t.TempDir()
	cfgPath := filepath.Join(root, "airon.yaml")
	cfg := `version: "1"
provider:
  type: openai
  model: gpt-4o-mini
  api_key: test-api-key
workspace:
  path: ./workspace
skills: []
agents: []
mcp_servers:
  - name: mock
    transport: streamable_http
    url: ` + mockServer.URL + `
prompts:
  - id: draft
    content: "Write about {{input}}"
    tools: [mock__echo]
`
	require.NoError(t, os.WriteFile(cfgPath, []byte(cfg), 0o644))

	mock := &mockProvider{
		responses: []provider.CompletionResponse{
			{Content: "done"},
		},
	}
	app, err := NewAppWithProvider(cfgPath, map[string]string{"input": "mcp"}, mock)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, app.Close())
	})

	_, ok := app.registry.Get("mock__echo")
	assert.True(t, ok)
}

func TestNewAppWithProvider_nilProviderFails(t *testing.T) {
	_, err := NewAppWithProvider("airon.yaml", nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "provider is required")
}

func TestParseLevel(t *testing.T) {
	assert.Equal(t, slog.LevelDebug, parseLevel("debug"))
	assert.Equal(t, slog.LevelWarn, parseLevel("warn"))
	assert.Equal(t, slog.LevelError, parseLevel("error"))
	assert.Equal(t, slog.LevelInfo, parseLevel("info"))
}

