package runlog

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"
	"time"

	"github.com/1node/airon/internal/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogger_FlushWritesRunRecordJSON(t *testing.T) {
	ws, err := workspace.NewManager(t.TempDir())
	require.NoError(t, err)

	var logBuffer bytes.Buffer
	logger := NewLogger(ws, slog.New(slog.NewTextHandler(&logBuffer, nil)))

	start := time.Now().UTC().Truncate(time.Second)
	finish := start.Add(2 * time.Second)
	step := StepRecord{
		ID:             "research",
		RenderedPrompt: "Research testing",
		Output:         "done",
		ToolCalls: []ToolCallRecord{
			{
				ID:        "call-1",
				Name:      "fs_read",
				Arguments: `{"path":"README.md"}`,
				Result:    "content",
				Duration:  "1ms",
				Depth:     0,
			},
		},
		Rounds:     1,
		StartedAt:  start,
		FinishedAt: finish,
	}
	logger.LogStep(step)

	run := RunRecord{
		RunID:      "run-123",
		ConfigPath: "testdata/config.yaml",
		Input: map[string]string{
			"input": "hello",
		},
		Steps:      []StepRecord{step},
		StartedAt:  start,
		FinishedAt: finish,
	}

	require.NoError(t, logger.Flush(run))

	raw, err := ws.Read("logs/run-123.json")
	require.NoError(t, err)

	var decoded RunRecord
	require.NoError(t, json.Unmarshal(raw, &decoded))
	assert.Equal(t, run.RunID, decoded.RunID)
	assert.Equal(t, run.ConfigPath, decoded.ConfigPath)
	assert.Equal(t, run.Input, decoded.Input)
	require.Len(t, decoded.Steps, 1)
	assert.Equal(t, "research", decoded.Steps[0].ID)
	assert.Equal(t, "1ms", decoded.Steps[0].ToolCalls[0].Duration)
	assert.NotEmpty(t, logBuffer.String())
}
