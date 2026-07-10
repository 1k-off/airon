package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/shared"
)

type openAIProvider struct {
	client       openai.Client
	defaultModel string
}

func NewOpenAI(apiKey, baseURL, model string) Provider {
	opts := []option.RequestOption{
		option.WithAPIKey(apiKey),
	}
	if strings.TrimSpace(baseURL) != "" {
		opts = append(opts, option.WithBaseURL(baseURL))
	}
	client := openai.NewClient(opts...)
	return &openAIProvider{
		client:       client,
		defaultModel: model,
	}
}

func (p *openAIProvider) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	params, err := toChatCompletionParams(req, p.defaultModel)
	if err != nil {
		return CompletionResponse{}, fmt.Errorf("provider complete: %w", err)
	}

	if req.OnDelta != nil {
		return p.completeStreaming(ctx, params, req.OnDelta)
	}

	completion, err := p.client.Chat.Completions.New(ctx, params)
	if err != nil {
		return CompletionResponse{}, fmt.Errorf("provider complete: %w", err)
	}

	return fromChatCompletion(completion), nil
}

func (p *openAIProvider) completeStreaming(
	ctx context.Context,
	params openai.ChatCompletionNewParams,
	onDelta func(string),
) (CompletionResponse, error) {
	stream := p.client.Chat.Completions.NewStreaming(ctx, params)
	defer stream.Close()

	acc := openai.ChatCompletionAccumulator{}
	for stream.Next() {
		chunk := stream.Current()
		acc.AddChunk(chunk)
		if len(chunk.Choices) > 0 {
			delta := chunk.Choices[0].Delta.Content
			if delta != "" {
				onDelta(delta)
			}
		}
	}
	if err := stream.Err(); err != nil {
		return CompletionResponse{}, fmt.Errorf("provider complete: %w", err)
	}

	if len(acc.Choices) == 0 {
		return CompletionResponse{}, nil
	}
	return fromChatCompletion(&acc.ChatCompletion), nil
}

func toChatCompletionParams(req CompletionRequest, defaultModel string) (openai.ChatCompletionNewParams, error) {
	model := req.Model
	if model == "" {
		model = defaultModel
	}

	messages := make([]openai.ChatCompletionMessageParamUnion, 0, len(req.Messages))
	for _, msg := range req.Messages {
		mapped, err := toOpenAIMessage(msg)
		if err != nil {
			return openai.ChatCompletionNewParams{}, err
		}
		messages = append(messages, mapped)
	}

	tools := make([]openai.ChatCompletionToolUnionParam, 0, len(req.Tools))
	for _, tool := range req.Tools {
		tools = append(tools, openai.ChatCompletionFunctionTool(shared.FunctionDefinitionParam{
			Name:        tool.Name,
			Description: openai.String(tool.Description),
			Parameters:  shared.FunctionParameters(tool.Parameters),
		}))
	}

	params := openai.ChatCompletionNewParams{
		Model:    shared.ChatModel(model),
		Messages: messages,
		Tools:    tools,
	}
	if req.Temperature != nil {
		params.Temperature = openai.Float(*req.Temperature)
	}

	return params, nil
}

func toOpenAIMessage(msg Message) (openai.ChatCompletionMessageParamUnion, error) {
	switch msg.Role {
	case RoleSystem:
		return openai.SystemMessage(msg.Content), nil
	case RoleUser:
		return openai.UserMessage(msg.Content), nil
	case RoleAssistant:
		assistant := openai.AssistantMessage(msg.Content)
		if len(msg.ToolCalls) > 0 {
			param := assistant.OfAssistant
			if param == nil {
				return openai.ChatCompletionMessageParamUnion{}, fmt.Errorf("assistant message mapping failed")
			}
			param.ToolCalls = make([]openai.ChatCompletionMessageToolCallUnionParam, 0, len(msg.ToolCalls))
			for _, tc := range msg.ToolCalls {
				param.ToolCalls = append(param.ToolCalls, openai.ChatCompletionMessageToolCallUnionParam{
					OfFunction: &openai.ChatCompletionMessageFunctionToolCallParam{
						ID: tc.ID,
						Function: openai.ChatCompletionMessageFunctionToolCallFunctionParam{
							Name:      tc.Name,
							Arguments: tc.Arguments,
						},
					},
				})
			}
		}
		return assistant, nil
	case RoleTool:
		return openai.ToolMessage(msg.Content, msg.ToolCallID), nil
	default:
		return openai.ChatCompletionMessageParamUnion{}, fmt.Errorf("unsupported message role %q", msg.Role)
	}
}

func fromChatCompletion(completion *openai.ChatCompletion) CompletionResponse {
	if completion == nil || len(completion.Choices) == 0 {
		return CompletionResponse{}
	}

	choice := completion.Choices[0]
	message := choice.Message

	resp := CompletionResponse{
		Content:      message.Content,
		FinishReason: choice.FinishReason,
	}

	for _, tc := range message.ToolCalls {
		switch v := tc.AsAny().(type) {
		case openai.ChatCompletionMessageFunctionToolCall:
			resp.ToolCalls = append(resp.ToolCalls, ToolCall{
				ID:        v.ID,
				Name:      v.Function.Name,
				Arguments: v.Function.Arguments,
			})
		}
	}

	if completion.Usage.JSON.TotalTokens.Valid() {
		resp.Usage = &TokenUsage{
			PromptTokens:     int(completion.Usage.PromptTokens),
			CompletionTokens: int(completion.Usage.CompletionTokens),
			TotalTokens:      int(completion.Usage.TotalTokens),
		}
	}

	return resp
}
