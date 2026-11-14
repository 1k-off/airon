package skill

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/1k-off/airon/internal/depth"
	"github.com/1k-off/airon/internal/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockProvider struct {
	lastRequest provider.CompletionRequest
	response    provider.CompletionResponse
	err         error
	calls       int
}

func (m *mockProvider) Complete(ctx context.Context, req provider.CompletionRequest) (provider.CompletionResponse, error) {
	m.calls++
	m.lastRequest = req
	if m.err != nil {
		return provider.CompletionResponse{}, m.err
	}
	return m.response, nil
}

func TestLoad_ParsesSkillMarkdownWithFrontmatter(t *testing.T) {
	toolInstance, err := Load(filepath.Join("..", "..", "testdata", "skills", "researcher"))
	require.NoError(t, err)

	assert.Equal(t, "researcher", toolInstance.Name())
	assert.Equal(t, "Research a topic and return structured findings", toolInstance.Description())
}

func TestLoad_LoadsMarkdownFilePath(t *testing.T) {
	tmpDir := t.TempDir()
	mdPath := filepath.Join(tmpDir, "summarizer.md")
	content := `---
description: Summarize content
---
# Summarizer
Return concise summary.
`
	require.NoError(t, os.WriteFile(mdPath, []byte(content), 0o644))

	toolInstance, err := Load(mdPath)
	require.NoError(t, err)

	assert.Equal(t, "summarizer", toolInstance.Name())
	assert.Equal(t, "Summarize content", toolInstance.Description())
}

func TestSkillTool_Execute_CallsProviderWithSkillPrompt(t *testing.T) {
	p := &mockProvider{
		response: provider.CompletionResponse{Content: "structured findings"},
	}
	toolInstance := NewSkillTool(
		"researcher",
		"Research a topic",
		"You are a researcher skill.",
		p,
		3,
	)

	result, err := toolInstance.Execute(context.Background(), json.RawMessage(`{"task":"Study TDD in Go"}`))
	require.NoError(t, err)
	assert.Equal(t, "structured findings", result)
	assert.Equal(t, 1, p.calls)
	assert.Empty(t, p.lastRequest.Tools)
	require.Len(t, p.lastRequest.Messages, 2)
	assert.Equal(t, provider.RoleSystem, p.lastRequest.Messages[0].Role)
	assert.Equal(t, "You are a researcher skill.", p.lastRequest.Messages[0].Content)
	assert.Equal(t, provider.RoleUser, p.lastRequest.Messages[1].Role)
	assert.Equal(t, "Study TDD in Go", p.lastRequest.Messages[1].Content)
}

func TestSkillTool_Execute_ReturnsErrorWhenDepthExceeded(t *testing.T) {
	toolInstance := NewSkillTool(
		"researcher",
		"Research a topic",
		"You are a researcher skill.",
		&mockProvider{},
		2,
	)

	ctx := depth.WithIncrement(depth.WithIncrement(context.Background()))
	_, err := toolInstance.Execute(ctx, json.RawMessage(`{"task":"anything"}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "max call depth exceeded")
}

func TestSkillTool_Execute_ProviderErrorIsWrapped(t *testing.T) {
	p := &mockProvider{
		err: errors.New("provider unavailable"),
	}
	toolInstance := NewSkillTool(
		"researcher",
		"Research a topic",
		"You are a researcher skill.",
		p,
		3,
	)

	_, err := toolInstance.Execute(context.Background(), json.RawMessage(`{"task":"anything"}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "provider complete")
	assert.Contains(t, err.Error(), "provider unavailable")
}

func TestLoad_RejectsInvalidSkillNameFromFrontmatter(t *testing.T) {
	tmpDir := t.TempDir()
	mdPath := filepath.Join(tmpDir, "bad-name.md")
	content := `---
name: Bad-Name
description: Invalid skill name
---
Body
`
	require.NoError(t, os.WriteFile(mdPath, []byte(content), 0o644))

	_, err := Load(mdPath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid skill name")
	assert.Contains(t, err.Error(), "Bad-Name")
}

func TestLoad_RejectsInvalidSkillNameFromFilename(t *testing.T) {
	tmpDir := t.TempDir()
	mdPath := filepath.Join(tmpDir, "BadSkill.md")
	content := `---
description: Invalid skill name from filename
---
Body
`
	require.NoError(t, os.WriteFile(mdPath, []byte(content), 0o644))

	_, err := Load(mdPath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid skill name")
	assert.Contains(t, err.Error(), "BadSkill")
}

func TestLoad_AcceptsValidSkillNamePatterns(t *testing.T) {
	tmpDir := t.TempDir()
	mdPath := filepath.Join(tmpDir, "my_skill-2.md")
	content := `---
description: Valid skill name
---
Body
`
	require.NoError(t, os.WriteFile(mdPath, []byte(content), 0o644))

	toolInstance, err := Load(mdPath)
	require.NoError(t, err)
	assert.Equal(t, "my_skill-2", toolInstance.Name())
}
