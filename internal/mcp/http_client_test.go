package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newMockMCPServer(t *testing.T) *httptest.Server {
	t.Helper()

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "mock-mcp",
		Version: "1.0.0",
	}, nil)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "echo",
		Description: "Echo the input message",
	}, func(_ context.Context, _ *mcp.CallToolRequest, params *struct {
		Message string `json:"message"`
	}) (*mcp.CallToolResult, any, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: params.Message},
			},
		}, nil, nil
	})

	handler := mcp.NewStreamableHTTPHandler(func(_ *http.Request) *mcp.Server {
		return server
	}, nil)

	return httptest.NewServer(handler)
}

func TestConnect_streamableHTTP_listsAndCallsTools(t *testing.T) {
	t.Parallel()

	mockServer := newMockMCPServer(t)
	t.Cleanup(mockServer.Close)

	client, err := Connect(context.Background(), ServerConfig{
		Name:      "mock",
		Transport: "streamable_http",
		URL:       mockServer.URL,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, client.Close())
	})

	tools, err := client.ListTools(context.Background())
	require.NoError(t, err)
	require.Len(t, tools, 1)
	assert.Equal(t, "mock__echo", tools[0].Name())

	result, err := tools[0].Execute(context.Background(), json.RawMessage(`{"message":"hello mcp"}`))
	require.NoError(t, err)
	assert.Equal(t, "hello mcp", result)
}

func TestConnect_requiresServerName(t *testing.T) {
	t.Parallel()

	_, err := Connect(context.Background(), ServerConfig{
		Transport: "streamable_http",
		URL:       "http://localhost:1/mcp",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name is required")
}

func TestBuildTransport_stdioOK(t *testing.T) {
	transport, err := buildTransport(context.Background(), ServerConfig{
		Transport: "stdio",
		Command:   os.Getenv("COMSPEC"),
		Args:      []string{"/c", "exit", "0"},
	})
	require.NoError(t, err)
	require.NotNil(t, transport)
}

func TestBuildTransport_sseOK(t *testing.T) {
	transport, err := buildTransport(context.Background(), ServerConfig{
		Transport: "sse",
		URL:       "http://localhost:8080/sse",
		Headers: map[string]string{
			"Authorization": "Bearer token",
		},
	})
	require.NoError(t, err)
	require.NotNil(t, transport)
}
