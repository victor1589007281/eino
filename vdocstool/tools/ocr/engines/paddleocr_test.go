package engines

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cloudwego/eino/vdocstool/tools/ocr"
)

func TestDefaultPaddleOCRConfig(t *testing.T) {
	config := DefaultPaddleOCRConfig()
	if config.Endpoint == "" {
		t.Error("expected default endpoint")
	}
	if config.Timeout == 0 {
		t.Error("expected default timeout")
	}
}

func TestNewPaddleOCREngine(t *testing.T) {
	// 测试 nil 配置
	engine := NewPaddleOCREngine(nil)
	if engine == nil {
		t.Fatal("expected engine")
	}
	if engine.Name() != ocr.EnginePaddleOCR {
		t.Errorf("expected paddleocr, got %s", engine.Name())
	}

	// 测试自定义配置
	config := &PaddleOCRConfig{
		Endpoint: "http://custom:8866",
		UseGPU:   true,
	}
	engine = NewPaddleOCREngine(config)
	if engine.config.Endpoint != "http://custom:8866" {
		t.Errorf("expected custom endpoint")
	}
}

func TestPaddleOCREngineProperties(t *testing.T) {
	engine := NewPaddleOCREngine(nil)

	if engine.Name() != ocr.EnginePaddleOCR {
		t.Errorf("expected paddleocr")
	}

	if engine.Layer() != ocr.LayerSmallModel {
		t.Errorf("expected small model layer")
	}

	scenes := engine.SupportedScenes()
	if len(scenes) == 0 {
		t.Error("expected supported scenes")
	}
}

func TestPaddleOCREngineMemoryUsage(t *testing.T) {
	// CPU 模式
	engine := NewPaddleOCREngine(&PaddleOCRConfig{UseGPU: false})
	cpuMemory := engine.MemoryUsage()
	if cpuMemory <= 0 {
		t.Error("expected positive memory usage for CPU mode")
	}

	// GPU 模式
	engine = NewPaddleOCREngine(&PaddleOCRConfig{UseGPU: true})
	gpuMemory := engine.MemoryUsage()
	if gpuMemory <= cpuMemory {
		t.Error("expected GPU mode to use more memory")
	}
}

func TestPaddleOCREngineLoadUnload(t *testing.T) {
	engine := NewPaddleOCREngine(nil)

	if engine.IsLoaded() {
		t.Error("should not be loaded initially")
	}

	err := engine.Unload()
	if err != nil {
		t.Errorf("unload should not fail: %v", err)
	}
}

func TestPaddleOCREngineStats(t *testing.T) {
	engine := NewPaddleOCREngine(nil)

	stats := engine.Stats()
	if stats.Engine != ocr.EnginePaddleOCR {
		t.Errorf("expected paddleocr")
	}
	if stats.TotalCalls != 0 {
		t.Errorf("expected 0 total calls")
	}
}

func TestPaddleOCREngineWithMockServer(t *testing.T) {
	// 创建 mock 服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.WriteHeader(http.StatusOK)
		case "/predict/ocr_system":
			resp := paddleOCRResponse{
				Status: "000",
				Results: []struct {
					Text       string      `json:"text"`
					Confidence float64     `json:"confidence"`
					TextRegion [][]float64 `json:"text_region"`
				}{
					{Text: "Hello", Confidence: 0.95, TextRegion: [][]float64{{0, 0}, {100, 0}, {100, 30}, {0, 30}}},
					{Text: "World", Confidence: 0.92, TextRegion: [][]float64{{0, 40}, {100, 40}, {100, 70}, {0, 70}}},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	config := &PaddleOCRConfig{
		Endpoint: server.URL,
	}
	engine := NewPaddleOCREngine(config)

	// 测试健康检查
	err := engine.Health(context.Background())
	if err != nil {
		t.Errorf("health check failed: %v", err)
	}
}

func TestContainsAny(t *testing.T) {
	tests := []struct {
		text     string
		keywords []string
		expected bool
	}{
		{"发票代码: 123", []string{"发票代码", "Invoice Code"}, true},
		{"Invoice Code: 456", []string{"发票代码", "Invoice Code"}, true},
		{"Hello World", []string{"发票代码", "Invoice Code"}, false},
		{"", []string{"test"}, false},
	}

	for _, tc := range tests {
		result := containsAny(tc.text, tc.keywords...)
		if result != tc.expected {
			t.Errorf("containsAny(%q, %v) = %v, expected %v", tc.text, tc.keywords, result, tc.expected)
		}
	}
}

func TestExtractValue(t *testing.T) {
	tests := []struct {
		text     string
		expected string
	}{
		{"发票代码: 123456", "123456"},
		{"Code: 100.00", "100.00"},      // 使用英文冒号测试
		{"NoColon", "NoColon"},
	}

	for _, tc := range tests {
		result := extractValue(tc.text)
		if result != tc.expected {
			t.Errorf("extractValue(%q) = %q, expected %q", tc.text, result, tc.expected)
		}
	}
}

func TestTrimSpace(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"  hello  ", "hello"},
		{"\tworld\t", "world"},
		{"no space", "no space"},
		{"", ""},
	}

	for _, tc := range tests {
		result := trimSpace(tc.input)
		if result != tc.expected {
			t.Errorf("trimSpace(%q) = %q, expected %q", tc.input, result, tc.expected)
		}
	}
}
