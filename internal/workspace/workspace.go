package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const maxFileSize = 10 * 1024 * 1024 // 10 MB

type Manager struct {
	Root string
}

func NewManager(root string) (*Manager, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve workspace root: %w", err)
	}
	return &Manager{Root: filepath.Clean(abs)}, nil
}

func (m *Manager) Resolve(rel string) (string, error) {
	if rel == "" {
		return m.Root, nil
	}
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("path outside workspace: %s", rel)
	}

	abs := filepath.Clean(filepath.Join(m.Root, rel))
	if !m.isWithinRoot(abs) {
		return "", fmt.Errorf("path outside workspace: %s", rel)
	}
	return abs, nil
}

func (m *Manager) Read(rel string) ([]byte, error) {
	abs, err := m.resolveExisting(rel)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return nil, fmt.Errorf("path is a directory: %s", rel)
	}
	if info.Size() > maxFileSize {
		return nil, fmt.Errorf("file exceeds maximum size of %d bytes", maxFileSize)
	}

	data, err := os.ReadFile(abs)
	if err != nil {
		return nil, err
	}
	if len(data) > maxFileSize {
		return nil, fmt.Errorf("file exceeds maximum size of %d bytes", maxFileSize)
	}
	return data, nil
}

func (m *Manager) Write(rel string, data []byte) error {
	if len(data) > maxFileSize {
		return fmt.Errorf("data exceeds maximum size of %d bytes", maxFileSize)
	}

	abs, err := m.Resolve(rel)
	if err != nil {
		return err
	}
	if err := m.ensureParentWithinRoot(abs); err != nil {
		return err
	}

	if err := m.rejectSymlinkEscape(abs); err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return err
	}
	return os.WriteFile(abs, data, 0o644)
}

func (m *Manager) List(rel string) ([]string, error) {
	abs, err := m.resolveExisting(rel)
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(abs)
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names, nil
}

func (m *Manager) MkdirAll(rel string) error {
	abs, err := m.Resolve(rel)
	if err != nil {
		return err
	}
	if err := m.ensureParentWithinRoot(abs); err != nil {
		return err
	}
	return os.MkdirAll(abs, 0o755)
}

func (m *Manager) resolveExisting(rel string) (string, error) {
	abs, err := m.Resolve(rel)
	if err != nil {
		return "", err
	}

	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return abs, nil
		}
		return "", err
	}
	if !m.isWithinRoot(resolved) {
		return "", fmt.Errorf("path outside workspace: %s", rel)
	}
	return resolved, nil
}

func (m *Manager) ensureParentWithinRoot(abs string) error {
	if abs == m.Root {
		return nil
	}

	parent := filepath.Dir(abs)
	if parent == abs {
		return nil
	}

	resolved, err := filepath.EvalSymlinks(parent)
	if err != nil {
		if os.IsNotExist(err) {
			return m.ensureParentWithinRoot(parent)
		}
		return err
	}
	if !m.isWithinRoot(resolved) {
		return fmt.Errorf("path outside workspace: %s", abs)
	}
	return nil
}

func (m *Manager) rejectSymlinkEscape(abs string) error {
	info, err := os.Lstat(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return nil
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return err
	}
	if !m.isWithinRoot(resolved) {
		return fmt.Errorf("path outside workspace: %s", abs)
	}
	return nil
}

func (m *Manager) isWithinRoot(abs string) bool {
	if abs == m.Root {
		return true
	}
	return strings.HasPrefix(abs, m.Root+string(filepath.Separator))
}
