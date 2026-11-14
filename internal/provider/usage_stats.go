package provider

// UsageStats aggregates token usage for a step or run.
type UsageStats struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
	ByModel          []ModelUsageStats
}

// ModelUsageStats is per-model usage within a step or run.
type ModelUsageStats struct {
	Model            string
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

func (u TokenUsage) Add(other TokenUsage) TokenUsage {
	return TokenUsage{
		PromptTokens:     u.PromptTokens + other.PromptTokens,
		CompletionTokens: u.CompletionTokens + other.CompletionTokens,
		TotalTokens:      u.TotalTokens + other.TotalTokens,
	}
}

func sumUsage(byModel map[string]TokenUsage) TokenUsage {
	total := TokenUsage{}
	for _, usage := range byModel {
		total = total.Add(usage)
	}
	return total
}

func buildUsageStats(byModel map[string]TokenUsage) UsageStats {
	models := make([]ModelUsageStats, 0, len(byModel))
	for model, usage := range byModel {
		models = append(models, ModelUsageStats{
			Model:            model,
			PromptTokens:     usage.PromptTokens,
			CompletionTokens: usage.CompletionTokens,
			TotalTokens:      usage.TotalTokens,
		})
	}

	total := sumUsage(byModel)
	return UsageStats{
		PromptTokens:     total.PromptTokens,
		CompletionTokens: total.CompletionTokens,
		TotalTokens:      total.TotalTokens,
		ByModel:          models,
	}
}
