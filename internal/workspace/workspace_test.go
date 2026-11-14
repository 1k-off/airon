package workspace_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/1k-off/airon/internal/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestManager(t *testing.T) *workspace.Manager {
	t.Helper()
	m, err := workspace.NewManager(t.TempDir())
	require.NoError(t, err)
	return m
}

func TestManager_Resolve_rejectsPathEscape(t *testing.T) {
	m := newTestManager(t)

	_, err := m.Resolve("../etc/passwd")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "outside workspace")
}

func TestManager(t *testing.T) {
	tests := []struct {
		name string
		run  func(t *testing.T, m *workspace.Manager, root string)
	}{
		{
			name: "valid read write",
			run: func(t *testing.T, m *workspace.Manager, root string) {
				err := m.Write("hello.txt", []byte("hello world"))
				require.NoError(t, err)

				data, err := m.Read("hello.txt")
				require.NoError(t, err)
				assert.Equal(t, "hello world", string(data))

				abs, err := m.Resolve("hello.txt")
				require.NoError(t, err)
				assert.Equal(t, filepath.Join(root, "hello.txt"), abs)
			},
		},
		{
			name: "nested dirs",
			run: func(t *testing.T, m *workspace.Manager, root string) {
				err := m.MkdirAll("a/b/c")
				require.NoError(t, err)

				err = m.Write("a/b/c/file.txt", []byte("nested"))
				require.NoError(t, err)

				entries, err := m.List("a/b")
				require.NoError(t, err)
				assert.Contains(t, entries, "c")

				data, err := m.Read("a/b/c/file.txt")
				require.NoError(t, err)
				assert.Equal(t, "nested", string(data))
			},
		},
		{
			name: "dot dot escape",
			run: func(t *testing.T, m *workspace.Manager, _ string) {
				_, err := m.Resolve("..")
				require.Error(t, err)
				assert.Contains(t, err.Error(), "outside workspace")

				_, err = m.Read("../../etc/passwd")
				require.Error(t, err)
				assert.Contains(t, err.Error(), "outside workspace")
			},
		},
		{
			name: "absolute escape",
			run: func(t *testing.T, m *workspace.Manager, _ string) {
				var absPath string
				if runtime.GOOS == "windows" {
					absPath = `C:\Windows\System32\drivers\etc\hosts`
				} else {
					absPath = "/etc/passwd"
				}
				_, err := m.Resolve(absPath)
				require.Error(t, err)
				assert.Contains(t, err.Error(), "outside workspace")
			},
		},
		{
			name: "empty path to root",
			run: func(t *testing.T, m *workspace.Manager, root string) {
				abs, err := m.Resolve("")
				require.NoError(t, err)
				assert.Equal(t, root, abs)
			},
		},
		{
			name: "10MB read limit",
			run: func(t *testing.T, m *workspace.Manager, root string) {
				large := make([]byte, 10*1024*1024+1)
				require.NoError(t, os.WriteFile(filepath.Join(root, "large.bin"), large, 0o644))

				_, err := m.Read("large.bin")
				require.Error(t, err)
				assert.Contains(t, err.Error(), "exceeds maximum")

				withinLimit := make([]byte, 10*1024*1024)
				err = m.Write("ok.bin", withinLimit)
				require.NoError(t, err)

				_, err = m.Read("ok.bin")
				require.NoError(t, err)
			},
		},
		{
			name: "10MB write limit",
			run: func(t *testing.T, m *workspace.Manager, _ string) {
				large := make([]byte, 10*1024*1024+1)
				err := m.Write("too-big.bin", large)
				require.Error(t, err)
				assert.Contains(t, err.Error(), "exceeds maximum")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			m, err := workspace.NewManager(root)
			require.NoError(t, err)
			require.True(t, filepath.IsAbs(m.Root))
			tt.run(t, m, m.Root)
		})
	}
}

func TestNewManager_resolvesAbsoluteRoot(t *testing.T) {
	root := t.TempDir()
	m, err := workspace.NewManager(root)
	require.NoError(t, err)
	assert.True(t, filepath.IsAbs(m.Root))
}

func TestManager_List_root(t *testing.T) {
	m := newTestManager(t)

	require.NoError(t, m.Write("one.txt", []byte("1")))
	require.NoError(t, m.Write("two.txt", []byte("2")))

	entries, err := m.List(".")
	require.NoError(t, err)
	assert.Contains(t, entries, "one.txt")
	assert.Contains(t, entries, "two.txt")
}

func TestManager_symlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("secret"), 0o644))

	linkPath := filepath.Join(root, "escape")
	if err := os.Symlink(filepath.Join(outside, "secret.txt"), linkPath); err != nil {
		if runtime.GOOS == "windows" && strings.Contains(err.Error(), "privilege") {
			t.Skip("symlinks require elevated privileges on Windows")
		}
		require.NoError(t, err)
	}

	m, err := workspace.NewManager(root)
	require.NoError(t, err)

	_, err = m.Read("escape")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "outside workspace")
}

func TestManager_Write_createsMissingWorkspaceRoot(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "workspace")
	m, err := workspace.NewManager(root)
	require.NoError(t, err)

	require.NoError(t, m.Write("logs/run.json", []byte(`{"ok":true}`)))

	data, err := os.ReadFile(filepath.Join(root, "logs", "run.json"))
	require.NoError(t, err)
	assert.Equal(t, `{"ok":true}`, string(data))
}
