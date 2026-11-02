package provider

import (
	"testing"

	"github.com/1k-off/airon/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFromConfig_openaiAndAnthropic(t *testing.T) {
	openaiProvider, err := NewFromConfig(config.ProviderConfig{
		Type:   "openai",
		APIKey: "key",
		Model:  "gpt-4o",
	})
	require.NoError(t, err)
	require.NotNil(t, openaiProvider)

	anthropicProvider, err := NewFromConfig(config.ProviderConfig{
		Type:   "anthropic",
		APIKey: "key",
		Model:  "claude-3-5-sonnet-latest",
	})
	require.NoError(t, err)
	require.NotNil(t, anthropicProvider)
}

func TestNewFromConfig_unsupportedTypeFails(t *testing.T) {
	_, err := NewFromConfig(config.ProviderConfig{
		Type:   "gemini",
		APIKey: "key",
		Model:  "model",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `unsupported provider type "gemini"`)
}
