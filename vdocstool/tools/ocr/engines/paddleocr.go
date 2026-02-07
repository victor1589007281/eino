// Package engines PaddleOCR 引擎实现
package engines

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"sync"
	"time"

	"github.com/cloudwego/eino/vdocstool/tools/ocr"
)

// PaddleOCREngine PaddleOCR 引擎 (通过 HTTP API 调用)
type PaddleOCREngine struct {
	config       *PaddleOCRConfig
	client       *http.Client
	mu           sync.RWMutex
	loaded       bool
	lastUsed     time.Time
	totalCalls   int64
	failedCalls  int64
	totalLatency int64
}

// PaddleOCRConfig PaddleOCR 配置
type PaddleOCRConfig struct {
	Endpoint    string        // PaddleOCR 服务端点
	Timeout     time.Duration // 请求超时
	UseGPU      bool          // 是否使用 GPU
	EnableTable bool          // 是否启用表格识别
}

// DefaultPaddleOCRConfig 默认 PaddleOCR 配置
func DefaultPaddleOCRConfig() *PaddleOCRConfig {
	return &PaddleOCRConfig{
		Endpoint:    "http://localhost:8866",
		Timeout:     60 * time.Second,
		UseGPU:      false,
		EnableTable: true,
	}
}

// NewPaddleOCREngine 创建 PaddleOCR 引擎
func NewPaddleOCREngine(config *PaddleOCRConfig) *PaddleOCREngine {
	if config == nil {
		config = DefaultPaddleOCRConfig()
	}
	return &PaddleOCREngine{
		config: config,
		client: &http.Client{Timeout: config.Timeout},
	}
}

// Name 返回引擎名称
func (e *PaddleOCREngine) Name() ocr.EngineType {
	return ocr.EnginePaddleOCR
}

// Layer 返回引擎层级
func (e *PaddleOCREngine) Layer() ocr.EngineLayer {
	return ocr.LayerSmallModel
}

// SupportedScenes 支持的场景
func (e *PaddleOCREngine) SupportedScenes() []ocr.SceneType {
	return []ocr.SceneType{
		ocr.SceneGeneral,
		ocr.ScenePrint,
		ocr.SceneDocument,
		ocr.SceneTable,
		ocr.SceneInvoice,
	}
}

// paddleOCRResponse PaddleOCR API 响应
type paddleOCRResponse struct {
	Status  string `json:"status"`
	Message string `json:"msg"`
	Results []struct {
		Text       string      `json:"text"`
		Confidence float64     `json:"confidence"`
		TextRegion [][]float64 `json:"text_region"`
	} `json:"results"`
}

// Recognize 执行 OCR 识别
func (e *PaddleOCREngine) Recognize(ctx context.Context, req *ocr.OCRRequest) (*ocr.OCRResult, error) {
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

	// 准备图像数据
	preprocessor := ocr.NewPreprocessor(nil)
	imgData, err := preprocessor.LoadImage(ctx, req)
	if err != nil {
		e.recordFailure()
		result.Error = fmt.Sprintf("load image failed: %v", err)
		return result, err
	}

	result.Metadata.ImageWidth = imgData.Width
	result.Metadata.ImageHeight = imgData.Height

	// 构建请求
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	part, err := writer.CreateFormFile("images", "image.jpg")
	if err != nil {
		e.recordFailure()
		result.Error = fmt.Sprintf("create form file failed: %v", err)
		return result, err
	}

	if _, err := part.Write(imgData.RawBytes); err != nil {
		e.recordFailure()
		result.Error = fmt.Sprintf("write image data failed: %v", err)
		return result, err
	}

	if err := writer.Close(); err != nil {
		e.recordFailure()
		result.Error = fmt.Sprintf("close writer failed: %v", err)
		return result, err
	}

	// 发送请求
	endpoint := e.config.Endpoint + "/predict/ocr_system"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", endpoint, &body)
	if err != nil {
		e.recordFailure()
		result.Error = fmt.Sprintf("create request failed: %v", err)
		return result, err
	}
	httpReq.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := e.client.Do(httpReq)
	if err != nil {
		e.recordFailure()
		result.Error = fmt.Sprintf("request failed: %v", err)
		return result, err
	}
	defer resp.Body.Close()

	latency := time.Since(startTime).Milliseconds()
	result.Latency = latency

	e.mu.Lock()
	e.totalLatency += latency
	e.mu.Unlock()

	if resp.StatusCode != http.StatusOK {
		e.recordFailure()
		bodyBytes, _ := io.ReadAll(resp.Body)
		result.Error = fmt.Sprintf("request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
		return result, fmt.Errorf("paddleocr: %s", result.Error)
	}

	// 解析响应
	var ocrResp paddleOCRResponse
	if err := json.NewDecoder(resp.Body).Decode(&ocrResp); err != nil {
		e.recordFailure()
		result.Error = fmt.Sprintf("decode response failed: %v", err)
		return result, err
	}

	if ocrResp.Status != "000" {
		e.recordFailure()
		result.Error = ocrResp.Message
		return result, fmt.Errorf("ocr failed: %s", ocrResp.Message)
	}

	// 构建结果
	result.Success = true
	var fullText string
	var totalConf float64

	for _, item := range ocrResp.Results {
		if fullText != "" {
			fullText += "\n"
		}
		fullText += item.Text
		totalConf += item.Confidence

		block := ocr.TextBlock{
			Text:       item.Text,
			Confidence: item.Confidence,
			BlockType:  "text",
		}

		// 转换边界框
		if len(item.TextRegion) >= 4 && req.ReturnPosition {
			minX, minY := item.TextRegion[0][0], item.TextRegion[0][1]
			maxX, maxY := item.TextRegion[0][0], item.TextRegion[0][1]
			for _, pt := range item.TextRegion {
				if pt[0] < minX {
					minX = pt[0]
				}
				if pt[0] > maxX {
					maxX = pt[0]
				}
				if pt[1] < minY {
					minY = pt[1]
				}
				if pt[1] > maxY {
					maxY = pt[1]
				}
			}
			block.Box = &ocr.BoundingBox{
				X:      int(minX),
				Y:      int(minY),
				Width:  int(maxX - minX),
				Height: int(maxY - minY),
			}
		}

		result.Blocks = append(result.Blocks, block)
	}

	result.FullText = fullText
	if len(ocrResp.Results) > 0 && req.ReturnConfidence {
		result.Confidence = totalConf / float64(len(ocrResp.Results))
	}

	return result, nil
}

// RecognizeTable 表格识别 (使用 PP-Structure)
func (e *PaddleOCREngine) RecognizeTable(ctx context.Context, req *ocr.OCRRequest) (*ocr.TableResult, error) {
	e.mu.Lock()
	e.totalCalls++
	e.lastUsed = time.Now()
	e.mu.Unlock()

	startTime := time.Now()
	result := &ocr.TableResult{
		OCRResult: ocr.OCRResult{
			Engine: e.Name(),
			Metadata: &ocr.OCRMeta{
				ProcessTime: startTime,
			},
		},
	}

	// 准备图像数据
	preprocessor := ocr.NewPreprocessor(nil)
	imgData, err := preprocessor.LoadImage(ctx, req)
	if err != nil {
		e.recordFailure()
		result.Error = fmt.Sprintf("load image failed: %v", err)
		return result, err
	}

	// 构建请求
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	part, err := writer.CreateFormFile("images", "image.jpg")
	if err != nil {
		e.recordFailure()
		return result, err
	}

	if _, err := part.Write(imgData.RawBytes); err != nil {
		e.recordFailure()
		return result, err
	}

	if err := writer.Close(); err != nil {
		e.recordFailure()
		return result, err
	}

	// 发送表格识别请求
	endpoint := e.config.Endpoint + "/predict/table"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", endpoint, &body)
	if err != nil {
		e.recordFailure()
		return result, err
	}
	httpReq.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := e.client.Do(httpReq)
	if err != nil {
		e.recordFailure()
		return result, err
	}
	defer resp.Body.Close()

	latency := time.Since(startTime).Milliseconds()
	result.Latency = latency

	if resp.StatusCode != http.StatusOK {
		e.recordFailure()
		bodyBytes, _ := io.ReadAll(resp.Body)
		result.Error = fmt.Sprintf("table recognition failed: %s", string(bodyBytes))
		return result, fmt.Errorf("paddleocr: %s", result.Error)
	}

	// 解析表格响应
	var tableResp struct {
		Status string `json:"status"`
		Tables []struct {
			HTML string     `json:"html"`
			Cells [][]string `json:"cells"`
		} `json:"tables"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tableResp); err != nil {
		e.recordFailure()
		return result, err
	}

	result.Success = true
	for _, t := range tableResp.Tables {
		table := ocr.Table{
			HTML: t.HTML,
			Rows: t.Cells,
		}
		if len(t.Cells) > 0 {
			table.Headers = t.Cells[0]
			if len(t.Cells) > 1 {
				table.Rows = t.Cells[1:]
			}
		}
		result.Tables = append(result.Tables, table)
	}

	return result, nil
}

// RecognizeInvoice 票据识别
func (e *PaddleOCREngine) RecognizeInvoice(ctx context.Context, req *ocr.OCRRequest) (*ocr.InvoiceResult, error) {
	// PaddleOCR 通用识别后进行结构化提取
	ocrResult, err := e.Recognize(ctx, req)
	if err != nil {
		return &ocr.InvoiceResult{OCRResult: *ocrResult}, err
	}

	result := &ocr.InvoiceResult{
		OCRResult: *ocrResult,
	}

	// 简单的发票信息提取（基于关键词匹配）
	result.ExtraFields = make(map[string]string)
	for _, block := range ocrResult.Blocks {
		text := block.Text
		// 发票代码
		if containsAny(text, "发票代码", "Invoice Code") {
			result.InvoiceCode = extractValue(text)
		}
		// 发票号码
		if containsAny(text, "发票号码", "Invoice No") {
			result.InvoiceNo = extractValue(text)
		}
		// 金额
		if containsAny(text, "合计", "总额", "金额", "Total") {
			result.TotalAmount = extractValue(text)
		}
		// 日期
		if containsAny(text, "日期", "Date", "开票日期") {
			result.Date = extractValue(text)
		}
	}

	return result, nil
}

// RecognizeIDCard 证件识别
func (e *PaddleOCREngine) RecognizeIDCard(ctx context.Context, req *ocr.OCRRequest) (*ocr.IDCardResult, error) {
	ocrResult, err := e.Recognize(ctx, req)
	if err != nil {
		return &ocr.IDCardResult{OCRResult: *ocrResult}, err
	}

	result := &ocr.IDCardResult{
		OCRResult: *ocrResult,
	}

	// 简单的身份证信息提取
	result.ExtraFields = make(map[string]string)
	for _, block := range ocrResult.Blocks {
		text := block.Text
		if containsAny(text, "姓名", "Name") {
			result.Name = extractValue(text)
		}
		if containsAny(text, "性别", "Sex") {
			result.Gender = extractValue(text)
		}
		if containsAny(text, "民族", "Nationality") {
			result.Ethnicity = extractValue(text)
		}
		if containsAny(text, "出生", "Birth") {
			result.Birthday = extractValue(text)
		}
		if containsAny(text, "住址", "Address") {
			result.Address = extractValue(text)
		}
		if containsAny(text, "公民身份号码", "ID Number") {
			result.IDNumber = extractValue(text)
		}
	}

	return result, nil
}

// MemoryUsage 返回内存占用
func (e *PaddleOCREngine) MemoryUsage() int {
	if e.config.UseGPU {
		return 1500 // GPU 模式约 1.5GB
	}
	return 800 // CPU 模式约 800MB
}

// IsLoaded 是否已加载
func (e *PaddleOCREngine) IsLoaded() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.loaded
}

// Load 加载引擎
func (e *PaddleOCREngine) Load(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if err := e.healthCheck(ctx); err != nil {
		return fmt.Errorf("paddleocr service not available: %w", err)
	}

	e.loaded = true
	return nil
}

// Unload 卸载引擎
func (e *PaddleOCREngine) Unload() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.loaded = false
	return nil
}

// Health 健康检查
func (e *PaddleOCREngine) Health(ctx context.Context) error {
	return e.healthCheck(ctx)
}

func (e *PaddleOCREngine) healthCheck(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", e.config.Endpoint+"/", nil)
	if err != nil {
		return err
	}

	resp, err := e.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check failed: status %d", resp.StatusCode)
	}

	return nil
}

func (e *PaddleOCREngine) recordFailure() {
	e.mu.Lock()
	e.failedCalls++
	e.mu.Unlock()
}

// Stats 获取统计信息
func (e *PaddleOCREngine) Stats() ocr.EngineStatus {
	e.mu.RLock()
	defer e.mu.RUnlock()

	avgLatency := float64(0)
	if e.totalCalls > 0 {
		avgLatency = float64(e.totalLatency) / float64(e.totalCalls)
	}

	return ocr.EngineStatus{
		Engine:      e.Name(),
		Available:   true,
		Loaded:      e.loaded,
		MemoryMB:    e.MemoryUsage(),
		LastUsed:    e.lastUsed,
		TotalCalls:  e.totalCalls,
		FailedCalls: e.failedCalls,
		AvgLatency:  avgLatency,
	}
}

// 辅助函数
func containsAny(text string, keywords ...string) bool {
	for _, kw := range keywords {
		if len(text) >= len(kw) {
			for i := 0; i <= len(text)-len(kw); i++ {
				if text[i:i+len(kw)] == kw {
					return true
				}
			}
		}
	}
	return false
}

func extractValue(text string) string {
	// 简单提取冒号后的值
	runes := []rune(text)
	for i, c := range runes {
		if c == ':' || c == '：' {
			if i+1 < len(runes) {
				return trimSpace(string(runes[i+1:]))
			}
		}
	}
	return text
}

func trimSpace(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}
