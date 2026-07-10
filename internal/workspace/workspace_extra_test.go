package workspace_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/1node/airon/internal/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManager_Read_missingFile(t *testing.T) {
	m := newTestManager(t)
	_, err := m.Read("missing.txt")
	require.Error(t, err)
}

func TestManager_Read_directoryRejected(t *testing.T) {
	m := newTestManager(t)
	require.NoError(t, m.MkdirAll("dir"))
	_, err := m.Read("dir")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "directory")
}

func TestManager_List_missingDirectory(t *testing.T) {
	m := newTestManager(t)
	_, err := m.List("nope")
	require.Error(t, err)
}

func TestManager_MkdirAll_nested(t *testing.T) {
	m := newTestManager(t)
	require.NoError(t, m.MkdirAll("a/b/c"))
	entries, err := m.List("a/b")
	require.NoError(t, err)
	assert.Contains(t, entries, "c")
}

func TestManager_Write_rejectsSymlinkTarget(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink write escape test is flaky on Windows without privileges")
	}

	root := t.TempDir()
	outside := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("secret"), 0o644))

	linkDir := filepath.Join(root, "linkdir")
	require.NoError(t, os.MkdirAll(linkDir, 0o755))
	require.NoError(t, os.Symlink(outside, filepath.Join(linkDir, "escape")))

	m, err := workspace.NewManager(root)
	require.NoError(t, err)

	err = m.Write("linkdir/escape/pwned.txt", []byte("x"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "outside workspace")
}

func TestManager_resolveExisting_symlinkWithinWorkspace(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink read test skipped on Windows")
	}

	root := t.TempDir()
	target := filepath.Join(root, "real.txt")
	require.NoError(t, os.WriteFile(target, []byte("data"), 0o644))
	require.NoError(t, os.Symlink(target, filepath.Join(root, "link.txt")))

	m, err := workspace.NewManager(root)
	require.NoError(t, err)

	data, err := m.Read("link.txt")
	require.NoError(t, err)
	assert.Equal(t, "data", string(data))
}

func TestNewManager_invalidRootStillAbs(t *testing.T) {
	m, err := workspace.NewManager(".")
	require.NoError(t, err)
	assert.True(t, filepath.IsAbs(m.Root))
}

func TestManager_List_emptyDirectory(t *testing.T) {
	m := newTestManager(t)
	entries, err := m.List(".")
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestManager_Write_emptyContent(t *testing.T) {
	m := newTestManager(t)
	require.NoError(t, m.Write("empty.txt", nil))
	data, err := m.Read("empty.txt")
	require.NoError(t, err)
	assert.Empty(t, data)
}

func TestManager_Resolve_rejectsSiblingDirectoryEscape(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "ws")
	require.NoError(t, os.MkdirAll(root, 0o755))

	outside := filepath.Join(base, "ws_outside")
	require.NoError(t, os.MkdirAll(outside, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("secret"), 0o644))

	m, err := workspace.NewManager(root)
	require.NoError(t, err)

	_, err = m.Resolve("../ws_outside/secret.txt")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "outside workspace")
}
