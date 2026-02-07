package ocr

import (
	"context"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.MaxMemoryMB != 1500 {
		t.Errorf("expected MaxMemoryMB=1500, got %d", config.MaxMemoryMB)
	}

	if config.IdleTimeout != 5*time.Minute {
		t.Errorf("expected IdleTimeout=5m, got %v", config.IdleTimeout)
	}

	if config.DefaultStrategy != StrategySmart {
		t.Errorf("expected DefaultStrategy=smart, got %s", config.DefaultStrategy)
	}

	if len(config.ResidentEngines) != 1 || config.ResidentEngines[0] != EngineRapidOCR {
		t.Errorf("expected ResidentEngines=[rapidocr], got %v", config.ResidentEngines)
	}
}

func TestNewTool(t *testing.T) {
	tool, err := NewTool(nil)
	if err != nil {
		t.Fatalf("NewTool failed: %v", err)
	}

	if tool.config == nil {
		t.Error("config should not be nil")
	}

	if tool.engineManager == nil {
		t.Error("engineManager should not be nil")
	}

	if tool.preprocessor == nil {
		t.Error("preprocessor should not be nil")
	}
}

func TestParseOCRRequest(t *testing.T) {
	tool, _ := NewTool(nil)

	tests := []struct {
		name    string
		params  map[string]interface{}
		wantErr bool
	}{
		{
			name: "valid image path",
			params: map[string]interface{}{
				"image_path": "/path/to/image.jpg",
			},
			wantErr: false,
		},
		{
			name: "valid image url",
			params: map[string]interface{}{
				"image_url": "https://example.com/image.jpg",
			},
			wantErr: false,
		},
		{
			name: "valid base64",
			params: map[string]interface{}{
				"image_base64": "iVBORw0KGgo...",
			},
			wantErr: false,
		},
		{
			name:    "no image source",
			params:  map[string]interface{}{},
			wantErr: true,
		},
		{
			name: "with scene and strategy",
			params: map[string]interface{}{
				"image_path": "/path/to/image.jpg",
				"scene":      "document",
				"strategy":   "quality_first",
			},
			wantErr: false,
		},
		{
			name: "with languages",
			params: map[string]interface{}{
				"image_path": "/path/to/image.jpg",
				"languages":  "zh,en",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := tool.parseOCRRequest(tt.params)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseOCRRequest() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && req == nil {
				t.Error("expected non-nil request")
			}
		})
	}
}

func TestEngineManager(t *testing.T) {
	config := DefaultConfig()
	config.MaxMemoryMB = 500
	manager := NewEngineManager(config)

	// 测试创建
	if manager == nil {
		t.Fatal("NewEngineManager returned nil")
	}

	// 测试获取状态
	status := manager.GetStatus()
	if status == nil {
		t.Error("GetStatus should not return nil")
	}

	// 测试降级链
	chain := manager.GetFallbackChain()
	if len(chain) == 0 {
		t.Error("fallback chain should not be empty")
	}
}

func TestImageQuality(t *testing.T) {
	quality := &ImageQuality{
		Score:      0.8,
		Clarity:    0.75,
		Contrast:   0.7,
		Brightness: 0.5,
		NoiseLevel: 0.1,
	}

	if quality.Score < 0.5 {
		t.Error("quality score should be >= 0.5")
	}

	if quality.Brightness < 0 || quality.Brightness > 1 {
		t.Error("brightness should be in [0, 1]")
	}
}

func TestSceneTypes(t *testing.T) {
	scenes := []SceneType{
		SceneGeneral,
		SceneDocument,
		SceneTable,
		SceneInvoice,
		SceneIDCard,
		SceneHandwriting,
		ScenePrint,
	}

	for _, scene := range scenes {
		if scene == "" {
			t.Error("scene type should not be empty")
		}
	}
}

func TestEngineTypes(t *testing.T) {
	engines := []EngineType{
		EngineRapidOCR,
		EnginePaddleOCR,
		EngineTesseract,
		EngineBaidu,
		EngineTencent,
		EngineQwenVL,
		EngineGPT4V,
		EngineClaude,
	}

	for _, engine := range engines {
		if engine == "" {
			t.Error("engine type should not be empty")
		}
	}
}

func TestRoutingStrategies(t *testing.T) {
	strategies := []RoutingStrategy{
		StrategyQualityFirst,
		StrategyCostFirst,
		StrategySpeedFirst,
		StrategySmart,
	}

	for _, strategy := range strategies {
		if strategy == "" {
			t.Error("strategy should not be empty")
		}
	}
}

func TestOCRResult(t *testing.T) {
	result := &OCRResult{
		Success:  true,
		FullText: "测试文本",
		Engine:   EngineTesseract,
		Latency:  100,
		Blocks: []TextBlock{
			{Text: "测试", Confidence: 0.95},
		},
	}

	if !result.Success {
		t.Error("result should be successful")
	}

	if result.FullText != "测试文本" {
		t.Errorf("expected '测试文本', got '%s'", result.FullText)
	}

	if len(result.Blocks) != 1 {
		t.Errorf("expected 1 block, got %d", len(result.Blocks))
	}
}

func TestTableResult(t *testing.T) {
	result := &TableResult{
		OCRResult: OCRResult{
			Success: true,
		},
		Tables: []Table{
			{
				Headers: []string{"名称", "数量", "单价"},
				Rows: [][]string{
					{"商品A", "10", "100"},
					{"商品B", "5", "200"},
				},
			},
		},
	}

	if len(result.Tables) != 1 {
		t.Errorf("expected 1 table, got %d", len(result.Tables))
	}

	if len(result.Tables[0].Rows) != 2 {
		t.Errorf("expected 2 rows, got %d", len(result.Tables[0].Rows))
	}
}

func TestInvoiceResult(t *testing.T) {
	result := &InvoiceResult{
		OCRResult:   OCRResult{Success: true},
		InvoiceType: "vat_invoice",
		InvoiceNo:   "12345678",
		TotalAmount: "1000.00",
	}

	if result.InvoiceType != "vat_invoice" {
		t.Errorf("expected 'vat_invoice', got '%s'", result.InvoiceType)
	}

	if result.TotalAmount != "1000.00" {
		t.Errorf("expected '1000.00', got '%s'", result.TotalAmount)
	}
}

func TestIDCardResult(t *testing.T) {
	result := &IDCardResult{
		OCRResult: OCRResult{Success: true},
		CardType:  "id_card",
		Name:      "张三",
		IDNumber:  "110101199001011234",
	}

	if result.CardType != "id_card" {
		t.Errorf("expected 'id_card', got '%s'", result.CardType)
	}

	if result.Name != "张三" {
		t.Errorf("expected '张三', got '%s'", result.Name)
	}
}

func TestSplitLanguages(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"zh,en", 2},
		{"zh;en;ja", 3},
		{"zh en ja", 3},
		{"", 0},
		{"zh", 1},
	}

	for _, tt := range tests {
		result := splitLanguages(tt.input)
		if len(result) != tt.expected {
			t.Errorf("splitLanguages(%s): expected %d, got %d", tt.input, tt.expected, len(result))
		}
	}
}

func TestJoinCSV(t *testing.T) {
	tests := []struct {
		input    []string
		expected string
	}{
		{[]string{"a", "b", "c"}, "a,b,c"},
		{[]string{"a,b", "c"}, "\"a,b\",c"},
		{[]string{"a\"b", "c"}, "\"a\"\"b\",c"},
		{[]string{}, ""},
	}

	for _, tt := range tests {
		result := joinCSV(tt.input)
		if result != tt.expected {
			t.Errorf("joinCSV(%v): expected %s, got %s", tt.input, tt.expected, result)
		}
	}
}

// Mock engine for testing
type mockEngine struct {
	name            EngineType
	layer           EngineLayer
	supportedScenes []SceneType
	loaded          bool
	memoryUsage     int
}

func (m *mockEngine) Name() EngineType            { return m.name }
func (m *mockEngine) Layer() EngineLayer          { return m.layer }
func (m *mockEngine) SupportedScenes() []SceneType { return m.supportedScenes }
func (m *mockEngine) MemoryUsage() int            { return m.memoryUsage }
func (m *mockEngine) IsLoaded() bool              { return m.loaded }
func (m *mockEngine) Load(ctx context.Context) error {
	m.loaded = true
	return nil
}
func (m *mockEngine) Unload() error {
	m.loaded = false
	return nil
}
func (m *mockEngine) Health(ctx context.Context) error { return nil }

func (m *mockEngine) Stats() EngineStatus {
	return EngineStatus{
		Engine:    m.name,
		Available: true,
		Loaded:    m.loaded,
		MemoryMB:  m.memoryUsage,
	}
}

func (m *mockEngine) Recognize(ctx context.Context, req *OCRRequest) (*OCRResult, error) {
	return &OCRResult{
		Success:  true,
		FullText: "mock text",
		Engine:   m.name,
		Latency:  50,
	}, nil
}

func (m *mockEngine) RecognizeTable(ctx context.Context, req *OCRRequest) (*TableResult, error) {
	return &TableResult{
		OCRResult: OCRResult{Success: true},
		Tables:    []Table{{Rows: [][]string{{"a", "b"}}}},
	}, nil
}

func (m *mockEngine) RecognizeInvoice(ctx context.Context, req *OCRRequest) (*InvoiceResult, error) {
	return &InvoiceResult{
		OCRResult: OCRResult{Success: true},
	}, nil
}

func (m *mockEngine) RecognizeIDCard(ctx context.Context, req *OCRRequest) (*IDCardResult, error) {
	return &IDCardResult{
		OCRResult: OCRResult{Success: true},
	}, nil
}

func TestEngineManagerWithMockEngine(t *testing.T) {
	config := DefaultConfig()
	manager := NewEngineManager(config)

	mock := &mockEngine{
		name:            EngineType("mock"),
		layer:           LayerSmallModel,
		supportedScenes: []SceneType{SceneGeneral, SceneDocument},
		memoryUsage:     100,
	}

	manager.RegisterEngine(mock)

	ctx := context.Background()
	engine, err := manager.GetEngine(ctx, EngineType("mock"))
	if err != nil {
		t.Fatalf("GetEngine failed: %v", err)
	}

	if !engine.IsLoaded() {
		t.Error("engine should be loaded")
	}

	result, err := engine.Recognize(ctx, &OCRRequest{ImagePath: "/test.jpg"})
	if err != nil {
		t.Fatalf("Recognize failed: %v", err)
	}

	if result.FullText != "mock text" {
		t.Errorf("expected 'mock text', got '%s'", result.FullText)
	}
}

func TestSetStrategy(t *testing.T) {
	tool, _ := NewTool(nil)
	ctx := context.Background()

	result, err := tool.handleSetStrategy(ctx, map[string]interface{}{
		"strategy": "quality_first",
	})

	if err != nil {
		t.Fatalf("handleSetStrategy failed: %v", err)
	}

	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("expected map result")
	}

	if resultMap["strategy"] != "quality_first" {
		t.Errorf("expected 'quality_first', got '%v'", resultMap["strategy"])
	}

	if tool.config.DefaultStrategy != StrategyQualityFirst {
		t.Errorf("config strategy not updated")
	}
}

func TestGetStatus(t *testing.T) {
	tool, _ := NewTool(nil)
	ctx := context.Background()

	result, err := tool.handleGetStatus(ctx, nil)
	if err != nil {
		t.Fatalf("handleGetStatus failed: %v", err)
	}

	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("expected map result")
	}

	if _, ok := resultMap["engines"]; !ok {
		t.Error("expected 'engines' field")
	}

	if _, ok := resultMap["default_strategy"]; !ok {
		t.Error("expected 'default_strategy' field")
	}
}
