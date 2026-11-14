package builtin

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/1k-off/airon/internal/tool"
	"github.com/1k-off/airon/internal/workspace"
)

func NewFSTools(ws *workspace.Manager) []tool.Tool {
	return []tool.Tool{
		&fsReadTool{ws: ws},
		&fsWriteTool{ws: ws},
		&fsListTool{ws: ws},
	}
}

type fsReadTool struct {
	ws *workspace.Manager
}

func (t *fsReadTool) Name() string        { return "fs_read" }
func (t *fsReadTool) Description() string { return "Read a file from the workspace." }
func (t *fsReadTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Path to file relative to workspace root.",
			},
		},
		"required": []string{"path"},
	}
}

func (t *fsReadTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var in struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(args, &in); err != nil {
		return "", fmt.Errorf("parse fs_read args: %w", err)
	}
	if in.Path == "" {
		return "", fmt.Errorf("path is required")
	}

	data, err := t.ws.Read(in.Path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

type fsWriteTool struct {
	ws *workspace.Manager
}

func (t *fsWriteTool) Name() string        { return "fs_write" }
func (t *fsWriteTool) Description() string { return "Write a file in the workspace." }
func (t *fsWriteTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Path to file relative to workspace root.",
			},
			"content": map[string]any{
				"type":        "string",
				"description": "File contents to write.",
			},
		},
		"required": []string{"path", "content"},
	}
}

func (t *fsWriteTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var in struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(args, &in); err != nil {
		return "", fmt.Errorf("parse fs_write args: %w", err)
	}
	if in.Path == "" {
		return "", fmt.Errorf("path is required")
	}

	if err := t.ws.Write(in.Path, []byte(in.Content)); err != nil {
		return "", err
	}
	return "ok", nil
}

type fsListTool struct {
	ws *workspace.Manager
}

func (t *fsListTool) Name() string        { return "fs_list" }
func (t *fsListTool) Description() string { return "List directory entries in the workspace." }
func (t *fsListTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Directory path relative to workspace root. Defaults to '.'.",
			},
		},
	}
}

func (t *fsListTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var in struct {
		Path string `json:"path"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &in); err != nil {
			return "", fmt.Errorf("parse fs_list args: %w", err)
		}
	}
	if in.Path == "" {
		in.Path = "."
	}

	entries, err := t.ws.List(in.Path)
	if err != nil {
		return "", err
	}
	out, err := json.Marshal(entries)
	if err != nil {
		return "", fmt.Errorf("marshal fs_list result: %w", err)
	}
	return string(out), nil
}
