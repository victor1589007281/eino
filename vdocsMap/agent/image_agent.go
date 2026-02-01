/*
 * Copyright 2025 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package agent

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"

	"github.com/cloudwego/eino/vdocsMap/config"
	"github.com/cloudwego/eino/vdocsMap/tools"
)

// ImageGenAgent 文生图 Agent
type ImageGenAgent struct {
	agent  adk.Agent
	config *config.Config
	tools  *tools.ToolSet
}

// NewImageGenAgent 创建新的文生图 Agent
func NewImageGenAgent(ctx context.Context, cfg *config.Config, chatModel model.ToolCallingChatModel) (*ImageGenAgent, error) {
	// 初始化工具集
	toolSet, err := tools.NewToolSet(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create tool set: %w", err)
	}

	// 创建工具配置
	toolsConfig := &compose.ToolsNodeConfig{
		Tools: toolSet.GetTools(),
	}

	// 配置 Agent
	agentConfig := &adk.ChatModelAgentConfig{
		Model:         chatModel,
		SystemPrompt:  cfg.Agent.SystemPrompt,
		ToolsConfig:   toolsConfig,
		MaxIterations: cfg.Agent.MaxIterations,
	}

	// 创建 ChatModel Agent
	agent, err := adk.NewChatModelAgent(ctx, agentConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create chat model agent: %w", err)
	}

	return &ImageGenAgent{
		agent:  agent,
		config: cfg,
		tools:  toolSet,
	}, nil
}

// Run 运行 Agent 处理用户请求
func (a *ImageGenAgent) Run(ctx context.Context, userMessage string) (*AgentResponse, error) {
	// 构建输入消息
	input := &adk.AgentInput{
		Messages: []*schema.Message{
			schema.UserMessage(userMessage),
		},
		EnableStreaming: false,
	}

	// 运行 Agent
	iterator := a.agent.Run(ctx, input)

	// 收集响应
	response := &AgentResponse{
		Messages:   make([]*schema.Message, 0),
		ToolCalls:  make([]*ToolCallResult, 0),
		ImagePaths: make([]string, 0),
	}

	for {
		event, ok := iterator.Next()
		if !ok {
			break
		}

		if event.Err != nil {
			return nil, event.Err
		}

		// 处理输出
		if event.Output != nil {
			if msg := event.Output.MessageOutput; msg != nil {
				response.Messages = append(response.Messages, msg)
				// 提取最终文本响应
				if msg.Role == schema.Assistant && msg.Content != "" {
					response.FinalResponse = msg.Content
				}
			}

			// 收集工具调用结果
			if event.Output.ToolOutput != nil {
				response.ToolCalls = append(response.ToolCalls, &ToolCallResult{
					ToolName: event.Output.ToolOutput.ToolName,
					CallID:   event.Output.ToolOutput.CallID,
					Result:   event.Output.ToolOutput.Result,
				})
				
				// 检查是否是图片生成结果
				if imagePath := extractImagePath(event.Output.ToolOutput.Result); imagePath != "" {
					response.ImagePaths = append(response.ImagePaths, imagePath)
				}
			}
		}

		// 处理动作
		if event.Action != nil && event.Action.Exit {
			break
		}
	}

	return response, nil
}

// RunStream 流式运行 Agent
func (a *ImageGenAgent) RunStream(ctx context.Context, userMessage string) <-chan *AgentEvent {
	eventChan := make(chan *AgentEvent, 100)

	go func() {
		defer close(eventChan)

		input := &adk.AgentInput{
			Messages: []*schema.Message{
				schema.UserMessage(userMessage),
			},
			EnableStreaming: true,
		}

		iterator := a.agent.Run(ctx, input)

		for {
			event, ok := iterator.Next()
			if !ok {
				break
			}

			agentEvent := &AgentEvent{}

			if event.Err != nil {
				agentEvent.Error = event.Err
				eventChan <- agentEvent
				break
			}

			if event.Output != nil {
				if msg := event.Output.MessageOutput; msg != nil {
					agentEvent.Message = msg
					agentEvent.Content = msg.Content
				}
				if event.Output.ToolOutput != nil {
					agentEvent.ToolCall = &ToolCallResult{
						ToolName: event.Output.ToolOutput.ToolName,
						CallID:   event.Output.ToolOutput.CallID,
						Result:   event.Output.ToolOutput.Result,
					}
				}
			}

			eventChan <- agentEvent

			if event.Action != nil && event.Action.Exit {
				break
			}
		}
	}()

	return eventChan
}

// Chat 简化的对话接口
func (a *ImageGenAgent) Chat(ctx context.Context, message string) (string, error) {
	response, err := a.Run(ctx, message)
	if err != nil {
		return "", err
	}
	return response.FinalResponse, nil
}

// GenerateImage 直接生成图片
func (a *ImageGenAgent) GenerateImage(ctx context.Context, prompt string, opts ...GenerateOption) (*ImageResult, error) {
	options := &GenerateOptions{
		Type:   ImageTypeStatic,
		Size:   a.config.Image.StaticImage.DefaultSize,
		Format: a.config.Image.Output.Format,
	}
	for _, opt := range opts {
		opt(options)
	}

	// 构建生成请求消息
	message := buildGenerateMessage(prompt, options)
	
	response, err := a.Run(ctx, message)
	if err != nil {
		return nil, err
	}

	if len(response.ImagePaths) == 0 {
		return nil, fmt.Errorf("no image generated")
	}

	return &ImageResult{
		Path:     response.ImagePaths[0],
		Prompt:   prompt,
		Type:     options.Type,
		Size:     options.Size,
		Format:   options.Format,
		Metadata: extractMetadata(response),
	}, nil
}

// Name 返回 Agent 名称
func (a *ImageGenAgent) Name(ctx context.Context) string {
	return a.config.Agent.Name
}

// Close 关闭 Agent
func (a *ImageGenAgent) Close() error {
	return a.tools.Close()
}

// AgentResponse Agent 响应结构
type AgentResponse struct {
	Messages      []*schema.Message `json:"messages"`
	ToolCalls     []*ToolCallResult `json:"tool_calls"`
	ImagePaths    []string          `json:"image_paths"`
	FinalResponse string            `json:"final_response"`
}

// AgentEvent 流式事件
type AgentEvent struct {
	Message  *schema.Message `json:"message,omitempty"`
	Content  string          `json:"content,omitempty"`
	ToolCall *ToolCallResult `json:"tool_call,omitempty"`
	Error    error           `json:"error,omitempty"`
}

// ToolCallResult 工具调用结果
type ToolCallResult struct {
	ToolName string `json:"tool_name"`
	CallID   string `json:"call_id"`
	Result   string `json:"result"`
}

// ImageResult 图片生成结果
type ImageResult struct {
	Path     string            `json:"path"`
	Prompt   string            `json:"prompt"`
	Type     ImageType         `json:"type"`
	Size     string            `json:"size"`
	Format   string            `json:"format"`
	Metadata map[string]any    `json:"metadata,omitempty"`
}

// ImageType 图片类型
type ImageType string

const (
	ImageTypeStatic  ImageType = "static"
	ImageTypeGIF     ImageType = "gif"
	ImageTypeSticker ImageType = "sticker"
)

// GenerateOptions 生成选项
type GenerateOptions struct {
	Type      ImageType
	Size      string
	Format    string
	Quality   string
	Text      string // 表情包文字
	FPS       int    // 动图帧率
	Duration  int    // 动图时长
}

// GenerateOption 生成选项函数
type GenerateOption func(*GenerateOptions)

// WithImageType 设置图片类型
func WithImageType(t ImageType) GenerateOption {
	return func(o *GenerateOptions) {
		o.Type = t
	}
}

// WithSize 设置图片尺寸
func WithSize(size string) GenerateOption {
	return func(o *GenerateOptions) {
		o.Size = size
	}
}

// WithFormat 设置输出格式
func WithFormat(format string) GenerateOption {
	return func(o *GenerateOptions) {
		o.Format = format
	}
}

// WithQuality 设置图片质量
func WithQuality(quality string) GenerateOption {
	return func(o *GenerateOptions) {
		o.Quality = quality
	}
}

// WithText 设置表情包文字
func WithText(text string) GenerateOption {
	return func(o *GenerateOptions) {
		o.Text = text
	}
}

// WithFPS 设置动图帧率
func WithFPS(fps int) GenerateOption {
	return func(o *GenerateOptions) {
		o.FPS = fps
	}
}

// WithDuration 设置动图时长
func WithDuration(duration int) GenerateOption {
	return func(o *GenerateOptions) {
		o.Duration = duration
	}
}

// extractImagePath 从工具结果中提取图片路径
func extractImagePath(result string) string {
	// 简单实现，实际需要解析 JSON 结果
	// TODO: 实现更完善的解析逻辑
	return ""
}

// buildGenerateMessage 构建生成请求消息
func buildGenerateMessage(prompt string, opts *GenerateOptions) string {
	switch opts.Type {
	case ImageTypeGIF:
		return fmt.Sprintf("请生成一个动图，描述：%s，尺寸：%s", prompt, opts.Size)
	case ImageTypeSticker:
		if opts.Text != "" {
			return fmt.Sprintf("请生成一个表情包，描述：%s，文字：%s，尺寸：%s", prompt, opts.Text, opts.Size)
		}
		return fmt.Sprintf("请生成一个表情包，描述：%s，尺寸：%s", prompt, opts.Size)
	default:
		return fmt.Sprintf("请生成一张图片，描述：%s，尺寸：%s，质量：%s", prompt, opts.Size, opts.Quality)
	}
}

// extractMetadata 从响应中提取元数据
func extractMetadata(response *AgentResponse) map[string]any {
	metadata := make(map[string]any)
	metadata["tool_calls_count"] = len(response.ToolCalls)
	metadata["messages_count"] = len(response.Messages)
	return metadata
}
