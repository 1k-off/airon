package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheck_validConfigWithMissingEnv(t *testing.T) {
	root := t.TempDir()
	skillDir := filepath.Join(root, "skills", "writer")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# writer\n"), 0o644))

	cfgPath := filepath.Join(root, "airon.yaml")
	cfg := `version: "1"
provider:
  type: openai
  model: gpt-4o
  api_key: ${OPENAI_API_KEY}
workspace:
  path: ./workspace
skills:
  - path: ./skills/writer
mcp_servers:
  - name: backup
    transport: streamable_http
    url: ${BACKUP_CHECK_MCP_URL}
notify:
  healthchecks:
    url: ${BACKUP_CHECK_HEALTHCHECKS_URL}
prompts:
  - id: verify
    content: "go"
    tools: [backup-verifier]
`
	require.NoError(t, os.WriteFile(cfgPath, []byte(cfg), 0o644))

	os.Unsetenv("OPENAI_API_KEY")
	os.Unsetenv("BACKUP_CHECK_MCP_URL")
	os.Unsetenv("BACKUP_CHECK_HEALTHCHECKS_URL")

	report, err := Check(cfgPath)
	require.NoError(t, err)
	require.NotNil(t, report.Config)
	assert.Equal(t, []string{"BACKUP_CHECK_HEALTHCHECKS_URL", "BACKUP_CHECK_MCP_URL", "OPENAI_API_KEY"}, report.MissingEnv)
	assert.Equal(t, "${OPENAI_API_KEY}", report.Config.Provider.APIKey)
	assert.Equal(t, "${BACKUP_CHECK_MCP_URL}", report.Config.MCPServers[0].URL)
	assert.Equal(t, "openai", report.Config.Provider.Type)
	assert.Len(t, report.Config.Prompts, 1)
}

func TestCheck_structuralErrorStillReturned(t *testing.T) {
	root := t.TempDir()
	cfgPath := filepath.Join(root, "airon.yaml")
	cfg := `version: "99"
provider:
  type: openai
  model: gpt-4o
  api_key: test
prompts:
  - id: a
    content: "go"
`
	require.NoError(t, os.WriteFile(cfgPath, []byte(cfg), 0o644))

	_, err := Check(cfgPath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported version")
}

func TestCheck_emptyHealthchecksWarns(t *testing.T) {
	root := t.TempDir()
	cfgPath := filepath.Join(root, "airon.yaml")
	cfg := `version: "1"
provider:
  type: openai
  model: gpt-4o
  api_key: test-key
prompts:
  - id: a
    content: "go"
`
	require.NoError(t, os.WriteFile(cfgPath, []byte(cfg), 0o644))

	report, err := Check(cfgPath)
	require.NoError(t, err)
	assert.Empty(t, report.MissingEnv)
	require.Len(t, report.Warnings, 1)
	assert.Contains(t, report.Warnings[0], "healthchecks")
}

func TestExpandEnvStringAllowMissing_keepsLiteral(t *testing.T) {
	os.Unsetenv("AIRON_CHECK_MISSING_X")
	missing := map[string]struct{}{}
	got, err := expandEnvStringAllowMissing("pre-${AIRON_CHECK_MISSING_X}-post", missing)
	require.NoError(t, err)
	assert.Equal(t, "pre-${AIRON_CHECK_MISSING_X}-post", got)
	_, ok := missing["AIRON_CHECK_MISSING_X"]
	assert.True(t, ok)
}
