package mcp

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func buildTransport(ctx context.Context, cfg ServerConfig) (mcp.Transport, error) {
	transport := strings.TrimSpace(cfg.Transport)
	if transport == "" {
		transport = "stdio"
	}

	switch transport {
	case "stdio":
		return buildStdioTransport(ctx, cfg)
	case "streamable_http":
		return buildStreamableHTTPTransport(cfg)
	case "sse":
		return buildSSETransport(cfg)
	default:
		return nil, fmt.Errorf("unsupported mcp transport %q", transport)
	}
}

func buildStdioTransport(ctx context.Context, cfg ServerConfig) (mcp.Transport, error) {
	if strings.TrimSpace(cfg.Command) == "" {
		return nil, fmt.Errorf("mcp server command is required for stdio transport")
	}

	cmd := exec.CommandContext(ctx, cfg.Command, cfg.Args...)
	if len(cfg.Env) > 0 {
		env := os.Environ()
		for key, value := range cfg.Env {
			env = append(env, key+"="+value)
		}
		cmd.Env = env
	}

	return &mcp.CommandTransport{Command: cmd}, nil
}

func buildStreamableHTTPTransport(cfg ServerConfig) (mcp.Transport, error) {
	if strings.TrimSpace(cfg.URL) == "" {
		return nil, fmt.Errorf("mcp server url is required for streamable_http transport")
	}

	return &mcp.StreamableClientTransport{
		Endpoint:             cfg.URL,
		HTTPClient:           httpClientWithHeaders(cfg.Headers),
		DisableStandaloneSSE: true,
	}, nil
}

func buildSSETransport(cfg ServerConfig) (mcp.Transport, error) {
	if strings.TrimSpace(cfg.URL) == "" {
		return nil, fmt.Errorf("mcp server url is required for sse transport")
	}

	return &mcp.SSEClientTransport{
		Endpoint:   cfg.URL,
		HTTPClient: httpClientWithHeaders(cfg.Headers),
	}, nil
}

func httpClientWithHeaders(headers map[string]string) *http.Client {
	if len(headers) == 0 {
		return http.DefaultClient
	}

	transport := http.DefaultTransport
	if transport == nil {
		transport = &http.Transport{}
	}

	return &http.Client{
		Transport: roundTripperWithHeaders{
			base:    transport,
			headers: headers,
		},
	}
}

type roundTripperWithHeaders struct {
	base    http.RoundTripper
	headers map[string]string
}

func (r roundTripperWithHeaders) RoundTrip(req *http.Request) (*http.Response, error) {
	for key, value := range r.headers {
		req.Header.Set(key, value)
	}
	return r.base.RoundTrip(req)
}
