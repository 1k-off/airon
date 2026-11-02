package builtin

import (
	"context"
	"testing"

	"github.com/1k-off/airon/internal/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFSTools_invalidJSON(t *testing.T) {
	reg := newFSRegistry(t)
	ctx := context.Background()

	for _, name := range []string{"fs_read", "fs_write", "fs_list"} {
		_, err := reg.Execute(ctx, provider.ToolCall{Name: name, Arguments: `{bad`})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "parse")
	}
}

func TestFSTools_writeRequiresPath(t *testing.T) {
	reg := newFSRegistry(t)

	_, err := reg.Execute(context.Background(), provider.ToolCall{
		Name:      "fs_write",
		Arguments: `{"content":"x"}`,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "path")
}

func TestFSTools_readMissingFile(t *testing.T) {
	reg := newFSRegistry(t)

	_, err := reg.Execute(context.Background(), provider.ToolCall{
		Name:      "fs_read",
		Arguments: `{"path":"missing.txt"}`,
	})
	require.Error(t, err)
}

func TestFSTools_listMissingDirectory(t *testing.T) {
	reg := newFSRegistry(t)

	_, err := reg.Execute(context.Background(), provider.ToolCall{
		Name:      "fs_list",
		Arguments: `{"path":"missing"}`,
	})
	require.Error(t, err)
}

func TestFSTools_listExplicitDotPath(t *testing.T) {
	reg := newFSRegistry(t)
	ctx := context.Background()

	_, err := reg.Execute(ctx, provider.ToolCall{
		Name:      "fs_write",
		Arguments: `{"path":"visible.txt","content":"v"}`,
	})
	require.NoError(t, err)

	result, err := reg.Execute(ctx, provider.ToolCall{
		Name:      "fs_list",
		Arguments: `{"path":"."}`,
	})
	require.NoError(t, err)
	assert.Contains(t, result, "visible.txt")
}

func TestNewFSTools_returnsThreeTools(t *testing.T) {
	reg := newFSRegistry(t)
	names := make(map[string]struct{})
	for _, t := range reg.List() {
		names[t.Name()] = struct{}{}
	}
	assert.Contains(t, names, "fs_read")
	assert.Contains(t, names, "fs_write")
	assert.Contains(t, names, "fs_list")
}
