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

func TestManager_List_onFileFails(t *testing.T) {
	m := newTestManager(t)
	require.NoError(t, m.Write("file.txt", []byte("x")))
	_, err := m.List("file.txt")
	require.Error(t, err)
}

func TestManager_Read_brokenSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink test skipped on Windows")
	}

	m := newTestManager(t)
	require.NoError(t, os.Symlink("missing-target", filepath.Join(m.Root, "broken")))

	_, err := m.Read("broken")
	require.Error(t, err)
}

func TestManager_Write_deepPathCreatesParents(t *testing.T) {
	m := newTestManager(t)
	require.NoError(t, m.Write("x/y/z.txt", []byte("nested")))
	data, err := m.Read("x/y/z.txt")
	require.NoError(t, err)
	assert.Equal(t, "nested", string(data))
}

func TestManager_MkdirAll_idempotent(t *testing.T) {
	m := newTestManager(t)
	require.NoError(t, m.MkdirAll("d1/d2"))
	require.NoError(t, m.MkdirAll("d1/d2"))
}

func TestManager_Write_overwritesExistingFile(t *testing.T) {
	m := newTestManager(t)
	require.NoError(t, m.Write("dup.txt", []byte("v1")))
	require.NoError(t, m.Write("dup.txt", []byte("v2")))
	data, err := m.Read("dup.txt")
	require.NoError(t, err)
	assert.Equal(t, "v2", string(data))
}

func TestManager_Write_rejectsSymlinkParentDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink test skipped on Windows")
	}

	root := t.TempDir()
	outside := t.TempDir()
	linkDir := filepath.Join(root, "linkdir")
	require.NoError(t, os.MkdirAll(linkDir, 0o755))
	require.NoError(t, os.Symlink(outside, filepath.Join(linkDir, "escape")))

	m, err := workspace.NewManager(root)
	require.NoError(t, err)

	err = m.Write("linkdir/escape/nested.txt", []byte("x"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "outside workspace")
}

func TestManager_MkdirAll_rejectsEscape(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink test skipped on Windows")
	}

	root := t.TempDir()
	outside := t.TempDir()
	linkDir := filepath.Join(root, "links")
	require.NoError(t, os.MkdirAll(linkDir, 0o755))
	require.NoError(t, os.Symlink(outside, filepath.Join(linkDir, "out")))

	m, err := workspace.NewManager(root)
	require.NoError(t, err)

	err = m.MkdirAll("links/out/deep")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "outside workspace")
}

func TestManager_Resolve_cleanDotSegments(t *testing.T) {
	m := newTestManager(t)
	abs, err := m.Resolve("a/./b/../c.txt")
	require.NoError(t, err)
	assert.Contains(t, abs, "c.txt")
}
