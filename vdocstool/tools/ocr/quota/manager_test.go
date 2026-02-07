package quota

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cloudwego/eino/vdocstool/tools/ocr"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()
	if !config.Enabled {
		t.Error("expected enabled by default")
	}
	if len(config.DailyQuota) == 0 {
		t.Error("expected default daily quotas")
	}
}

func TestNewManager(t *testing.T) {
	manager := NewManager(nil)
	if manager == nil {
		t.Fatal("expected manager")
	}
}

func TestNewManagerWithConfig(t *testing.T) {
	config := &Config{
		Enabled:     true,
		StoragePath: "/tmp/test_quota.json",
		DailyQuota: map[ocr.EngineType]int{
			ocr.EngineBaidu: 100,
		},
	}
	manager := NewManager(config)

	status := manager.GetQuotaStatus(ocr.EngineBaidu)
	if status == nil {
		t.Fatal("expected status")
	}
	if status.DailyLimit != 100 {
		t.Errorf("expected daily limit 100, got %d", status.DailyLimit)
	}
}

func TestManagerCheckQuota(t *testing.T) {
	config := &Config{
		Enabled: true,
		DailyQuota: map[ocr.EngineType]int{
			ocr.EngineBaidu: 5,
		},
	}
	manager := NewManager(config)

	// 初始时应该有配额
	ok, msg := manager.CheckQuota(ocr.EngineBaidu)
	if !ok {
		t.Errorf("expected quota available: %s", msg)
	}

	// 使用配额
	for i := 0; i < 5; i++ {
		manager.UseQuota(ocr.EngineBaidu)
	}

	// 配额耗尽
	ok, msg = manager.CheckQuota(ocr.EngineBaidu)
	if ok {
		t.Error("expected quota exhausted")
	}
	if msg == "" {
		t.Error("expected error message")
	}
}

func TestManagerCheckQuotaDisabled(t *testing.T) {
	config := &Config{
		Enabled: false,
	}
	manager := NewManager(config)

	// 禁用时总是返回 true
	ok, _ := manager.CheckQuota(ocr.EngineBaidu)
	if !ok {
		t.Error("expected true when disabled")
	}
}

func TestManagerCheckQuotaUnknownEngine(t *testing.T) {
	manager := NewManager(&Config{Enabled: true})

	// 未配置的引擎应该允许
	ok, _ := manager.CheckQuota("unknown_engine")
	if !ok {
		t.Error("expected true for unknown engine")
	}
}

func TestManagerUseQuota(t *testing.T) {
	config := &Config{
		Enabled: true,
		DailyQuota: map[ocr.EngineType]int{
			ocr.EngineTencent: 100,
		},
	}
	manager := NewManager(config)

	err := manager.UseQuota(ocr.EngineTencent)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	status := manager.GetQuotaStatus(ocr.EngineTencent)
	if status.DailyUsed != 1 {
		t.Errorf("expected 1 used, got %d", status.DailyUsed)
	}
}

func TestManagerGetQuotaStatus(t *testing.T) {
	config := &Config{
		Enabled: true,
		DailyQuota: map[ocr.EngineType]int{
			ocr.EngineBaidu: 500,
		},
		MonthlyQuota: map[ocr.EngineType]int{
			ocr.EngineBaidu: 10000,
		},
	}
	manager := NewManager(config)

	// 使用一些配额
	for i := 0; i < 10; i++ {
		manager.UseQuota(ocr.EngineBaidu)
	}

	status := manager.GetQuotaStatus(ocr.EngineBaidu)
	if status == nil {
		t.Fatal("expected status")
	}
	if status.DailyUsed != 10 {
		t.Errorf("expected 10 daily used, got %d", status.DailyUsed)
	}
	if status.MonthlyUsed != 10 {
		t.Errorf("expected 10 monthly used, got %d", status.MonthlyUsed)
	}
	if status.DailyRemaining != 490 {
		t.Errorf("expected 490 remaining, got %d", status.DailyRemaining)
	}
}

func TestManagerGetAllQuotaStatus(t *testing.T) {
	config := &Config{
		Enabled: true,
		DailyQuota: map[ocr.EngineType]int{
			ocr.EngineBaidu:   500,
			ocr.EngineTencent: 1000,
		},
	}
	manager := NewManager(config)

	statuses := manager.GetAllQuotaStatus()
	if len(statuses) != 2 {
		t.Errorf("expected 2 statuses, got %d", len(statuses))
	}
}

func TestManagerSetDailyLimit(t *testing.T) {
	manager := NewManager(&Config{Enabled: true})

	manager.SetDailyLimit(ocr.EngineQwenVL, 200)

	status := manager.GetQuotaStatus(ocr.EngineQwenVL)
	if status == nil {
		t.Fatal("expected status")
	}
	if status.DailyLimit != 200 {
		t.Errorf("expected 200, got %d", status.DailyLimit)
	}
}

func TestManagerSetMonthlyLimit(t *testing.T) {
	manager := NewManager(&Config{Enabled: true})

	manager.SetMonthlyLimit(ocr.EngineGPT4V, 5000)

	status := manager.GetQuotaStatus(ocr.EngineGPT4V)
	if status == nil {
		t.Fatal("expected status")
	}
	if status.MonthlyLimit != 5000 {
		t.Errorf("expected 5000, got %d", status.MonthlyLimit)
	}
}

func TestManagerResetQuota(t *testing.T) {
	config := &Config{
		Enabled: true,
		DailyQuota: map[ocr.EngineType]int{
			ocr.EngineBaidu: 100,
		},
	}
	manager := NewManager(config)

	// 使用配额
	for i := 0; i < 50; i++ {
		manager.UseQuota(ocr.EngineBaidu)
	}

	status := manager.GetQuotaStatus(ocr.EngineBaidu)
	if status.DailyUsed != 50 {
		t.Errorf("expected 50, got %d", status.DailyUsed)
	}

	// 重置
	manager.ResetQuota(ocr.EngineBaidu, true, false)

	status = manager.GetQuotaStatus(ocr.EngineBaidu)
	if status.DailyUsed != 0 {
		t.Errorf("expected 0 after reset, got %d", status.DailyUsed)
	}
}

func TestManagerSaveAndLoad(t *testing.T) {
	tmpDir := os.TempDir()
	savePath := filepath.Join(tmpDir, "test_ocr_quota.json")
	defer os.Remove(savePath)

	config := &Config{
		Enabled:     true,
		StoragePath: savePath,
		DailyQuota: map[ocr.EngineType]int{
			ocr.EngineBaidu: 100,
		},
	}

	// 创建管理器并使用配额
	manager := NewManager(config)
	for i := 0; i < 25; i++ {
		manager.UseQuota(ocr.EngineBaidu)
	}

	// 保存
	err := manager.Save()
	if err != nil {
		t.Fatalf("save failed: %v", err)
	}

	// 创建新管理器并加载
	manager2 := NewManager(config)
	err = manager2.Load()
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}

	status := manager2.GetQuotaStatus(ocr.EngineBaidu)
	if status.DailyUsed != 25 {
		t.Errorf("expected 25 after load, got %d", status.DailyUsed)
	}
}

func TestManagerWithContext(t *testing.T) {
	config := &Config{
		Enabled: true,
		DailyQuota: map[ocr.EngineType]int{
			ocr.EngineBaidu: 10,
		},
	}
	manager := NewManager(config)

	// 成功执行
	called := false
	err := manager.WithContext(nil, ocr.EngineBaidu, func() error {
		called = true
		return nil
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !called {
		t.Error("function should be called")
	}

	// 配额耗尽时
	for i := 0; i < 10; i++ {
		manager.UseQuota(ocr.EngineBaidu)
	}

	err = manager.WithContext(nil, ocr.EngineBaidu, func() error {
		t.Error("function should not be called when quota exhausted")
		return nil
	})
	if err == nil {
		t.Error("expected error when quota exhausted")
	}
}

func TestMax(t *testing.T) {
	tests := []struct {
		a, b, expected int
	}{
		{1, 2, 2},
		{5, 3, 5},
		{0, 0, 0},
		{-1, 1, 1},
	}

	for _, tc := range tests {
		result := max(tc.a, tc.b)
		if result != tc.expected {
			t.Errorf("max(%d, %d) = %d, expected %d", tc.a, tc.b, result, tc.expected)
		}
	}
}
