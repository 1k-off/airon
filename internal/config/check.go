package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

// CheckReport summarizes a structural config check that does not contact
// LLM providers or MCP servers.
type CheckReport struct {
	// Config is the loaded configuration (env placeholders left intact when
	// variables are missing). Nil when parsing/validation failed hard.
	Config *Config

	// MissingEnv lists environment variable names referenced by ${VAR}
	// that were unset at check time. Sorted uniquely.
	MissingEnv []string

	// Warnings are non-fatal findings (e.g. empty healthchecks URL).
	Warnings []string
}

// Check loads and validates configuration without dialing providers or MCP.
// Missing ${ENV} references are recorded in CheckReport.MissingEnv and left
// as the literal "${ENV}" in the returned Config so structural validation can
// still succeed offline. Callers that need every variable present should treat
// a non-empty MissingEnv as a soft (or, with --strict, hard) failure.
func Check(path string) (CheckReport, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return CheckReport{}, fmt.Errorf("config: read file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return CheckReport{}, fmt.Errorf("config: parse yaml: %w", err)
	}

	absBaseDir, err := filepath.Abs(filepath.Dir(path))
	if err != nil {
		return CheckReport{}, fmt.Errorf("config: resolve base dir: %w", err)
	}

	missingSet := map[string]struct{}{}
	if err := expandConfigAllowMissing(&cfg, missingSet); err != nil {
		return CheckReport{}, err
	}

	resolvePaths(&cfg, absBaseDir)

	if err := validate(&cfg); err != nil {
		return CheckReport{MissingEnv: sortedKeys(missingSet)}, err
	}

	report := CheckReport{
		Config:     &cfg,
		MissingEnv: sortedKeys(missingSet),
	}

	if cfg.Notify.Healthchecks.URL == "" {
		report.Warnings = append(report.Warnings, "notify.healthchecks.url is empty (notifier will be a no-op)")
	}

	return report, nil
}

func expandConfigAllowMissing(cfg *Config, missing map[string]struct{}) error {
	expand := func(s string) (string, error) {
		return expandEnvStringAllowMissing(s, missing)
	}

	var err error
	cfg.Version, err = expand(cfg.Version)
	if err != nil {
		return err
	}
	if err := expandProviderAllowMissing(&cfg.Provider, expand); err != nil {
		return err
	}
	cfg.Workspace.Path, err = expand(cfg.Workspace.Path)
	if err != nil {
		return err
	}
	cfg.Log.Level, err = expand(cfg.Log.Level)
	if err != nil {
		return err
	}
	cfg.Notify.Healthchecks.URL, err = expand(cfg.Notify.Healthchecks.URL)
	if err != nil {
		return err
	}

	for i := range cfg.Skills {
		cfg.Skills[i].Path, err = expand(cfg.Skills[i].Path)
		if err != nil {
			return err
		}
	}
	for i := range cfg.Agents {
		if err := expandAgentAllowMissing(&cfg.Agents[i], expand); err != nil {
			return err
		}
	}
	for i := range cfg.MCPServers {
		if err := expandMCPServerAllowMissing(&cfg.MCPServers[i], expand); err != nil {
			return err
		}
	}
	for i := range cfg.Prompts {
		if err := expandPromptAllowMissing(&cfg.Prompts[i], expand); err != nil {
			return err
		}
	}
	return nil
}

type expandFn func(string) (string, error)

func expandProviderAllowMissing(p *ProviderConfig, expand expandFn) error {
	var err error
	p.Type, err = expand(p.Type)
	if err != nil {
		return err
	}
	p.Model, err = expand(p.Model)
	if err != nil {
		return err
	}
	p.APIKey, err = expand(p.APIKey)
	if err != nil {
		return err
	}
	p.BaseURL, err = expand(p.BaseURL)
	return err
}

func expandAgentAllowMissing(a *AgentConfig, expand expandFn) error {
	var err error
	a.ID, err = expand(a.ID)
	if err != nil {
		return err
	}
	a.Description, err = expand(a.Description)
	if err != nil {
		return err
	}
	a.SystemPrompt, err = expand(a.SystemPrompt)
	if err != nil {
		return err
	}
	a.Model, err = expand(a.Model)
	if err != nil {
		return err
	}
	for i := range a.Tools {
		a.Tools[i], err = expand(a.Tools[i])
		if err != nil {
			return err
		}
	}
	return nil
}

func expandMCPServerAllowMissing(s *MCPServerConfig, expand expandFn) error {
	var err error
	s.Name, err = expand(s.Name)
	if err != nil {
		return err
	}
	s.Transport, err = expand(s.Transport)
	if err != nil {
		return err
	}
	s.Command, err = expand(s.Command)
	if err != nil {
		return err
	}
	for i := range s.Args {
		s.Args[i], err = expand(s.Args[i])
		if err != nil {
			return err
		}
	}
	if s.Env != nil {
		expanded := make(map[string]string, len(s.Env))
		for key, value := range s.Env {
			ek, err := expand(key)
			if err != nil {
				return err
			}
			ev, err := expand(value)
			if err != nil {
				return err
			}
			expanded[ek] = ev
		}
		s.Env = expanded
	}
	s.URL, err = expand(s.URL)
	if err != nil {
		return err
	}
	if s.Headers != nil {
		expanded := make(map[string]string, len(s.Headers))
		for key, value := range s.Headers {
			ek, err := expand(key)
			if err != nil {
				return err
			}
			ev, err := expand(value)
			if err != nil {
				return err
			}
			expanded[ek] = ev
		}
		s.Headers = expanded
	}
	return nil
}

func expandPromptAllowMissing(p *PromptConfig, expand expandFn) error {
	var err error
	p.ID, err = expand(p.ID)
	if err != nil {
		return err
	}
	p.System, err = expand(p.System)
	if err != nil {
		return err
	}
	p.Content, err = expand(p.Content)
	if err != nil {
		return err
	}
	for i := range p.Tools {
		p.Tools[i], err = expand(p.Tools[i])
		if err != nil {
			return err
		}
	}
	for i := range p.DependsOn {
		p.DependsOn[i], err = expand(p.DependsOn[i])
		if err != nil {
			return err
		}
	}
	return nil
}

// expandEnvStringAllowMissing is like expandEnvString but records unset
// variables and leaves their ${NAME} literal in place instead of erroring.
func expandEnvStringAllowMissing(value string, missing map[string]struct{}) (string, error) {
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
			missing[name] = struct{}{}
			continue // keep "${NAME}" literal
		}
		result = result[:match[0]] + envValue + result[match[1]:]
	}

	return result, nil
}

func sortedKeys(set map[string]struct{}) []string {
	if len(set) == 0 {
		return nil
	}
	keys := make([]string, 0, len(set))
	for k := range set {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
