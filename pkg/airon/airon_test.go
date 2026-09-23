package airon

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/1node/airon/internal/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockProvider is a fake provider whose responses are pre-queued. It records
// completion requests so tests can assert the executor was actually invoked.
type mockProvider struct {
	mu        sync.Mutex
	requests  []provider.CompletionRequest
	responses []provider.CompletionResponse
	err       error
}

func (m *mockProvider) Complete(_ context.Context, req provider.CompletionRequest) (provider.CompletionResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.requests = append(m.requests, req)
	if m.err != nil {
		return provider.CompletionResponse{}, m.err
	}
	if len(m.responses) == 0 {
		return provider.CompletionResponse{}, nil
	}
	resp := m.responses[0]
	m.responses = m.responses[1:]
	return resp, nil
}

func (m *mockProvider) requestCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.requests)
}

// setTestProvider installs a package-level provider override that Run picks
// up. Cleanup restores the previous (nil) value at test end.
func setTestProvider(t *testing.T, p provider.Provider) {
	t.Helper()
	prev := testProviderOverride
	testProviderOverride = p
	t.Cleanup(func() {
		testProviderOverride = prev
	})
}

func writeConfig(t *testing.T, extra string) string {
	t.Helper()
	root := t.TempDir()
	cfgPath := filepath.Join(root, "airon.yaml")
	base := `version: "1"
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
		base = extra
	}
	require.NoError(t, os.WriteFile(cfgPath, []byte(base), 0o644))
	return cfgPath
}

func TestRun_executesConfiguredPromptWithInjectedProvider(t *testing.T) {
	cfgPath := writeConfig(t, "")

	mock := &mockProvider{responses: []provider.CompletionResponse{{Content: "Draft complete"}}}
	setTestProvider(t, mock)

	res, err := Run(context.Background(), Options{
		ConfigPath:     cfgPath,
		Input:          map[string]string{"input": "world"},
		NoHealthchecks: true,
	})

	require.NoError(t, err)
	assert.True(t, res.Success)
	assert.Empty(t, res.Error)
	require.NotEmpty(t, res.RunID)
	require.Len(t, res.Steps, 1)
	assert.Equal(t, "draft", res.Steps[0].ID)
	assert.Equal(t, "Draft complete", res.Steps[0].Output)
	assert.Equal(t, 1, mock.requestCount())
}

func TestRun_returnsErrorWhenConfigMissing(t *testing.T) {
	_, err := Run(context.Background(), Options{ConfigPath: filepath.Join(t.TempDir(), "nope.yaml")})
	require.Error(t, err)
}

func TestRun_populatesResultOnPromptError(t *testing.T) {
	cfgPath := writeConfig(t, `version: "1"
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
    content: "Needs {{missing}}"
`)

	setTestProvider(t, &mockProvider{})

	res, err := Run(context.Background(), Options{
		ConfigPath:     cfgPath,
		NoHealthchecks: true,
	})
	require.Error(t, err)
	assert.False(t, res.Success)
	assert.NotEmpty(t, res.Error)
	assert.Contains(t, res.Error, err.Error())
}

func TestRun_nilContextDefaultsToBackground(t *testing.T) {
	cfgPath := writeConfig(t, "")
	setTestProvider(t, &mockProvider{responses: []provider.CompletionResponse{{Content: "ok"}}})

	// Exercise Run's nil-context guard via a helper that intentionally
	// hides the untyped nil from staticcheck's SA1012 lint.
	res, err := Run(nilContext(), Options{
		ConfigPath:     cfgPath,
		Input:          map[string]string{"input": "x"},
		NoHealthchecks: true,
	})
	require.NoError(t, err)
	assert.True(t, res.Success)
}

// nilContext returns a nil context.Context. It exists solely to exercise the
// nil-context guard inside Run without triggering SA1012 at the call site.
func nilContext() context.Context {
	return nil
}

func TestOptions_NoHealthchecksZeroValueDoesNotPanic(t *testing.T) {
	cfgPath := writeConfig(t, "")
	setTestProvider(t, &mockProvider{responses: []provider.CompletionResponse{{Content: "ok"}}})

	res, err := Run(context.Background(), Options{
		ConfigPath: cfgPath,
		Input:      map[string]string{"input": "x"},
	})
	require.NoError(t, err)
	assert.True(t, res.Success)
}
