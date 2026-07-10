package template

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRender_replacesVariables(t *testing.T) {
	out, err := Render("Hello {{name}}", map[string]string{"name": "world"})
	require.NoError(t, err)
	assert.Equal(t, "Hello world", out)
}

func TestRender_missingVariable(t *testing.T) {
	_, err := Render("Hello {{missing}}", map[string]string{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing")
}

func TestRender_multipleVariables(t *testing.T) {
	out, err := Render("{{greeting}} {{name}}!", map[string]string{
		"greeting": "Hello",
		"name":     "world",
	})
	require.NoError(t, err)
	assert.Equal(t, "Hello world!", out)
}

func TestRender_noVariables(t *testing.T) {
	out, err := Render("Hello world", map[string]string{})
	require.NoError(t, err)
	assert.Equal(t, "Hello world", out)
}

func TestRender_withFilePlaceholder(t *testing.T) {
	out, err := RenderWithFiles("Prefix {{file:notes.md}} suffix", map[string]string{}, func(path string) (string, error) {
		assert.Equal(t, "notes.md", path)
		return "FILE_BODY", nil
	})
	require.NoError(t, err)
	assert.Equal(t, "Prefix FILE_BODY suffix", out)
}
