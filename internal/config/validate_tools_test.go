package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidatePromptTools_knownToolsOK(t *testing.T) {
	known := map[string]struct{}{
		"fs_read":    {},
		"researcher": {},
	}
	err := ValidatePromptTools([]PromptConfig{
		{ID: "step1", Tools: []string{"researcher", "fs_read"}},
	}, known)
	require.NoError(t, err)
}

func TestValidatePromptTools_unknownToolFails(t *testing.T) {
	known := map[string]struct{}{"fs_read": {}}
	err := ValidatePromptTools([]PromptConfig{
		{ID: "step1", Tools: []string{"ghost"}},
	}, known)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `unknown tool "ghost"`)
	assert.Contains(t, err.Error(), `prompt "step1"`)
}
