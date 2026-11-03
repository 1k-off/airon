package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"gopkg.in/yaml.v3"
)

var envVarPattern = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)

// Load reads and validates configuration from the given YAML file path.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config: read file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("config: parse yaml: %w", err)
	}

	absBaseDir, err := filepath.Abs(filepath.Dir(path))
	if err != nil {
		return nil, fmt.Errorf("config: resolve base dir: %w", err)
	}

	if err := expandConfig(&cfg); err != nil {
		return nil, err
	}

	resolvePaths(&cfg, absBaseDir)

	if err := validate(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func expandConfig(cfg *Config) error {
	var err error

	cfg.Version, err = expandEnvString(cfg.Version)
	if err != nil {
		return err
	}

	if err := expandProvider(&cfg.Provider); err != nil {
		return err
	}
	if err := expandWorkspace(&cfg.Workspace); err != nil {
		return err
	}
	if err := expandRuntime(&cfg.Runtime); err != nil {
		return err
	}
	if err := expandLog(&cfg.Log); err != nil {
		return err
	}
	if err := expandNotify(&cfg.Notify); err != nil {
		return err
	}

	for i := range cfg.Skills {
		cfg.Skills[i].Path, err = expandEnvString(cfg.Skills[i].Path)
		if err != nil {
			return err
		}
	}

	for i := range cfg.Agents {
		if err := expandAgent(&cfg.Agents[i]); err != nil {
			return err
		}
	}

	for i := range cfg.MCPServers {
		if err := expandMCPServer(&cfg.MCPServers[i]); err != nil {
			return err
		}
	}

	for i := range cfg.Prompts {
		if err := expandPrompt(&cfg.Prompts[i]); err != nil {
			return err
		}
	}

	return nil
}

func expandProvider(p *ProviderConfig) error {
	var err error

	p.Type, err = expandEnvString(p.Type)
	if err != nil {
		return err
	}
	p.Model, err = expandEnvString(p.Model)
	if err != nil {
		return err
	}
	p.APIKey, err = expandEnvString(p.APIKey)
	if err != nil {
		return err
	}
	p.BaseURL, err = expandEnvString(p.BaseURL)
	return err
}

func expandWorkspace(w *WorkspaceConfig) error {
	var err error
	w.Path, err = expandEnvString(w.Path)
	return err
}

func expandRuntime(r *RuntimeConfig) error {
	var err error
	r.Timeout, err = expandEnvString(r.Timeout)
	return err
}

func expandLog(l *LogConfig) error {
	var err error
	l.Level, err = expandEnvString(l.Level)
	return err
}

func expandNotify(n *NotifyConfig) error {
	var err error
	n.Healthchecks.URL, err = expandEnvString(n.Healthchecks.URL)
	return err
}

func expandAgent(a *AgentConfig) error {
	var err error

	a.ID, err = expandEnvString(a.ID)
	if err != nil {
		return err
	}
	a.Description, err = expandEnvString(a.Description)
	if err != nil {
		return err
	}
	a.SystemPrompt, err = expandEnvString(a.SystemPrompt)
	if err != nil {
		return err
	}
	a.Model, err = expandEnvString(a.Model)
	if err != nil {
		return err
	}

	for i := range a.Tools {
		a.Tools[i], err = expandEnvString(a.Tools[i])
		if err != nil {
			return err
		}
	}

	return nil
}

func expandMCPServer(s *MCPServerConfig) error {
	var err error

	s.Name, err = expandEnvString(s.Name)
	if err != nil {
		return err
	}
	s.Transport, err = expandEnvString(s.Transport)
	if err != nil {
		return err
	}
	s.Command, err = expandEnvString(s.Command)
	if err != nil {
		return err
	}

	for i := range s.Args {
		s.Args[i], err = expandEnvString(s.Args[i])
		if err != nil {
			return err
		}
	}

	if s.Env != nil {
		expanded := make(map[string]string, len(s.Env))
		for key, value := range s.Env {
			expandedKey, err := expandEnvString(key)
			if err != nil {
				return err
			}
			expandedValue, err := expandEnvString(value)
			if err != nil {
				return err
			}
			expanded[expandedKey] = expandedValue
		}
		s.Env = expanded
	}

	s.URL, err = expandEnvString(s.URL)
	if err != nil {
		return err
	}
	s.Timeout, err = expandEnvString(s.Timeout)
	if err != nil {
		return err
	}

	if s.Headers != nil {
		expanded := make(map[string]string, len(s.Headers))
		for key, value := range s.Headers {
			expandedKey, err := expandEnvString(key)
			if err != nil {
				return err
			}
			expandedValue, err := expandEnvString(value)
			if err != nil {
				return err
			}
			expanded[expandedKey] = expandedValue
		}
		s.Headers = expanded
	}

	return nil
}

func expandPrompt(p *PromptConfig) error {
	var err error

	p.ID, err = expandEnvString(p.ID)
	if err != nil {
		return err
	}
	p.System, err = expandEnvString(p.System)
	if err != nil {
		return err
	}
	p.Content, err = expandEnvString(p.Content)
	if err != nil {
		return err
	}

	for i := range p.Tools {
		p.Tools[i], err = expandEnvString(p.Tools[i])
		if err != nil {
			return err
		}
	}

	for i := range p.DependsOn {
		p.DependsOn[i], err = expandEnvString(p.DependsOn[i])
		if err != nil {
			return err
		}
	}

	return nil
}

func expandEnvString(value string) (string, error) {
	if value == "" {
		return value, nil
	}

	matches := envVarPattern.FindAllStringSubmatchIndex(value, -1)
	if len(matches) == 0 {
		return value, nil
	}

	result := value
	for i := len(matches) - 1; i >= 0; i-- {
		match := matches[i]
		name := value[match[2]:match[3]]
		envValue, ok := os.LookupEnv(name)
		if !ok {
			return "", fmt.Errorf("config: missing environment variable %s", name)
		}
		result = result[:match[0]] + envValue + result[match[1]:]
	}

	return result, nil
}

func resolvePaths(cfg *Config, baseDir string) {
	cfg.Workspace.Path = resolveWorkspacePath(baseDir, cfg.Workspace.Path)

	for i := range cfg.Skills {
		cfg.Skills[i].Path = resolveOptionalPath(baseDir, cfg.Skills[i].Path)
	}

	for i := range cfg.Agents {
		cfg.Agents[i].SystemPrompt = resolveOptionalPath(baseDir, cfg.Agents[i].SystemPrompt)
	}

	for i := range cfg.MCPServers {
		resolveMCPServerPaths(baseDir, &cfg.MCPServers[i])
	}

	for i := range cfg.Prompts {
		cfg.Prompts[i].System = resolveContentPath(baseDir, cfg.Prompts[i].System)
		cfg.Prompts[i].Content = resolveContentPath(baseDir, cfg.Prompts[i].Content)
	}
}

func resolveMCPServerPaths(baseDir string, server *MCPServerConfig) {
	for i, arg := range server.Args {
		server.Args[i] = resolveOptionalPath(baseDir, arg)
	}
}

func resolveContentPath(baseDir, value string) string {
	if value == "" {
		return value
	}

	path := resolveOptionalPath(baseDir, value)
	if pathExists(path) {
		data, err := os.ReadFile(path)
		if err == nil {
			return string(data)
		}
	}

	return value
}

func resolveWorkspacePath(baseDir, value string) string {
	if value == "" {
		value = "./workspace"
	}

	if filepath.IsAbs(value) {
		return filepath.Clean(value)
	}

	return filepath.Clean(filepath.Join(baseDir, value))
}

func resolveOptionalPath(baseDir, value string) string {
	if value == "" {
		return value
	}

	// Prefer paths relative to the config directory so jobs are portable
	// regardless of the process CWD. Absolute paths still win as-is.
	if filepath.IsAbs(value) {
		if pathExists(value) {
			return cleanAbsPath(value)
		}
		return value
	}

	candidate := filepath.Join(baseDir, value)
	if pathExists(candidate) {
		return cleanAbsPath(candidate)
	}

	// Fall back to CWD-relative for backwards compatibility.
	if pathExists(value) {
		return cleanAbsPath(value)
	}

	return value
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func cleanAbsPath(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return filepath.Clean(path)
	}
	return abs
}
