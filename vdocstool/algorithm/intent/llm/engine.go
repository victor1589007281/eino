// Package llm LLM引擎实现
package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/cloudwego/eino/vdocstool/algorithm/types"
)

// Engine LLM引擎
type Engine struct {
	config   *types.LLMEngineConfig
	client   Client
	prompts  *PromptTemplates
}

// Client LLM客户端接口
type Client interface {
	// Complete 完成请求
	Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error)
	// HealthCheck 健康检查
	HealthCheck(ctx context.Context) error
	// Close 关闭
	Close() error
}

// CompletionRequest 完成请求
type CompletionRequest struct {
	Prompt      string   `json:"prompt"`
	Messages    []ChatMessage `json:"messages,omitempty"`
	MaxTokens   int      `json:"max_tokens"`
	Temperature float64  `json:"temperature"`
	Stop        []string `json:"stop,omitempty"`
}

// ChatMessage 聊天消息
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// CompletionResponse 完成响应
type CompletionResponse struct {
	Text       string `json:"text"`
	FinishReason string `json:"finish_reason"`
	Usage      *Usage `json:"usage,omitempty"`
}

// Usage 使用量
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// NewEngine 创建LLM引擎
func NewEngine(cfg *types.LLMEngineConfig) (*Engine, error) {
	e := &Engine{
		config:  cfg,
		prompts: NewPromptTemplates(),
	}

	// 根据 provider 创建客户端
	var err error
	switch cfg.Provider {
	// 国外大模型
	case types.LLMProviderOpenAI:
		e.client, err = NewOpenAIClient(cfg)
	case types.LLMProviderClaude:
		e.client, err = NewClaudeClient(cfg)
	case types.LLMProviderGemini:
		e.client, err = NewGeminiClient(cfg)
	case types.LLMProviderCohere:
		e.client, err = NewCohereClient(cfg)

	// 国内大模型
	case types.LLMProviderQwen, "tongyi", "dashscope":
		e.client, err = NewQwenClient(cfg)
	case types.LLMProviderWenxin, "ernie", "baidu":
		e.client, err = NewWenxinClient(cfg)
	case types.LLMProviderGLM, "chatglm", "zhipu":
		e.client, err = NewGLMClient(cfg)
	case types.LLMProviderSpark, "xunfei", "iflytek":
		e.client, err = NewSparkClient(cfg)
	case types.LLMProviderDeepSeek:
		e.client, err = NewDeepSeekClient(cfg)
	case types.LLMProviderBaichuan:
		e.client, err = NewBaichuanClient(cfg)

	default:
		// 默认使用 OpenAI 兼容接口（支持各种 OpenAI 兼容的本地/代理服务）
		e.client, err = NewOpenAIClient(cfg)
	}

	if err != nil {
		return nil, fmt.Errorf("create client for provider %s: %w", cfg.Provider, err)
	}

	return e, nil
}

// SupportedProviders 返回支持的提供商列表
func SupportedProviders() []string {
	return []string{
		// 国外
		"openai", "claude", "gemini", "cohere",
		// 国内
		"qwen", "wenxin", "glm", "spark", "deepseek", "baichuan",
	}
}

// Recognize 识别意图
func (e *Engine) Recognize(ctx context.Context, input *types.IntentInput) (*types.IntentResult, error) {
	start := time.Now()

	// 构建 prompt
	prompt, err := e.prompts.BuildIntentPrompt(input)
	if err != nil {
		return nil, fmt.Errorf("build prompt: %w", err)
	}

	// 设置超时
	if e.config.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, e.config.Timeout)
		defer cancel()
	}

	// 调用 LLM
	resp, err := e.client.Complete(ctx, &CompletionRequest{
		Messages: []ChatMessage{
			{Role: "system", Content: e.prompts.SystemPrompt(input.Domain)},
			{Role: "user", Content: prompt},
		},
		MaxTokens:   e.config.MaxTokens,
		Temperature: e.config.Temperature,
	})
	if err != nil {
		return nil, fmt.Errorf("llm call: %w", err)
	}

	// 解析响应
	result, err := e.parseResponse(resp.Text, input)
	if err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	result.Source = "llm"
	result.Latency = time.Since(start)

	return result, nil
}

// parseResponse 解析LLM响应
func (e *Engine) parseResponse(text string, input *types.IntentInput) (*types.IntentResult, error) {
	// 尝试解析 JSON
	var parsed struct {
		Intent     string            `json:"intent"`
		Confidence float64           `json:"confidence"`
		Slots      map[string]string `json:"slots,omitempty"`
		Entities   []struct {
			Text string `json:"text"`
			Type string `json:"type"`
		} `json:"entities,omitempty"`
		Reasoning string `json:"reasoning,omitempty"`
	}

	// 尝试提取 JSON（LLM 可能在 JSON 前后有额外文本）
	jsonText := extractJSON(text)
	if jsonText == "" {
		// 无法提取 JSON，尝试简单解析
		return &types.IntentResult{
			Intent: &types.Intent{
				Name:       types.IntentUnknown,
				Confidence: 0.5,
			},
			Confidence: 0.5,
			Reasoning:  text,
		}, nil
	}

	if err := json.Unmarshal([]byte(jsonText), &parsed); err != nil {
		return &types.IntentResult{
			Intent: &types.Intent{
				Name:       types.IntentUnknown,
				Confidence: 0.5,
			},
			Confidence: 0.5,
			Reasoning:  text,
		}, nil
	}

	// 构建结果
	result := &types.IntentResult{
		Intent: &types.Intent{
			Name:       parsed.Intent,
			Confidence: parsed.Confidence,
			Slots:      parsed.Slots,
		},
		Confidence: parsed.Confidence,
		Reasoning:  parsed.Reasoning,
	}

	// 转换实体
	for _, ent := range parsed.Entities {
		result.Entities = append(result.Entities, &types.Entity{
			Text: ent.Text,
			Type: ent.Type,
		})
	}

	return result, nil
}

// extractJSON 从文本中提取 JSON
func extractJSON(text string) string {
	// 找到第一个 { 和最后一个 }
	start := -1
	end := -1
	depth := 0

	for i, c := range text {
		if c == '{' {
			if start == -1 {
				start = i
			}
			depth++
		} else if c == '}' {
			depth--
			if depth == 0 {
				end = i + 1
				break
			}
		}
	}

	if start >= 0 && end > start {
		return text[start:end]
	}
	return ""
}

// Name 引擎名称
func (e *Engine) Name() string {
	return "llm"
}

// Domains 支持的领域
func (e *Engine) Domains() []string {
	return []string{types.DomainEmail, types.DomainMemory, types.DomainSearch, types.DomainGeneral}
}

// HealthCheck 健康检查
func (e *Engine) HealthCheck(ctx context.Context) error {
	if e.client == nil {
		return fmt.Errorf("client not initialized")
	}
	return e.client.HealthCheck(ctx)
}

// Close 关闭引擎
func (e *Engine) Close() error {
	if e.client != nil {
		return e.client.Close()
	}
	return nil
}
