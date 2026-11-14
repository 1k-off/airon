package config

import "fmt"

// ValidatePromptTools ensures every prompt references only known tool names.
func ValidatePromptTools(prompts []PromptConfig, known map[string]struct{}) error {
	for _, prompt := range prompts {
		for _, toolName := range prompt.Tools {
			if _, ok := known[toolName]; !ok {
				return fmt.Errorf("config: unknown tool %q in prompt %q", toolName, prompt.ID)
			}
		}
	}
	return nil
}
