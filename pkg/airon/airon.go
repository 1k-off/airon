// Package airon exposes airon as an embeddable library so it can be driven
// from Go code (cron wrappers, other services, tests) without shelling out
// to the airon binary.
//
// The public surface is intentionally minimal:
//
//	opts := airon.Options{ConfigPath: "./airon.yaml", Input: map[string]string{"input": "hello"}}
//	res, err := airon.Run(ctx, opts)
//
// Run wraps the same bootstrap pipeline used by the CLI: it loads the
// YAML config, builds the executor, invokes it, and (optionally) pings a
// healthchecks.io URL configured in notify.healthchecks.url. Nothing in this
// package calls os.Exit — callers are responsible for translating errors and
// Result.Success into their own exit semantics.
package airon

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/1k-off/airon/internal/bootstrap"
	"github.com/1k-off/airon/internal/config"
	"github.com/1k-off/airon/internal/executor"
	"github.com/1k-off/airon/internal/provider"
)

// Options configures a single embedded airon run.
type Options struct {
	// ConfigPath is the path to the airon YAML config file. Required.
	ConfigPath string

	// Input supplies template variables (e.g. "input") consumed by the
	// prompt bodies at render time. May be nil.
	Input map[string]string

	// Stream, when true, streams assistant deltas to Stdout even if the
	// config disables streaming.
	Stream bool

	// NoHealthchecks disables the healthchecks.io notifier for this run,
	// regardless of what notify.healthchecks.url resolves to in the config.
	// The AIRON_HEALTHCHECKS environment variable (values "0", "false",
	// "no", "off") also disables the notifier.
	NoHealthchecks bool

	// Stdout receives streamed assistant deltas. Nil defaults to os.Stdout.
	Stdout io.Writer

	// Stderr receives human log lines. Nil defaults to os.Stderr.
	Stderr io.Writer

	// Provider, when non-nil, replaces the config-derived LLM provider.
	// Intended for tests and advanced embedding. The value must implement
	// the same Complete contract as airon's internal providers; use
	// NewStaticProvider in tests.
	Provider Provider
}

// Provider is the embeddable LLM completion interface.
type Provider interface {
	Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error)
}

// MessageRole identifies a chat message author.
type MessageRole string

const (
	RoleSystem    MessageRole = "system"
	RoleUser      MessageRole = "user"
	RoleAssistant MessageRole = "assistant"
	RoleTool      MessageRole = "tool"
)

// Message is a single chat turn for an injected Provider.
type Message struct {
	Role       MessageRole
	Content    string
	ToolCallID string
	ToolCalls  []ToolCall
}

// ToolCall is a model-requested tool invocation.
type ToolCall struct {
	ID        string
	Name      string
	Arguments string
}

// ToolDefinition describes a tool the model may call.
type ToolDefinition struct {
	Name        string
	Description string
	Parameters  map[string]any
}

// CompletionRequest is the input to Provider.Complete.
type CompletionRequest struct {
	Model       string
	Messages    []Message
	Tools       []ToolDefinition
	Temperature *float64
	MaxTokens   *int
	OnDelta     func(string)
}

// CompletionResponse is the output from Provider.Complete.
type CompletionResponse struct {
	Content      string
	ToolCalls    []ToolCall
	FinishReason string
	Usage        *TokenUsage
}

// TokenUsage reports provider token accounting for one completion.
type TokenUsage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

// Result summarizes the outcome of a Run call.
type Result struct {
	RunID      string
	Success    bool
	Error      string
	StartedAt  time.Time
	FinishedAt time.Time
	Steps      []StepSummary
	Usage      Usage
	// WorkspaceRoot is the absolute sandboxed workspace path.
	WorkspaceRoot string
	// LogPath is the workspace-relative JSON run log path (logs/<run_id>.json).
	LogPath string
}

// StepSummary summarizes a single executed prompt step.
type StepSummary struct {
	ID         string
	Output     string
	Rounds     int
	Duration   time.Duration
	StartedAt  time.Time
	FinishedAt time.Time
	Error      string
	ToolCalls  int
	Usage      Usage
}

// Usage aggregates token counts reported by the underlying provider.
type Usage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

// CheckReport summarizes an offline config check.
type CheckReport struct {
	MissingEnv []string
	Warnings   []string
	Provider   string
	Model      string
	Prompts    int
	Skills     int
	MCPServers int
}

// testProviderOverride forces Run to use a given internal provider (tests only).
var testProviderOverride provider.Provider

// Run executes the configured airon job end-to-end and returns a summarized
// Result. It never calls os.Exit.
func Run(ctx context.Context, opts Options) (Result, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	appOpts := bootstrap.AppOptions{
		Stream:         opts.Stream,
		NoHealthchecks: opts.NoHealthchecks,
		Stdout:         opts.Stdout,
		Stderr:         opts.Stderr,
	}

	var (
		app *bootstrap.App
		err error
	)

	injected := testProviderOverride
	if opts.Provider != nil {
		injected = adaptProvider(opts.Provider)
	}

	if injected != nil {
		app, err = bootstrap.NewAppWithProviderAndOptions(ctx, opts.ConfigPath, opts.Input, injected, appOpts)
	} else {
		app, err = bootstrap.NewAppWithOptions(ctx, opts.ConfigPath, opts.Input, appOpts)
	}
	if err != nil {
		return Result{}, err
	}
	defer func() { _ = app.Close() }()

	runResult, runErr := app.Run(ctx)
	res := toResult(runResult, runErr)
	res.WorkspaceRoot = app.WorkspaceRoot()
	res.LogPath = app.LogPath(res.RunID)
	return res, runErr
}

// Check validates a config file offline (no LLM, no MCP dial).
func Check(path string) (CheckReport, error) {
	report, err := config.Check(path)
	out := CheckReport{MissingEnv: report.MissingEnv, Warnings: report.Warnings}
	if report.Config != nil {
		out.Provider = report.Config.Provider.Type
		out.Model = report.Config.Provider.Model
		out.Prompts = len(report.Config.Prompts)
		out.Skills = len(report.Config.Skills)
		out.MCPServers = len(report.Config.MCPServers)
	}
	return out, err
}

// NewStaticProvider returns a Provider that serves canned responses in order.
// Useful for embedding tests without a live LLM.
func NewStaticProvider(responses ...CompletionResponse) Provider {
	return &staticProvider{responses: responses}
}

type staticProvider struct {
	responses []CompletionResponse
	idx       int
}

func (p *staticProvider) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	if err := ctx.Err(); err != nil {
		return CompletionResponse{}, err
	}
	if p.idx >= len(p.responses) {
		return CompletionResponse{}, fmt.Errorf("static provider: no more canned responses")
	}
	if req.OnDelta != nil && p.responses[p.idx].Content != "" {
		req.OnDelta(p.responses[p.idx].Content)
	}
	resp := p.responses[p.idx]
	p.idx++
	return resp, nil
}

func adaptProvider(p Provider) provider.Provider {
	return &providerAdapter{inner: p}
}

type providerAdapter struct {
	inner Provider
}

func (a *providerAdapter) Complete(ctx context.Context, req provider.CompletionRequest) (provider.CompletionResponse, error) {
	pubReq := CompletionRequest{
		Model:       req.Model,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
		OnDelta:     req.OnDelta,
	}
	for _, m := range req.Messages {
		pm := Message{Role: MessageRole(m.Role), Content: m.Content, ToolCallID: m.ToolCallID}
		for _, tc := range m.ToolCalls {
			pm.ToolCalls = append(pm.ToolCalls, ToolCall{ID: tc.ID, Name: tc.Name, Arguments: tc.Arguments})
		}
		pubReq.Messages = append(pubReq.Messages, pm)
	}
	for _, t := range req.Tools {
		pubReq.Tools = append(pubReq.Tools, ToolDefinition{Name: t.Name, Description: t.Description, Parameters: t.Parameters})
	}
	resp, err := a.inner.Complete(ctx, pubReq)
	if err != nil {
		return provider.CompletionResponse{}, err
	}
	out := provider.CompletionResponse{Content: resp.Content, FinishReason: resp.FinishReason}
	for _, tc := range resp.ToolCalls {
		out.ToolCalls = append(out.ToolCalls, provider.ToolCall{ID: tc.ID, Name: tc.Name, Arguments: tc.Arguments})
	}
	if resp.Usage != nil {
		out.Usage = &provider.TokenUsage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		}
	}
	return out, nil
}

func toResult(r executor.RunResult, runErr error) Result {
	res := Result{
		RunID:      r.RunID,
		Success:    runErr == nil,
		StartedAt:  r.StartedAt,
		FinishedAt: r.FinishedAt,
		Usage:      usageFrom(r.Usage),
	}
	if runErr != nil {
		res.Error = runErr.Error()
	}
	if len(r.Steps) > 0 {
		res.Steps = make([]StepSummary, 0, len(r.Steps))
		for _, step := range r.Steps {
			res.Steps = append(res.Steps, StepSummary{
				ID:         step.ID,
				Output:     step.Output,
				Rounds:     step.Rounds,
				Duration:   step.Duration,
				StartedAt:  step.StartedAt,
				FinishedAt: step.FinishedAt,
				Error:      step.Error,
				ToolCalls:  len(step.ToolCalls),
				Usage:      usageFrom(step.Usage),
			})
		}
	}
	return res
}

func usageFrom(u provider.UsageStats) Usage {
	return Usage{
		PromptTokens:     u.PromptTokens,
		CompletionTokens: u.CompletionTokens,
		TotalTokens:      u.TotalTokens,
	}
}
