package config

import "fmt"

const supportedVersion = "1"

// validate checks configuration constraints defined in the spec.
func validate(cfg *Config) error {
	if cfg.Version != supportedVersion {
		return fmt.Errorf("config: unsupported version")
	}

	if cfg.Provider.Type != "openai" && cfg.Provider.Type != "anthropic" {
		return fmt.Errorf("config: unsupported provider")
	}

	if cfg.Provider.APIKey == "" {
		return fmt.Errorf("config: provider.api_key is required")
	}

	if err := validatePrompts(cfg.Prompts); err != nil {
		return err
	}

	for _, server := range cfg.MCPServers {
		if err := validateMCPServer(server); err != nil {
			return err
		}
	}

	return nil
}

func validateMCPServer(server MCPServerConfig) error {
	transport := server.Transport
	if transport == "" {
		transport = "stdio"
	}

	switch transport {
	case "stdio":
		if server.Command == "" {
			return fmt.Errorf("config: mcp server %q requires command for stdio transport", server.Name)
		}
	case "streamable_http", "sse":
		if server.URL == "" {
			return fmt.Errorf("config: mcp server %q requires url for %s transport", server.Name, transport)
		}
	default:
		return fmt.Errorf("config: unsupported mcp transport %q", transport)
	}

	return nil
}

func validatePrompts(prompts []PromptConfig) error {
	ids := make(map[string]struct{}, len(prompts))

	for _, prompt := range prompts {
		if _, exists := ids[prompt.ID]; exists {
			return fmt.Errorf("config: duplicate prompt id")
		}
		ids[prompt.ID] = struct{}{}
	}

	for _, prompt := range prompts {
		for _, dep := range prompt.DependsOn {
			if _, ok := ids[dep]; !ok {
				return fmt.Errorf("config: unknown dependency")
			}
		}
	}

	if err := detectCycles(prompts); err != nil {
		return err
	}

	return nil
}

func detectCycles(prompts []PromptConfig) error {
	indexByID := make(map[string]int, len(prompts))
	for i, prompt := range prompts {
		indexByID[prompt.ID] = i
	}

	n := len(prompts)
	inDegree := make([]int, n)
	adjacency := make([][]int, n)

	for i, prompt := range prompts {
		for _, dep := range prompt.DependsOn {
			depIndex, ok := indexByID[dep]
			if !ok {
				continue
			}
			adjacency[depIndex] = append(adjacency[depIndex], i)
			inDegree[i]++
		}
	}

	queue := make([]int, 0, n)
	for i := 0; i < n; i++ {
		if inDegree[i] == 0 {
			queue = append(queue, i)
		}
	}

	visited := 0
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		visited++

		for _, next := range adjacency[node] {
			inDegree[next]--
			if inDegree[next] == 0 {
				queue = append(queue, next)
			}
		}
	}

	if visited != n {
		return fmt.Errorf("config: cyclic dependency")
	}

	return nil
}
