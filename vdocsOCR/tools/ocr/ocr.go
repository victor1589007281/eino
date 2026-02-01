// Package ocr provides OCR (Optical Character Recognition) capabilities.
package ocr

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino/vdocsOCR/config"
)

// OCREngine represents the OCR engine interface.
type OCREngine interface {
	// Recognize performs OCR on the given image data.
	Recognize(ctx context.Context, imageData []byte, opts *RecognizeOptions) (*OCRResult, error)

	// RecognizeFile performs OCR on the given image file.
	RecognizeFile(ctx context.Context, filePath string, opts *RecognizeOptions) (*OCRResult, error)

	// Close releases resources.
	Close() error
}

// RecognizeOptions represents options for OCR recognition.
type RecognizeOptions struct {
	Language    string   `json:"language"`     // OCR language (e.g., "chi_sim+eng")
	DPI         int      `json:"dpi"`          // Image DPI
	PSM         int      `json:"psm"`          // Page segmentation mode (tesseract)
	OEM         int      `json:"oem"`          // OCR engine mode (tesseract)
	Whitelist   string   `json:"whitelist"`    // Whitelist of characters
	Regions     []Region `json:"regions"`      // Specific regions to recognize
	Preprocess  bool     `json:"preprocess"`   // Enable image preprocessing
	OutputType  string   `json:"output_type"`  // Output type: text, hocr, json
}

// Region represents a rectangular region in the image.
type Region struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

// OCRResult represents the result of OCR recognition.
type OCRResult struct {
	Text       string      `json:"text"`
	Confidence float64     `json:"confidence"`
	Blocks     []TextBlock `json:"blocks,omitempty"`
	Language   string      `json:"language"`
	Duration   time.Duration `json:"duration"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// TextBlock represents a block of recognized text.
type TextBlock struct {
	Text       string    `json:"text"`
	Confidence float64   `json:"confidence"`
	BoundingBox BoundingBox `json:"bounding_box"`
	Lines      []TextLine `json:"lines,omitempty"`
}

// TextLine represents a line of recognized text.
type TextLine struct {
	Text        string      `json:"text"`
	Confidence  float64     `json:"confidence"`
	BoundingBox BoundingBox `json:"bounding_box"`
	Words       []Word      `json:"words,omitempty"`
}

// Word represents a recognized word.
type Word struct {
	Text        string      `json:"text"`
	Confidence  float64     `json:"confidence"`
	BoundingBox BoundingBox `json:"bounding_box"`
}

// BoundingBox represents a rectangular bounding box.
type BoundingBox struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

// TesseractEngine implements OCR using Tesseract.
type TesseractEngine struct {
	config      *config.OCRConfig
	tesseractPath string
	mu          sync.Mutex
	tempDir     string
}

// NewTesseractEngine creates a new Tesseract OCR engine.
func NewTesseractEngine(cfg *config.OCRConfig) (*TesseractEngine, error) {
	tesseractPath := cfg.TesseractPath
	if tesseractPath == "" {
		tesseractPath = "tesseract"
	}

	// Check if tesseract is available
	if _, err := exec.LookPath(tesseractPath); err != nil {
		return nil, fmt.Errorf("tesseract not found at %s: %w", tesseractPath, err)
	}

	// Create temp directory
	tempDir, err := os.MkdirTemp("", "ocr-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	return &TesseractEngine{
		config:        cfg,
		tesseractPath: tesseractPath,
		tempDir:       tempDir,
	}, nil
}

// Recognize performs OCR on the given image data.
func (e *TesseractEngine) Recognize(ctx context.Context, imageData []byte, opts *RecognizeOptions) (*OCRResult, error) {
	if opts == nil {
		opts = &RecognizeOptions{
			Language: e.config.Language,
		}
	}

	// Write image to temp file
	e.mu.Lock()
	tempFile := filepath.Join(e.tempDir, fmt.Sprintf("ocr_%d.png", time.Now().UnixNano()))
	e.mu.Unlock()

	if err := os.WriteFile(tempFile, imageData, 0644); err != nil {
		return nil, fmt.Errorf("failed to write temp image: %w", err)
	}
	defer os.Remove(tempFile)

	return e.RecognizeFile(ctx, tempFile, opts)
}

// RecognizeFile performs OCR on the given image file.
func (e *TesseractEngine) RecognizeFile(ctx context.Context, filePath string, opts *RecognizeOptions) (*OCRResult, error) {
	start := time.Now()

	if opts == nil {
		opts = &RecognizeOptions{
			Language: e.config.Language,
		}
	}

	// Build tesseract command
	args := []string{filePath, "stdout"}

	// Add language
	if opts.Language != "" {
		args = append(args, "-l", opts.Language)
	} else if e.config.Language != "" {
		args = append(args, "-l", e.config.Language)
	}

	// Add PSM (Page Segmentation Mode)
	if opts.PSM > 0 {
		args = append(args, "--psm", fmt.Sprintf("%d", opts.PSM))
	} else {
		args = append(args, "--psm", "3") // Default: fully automatic page segmentation
	}

	// Add OEM (OCR Engine Mode)
	if opts.OEM > 0 {
		args = append(args, "--oem", fmt.Sprintf("%d", opts.OEM))
	}

	// Add whitelist
	if opts.Whitelist != "" {
		args = append(args, "-c", fmt.Sprintf("tessedit_char_whitelist=%s", opts.Whitelist))
	}

	// Run tesseract
	cmd := exec.CommandContext(ctx, e.tesseractPath, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("tesseract failed: %w, stderr: %s", err, stderr.String())
	}

	text := strings.TrimSpace(stdout.String())

	return &OCRResult{
		Text:       text,
		Confidence: 0.0, // Tesseract stdout mode doesn't provide confidence
		Language:   opts.Language,
		Duration:   time.Since(start),
		Metadata: map[string]interface{}{
			"engine":   "tesseract",
			"file":     filePath,
			"args":     args,
		},
	}, nil
}

// Close releases resources.
func (e *TesseractEngine) Close() error {
	if e.tempDir != "" {
		return os.RemoveAll(e.tempDir)
	}
	return nil
}

// APIEngine implements OCR using cloud APIs.
type APIEngine struct {
	config   *config.OCRConfig
	provider string
	client   OCRAPIClient
}

// OCRAPIClient represents an OCR API client interface.
type OCRAPIClient interface {
	Recognize(ctx context.Context, imageData []byte, opts *RecognizeOptions) (*OCRResult, error)
}

// NewAPIEngine creates a new API-based OCR engine.
func NewAPIEngine(cfg *config.OCRConfig, provider string) (*APIEngine, error) {
	apiCfg, ok := cfg.APIProviders[provider]
	if !ok || !apiCfg.Enabled {
		return nil, fmt.Errorf("OCR API provider not available: %s", provider)
	}

	var client OCRAPIClient
	switch provider {
	case "baidu":
		client = NewBaiduOCRClient(apiCfg)
	case "tencent":
		client = NewTencentOCRClient(apiCfg)
	case "aliyun":
		client = NewAliyunOCRClient(apiCfg)
	default:
		return nil, fmt.Errorf("unsupported OCR API provider: %s", provider)
	}

	return &APIEngine{
		config:   cfg,
		provider: provider,
		client:   client,
	}, nil
}

// Recognize performs OCR using the API.
func (e *APIEngine) Recognize(ctx context.Context, imageData []byte, opts *RecognizeOptions) (*OCRResult, error) {
	return e.client.Recognize(ctx, imageData, opts)
}

// RecognizeFile performs OCR on the given image file.
func (e *APIEngine) RecognizeFile(ctx context.Context, filePath string, opts *RecognizeOptions) (*OCRResult, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read image file: %w", err)
	}
	return e.Recognize(ctx, data, opts)
}

// Close releases resources.
func (e *APIEngine) Close() error {
	return nil
}

// BaiduOCRClient implements Baidu OCR API.
type BaiduOCRClient struct {
	config *config.OCRAPIConfig
}

// NewBaiduOCRClient creates a new Baidu OCR client.
func NewBaiduOCRClient(cfg *config.OCRAPIConfig) *BaiduOCRClient {
	return &BaiduOCRClient{config: cfg}
}

// Recognize performs OCR using Baidu API.
func (c *BaiduOCRClient) Recognize(ctx context.Context, imageData []byte, opts *RecognizeOptions) (*OCRResult, error) {
	// TODO: Implement Baidu OCR API
	// This is a placeholder implementation
	return &OCRResult{
		Text: "",
		Metadata: map[string]interface{}{
			"engine":   "baidu",
			"provider": "baidu",
		},
	}, fmt.Errorf("Baidu OCR API not implemented")
}

// TencentOCRClient implements Tencent OCR API.
type TencentOCRClient struct {
	config *config.OCRAPIConfig
}

// NewTencentOCRClient creates a new Tencent OCR client.
func NewTencentOCRClient(cfg *config.OCRAPIConfig) *TencentOCRClient {
	return &TencentOCRClient{config: cfg}
}

// Recognize performs OCR using Tencent API.
func (c *TencentOCRClient) Recognize(ctx context.Context, imageData []byte, opts *RecognizeOptions) (*OCRResult, error) {
	// TODO: Implement Tencent OCR API
	return &OCRResult{
		Text: "",
		Metadata: map[string]interface{}{
			"engine":   "tencent",
			"provider": "tencent",
		},
	}, fmt.Errorf("Tencent OCR API not implemented")
}

// AliyunOCRClient implements Aliyun OCR API.
type AliyunOCRClient struct {
	config *config.OCRAPIConfig
}

// NewAliyunOCRClient creates a new Aliyun OCR client.
func NewAliyunOCRClient(cfg *config.OCRAPIConfig) *AliyunOCRClient {
	return &AliyunOCRClient{config: cfg}
}

// Recognize performs OCR using Aliyun API.
func (c *AliyunOCRClient) Recognize(ctx context.Context, imageData []byte, opts *RecognizeOptions) (*OCRResult, error) {
	// TODO: Implement Aliyun OCR API
	return &OCRResult{
		Text: "",
		Metadata: map[string]interface{}{
			"engine":   "aliyun",
			"provider": "aliyun",
		},
	}, fmt.Errorf("Aliyun OCR API not implemented")
}

// OCRManager manages multiple OCR engines.
type OCRManager struct {
	config       *config.OCRConfig
	engines      map[string]OCREngine
	defaultEngine OCREngine
	mu           sync.RWMutex
	semaphore    chan struct{}
}

// NewOCRManager creates a new OCR manager.
func NewOCRManager(cfg *config.OCRConfig) (*OCRManager, error) {
	manager := &OCRManager{
		config:    cfg,
		engines:   make(map[string]OCREngine),
		semaphore: make(chan struct{}, cfg.MaxConcurrency),
	}

	// Initialize default engine
	var err error
	switch cfg.Engine {
	case "tesseract":
		manager.defaultEngine, err = NewTesseractEngine(cfg)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize tesseract engine: %w", err)
		}
		manager.engines["tesseract"] = manager.defaultEngine
	case "api":
		// Find first enabled API provider
		for name, apiCfg := range cfg.APIProviders {
			if apiCfg.Enabled {
				manager.defaultEngine, err = NewAPIEngine(cfg, name)
				if err != nil {
					continue
				}
				manager.engines[name] = manager.defaultEngine
				break
			}
		}
		if manager.defaultEngine == nil {
			return nil, fmt.Errorf("no OCR API provider available")
		}
	default:
		return nil, fmt.Errorf("unsupported OCR engine: %s", cfg.Engine)
	}

	return manager, nil
}

// Recognize performs OCR using the default engine.
func (m *OCRManager) Recognize(ctx context.Context, imageData []byte, opts *RecognizeOptions) (*OCRResult, error) {
	// Acquire semaphore
	select {
	case m.semaphore <- struct{}{}:
		defer func() { <-m.semaphore }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// Validate image
	if err := m.validateImage(imageData); err != nil {
		return nil, err
	}

	return m.defaultEngine.Recognize(ctx, imageData, opts)
}

// RecognizeFile performs OCR on a file using the default engine.
func (m *OCRManager) RecognizeFile(ctx context.Context, filePath string, opts *RecognizeOptions) (*OCRResult, error) {
	// Acquire semaphore
	select {
	case m.semaphore <- struct{}{}:
		defer func() { <-m.semaphore }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// Check file exists and is valid
	info, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("file not found: %w", err)
	}

	if info.Size() > m.config.ImageConfig.MaxFileSize {
		return nil, fmt.Errorf("file size %d exceeds maximum %d", info.Size(), m.config.ImageConfig.MaxFileSize)
	}

	return m.defaultEngine.RecognizeFile(ctx, filePath, opts)
}

// RecognizeWithEngine performs OCR using a specific engine.
func (m *OCRManager) RecognizeWithEngine(ctx context.Context, engineName string, imageData []byte, opts *RecognizeOptions) (*OCRResult, error) {
	m.mu.RLock()
	engine, ok := m.engines[engineName]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("engine not found: %s", engineName)
	}

	// Acquire semaphore
	select {
	case m.semaphore <- struct{}{}:
		defer func() { <-m.semaphore }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	return engine.Recognize(ctx, imageData, opts)
}

// validateImage validates image data.
func (m *OCRManager) validateImage(data []byte) error {
	if len(data) == 0 {
		return fmt.Errorf("empty image data")
	}

	if int64(len(data)) > m.config.ImageConfig.MaxFileSize {
		return fmt.Errorf("image size %d exceeds maximum %d", len(data), m.config.ImageConfig.MaxFileSize)
	}

	// Try to decode image to validate format
	reader := bytes.NewReader(data)
	img, format, err := image.DecodeConfig(reader)
	if err != nil {
		return fmt.Errorf("invalid image format: %w", err)
	}

	// Check dimensions
	if img.Width > m.config.ImageConfig.MaxWidth || img.Height > m.config.ImageConfig.MaxHeight {
		return fmt.Errorf("image dimensions %dx%d exceed maximum %dx%d",
			img.Width, img.Height, m.config.ImageConfig.MaxWidth, m.config.ImageConfig.MaxHeight)
	}

	// Check format
	supported := false
	for _, t := range m.config.ImageConfig.SupportedTypes {
		if strings.EqualFold(format, t) {
			supported = true
			break
		}
	}
	if !supported {
		return fmt.Errorf("unsupported image format: %s", format)
	}

	return nil
}

// Close releases all resources.
func (m *OCRManager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var errs []error
	for _, engine := range m.engines {
		if err := engine.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors closing engines: %v", errs)
	}
	return nil
}

// ReadImageFromBase64 reads image data from a base64 string.
func ReadImageFromBase64(data string) ([]byte, error) {
	// Remove data URL prefix if present
	if idx := strings.Index(data, ","); idx != -1 {
		data = data[idx+1:]
	}

	return base64.StdEncoding.DecodeString(data)
}

// ReadImageFromReader reads image data from a reader.
func ReadImageFromReader(r io.Reader) ([]byte, error) {
	return io.ReadAll(r)
}

// EncodeImageToBase64 encodes image data to base64.
func EncodeImageToBase64(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

// OCRResultToJSON converts OCR result to JSON.
func OCRResultToJSON(result *OCRResult) (string, error) {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}
