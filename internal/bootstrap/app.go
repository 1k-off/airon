package bootstrap

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/1node/airon/internal/agent"
	"github.com/1node/airon/internal/builtin"
	"github.com/1node/airon/internal/config"
	"github.com/1node/airon/internal/executor"
	"github.com/1node/airon/internal/mcp"
	"github.com/1node/airon/internal/provider"
	"github.com/1node/airon/internal/runlog"
	"github.com/1node/airon/internal/skill"
	"github.com/1node/airon/internal/tool"
	"github.com/1node/airon/internal/workspace"
)

type App struct {
	configPath string
	input      map[string]string
	cfg        *config.Config
	workspace  *workspace.Manager
	provider   provider.Provider
	registry   *tool.Registry
	executor   *executor.Executor
	logger     runlog.Logger
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
	if providerClient == nil {
		providerClient, err = provider.NewFromConfig(cfg.Provider)
		if err != nil {
			return nil, err
		}
	}

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
			Name:      server.Name,
			Transport: server.Transport,
			Command:   server.Command,
			Args:      server.Args,
			Env:       server.Env,
			URL:       server.URL,
			Headers:   server.Headers,
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

	var onDelta func(string)
	if cfg.Runtime.Stream || opts.Stream {
		onDelta = func(chunk string) {
			_, _ = os.Stdout.WriteString(chunk)
		}
	}

	fileReader := func(path string) (string, error) {
		data, readErr := ws.Read(path)
		if readErr != nil {
			return "", readErr
		}
		return string(data), nil
	}

	exec := executor.NewExecutor(executor.ExecutorConfig{
		Provider:      providerClient,
		Registry:      registry,
		Prompts:       cfg.Prompts,
		Model:         cfg.Provider.Model,
		Temperature:   cfg.Provider.Temperature,
		MaxToolRounds: cfg.EffectiveMaxToolRounds(),
		OnDelta:       onDelta,
		SaveOutputs:   cfg.EffectiveSaveOutputs(),
		Workspace:     executor.NewWorkspaceOutputWriter(ws),
		FileReader:    fileReader,
	})

	app := &App{
		configPath: filepath.Clean(cfgPath),
		input:      cloneInput(input),
		cfg:        cfg,
		workspace:  ws,
		provider:   providerClient,
		registry:   registry,
		executor:   exec,
		logger:     runlog.NewLogger(ws, newHumanLogger(cfg.Log.Level)),
		mcpClients: mcpClients,
	}
	return app, nil
}

func (a *App) Run(ctx context.Context) (executor.RunResult, error) {
	result, runErr := a.executor.Run(ctx, executor.RunInput{
		Variables: cloneInput(a.input),
	})

	stepRecords := make([]runlog.StepRecord, 0, len(result.Steps))
	for _, step := range result.Steps {
		record := toStepRecord(step)
		a.logger.LogStep(record)
		stepRecords = append(stepRecords, record)
	}

	runRecord := runlog.RunRecord{
		RunID:      result.RunID,
		ConfigPath: a.configPath,
		Input:      cloneInput(a.input),
		Steps:      stepRecords,
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
		StartedAt:      step.StartedAt,
		FinishedAt:     step.FinishedAt,
		Error:          step.Error,
	}
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

func newHumanLogger(level string) *slog.Logger {
	options := &slog.HandlerOptions{Level: parseLevel(level)}
	return slog.New(slog.NewTextHandler(os.Stderr, options))
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
