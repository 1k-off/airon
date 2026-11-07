package provider

import (
	"fmt"
	"strings"

	"github.com/1k-off/airon/internal/config"
)

// NewFromConfig constructs a provider implementation from configuration.
func NewFromConfig(cfg config.ProviderConfig) (Provider, error) {
	switch strings.TrimSpace(cfg.Type) {
	case "openai":
		return NewOpenAI(cfg.APIKey, cfg.BaseURL, cfg.Model), nil
	case "anthropic":
		return NewAnthropic(cfg.APIKey, cfg.BaseURL, cfg.Model), nil
	default:
		return nil, fmt.Errorf("unsupported provider type %q", cfg.Type)
	}
}
