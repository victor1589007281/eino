package llm

import (
	"bufio"
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// WenxinProvider implements the Provider interface for Baidu Wenxin (文心一言).
type WenxinProvider struct {
	config      *WenxinConfig
	httpClient  *http.Client
	accessToken string
	tokenExpiry time.Time
}

// WenxinConfig contains Wenxin-specific configuration.
type WenxinConfig struct {
	APIKey       string
	SecretKey    string
	APIKeyEnv    string
	SecretKeyEnv string
	Timeout      time.Duration
}

// NewWenxinProvider creates a new Wenxin provider.
func NewWenxinProvider(config *WenxinConfig) (*WenxinProvider, error) {
	apiKey := config.APIKey
	if apiKey == "" && config.APIKeyEnv != "" {
		apiKey = os.Getenv(config.APIKeyEnv)
	}
	if apiKey == "" {
		return nil, fmt.Errorf("Wenxin API key not provided")
	}
	config.APIKey = apiKey

	secretKey := config.SecretKey
	if secretKey == "" && config.SecretKeyEnv != "" {
		secretKey = os.Getenv(config.SecretKeyEnv)
	}
	if secretKey == "" {
		return nil, fmt.Errorf("Wenxin Secret key not provided")
	}
	config.SecretKey = secretKey

	timeout := config.Timeout
	if timeout == 0 {
		timeout = 120 * time.Second
	}

	return &WenxinProvider{
		config:     config,
		httpClient: &http.Client{Timeout: timeout},
	}, nil
}

// Name returns the provider name.
func (p *WenxinProvider) Name() string {
	return "wenxin"
}

func (p *WenxinProvider) getAccessToken(ctx context.Context) (string, error) {
	if p.accessToken != "" && time.Now().Before(p.tokenExpiry) {
		return p.accessToken, nil
	}

	tokenURL := fmt.Sprintf("https://aip.baidubce.com/oauth/2.0/token?grant_type=client_credentials&client_id=%s&client_secret=%s",
		p.config.APIKey, p.config.SecretKey)

	req, err := http.NewRequestWithContext(ctx, "POST", tokenURL, nil)
	if err != nil {
		return "", err
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	p.accessToken = result.AccessToken
	p.tokenExpiry = time.Now().Add(time.Duration(result.ExpiresIn-60) * time.Second)

	return p.accessToken, nil
}

// Chat sends a chat completion request.
func (p *WenxinProvider) Chat(ctx context.Context, request *ChatRequest) (*ChatResponse, error) {
	accessToken, err := p.getAccessToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get access token: %w", err)
	}

	// Map model to endpoint
	modelEndpoints := map[string]string{
		"ernie-bot-4":        "completions_pro",
		"ernie-bot":          "completions",
		"ernie-bot-turbo":    "eb-instant",
		"ernie-speed":        "ernie_speed",
		"ernie-lite":         "ernie-lite-8k",
	}

	endpoint := modelEndpoints[request.Model]
	if endpoint == "" {
		endpoint = "completions_pro"
	}

	apiURL := fmt.Sprintf("https://aip.baidubce.com/rpc/2.0/ai_custom/v1/wenxinworkshop/chat/%s?access_token=%s",
		endpoint, accessToken)

	// Convert messages to Wenxin format
	wenxinReq := map[string]interface{}{
		"messages": p.convertMessages(request.Messages),
	}
	if request.Temperature > 0 {
		wenxinReq["temperature"] = request.Temperature
	}
	if request.TopP > 0 {
		wenxinReq["top_p"] = request.TopP
	}

	reqBody, err := json.Marshal(wenxinReq)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		ID           string `json:"id"`
		Result       string `json:"result"`
		ErrorCode    int    `json:"error_code"`
		ErrorMsg     string `json:"error_msg"`
		Usage        struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if result.ErrorCode != 0 {
		return nil, &ProviderError{
			Provider: "wenxin",
			Message:  result.ErrorMsg,
		}
	}

	return &ChatResponse{
		ID:    result.ID,
		Model: request.Model,
		Choices: []Choice{
			{
				Index: 0,
				Message: &Message{
					Role:    "assistant",
					Content: result.Result,
				},
				FinishReason: "stop",
			},
		},
		Usage: &Usage{
			PromptTokens:     result.Usage.PromptTokens,
			CompletionTokens: result.Usage.CompletionTokens,
			TotalTokens:      result.Usage.TotalTokens,
		},
	}, nil
}

func (p *WenxinProvider) convertMessages(messages []Message) []map[string]string {
	result := make([]map[string]string, 0, len(messages))
	for _, msg := range messages {
		if msg.Role == "system" {
			// Wenxin doesn't support system messages directly, prepend to first user message
			continue
		}
		result = append(result, map[string]string{
			"role":    msg.Role,
			"content": msg.Content,
		})
	}
	return result
}

// ChatStream sends a streaming chat completion request.
func (p *WenxinProvider) ChatStream(ctx context.Context, request *ChatRequest) (<-chan *ChatStreamEvent, error) {
	accessToken, err := p.getAccessToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get access token: %w", err)
	}

	modelEndpoints := map[string]string{
		"ernie-bot-4":     "completions_pro",
		"ernie-bot":       "completions",
		"ernie-bot-turbo": "eb-instant",
	}

	endpoint := modelEndpoints[request.Model]
	if endpoint == "" {
		endpoint = "completions_pro"
	}

	apiURL := fmt.Sprintf("https://aip.baidubce.com/rpc/2.0/ai_custom/v1/wenxinworkshop/chat/%s?access_token=%s",
		endpoint, accessToken)

	wenxinReq := map[string]interface{}{
		"messages": p.convertMessages(request.Messages),
		"stream":   true,
	}

	reqBody, err := json.Marshal(wenxinReq)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	eventCh := make(chan *ChatStreamEvent, 100)

	go func() {
		defer close(eventCh)
		defer resp.Body.Close()

		reader := bufio.NewReader(resp.Body)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				if err != io.EOF {
					eventCh <- &ChatStreamEvent{Error: err, Done: true}
				}
				eventCh <- &ChatStreamEvent{Done: true}
				return
			}

			line = strings.TrimSpace(line)
			if line == "" || !strings.HasPrefix(line, "data: ") {
				continue
			}

			data := strings.TrimPrefix(line, "data: ")

			var result struct {
				Result   string `json:"result"`
				IsEnd    bool   `json:"is_end"`
			}

			if err := json.Unmarshal([]byte(data), &result); err != nil {
				continue
			}

			eventCh <- &ChatStreamEvent{
				Model: request.Model,
				Choices: []StreamChoice{
					{
						Index: 0,
						Delta: &Delta{
							Content: result.Result,
						},
					},
				},
				Done: result.IsEnd,
			}

			if result.IsEnd {
				return
			}
		}
	}()

	return eventCh, nil
}

// ListModels returns available models.
func (p *WenxinProvider) ListModels(ctx context.Context) ([]string, error) {
	return []string{"ernie-bot-4", "ernie-bot", "ernie-bot-turbo", "ernie-speed", "ernie-lite"}, nil
}

// Close closes the provider.
func (p *WenxinProvider) Close() error {
	return nil
}

// SparkProvider implements the Provider interface for iFlytek Spark (讯飞星火).
type SparkProvider struct {
	config     *SparkConfig
	httpClient *http.Client
}

// SparkConfig contains Spark-specific configuration.
type SparkConfig struct {
	AppID       string
	APIKey      string
	APISecret   string
	AppIDEnv    string
	APIKeyEnv   string
	APISecretEnv string
	Timeout     time.Duration
}

// NewSparkProvider creates a new Spark provider.
func NewSparkProvider(config *SparkConfig) (*SparkProvider, error) {
	appID := config.AppID
	if appID == "" && config.AppIDEnv != "" {
		appID = os.Getenv(config.AppIDEnv)
	}
	if appID == "" {
		return nil, fmt.Errorf("Spark App ID not provided")
	}
	config.AppID = appID

	apiKey := config.APIKey
	if apiKey == "" && config.APIKeyEnv != "" {
		apiKey = os.Getenv(config.APIKeyEnv)
	}
	if apiKey == "" {
		return nil, fmt.Errorf("Spark API key not provided")
	}
	config.APIKey = apiKey

	apiSecret := config.APISecret
	if apiSecret == "" && config.APISecretEnv != "" {
		apiSecret = os.Getenv(config.APISecretEnv)
	}
	if apiSecret == "" {
		return nil, fmt.Errorf("Spark API secret not provided")
	}
	config.APISecret = apiSecret

	timeout := config.Timeout
	if timeout == 0 {
		timeout = 120 * time.Second
	}

	return &SparkProvider{
		config:     config,
		httpClient: &http.Client{Timeout: timeout},
	}, nil
}

// Name returns the provider name.
func (p *SparkProvider) Name() string {
	return "spark"
}

func (p *SparkProvider) generateSignature(host, path, method string) (string, error) {
	date := time.Now().UTC().Format(time.RFC1123)
	
	signString := fmt.Sprintf("host: %s\ndate: %s\n%s %s HTTP/1.1", host, date, method, path)
	
	mac := hmac.New(sha256.New, []byte(p.config.APISecret))
	mac.Write([]byte(signString))
	signature := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	
	authString := fmt.Sprintf(`api_key="%s", algorithm="hmac-sha256", headers="host date request-line", signature="%s"`,
		p.config.APIKey, signature)
	authorization := base64.StdEncoding.EncodeToString([]byte(authString))
	
	return fmt.Sprintf("authorization=%s&date=%s&host=%s",
		url.QueryEscape(authorization),
		url.QueryEscape(date),
		url.QueryEscape(host)), nil
}

// Chat sends a chat completion request (Spark uses WebSocket, this is a simplified HTTP version).
func (p *SparkProvider) Chat(ctx context.Context, request *ChatRequest) (*ChatResponse, error) {
	// Note: Full Spark implementation requires WebSocket
	// This is a simplified placeholder using their HTTP API if available
	
	return nil, fmt.Errorf("Spark Chat requires WebSocket implementation")
}

// ChatStream sends a streaming chat completion request.
func (p *SparkProvider) ChatStream(ctx context.Context, request *ChatRequest) (<-chan *ChatStreamEvent, error) {
	// Note: Full Spark implementation requires WebSocket
	return nil, fmt.Errorf("Spark ChatStream requires WebSocket implementation")
}

// ListModels returns available models.
func (p *SparkProvider) ListModels(ctx context.Context) ([]string, error) {
	return []string{"spark-v3.5", "spark-v3.0", "spark-v2.0", "spark-v1.5"}, nil
}

// Close closes the provider.
func (p *SparkProvider) Close() error {
	return nil
}

// QwenProvider implements the Provider interface for Alibaba Qwen (通义千问).
type QwenProvider struct {
	config     *QwenConfig
	httpClient *http.Client
}

// QwenConfig contains Qwen-specific configuration.
type QwenConfig struct {
	APIKey    string
	APIKeyEnv string
	BaseURL   string
	Timeout   time.Duration
}

// NewQwenProvider creates a new Qwen provider.
func NewQwenProvider(config *QwenConfig) (*QwenProvider, error) {
	apiKey := config.APIKey
	if apiKey == "" && config.APIKeyEnv != "" {
		apiKey = os.Getenv(config.APIKeyEnv)
	}
	if apiKey == "" {
		return nil, fmt.Errorf("Qwen API key not provided")
	}
	config.APIKey = apiKey

	if config.BaseURL == "" {
		config.BaseURL = "https://dashscope.aliyuncs.com/api/v1"
	}

	timeout := config.Timeout
	if timeout == 0 {
		timeout = 120 * time.Second
	}

	return &QwenProvider{
		config:     config,
		httpClient: &http.Client{Timeout: timeout},
	}, nil
}

// Name returns the provider name.
func (p *QwenProvider) Name() string {
	return "qwen"
}

// Chat sends a chat completion request.
func (p *QwenProvider) Chat(ctx context.Context, request *ChatRequest) (*ChatResponse, error) {
	apiURL := p.config.BaseURL + "/services/aigc/text-generation/generation"

	qwenReq := map[string]interface{}{
		"model": request.Model,
		"input": map[string]interface{}{
			"messages": request.Messages,
		},
		"parameters": map[string]interface{}{},
	}

	if request.Temperature > 0 {
		qwenReq["parameters"].(map[string]interface{})["temperature"] = request.Temperature
	}
	if request.MaxTokens > 0 {
		qwenReq["parameters"].(map[string]interface{})["max_tokens"] = request.MaxTokens
	}

	reqBody, err := json.Marshal(qwenReq)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.config.APIKey)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Output struct {
			Text         string `json:"text"`
			FinishReason string `json:"finish_reason"`
		} `json:"output"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
		RequestID string `json:"request_id"`
		Code      string `json:"code"`
		Message   string `json:"message"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if result.Code != "" {
		return nil, &ProviderError{
			Provider: "qwen",
			Message:  result.Message,
		}
	}

	return &ChatResponse{
		ID:    result.RequestID,
		Model: request.Model,
		Choices: []Choice{
			{
				Index: 0,
				Message: &Message{
					Role:    "assistant",
					Content: result.Output.Text,
				},
				FinishReason: result.Output.FinishReason,
			},
		},
		Usage: &Usage{
			PromptTokens:     result.Usage.InputTokens,
			CompletionTokens: result.Usage.OutputTokens,
			TotalTokens:      result.Usage.InputTokens + result.Usage.OutputTokens,
		},
	}, nil
}

// ChatStream sends a streaming chat completion request.
func (p *QwenProvider) ChatStream(ctx context.Context, request *ChatRequest) (<-chan *ChatStreamEvent, error) {
	apiURL := p.config.BaseURL + "/services/aigc/text-generation/generation"

	qwenReq := map[string]interface{}{
		"model": request.Model,
		"input": map[string]interface{}{
			"messages": request.Messages,
		},
		"parameters": map[string]interface{}{
			"incremental_output": true,
		},
	}

	reqBody, err := json.Marshal(qwenReq)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.config.APIKey)
	req.Header.Set("X-DashScope-SSE", "enable")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	eventCh := make(chan *ChatStreamEvent, 100)

	go func() {
		defer close(eventCh)
		defer resp.Body.Close()

		reader := bufio.NewReader(resp.Body)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				if err != io.EOF {
					eventCh <- &ChatStreamEvent{Error: err, Done: true}
				}
				eventCh <- &ChatStreamEvent{Done: true}
				return
			}

			line = strings.TrimSpace(line)
			if line == "" || !strings.HasPrefix(line, "data:") {
				continue
			}

			data := strings.TrimPrefix(line, "data:")

			var result struct {
				Output struct {
					Text         string `json:"text"`
					FinishReason string `json:"finish_reason"`
				} `json:"output"`
			}

			if err := json.Unmarshal([]byte(data), &result); err != nil {
				continue
			}

			done := result.Output.FinishReason != ""
			eventCh <- &ChatStreamEvent{
				Model: request.Model,
				Choices: []StreamChoice{
					{
						Index: 0,
						Delta: &Delta{
							Content: result.Output.Text,
						},
						FinishReason: result.Output.FinishReason,
					},
				},
				Done: done,
			}

			if done {
				return
			}
		}
	}()

	return eventCh, nil
}

// ListModels returns available models.
func (p *QwenProvider) ListModels(ctx context.Context) ([]string, error) {
	return []string{"qwen-turbo", "qwen-plus", "qwen-max", "qwen-max-longcontext"}, nil
}

// Close closes the provider.
func (p *QwenProvider) Close() error {
	return nil
}

// DeepSeekProvider implements the Provider interface for DeepSeek.
type DeepSeekProvider struct {
	*OpenAIProvider
}

// NewDeepSeekProvider creates a new DeepSeek provider.
func NewDeepSeekProvider(config *OpenAIConfig) (*DeepSeekProvider, error) {
	if config.BaseURL == "" {
		config.BaseURL = "https://api.deepseek.com/v1"
	}
	
	provider, err := NewOpenAIProvider(config)
	if err != nil {
		return nil, err
	}
	
	return &DeepSeekProvider{OpenAIProvider: provider}, nil
}

// Name returns the provider name.
func (p *DeepSeekProvider) Name() string {
	return "deepseek"
}

// ListModels returns available models.
func (p *DeepSeekProvider) ListModels(ctx context.Context) ([]string, error) {
	return []string{"deepseek-chat", "deepseek-coder"}, nil
}
