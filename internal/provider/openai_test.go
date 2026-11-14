package provider_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/1k-off/airon/internal/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type chatCompletionRequest struct {
	Model    string `json:"model"`
	Messages []struct {
		Role       string `json:"role"`
		Content    any    `json:"content"`
		ToolCallID string `json:"tool_call_id,omitempty"`
		ToolCalls  []struct {
			ID       string `json:"id"`
			Type     string `json:"type"`
			Function struct {
				Name      string `json:"name"`
				Arguments string `json:"arguments"`
			} `json:"function"`
		} `json:"tool_calls,omitempty"`
	} `json:"messages"`
}

func newOpenAITestServer(t *testing.T, handler func(w http.ResponseWriter, r *http.Request)) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/v1/chat/completions", r.URL.Path)
		require.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		handler(w, r)
	}))
}

func TestOpenAIComplete_textOnly(t *testing.T) {
	server := newOpenAITestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{
			"id": "chatcmpl-test",
			"object": "chat.completion",
			"created": 1234567890,
			"model": "gpt-4o",
			"choices": [{
				"index": 0,
				"message": {
					"role": "assistant",
					"content": "Hello there!"
				},
				"finish_reason": "stop"
			}],
			"usage": {
				"prompt_tokens": 10,
				"completion_tokens": 5,
				"total_tokens": 15
			}
		}`)
	})
	defer server.Close()

	p := provider.NewOpenAI("test-key", server.URL+"/v1", "gpt-4o")

	resp, err := p.Complete(context.Background(), provider.CompletionRequest{
		Messages: []provider.Message{
			{Role: provider.RoleUser, Content: "Hi"},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "Hello there!", resp.Content)
	assert.Empty(t, resp.ToolCalls)
	assert.Equal(t, "stop", resp.FinishReason)
	require.NotNil(t, resp.Usage)
	assert.Equal(t, 10, resp.Usage.PromptTokens)
	assert.Equal(t, 5, resp.Usage.CompletionTokens)
	assert.Equal(t, 15, resp.Usage.TotalTokens)
}

func TestOpenAIComplete_withToolCalls(t *testing.T) {
	server := newOpenAITestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{
			"id": "chatcmpl-test",
			"object": "chat.completion",
			"created": 1234567890,
			"model": "gpt-4o",
			"choices": [{
				"index": 0,
				"message": {
					"role": "assistant",
					"content": "",
					"tool_calls": [{
						"id": "call_abc123",
						"type": "function",
						"function": {
							"name": "fs_read",
							"arguments": "{\"path\":\"hello.txt\"}"
						}
					}]
				},
				"finish_reason": "tool_calls"
			}],
			"usage": {
				"prompt_tokens": 20,
				"completion_tokens": 8,
				"total_tokens": 28
			}
		}`)
	})
	defer server.Close()

	p := provider.NewOpenAI("test-key", server.URL+"/v1", "gpt-4o")

	resp, err := p.Complete(context.Background(), provider.CompletionRequest{
		Messages: []provider.Message{
			{Role: provider.RoleUser, Content: "Read hello.txt"},
		},
		Tools: []provider.ToolDefinition{
			{
				Name:        "fs_read",
				Description: "Read a file",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"path": map[string]any{"type": "string"},
					},
				},
			},
		},
	})
	require.NoError(t, err)
	assert.Empty(t, resp.Content)
	assert.Equal(t, "tool_calls", resp.FinishReason)
	require.Len(t, resp.ToolCalls, 1)
	assert.Equal(t, "call_abc123", resp.ToolCalls[0].ID)
	assert.Equal(t, "fs_read", resp.ToolCalls[0].Name)
	assert.JSONEq(t, `{"path":"hello.txt"}`, resp.ToolCalls[0].Arguments)
}

func TestOpenAIComplete_mapsMessagesCorrectly(t *testing.T) {
	var captured chatCompletionRequest

	server := newOpenAITestServer(t, func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(body, &captured))

		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{
			"id": "chatcmpl-test",
			"object": "chat.completion",
			"created": 1234567890,
			"model": "gpt-4o",
			"choices": [{
				"index": 0,
				"message": {"role": "assistant", "content": "ok"},
				"finish_reason": "stop"
			}]
		}`)
	})
	defer server.Close()

	p := provider.NewOpenAI("test-key", server.URL+"/v1", "gpt-4o")

	_, err := p.Complete(context.Background(), provider.CompletionRequest{
		Model: "gpt-4o-mini",
		Messages: []provider.Message{
			{Role: provider.RoleSystem, Content: "You are helpful."},
			{Role: provider.RoleUser, Content: "What is 2+2?"},
			{
				Role:    provider.RoleAssistant,
				Content: "",
				ToolCalls: []provider.ToolCall{
					{ID: "call_1", Name: "calc", Arguments: `{"expr":"2+2"}`},
				},
			},
			{Role: provider.RoleTool, Content: "4", ToolCallID: "call_1"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, "gpt-4o-mini", captured.Model)
	require.Len(t, captured.Messages, 4)

	assert.Equal(t, "system", captured.Messages[0].Role)
	assert.Equal(t, "You are helpful.", captured.Messages[0].Content)

	assert.Equal(t, "user", captured.Messages[1].Role)
	assert.Equal(t, "What is 2+2?", captured.Messages[1].Content)

	assert.Equal(t, "assistant", captured.Messages[2].Role)
	require.Len(t, captured.Messages[2].ToolCalls, 1)
	assert.Equal(t, "call_1", captured.Messages[2].ToolCalls[0].ID)
	assert.Equal(t, "function", captured.Messages[2].ToolCalls[0].Type)
	assert.Equal(t, "calc", captured.Messages[2].ToolCalls[0].Function.Name)
	assert.JSONEq(t, `{"expr":"2+2"}`, captured.Messages[2].ToolCalls[0].Function.Arguments)

	assert.Equal(t, "tool", captured.Messages[3].Role)
	assert.Equal(t, "4", captured.Messages[3].Content)
	assert.Equal(t, "call_1", captured.Messages[3].ToolCallID)
}

func TestOpenAIComplete_emptyBaseURLDoesNotOverrideDefault(t *testing.T) {
	server := newOpenAITestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{
			"id": "chatcmpl-test",
			"object": "chat.completion",
			"created": 1234567890,
			"model": "gpt-4o",
			"choices": [{
				"index": 0,
				"message": {"role": "assistant", "content": "ok"},
				"finish_reason": "stop"
			}]
		}`)
	})
	defer server.Close()

	// Empty baseURL must not be passed to SDK (would break default URL).
	// Custom URL still works when explicitly set.
	p := provider.NewOpenAI("test-key", server.URL+"/v1", "gpt-4o")
	_, err := p.Complete(context.Background(), provider.CompletionRequest{
		Messages: []provider.Message{{Role: provider.RoleUser, Content: "hi"}},
	})
	require.NoError(t, err)

	pEmpty := provider.NewOpenAI("test-key", "", "gpt-4o")
	_, err = pEmpty.Complete(context.Background(), provider.CompletionRequest{
		Messages: []provider.Message{{Role: provider.RoleUser, Content: "hi"}},
	})
	require.Error(t, err)
	assert.NotContains(t, err.Error(), `unsupported protocol scheme ""`)
}
