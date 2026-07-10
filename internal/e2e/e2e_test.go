//go:build integration

package e2e_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/1node/airon/internal/bootstrap"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	defaultOpenAIE2EModel     = "gpt-4o-mini"
	defaultAnthropicE2EModel  = "claude-haiku-4-5"
)

func openAIE2EModel() string {
	if model := strings.TrimSpace(os.Getenv("OPENAI_E2E_MODEL")); model != "" {
		return model
	}
	return defaultOpenAIE2EModel
}

func anthropicE2EModel() string {
	if model := strings.TrimSpace(os.Getenv("ANTHROPIC_E2E_MODEL")); model != "" {
		return model
	}
	return defaultAnthropicE2EModel
}

func TestE2E_OpenAI_run(t *testing.T) {
	apiKey := strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY not set")
	}

	model := openAIE2EModel()

	cfgPath := writeProviderConfig(t, providerConfig{
		providerType: "openai",
		model:        model,
		apiKeyEnv:    "OPENAI_API_KEY",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	app, err := bootstrap.NewApp(cfgPath, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = app.Close() })

	result, err := app.Run(ctx)
	require.NoError(t, err)
	require.Len(t, result.Steps, 1)
	assert.Contains(t, strings.TrimSpace(result.Steps[0].Output), "42")
}

func TestE2E_OpenAI_streaming(t *testing.T) {
	apiKey := strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY not set")
	}

	model := openAIE2EModel()

	cfgPath := writeProviderConfig(t, providerConfig{
		providerType: "openai",
		model:        model,
		apiKeyEnv:    "OPENAI_API_KEY",
		stream:       true,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	app, err := bootstrap.NewAppWithOptions(ctx, cfgPath, nil, bootstrap.AppOptions{Stream: true})
	require.NoError(t, err)
	t.Cleanup(func() { _ = app.Close() })

	result, err := app.Run(ctx)
	require.NoError(t, err)
	require.Len(t, result.Steps, 1)
	assert.NotEmpty(t, strings.TrimSpace(result.Steps[0].Output))
}

func TestE2E_Anthropic_run(t *testing.T) {
	apiKey := strings.TrimSpace(os.Getenv("ANTHROPIC_API_KEY"))
	if apiKey == "" {
		t.Skip("ANTHROPIC_API_KEY not set")
	}

	model := anthropicE2EModel()

	cfgPath := writeProviderConfig(t, providerConfig{
		providerType: "anthropic",
		model:        model,
		apiKeyEnv:    "ANTHROPIC_API_KEY",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	app, err := bootstrap.NewApp(cfgPath, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = app.Close() })

	result, err := app.Run(ctx)
	require.NoError(t, err)
	require.Len(t, result.Steps, 1)
	assert.Contains(t, strings.TrimSpace(result.Steps[0].Output), "42")
}

func TestE2E_Anthropic_streaming(t *testing.T) {
	apiKey := strings.TrimSpace(os.Getenv("ANTHROPIC_API_KEY"))
	if apiKey == "" {
		t.Skip("ANTHROPIC_API_KEY not set")
	}

	model := anthropicE2EModel()

	cfgPath := writeProviderConfig(t, providerConfig{
		providerType: "anthropic",
		model:        model,
		apiKeyEnv:    "ANTHROPIC_API_KEY",
		stream:       true,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	app, err := bootstrap.NewAppWithOptions(ctx, cfgPath, nil, bootstrap.AppOptions{Stream: true})
	require.NoError(t, err)
	t.Cleanup(func() { _ = app.Close() })

	result, err := app.Run(ctx)
	require.NoError(t, err)
	require.Len(t, result.Steps, 1)
	assert.NotEmpty(t, strings.TrimSpace(result.Steps[0].Output))
}

type providerConfig struct {
	providerType string
	model        string
	apiKeyEnv    string
	stream       bool
}

func writeProviderConfig(t *testing.T, cfg providerConfig) string {
	t.Helper()

	root := t.TempDir()
	streamLine := "false"
	if cfg.stream {
		streamLine = "true"
	}

	content := `version: "1"
provider:
  type: ` + cfg.providerType + `
  model: ` + cfg.model + `
  api_key: ${` + cfg.apiKeyEnv + `}
workspace:
  path: ./workspace
runtime:
  max_tool_rounds: 3
  stream: ` + streamLine + `
skills: []
agents: []
mcp_servers: []
prompts:
  - id: math
    system: |
      You are a calculator. Reply with only the numeric answer. No words or punctuation.
    content: |
      What is 19 + 23?
`

	cfgPath := filepath.Join(root, "airon.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte(content), 0o644))
	return cfgPath
}
