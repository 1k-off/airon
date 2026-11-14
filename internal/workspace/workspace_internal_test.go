package workspace

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnsureParentWithinRoot_atRoot(t *testing.T) {
	m, err := NewManager(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, m.ensureParentWithinRoot(m.Root))
}

func TestEnsureParentWithinRoot_deepNewPath(t *testing.T) {
	m, err := NewManager(t.TempDir())
	require.NoError(t, err)

	target := filepath.Join(m.Root, "a", "b", "c", "file.txt")
	require.NoError(t, m.ensureParentWithinRoot(target))
}

func TestRejectSymlinkEscape_missingPath(t *testing.T) {
	m, err := NewManager(t.TempDir())
	require.NoError(t, err)

	target := filepath.Join(m.Root, "new.txt")
	require.NoError(t, m.rejectSymlinkEscape(target))
}

func TestRejectSymlinkEscape_regularFile(t *testing.T) {
	m, err := NewManager(t.TempDir())
	require.NoError(t, err)

	target := filepath.Join(m.Root, "existing.txt")
	require.NoError(t, os.WriteFile(target, []byte("ok"), 0o644))
	require.NoError(t, m.rejectSymlinkEscape(target))
}

func TestRejectSymlinkEscape_symlinkWithinWorkspace(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink test skipped on Windows")
	}

	root := t.TempDir()
	target := filepath.Join(root, "real.txt")
	require.NoError(t, os.WriteFile(target, []byte("data"), 0o644))
	require.NoError(t, os.Symlink(target, filepath.Join(root, "link.txt")))

	m, err := NewManager(root)
	require.NoError(t, err)
	require.NoError(t, m.rejectSymlinkEscape(filepath.Join(root, "link.txt")))
}

func TestRejectSymlinkEscape_symlinkOutsideWorkspace(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink test skipped on Windows")
	}

	root := t.TempDir()
	outside := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("x"), 0o644))
	require.NoError(t, os.Symlink(filepath.Join(outside, "secret.txt"), filepath.Join(root, "escape")))

	m, err := NewManager(root)
	require.NoError(t, err)

	err = m.rejectSymlinkEscape(filepath.Join(root, "escape"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "outside workspace")
}

func TestResolveExisting_missingFile(t *testing.T) {
	m, err := NewManager(t.TempDir())
	require.NoError(t, err)

	abs, err := m.resolveExisting("missing.txt")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(m.Root, "missing.txt"), abs)
}

func TestManager_MkdirAll_root(t *testing.T) {
	m, err := NewManager(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, m.MkdirAll("."))
}
