package provider_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/1k-off/airon/internal/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnthropicComplete_toolResultAndSystemMessages(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{
			"id":"msg_3",
			"type":"message",
			"role":"assistant",
			"model":"claude-sonnet-4-20250514",
			"content":[{"type":"text","text":"All set"}],
			"stop_reason":"end_turn",
			"stop_sequence":null,
			"usage":{"input_tokens":12,"output_tokens":4}
		}`)
	}))
	defer server.Close()

	p := provider.NewAnthropic("test-key", server.URL, "claude-sonnet-4-20250514")
	resp, err := p.Complete(context.Background(), provider.CompletionRequest{
		Messages: []provider.Message{
			{Role: provider.RoleSystem, Content: "Be concise"},
			{Role: provider.RoleUser, Content: "Hi"},
			{Role: provider.RoleAssistant, Content: "Working", ToolCalls: []provider.ToolCall{
				{ID: "toolu_1", Name: "fs_read", Arguments: `{"path":"a.txt"}`},
			}},
			{Role: provider.RoleTool, Content: "file body", ToolCallID: "toolu_1"},
		},
		Tools: []provider.ToolDefinition{{
			Name:        "fs_read",
			Description: "Read",
			Parameters: map[string]any{
				"type":       "object",
				"properties": map[string]any{"path": map[string]any{"type": "string"}},
				"required":   []string{"path"},
			},
		}},
	})
	require.NoError(t, err)
	assert.Equal(t, "All set", resp.Content)
}

func TestAnthropicComplete_unsupportedRoleFails(t *testing.T) {
	p := provider.NewAnthropic("test-key", "", "claude-sonnet-4-20250514")
	_, err := p.Complete(context.Background(), provider.CompletionRequest{
		Messages: []provider.Message{{Role: provider.MessageRole("nope"), Content: "x"}},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported message role")
}
