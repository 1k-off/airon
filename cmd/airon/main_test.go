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
