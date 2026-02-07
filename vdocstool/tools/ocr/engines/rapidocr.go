// Package engines RapidOCR 引擎实现
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

// RapidOCREngine RapidOCR 引擎 (通过 HTTP API 调用)
type RapidOCREngine struct {
	config       *RapidOCRConfig
	client       *http.Client
	mu           sync.RWMutex
	loaded       bool
	lastUsed     time.Time
	totalCalls   int64
	failedCalls  int64
	totalLatency int64
}

// RapidOCRConfig RapidOCR 配置
type RapidOCRConfig struct {
	Endpoint string        // RapidOCR 服务端点
	Timeout  time.Duration // 请求超时
}

// DefaultRapidOCRConfig 默认 RapidOCR 配置
func DefaultRapidOCRConfig() *RapidOCRConfig {
	return &RapidOCRConfig{
		Endpoint: "http://localhost:8089",
		Timeout:  30 * time.Second,
	}
}

// NewRapidOCREngine 创建 RapidOCR 引擎
func NewRapidOCREngine(config *RapidOCRConfig) *RapidOCREngine {
	if config == nil {
		config = DefaultRapidOCRConfig()
	}
	return &RapidOCREngine{
		config: config,
		client: &http.Client{Timeout: config.Timeout},
	}
}

// Name 返回引擎名称
func (e *RapidOCREngine) Name() ocr.EngineType {
	return ocr.EngineRapidOCR
}

// Layer 返回引擎层级
func (e *RapidOCREngine) Layer() ocr.EngineLayer {
	return ocr.LayerSmallModel
}

// SupportedScenes 支持的场景
func (e *RapidOCREngine) SupportedScenes() []ocr.SceneType {
	return []ocr.SceneType{
		ocr.SceneGeneral,
		ocr.ScenePrint,
		ocr.SceneDocument,
		ocr.SceneTable,
	}
}

// rapidOCRResponse RapidOCR API 响应
type rapidOCRResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Results []struct {
			Text       string      `json:"text"`
			Confidence float64     `json:"confidence"`
			Box        [][]float64 `json:"box"` // [[x1,y1], [x2,y2], [x3,y3], [x4,y4]]
		} `json:"results"`
		ElapsedTime float64 `json:"elapsed_time"`
	} `json:"data"`
}

// Recognize 执行 OCR 识别
func (e *RapidOCREngine) Recognize(ctx context.Context, req *ocr.OCRRequest) (*ocr.OCRResult, error) {
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

	// 构建 multipart 请求
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	part, err := writer.CreateFormFile("image", "image.jpg")
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

	// 添加可选参数
	if req.DetectAngle {
		_ = writer.WriteField("detect_angle", "true")
	}

	if err := writer.Close(); err != nil {
		e.recordFailure()
		result.Error = fmt.Sprintf("close writer failed: %v", err)
		return result, err
	}

	// 发送请求
	httpReq, err := http.NewRequestWithContext(ctx, "POST", e.config.Endpoint+"/ocr", &body)
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
		return result, fmt.Errorf("request failed: %s", result.Error)
	}

	// 解析响应
	var ocrResp rapidOCRResponse
	if err := json.NewDecoder(resp.Body).Decode(&ocrResp); err != nil {
		e.recordFailure()
		result.Error = fmt.Sprintf("decode response failed: %v", err)
		return result, err
	}

	if ocrResp.Code != 0 {
		e.recordFailure()
		result.Error = ocrResp.Message
		return result, fmt.Errorf("ocr failed: %s", ocrResp.Message)
	}

	// 构建结果
	result.Success = true
	var fullText string
	var totalConf float64

	for _, item := range ocrResp.Data.Results {
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
		if len(item.Box) >= 4 && req.ReturnPosition {
			minX, minY := item.Box[0][0], item.Box[0][1]
			maxX, maxY := item.Box[0][0], item.Box[0][1]
			for _, pt := range item.Box {
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
	if len(ocrResp.Data.Results) > 0 && req.ReturnConfidence {
		result.Confidence = totalConf / float64(len(ocrResp.Data.Results))
	}

	return result, nil
}

// RecognizeTable 表格识别
func (e *RapidOCREngine) RecognizeTable(ctx context.Context, req *ocr.OCRRequest) (*ocr.TableResult, error) {
	// RapidOCR 本身不直接支持表格结构识别
	// 返回基础 OCR 结果
	result, err := e.Recognize(ctx, req)
	return &ocr.TableResult{OCRResult: *result}, err
}

// RecognizeInvoice 票据识别
func (e *RapidOCREngine) RecognizeInvoice(ctx context.Context, req *ocr.OCRRequest) (*ocr.InvoiceResult, error) {
	result, err := e.Recognize(ctx, req)
	return &ocr.InvoiceResult{OCRResult: *result}, err
}

// RecognizeIDCard 证件识别
func (e *RapidOCREngine) RecognizeIDCard(ctx context.Context, req *ocr.OCRRequest) (*ocr.IDCardResult, error) {
	result, err := e.Recognize(ctx, req)
	return &ocr.IDCardResult{OCRResult: *result}, err
}

// MemoryUsage 返回内存占用
func (e *RapidOCREngine) MemoryUsage() int {
	return 300 // RapidOCR 典型内存占用约 300MB
}

// IsLoaded 是否已加载
func (e *RapidOCREngine) IsLoaded() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.loaded
}

// Load 加载引擎 (检查服务可用性)
func (e *RapidOCREngine) Load(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// 健康检查
	if err := e.healthCheck(ctx); err != nil {
		return fmt.Errorf("rapidocr service not available: %w", err)
	}

	e.loaded = true
	return nil
}

// Unload 卸载引擎
func (e *RapidOCREngine) Unload() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.loaded = false
	return nil
}

// Health 健康检查
func (e *RapidOCREngine) Health(ctx context.Context) error {
	return e.healthCheck(ctx)
}

// healthCheck 内部健康检查
func (e *RapidOCREngine) healthCheck(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", e.config.Endpoint+"/health", nil)
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

// recordFailure 记录失败
func (e *RapidOCREngine) recordFailure() {
	e.mu.Lock()
	e.failedCalls++
	e.mu.Unlock()
}

// Stats 获取统计信息
func (e *RapidOCREngine) Stats() ocr.EngineStatus {
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
