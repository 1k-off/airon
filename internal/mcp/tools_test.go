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

func TestDecodeArguments_emptyAndInvalid(t *testing.T) {
	t.Parallel()

	args, err := decodeArguments(nil)
	require.NoError(t, err)
	assert.Empty(t, args)

	args, err = decodeArguments(json.RawMessage(`   `))
	require.NoError(t, err)
	assert.Empty(t, args)

	_, err = decodeArguments(json.RawMessage(`not-json`))
	require.Error(t, err)
}

func TestRenderCallToolResult_textAndStructured(t *testing.T) {
	t.Parallel()

	text, err := renderCallToolResult(&mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: "line one"},
			&mcp.TextContent{Text: "line two"},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "line one\nline two", text)

	structured, err := renderCallToolResult(&mcp.CallToolResult{
		StructuredContent: map[string]any{"ok": true},
	})
	require.NoError(t, err)
	assert.JSONEq(t, `{"ok":true}`, structured)
}

func TestWrappedTool_Execute_returnsErrorForMCPToolErrorResult(t *testing.T) {
	t.Parallel()

	toolInstance := &wrappedTool{
		name:       "mock__fail",
		remoteName: "fail",
		callTool: func(_ context.Context, _ *mcp.CallToolParams) (*mcp.CallToolResult, error) {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{
					&mcp.TextContent{Text: "tool failed"},
				},
			}, nil
		},
	}

	out, err := toolInstance.Execute(context.Background(), json.RawMessage(`{}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "error result")
	assert.Contains(t, err.Error(), "tool failed")
	assert.Equal(t, "tool failed", out)
}

func TestWrappedTool_Execute_propagatesCallToolError(t *testing.T) {
	t.Parallel()

	toolInstance := &wrappedTool{
		name:       "mock__fail",
		remoteName: "fail",
		callTool: func(_ context.Context, _ *mcp.CallToolParams) (*mcp.CallToolResult, error) {
			return nil, errors.New("transport down")
		},
	}

	_, err := toolInstance.Execute(context.Background(), json.RawMessage(`{}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "transport down")
}

func TestNormalizeSchema_defaultsForNilAndInvalid(t *testing.T) {
	t.Parallel()

	assert.Equal(t, map[string]any{"type": "object"}, normalizeSchema(nil))

	schema := normalizeSchema(map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name": map[string]any{"type": "string"},
		},
	})
	assert.Equal(t, "object", schema["type"])
}
