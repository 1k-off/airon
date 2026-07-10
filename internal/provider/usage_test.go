package provider

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUsageTracker_recordsPerStepAndTotal(t *testing.T) {
	tracker := NewUsageTracker()

	tracker.BeginStep("research")
	tracker.Record("gpt-4o-mini", TokenUsage{PromptTokens: 100, CompletionTokens: 50, TotalTokens: 150})
	stepUsage := tracker.EndStep()
	assert.Equal(t, 150, stepUsage.TotalTokens)
	require.Len(t, stepUsage.ByModel, 1)

	tracker.BeginStep("draft")
	tracker.Record("gpt-4o-mini", TokenUsage{PromptTokens: 20, CompletionTokens: 10, TotalTokens: 30})
	stepUsage = tracker.EndStep()
	assert.Equal(t, 30, stepUsage.TotalTokens)

	total := tracker.TotalStats()
	assert.Equal(t, 180, total.TotalTokens)
}

func TestNewTrackingProvider_recordsUsage(t *testing.T) {
	tracker := NewUsageTracker()
	mock := &MockProvider{
		Response: CompletionResponse{
			Content: "ok",
			Usage:   &TokenUsage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
		},
	}
	tracked := NewTrackingProvider(mock, "gpt-4o-mini", tracker)

	_, err := tracked.Complete(t.Context(), CompletionRequest{Messages: []Message{{Role: RoleUser, Content: "hi"}}})
	require.NoError(t, err)

	stats := tracker.TotalStats()
	assert.Equal(t, 15, stats.TotalTokens)
}
