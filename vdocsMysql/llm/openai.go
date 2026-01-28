package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// OpenAIProvider implements the Provider interface for OpenAI.
type OpenAIProvider struct {
	config     *OpenAIConfig
	httpClient *http.Client
}

// OpenAIConfig contains OpenAI-specific configuration.
type OpenAIConfig struct {
	BaseURL      string
	APIKey       string
	APIKeyEnv    string
	Organization string
	Timeout      time.Duration
	MaxRetries   int
	RetryDelay   time.Duration
	HTTPProxy    string
}

// NewOpenAIProvider creates a new OpenAI provider.
func NewOpenAIProvider(config *OpenAIConfig) (*OpenAIProvider, error) {
	if config.BaseURL == "" {
		config.BaseURL = "https://api.openai.com/v1"
	}
	
	apiKey := config.APIKey
	if apiKey == "" && config.APIKeyEnv != "" {
		apiKey = os.Getenv(config.APIKeyEnv)
	}
	if apiKey == "" {
		return nil, fmt.Errorf("OpenAI API key not provided")
	}
	config.APIKey = apiKey

	timeout := config.Timeout
	if timeout == 0 {
		timeout = 120 * time.Second
	}

	httpClient := &http.Client{
		Timeout: timeout,
	}

	return &OpenAIProvider{
		config:     config,
		httpClient: httpClient,
	}, nil
}

// Name returns the provider name.
func (p *OpenAIProvider) Name() string {
	return "openai"
}

// Chat sends a chat completion request.
func (p *OpenAIProvider) Chat(ctx context.Context, request *ChatRequest) (*ChatResponse, error) {
	request.Stream = false
	
	reqBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.config.BaseURL+"/chat/completions", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	p.setHeaders(req)

	resp, err := p.doRequestWithRetry(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, &ProviderError{
			Provider:   "openai",
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("OpenAI API error: %s", string(body)),
			Retryable:  resp.StatusCode >= 500 || resp.StatusCode == 429,
		}
	}

	var chatResp ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &chatResp, nil
}

// ChatStream sends a streaming chat completion request.
func (p *OpenAIProvider) ChatStream(ctx context.Context, request *ChatRequest) (<-chan *ChatStreamEvent, error) {
	request.Stream = true
	
	reqBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.config.BaseURL+"/chat/completions", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	p.setHeaders(req)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, &ProviderError{
			Provider:   "openai",
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("OpenAI API error: %s", string(body)),
			Retryable:  resp.StatusCode >= 500 || resp.StatusCode == 429,
		}
	}

	eventCh := make(chan *ChatStreamEvent, 100)

	go func() {
		defer close(eventCh)
		defer resp.Body.Close()

		reader := bufio.NewReader(resp.Body)
		for {
			select {
			case <-ctx.Done():
				eventCh <- &ChatStreamEvent{Error: ctx.Err(), Done: true}
				return
			default:
			}

			line, err := reader.ReadString('\n')
			if err != nil {
				if err != io.EOF {
					eventCh <- &ChatStreamEvent{Error: err, Done: true}
				}
				eventCh <- &ChatStreamEvent{Done: true}
				return
			}

			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}

			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				eventCh <- &ChatStreamEvent{Done: true}
				return
			}

			var event ChatStreamEvent
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				continue
			}

			eventCh <- &event
		}
	}()

	return eventCh, nil
}

// ListModels returns available models.
func (p *OpenAIProvider) ListModels(ctx context.Context) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", p.config.BaseURL+"/models", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	p.setHeaders(req)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to list models: status %d", resp.StatusCode)
	}

	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	models := make([]string, len(result.Data))
	for i, m := range result.Data {
		models[i] = m.ID
	}

	return models, nil
}

// Close closes the provider.
func (p *OpenAIProvider) Close() error {
	return nil
}

func (p *OpenAIProvider) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.config.APIKey)
	if p.config.Organization != "" {
		req.Header.Set("OpenAI-Organization", p.config.Organization)
	}
}

func (p *OpenAIProvider) doRequestWithRetry(req *http.Request) (*http.Response, error) {
	maxRetries := p.config.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 3
	}

	retryDelay := p.config.RetryDelay
	if retryDelay == 0 {
		retryDelay = time.Second
	}

	var lastErr error
	for i := 0; i < maxRetries; i++ {
		// Clone request body for retry
		var bodyReader io.Reader
		if req.GetBody != nil {
			body, err := req.GetBody()
			if err == nil {
				bodyReader = body
			}
		}
		if bodyReader != nil {
			req.Body = io.NopCloser(bodyReader)
		}

		resp, err := p.httpClient.Do(req)
		if err != nil {
			lastErr = err
			time.Sleep(retryDelay * time.Duration(i+1))
			continue
		}

		// Check if should retry
		if resp.StatusCode == 429 || resp.StatusCode >= 500 {
			resp.Body.Close()
			lastErr = fmt.Errorf("status %d", resp.StatusCode)
			time.Sleep(retryDelay * time.Duration(i+1))
			continue
		}

		return resp, nil
	}

	return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
}

// AzureOpenAIProvider implements the Provider interface for Azure OpenAI.
type AzureOpenAIProvider struct {
	config     *AzureOpenAIConfig
	httpClient *http.Client
}

// AzureOpenAIConfig contains Azure OpenAI-specific configuration.
type AzureOpenAIConfig struct {
	Endpoint    string
	APIKey      string
	APIKeyEnv   string
	APIVersion  string
	DeploymentName string
	Timeout     time.Duration
	MaxRetries  int
}

// NewAzureOpenAIProvider creates a new Azure OpenAI provider.
func NewAzureOpenAIProvider(config *AzureOpenAIConfig) (*AzureOpenAIProvider, error) {
	apiKey := config.APIKey
	if apiKey == "" && config.APIKeyEnv != "" {
		apiKey = os.Getenv(config.APIKeyEnv)
	}
	if apiKey == "" {
		return nil, fmt.Errorf("Azure OpenAI API key not provided")
	}
	config.APIKey = apiKey

	if config.APIVersion == "" {
		config.APIVersion = "2024-02-15-preview"
	}

	timeout := config.Timeout
	if timeout == 0 {
		timeout = 120 * time.Second
	}

	return &AzureOpenAIProvider{
		config:     config,
		httpClient: &http.Client{Timeout: timeout},
	}, nil
}

// Name returns the provider name.
func (p *AzureOpenAIProvider) Name() string {
	return "azure_openai"
}

// Chat sends a chat completion request.
func (p *AzureOpenAIProvider) Chat(ctx context.Context, request *ChatRequest) (*ChatResponse, error) {
	request.Stream = false
	
	reqBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/openai/deployments/%s/chat/completions?api-version=%s",
		p.config.Endpoint, p.config.DeploymentName, p.config.APIVersion)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("api-key", p.config.APIKey)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, &ProviderError{
			Provider:   "azure_openai",
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("Azure OpenAI API error: %s", string(body)),
			Retryable:  resp.StatusCode >= 500 || resp.StatusCode == 429,
		}
	}

	var chatResp ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &chatResp, nil
}

// ChatStream sends a streaming chat completion request.
func (p *AzureOpenAIProvider) ChatStream(ctx context.Context, request *ChatRequest) (<-chan *ChatStreamEvent, error) {
	request.Stream = true
	
	reqBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/openai/deployments/%s/chat/completions?api-version=%s",
		p.config.Endpoint, p.config.DeploymentName, p.config.APIVersion)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("api-key", p.config.APIKey)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, &ProviderError{
			Provider:   "azure_openai",
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("Azure OpenAI API error: %s", string(body)),
		}
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
			if data == "[DONE]" {
				eventCh <- &ChatStreamEvent{Done: true}
				return
			}

			var event ChatStreamEvent
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				continue
			}

			eventCh <- &event
		}
	}()

	return eventCh, nil
}

// ListModels returns available models.
func (p *AzureOpenAIProvider) ListModels(ctx context.Context) ([]string, error) {
	return []string{p.config.DeploymentName}, nil
}

// Close closes the provider.
func (p *AzureOpenAIProvider) Close() error {
	return nil
}
