package mcp

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const defaultHTTPTimeout = 60 * time.Second

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
	cmd.Env = buildStdioEnv(cfg)
	// Keep noisy MCP stderr out of the operator's main log stream unless debug.
	cmd.Stderr = &slogWriter{logger: slog.Default(), level: slog.LevelDebug, prefix: "mcp/" + cfg.Name}

	return &mcp.CommandTransport{Command: cmd}, nil
}

func buildStdioEnv(cfg ServerConfig) []string {
	passthrough := true
	if cfg.EnvPassthrough != nil {
		passthrough = *cfg.EnvPassthrough
	}

	if passthrough {
		env := os.Environ()
		for key, value := range cfg.Env {
			env = append(env, key+"="+value)
		}
		return env
	}

	// Minimal env so binaries still resolve without inheriting API keys.
	env := make([]string, 0, len(cfg.Env)+4)
	for _, key := range []string{"PATH", "Path", "SystemRoot", "HOME", "USERPROFILE"} {
		if v, ok := os.LookupEnv(key); ok {
			env = append(env, key+"="+v)
		}
	}
	for key, value := range cfg.Env {
		env = append(env, key+"="+value)
	}
	return env
}

func buildStreamableHTTPTransport(cfg ServerConfig) (mcp.Transport, error) {
	if strings.TrimSpace(cfg.URL) == "" {
		return nil, fmt.Errorf("mcp server url is required for streamable_http transport")
	}

	return &mcp.StreamableClientTransport{
		Endpoint:             cfg.URL,
		HTTPClient:           httpClientWithHeaders(cfg.Headers, parseTimeout(cfg.Timeout, defaultHTTPTimeout)),
		DisableStandaloneSSE: true,
	}, nil
}

func buildSSETransport(cfg ServerConfig) (mcp.Transport, error) {
	if strings.TrimSpace(cfg.URL) == "" {
		return nil, fmt.Errorf("mcp server url is required for sse transport")
	}

	return &mcp.SSEClientTransport{
		Endpoint:   cfg.URL,
		HTTPClient: httpClientWithHeaders(cfg.Headers, parseTimeout(cfg.Timeout, defaultHTTPTimeout)),
	}, nil
}

func parseTimeout(raw string, fallback time.Duration) time.Duration {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		return fallback
	}
	return d
}

func httpClientWithHeaders(headers map[string]string, timeout time.Duration) *http.Client {
	transport := http.DefaultTransport
	if transport == nil {
		transport = &http.Transport{}
	}

	client := &http.Client{Timeout: timeout}
	if len(headers) == 0 {
		client.Transport = transport
		return client
	}

	client.Transport = roundTripperWithHeaders{
		base:    transport,
		headers: headers,
	}
	return client
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

// slogWriter adapts slog to io.Writer for subprocess stderr.
type slogWriter struct {
	logger *slog.Logger
	level  slog.Level
	prefix string
}

func (w *slogWriter) Write(p []byte) (int, error) {
	if w == nil || w.logger == nil {
		return len(p), nil
	}
	msg := strings.TrimRight(string(p), "\r\n")
	if msg != "" {
		w.logger.Log(context.Background(), w.level, w.prefix, "stderr", msg)
	}
	return len(p), nil
}

var _ io.Writer = (*slogWriter)(nil)
