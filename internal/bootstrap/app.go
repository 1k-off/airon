package bootstrap

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/1k-off/airon/internal/agent"
	"github.com/1k-off/airon/internal/builtin"
	"github.com/1k-off/airon/internal/config"
	"github.com/1k-off/airon/internal/executor"
	"github.com/1k-off/airon/internal/mcp"
	"github.com/1k-off/airon/internal/notify"
	"github.com/1k-off/airon/internal/provider"
	"github.com/1k-off/airon/internal/runlog"
	"github.com/1k-off/airon/internal/skill"
	"github.com/1k-off/airon/internal/tool"
	"github.com/1k-off/airon/internal/workspace"
)

// EnvHealthchecks is the name of the environment variable that can be set to
// "0", "false", or "no" (case-insensitive) to disable the healthchecks
// notifier for a run without changing the config or the CLI invocation.
const EnvHealthchecks = "AIRON_HEALTHCHECKS"

type App struct {
	configPath string
	input      map[string]string
	cfg        *config.Config
	workspace  *workspace.Manager
	provider   provider.Provider
	registry   *tool.Registry
	executor   *executor.Executor
	logger     runlog.Logger
	slog       *slog.Logger
	notifier   *notify.Healthchecks
	mcpClients []*mcp.Client
}

func NewApp(cfgPath string, input map[string]string) (*App, error) {
	return newApp(context.Background(), cfgPath, input, nil, AppOptions{})
}

func NewAppWithOptions(ctx context.Context, cfgPath string, input map[string]string, opts AppOptions) (*App, error) {
	return newApp(ctx, cfgPath, input, nil, opts)
}

func NewAppWithProvider(cfgPath string, input map[string]string, providerClient provider.Provider) (*App, error) {
	if providerClient == nil {
		return nil, fmt.Errorf("provider is required")
	}
	return newApp(context.Background(), cfgPath, input, providerClient, AppOptions{})
}

// NewAppWithProviderAndOptions is like NewAppWithProvider but also accepts
// AppOptions. Intended for callers (including tests) that need to inject a
// provider while still exercising runtime option toggles such as
// NoHealthchecks or Stream.
func NewAppWithProviderAndOptions(ctx context.Context, cfgPath string, input map[string]string, providerClient provider.Provider, opts AppOptions) (*App, error) {
	if providerClient == nil {
		return nil, fmt.Errorf("provider is required")
	}
	return newApp(ctx, cfgPath, input, providerClient, opts)
}

func newApp(ctx context.Context, cfgPath string, input map[string]string, injectedProvider provider.Provider, opts AppOptions) (*App, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, err
	}

	ws, err := workspace.NewManager(cfg.Workspace.Path)
	if err != nil {
		return nil, fmt.Errorf("init workspace: %w", err)
	}

	providerClient := injectedProvider
	usageTracker := provider.NewUsageTracker()
	if providerClient == nil {
		providerClient, err = provider.NewFromConfig(cfg.Provider)
		if err != nil {
			return nil, err
		}
	}
	providerClient = provider.NewTrackingProvider(providerClient, cfg.Provider.Model, usageTracker)

	allTools := make([]tool.Tool, 0)
	allTools = append(allTools, builtin.NewFSTools(ws)...)

	skillPaths := make([]string, 0, len(cfg.Skills))
	for _, skillCfg := range cfg.Skills {
		skillPaths = append(skillPaths, skillCfg.Path)
	}
	skillTools, err := skill.LoadAllConfigured(skillPaths, providerClient, cfg.EffectiveMaxCallDepth())
	if err != nil {
		return nil, err
	}
	allTools = append(allTools, skillTools...)

	mcpClients := make([]*mcp.Client, 0, len(cfg.MCPServers))
	for _, server := range cfg.MCPServers {
		client, connectErr := mcp.Connect(ctx, mcp.ServerConfig{
			Name:           server.Name,
			Transport:      server.Transport,
			Command:        server.Command,
			Args:           server.Args,
			Env:            server.Env,
			URL:            server.URL,
			Headers:        server.Headers,
			Timeout:        server.Timeout,
			EnvPassthrough: server.EnvPassthrough,
		})
		if connectErr != nil {
			closeClients(mcpClients)
			return nil, connectErr
		}
		tools, listErr := client.ListTools(ctx)
		if listErr != nil {
			_ = client.Close()
			closeClients(mcpClients)
			return nil, listErr
		}

		mcpClients = append(mcpClients, client)
		allTools = append(allTools, tools...)
	}

	baseRegistry, err := tool.NewRegistry(allTools...)
	if err != nil {
		closeClients(mcpClients)
		return nil, fmt.Errorf("init tool registry: %w", err)
	}

	agentTools, err := agent.LoadAgents(
		cfg.Agents,
		baseRegistry,
		providerClient,
		cfg.Provider.Model,
		cfg.EffectiveMaxCallDepth(),
		cfg.EffectiveMaxToolRounds(),
	)
	if err != nil {
		closeClients(mcpClients)
		return nil, err
	}

	allTools = append(allTools, agentTools...)
	registry, err := tool.NewRegistry(allTools...)
	if err != nil {
		closeClients(mcpClients)
		return nil, fmt.Errorf("init final tool registry: %w", err)
	}

	if err := config.ValidatePromptTools(cfg.Prompts, knownToolNames(registry)); err != nil {
		closeClients(mcpClients)
		return nil, err
	}

	stdout := opts.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}
	stderr := opts.Stderr
	if stderr == nil {
		stderr = os.Stderr
	}

	var onDelta func(string)
	if cfg.Runtime.Stream || opts.Stream {
		onDelta = func(chunk string) {
			_, _ = io.WriteString(stdout, chunk)
		}
	}

	fileReader := func(path string) (string, error) {
		data, readErr := ws.Read(path)
		if readErr != nil {
			return "", readErr
		}
		return string(data), nil
	}

	humanLogger := newHumanLogger(cfg.Log.Level, stderr)
	logger := runlog.NewLogger(ws, humanLogger)

	notifier := notify.NewHealthchecks(
		cfg.Notify.Healthchecks.URL,
		healthchecksDisabled(opts.NoHealthchecks, os.Getenv(EnvHealthchecks)),
	)

	exec := executor.NewExecutor(executor.ExecutorConfig{
		Provider:      providerClient,
		Registry:      registry,
		Prompts:       cfg.Prompts,
		Model:         cfg.Provider.Model,
		Temperature:   cfg.Provider.Temperature,
		MaxTokens:     cfg.Provider.MaxTokens,
		MaxToolRounds: cfg.EffectiveMaxToolRounds(),
		OnDelta:       onDelta,
		OnStepComplete: func(step executor.StepResult) {
			logger.LogStep(toStepRecord(step))
		},
		SaveOutputs:  cfg.EffectiveSaveOutputs(),
		Workspace:    executor.NewWorkspaceOutputWriter(ws),
		FileReader:   fileReader,
		UsageTracker: usageTracker,
	})

	app := &App{
		configPath: filepath.Clean(cfgPath),
		input:      cloneInput(input),
		cfg:        cfg,
		workspace:  ws,
		provider:   providerClient,
		registry:   registry,
		executor:   exec,
		logger:     logger,
		slog:       humanLogger,
		notifier:   notifier,
		mcpClients: mcpClients,
	}
	return app, nil
}

func (a *App) Run(ctx context.Context) (executor.RunResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	runCtx := ctx
	cancel := func() {}
	if timeout := strings.TrimSpace(a.cfg.Runtime.Timeout); timeout != "" {
		if d, err := time.ParseDuration(timeout); err == nil && d > 0 {
			runCtx, cancel = context.WithTimeout(ctx, d)
		}
	}
	defer cancel()

	if a.notifier != nil && a.notifier.Enabled() {
		if err := a.notifier.Start(runCtx); err != nil {
			a.logNotifyError("start", err)
		}
	}

	result, runErr := a.executor.Run(runCtx, executor.RunInput{
		Variables: cloneInput(a.input),
	})

	// Post-run pings must survive cancellation of the run context (SIGINT,
	// runtime.timeout, caller deadline) — that is exactly when operators need
	// the /fail notification.
	notifyCtx, notifyCancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer notifyCancel()
	if a.notifier != nil && a.notifier.Enabled() {
		if runErr != nil {
			if err := a.notifier.Fail(notifyCtx, runErr.Error()); err != nil {
				a.logNotifyError("fail", err)
			}
		} else {
			if err := a.notifier.Success(notifyCtx); err != nil {
				a.logNotifyError("success", err)
			}
		}
	}

	stepRecords := make([]runlog.StepRecord, 0, len(result.Steps))
	for _, step := range result.Steps {
		stepRecords = append(stepRecords, toStepRecord(step))
	}

	inputForLog := map[string]string(nil)
	if a.cfg.EffectiveLogInputs() {
		inputForLog = runlog.RedactInput(cloneInput(a.input))
	}
	for i := range stepRecords {
		stepRecords[i].RenderedPrompt = runlog.RedactString(stepRecords[i].RenderedPrompt, a.input)
	}

	runRecord := runlog.RunRecord{
		RunID:      result.RunID,
		ConfigPath: a.configPath,
		Input:      inputForLog,
		Steps:      stepRecords,
		Usage:      toUsageRecord(result.Usage),
		StartedAt:  result.StartedAt,
		FinishedAt: result.FinishedAt,
	}
	if runErr != nil {
		runRecord.Error = runErr.Error()
	}

	if flushErr := a.logger.Flush(runRecord); flushErr != nil {
		if runErr != nil {
			return result, fmt.Errorf("%w (also failed to flush run log: %v)", runErr, flushErr)
		}
		return result, fmt.Errorf("flush run log: %w", flushErr)
	}

	if runErr != nil {
		return result, runErr
	}
	return result, nil
}

// WorkspaceRoot returns the sandboxed workspace absolute path.
func (a *App) WorkspaceRoot() string {
	if a == nil || a.workspace == nil {
		return ""
	}
	return a.workspace.Root
}

// LogPath returns the workspace-relative run log path for a run ID.
func (a *App) LogPath(runID string) string {
	if runID == "" {
		return ""
	}
	return fmt.Sprintf("logs/%s.json", runID)
}

func (a *App) Close() error {
	if len(a.mcpClients) == 0 {
		return nil
	}

	var errs []string
	for _, client := range a.mcpClients {
		if err := client.Close(); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("close mcp clients: %s", strings.Join(errs, "; "))
	}
	return nil
}

func toStepRecord(step executor.StepResult) runlog.StepRecord {
	toolCalls := make([]runlog.ToolCallRecord, 0, len(step.ToolCalls))
	for _, call := range step.ToolCalls {
		toolCalls = append(toolCalls, runlog.ToolCallRecord{
			ID:        call.ID,
			Name:      call.Name,
			Arguments: call.Arguments,
			Result:    call.Result,
			Error:     call.Error,
			Duration:  call.Duration.String(),
			Depth:     call.Depth,
		})
	}
	return runlog.StepRecord{
		ID:             step.ID,
		RenderedPrompt: step.RenderedPrompt,
		Output:         step.Output,
		ToolCalls:      toolCalls,
		Rounds:         step.Rounds,
		Usage:          toUsageRecord(step.Usage),
		StartedAt:      step.StartedAt,
		FinishedAt:     step.FinishedAt,
		Error:          step.Error,
	}
}

func toUsageRecord(stats provider.UsageStats) *runlog.UsageRecord {
	if stats.TotalTokens == 0 && stats.PromptTokens == 0 && stats.CompletionTokens == 0 {
		return nil
	}
	record := &runlog.UsageRecord{
		PromptTokens:     stats.PromptTokens,
		CompletionTokens: stats.CompletionTokens,
		TotalTokens:      stats.TotalTokens,
	}
	if len(stats.ByModel) > 0 {
		record.ByModel = make([]runlog.ModelUsageRecord, 0, len(stats.ByModel))
		for _, model := range stats.ByModel {
			record.ByModel = append(record.ByModel, runlog.ModelUsageRecord{
				Model:            model.Model,
				PromptTokens:     model.PromptTokens,
				CompletionTokens: model.CompletionTokens,
				TotalTokens:      model.TotalTokens,
			})
		}
	}
	return record
}

func cloneInput(input map[string]string) map[string]string {
	if len(input) == 0 {
		return map[string]string{}
	}
	cloned := make(map[string]string, len(input))
	for key, value := range input {
		cloned[key] = value
	}
	return cloned
}

func newHumanLogger(level string, stderr io.Writer) *slog.Logger {
	if stderr == nil {
		stderr = os.Stderr
	}
	options := &slog.HandlerOptions{Level: parseLevel(level)}
	return slog.New(slog.NewTextHandler(stderr, options))
}

func parseLevel(level string) slog.Leveler {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// healthchecksDisabled reports whether the healthchecks notifier should be
// disabled. It considers both the AppOptions.NoHealthchecks toggle and the
// AIRON_HEALTHCHECKS environment variable, which disables the notifier when
// set to "0", "false", or "no" (case-insensitive).
func healthchecksDisabled(optDisabled bool, envValue string) bool {
	if optDisabled {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(envValue)) {
	case "0", "false", "no", "off":
		return true
	}
	return false
}

func (a *App) logNotifyError(phase string, err error) {
	if a == nil || a.slog == nil || err == nil {
		return
	}
	a.slog.Warn("healthchecks notify failed", "phase", phase, "error", err.Error())
}

func closeClients(clients []*mcp.Client) {
	for _, client := range clients {
		_ = client.Close()
	}
}

func knownToolNames(registry *tool.Registry) map[string]struct{} {
	known := make(map[string]struct{})
	if registry == nil {
		return known
	}
	for _, t := range registry.List() {
		known[t.Name()] = struct{}{}
	}
	return known
}
