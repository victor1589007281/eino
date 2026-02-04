package llm

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

type ChatModelAdapter struct {
	provider *OpenAIProvider
	model    string
	tools    []*schema.ToolInfo
}

func NewChatModel(ctx context.Context, provider *OpenAIProvider, modelName string) (model.ToolCallingChatModel, error) {
	return &ChatModelAdapter{
		provider: provider,
		model:    modelName,
	}, nil
}

func (m *ChatModelAdapter) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return &ChatModelAdapter{
		provider: m.provider,
		model:    m.model,
		tools:    tools,
	}, nil
}

func (m *ChatModelAdapter) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	msgs := m.convertMessages(input)

	req := &ChatRequest{
		Model:    m.model,
		Messages: msgs,
	}

	if len(m.tools) > 0 {
		req.Tools = m.convertTools(m.tools)
	}

	resp, err := m.provider.Chat(ctx, req)
	if err != nil {
		return nil, err
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	choice := resp.Choices[0]
	resMsg := &schema.Message{
		Role:    schema.RoleType(choice.Message.Role),
		Content: choice.Message.Content,
	}

	if len(choice.Message.ToolCalls) > 0 {
		tcs := make([]schema.ToolCall, len(choice.Message.ToolCalls))
		for i, tc := range choice.Message.ToolCalls {
			tcs[i] = schema.ToolCall{
				ID:   tc.ID,
				Type: tc.Type,
				Function: schema.FunctionCall{
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				},
			}
		}
		resMsg.ToolCalls = tcs
	}

	if choice.FinishReason != "" {
		resMsg.ResponseMeta = &schema.ResponseMeta{
			FinishReason: choice.FinishReason,
		}
	}

	// Fill usage if available
	if resp.Usage != nil {
		// Create a new ResponseMeta if it doesn't exist
		if resMsg.ResponseMeta == nil {
			resMsg.ResponseMeta = &schema.ResponseMeta{}
		}
		// Set FinishReason again to be safe
		resMsg.ResponseMeta.FinishReason = choice.FinishReason

		resMsg.ResponseMeta.Usage = &schema.TokenUsage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		}
	}

	return resMsg, nil
}

func (m *ChatModelAdapter) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	msgs := m.convertMessages(input)

	req := &ChatRequest{
		Model:    m.model,
		Messages: msgs,
	}

	if len(m.tools) > 0 {
		req.Tools = m.convertTools(m.tools)
	}

	streamCh, err := m.provider.ChatStream(ctx, req)
	if err != nil {
		return nil, err
	}

	reader, writer := schema.Pipe[*schema.Message](10)

	go func() {
		defer writer.Close()

		for event := range streamCh {
			if event.Error != nil {
				writer.Send(nil, event.Error)
				return
			}
			if event.Done {
				return
			}

			if len(event.Choices) > 0 {
				choice := event.Choices[0]

				// Handle delta
				msg := &schema.Message{}

				if choice.Delta != nil {
					msg.Role = schema.RoleType(choice.Delta.Role)
					msg.Content = choice.Delta.Content

					if len(choice.Delta.ToolCalls) > 0 {
						tcs := make([]schema.ToolCall, len(choice.Delta.ToolCalls))
						for i, tc := range choice.Delta.ToolCalls {
							tcs[i] = schema.ToolCall{
								ID:   tc.ID,
								Type: tc.Type,
								Function: schema.FunctionCall{
									Name:      tc.Function.Name,
									Arguments: tc.Function.Arguments,
								},
							}
						}
						msg.ToolCalls = tcs
					}
				}

				if choice.FinishReason != "" {
					msg.ResponseMeta = &schema.ResponseMeta{
						FinishReason: choice.FinishReason,
					}
				}

				writer.Send(msg, nil)
			}
		}
	}()

	return reader, nil
}

func (m *ChatModelAdapter) BindTools(tools []*schema.ToolInfo) error {
	m.tools = tools
	return nil
}

func (m *ChatModelAdapter) convertMessages(input []*schema.Message) []Message {
	msgs := make([]Message, len(input))
	for i, msg := range input {
		msgs[i] = Message{
			Role:       string(msg.Role),
			Content:    msg.Content,
			ToolCallID: msg.ToolCallID,
		}
		if len(msg.ToolCalls) > 0 {
			tcs := make([]ToolCall, len(msg.ToolCalls))
			for j, tc := range msg.ToolCalls {
				tcs[j] = ToolCall{
					ID:   tc.ID,
					Type: tc.Type,
					Function: FuncCall{
						Name:      tc.Function.Name,
						Arguments: tc.Function.Arguments,
					},
				}
			}
			msgs[i].ToolCalls = tcs
		}
	}
	return msgs
}

func (m *ChatModelAdapter) convertTools(tools []*schema.ToolInfo) []Tool {
	reqTools := make([]Tool, len(tools))
	for i, t := range tools {
		js, _ := t.ToJSONSchema()
		reqTools[i] = Tool{
			Type: "function",
			Function: Function{
				Name:        t.Name,
				Description: t.Desc,
				Parameters:  js,
			},
		}
	}
	return reqTools
}
