package skill

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/1node/airon/internal/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadAll_LoadsMultipleSkills(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"alpha", "beta"} {
		dir := filepath.Join(root, name)
		require.NoError(t, os.MkdirAll(dir, 0o755))
		content := "---\nname: " + name + "\ndescription: " + name + " skill\n---\nBody\n"
		require.NoError(t, os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644))
	}

	tools, err := LoadAll([]string{
		filepath.Join(root, "alpha"),
		filepath.Join(root, "beta"),
	})
	require.NoError(t, err)
	require.Len(t, tools, 2)
	assert.Equal(t, "alpha", tools[0].Name())
	assert.Equal(t, "beta", tools[1].Name())
}

func TestLoadAllConfigured_usesProvider(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "writer")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(`---
name: writer
description: Write content
---
Write well.
`), 0o644))

	p := &mockProvider{response: provider.CompletionResponse{Content: "done"}}
	tools, err := LoadAllConfigured([]string{dir}, p, 3)
	require.NoError(t, err)
	require.Len(t, tools, 1)

	result, err := tools[0].Execute(context.Background(), json.RawMessage(`{"task":"draft"}`))
	require.NoError(t, err)
	assert.Equal(t, "done", result)
}

func TestSkillTool_Execute_requiresTaskAndProvider(t *testing.T) {
	toolInstance := NewSkillTool("writer", "Write", "Prompt", nil, 3)

	_, err := toolInstance.Execute(context.Background(), json.RawMessage(`{"task":"go"}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "skill provider is required")

	p := &mockProvider{}
	toolInstance = NewSkillTool("writer", "Write", "Prompt", p, 3)
	_, err = toolInstance.Execute(context.Background(), json.RawMessage(`{}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "task is required")
}

func TestLoad_missingFrontmatterFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "broken.md")
	require.NoError(t, os.WriteFile(path, []byte("no frontmatter"), 0o644))

	_, err := Load(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing YAML frontmatter")
}
