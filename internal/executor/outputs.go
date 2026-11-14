package executor

import (
	"fmt"

	"github.com/1k-off/airon/internal/workspace"
)

type workspaceOutputWriter struct {
	manager *workspace.Manager
}

func NewWorkspaceOutputWriter(manager *workspace.Manager) OutputWriter {
	return &workspaceOutputWriter{manager: manager}
}

func (w *workspaceOutputWriter) WriteStepOutput(stepID, content string) error {
	if w == nil || w.manager == nil {
		return fmt.Errorf("workspace output writer is not configured")
	}
	path := fmt.Sprintf("outputs/%s.md", stepID)
	return w.manager.Write(path, []byte(content))
}
