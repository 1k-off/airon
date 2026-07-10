package runlog

import (
	"bytes"
	"log/slog"
	"testing"
	"time"

	"github.com/1node/airon/internal/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogger_LogStep_recordsFailure(t *testing.T) {
	var logBuffer bytes.Buffer
	logger := NewLogger(nil, slog.New(slog.NewTextHandler(&logBuffer, nil)))

	logger.LogStep(StepRecord{
		ID:         "broken",
		Error:      "step failed",
		StartedAt:  time.Now(),
		FinishedAt: time.Now().Add(time.Second),
	})

	assert.Contains(t, logBuffer.String(), "step failed")
	assert.Contains(t, logBuffer.String(), "step=broken")
}

func TestLogger_Flush_requiresWorkspaceAndRunID(t *testing.T) {
	logger := NewLogger(nil, slog.New(slog.NewTextHandler(ioDiscard{}, nil)))

	err := logger.Flush(RunRecord{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "workspace manager is required")

	ws, err := workspace.NewManager(t.TempDir())
	require.NoError(t, err)
	logger = NewLogger(ws, slog.New(slog.NewTextHandler(ioDiscard{}, nil)))

	err = logger.Flush(RunRecord{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "run_id is required")
}

type ioDiscard struct{}

func (ioDiscard) Write(p []byte) (int, error) { return len(p), nil }
