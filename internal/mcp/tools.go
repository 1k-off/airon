package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/1k-off/airon/internal/tool"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type wrappedTool struct {
	name        string
	remoteName  string
	description string
	parameters  map[string]any
	callTool    func(ctx context.Context, params *mcp.CallToolParams) (*mcp.CallToolResult, error)
}

func (t *wrappedTool) Name() string {
	return t.name
}

func (t *wrappedTool) Description() string {
	return t.description
}

func (t *wrappedTool) Parameters() map[string]any {
	return t.parameters
}

func (t *wrappedTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	arguments, err := decodeArguments(args)
	if err != nil {
		return "", fmt.Errorf("decode arguments for mcp tool %q: %w", t.name, err)
	}

	result, err := t.callTool(ctx, &mcp.CallToolParams{
		Name:      t.remoteName,
		Arguments: arguments,
	})
	if err != nil {
		return "", fmt.Errorf("call mcp tool %q: %w", t.name, err)
	}

	output, err := renderCallToolResult(result)
	if err != nil {
		return "", fmt.Errorf("render result for mcp tool %q: %w", t.name, err)
	}

	if result != nil && result.IsError {
		return output, fmt.Errorf("mcp tool %q returned an error result", t.name)
	}

	return output, nil
}

func wrapTools(serverName string, session sessionClient, definitions []*mcp.Tool) []tool.Tool {
	wrapped := make([]tool.Tool, 0, len(definitions))

	for _, definition := range definitions {
		if definition == nil {
			continue
		}

		current := *definition
		wrapped = append(wrapped, &wrappedTool{
			name:        namespacedToolName(serverName, current.Name),
			remoteName:  current.Name,
			description: current.Description,
			parameters:  normalizeSchema(current.InputSchema),
			callTool:    session.CallTool,
		})
	}

	return wrapped
}

func namespacedToolName(serverName string, toolName string) string {
	return serverName + "__" + toolName
}

func normalizeSchema(schema any) map[string]any {
	if schema == nil {
		return map[string]any{"type": "object"}
	}

	if typed, ok := schema.(map[string]any); ok {
		return copyMap(typed)
	}

	encoded, err := json.Marshal(schema)
	if err != nil {
		return map[string]any{"type": "object"}
	}

	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		return map[string]any{"type": "object"}
	}

	return decoded
}

func copyMap(source map[string]any) map[string]any {
	copied := make(map[string]any, len(source))
	for key, value := range source {
		copied[key] = value
	}
	return copied
}

func decodeArguments(raw json.RawMessage) (map[string]any, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return map[string]any{}, nil
	}

	var arguments map[string]any
	if err := json.Unmarshal(raw, &arguments); err != nil {
		return nil, err
	}
	if arguments == nil {
		return map[string]any{}, nil
	}
	return arguments, nil
}

func renderCallToolResult(result *mcp.CallToolResult) (string, error) {
	if result == nil {
		return "", nil
	}

	parts := make([]string, 0, len(result.Content))
	for _, content := range result.Content {
		text, err := renderContent(content)
		if err != nil {
			return "", err
		}
		if text != "" {
			parts = append(parts, text)
		}
	}

	if len(parts) > 0 {
		return strings.Join(parts, "\n"), nil
	}

	if result.StructuredContent != nil {
		encoded, err := json.Marshal(result.StructuredContent)
		if err != nil {
			return "", err
		}
		return string(encoded), nil
	}

	return "", nil
}

func renderContent(content mcp.Content) (string, error) {
	if textContent, ok := content.(*mcp.TextContent); ok {
		return textContent.Text, nil
	}

	encoded, err := content.MarshalJSON()
	if err != nil {
		return "", err
	}

	return string(encoded), nil
}

