package runlog

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/1node/airon/internal/workspace"
)

type ToolCallRecord struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
	Result    string `json:"result,omitempty"`
	Error     string `json:"error,omitempty"`
	Duration  string `json:"duration"`
	Depth     int    `json:"depth"`
}

type StepRecord struct {
	ID             string           `json:"id"`
	RenderedPrompt string           `json:"rendered_prompt"`
	Output         string           `json:"output"`
	ToolCalls      []ToolCallRecord `json:"tool_calls"`
	Rounds         int              `json:"rounds"`
	StartedAt      time.Time        `json:"started_at"`
	FinishedAt     time.Time        `json:"finished_at"`
	Error          string           `json:"error,omitempty"`
}

type RunRecord struct {
	RunID      string            `json:"run_id"`
	ConfigPath string            `json:"config_path"`
	Input      map[string]string `json:"input"`
	Steps      []StepRecord      `json:"steps"`
	StartedAt  time.Time         `json:"started_at"`
	FinishedAt time.Time         `json:"finished_at"`
	Error      string            `json:"error,omitempty"`
}

type Logger interface {
	LogStep(record StepRecord)
	Flush(run RunRecord) error
}

type jsonLogger struct {
	ws     *workspace.Manager
	logger *slog.Logger
}

func NewLogger(ws *workspace.Manager, humanLogger *slog.Logger) Logger {
	logger := humanLogger
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(os.Stderr, nil))
	}
	return &jsonLogger{
		ws:     ws,
		logger: logger,
	}
}

func (l *jsonLogger) LogStep(record StepRecord) {
	if l == nil || l.logger == nil {
		return
	}

	attrs := []any{
		"step", record.ID,
		"rounds", record.Rounds,
		"tool_calls", len(record.ToolCalls),
		"duration", record.FinishedAt.Sub(record.StartedAt).String(),
	}
	if record.Error != "" {
		attrs = append(attrs, "error", record.Error)
		l.logger.Error("step failed", attrs...)
		return
	}
	l.logger.Info("step completed", attrs...)
}

func (l *jsonLogger) Flush(run RunRecord) error {
	if l == nil || l.ws == nil {
		return fmt.Errorf("runlog workspace manager is required")
	}
	if run.RunID == "" {
		return fmt.Errorf("run_id is required")
	}

	data, err := json.MarshalIndent(run, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal run log: %w", err)
	}

	path := fmt.Sprintf("logs/%s.json", run.RunID)
	if err := l.ws.Write(path, data); err != nil {
		return fmt.Errorf("write run log: %w", err)
	}
	return nil
}
