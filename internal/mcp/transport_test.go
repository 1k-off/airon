package mcp

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildTransport_stdioRequiresCommand(t *testing.T) {
	_, err := buildTransport(context.Background(), ServerConfig{
		Transport: "stdio",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "command is required")
}

func TestBuildTransport_streamableHTTPRequiresURL(t *testing.T) {
	_, err := buildTransport(context.Background(), ServerConfig{
		Transport: "streamable_http",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "url is required")
}

func TestBuildTransport_sseRequiresURL(t *testing.T) {
	_, err := buildTransport(context.Background(), ServerConfig{
		Transport: "sse",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "url is required")
}

func TestBuildTransport_streamableHTTPOK(t *testing.T) {
	transport, err := buildTransport(context.Background(), ServerConfig{
		Transport: "streamable_http",
		URL:       "http://localhost:8080/mcp",
	})
	require.NoError(t, err)
	require.NotNil(t, transport)
}

func TestBuildTransport_unsupported(t *testing.T) {
	_, err := buildTransport(context.Background(), ServerConfig{
		Transport: "websocket",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported mcp transport")
}
