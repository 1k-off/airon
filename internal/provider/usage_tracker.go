package provider

import "sync"

// UsageTracker records token usage per step and for the full run.
type UsageTracker struct {
	mu           sync.Mutex
	totalByModel map[string]TokenUsage
	stepByModel  map[string]TokenUsage
}

func NewUsageTracker() *UsageTracker {
	return &UsageTracker{
		totalByModel: make(map[string]TokenUsage),
		stepByModel:  make(map[string]TokenUsage),
	}
}

func (t *UsageTracker) BeginStep(_ string) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.stepByModel = make(map[string]TokenUsage)
}

func (t *UsageTracker) Record(model string, usage TokenUsage) {
	if t == nil || usage.TotalTokens == 0 && usage.PromptTokens == 0 && usage.CompletionTokens == 0 {
		return
	}
	if model == "" {
		model = "unknown"
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	t.totalByModel[model] = t.totalByModel[model].Add(usage)
	t.stepByModel[model] = t.stepByModel[model].Add(usage)
}

func (t *UsageTracker) EndStep() UsageStats {
	if t == nil {
		return UsageStats{}
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return buildUsageStats(cloneUsageMap(t.stepByModel))
}

func (t *UsageTracker) TotalStats() UsageStats {
	if t == nil {
		return UsageStats{}
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return buildUsageStats(cloneUsageMap(t.totalByModel))
}

func cloneUsageMap(source map[string]TokenUsage) map[string]TokenUsage {
	if len(source) == 0 {
		return map[string]TokenUsage{}
	}
	cloned := make(map[string]TokenUsage, len(source))
	for model, usage := range source {
		cloned[model] = usage
	}
	return cloned
}
