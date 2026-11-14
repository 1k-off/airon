package mcp

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_Close_nilClientAndSession(t *testing.T) {
	t.Parallel()

	var client *Client
	require.NoError(t, client.Close())

	client = &Client{serverName: "mock"}
	require.NoError(t, client.Close())
}

func TestClient_listToolDefinitions_error(t *testing.T) {
	t.Parallel()

	client := &Client{
		serverName: "mock",
		session: &mockSession{
			listToolsErr: errors.New("list failed"),
		},
	}
	_, err := client.listToolDefinitions(context.Background())
	require.Error(t, err)
}

func TestClient_listToolDefinitions_nilResult(t *testing.T) {
	t.Parallel()

	client := &Client{
		serverName: "mock",
		session: &mockSession{
			listToolsResult: nil,
		},
	}
	tools, err := client.listToolDefinitions(context.Background())
	require.NoError(t, err)
	assert.Empty(t, tools)
}

func TestClient_ListTools_wrapError(t *testing.T) {
	t.Parallel()

	client := &Client{
		serverName: "mock",
		session: &mockSession{
			listToolsErr: errors.New("rpc down"),
		},
	}
	_, err := client.ListTools(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), `list mcp tools from "mock"`)
}

func TestConnect_buildTransportError(t *testing.T) {
	t.Parallel()

	_, err := Connect(context.Background(), ServerConfig{
		Name:      "bad",
		Transport: "websocket",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `mcp server "bad"`)
}

func TestConnect_connectionFailure(t *testing.T) {
	t.Parallel()

	_, err := Connect(context.Background(), ServerConfig{
		Name:      "offline",
		Transport: "streamable_http",
		URL:       "http://127.0.0.1:1/mcp",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `connect mcp server "offline"`)
}

func TestRoundTripperWithHeaders_setsHeaders(t *testing.T) {
	t.Parallel()

	var seenAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	client := httpClientWithHeaders(map[string]string{
		"Authorization": "Bearer test-token",
	}, defaultHTTPTimeout)
	req, err := http.NewRequest(http.MethodGet, server.URL, nil)
	require.NoError(t, err)

	resp, err := client.Do(req)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	assert.Equal(t, "Bearer test-token", seenAuth)
}

func TestBuildStdioTransport_withEnv(t *testing.T) {
	transport, err := buildStdioTransport(context.Background(), ServerConfig{
		Command: "echo",
		Args:    []string{"hello"},
		Env:     map[string]string{"AIRON_TEST": "1"},
	})
	require.NoError(t, err)
	require.NotNil(t, transport)
}

func TestWrapTools_skipsNilDefinitions(t *testing.T) {
	t.Parallel()

	session := &mockSession{}
	tools := wrapTools("mock", session, []*mcp.Tool{
		nil,
		{Name: "ok", Description: "works"},
	})
	require.Len(t, tools, 1)
	assert.Equal(t, "mock__ok", tools[0].Name())
}
