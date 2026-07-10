package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

type anthropicProvider struct {
	client       anthropic.Client
	defaultModel string
}

func NewAnthropic(apiKey, baseURL, model string) Provider {
	opts := []option.RequestOption{
		option.WithAPIKey(apiKey),
	}
	if strings.TrimSpace(baseURL) != "" {
		opts = append(opts, option.WithBaseURL(baseURL))
	}
	return &anthropicProvider{
		client:       anthropic.NewClient(opts...),
		defaultModel: model,
	}
}

func (p *anthropicProvider) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	params, err := toAnthropicParams(req, p.defaultModel)
	if err != nil {
		return CompletionResponse{}, fmt.Errorf("provider complete: %w", err)
	}

	if req.OnDelta != nil {
		return p.completeStreaming(ctx, params, req.OnDelta)
	}

	message, err := p.client.Messages.New(ctx, params)
	if err != nil {
		return CompletionResponse{}, fmt.Errorf("provider complete: %w", err)
	}

	return fromAnthropicMessage(message), nil
}

func (p *anthropicProvider) completeStreaming(
	ctx context.Context,
	params anthropic.MessageNewParams,
	onDelta func(string),
) (CompletionResponse, error) {
	stream := p.client.Messages.NewStreaming(ctx, params)
	defer stream.Close()

	var message anthropic.Message
	for stream.Next() {
		event := stream.Current()
		if err := message.Accumulate(event); err != nil {
			return CompletionResponse{}, fmt.Errorf("provider complete: %w", err)
		}
		if event.Type == "content_block_delta" {
			if event.Delta.Type == "text_delta" && event.Delta.Text != "" {
				onDelta(event.Delta.Text)
			}
		}
	}
	if err := stream.Err(); err != nil {
		return CompletionResponse{}, fmt.Errorf("provider complete: %w", err)
	}

	return fromAnthropicMessage(&message), nil
}

func toAnthropicParams(req CompletionRequest, defaultModel string) (anthropic.MessageNewParams, error) {
	model := req.Model
	if model == "" {
		model = defaultModel
	}

	var systemBlocks []anthropic.TextBlockParam
	messages := make([]anthropic.MessageParam, 0, len(req.Messages))

	for _, msg := range req.Messages {
		switch msg.Role {
		case RoleSystem:
			systemBlocks = append(systemBlocks, anthropic.TextBlockParam{Text: msg.Content})
		case RoleUser:
			messages = append(messages, anthropic.NewUserMessage(anthropic.NewTextBlock(msg.Content)))
		case RoleAssistant:
			blocks := []anthropic.ContentBlockParamUnion{}
			if msg.Content != "" {
				blocks = append(blocks, anthropic.NewTextBlock(msg.Content))
			}
			for _, call := range msg.ToolCalls {
				var input any
				if err := json.Unmarshal([]byte(call.Arguments), &input); err != nil {
					input = map[string]any{"raw": call.Arguments}
				}
				blocks = append(blocks, anthropic.ContentBlockParamUnion{
					OfToolUse: &anthropic.ToolUseBlockParam{
						ID:    call.ID,
						Name:  call.Name,
						Input: input,
					},
				})
			}
			messages = append(messages, anthropic.NewAssistantMessage(blocks...))
		case RoleTool:
			messages = append(messages, anthropic.NewUserMessage(anthropic.ContentBlockParamUnion{
				OfToolResult: &anthropic.ToolResultBlockParam{
					ToolUseID: msg.ToolCallID,
					Content: []anthropic.ToolResultBlockParamContentUnion{
						{OfText: &anthropic.TextBlockParam{Text: msg.Content}},
					},
				},
			}))
		default:
			return anthropic.MessageNewParams{}, fmt.Errorf("unsupported message role %q", msg.Role)
		}
	}

	tools := make([]anthropic.ToolUnionParam, 0, len(req.Tools))
	for _, tool := range req.Tools {
		properties, _ := tool.Parameters["properties"].(map[string]any)
		if properties == nil {
			properties = map[string]any{}
		}
		required := extractRequired(tool.Parameters)
		tools = append(tools, anthropic.ToolUnionParam{
			OfTool: &anthropic.ToolParam{
				Name: tool.Name,
				Description: anthropic.String(tool.Description),
				InputSchema: anthropic.ToolInputSchemaParam{
					Properties: properties,
					Required:   required,
				},
			},
		})
	}

	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(model),
		MaxTokens: 4096,
		Messages:  messages,
		Tools:     tools,
	}
	if len(systemBlocks) > 0 {
		params.System = systemBlocks
	}
	if req.Temperature != nil {
		params.Temperature = anthropic.Float(*req.Temperature)
	}

	return params, nil
}

func extractRequired(schema map[string]any) []string {
	raw, ok := schema["required"].([]any)
	if !ok {
		if typed, ok := schema["required"].([]string); ok {
			return typed
		}
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func fromAnthropicMessage(message *anthropic.Message) CompletionResponse {
	if message == nil {
		return CompletionResponse{}
	}

	resp := CompletionResponse{
		FinishReason: string(message.StopReason),
	}

	for _, block := range message.Content {
		switch variant := block.AsAny().(type) {
		case anthropic.TextBlock:
			resp.Content += variant.Text
		case anthropic.ToolUseBlock:
			args, err := json.Marshal(variant.Input)
			if err != nil {
				args = []byte("{}")
			}
			resp.ToolCalls = append(resp.ToolCalls, ToolCall{
				ID:        variant.ID,
				Name:      variant.Name,
				Arguments: string(args),
			})
		}
	}

	if message.Usage.InputTokens > 0 || message.Usage.OutputTokens > 0 {
		resp.Usage = &TokenUsage{
			PromptTokens:     int(message.Usage.InputTokens),
			CompletionTokens: int(message.Usage.OutputTokens),
			TotalTokens:      int(message.Usage.InputTokens + message.Usage.OutputTokens),
		}
	}

	return resp
}
