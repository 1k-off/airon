package executor

import (
	"context"
	"fmt"
	"time"

	"github.com/1k-off/airon/internal/config"
	"github.com/1k-off/airon/internal/depth"
	"github.com/1k-off/airon/internal/provider"
	"github.com/1k-off/airon/internal/template"
	"github.com/1k-off/airon/internal/tool"
)

type RunInput struct {
	Variables map[string]string
}

type ToolCallResult struct {
	ID        string
	Name      string
	Arguments string
	Result    string
	Error     string
	Duration  time.Duration
	Depth     int
}

type StepResult struct {
	ID             string
	RenderedPrompt string
	Output         string
	ToolCalls      []ToolCallResult
	Rounds         int
	Duration       time.Duration
	StartedAt      time.Time
	FinishedAt     time.Time
	Error          string
	Usage          provider.UsageStats
}

type RunResult struct {
	RunID      string
	StartedAt  time.Time
	FinishedAt time.Time
	Steps      []StepResult
	Error      error
	Usage      provider.UsageStats
}

type ExecutorConfig struct {
	Provider       provider.Provider
	Registry       *tool.Registry
	Prompts        []config.Prompt
	Model          string
	Temperature    *float64
	MaxTokens      *int
	MaxToolRounds  int
	OnDelta        func(string)
	OnStepComplete func(StepResult)
	SaveOutputs    bool
	Workspace      OutputWriter
	FileReader     template.FileReader
	UsageTracker   *provider.UsageTracker
}

// OutputWriter persists step outputs.
type OutputWriter interface {
	WriteStepOutput(stepID, content string) error
}

type Executor struct {
	provider       provider.Provider
	registry       *tool.Registry
	prompts        []config.Prompt
	model          string
	temperature    *float64
	maxTokens      *int
	maxToolRounds  int
	onDelta        func(string)
	onStepComplete func(StepResult)
	saveOutputs    bool
	workspace      OutputWriter
	fileReader     template.FileReader
	usageTracker   *provider.UsageTracker
}

func NewExecutor(cfg ExecutorConfig) *Executor {
	maxToolRounds := cfg.MaxToolRounds
	if maxToolRounds <= 0 {
		maxToolRounds = 10
	}

	promptsCopy := make([]config.Prompt, len(cfg.Prompts))
	copy(promptsCopy, cfg.Prompts)

	return &Executor{
		provider:       cfg.Provider,
		registry:       cfg.Registry,
		prompts:        promptsCopy,
		model:          cfg.Model,
		temperature:    cfg.Temperature,
		maxTokens:      cfg.MaxTokens,
		maxToolRounds:  maxToolRounds,
		onDelta:        cfg.OnDelta,
		onStepComplete: cfg.OnStepComplete,
		saveOutputs:    cfg.SaveOutputs,
		workspace:      cfg.Workspace,
		fileReader:     cfg.FileReader,
		usageTracker:   cfg.UsageTracker,
	}
}

func (e *Executor) Run(ctx context.Context, input RunInput) (RunResult, error) {
	started := time.Now()
	result := RunResult{
		RunID:     fmt.Sprintf("run-%d", started.UnixNano()),
		StartedAt: started,
		Steps:     make([]StepResult, 0, len(e.prompts)),
	}

	sorted, err := SortPrompts(e.prompts)
	if err != nil {
		result.FinishedAt = time.Now()
		result.Error = err
		return result, err
	}

	vars := cloneVars(input.Variables)
	for _, step := range sorted {
		stepStart := time.Now()
		stepResult := StepResult{ID: step.ID, StartedAt: stepStart}
		if e.usageTracker != nil {
			e.usageTracker.BeginStep(step.ID)
		}

		rendered, err := template.RenderWithFiles(step.Content, vars, e.fileReader)
		if err != nil {
			return e.failStep(&result, stepResult, stepStart, err)
		}
		stepResult.RenderedPrompt = rendered

		tools, allowed, err := e.stepTools(step.Tools)
		if err != nil {
			return e.failStep(&result, stepResult, stepStart, err)
		}

		messages := make([]provider.Message, 0, 2)
		if step.System != "" {
			systemPrompt, err := template.RenderWithFiles(step.System, vars, e.fileReader)
			if err != nil {
				return e.failStep(&result, stepResult, stepStart, err)
			}
			messages = append(messages, provider.Message{
				Role:    provider.RoleSystem,
				Content: systemPrompt,
			})
		}
		messages = append(messages, provider.Message{
			Role:    provider.RoleUser,
			Content: rendered,
		})

		output, rounds, calls, err := e.runStep(ctx, messages, tools, allowed)
		if err != nil {
			stepResult.Error = err.Error()
			stepResult.Rounds = rounds
			stepResult.ToolCalls = calls
			return e.failStep(&result, stepResult, stepStart, err)
		}

		stepResult.Output = output
		stepResult.Rounds = rounds
		stepResult.ToolCalls = calls
		stepResult.Duration = time.Since(stepStart)
		stepResult.FinishedAt = stepStart.Add(stepResult.Duration)
		if e.usageTracker != nil {
			stepResult.Usage = e.usageTracker.EndStep()
		}
		result.Steps = append(result.Steps, stepResult)
		if e.onStepComplete != nil {
			e.onStepComplete(stepResult)
		}
		vars[step.ID] = output

		if e.saveOutputs && e.workspace != nil && output != "" {
			if writeErr := e.workspace.WriteStepOutput(step.ID, output); writeErr != nil {
				return e.failStep(&result, stepResult, stepStart, writeErr)
			}
		}
	}

	result.FinishedAt = time.Now()
	if e.usageTracker != nil {
		result.Usage = e.usageTracker.TotalStats()
	}
	return result, nil
}

func (e *Executor) failStep(result *RunResult, stepResult StepResult, stepStart time.Time, err error) (RunResult, error) {
	stepResult.Error = err.Error()
	stepResult.Duration = time.Since(stepStart)
	stepResult.FinishedAt = stepStart.Add(stepResult.Duration)
	if e.usageTracker != nil {
		stepResult.Usage = e.usageTracker.EndStep()
	}
	result.Steps = append(result.Steps, stepResult)
	if e.onStepComplete != nil {
		e.onStepComplete(stepResult)
	}
	result.FinishedAt = time.Now()
	result.Error = err
	if e.usageTracker != nil {
		result.Usage = e.usageTracker.TotalStats()
	}
	return *result, err
}

func (e *Executor) runStep(
	ctx context.Context,
	messages []provider.Message,
	tools []provider.ToolDefinition,
	allowed map[string]tool.Tool,
) (string, int, []ToolCallResult, error) {
	toolCalls := make([]ToolCallResult, 0)

	for rounds := 1; rounds <= e.maxToolRounds; rounds++ {
		resp, err := e.provider.Complete(ctx, provider.CompletionRequest{
			Model:       e.model,
			Messages:    messages,
			Tools:       tools,
			Temperature: e.temperature,
			MaxTokens:   e.maxTokens,
			OnDelta:     e.onDelta,
		})
		if err != nil {
			return "", rounds, toolCalls, fmt.Errorf("provider complete: %w", err)
		}

		if len(resp.ToolCalls) == 0 {
			return resp.Content, rounds, toolCalls, nil
		}

		messages = append(messages, provider.Message{
			Role:      provider.RoleAssistant,
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		})

		for _, call := range resp.ToolCalls {
			callStart := time.Now()
			target, ok := allowed[call.Name]
			toolCtx := depth.WithIncrement(ctx)
			var toolResult string
			var execErr error
			if !ok {
				toolResult = fmt.Sprintf("error: tool %q is not allowed in this step", call.Name)
				execErr = fmt.Errorf("tool %q is not allowed in this step", call.Name)
			} else {
				toolResult, execErr = target.Execute(toolCtx, []byte(call.Arguments))
				if execErr != nil {
					toolResult = fmt.Sprintf("error: %v", execErr)
				}
			}

			callRecord := ToolCallResult{
				ID:        call.ID,
				Name:      call.Name,
				Arguments: call.Arguments,
				Result:    toolResult,
				Duration:  time.Since(callStart),
				Depth:     depth.FromContext(toolCtx),
			}
			if execErr != nil {
				callRecord.Error = execErr.Error()
			}
			toolCalls = append(toolCalls, callRecord)

			messages = append(messages, provider.Message{
				Role:       provider.RoleTool,
				Content:    toolResult,
				ToolCallID: call.ID,
			})
		}
	}

	return "", e.maxToolRounds, toolCalls, fmt.Errorf("max tool rounds exceeded: %d", e.maxToolRounds)
}

func (e *Executor) stepTools(names []string) ([]provider.ToolDefinition, map[string]tool.Tool, error) {
	if e.registry == nil {
		if len(names) > 0 {
			return nil, nil, fmt.Errorf("tool registry is required for step tools")
		}
		return nil, nil, nil
	}

	if len(names) == 0 {
		allowed := make(map[string]tool.Tool, len(e.registry.List()))
		for _, t := range e.registry.List() {
			allowed[t.Name()] = t
		}
		return e.registry.Definitions(), allowed, nil
	}

	defs := make([]provider.ToolDefinition, 0, len(names))
	allowed := make(map[string]tool.Tool, len(names))
	for _, name := range names {
		t, ok := e.registry.Get(name)
		if !ok {
			return nil, nil, fmt.Errorf("unknown tool: %s", name)
		}
		defs = append(defs, provider.ToolDefinition{
			Name:        t.Name(),
			Description: t.Description(),
			Parameters:  t.Parameters(),
		})
		allowed[name] = t
	}
	return defs, allowed, nil
}

func cloneVars(vars map[string]string) map[string]string {
	if len(vars) == 0 {
		return make(map[string]string)
	}
	out := make(map[string]string, len(vars))
	for k, v := range vars {
		out[k] = v
	}
	return out
}
