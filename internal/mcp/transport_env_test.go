package mcp

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildStdioEnv_passthroughDefaultIncludesHost(t *testing.T) {
	t.Setenv("AIRON_TEST_HOST_SECRET", "should-leak-when-passthrough")
	env := buildStdioEnv(ServerConfig{Env: map[string]string{"CHILD": "1"}})
	joined := strings.Join(env, "\n")
	assert.Contains(t, joined, "AIRON_TEST_HOST_SECRET=should-leak-when-passthrough")
	assert.Contains(t, joined, "CHILD=1")
}

func TestBuildStdioEnv_noPassthroughStripsHostSecrets(t *testing.T) {
	t.Setenv("AIRON_TEST_HOST_SECRET", "must-not-appear")
	falseVal := false
	env := buildStdioEnv(ServerConfig{
		Env:            map[string]string{"CHILD": "1"},
		EnvPassthrough: &falseVal,
	})
	joined := strings.Join(env, "\n")
	assert.NotContains(t, joined, "AIRON_TEST_HOST_SECRET")
	assert.Contains(t, joined, "CHILD=1")
	// PATH should still be present so binaries resolve.
	require.NotEmpty(t, os.Getenv("PATH"))
	assert.Contains(t, joined, "PATH=")
}

func TestParseTimeout_defaultsAndOverrides(t *testing.T) {
	assert.Equal(t, defaultHTTPTimeout, parseTimeout("", defaultHTTPTimeout))
	assert.Equal(t, defaultHTTPTimeout, parseTimeout("nope", defaultHTTPTimeout))
	assert.Equal(t, 90*time.Second, parseTimeout("90s", defaultHTTPTimeout))
}
