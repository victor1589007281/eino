// Package vlm 通义千问 VL 引擎
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

// QwenVLEngine 通义千问 VL 引擎
type QwenVLEngine struct {
	config       *QwenVLConfig
	client       *http.Client
	mu           sync.RWMutex
	loaded       bool
	lastUsed     time.Time
	totalCalls   int64
	failedCalls  int64
	totalLatency int64
}

// QwenVLConfig 通义千问配置
type QwenVLConfig struct {
	APIKey   string
	Model    string
	Endpoint string
	Timeout  time.Duration
}

// DefaultQwenVLConfig 默认配置
func DefaultQwenVLConfig() *QwenVLConfig {
	return &QwenVLConfig{
		Model:    "qwen-vl-max",
		Endpoint: "https://dashscope.aliyuncs.com/api/v1/services/aigc/multimodal-generation/generation",
		Timeout:  60 * time.Second,
	}
}

// NewQwenVLEngine 创建通义千问 VL 引擎
func NewQwenVLEngine(config *QwenVLConfig) *QwenVLEngine {
	if config == nil {
		config = DefaultQwenVLConfig()
	}
	return &QwenVLEngine{
		config: config,
		client: &http.Client{Timeout: config.Timeout},
	}
}

// Name 返回引擎名称
func (e *QwenVLEngine) Name() ocr.EngineType {
	return ocr.EngineQwenVL
}

// Layer 返回引擎层级
func (e *QwenVLEngine) Layer() ocr.EngineLayer {
	return ocr.LayerVLM
}

// SupportedScenes 支持的场景
func (e *QwenVLEngine) SupportedScenes() []ocr.SceneType {
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
func (e *QwenVLEngine) Recognize(ctx context.Context, req *ocr.OCRRequest) (*ocr.OCRResult, error) {
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

	// 准备图像
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

	// 获取提示词
	prompt := GetPromptTemplate(req.Scene)

	// 调用 API
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

	// 解析结果
	parsed := ParseOCRResponse(text)
	result.Success = true
	result.FullText = parsed.FullText
	result.Blocks = parsed.Blocks

	return result, nil
}

// Chat 图文对话
func (e *QwenVLEngine) Chat(ctx context.Context, imageBase64 string, prompt string) (string, error) {
	return e.ChatWithSystem(ctx, imageBase64, "", prompt)
}

// ChatWithSystem 带系统提示的图文对话
func (e *QwenVLEngine) ChatWithSystem(ctx context.Context, imageBase64, systemPrompt, userPrompt string) (string, error) {
	// 构建消息
	messages := []map[string]interface{}{}

	if systemPrompt != "" {
		messages = append(messages, map[string]interface{}{
			"role":    "system",
			"content": []map[string]string{{"text": systemPrompt}},
		})
	}

	messages = append(messages, map[string]interface{}{
		"role": "user",
		"content": []map[string]interface{}{
			{"image": "data:image/jpeg;base64," + imageBase64},
			{"text": userPrompt},
		},
	})

	payload := map[string]interface{}{
		"model": e.config.Model,
		"input": map[string]interface{}{
			"messages": messages,
		},
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
	req.Header.Set("Authorization", "Bearer "+e.config.APIKey)

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

	// 解析响应
	var result struct {
		Output struct {
			Choices []struct {
				Message struct {
					Content []struct {
						Text string `json:"text"`
					} `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		} `json:"output"`
		Code    string `json:"code"`
		Message string `json:"message"`
	}

	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return "", err
	}

	if result.Code != "" {
		return "", fmt.Errorf("qwen error: %s - %s", result.Code, result.Message)
	}

	if len(result.Output.Choices) > 0 && len(result.Output.Choices[0].Message.Content) > 0 {
		return result.Output.Choices[0].Message.Content[0].Text, nil
	}

	return "", fmt.Errorf("empty response")
}

// RecognizeTable 表格识别
func (e *QwenVLEngine) RecognizeTable(ctx context.Context, req *ocr.OCRRequest) (*ocr.TableResult, error) {
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
func (e *QwenVLEngine) RecognizeInvoice(ctx context.Context, req *ocr.OCRRequest) (*ocr.InvoiceResult, error) {
	req.Scene = ocr.SceneInvoice
	ocrResult, err := e.Recognize(ctx, req)
	return &ocr.InvoiceResult{OCRResult: *ocrResult}, err
}

// RecognizeIDCard 证件识别
func (e *QwenVLEngine) RecognizeIDCard(ctx context.Context, req *ocr.OCRRequest) (*ocr.IDCardResult, error) {
	req.Scene = ocr.SceneIDCard
	ocrResult, err := e.Recognize(ctx, req)
	return &ocr.IDCardResult{OCRResult: *ocrResult}, err
}

// MemoryUsage 返回内存占用
func (e *QwenVLEngine) MemoryUsage() int {
	return 10 // 云服务不占用本地内存
}

// IsLoaded 是否已加载
func (e *QwenVLEngine) IsLoaded() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.loaded
}

// Load 加载引擎
func (e *QwenVLEngine) Load(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.config.APIKey == "" {
		return fmt.Errorf("qwen api key not configured")
	}

	e.loaded = true
	return nil
}

// Unload 卸载引擎
func (e *QwenVLEngine) Unload() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.loaded = false
	return nil
}

// Health 健康检查
func (e *QwenVLEngine) Health(ctx context.Context) error {
	if e.config.APIKey == "" {
		return fmt.Errorf("api key not configured")
	}
	return nil
}

func (e *QwenVLEngine) recordFailure() {
	e.mu.Lock()
	e.failedCalls++
	e.mu.Unlock()
}

// Stats 获取统计信息
func (e *QwenVLEngine) Stats() ocr.EngineStatus {
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

var _ ocr.Engine = (*QwenVLEngine)(nil)
var _ VLMEngine = (*QwenVLEngine)(nil)
