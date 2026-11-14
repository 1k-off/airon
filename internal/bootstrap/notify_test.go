package bootstrap

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/1k-off/airon/internal/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordingHandler struct {
	mu         sync.Mutex
	requests   []string
	bodyTexts  []string
}

func (r *recordingHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	body, _ := io.ReadAll(req.Body)
	r.mu.Lock()
	r.requests = append(r.requests, req.URL.Path)
	r.bodyTexts = append(r.bodyTexts, string(body))
	r.mu.Unlock()
	w.WriteHeader(http.StatusOK)
}

func (r *recordingHandler) paths() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, len(r.requests))
	copy(out, r.requests)
	return out
}

func (r *recordingHandler) bodies() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, len(r.bodyTexts))
	copy(out, r.bodyTexts)
	return out
}

func writeNotifyConfig(t *testing.T, hcURL string) string {
	t.Helper()
	root := t.TempDir()
	cfgPath := filepath.Join(root, "airon.yaml")
	cfg := `version: "1"
provider:
  type: openai
  model: gpt-4o-mini
  api_key: test-api-key
workspace:
  path: ./workspace
notify:
  healthchecks:
    url: ` + hcURL + `
skills: []
agents: []
mcp_servers: []
prompts:
  - id: draft
    content: "Hello {{input}}"
`
	require.NoError(t, os.WriteFile(cfgPath, []byte(cfg), 0o644))
	return cfgPath
}

func TestHealthchecksDisabled_matrix(t *testing.T) {
	tests := []struct {
		name       string
		optDisabled bool
		env        string
		want       bool
	}{
		{name: "opt false, env empty", want: false},
		{name: "opt true short-circuits", optDisabled: true, want: true},
		{name: "env 0 disables", env: "0", want: true},
		{name: "env false disables", env: "false", want: true},
		{name: "env FALSE disables (case-insensitive)", env: "FALSE", want: true},
		{name: "env no disables", env: "no", want: true},
		{name: "env off disables", env: "off", want: true},
		{name: "env 1 does not disable", env: "1", want: false},
		{name: "env true does not disable", env: "true", want: false},
		{name: "env with surrounding whitespace", env: "  0  ", want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := healthchecksDisabled(tt.optDisabled, tt.env)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestApp_Run_pingsHealthchecksStartAndSuccess(t *testing.T) {
	handler := &recordingHandler{}
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	cfgPath := writeNotifyConfig(t, server.URL)

	mock := &mockProvider{responses: []provider.CompletionResponse{{Content: "ok"}}}
	app, err := NewAppWithProviderAndOptions(context.Background(), cfgPath, map[string]string{"input": "x"}, mock, AppOptions{})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, app.Close()) })

	require.NotNil(t, app.notifier)
	require.True(t, app.notifier.Enabled(), "notifier should be enabled for a real URL")
	app.notifier.WithHTTPClient(server.Client())

	_, err = app.Run(context.Background())
	require.NoError(t, err)

	paths := handler.paths()
	require.Len(t, paths, 2)
	assert.Equal(t, "/start", paths[0])
	assert.Equal(t, "/", paths[1])
}

func TestApp_Run_pingsHealthchecksFailOnAgentFAILOutcome(t *testing.T) {
	handler := &recordingHandler{}
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	cfgPath := writeNotifyConfig(t, server.URL)
	// Enable pass_fail outcome on the written config.
	raw, err := os.ReadFile(cfgPath)
	require.NoError(t, err)
	patched := strings.Replace(string(raw), "workspace:\n  path: ./workspace\n",
		"workspace:\n  path: ./workspace\nruntime:\n  outcome: pass_fail\n", 1)
	require.NoError(t, os.WriteFile(cfgPath, []byte(patched), 0o600))

	mock := &mockProvider{responses: []provider.CompletionResponse{{Content: "FAIL: nested bak missing"}}}
	app, err := NewAppWithProviderAndOptions(context.Background(), cfgPath, map[string]string{"input": "x"}, mock, AppOptions{})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, app.Close()) })
	app.notifier.WithHTTPClient(server.Client())

	_, err = app.Run(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "agent reported failure")

	paths := handler.paths()
	require.GreaterOrEqual(t, len(paths), 2)
	assert.Equal(t, "/start", paths[0])
	assert.Equal(t, "/fail", paths[len(paths)-1])
	bodies := handler.bodies()
	require.NotEmpty(t, bodies)
	assert.Contains(t, bodies[len(bodies)-1], "nested bak missing")
}

func TestApp_Run_pingsHealthchecksFailOnRunError(t *testing.T) {
	handler := &recordingHandler{}
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	cfgPath := writeNotifyConfig(t, server.URL)

	// The template references {{missing}} which the executor cannot resolve —
	// so the run fails and the notifier should ping /fail.
	root := filepath.Dir(cfgPath)
	failCfg := `version: "1"
provider:
  type: openai
  model: gpt-4o-mini
  api_key: test-api-key
workspace:
  path: ./workspace
notify:
  healthchecks:
    url: ` + server.URL + `
skills: []
agents: []
mcp_servers: []
prompts:
  - id: draft
    content: "Hello {{missing}}"
`
	failCfgPath := filepath.Join(root, "fail.yaml")
	require.NoError(t, os.WriteFile(failCfgPath, []byte(failCfg), 0o644))

	app, err := NewAppWithProviderAndOptions(context.Background(), failCfgPath, nil, &mockProvider{}, AppOptions{})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, app.Close()) })

	app.notifier.WithHTTPClient(server.Client())

	_, runErr := app.Run(context.Background())
	require.Error(t, runErr)

	paths := handler.paths()
	require.Len(t, paths, 2)
	assert.Equal(t, "/start", paths[0])
	assert.Equal(t, "/fail", paths[1])
}

func TestApp_Run_noPingsWhenAppOptionsDisable(t *testing.T) {
	handler := &recordingHandler{}
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	cfgPath := writeNotifyConfig(t, server.URL)

	mock := &mockProvider{responses: []provider.CompletionResponse{{Content: "ok"}}}
	app, err := NewAppWithProviderAndOptions(context.Background(), cfgPath, map[string]string{"input": "x"}, mock, AppOptions{NoHealthchecks: true})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, app.Close()) })

	require.NotNil(t, app.notifier)
	assert.False(t, app.notifier.Enabled(), "notifier should be disabled by AppOptions")

	_, err = app.Run(context.Background())
	require.NoError(t, err)

	assert.Empty(t, handler.paths(), "no HTTP pings should be made when NoHealthchecks=true")
}

func TestApp_Run_noPingsWhenEnvDisables(t *testing.T) {
	handler := &recordingHandler{}
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	cfgPath := writeNotifyConfig(t, server.URL)

	t.Setenv(EnvHealthchecks, "0")

	mock := &mockProvider{responses: []provider.CompletionResponse{{Content: "ok"}}}
	app, err := NewAppWithProviderAndOptions(context.Background(), cfgPath, map[string]string{"input": "x"}, mock, AppOptions{})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, app.Close()) })

	assert.False(t, app.notifier.Enabled(), "notifier should be disabled by AIRON_HEALTHCHECKS=0")

	_, err = app.Run(context.Background())
	require.NoError(t, err)

	assert.Empty(t, handler.paths(), "no HTTP pings should be made when env disables")
}

func TestApp_Run_noPingsWhenURLEmpty(t *testing.T) {
	handler := &recordingHandler{}
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	// Config without notify URL.
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
    content: "Hello {{input}}"
`
	require.NoError(t, os.WriteFile(cfgPath, []byte(cfg), 0o644))

	mock := &mockProvider{responses: []provider.CompletionResponse{{Content: "ok"}}}
	app, err := NewAppWithProviderAndOptions(context.Background(), cfgPath, map[string]string{"input": "x"}, mock, AppOptions{})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, app.Close()) })

	assert.False(t, app.notifier.Enabled(), "empty URL should keep notifier disabled")

	_, err = app.Run(context.Background())
	require.NoError(t, err)
	assert.Empty(t, handler.paths())
}

func TestApp_Run_notifyHTTPErrorDoesNotOverrideRunError(t *testing.T) {
	// Server that always returns 500 — the fail ping will error, but the
	// primary run error must survive intact.
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
notify:
  healthchecks:
    url: ` + server.URL + `
skills: []
agents: []
mcp_servers: []
prompts:
  - id: draft
    content: "Hello {{missing}}"
`
	require.NoError(t, os.WriteFile(cfgPath, []byte(cfg), 0o644))

	app, err := NewAppWithProviderAndOptions(context.Background(), cfgPath, nil, &mockProvider{}, AppOptions{})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, app.Close()) })

	app.notifier.WithHTTPClient(server.Client())

	_, runErr := app.Run(context.Background())
	require.Error(t, runErr)
	// The error should mention the missing template variable, not any HTTP
	// status from the healthchecks server.
	assert.NotContains(t, runErr.Error(), "500")
}

func TestApp_Run_notifyHTTPErrorDoesNotOverrideRunSuccess(t *testing.T) {
	// Server returning 500 for the success ping — run itself should still be
	// reported as successful (nil error).
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)

	cfgPath := writeNotifyConfig(t, server.URL)

	mock := &mockProvider{responses: []provider.CompletionResponse{{Content: "ok"}}}
	app, err := NewAppWithProviderAndOptions(context.Background(), cfgPath, map[string]string{"input": "x"}, mock, AppOptions{})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, app.Close()) })

	app.notifier.WithHTTPClient(server.Client())

	_, err = app.Run(context.Background())
	require.NoError(t, err, "healthchecks ping failure must not surface as run error")
}

func TestLogNotifyError_safeOnNilReceiverOrNilError(t *testing.T) {
	var app *App
	app.logNotifyError("start", nil) // must not panic
	app = &App{}
	app.logNotifyError("start", errors.New("boom")) // slog nil, must not panic
}

func TestApp_Run_failPingSurvivesCanceledContext(t *testing.T) {
	handler := &recordingHandler{}
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	cfgPath := writeNotifyConfig(t, server.URL)
	mock := &mockProvider{err: errors.New("provider boom")}
	app, err := NewAppWithProviderAndOptions(context.Background(), cfgPath, map[string]string{"input": "x"}, mock, AppOptions{})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, app.Close()) })
	app.notifier.WithHTTPClient(server.Client())

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, runErr := app.Run(ctx)
	require.Error(t, runErr)

	paths := handler.paths()
	assert.Contains(t, paths, "/fail", "fail ping must use an uncanceled context")
}
