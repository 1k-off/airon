package provider_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/1k-off/airon/internal/config"
	"github.com/1k-off/airon/internal/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnthropicComplete_textOnly(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/messages", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{
			"id":"msg_1",
			"type":"message",
			"role":"assistant",
			"model":"claude-sonnet-4-20250514",
			"content":[{"type":"text","text":"Hello from Claude"}],
			"stop_reason":"end_turn",
			"stop_sequence":null,
			"usage":{"input_tokens":10,"output_tokens":5}
		}`)
	}))
	defer server.Close()

	p := provider.NewAnthropic("test-key", server.URL, "claude-sonnet-4-20250514")
	resp, err := p.Complete(context.Background(), provider.CompletionRequest{
		Messages: []provider.Message{{Role: provider.RoleUser, Content: "Hi"}},
	})
	require.NoError(t, err)
	assert.Equal(t, "Hello from Claude", resp.Content)
}

func TestAnthropicComplete_withToolUse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{
			"id":"msg_2",
			"type":"message",
			"role":"assistant",
			"model":"claude-sonnet-4-20250514",
			"content":[{"type":"tool_use","id":"toolu_1","name":"fs_read","input":{"path":"notes.txt"}}],
			"stop_reason":"tool_use",
			"stop_sequence":null,
			"usage":{"input_tokens":20,"output_tokens":8}
		}`)
	}))
	defer server.Close()

	p := provider.NewAnthropic("test-key", server.URL, "claude-sonnet-4-20250514")
	resp, err := p.Complete(context.Background(), provider.CompletionRequest{
		Messages: []provider.Message{{Role: provider.RoleUser, Content: "read file"}},
		Tools: []provider.ToolDefinition{{
			Name:        "fs_read",
			Description: "Read file",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{"type": "string"},
				},
				"required": []any{"path"},
			},
		}},
	})
	require.NoError(t, err)
	require.Len(t, resp.ToolCalls, 1)
	assert.Equal(t, "toolu_1", resp.ToolCalls[0].ID)
	assert.Equal(t, "fs_read", resp.ToolCalls[0].Name)
	assert.JSONEq(t, `{"path":"notes.txt"}`, resp.ToolCalls[0].Arguments)
}

func TestNewFromConfig_anthropic(t *testing.T) {
	p, err := provider.NewFromConfig(config.ProviderConfig{
		Type:   "anthropic",
		APIKey: "k",
		Model:  "claude-sonnet-4-20250514",
	})
	require.NoError(t, err)
	require.NotNil(t, p)
}

func TestAnthropicComplete_streamsDeltas(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, `event: message_start
data: {"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","model":"claude-sonnet-4-20250514","content":[],"stop_reason":null,"stop_sequence":null,"usage":{"input_tokens":1,"output_tokens":0}}}

event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}

event: content_block_stop
data: {"type":"content_block_stop","index":0}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"end_turn","stop_sequence":null},"usage":{"output_tokens":1}}

event: message_stop
data: {"type":"message_stop"}

`)
	}))
	defer server.Close()

	p := provider.NewAnthropic("test-key", server.URL, "claude-sonnet-4-20250514")
	var chunks []string
	resp, err := p.Complete(context.Background(), provider.CompletionRequest{
		Messages: []provider.Message{{Role: provider.RoleUser, Content: "Hi"}},
		OnDelta:  func(chunk string) { chunks = append(chunks, chunk) },
	})
	require.NoError(t, err)
	assert.Equal(t, "Hello", resp.Content)
	assert.Equal(t, []string{"Hello"}, chunks)
}
