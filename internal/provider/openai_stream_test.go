package provider_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/1k-off/airon/internal/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenAIComplete_streamingInvokesOnDelta(t *testing.T) {
	server := newOpenAITestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(`data: {"id":"c1","object":"chat.completion.chunk","created":1,"model":"gpt-4o","choices":[{"index":0,"delta":{"content":"Hel"},"finish_reason":null}]}

data: {"id":"c1","object":"chat.completion.chunk","created":1,"model":"gpt-4o","choices":[{"index":0,"delta":{"content":"lo"},"finish_reason":null}]}

data: {"id":"c1","object":"chat.completion.chunk","created":1,"model":"gpt-4o","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}

data: [DONE]

`))
	})
	defer server.Close()

	p := provider.NewOpenAI("test-key", server.URL+"/v1", "gpt-4o")
	var chunks []string
	resp, err := p.Complete(context.Background(), provider.CompletionRequest{
		Messages: []provider.Message{{Role: provider.RoleUser, Content: "hi"}},
		OnDelta:  func(s string) { chunks = append(chunks, s) },
	})
	require.NoError(t, err)
	assert.Equal(t, "Hello", resp.Content)
	assert.Equal(t, []string{"Hel", "lo"}, chunks)
}

func TestOpenAIComplete_unsupportedRole(t *testing.T) {
	p := provider.NewOpenAI("test-key", "", "gpt-4o")
	_, err := p.Complete(context.Background(), provider.CompletionRequest{
		Messages: []provider.Message{{Role: provider.MessageRole("nope"), Content: "x"}},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported message role")
}
