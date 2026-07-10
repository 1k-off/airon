package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockSession struct {
	listToolsResult *mcp.ListToolsResult
	listToolsErr    error
	callToolResult  *mcp.CallToolResult
	callToolErr     error
	closedErr       error

	receivedListToolsParams []*mcp.ListToolsParams
	receivedCallToolParams  []*mcp.CallToolParams
}

func (m *mockSession) ListTools(_ context.Context, params *mcp.ListToolsParams) (*mcp.ListToolsResult, error) {
	m.receivedListToolsParams = append(m.receivedListToolsParams, params)
	if m.listToolsErr != nil {
		return nil, m.listToolsErr
	}
	return m.listToolsResult, nil
}

func (m *mockSession) CallTool(_ context.Context, params *mcp.CallToolParams) (*mcp.CallToolResult, error) {
	m.receivedCallToolParams = append(m.receivedCallToolParams, params)
	if m.callToolErr != nil {
		return nil, m.callToolErr
	}
	return m.callToolResult, nil
}

func (m *mockSession) Close() error {
	return m.closedErr
}

func TestNamespacedToolName(t *testing.T) {
	assert.Equal(t, "filesystem__read_file", namespacedToolName("filesystem", "read_file"))
}

func TestClientListTools_WrapsNamespacedMCPTools(t *testing.T) {
	t.Parallel()

	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{"type": "string"},
		},
	}

	session := &mockSession{
		listToolsResult: &mcp.ListToolsResult{
			Tools: []*mcp.Tool{
				{
					Name:        "read_file",
					Description: "Read a file",
					InputSchema: schema,
				},
			},
		},
		callToolResult: &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: "file contents"},
			},
		},
	}

	client := &Client{
		serverName: "filesystem",
		session:    session,
	}

	tools, err := client.ListTools(context.Background())
	require.NoError(t, err)
	require.Len(t, tools, 1)

	wrapped := tools[0]
	assert.Equal(t, "filesystem__read_file", wrapped.Name())
	assert.Equal(t, "Read a file", wrapped.Description())
	assert.Equal(t, schema, wrapped.Parameters())

	result, err := wrapped.Execute(context.Background(), json.RawMessage(`{"path":"notes.txt"}`))
	require.NoError(t, err)
	assert.Equal(t, "file contents", result)

	require.Len(t, session.receivedCallToolParams, 1)
	assert.Equal(t, "read_file", session.receivedCallToolParams[0].Name)

	encodedArgs, err := json.Marshal(session.receivedCallToolParams[0].Arguments)
	require.NoError(t, err)
	assert.JSONEq(t, `{"path":"notes.txt"}`, string(encodedArgs))
}

func TestClientClose_ForwardsClose(t *testing.T) {
	t.Parallel()

	session := &mockSession{closedErr: errors.New("close failed")}
	client := &Client{
		serverName: "filesystem",
		session:    session,
	}

	err := client.Close()
	require.EqualError(t, err, "close failed")
}

