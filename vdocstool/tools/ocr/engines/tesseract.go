// Package engines OCR 引擎实现
package engines

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino/vdocstool/tools/ocr"
)

// TesseractEngine Tesseract OCR 引擎
type TesseractEngine struct {
	config       *TesseractConfig
	mu           sync.RWMutex
	loaded       bool
	lastUsed     time.Time
	totalCalls   int64
	failedCalls  int64
	totalLatency int64
}

// TesseractConfig Tesseract 配置
type TesseractConfig struct {
	TesseractPath string   // tesseract 可执行文件路径
	Languages     []string // 语言列表 ["chi_sim", "eng"]
	PSM           int      // 页面分割模式 (0-13)
	OEM           int      // OCR 引擎模式 (0-3)
	DPI           int      // DPI
	Whitelist     string   // 字符白名单
	Blacklist     string   // 字符黑名单
	Timeout       time.Duration
}

// DefaultTesseractConfig 默认 Tesseract 配置
func DefaultTesseractConfig() *TesseractConfig {
	return &TesseractConfig{
		TesseractPath: "tesseract",
		Languages:     []string{"chi_sim", "eng"},
		PSM:           3,  // Fully automatic page segmentation
		OEM:           3,  // Default, based on available models
		DPI:           300,
		Timeout:       30 * time.Second,
	}
}

// NewTesseractEngine 创建 Tesseract 引擎
func NewTesseractEngine(config *TesseractConfig) *TesseractEngine {
	if config == nil {
		config = DefaultTesseractConfig()
	}
	return &TesseractEngine{
		config: config,
	}
}

// Name 返回引擎名称
func (e *TesseractEngine) Name() ocr.EngineType {
	return ocr.EngineTesseract
}

// Layer 返回引擎层级
func (e *TesseractEngine) Layer() ocr.EngineLayer {
	return ocr.LayerTraditional
}

// SupportedScenes 支持的场景
func (e *TesseractEngine) SupportedScenes() []ocr.SceneType {
	return []ocr.SceneType{
		ocr.SceneGeneral,
		ocr.ScenePrint,
		ocr.SceneDocument,
	}
}

// Recognize 执行 OCR 识别
func (e *TesseractEngine) Recognize(ctx context.Context, req *ocr.OCRRequest) (*ocr.OCRResult, error) {
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
	var imageInput string
	var stdin *bytes.Buffer

	if req.ImagePath != "" {
		imageInput = req.ImagePath
	} else if req.ImageBase64 != "" || req.ImageURL != "" {
		// 需要预处理器加载图像
		preprocessor := ocr.NewPreprocessor(nil)
		imgData, err := preprocessor.LoadImage(ctx, req)
		if err != nil {
			e.recordFailure()
			result.Error = fmt.Sprintf("load image failed: %v", err)
			return result, err
		}
		imageInput = "stdin"
		stdin = bytes.NewBuffer(imgData.RawBytes)
		result.Metadata.ImageWidth = imgData.Width
		result.Metadata.ImageHeight = imgData.Height
	} else {
		e.recordFailure()
		result.Error = "no image source provided"
		return result, fmt.Errorf("no image source provided")
	}

	// 构建命令参数
	args := e.buildArgs(req, imageInput)

	// 创建带超时的上下文
	timeout := e.config.Timeout
	if req.Timeout > 0 {
		timeout = time.Duration(req.Timeout) * time.Second
	}
	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// 执行 tesseract
	cmd := exec.CommandContext(cmdCtx, e.config.TesseractPath, args...)
	
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	
	if stdin != nil {
		cmd.Stdin = stdin
	}

	err := cmd.Run()
	latency := time.Since(startTime).Milliseconds()
	result.Latency = latency

	e.mu.Lock()
	e.totalLatency += latency
	e.mu.Unlock()

	if err != nil {
		e.recordFailure()
		if cmdCtx.Err() == context.DeadlineExceeded {
			result.Error = "timeout"
			return result, fmt.Errorf("tesseract timeout after %v", timeout)
		}
		result.Error = fmt.Sprintf("tesseract failed: %v, stderr: %s", err, stderr.String())
		return result, fmt.Errorf("tesseract failed: %w", err)
	}

	// 解析输出
	text := stdout.String()
	text = strings.TrimSpace(text)
	
	result.Success = true
	result.FullText = text
	
	// 解析文本块 (如果启用了位置信息)
	if req.ReturnPosition {
		result.Blocks = e.parseBlocks(text)
	}

	// 估算置信度 (Tesseract 不直接返回，这里用简单启发式)
	if req.ReturnConfidence {
		result.Confidence = e.estimateConfidence(text)
	}

	return result, nil
}

// buildArgs 构建命令参数
func (e *TesseractEngine) buildArgs(req *ocr.OCRRequest, imageInput string) []string {
	args := []string{imageInput, "stdout"}

	// 语言
	langs := e.config.Languages
	if len(req.Languages) > 0 {
		langs = e.convertLanguages(req.Languages)
	}
	args = append(args, "-l", strings.Join(langs, "+"))

	// PSM
	args = append(args, "--psm", strconv.Itoa(e.config.PSM))

	// OEM
	args = append(args, "--oem", strconv.Itoa(e.config.OEM))

	// DPI
	if e.config.DPI > 0 {
		args = append(args, "--dpi", strconv.Itoa(e.config.DPI))
	}

	// 字符白名单/黑名单
	if e.config.Whitelist != "" {
		args = append(args, "-c", fmt.Sprintf("tessedit_char_whitelist=%s", e.config.Whitelist))
	}
	if e.config.Blacklist != "" {
		args = append(args, "-c", fmt.Sprintf("tessedit_char_blacklist=%s", e.config.Blacklist))
	}

	return args
}

// convertLanguages 转换语言代码
func (e *TesseractEngine) convertLanguages(langs []string) []string {
	langMap := map[string]string{
		"zh":      "chi_sim",
		"zh-cn":   "chi_sim",
		"zh-tw":   "chi_tra",
		"en":      "eng",
		"ja":      "jpn",
		"ko":      "kor",
		"fr":      "fra",
		"de":      "deu",
		"es":      "spa",
		"ru":      "rus",
		"ar":      "ara",
	}

	result := make([]string, 0, len(langs))
	for _, lang := range langs {
		lang = strings.ToLower(lang)
		if mapped, ok := langMap[lang]; ok {
			result = append(result, mapped)
		} else {
			result = append(result, lang)
		}
	}
	return result
}

// parseBlocks 解析文本块
func (e *TesseractEngine) parseBlocks(text string) []ocr.TextBlock {
	lines := strings.Split(text, "\n")
	blocks := make([]ocr.TextBlock, 0, len(lines))
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		blocks = append(blocks, ocr.TextBlock{
			Text:      line,
			BlockType: "text",
		})
	}
	
	return blocks
}

// estimateConfidence 估算置信度
func (e *TesseractEngine) estimateConfidence(text string) float64 {
	if text == "" {
		return 0.0
	}

	// 基于文本特征估算
	// 1. 检查是否有太多乱码字符
	totalChars := len([]rune(text))
	if totalChars == 0 {
		return 0.0
	}

	// 统计有效字符
	validChars := 0
	for _, r := range text {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || (r >= 0x4e00 && r <= 0x9fff) ||
			r == ' ' || r == '\n' || r == '.' || r == ',' ||
			r == '。' || r == '，' || r == '、' || r == '！' || r == '？' {
			validChars++
		}
	}

	// 有效字符比例
	validRatio := float64(validChars) / float64(totalChars)
	
	// 基础置信度
	confidence := validRatio * 0.8

	// 如果有连续的中文或英文单词，提高置信度
	chinesePattern := regexp.MustCompile(`[\x{4e00}-\x{9fff}]{2,}`)
	englishPattern := regexp.MustCompile(`[a-zA-Z]{3,}`)
	
	if chinesePattern.MatchString(text) || englishPattern.MatchString(text) {
		confidence += 0.15
	}

	return min(confidence, 0.95)
}

// RecognizeTable 表格识别 (Tesseract 不直接支持)
func (e *TesseractEngine) RecognizeTable(ctx context.Context, req *ocr.OCRRequest) (*ocr.TableResult, error) {
	result, err := e.Recognize(ctx, req)
	return &ocr.TableResult{OCRResult: *result}, err
}

// RecognizeInvoice 票据识别 (Tesseract 不直接支持)
func (e *TesseractEngine) RecognizeInvoice(ctx context.Context, req *ocr.OCRRequest) (*ocr.InvoiceResult, error) {
	result, err := e.Recognize(ctx, req)
	return &ocr.InvoiceResult{OCRResult: *result}, err
}

// RecognizeIDCard 证件识别 (Tesseract 不直接支持)
func (e *TesseractEngine) RecognizeIDCard(ctx context.Context, req *ocr.OCRRequest) (*ocr.IDCardResult, error) {
	result, err := e.Recognize(ctx, req)
	return &ocr.IDCardResult{OCRResult: *result}, err
}

// MemoryUsage 返回内存占用 (Tesseract 是外部进程，这里返回估计值)
func (e *TesseractEngine) MemoryUsage() int {
	return 50 // ~50MB 估计值
}

// IsLoaded 是否已加载
func (e *TesseractEngine) IsLoaded() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.loaded
}

// Load 加载引擎 (验证 tesseract 可用)
func (e *TesseractEngine) Load(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// 检查 tesseract 是否存在
	cmd := exec.CommandContext(ctx, e.config.TesseractPath, "--version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("tesseract not available: %w", err)
	}

	e.loaded = true
	return nil
}

// Unload 卸载引擎 (Tesseract 无需特殊处理)
func (e *TesseractEngine) Unload() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.loaded = false
	return nil
}

// Health 健康检查
func (e *TesseractEngine) Health(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, e.config.TesseractPath, "--version")
	return cmd.Run()
}

// recordFailure 记录失败
func (e *TesseractEngine) recordFailure() {
	e.mu.Lock()
	e.failedCalls++
	e.mu.Unlock()
}

// Stats 获取统计信息
func (e *TesseractEngine) Stats() ocr.EngineStatus {
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

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
