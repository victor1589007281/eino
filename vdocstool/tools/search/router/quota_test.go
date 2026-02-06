package router

import (
	"os"
	"testing"
)

func TestNewQuotaManager(t *testing.T) {
	// 使用临时文件路径
	tmpFile, err := os.CreateTemp("", "quota_test_*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	manager := NewQuotaManager(tmpFile.Name())
	if manager == nil {
		t.Fatal("NewQuotaManager returned nil")
	}
}

func TestQuotaManager_GetQuota(t *testing.T) {
	manager := NewQuotaManager("")

	// 测试获取预定义配额
	quota := manager.GetQuota("serper")
	if quota == nil {
		t.Fatal("GetQuota returned nil for serper")
	}

	if quota.DailyLimit != 100 {
		t.Errorf("DailyLimit = %d, want 100", quota.DailyLimit)
	}

	if quota.MonthlyLimit != 2500 {
		t.Errorf("MonthlyLimit = %d, want 2500", quota.MonthlyLimit)
	}
}

func TestQuotaManager_IncrementUsage(t *testing.T) {
	manager := NewQuotaManager("")

	initialQuota := manager.GetQuota("serper")
	initialDaily := initialQuota.DailyUsed
	initialMonthly := initialQuota.MonthlyUsed

	// 增加使用量
	manager.IncrementUsage("serper")

	quota := manager.GetQuota("serper")
	if quota.DailyUsed != initialDaily+1 {
		t.Errorf("DailyUsed = %d, want %d", quota.DailyUsed, initialDaily+1)
	}
	if quota.MonthlyUsed != initialMonthly+1 {
		t.Errorf("MonthlyUsed = %d, want %d", quota.MonthlyUsed, initialMonthly+1)
	}
}

func TestQuotaManager_IsAvailable(t *testing.T) {
	manager := NewQuotaManager("")

	// 免费引擎应该始终可用
	if !manager.IsAvailable("duckduckgo") {
		t.Error("Free engine duckduckgo should be available")
	}

	// 配额充足的引擎应该可用
	if !manager.IsAvailable("serper") {
		t.Error("Engine with available quota should be available")
	}

	// 未配置的引擎默认可用
	if !manager.IsAvailable("unknown_engine") {
		t.Error("Unknown engine should be available by default")
	}
}

func TestQuotaManager_GetQuotaHealth(t *testing.T) {
	manager := NewQuotaManager("")

	// 免费引擎健康度应该是1.0
	health := manager.GetQuotaHealth("duckduckgo")
	if health != 1.0 {
		t.Errorf("Free engine health = %v, want 1.0", health)
	}

	// 新初始化的付费引擎健康度应该是1.0
	health = manager.GetQuotaHealth("serper")
	if health != 1.0 {
		t.Errorf("Fresh engine health = %v, want 1.0", health)
	}
}

func TestEngineQuota_GetStatus(t *testing.T) {
	tests := []struct {
		name         string
		monthlyLimit int64
		monthlyUsed  int64
		want         QuotaStatus
	}{
		{"healthy", 100, 50, QuotaHealthy},
		{"warning", 100, 75, QuotaWarning},
		{"critical", 100, 95, QuotaCritical},
		{"exhausted", 100, 100, QuotaExhausted},
		{"unlimited", 0, 1000, QuotaHealthy},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			quota := &EngineQuota{
				MonthlyLimit:      tt.monthlyLimit,
				MonthlyUsed:       tt.monthlyUsed,
				WarningThreshold:  0.7,
				CriticalThreshold: 0.9,
			}
			if got := quota.GetStatus(); got != tt.want {
				t.Errorf("GetStatus() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEngineQuota_GetHealth(t *testing.T) {
	tests := []struct {
		name         string
		monthlyLimit int64
		monthlyUsed  int64
		wantMin      float64
		wantMax      float64
	}{
		{"unused", 100, 0, 1.0, 1.0},
		{"low_usage", 100, 30, 1.0, 1.0},
		{"medium_usage", 100, 60, 0.85, 1.0},
		{"high_usage", 100, 85, 0.5, 0.9},
		{"unlimited", 0, 1000, 1.0, 1.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			quota := &EngineQuota{
				MonthlyLimit: tt.monthlyLimit,
				MonthlyUsed:  tt.monthlyUsed,
			}
			health := quota.GetHealth()
			if health < tt.wantMin || health > tt.wantMax {
				t.Errorf("GetHealth() = %v, want between %v and %v", health, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestQuotaManager_GetAllQuotaStatus(t *testing.T) {
	manager := NewQuotaManager("")

	status := manager.GetAllQuotaStatus()
	if status == nil {
		t.Fatal("GetAllQuotaStatus returned nil")
	}

	// 应该有预定义的引擎
	expectedEngines := []string{"serper", "tavily", "exa", "duckduckgo", "searxng"}
	for _, engine := range expectedEngines {
		if _, ok := status[engine]; !ok {
			t.Errorf("Missing engine %s in status", engine)
		}
	}
}
