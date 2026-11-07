package config

import "strings"

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
	// MaxTokens caps completion length. When unset, Anthropic defaults to 4096;
	// OpenAI uses the provider default.
	MaxTokens *int `yaml:"max_tokens"`
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
	// Timeout bounds the entire run (Go duration, e.g. "30m", "1h"). Empty = no bound.
	Timeout string `yaml:"timeout"`
	// LogInputs controls whether template input vars are persisted to the run
	// JSON. Sensitive-looking keys are always redacted when logging.
	LogInputs *bool `yaml:"log_inputs"`
	// Outcome controls post-run interpretation of agent text. Empty / "none"
	// leaves executor success as the only signal. "pass_fail" requires a
	// PASS/FAIL verdict line in step output: FAIL (or missing PASS) becomes a
	// run error so healthchecks /fail and the process exits non-zero.
	Outcome string `yaml:"outcome"`
}

// Outcome mode values for RuntimeConfig.Outcome.
const (
	OutcomeNone     = ""
	OutcomeNoneAlt  = "none"
	OutcomePassFail = "pass_fail"
)

// EffectiveOutcome returns the normalized outcome mode.
func (c *Config) EffectiveOutcome() string {
	switch strings.ToLower(strings.TrimSpace(c.Runtime.Outcome)) {
	case OutcomePassFail:
		return OutcomePassFail
	default:
		return OutcomeNone
	}
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
	// Timeout for HTTP MCP transports (Go duration). Empty defaults to 60s.
	Timeout string `yaml:"timeout"`
	// EnvPassthrough controls whether stdio children inherit the host
	// environment. Nil/true = inherit (plus Env overlays). False = only Env
	// (and PATH/SystemRoot so binaries still resolve).
	EnvPassthrough *bool `yaml:"env_passthrough"`
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
	defaultMCPHTTPTimeout = "60s"
)

// EffectiveLogInputs reports whether run logs should include the input map.
// Defaults to true; sensitive keys are still redacted by the runlog layer.
func (c *Config) EffectiveLogInputs() bool {
	if c.Runtime.LogInputs == nil {
		return true
	}
	return *c.Runtime.LogInputs
}

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
