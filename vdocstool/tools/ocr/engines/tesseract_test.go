package engines

import (
	"context"
	"testing"
	"time"

	"github.com/cloudwego/eino/vdocstool/tools/ocr"
)

func TestDefaultTesseractConfig(t *testing.T) {
	config := DefaultTesseractConfig()

	if config.TesseractPath != "tesseract" {
		t.Errorf("expected TesseractPath=tesseract, got %s", config.TesseractPath)
	}

	if len(config.Languages) != 2 {
		t.Errorf("expected 2 languages, got %d", len(config.Languages))
	}

	if config.PSM != 3 {
		t.Errorf("expected PSM=3, got %d", config.PSM)
	}

	if config.OEM != 3 {
		t.Errorf("expected OEM=3, got %d", config.OEM)
	}

	if config.DPI != 300 {
		t.Errorf("expected DPI=300, got %d", config.DPI)
	}

	if config.Timeout != 30*time.Second {
		t.Errorf("expected Timeout=30s, got %v", config.Timeout)
	}
}

func TestNewTesseractEngine(t *testing.T) {
	// 测试默认配置
	engine := NewTesseractEngine(nil)
	if engine == nil {
		t.Fatal("NewTesseractEngine returned nil")
	}

	// 测试自定义配置
	config := &TesseractConfig{
		TesseractPath: "/usr/local/bin/tesseract",
		Languages:     []string{"eng"},
		PSM:           6,
	}
	engine = NewTesseractEngine(config)
	if engine.config.PSM != 6 {
		t.Errorf("expected PSM=6, got %d", engine.config.PSM)
	}
}

func TestTesseractEngineName(t *testing.T) {
	engine := NewTesseractEngine(nil)

	if engine.Name() != ocr.EngineTesseract {
		t.Errorf("expected name=%s, got %s", ocr.EngineTesseract, engine.Name())
	}
}

func TestTesseractEngineLayer(t *testing.T) {
	engine := NewTesseractEngine(nil)

	if engine.Layer() != ocr.LayerTraditional {
		t.Errorf("expected layer=%d, got %d", ocr.LayerTraditional, engine.Layer())
	}
}

func TestTesseractEngineSupportedScenes(t *testing.T) {
	engine := NewTesseractEngine(nil)
	scenes := engine.SupportedScenes()

	if len(scenes) == 0 {
		t.Error("expected at least one supported scene")
	}

	// 检查是否包含 General 场景
	hasGeneral := false
	for _, scene := range scenes {
		if scene == ocr.SceneGeneral {
			hasGeneral = true
			break
		}
	}
	if !hasGeneral {
		t.Error("expected to support General scene")
	}
}

func TestTesseractEngineMemoryUsage(t *testing.T) {
	engine := NewTesseractEngine(nil)

	usage := engine.MemoryUsage()
	if usage <= 0 {
		t.Errorf("expected positive memory usage, got %d", usage)
	}
}

func TestTesseractEngineIsLoaded(t *testing.T) {
	engine := NewTesseractEngine(nil)

	if engine.IsLoaded() {
		t.Error("engine should not be loaded initially")
	}
}

func TestTesseractEngineUnload(t *testing.T) {
	engine := NewTesseractEngine(nil)
	engine.loaded = true

	err := engine.Unload()
	if err != nil {
		t.Errorf("Unload failed: %v", err)
	}

	if engine.IsLoaded() {
		t.Error("engine should not be loaded after Unload")
	}
}

func TestTesseractConvertLanguages(t *testing.T) {
	engine := NewTesseractEngine(nil)

	tests := []struct {
		input    []string
		expected []string
	}{
		{[]string{"zh"}, []string{"chi_sim"}},
		{[]string{"en"}, []string{"eng"}},
		{[]string{"zh", "en"}, []string{"chi_sim", "eng"}},
		{[]string{"zh-cn"}, []string{"chi_sim"}},
		{[]string{"zh-tw"}, []string{"chi_tra"}},
		{[]string{"ja"}, []string{"jpn"}},
		{[]string{"ko"}, []string{"kor"}},
		{[]string{"unknown"}, []string{"unknown"}},
	}

	for _, tt := range tests {
		result := engine.convertLanguages(tt.input)
		if len(result) != len(tt.expected) {
			t.Errorf("convertLanguages(%v): expected %v, got %v", tt.input, tt.expected, result)
			continue
		}
		for i, v := range result {
			if v != tt.expected[i] {
				t.Errorf("convertLanguages(%v)[%d]: expected %s, got %s", tt.input, i, tt.expected[i], v)
			}
		}
	}
}

func TestTesseractBuildArgs(t *testing.T) {
	engine := NewTesseractEngine(&TesseractConfig{
		Languages: []string{"chi_sim", "eng"},
		PSM:       3,
		OEM:       3,
		DPI:       300,
	})

	req := &ocr.OCRRequest{
		ImagePath: "/test/image.jpg",
	}

	args := engine.buildArgs(req, "/test/image.jpg")

	// 检查基本参数
	if args[0] != "/test/image.jpg" {
		t.Errorf("expected image path as first arg, got %s", args[0])
	}
	if args[1] != "stdout" {
		t.Errorf("expected stdout as second arg, got %s", args[1])
	}

	// 检查语言参数
	hasLang := false
	for i, arg := range args {
		if arg == "-l" && i+1 < len(args) {
			hasLang = true
			if args[i+1] != "chi_sim+eng" {
				t.Errorf("expected languages=chi_sim+eng, got %s", args[i+1])
			}
			break
		}
	}
	if !hasLang {
		t.Error("missing language parameter")
	}
}

func TestTesseractBuildArgsWithCustomLanguages(t *testing.T) {
	engine := NewTesseractEngine(nil)

	req := &ocr.OCRRequest{
		ImagePath: "/test/image.jpg",
		Languages: []string{"ja", "ko"},
	}

	args := engine.buildArgs(req, "/test/image.jpg")

	// 检查语言参数
	for i, arg := range args {
		if arg == "-l" && i+1 < len(args) {
			if args[i+1] != "jpn+kor" {
				t.Errorf("expected languages=jpn+kor, got %s", args[i+1])
			}
			break
		}
	}
}

func TestTesseractParseBlocks(t *testing.T) {
	engine := NewTesseractEngine(nil)

	text := "Line 1\nLine 2\n\nLine 3"
	blocks := engine.parseBlocks(text)

	if len(blocks) != 3 {
		t.Errorf("expected 3 blocks, got %d", len(blocks))
	}

	if blocks[0].Text != "Line 1" {
		t.Errorf("expected 'Line 1', got '%s'", blocks[0].Text)
	}
}

func TestTesseractEstimateConfidence(t *testing.T) {
	engine := NewTesseractEngine(nil)

	tests := []struct {
		text        string
		minExpected float64
		maxExpected float64
	}{
		{"", 0.0, 0.0},
		{"Hello World", 0.5, 1.0},
		{"你好世界", 0.5, 1.0},
		{"Hello 你好", 0.5, 1.0},
		{"!@#$%^&*()", 0.0, 0.5},
	}

	for _, tt := range tests {
		conf := engine.estimateConfidence(tt.text)
		if conf < tt.minExpected || conf > tt.maxExpected {
			t.Errorf("estimateConfidence(%q): expected [%f, %f], got %f",
				tt.text, tt.minExpected, tt.maxExpected, conf)
		}
	}
}

func TestTesseractStats(t *testing.T) {
	engine := NewTesseractEngine(nil)
	engine.totalCalls = 10
	engine.failedCalls = 2
	engine.totalLatency = 1000
	engine.loaded = true
	engine.lastUsed = time.Now()

	stats := engine.Stats()

	if stats.Engine != ocr.EngineTesseract {
		t.Errorf("expected engine=%s, got %s", ocr.EngineTesseract, stats.Engine)
	}

	if stats.TotalCalls != 10 {
		t.Errorf("expected TotalCalls=10, got %d", stats.TotalCalls)
	}

	if stats.FailedCalls != 2 {
		t.Errorf("expected FailedCalls=2, got %d", stats.FailedCalls)
	}

	if stats.AvgLatency != 100.0 {
		t.Errorf("expected AvgLatency=100.0, got %f", stats.AvgLatency)
	}

	if !stats.Loaded {
		t.Error("expected Loaded=true")
	}
}

func TestTesseractRecordFailure(t *testing.T) {
	engine := NewTesseractEngine(nil)

	if engine.failedCalls != 0 {
		t.Errorf("expected initial failedCalls=0, got %d", engine.failedCalls)
	}

	engine.recordFailure()
	if engine.failedCalls != 1 {
		t.Errorf("expected failedCalls=1 after recordFailure, got %d", engine.failedCalls)
	}

	engine.recordFailure()
	if engine.failedCalls != 2 {
		t.Errorf("expected failedCalls=2, got %d", engine.failedCalls)
	}
}

func TestTesseractRecognizeNoImageSource(t *testing.T) {
	engine := NewTesseractEngine(nil)
	ctx := context.Background()

	req := &ocr.OCRRequest{}
	result, err := engine.Recognize(ctx, req)

	if err == nil {
		t.Error("expected error for empty request")
	}

	if result == nil {
		t.Fatal("result should not be nil even on error")
	}

	if result.Success {
		t.Error("result should not be successful")
	}

	if result.Error == "" {
		t.Error("expected error message in result")
	}
}

func TestTesseractRecognizeTable(t *testing.T) {
	engine := NewTesseractEngine(nil)
	ctx := context.Background()

	req := &ocr.OCRRequest{}
	result, err := engine.RecognizeTable(ctx, req)

	// Tesseract 不直接支持表格，会退化为普通识别
	if result == nil {
		t.Fatal("result should not be nil")
	}

	// 应该返回错误（因为没有图像源）
	if err == nil {
		t.Error("expected error for empty request")
	}
}

func TestTesseractRecognizeInvoice(t *testing.T) {
	engine := NewTesseractEngine(nil)
	ctx := context.Background()

	req := &ocr.OCRRequest{}
	result, err := engine.RecognizeInvoice(ctx, req)

	if result == nil {
		t.Fatal("result should not be nil")
	}

	if err == nil {
		t.Error("expected error for empty request")
	}
}

func TestTesseractRecognizeIDCard(t *testing.T) {
	engine := NewTesseractEngine(nil)
	ctx := context.Background()

	req := &ocr.OCRRequest{}
	result, err := engine.RecognizeIDCard(ctx, req)

	if result == nil {
		t.Fatal("result should not be nil")
	}

	if err == nil {
		t.Error("expected error for empty request")
	}
}

func TestTesseractWithWhitelistBlacklist(t *testing.T) {
	config := &TesseractConfig{
		TesseractPath: "tesseract",
		Languages:     []string{"eng"},
		PSM:           3,
		OEM:           3,
		Whitelist:     "0123456789",
		Blacklist:     "abcdef",
	}
	engine := NewTesseractEngine(config)

	req := &ocr.OCRRequest{
		ImagePath: "/test/image.jpg",
	}

	args := engine.buildArgs(req, "/test/image.jpg")

	// 检查白名单参数
	hasWhitelist := false
	hasBlacklist := false
	for _, arg := range args {
		if arg == "tessedit_char_whitelist=0123456789" {
			hasWhitelist = true
		}
		if arg == "tessedit_char_blacklist=abcdef" {
			hasBlacklist = true
		}
	}

	if !hasWhitelist {
		t.Errorf("expected whitelist parameter, args=%v", args)
	}
	if !hasBlacklist {
		t.Errorf("expected blacklist parameter, args=%v", args)
	}
}

func TestMinFunction(t *testing.T) {
	tests := []struct {
		a, b     float64
		expected float64
	}{
		{1.0, 2.0, 1.0},
		{2.0, 1.0, 1.0},
		{1.5, 1.5, 1.5},
		{-1.0, 1.0, -1.0},
		{0.0, 0.0, 0.0},
	}

	for _, tt := range tests {
		result := min(tt.a, tt.b)
		if result != tt.expected {
			t.Errorf("min(%f, %f): expected %f, got %f", tt.a, tt.b, tt.expected, result)
		}
	}
}
