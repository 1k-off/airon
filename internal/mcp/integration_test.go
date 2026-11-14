//go:build integration

package mcp

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConnect_liveStreamableHTTP(t *testing.T) {
	url := os.Getenv("AIRON_MCP_LIVE_URL")
	if url == "" {
		t.Skip("set AIRON_MCP_LIVE_URL to run live MCP integration test")
	}

	client, err := Connect(context.Background(), ServerConfig{
		Name:      "live",
		Transport: "streamable_http",
		URL:       url,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, client.Close())
	})

	_, err = client.ListTools(context.Background())
	require.NoError(t, err)
}
