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

func TestNormalizeSchema_marshalsNonMapSchema(t *testing.T) {
	t.Parallel()

	schema := normalizeSchema(struct {
		Type string `json:"type"`
	}{Type: "object"})
	assert.Equal(t, "object", schema["type"])
}

func TestNormalizeSchema_invalidJSONFallsBackToObject(t *testing.T) {
	t.Parallel()

	schema := normalizeSchema(json.RawMessage(`{`))
	assert.Equal(t, "object", schema["type"])
}

func TestRenderCallToolResult_nilResult(t *testing.T) {
	t.Parallel()

	text, err := renderCallToolResult(nil)
	require.NoError(t, err)
	assert.Empty(t, text)
}

func TestRenderContent_nonTextContent(t *testing.T) {
	t.Parallel()

	text, err := renderContent(&mcp.ImageContent{
		Data:     []byte("img"),
		MIMEType: "image/png",
	})
	require.NoError(t, err)
	assert.Contains(t, text, "image/png")
}

func TestDecodeArguments_nullJSON(t *testing.T) {
	t.Parallel()

	args, err := decodeArguments(json.RawMessage(`null`))
	require.NoError(t, err)
	assert.Empty(t, args)
}

func TestWrappedTool_Execute_decodeArgumentsError(t *testing.T) {
	t.Parallel()

	toolInstance := &wrappedTool{
		name:       "mock__bad",
		remoteName: "bad",
		callTool: func(context.Context, *mcp.CallToolParams) (*mcp.CallToolResult, error) {
			return nil, errors.New("should not be called")
		},
	}

	_, err := toolInstance.Execute(context.Background(), json.RawMessage(`not-json`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decode arguments")
}

func TestRenderCallToolResult_structuredMarshalError(t *testing.T) {
	t.Parallel()

	_, err := renderCallToolResult(&mcp.CallToolResult{
		StructuredContent: map[string]any{
			"bad": make(chan int),
		},
	})
	require.Error(t, err)
}
