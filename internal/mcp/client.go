package mcp

import (
	"context"
	"fmt"

	"github.com/1node/airon/internal/tool"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type ServerConfig struct {
	Name      string
	Transport string
	Command   string
	Args      []string
	Env       map[string]string
	URL       string
	Headers   map[string]string
}

type sessionClient interface {
	ListTools(ctx context.Context, params *mcp.ListToolsParams) (*mcp.ListToolsResult, error)
	CallTool(ctx context.Context, params *mcp.CallToolParams) (*mcp.CallToolResult, error)
	Close() error
}

type Client struct {
	serverName string
	session    sessionClient
}

func Connect(ctx context.Context, cfg ServerConfig) (*Client, error) {
	if cfg.Name == "" {
		return nil, fmt.Errorf("mcp server name is required")
	}

	transport, err := buildTransport(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("mcp server %q: %w", cfg.Name, err)
	}

	client := mcp.NewClient(&mcp.Implementation{Name: "airon", Version: "v0.1.0"}, nil)
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, fmt.Errorf("connect mcp server %q: %w", cfg.Name, err)
	}

	return &Client{
		serverName: cfg.Name,
		session:    session,
	}, nil
}

func (c *Client) ListTools(ctx context.Context) ([]tool.Tool, error) {
	toolDefs, err := c.listToolDefinitions(ctx)
	if err != nil {
		return nil, fmt.Errorf("list mcp tools from %q: %w", c.serverName, err)
	}
	return wrapTools(c.serverName, c.session, toolDefs), nil
}

func (c *Client) Close() error {
	if c == nil || c.session == nil {
		return nil
	}
	return c.session.Close()
}

func (c *Client) listToolDefinitions(ctx context.Context) ([]*mcp.Tool, error) {
	cursor := ""
	allTools := make([]*mcp.Tool, 0)

	for {
		res, err := c.session.ListTools(ctx, &mcp.ListToolsParams{Cursor: cursor})
		if err != nil {
			return nil, err
		}
		if res == nil {
			return allTools, nil
		}

		allTools = append(allTools, res.Tools...)
		if res.NextCursor == "" {
			return allTools, nil
		}
		cursor = res.NextCursor
	}
}
