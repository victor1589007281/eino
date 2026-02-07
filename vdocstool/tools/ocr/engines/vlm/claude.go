// Package vlm Claude Vision 引擎
package vlm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/cloudwego/eino/vdocstool/tools/ocr"
)

// ClaudeVLEngine Claude Vision 引擎
type ClaudeVLEngine struct {
	config       *ClaudeVLConfig
	client       *http.Client
	mu           sync.RWMutex
	loaded       bool
	lastUsed     time.Time
	totalCalls   int64
	failedCalls  int64
	totalLatency int64
}

// ClaudeVLConfig Claude 配置
type ClaudeVLConfig struct {
	APIKey   string
	Model    string
	Endpoint string
	Timeout  time.Duration
}

// DefaultClaudeVLConfig 默认配置
func DefaultClaudeVLConfig() *ClaudeVLConfig {
	return &ClaudeVLConfig{
		Model:    "claude-sonnet-4-20250514",
		Endpoint: "https://api.anthropic.com/v1/messages",
		Timeout:  60 * time.Second,
	}
}

// NewClaudeVLEngine 创建 Claude VL 引擎
func NewClaudeVLEngine(config *ClaudeVLConfig) *ClaudeVLEngine {
	if config == nil {
		config = DefaultClaudeVLConfig()
	}
	return &ClaudeVLEngine{
		config: config,
		client: &http.Client{Timeout: config.Timeout},
	}
}

// Name 返回引擎名称
func (e *ClaudeVLEngine) Name() ocr.EngineType {
	return ocr.EngineClaude
}

// Layer 返回引擎层级
func (e *ClaudeVLEngine) Layer() ocr.EngineLayer {
	return ocr.LayerVLM
}

// SupportedScenes 支持的场景
func (e *ClaudeVLEngine) SupportedScenes() []ocr.SceneType {
	return []ocr.SceneType{
		ocr.SceneGeneral,
		ocr.ScenePrint,
		ocr.SceneDocument,
		ocr.SceneTable,
		ocr.SceneInvoice,
		ocr.SceneIDCard,
		ocr.SceneHandwriting,
	}
}

// Recognize 执行 OCR 识别
func (e *ClaudeVLEngine) Recognize(ctx context.Context, req *ocr.OCRRequest) (*ocr.OCRResult, error) {
	e.mu.Lock()
	e.totalCalls++
	e.lastUsed = time.Now()
	e.mu.Unlock()

	startTime := time.Now()
	result := &ocr.OCRResult{
		Engine: e.Name(),
		Metadata: &ocr.OCRMeta{
			ProcessTime: startTime,
		},
	}

	preprocessor := ocr.NewPreprocessor(nil)
	imgData, err := preprocessor.LoadImage(ctx, req)
	if err != nil {
		e.recordFailure()
		result.Error = fmt.Sprintf("load image failed: %v", err)
		return result, err
	}

	result.Metadata.ImageWidth = imgData.Width
	result.Metadata.ImageHeight = imgData.Height

	imageBase64, err := preprocessor.ToBase64(imgData)
	if err != nil {
		e.recordFailure()
		result.Error = fmt.Sprintf("encode base64 failed: %v", err)
		return result, err
	}

	prompt := GetPromptTemplate(req.Scene)
	text, err := e.ChatWithSystem(ctx, imageBase64, prompt.System, prompt.User)
	if err != nil {
		e.recordFailure()
		result.Error = fmt.Sprintf("api call failed: %v", err)
		return result, err
	}

	latency := time.Since(startTime).Milliseconds()
	result.Latency = latency

	e.mu.Lock()
	e.totalLatency += latency
	e.mu.Unlock()

	parsed := ParseOCRResponse(text)
	result.Success = true
	result.FullText = parsed.FullText
	result.Blocks = parsed.Blocks

	return result, nil
}

// Chat 图文对话
func (e *ClaudeVLEngine) Chat(ctx context.Context, imageBase64 string, prompt string) (string, error) {
	return e.ChatWithSystem(ctx, imageBase64, "", prompt)
}

// ChatWithSystem 带系统提示的图文对话
func (e *ClaudeVLEngine) ChatWithSystem(ctx context.Context, imageBase64, systemPrompt, userPrompt string) (string, error) {
	// 构建 Claude 格式的消息
	content := []map[string]interface{}{
		{
			"type": "image",
			"source": map[string]string{
				"type":       "base64",
				"media_type": "image/jpeg",
				"data":       imageBase64,
			},
		},
		{
			"type": "text",
			"text": userPrompt,
		},
	}

	messages := []map[string]interface{}{
		{
			"role":    "user",
			"content": content,
		},
	}

	payload := map[string]interface{}{
		"model":      e.config.Model,
		"messages":   messages,
		"max_tokens": 4096,
	}

	if systemPrompt != "" {
		payload["system"] = systemPrompt
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", e.config.Endpoint, bytes.NewReader(payloadBytes))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", e.config.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := e.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("api error: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}

	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return "", err
	}

	if result.Error.Message != "" {
		return "", fmt.Errorf("claude error: %s - %s", result.Error.Type, result.Error.Message)
	}

	for _, c := range result.Content {
		if c.Type == "text" {
			return c.Text, nil
		}
	}

	return "", fmt.Errorf("empty response")
}

// RecognizeTable 表格识别
func (e *ClaudeVLEngine) RecognizeTable(ctx context.Context, req *ocr.OCRRequest) (*ocr.TableResult, error) {
	req.Scene = ocr.SceneTable
	ocrResult, err := e.Recognize(ctx, req)
	if err != nil {
		return &ocr.TableResult{OCRResult: *ocrResult}, err
	}

	tableResult := ParseTableResponse(ocrResult.FullText)
	tableResult.OCRResult = *ocrResult
	return tableResult, nil
}

// RecognizeInvoice 票据识别
func (e *ClaudeVLEngine) RecognizeInvoice(ctx context.Context, req *ocr.OCRRequest) (*ocr.InvoiceResult, error) {
	req.Scene = ocr.SceneInvoice
	ocrResult, err := e.Recognize(ctx, req)
	return &ocr.InvoiceResult{OCRResult: *ocrResult}, err
}

// RecognizeIDCard 证件识别
func (e *ClaudeVLEngine) RecognizeIDCard(ctx context.Context, req *ocr.OCRRequest) (*ocr.IDCardResult, error) {
	req.Scene = ocr.SceneIDCard
	ocrResult, err := e.Recognize(ctx, req)
	return &ocr.IDCardResult{OCRResult: *ocrResult}, err
}

// MemoryUsage 返回内存占用
func (e *ClaudeVLEngine) MemoryUsage() int {
	return 10
}

// IsLoaded 是否已加载
func (e *ClaudeVLEngine) IsLoaded() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.loaded
}

// Load 加载引擎
func (e *ClaudeVLEngine) Load(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.config.APIKey == "" {
		return fmt.Errorf("claude api key not configured")
	}

	e.loaded = true
	return nil
}

// Unload 卸载引擎
func (e *ClaudeVLEngine) Unload() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.loaded = false
	return nil
}

// Health 健康检查
func (e *ClaudeVLEngine) Health(ctx context.Context) error {
	if e.config.APIKey == "" {
		return fmt.Errorf("api key not configured")
	}
	return nil
}

func (e *ClaudeVLEngine) recordFailure() {
	e.mu.Lock()
	e.failedCalls++
	e.mu.Unlock()
}

// Stats 获取统计信息
func (e *ClaudeVLEngine) Stats() ocr.EngineStatus {
	e.mu.RLock()
	defer e.mu.RUnlock()

	avgLatency := float64(0)
	if e.totalCalls > 0 {
		avgLatency = float64(e.totalLatency) / float64(e.totalCalls)
	}

	return ocr.EngineStatus{
		Engine:      e.Name(),
		Available:   e.config.APIKey != "",
		Loaded:      e.loaded,
		MemoryMB:    e.MemoryUsage(),
		LastUsed:    e.lastUsed,
		TotalCalls:  e.totalCalls,
		FailedCalls: e.failedCalls,
		AvgLatency:  avgLatency,
	}
}

var _ ocr.Engine = (*ClaudeVLEngine)(nil)
var _ VLMEngine = (*ClaudeVLEngine)(nil)
