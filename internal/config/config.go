package config

// Config is the top-level airon runtime configuration.
type Config struct {
	Version    string            `yaml:"version"`
	Provider   ProviderConfig    `yaml:"provider"`
	Workspace  WorkspaceConfig   `yaml:"workspace"`
	Runtime    RuntimeConfig     `yaml:"runtime"`
	Log        LogConfig         `yaml:"log"`
	Notify     NotifyConfig      `yaml:"notify"`
	Skills     []SkillConfig     `yaml:"skills"`
	Agents     []AgentConfig     `yaml:"agents"`
	MCPServers []MCPServerConfig `yaml:"mcp_servers"`
	Prompts    []PromptConfig    `yaml:"prompts"`
}

// NotifyConfig groups run-notification integrations.
type NotifyConfig struct {
	Healthchecks HealthchecksConfig `yaml:"healthchecks"`
}

// HealthchecksConfig configures a healthchecks.io compatible ping endpoint.
// When URL is empty, the notifier becomes a no-op.
type HealthchecksConfig struct {
	URL string `yaml:"url"`
}

// ProviderConfig holds LLM provider settings.
type ProviderConfig struct {
	Type        string   `yaml:"type"`
	Model       string   `yaml:"model"`
	APIKey      string   `yaml:"api_key"`
	BaseURL     string   `yaml:"base_url"`
	Temperature *float64 `yaml:"temperature"`
}

// WorkspaceConfig holds sandbox workspace settings.
type WorkspaceConfig struct {
	Path string `yaml:"path"`
}

// RuntimeConfig holds execution limits and output options.
type RuntimeConfig struct {
	MaxToolRounds int  `yaml:"max_tool_rounds"`
	MaxCallDepth  int  `yaml:"max_call_depth"`
	Stream        bool `yaml:"stream"`
	SaveOutputs   bool `yaml:"save_outputs"`
}

// LogConfig holds logging settings.
type LogConfig struct {
	Level string `yaml:"level"`
}

// SkillConfig references a skill definition.
type SkillConfig struct {
	Path string `yaml:"path"`
}

// AgentConfig defines a callable agent tool.
type AgentConfig struct {
	ID            string   `yaml:"id"`
	Description   string   `yaml:"description"`
	SystemPrompt  string   `yaml:"system_prompt"`
	Model         string   `yaml:"model"`
	Tools         []string `yaml:"tools"`
	MaxToolRounds int      `yaml:"max_tool_rounds"`
}

// MCPServerConfig defines an MCP server connection.
type MCPServerConfig struct {
	Name      string            `yaml:"name"`
	Transport string            `yaml:"transport"` // stdio, streamable_http, sse
	Command   string            `yaml:"command"`
	Args      []string          `yaml:"args"`
	Env       map[string]string `yaml:"env"`
	URL       string            `yaml:"url"`
	Headers   map[string]string `yaml:"headers"`
}

// PromptConfig defines a sequential execution step.
type PromptConfig struct {
	ID        string   `yaml:"id"`
	System    string   `yaml:"system"`
	Content   string   `yaml:"content"`
	Tools     []string `yaml:"tools"`
	DependsOn []string `yaml:"depends_on"`
}

// Prompt is a backward-compatible alias used by runtime packages.
type Prompt = PromptConfig

// Agent is a backward-compatible alias used by runtime packages.
type Agent = AgentConfig

const (
	defaultMaxToolRounds = 10
	defaultMaxCallDepth  = 3
)

// EffectiveMaxToolRounds returns the configured limit or the default.
func (c *Config) EffectiveMaxToolRounds() int {
	if c.Runtime.MaxToolRounds <= 0 {
		return defaultMaxToolRounds
	}
	return c.Runtime.MaxToolRounds
}

// EffectiveSaveOutputs returns whether step outputs should be persisted.
func (c *Config) EffectiveSaveOutputs() bool {
	return c.Runtime.SaveOutputs
}

// EffectiveMaxCallDepth returns the configured limit or the default.
func (c *Config) EffectiveMaxCallDepth() int {
	if c.Runtime.MaxCallDepth <= 0 {
		return defaultMaxCallDepth
	}
	return c.Runtime.MaxCallDepth
}
