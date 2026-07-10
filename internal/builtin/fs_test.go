package builtin

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/1node/airon/internal/provider"
	"github.com/1node/airon/internal/tool"
	"github.com/1node/airon/internal/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newFSRegistry(t *testing.T) *tool.Registry {
	t.Helper()

	ws, err := workspace.NewManager(t.TempDir())
	require.NoError(t, err)

	reg, err := tool.NewRegistry(NewFSTools(ws)...)
	require.NoError(t, err)
	return reg
}

func TestFSTools_writeThenRead(t *testing.T) {
	reg := newFSRegistry(t)
	ctx := context.Background()

	result, err := reg.Execute(ctx, toolCall("fs_write", `{"path":"notes/a.txt","content":"hello"}`))
	require.NoError(t, err)
	assert.Equal(t, "ok", result)

	result, err = reg.Execute(ctx, toolCall("fs_read", `{"path":"notes/a.txt"}`))
	require.NoError(t, err)
	assert.Equal(t, "hello", result)
}

func TestFSTools_listDefaultsToDotPath(t *testing.T) {
	reg := newFSRegistry(t)
	ctx := context.Background()

	_, err := reg.Execute(ctx, toolCall("fs_write", `{"path":"a.txt","content":"a"}`))
	require.NoError(t, err)
	_, err = reg.Execute(ctx, toolCall("fs_write", `{"path":"b.txt","content":"b"}`))
	require.NoError(t, err)

	result, err := reg.Execute(ctx, toolCall("fs_list", `{}`))
	require.NoError(t, err)

	var entries []string
	require.NoError(t, json.Unmarshal([]byte(result), &entries))
	assert.Contains(t, entries, "a.txt")
	assert.Contains(t, entries, "b.txt")
}

func TestFSTools_readRequiresPath(t *testing.T) {
	reg := newFSRegistry(t)

	_, err := reg.Execute(context.Background(), toolCall("fs_read", `{}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "path")
}

func toolCall(name, args string) provider.ToolCall {
	return provider.ToolCall{Name: name, Arguments: args}
}
