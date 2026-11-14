package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateMCPServer_stdioRequiresCommand(t *testing.T) {
	err := validateMCPServer(MCPServerConfig{Name: "local", Transport: "stdio"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "requires command")
}

func TestValidateMCPServer_streamableHTTPRequiresURL(t *testing.T) {
	err := validateMCPServer(MCPServerConfig{Name: "remote", Transport: "streamable_http"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "requires url")
}

func TestValidateMCPServer_sseRequiresURL(t *testing.T) {
	err := validateMCPServer(MCPServerConfig{Name: "remote", Transport: "sse"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "requires url")
}

func TestValidateMCPServer_unsupportedTransport(t *testing.T) {
	err := validateMCPServer(MCPServerConfig{Name: "bad", Transport: "websocket"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported mcp transport")
}

func TestValidateMCPServer_validHTTPTransports(t *testing.T) {
	require.NoError(t, validateMCPServer(MCPServerConfig{
		Name: "http", Transport: "streamable_http", URL: "http://localhost:8080/mcp",
	}))
	require.NoError(t, validateMCPServer(MCPServerConfig{
		Name: "sse", Transport: "sse", URL: "http://localhost:8080/sse",
	}))
}
