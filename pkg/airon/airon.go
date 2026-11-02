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
	"time"

	"github.com/1k-off/airon/internal/bootstrap"
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

	// Stream, when true, streams assistant deltas to stdout even if the
	// config disables streaming.
	Stream bool

	// NoHealthchecks disables the healthchecks.io notifier for this run,
	// regardless of what notify.healthchecks.url resolves to in the config.
	// The AIRON_HEALTHCHECKS environment variable (values "0", "false",
	// "no", "off") also disables the notifier.
	NoHealthchecks bool
}

// Result summarizes the outcome of a Run call. It intentionally exposes only
// the fields most useful to embedding callers; consumers that need the full
// executor state should call bootstrap.NewAppWithOptions directly.
type Result struct {
	// RunID is the unique identifier assigned by the executor. Empty when
	// bootstrap failed before the executor started.
	RunID string

	// Success is true when the executor finished all prompts without error.
	Success bool

	// Error is a copy of the primary run error message (if any). It is the
	// same value returned as the second return of Run.
	Error string

	// StartedAt and FinishedAt bracket the executor invocation. Zero when
	// the run did not reach the executor.
	StartedAt  time.Time
	FinishedAt time.Time

	// Steps summarizes each prompt step in execution order.
	Steps []StepSummary

	// Usage aggregates token accounting across the run.
	Usage Usage
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

// testProviderOverride, when non-nil, forces Run to use the given provider
// instead of constructing one from the config. It is unexported so it cannot
// be set from outside the package; internal tests set it via
// setTestProvider in the same package. In production builds it is always nil.
var testProviderOverride provider.Provider

// Run executes the configured airon job end-to-end and returns a summarized
// Result. It wraps bootstrap.NewAppWithOptions + App.Run + App.Close so that
// the CLI and library share the same runtime path.
//
// Run never calls os.Exit. If setup fails (bad config, MCP handshake, etc.)
// Run returns a zero-valued Result and the underlying error. If the run
// starts but a prompt fails, Run returns a Result with Success=false and the
// error value. Healthchecks notifier failures never override the primary
// run error — they are logged and swallowed at the bootstrap layer.
func Run(ctx context.Context, opts Options) (Result, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	appOpts := bootstrap.AppOptions{
		Stream:         opts.Stream,
		NoHealthchecks: opts.NoHealthchecks,
	}

	var (
		app *bootstrap.App
		err error
	)
	if testProviderOverride != nil {
		app, err = bootstrap.NewAppWithProviderAndOptions(ctx, opts.ConfigPath, opts.Input, testProviderOverride, appOpts)
	} else {
		app, err = bootstrap.NewAppWithOptions(ctx, opts.ConfigPath, opts.Input, appOpts)
	}
	if err != nil {
		return Result{}, err
	}
	defer func() { _ = app.Close() }()

	runResult, runErr := app.Run(ctx)
	res := toResult(runResult, runErr)
	return res, runErr
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
