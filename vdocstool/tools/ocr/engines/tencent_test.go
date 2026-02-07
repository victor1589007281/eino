package engines

import (
	"testing"

	"github.com/cloudwego/eino/vdocstool/tools/ocr"
)

func TestDefaultTencentOCRConfig(t *testing.T) {
	config := DefaultTencentOCRConfig()
	if config.Timeout == 0 {
		t.Error("expected default timeout")
	}
	if config.Region == "" {
		t.Error("expected default region")
	}
}

func TestNewTencentOCREngine(t *testing.T) {
	// 测试 nil 配置
	engine := NewTencentOCREngine(nil)
	if engine == nil {
		t.Fatal("expected engine")
	}
	if engine.Name() != ocr.EngineTencent {
		t.Errorf("expected tencent, got %s", engine.Name())
	}

	// 测试自定义配置
	config := &TencentOCRConfig{
		SecretID:  "test-id",
		SecretKey: "test-secret",
		Region:    "ap-beijing",
	}
	engine = NewTencentOCREngine(config)
	if engine.config.SecretID != "test-id" {
		t.Errorf("expected test-id")
	}
	if engine.config.Region != "ap-beijing" {
		t.Errorf("expected ap-beijing")
	}
}

func TestTencentOCREngineProperties(t *testing.T) {
	engine := NewTencentOCREngine(nil)

	if engine.Name() != ocr.EngineTencent {
		t.Errorf("expected tencent")
	}

	if engine.Layer() != ocr.LayerSmallModel {
		t.Errorf("expected small model layer")
	}

	scenes := engine.SupportedScenes()
	if len(scenes) == 0 {
		t.Error("expected supported scenes")
	}

	// 检查场景支持
	sceneMap := make(map[ocr.SceneType]bool)
	for _, s := range scenes {
		sceneMap[s] = true
	}

	if !sceneMap[ocr.SceneGeneral] {
		t.Error("should support general scene")
	}
	if !sceneMap[ocr.SceneTable] {
		t.Error("should support table scene")
	}
	if !sceneMap[ocr.SceneHandwriting] {
		t.Error("should support handwriting scene")
	}
}

func TestTencentOCREngineMemoryUsage(t *testing.T) {
	engine := NewTencentOCREngine(nil)
	memory := engine.MemoryUsage()
	if memory <= 0 {
		t.Error("expected positive memory usage")
	}
	if memory > 50 {
		t.Error("cloud service should use minimal memory")
	}
}

func TestTencentOCREngineLoadUnload(t *testing.T) {
	engine := NewTencentOCREngine(&TencentOCRConfig{
		SecretID:  "test-id",
		SecretKey: "test-secret",
	})

	if engine.IsLoaded() {
		t.Error("should not be loaded initially")
	}

	err := engine.Load(nil)
	if err != nil {
		t.Errorf("load should not fail with valid config: %v", err)
	}

	if !engine.IsLoaded() {
		t.Error("should be loaded after Load()")
	}

	err = engine.Unload()
	if err != nil {
		t.Errorf("unload should not fail: %v", err)
	}

	if engine.IsLoaded() {
		t.Error("should not be loaded after Unload()")
	}
}

func TestTencentOCREngineLoadWithoutKey(t *testing.T) {
	engine := NewTencentOCREngine(nil)

	err := engine.Load(nil)
	if err == nil {
		t.Error("expected error without secret id/key")
	}
}

func TestTencentOCREngineStats(t *testing.T) {
	engine := NewTencentOCREngine(&TencentOCRConfig{
		SecretID: "test",
	})

	stats := engine.Stats()
	if stats.Engine != ocr.EngineTencent {
		t.Errorf("expected tencent")
	}
	if stats.TotalCalls != 0 {
		t.Errorf("expected 0 total calls")
	}
	if !stats.Available {
		t.Error("should be available with secret id")
	}
}

func TestTencentOCREngineHealth(t *testing.T) {
	// 没有配置
	engine := NewTencentOCREngine(nil)
	err := engine.Health(nil)
	if err == nil {
		t.Error("expected error without secret id/key")
	}

	// 有配置
	engine = NewTencentOCREngine(&TencentOCRConfig{
		SecretID:  "test",
		SecretKey: "test",
	})
	err = engine.Health(nil)
	if err != nil {
		t.Errorf("health should pass with config: %v", err)
	}
}

func TestTencentOCRRecordFailure(t *testing.T) {
	engine := NewTencentOCREngine(nil)

	engine.recordFailure()
	engine.recordFailure()
	engine.recordFailure()

	stats := engine.Stats()
	if stats.FailedCalls != 3 {
		t.Errorf("expected 3 failed calls, got %d", stats.FailedCalls)
	}
}

func TestSha256Hex(t *testing.T) {
	result := sha256Hex([]byte("hello"))
	if len(result) != 64 {
		t.Errorf("expected 64 char hex string, got %d", len(result))
	}
}

func TestHmacSHA256(t *testing.T) {
	key := []byte("secret")
	data := []byte("message")
	result := hmacSHA256(key, data)
	if len(result) != 32 {
		t.Errorf("expected 32 bytes, got %d", len(result))
	}
}
