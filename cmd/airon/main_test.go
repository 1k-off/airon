package main

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()

	original := os.Stderr
	reader, writer, err := os.Pipe()
	require.NoError(t, err)
	os.Stderr = writer

	t.Cleanup(func() {
		os.Stderr = original
	})

	done := make(chan struct{})
	var output bytes.Buffer
	go func() {
		_, _ = io.Copy(&output, reader)
		close(done)
	}()

	fn()

	require.NoError(t, writer.Close())
	<-done
	return output.String()
}

func TestRunCLI_usageErrorsReturnExitCode2(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "missing subcommand",
			args: nil,
			want: "Usage:",
		},
		{
			name: "unknown subcommand",
			args: []string{"serve"},
			want: "Usage:",
		},
		{
			name: "missing config flag",
			args: []string{"run"},
			want: "--config is required",
		},
		{
			name: "unknown run flag",
			args: []string{"run", "--unknown"},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stderr := captureStderr(t, func() {
				code := runCLI(tt.args)
				assert.Equal(t, exitUsage, code)
			})

			if tt.want != "" {
				assert.Contains(t, stderr, tt.want)
			}
		})
	}
}

func TestRunCLI_noHealthchecksFlagIsAccepted(t *testing.T) {
	// --no-healthchecks by itself should be a recognized flag; the missing
	// --config triggers the usage error, but the flag must have been parsed
	// (i.e. no "flag provided but not defined" error on stderr).
	stderr := captureStderr(t, func() {
		code := runCLI([]string{"run", "--no-healthchecks"})
		assert.Equal(t, exitUsage, code)
	})
	assert.NotContains(t, stderr, "flag provided but not defined")
	assert.Contains(t, stderr, "--config is required")
}

func TestRunCLI_checkRequiresConfig(t *testing.T) {
	stderr := captureStderr(t, func() {
		code := runCLI([]string{"check"})
		assert.Equal(t, exitUsage, code)
	})
	assert.Contains(t, stderr, "--config is required")
}

func TestRunCLI_checkValidConfigOffline(t *testing.T) {
	root := t.TempDir()
	cfgPath := root + string(os.PathSeparator) + "airon.yaml"
	content := "version: \"1\"\n" +
		"provider:\n  type: openai\n  model: gpt-4o\n  api_key: ${OPENAI_API_KEY}\n" +
		"prompts:\n  - id: a\n    content: \"go\"\n"
	require.NoError(t, os.WriteFile(cfgPath, []byte(content), 0o644))
	os.Unsetenv("OPENAI_API_KEY")

	var stdoutBuf bytes.Buffer
	origOut := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	done := make(chan struct{})
	go func() {
		_, _ = io.Copy(&stdoutBuf, r)
		close(done)
	}()

	stderr := captureStderr(t, func() {
		code := runCLI([]string{"check", "--config", cfgPath})
		assert.Equal(t, exitSuccess, code)
	})

	require.NoError(t, w.Close())
	os.Stdout = origOut
	<-done

	assert.Contains(t, stdoutBuf.String(), "ok: config valid")
	assert.Contains(t, stderr, "OPENAI_API_KEY")
}

func TestRunCLI_checkStrictFailsOnMissingEnv(t *testing.T) {
	root := t.TempDir()
	cfgPath := root + string(os.PathSeparator) + "airon.yaml"
	content := "version: \"1\"\n" +
		"provider:\n  type: openai\n  model: gpt-4o\n  api_key: ${OPENAI_API_KEY}\n" +
		"prompts:\n  - id: a\n    content: \"go\"\n"
	require.NoError(t, os.WriteFile(cfgPath, []byte(content), 0o644))
	os.Unsetenv("OPENAI_API_KEY")

	_ = captureStderr(t, func() {
		code := runCLI([]string{"check", "--config", cfgPath, "--strict"})
		assert.Equal(t, exitRuntime, code)
	})
}

func TestVarFlags_SetAndString(t *testing.T) {
	var flags varFlags
	require.NoError(t, flags.Set("topic=testing"))
	require.NoError(t, flags.Set("lang=go"))

	assert.Contains(t, flags.String(), "topic=testing")
	assert.Contains(t, flags.String(), "lang=go")
	assert.Equal(t, "go", flags["lang"])

	err := flags.Set("invalid")
	require.Error(t, err)
}
