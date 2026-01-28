// Package llm provides LLM provider adapters and model routing for the MySQL Expert Agent.
package llm

import (
	"context"
	"time"
)

// Provider defines the interface for LLM providers.
type Provider interface {
	// Name returns the provider name.
	Name() string
	
	// Chat sends a chat completion request.
	Chat(ctx context.Context, request *ChatRequest) (*ChatResponse, error)
	
	// ChatStream sends a streaming chat completion request.
	ChatStream(ctx context.Context, request *ChatRequest) (<-chan *ChatStreamEvent, error)
	
	// ListModels returns available models.
	ListModels(ctx context.Context) ([]string, error)
	
	// Close closes the provider connection.
	Close() error
}

// ChatRequest represents a chat completion request.
type ChatRequest struct {
	Model       string         `json:"model"`
	Messages    []Message      `json:"messages"`
	MaxTokens   int            `json:"max_tokens,omitempty"`
	Temperature float64        `json:"temperature,omitempty"`
	TopP        float64        `json:"top_p,omitempty"`
	Stop        []string       `json:"stop,omitempty"`
	Tools       []Tool         `json:"tools,omitempty"`
	ToolChoice  interface{}    `json:"tool_choice,omitempty"`
	Stream      bool           `json:"stream,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// Message represents a chat message.
type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	Name       string     `json:"name,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

// Tool represents a tool definition.
type Tool struct {
	Type     string   `json:"type"`
	Function Function `json:"function"`
}

// Function represents a function definition.
type Function struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Parameters  interface{} `json:"parameters"`
}

// ToolCall represents a tool call from the model.
type ToolCall struct {
	ID       string   `json:"id"`
	Type     string   `json:"type"`
	Function FuncCall `json:"function"`
}

// FuncCall represents a function call.
type FuncCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ChatResponse represents a chat completion response.
type ChatResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   *Usage   `json:"usage,omitempty"`
}

// Choice represents a response choice.
type Choice struct {
	Index        int      `json:"index"`
	Message      *Message `json:"message,omitempty"`
	FinishReason string   `json:"finish_reason,omitempty"`
}

// Usage represents token usage.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ChatStreamEvent represents a streaming event.
type ChatStreamEvent struct {
	ID      string        `json:"id"`
	Object  string        `json:"object"`
	Created int64         `json:"created"`
	Model   string        `json:"model"`
	Choices []StreamChoice `json:"choices"`
	Error   error         `json:"-"`
	Done    bool          `json:"-"`
	Usage   *Usage        `json:"usage,omitempty"`
}

// StreamChoice represents a streaming choice.
type StreamChoice struct {
	Index        int     `json:"index"`
	Delta        *Delta  `json:"delta,omitempty"`
	FinishReason string  `json:"finish_reason,omitempty"`
}

// Delta represents content delta in streaming.
type Delta struct {
	Role      string     `json:"role,omitempty"`
	Content   string     `json:"content,omitempty"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

// ProviderConfig contains common provider configuration.
type ProviderConfig struct {
	Name       string
	BaseURL    string
	APIKey     string
	APIKeyEnv  string
	Timeout    time.Duration
	MaxRetries int
	RetryDelay time.Duration
	Headers    map[string]string
	HTTPProxy  string
}

// Model represents a model configuration.
type Model struct {
	Name             string
	Provider         string
	MaxContextLength int
	MaxOutputLength  int
	InputCostPer1K   float64
	OutputCostPer1K  float64
	Capabilities     []string
	Temperature      float64
	TopP             float64
}

// ProviderError represents a provider error.
type ProviderError struct {
	Provider   string
	StatusCode int
	Message    string
	Retryable  bool
}

func (e *ProviderError) Error() string {
	return e.Message
}

// TokenCounter provides token counting functionality.
type TokenCounter interface {
	Count(text string) int
	CountMessages(messages []Message) int
}

// ModelRouter routes requests to appropriate models.
type ModelRouter interface {
	// Route selects the best model for a request.
	Route(ctx context.Context, request *ChatRequest, intent string) (Provider, string, error)
	
	// RegisterProvider registers a provider.
	RegisterProvider(provider Provider)
	
	// RegisterModel registers a model.
	RegisterModel(model *Model)
}
