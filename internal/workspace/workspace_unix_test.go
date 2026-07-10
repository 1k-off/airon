//go:build unix

package workspace_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/1node/airon/internal/workspace"
	"github.com/stretchr/testify/require"
)

func TestManager_Read_permissionDenied(t *testing.T) {
	m := newTestManager(t)
	path := filepath.Join(m.Root, "secret.txt")
	require.NoError(t, os.WriteFile(path, []byte("secret"), 0o644))
	require.NoError(t, os.Chmod(path, 0o000))
	t.Cleanup(func() { _ = os.Chmod(path, 0o644) })

	_, err := m.Read("secret.txt")
	require.Error(t, err)
}

func TestManager_Write_mkdirAllFails(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.Chmod(root, 0o555))
	t.Cleanup(func() { _ = os.Chmod(root, 0o755) })

	m, err := workspace.NewManager(root)
	require.NoError(t, err)

	err = m.Write("nested/file.txt", []byte("x"))
	require.Error(t, err)
}

func TestManager_List_permissionDenied(t *testing.T) {
	m := newTestManager(t)
	dir := filepath.Join(m.Root, "locked")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.Chmod(dir, 0o000))
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	_, err := m.List("locked")
	require.Error(t, err)
}
