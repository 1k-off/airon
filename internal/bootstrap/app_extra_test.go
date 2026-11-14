package bootstrap

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/1k-off/airon/internal/executor"
	"github.com/1k-off/airon/internal/mcp"
	"github.com/1k-off/airon/internal/provider"
	mcpgo "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type bootstrapFailingSession struct {
	closeErr error
}

func (s *bootstrapFailingSession) ListTools(context.Context, *mcpgo.ListToolsParams) (*mcpgo.ListToolsResult, error) {
	return &mcpgo.ListToolsResult{}, nil
}

func (s *bootstrapFailingSession) CallTool(context.Context, *mcpgo.CallToolParams) (*mcpgo.CallToolResult, error) {
	return nil, nil
}

func (s *bootstrapFailingSession) Close() error {
	return s.closeErr
}

func writeMinimalBootstrapConfig(t *testing.T, root string, extra string) string {
	t.Helper()

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
mcp_servers: []
prompts:
  - id: draft
    content: "Hello {{input}}"
`
	if extra != "" {
		cfg = extra
	}
	require.NoError(t, os.WriteFile(cfgPath, []byte(cfg), 0o644))
	return cfgPath
}

func TestNewApp_loadsProviderFromConfig(t *testing.T) {
	root := t.TempDir()
	cfgPath := writeMinimalBootstrapConfig(t, root, "")

	app, err := NewApp(cfgPath, map[string]string{"input": "world"})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, app.Close()) })
	require.NotNil(t, app.provider)
}

func TestNewAppWithOptions_appliesStreamOption(t *testing.T) {
	root := t.TempDir()
	cfgPath := writeMinimalBootstrapConfig(t, root, `version: "1"
provider:
  type: openai
  model: gpt-4o-mini
  api_key: test-api-key
workspace:
  path: ./workspace
runtime:
  stream: false
skills: []
agents: []
mcp_servers: []
prompts:
  - id: draft
    content: "Hello"
`)

	app, err := NewAppWithOptions(context.Background(), cfgPath, nil, AppOptions{Stream: true})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, app.Close()) })
	require.NotNil(t, app.executor)
}

func TestNewAppWithProvider_configLoadError(t *testing.T) {
	_, err := NewAppWithProvider(filepath.Join(t.TempDir(), "missing.yaml"), nil, &mockProvider{})
	require.Error(t, err)
}

func TestNewAppWithProvider_invalidProviderType(t *testing.T) {
	root := t.TempDir()
	cfgPath := writeMinimalBootstrapConfig(t, root, `version: "1"
provider:
  type: unknown
  model: x
  api_key: key
workspace:
  path: ./workspace
skills: []
agents: []
mcp_servers: []
prompts:
  - id: draft
    content: "Hello"
`)

	_, err := NewApp(cfgPath, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported provider")
}

func TestNewAppWithProvider_missingSkillPathFails(t *testing.T) {
	root := t.TempDir()
	cfgPath := filepath.Join(root, "airon.yaml")
	cfg := `version: "1"
provider:
  type: openai
  model: gpt-4o-mini
  api_key: test-api-key
workspace:
  path: ./workspace
skills:
  - path: ./skills/missing
agents: []
mcp_servers: []
prompts:
  - id: draft
    content: "Hello"
`
	require.NoError(t, os.WriteFile(cfgPath, []byte(cfg), 0o644))

	_, err := NewAppWithProvider(cfgPath, nil, &mockProvider{})
	require.Error(t, err)
}

func TestNewAppWithProvider_mcpListToolsFailureClosesPriorClients(t *testing.T) {
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
  - name: good
    transport: streamable_http
    url: ` + mockServer.URL + `
  - name: bad
    transport: streamable_http
    url: http://127.0.0.1:1/mcp
prompts:
  - id: draft
    content: "Hello"
`
	require.NoError(t, os.WriteFile(cfgPath, []byte(cfg), 0o644))

	_, err := NewAppWithProvider(cfgPath, nil, &mockProvider{})
	require.Error(t, err)
}

func TestNewAppWithProvider_agentLoadFailure(t *testing.T) {
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
agents:
  - id: analyst
    description: Analyst
    system_prompt: You are an analyst.
    tools: [missing_tool]
mcp_servers: []
prompts:
  - id: draft
    content: "Hello"
`
	require.NoError(t, os.WriteFile(cfgPath, []byte(cfg), 0o644))

	_, err := NewAppWithProvider(cfgPath, nil, &mockProvider{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown tool")
}

func TestApp_Close_aggregatesMCPClientErrors(t *testing.T) {
	app := &App{
		mcpClients: []*mcp.Client{
			mcp.NewClientForTest("one", &bootstrapFailingSession{closeErr: errors.New("close one")}),
			mcp.NewClientForTest("two", &bootstrapFailingSession{closeErr: errors.New("close two")}),
		},
	}

	err := app.Close()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "close one")
	assert.Contains(t, err.Error(), "close two")
}

func TestApp_Close_noClientsIsNoop(t *testing.T) {
	app := &App{}
	require.NoError(t, app.Close())
}

func TestApp_Run_executorErrorIsRecorded(t *testing.T) {
	root := t.TempDir()
	cfgPath := writeMinimalBootstrapConfig(t, root, `version: "1"
provider:
  type: openai
  model: gpt-4o-mini
  api_key: test-api-key
workspace:
  path: ./workspace
skills: []
agents: []
mcp_servers: []
prompts:
  - id: draft
    content: "Missing {{unknown}}"
`)

	app, err := NewAppWithProvider(cfgPath, nil, &mockProvider{})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, app.Close()) })

	_, err = app.Run(context.Background())
	require.Error(t, err)
}

func TestApp_Run_flushFailureWithRunError(t *testing.T) {
	root := t.TempDir()
	cfgPath := writeMinimalBootstrapConfig(t, root, "")

	app, err := NewAppWithProvider(cfgPath, nil, &mockProvider{
		responses: []provider.CompletionResponse{{Content: "ok"}},
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = app.Close() })

	logsPath := filepath.Join(app.workspace.Root, "logs")
	require.NoError(t, os.MkdirAll(app.workspace.Root, 0o755))
	require.NoError(t, os.WriteFile(logsPath, []byte("block"), 0o644))

	_, err = app.Run(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "flush run log")
}

func TestApp_Run_recordsToolCallsInStepLog(t *testing.T) {
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
mcp_servers: []
prompts:
  - id: draft
    content: "go"
    tools: [fs_list]
`
	require.NoError(t, os.WriteFile(cfgPath, []byte(cfg), 0o644))

	mock := &mockProvider{
		responses: []provider.CompletionResponse{
			{
				Content: "listing",
				ToolCalls: []provider.ToolCall{
					{ID: "call-1", Name: "fs_list", Arguments: `{"path":"."}`},
				},
			},
			{Content: "done"},
		},
	}
	app, err := NewAppWithProvider(cfgPath, nil, mock)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, app.Close()) })

	result, err := app.Run(context.Background())
	require.NoError(t, err)
	require.Len(t, result.Steps, 1)
	require.Len(t, result.Steps[0].ToolCalls, 1)
	assert.Equal(t, "fs_list", result.Steps[0].ToolCalls[0].Name)
}

func TestNewAppWithProvider_mcpConnectValidationError(t *testing.T) {
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
  - name: ""
    transport: streamable_http
    url: http://example.com/mcp
prompts:
  - id: draft
    content: "Hello"
`
	require.NoError(t, os.WriteFile(cfgPath, []byte(cfg), 0o644))

	_, err := NewAppWithProvider(cfgPath, nil, &mockProvider{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name is required")
}

func TestNewAppWithProvider_listToolsHTTPFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)

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
  - name: broken
    transport: streamable_http
    url: ` + server.URL + `
prompts:
  - id: draft
    content: "Hello"
`
	require.NoError(t, os.WriteFile(cfgPath, []byte(cfg), 0o644))

	_, err := NewAppWithProvider(cfgPath, nil, &mockProvider{})
	require.Error(t, err)
}

func TestKnownToolNames_nilRegistry(t *testing.T) {
	names := knownToolNames(nil)
	assert.Empty(t, names)
}

func TestToStepRecord_mapsToolCallFields(t *testing.T) {
	record := toStepRecord(executor.StepResult{
		ID:     "step1",
		Output: "out",
		ToolCalls: []executor.ToolCallResult{
			{
				ID:        "c1",
				Name:      "echo",
				Arguments: `{"x":1}`,
				Result:    "ok",
				Error:     "boom",
				Depth:     2,
			},
		},
	})
	require.Len(t, record.ToolCalls, 1)
	assert.Equal(t, "c1", record.ToolCalls[0].ID)
	assert.Equal(t, "boom", record.ToolCalls[0].Error)
	assert.Equal(t, 2, record.ToolCalls[0].Depth)
}

func TestCloneInput_copiesValues(t *testing.T) {
	src := map[string]string{"a": "1", "b": "2"}
	cloned := cloneInput(src)
	cloned["a"] = "changed"
	assert.Equal(t, "1", src["a"])
}
