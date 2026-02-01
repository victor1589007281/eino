package ocr

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/eino/vdocsOCR/config"
)

// Test helper functions

func createTestImage(t *testing.T, width, height int) []byte {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	// Fill with white
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.White)
		}
	}

	// Create temp file
	tmpFile, err := os.CreateTemp("", "test-image-*.png")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer tmpFile.Close()

	if err := png.Encode(tmpFile, img); err != nil {
		t.Fatalf("Failed to encode PNG: %v", err)
	}

	// Read back
	data, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to read image file: %v", err)
	}

	os.Remove(tmpFile.Name())
	return data
}

func createTestConfig() *config.OCRConfig {
	return &config.OCRConfig{
		Engine:         "tesseract",
		Language:       "eng",
		TesseractPath:  "tesseract",
		MaxConcurrency: 2,
		Timeout:        30 * time.Second,
		ImageConfig: config.ImageConfig{
			MaxWidth:       4096,
			MaxHeight:      4096,
			MaxFileSize:    50 * 1024 * 1024,
			SupportedTypes: []string{"png", "jpg", "jpeg", "gif"},
			Preprocessing:  true,
			DPI:            300,
		},
		APIProviders: map[string]*config.OCRAPIConfig{},
	}
}

// Tests

func TestRecognizeOptions(t *testing.T) {
	opts := &RecognizeOptions{
		Language:   "chi_sim+eng",
		DPI:        300,
		PSM:        3,
		OEM:        1,
		Whitelist:  "0123456789",
		Preprocess: true,
		OutputType: "text",
	}

	if opts.Language != "chi_sim+eng" {
		t.Errorf("Expected language chi_sim+eng, got %s", opts.Language)
	}
	if opts.DPI != 300 {
		t.Errorf("Expected DPI 300, got %d", opts.DPI)
	}
	if opts.PSM != 3 {
		t.Errorf("Expected PSM 3, got %d", opts.PSM)
	}
}

func TestOCRResult(t *testing.T) {
	result := &OCRResult{
		Text:       "Test text",
		Confidence: 0.95,
		Language:   "eng",
		Duration:   100 * time.Millisecond,
		Metadata: map[string]interface{}{
			"engine": "tesseract",
		},
	}

	if result.Text != "Test text" {
		t.Errorf("Expected text 'Test text', got '%s'", result.Text)
	}
	if result.Confidence != 0.95 {
		t.Errorf("Expected confidence 0.95, got %f", result.Confidence)
	}
}

func TestTextBlock(t *testing.T) {
	block := TextBlock{
		Text:       "Block text",
		Confidence: 0.9,
		BoundingBox: BoundingBox{
			X:      10,
			Y:      20,
			Width:  100,
			Height: 50,
		},
	}

	if block.BoundingBox.Width != 100 {
		t.Errorf("Expected width 100, got %d", block.BoundingBox.Width)
	}
}

func TestReadImageFromBase64(t *testing.T) {
	// Create test image
	imgData := createTestImage(t, 100, 100)
	encoded := base64.StdEncoding.EncodeToString(imgData)

	// Test without prefix
	decoded, err := ReadImageFromBase64(encoded)
	if err != nil {
		t.Fatalf("Failed to decode base64: %v", err)
	}
	if len(decoded) != len(imgData) {
		t.Errorf("Decoded length mismatch: expected %d, got %d", len(imgData), len(decoded))
	}

	// Test with data URL prefix
	dataURL := "data:image/png;base64," + encoded
	decoded2, err := ReadImageFromBase64(dataURL)
	if err != nil {
		t.Fatalf("Failed to decode data URL: %v", err)
	}
	if len(decoded2) != len(imgData) {
		t.Errorf("Decoded length mismatch: expected %d, got %d", len(imgData), len(decoded2))
	}
}

func TestEncodeImageToBase64(t *testing.T) {
	imgData := createTestImage(t, 100, 100)
	encoded := EncodeImageToBase64(imgData)

	if encoded == "" {
		t.Error("Encoded string should not be empty")
	}

	// Verify it can be decoded
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("Failed to decode: %v", err)
	}
	if len(decoded) != len(imgData) {
		t.Errorf("Round-trip failed: expected %d bytes, got %d", len(imgData), len(decoded))
	}
}

func TestOCRResultToJSON(t *testing.T) {
	result := &OCRResult{
		Text:       "Test text",
		Confidence: 0.95,
		Language:   "eng",
		Duration:   100 * time.Millisecond,
		Blocks: []TextBlock{
			{
				Text:       "Block 1",
				Confidence: 0.9,
				BoundingBox: BoundingBox{
					X: 0, Y: 0, Width: 100, Height: 50,
				},
			},
		},
		Metadata: map[string]interface{}{
			"engine": "tesseract",
		},
	}

	jsonStr, err := OCRResultToJSON(result)
	if err != nil {
		t.Fatalf("Failed to convert to JSON: %v", err)
	}

	// Verify JSON structure
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		t.Fatalf("Invalid JSON: %v", err)
	}

	if parsed["text"] != "Test text" {
		t.Error("JSON should contain correct text")
	}
	if parsed["confidence"] != 0.95 {
		t.Error("JSON should contain correct confidence")
	}
}

func TestOCRManagerValidateImage(t *testing.T) {
	cfg := createTestConfig()
	
	// Create manager (may fail if tesseract not installed, skip gracefully)
	manager, err := NewOCRManager(cfg)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			t.Skip("Tesseract not installed, skipping test")
		}
		t.Fatalf("Failed to create manager: %v", err)
	}
	defer manager.Close()

	// Test valid image
	validImg := createTestImage(t, 100, 100)
	err = manager.validateImage(validImg)
	if err != nil {
		t.Errorf("Valid image should pass validation: %v", err)
	}

	// Test empty image
	err = manager.validateImage([]byte{})
	if err == nil {
		t.Error("Empty image should fail validation")
	}

	// Test oversized image data
	oversized := make([]byte, cfg.ImageConfig.MaxFileSize+1)
	err = manager.validateImage(oversized)
	if err == nil {
		t.Error("Oversized image should fail validation")
	}
}

func TestTesseractEngineCreation(t *testing.T) {
	cfg := createTestConfig()

	engine, err := NewTesseractEngine(cfg)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			t.Skip("Tesseract not installed, skipping test")
		}
		t.Fatalf("Failed to create tesseract engine: %v", err)
	}
	defer engine.Close()

	if engine.tesseractPath == "" {
		t.Error("Tesseract path should be set")
	}
	if engine.tempDir == "" {
		t.Error("Temp directory should be set")
	}
}

func TestTesseractEngineRecognizeFile(t *testing.T) {
	cfg := createTestConfig()

	engine, err := NewTesseractEngine(cfg)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			t.Skip("Tesseract not installed, skipping test")
		}
		t.Fatalf("Failed to create tesseract engine: %v", err)
	}
	defer engine.Close()

	// Create test image file
	imgData := createTestImage(t, 100, 100)
	tmpFile := filepath.Join(t.TempDir(), "test.png")
	if err := os.WriteFile(tmpFile, imgData, 0644); err != nil {
		t.Fatalf("Failed to write test image: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := engine.RecognizeFile(ctx, tmpFile, &RecognizeOptions{
		Language: "eng",
	})
	if err != nil {
		// May fail due to no text in image, but shouldn't error
		t.Logf("Recognition returned error (expected for blank image): %v", err)
	}

	if result != nil {
		if result.Metadata == nil {
			t.Error("Result should have metadata")
		}
	}
}

func TestAPIEngineCreation(t *testing.T) {
	cfg := createTestConfig()
	cfg.APIProviders["baidu"] = &config.OCRAPIConfig{
		Enabled:  true,
		Provider: "baidu",
		APIKey:   "test-key",
	}

	engine, err := NewAPIEngine(cfg, "baidu")
	if err != nil {
		t.Fatalf("Failed to create API engine: %v", err)
	}
	defer engine.Close()

	if engine.provider != "baidu" {
		t.Errorf("Expected provider baidu, got %s", engine.provider)
	}
}

func TestAPIEngineNotEnabled(t *testing.T) {
	cfg := createTestConfig()
	cfg.APIProviders["baidu"] = &config.OCRAPIConfig{
		Enabled:  false,
		Provider: "baidu",
	}

	_, err := NewAPIEngine(cfg, "baidu")
	if err == nil {
		t.Error("Should fail for disabled provider")
	}
}

func TestAPIEngineNotFound(t *testing.T) {
	cfg := createTestConfig()

	_, err := NewAPIEngine(cfg, "nonexistent")
	if err == nil {
		t.Error("Should fail for non-existent provider")
	}
}

func TestBaiduOCRClient(t *testing.T) {
	cfg := &config.OCRAPIConfig{
		Enabled:  true,
		Provider: "baidu",
		APIKey:   "test-key",
	}

	client := NewBaiduOCRClient(cfg)
	
	ctx := context.Background()
	imgData := createTestImage(t, 100, 100)

	// This should fail gracefully as API is not implemented
	_, err := client.Recognize(ctx, imgData, nil)
	if err == nil {
		t.Error("Unimplemented API should return error")
	}
}

func TestOCRManagerConcurrency(t *testing.T) {
	cfg := createTestConfig()
	cfg.MaxConcurrency = 2

	manager, err := NewOCRManager(cfg)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			t.Skip("Tesseract not installed, skipping test")
		}
		t.Fatalf("Failed to create manager: %v", err)
	}
	defer manager.Close()

	// Verify semaphore is created with correct capacity
	if cap(manager.semaphore) != 2 {
		t.Errorf("Expected semaphore capacity 2, got %d", cap(manager.semaphore))
	}
}

func TestRegion(t *testing.T) {
	region := Region{
		X:      10,
		Y:      20,
		Width:  100,
		Height: 50,
	}

	if region.X != 10 || region.Y != 20 {
		t.Error("Region coordinates incorrect")
	}
	if region.Width != 100 || region.Height != 50 {
		t.Error("Region dimensions incorrect")
	}
}

func TestTextLine(t *testing.T) {
	line := TextLine{
		Text:       "Test line",
		Confidence: 0.85,
		BoundingBox: BoundingBox{
			X: 0, Y: 0, Width: 200, Height: 20,
		},
		Words: []Word{
			{Text: "Test", Confidence: 0.9},
			{Text: "line", Confidence: 0.8},
		},
	}

	if len(line.Words) != 2 {
		t.Errorf("Expected 2 words, got %d", len(line.Words))
	}
	if line.Words[0].Text != "Test" {
		t.Error("First word should be 'Test'")
	}
}

func TestWord(t *testing.T) {
	word := Word{
		Text:       "Hello",
		Confidence: 0.95,
		BoundingBox: BoundingBox{
			X: 0, Y: 0, Width: 50, Height: 20,
		},
	}

	if word.Text != "Hello" {
		t.Errorf("Expected word 'Hello', got '%s'", word.Text)
	}
}

func TestBoundingBox(t *testing.T) {
	bbox := BoundingBox{
		X:      10,
		Y:      20,
		Width:  100,
		Height: 50,
	}

	// Test basic properties
	if bbox.X != 10 {
		t.Errorf("Expected X=10, got %d", bbox.X)
	}
	if bbox.Y != 20 {
		t.Errorf("Expected Y=20, got %d", bbox.Y)
	}
	
	// Calculate right and bottom
	right := bbox.X + bbox.Width
	bottom := bbox.Y + bbox.Height
	
	if right != 110 {
		t.Errorf("Expected right=110, got %d", right)
	}
	if bottom != 70 {
		t.Errorf("Expected bottom=70, got %d", bottom)
	}
}

func TestOCRManagerClose(t *testing.T) {
	cfg := createTestConfig()

	manager, err := NewOCRManager(cfg)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			t.Skip("Tesseract not installed, skipping test")
		}
		t.Fatalf("Failed to create manager: %v", err)
	}

	err = manager.Close()
	if err != nil {
		t.Errorf("Close should not return error: %v", err)
	}
}

func TestTesseractEngineClose(t *testing.T) {
	cfg := createTestConfig()

	engine, err := NewTesseractEngine(cfg)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			t.Skip("Tesseract not installed, skipping test")
		}
		t.Fatalf("Failed to create engine: %v", err)
	}

	// Verify temp dir exists
	if _, err := os.Stat(engine.tempDir); os.IsNotExist(err) {
		t.Error("Temp dir should exist before close")
	}

	err = engine.Close()
	if err != nil {
		t.Errorf("Close should not return error: %v", err)
	}

	// Verify temp dir is removed
	if _, err := os.Stat(engine.tempDir); !os.IsNotExist(err) {
		t.Error("Temp dir should be removed after close")
	}
}
