package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad(t *testing.T) {
	configsDir := filepath.Join("..", "..", "testdata", "configs")

	tests := []struct {
		name      string
		path      string
		setupEnv  func(t *testing.T)
		wantErr   string
		assertion func(t *testing.T, cfg *Config)
	}{
		{
			name: "valid config loads",
			path: filepath.Join(configsDir, "valid.yaml"),
			setupEnv: func(t *testing.T) {
				t.Setenv("OPENAI_API_KEY", "sk-test-key-12345")
			},
			assertion: func(t *testing.T, cfg *Config) {
				assert.Equal(t, "1", cfg.Version)
				assert.Equal(t, "openai", cfg.Provider.Type)
				assert.Equal(t, "gpt-4o", cfg.Provider.Model)
				assert.Equal(t, "sk-test-key-12345", cfg.Provider.APIKey)
				require.Len(t, cfg.Prompts, 2)
				assert.Equal(t, "research", cfg.Prompts[0].ID)
				assert.Equal(t, "summarize", cfg.Prompts[1].ID)
				assert.Equal(t, []string{"research"}, cfg.Prompts[1].DependsOn)
			},
		},
		{
			name: "missing api key fails",
			path: filepath.Join(configsDir, "valid.yaml"),
			setupEnv: func(t *testing.T) {
				os.Unsetenv("OPENAI_API_KEY")
			},
			wantErr: "config: missing environment variable OPENAI_API_KEY",
		},
		{
			name: "cyclic dependency fails",
			path: filepath.Join(configsDir, "invalid_cycle.yaml"),
			wantErr: "config: cyclic dependency",
		},
		{
			name: "unsupported version fails",
			path: filepath.Join(configsDir, "invalid_version.yaml"),
			wantErr: "config: unsupported version",
		},
		{
			name: "env expansion works",
			path: filepath.Join(configsDir, "valid.yaml"),
			setupEnv: func(t *testing.T) {
				t.Setenv("OPENAI_API_KEY", "expanded-secret-key")
			},
			assertion: func(t *testing.T, cfg *Config) {
				assert.Equal(t, "expanded-secret-key", cfg.Provider.APIKey)
			},
		},
		{
			name: "loads prompt content from file",
			path: filepath.Join(configsDir, "prompt_from_file.yaml"),
			setupEnv: func(t *testing.T) {
				t.Setenv("OPENAI_API_KEY", "sk-test")
			},
			assertion: func(t *testing.T, cfg *Config) {
				require.Len(t, cfg.Prompts, 1)
				assert.Equal(t, "File prompt body", strings.TrimSpace(cfg.Prompts[0].Content))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setupEnv != nil {
				tt.setupEnv(t)
			}

			cfg, err := Load(tt.path)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, cfg)
			if tt.assertion != nil {
				tt.assertion(t, cfg)
			}
		})
	}
}

func TestEffectiveDefaults(t *testing.T) {
	cfg := &Config{}
	assert.Equal(t, 10, cfg.EffectiveMaxToolRounds())
	assert.Equal(t, 3, cfg.EffectiveMaxCallDepth())
}

func TestLoad_expandsNotifyHealthchecksURL(t *testing.T) {
	root := t.TempDir()
	cfgPath := filepath.Join(root, "airon.yaml")
	cfg := `version: "1"
provider:
  type: openai
  model: gpt-4o
  api_key: test-key
workspace:
  path: ./workspace
notify:
  healthchecks:
    url: https://hc-ping.com/${HC_UUID}
prompts:
  - id: step1
    content: "go"
`
	require.NoError(t, os.WriteFile(cfgPath, []byte(cfg), 0o644))
	t.Setenv("HC_UUID", "deadbeef-uuid")

	loaded, err := Load(cfgPath)
	require.NoError(t, err)
	assert.Equal(t, "https://hc-ping.com/deadbeef-uuid", loaded.Notify.Healthchecks.URL)
}

func TestLoad_missingNotifyURLEnvFails(t *testing.T) {
	root := t.TempDir()
	cfgPath := filepath.Join(root, "airon.yaml")
	cfg := `version: "1"
provider:
  type: openai
  model: gpt-4o
  api_key: test-key
workspace:
  path: ./workspace
notify:
  healthchecks:
    url: https://hc-ping.com/${MISSING_HC_UUID}
prompts:
  - id: step1
    content: "go"
`
	require.NoError(t, os.WriteFile(cfgPath, []byte(cfg), 0o644))
	os.Unsetenv("MISSING_HC_UUID")

	_, err := Load(cfgPath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing environment variable MISSING_HC_UUID")
}

func TestLoad_notifyOptional(t *testing.T) {
	root := t.TempDir()
	cfgPath := filepath.Join(root, "airon.yaml")
	cfg := `version: "1"
provider:
  type: openai
  model: gpt-4o
  api_key: test-key
workspace:
  path: ./workspace
prompts:
  - id: step1
    content: "go"
`
	require.NoError(t, os.WriteFile(cfgPath, []byte(cfg), 0o644))

	loaded, err := Load(cfgPath)
	require.NoError(t, err)
	assert.Equal(t, "", loaded.Notify.Healthchecks.URL)
}

func TestLoad_expandsMCPServerHeadersAndURL(t *testing.T) {
	root := t.TempDir()
	cfgPath := filepath.Join(root, "mcp.yaml")
	cfg := `version: "1"
provider:
  type: openai
  model: gpt-4o
  api_key: test-key
workspace:
  path: ./workspace
mcp_servers:
  - name: remote
    transport: streamable_http
    url: https://example.com/mcp
    headers:
      Authorization: Bearer ${MCP_AUTH_TOKEN}
prompts:
  - id: step1
    content: "go"
`
	require.NoError(t, os.WriteFile(cfgPath, []byte(cfg), 0o644))
	t.Setenv("MCP_AUTH_TOKEN", "secret-token")

	loaded, err := Load(cfgPath)
	require.NoError(t, err)
	require.Len(t, loaded.MCPServers, 1)
	assert.Equal(t, "Bearer secret-token", loaded.MCPServers[0].Headers["Authorization"])
	assert.Equal(t, "https://example.com/mcp", loaded.MCPServers[0].URL)
}

func TestLoad_resolvesMCPServerArgsRelativeToConfigDir(t *testing.T) {
	root := t.TempDir()
	workspace := filepath.Join(root, "workspace")
	require.NoError(t, os.MkdirAll(workspace, 0o755))

	cfgPath := filepath.Join(root, "airon.yaml")
	cfg := `version: "1"
provider:
  type: openai
  model: gpt-4o
  api_key: test-key
workspace:
  path: ./workspace
mcp_servers:
  - name: fs
    transport: stdio
    command: npx
    args: ["-y", "@modelcontextprotocol/server-filesystem", "./workspace"]
prompts:
  - id: step1
    content: "go"
`
	require.NoError(t, os.WriteFile(cfgPath, []byte(cfg), 0o644))

	loaded, err := Load(cfgPath)
	require.NoError(t, err)
	require.Len(t, loaded.MCPServers, 1)
	require.Len(t, loaded.MCPServers[0].Args, 3)

	wantWorkspace, err := filepath.Abs(workspace)
	require.NoError(t, err)
	assert.Equal(t, wantWorkspace, loaded.MCPServers[0].Args[2])
}

func TestLoad_missingEnvVarFails(t *testing.T) {
	root := t.TempDir()
	cfgPath := filepath.Join(root, "agent.yaml")
	cfg := `version: "1"
provider:
  type: openai
  model: gpt-4o
  api_key: test-key
workspace:
  path: ./workspace
agents:
  - id: analyst
    description: Analyst
    system_prompt: You are an analyst.
    model: ${MISSING_AGENT_MODEL}
prompts:
  - id: step1
    content: "go"
`
	require.NoError(t, os.WriteFile(cfgPath, []byte(cfg), 0o644))
	os.Unsetenv("MISSING_AGENT_MODEL")

	_, err := Load(cfgPath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing environment variable MISSING_AGENT_MODEL")
}

func TestExpandEnvString_missingVarReturnsError(t *testing.T) {
	os.Unsetenv("AIRON_TEST_MISSING_VAR")

	_, err := expandEnvString("prefix-${AIRON_TEST_MISSING_VAR}-suffix")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing environment variable AIRON_TEST_MISSING_VAR")
}

func TestLoad_validatesSkillPaths(t *testing.T) {
	root := t.TempDir()
	cfgPath := filepath.Join(root, "airon.yaml")
	cfg := `version: "1"
provider:
  type: openai
  model: gpt-4o
  api_key: test-key
skills:
  - path: ./missing-skill
prompts:
  - id: step1
    content: "go"
`
	require.NoError(t, os.WriteFile(cfgPath, []byte(cfg), 0o644))

	_, err := Load(cfgPath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "config: path not found:")
	assert.Contains(t, err.Error(), "missing-skill")
}

func TestLoad_validatesSkillDirectoryRequiresSkillMarkdown(t *testing.T) {
	root := t.TempDir()
	skillDir := filepath.Join(root, "empty-skill")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))

	cfgPath := filepath.Join(root, "airon.yaml")
	cfg := `version: "1"
provider:
  type: openai
  model: gpt-4o
  api_key: test-key
skills:
  - path: ./empty-skill
prompts:
  - id: step1
    content: "go"
`
	require.NoError(t, os.WriteFile(cfgPath, []byte(cfg), 0o644))

	_, err := Load(cfgPath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "config: path not found:")
	assert.Contains(t, err.Error(), "SKILL.md")
}

func TestLoad_validatesAgentSystemPromptFilePath(t *testing.T) {
	root := t.TempDir()
	cfgPath := filepath.Join(root, "airon.yaml")
	cfg := `version: "1"
provider:
  type: openai
  model: gpt-4o
  api_key: test-key
agents:
  - id: analyst
    description: Analyst
    system_prompt: ./missing-prompt.md
prompts:
  - id: step1
    content: "go"
`
	require.NoError(t, os.WriteFile(cfgPath, []byte(cfg), 0o644))

	_, err := Load(cfgPath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "config: path not found:")
	assert.Contains(t, err.Error(), "missing-prompt.md")
}

func TestLoad_allowsInlineAgentSystemPrompt(t *testing.T) {
	root := t.TempDir()
	cfgPath := filepath.Join(root, "airon.yaml")
	cfg := `version: "1"
provider:
  type: openai
  model: gpt-4o
  api_key: test-key
agents:
  - id: analyst
    description: Analyst
    system_prompt: You are a helpful analyst.
prompts:
  - id: step1
    content: "go"
`
	require.NoError(t, os.WriteFile(cfgPath, []byte(cfg), 0o644))

	loaded, err := Load(cfgPath)
	require.NoError(t, err)
	require.Len(t, loaded.Agents, 1)
	assert.Equal(t, "You are a helpful analyst.", loaded.Agents[0].SystemPrompt)
}

func TestLoad_resolvesExistingAgentSystemPromptFile(t *testing.T) {
	root := t.TempDir()
	promptPath := filepath.Join(root, "analyst.md")
	require.NoError(t, os.WriteFile(promptPath, []byte("You are a file-based analyst."), 0o644))

	cfgPath := filepath.Join(root, "airon.yaml")
	cfg := `version: "1"
provider:
  type: openai
  model: gpt-4o
  api_key: test-key
agents:
  - id: analyst
    description: Analyst
    system_prompt: ./analyst.md
prompts:
  - id: step1
    content: "go"
`
	require.NoError(t, os.WriteFile(cfgPath, []byte(cfg), 0o644))

	loaded, err := Load(cfgPath)
	require.NoError(t, err)
	require.Len(t, loaded.Agents, 1)
	assert.Equal(t, filepath.Clean(promptPath), filepath.Clean(loaded.Agents[0].SystemPrompt))
}
