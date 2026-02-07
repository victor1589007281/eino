// Package quota OCR 配额管理器
package quota

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/cloudwego/eino/vdocstool/tools/ocr"
)

// Manager 配额管理器
type Manager struct {
	config   *Config
	quotas   map[ocr.EngineType]*EngineQuota
	mu       sync.RWMutex
	savePath string
}

// Config 配额配置
type Config struct {
	// 每日配额
	DailyQuota map[ocr.EngineType]int `json:"daily_quota"`
	// 每月配额
	MonthlyQuota map[ocr.EngineType]int `json:"monthly_quota"`
	// 配额存储路径
	StoragePath string `json:"storage_path"`
	// 是否启用配额限制
	Enabled bool `json:"enabled"`
}

// EngineQuota 引擎配额
type EngineQuota struct {
	Engine      ocr.EngineType `json:"engine"`
	DailyLimit  int            `json:"daily_limit"`
	MonthlyLimit int           `json:"monthly_limit"`
	DailyUsed   int            `json:"daily_used"`
	MonthlyUsed int            `json:"monthly_used"`
	LastReset   time.Time      `json:"last_reset"`
	LastDay     int            `json:"last_day"`
	LastMonth   int            `json:"last_month"`
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		Enabled:     true,
		StoragePath: "/tmp/ocr_quota.json",
		DailyQuota: map[ocr.EngineType]int{
			ocr.EngineBaidu:   500,  // 百度免费版每日 500 次
			ocr.EngineTencent: 1000, // 腾讯免费版每日 1000 次
			ocr.EngineQwenVL:  100,
			ocr.EngineGPT4V:   50,
			ocr.EngineClaude:  50,
		},
		MonthlyQuota: map[ocr.EngineType]int{
			ocr.EngineBaidu:   50000,
			ocr.EngineTencent: 10000,
		},
	}
}

// NewManager 创建配额管理器
func NewManager(config *Config) *Manager {
	if config == nil {
		config = DefaultConfig()
	}

	m := &Manager{
		config:   config,
		quotas:   make(map[ocr.EngineType]*EngineQuota),
		savePath: config.StoragePath,
	}

	// 初始化配额
	for engine, daily := range config.DailyQuota {
		monthly := 0
		if m, ok := config.MonthlyQuota[engine]; ok {
			monthly = m
		}
		m.quotas[engine] = &EngineQuota{
			Engine:       engine,
			DailyLimit:   daily,
			MonthlyLimit: monthly,
			LastReset:    time.Now(),
			LastDay:      time.Now().Day(),
			LastMonth:    int(time.Now().Month()),
		}
	}

	// 尝试从文件加载
	_ = m.Load()

	return m
}

// CheckQuota 检查配额是否可用
func (m *Manager) CheckQuota(engine ocr.EngineType) (bool, string) {
	if !m.config.Enabled {
		return true, ""
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	quota, ok := m.quotas[engine]
	if !ok {
		return true, "" // 没有配置配额限制
	}

	// 检查是否需要重置
	m.checkAndReset(quota)

	// 检查日配额
	if quota.DailyLimit > 0 && quota.DailyUsed >= quota.DailyLimit {
		return false, fmt.Sprintf("daily quota exceeded for %s: %d/%d", engine, quota.DailyUsed, quota.DailyLimit)
	}

	// 检查月配额
	if quota.MonthlyLimit > 0 && quota.MonthlyUsed >= quota.MonthlyLimit {
		return false, fmt.Sprintf("monthly quota exceeded for %s: %d/%d", engine, quota.MonthlyUsed, quota.MonthlyLimit)
	}

	return true, ""
}

// UseQuota 使用配额
func (m *Manager) UseQuota(engine ocr.EngineType) error {
	if !m.config.Enabled {
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	quota, ok := m.quotas[engine]
	if !ok {
		return nil
	}

	m.checkAndReset(quota)

	quota.DailyUsed++
	quota.MonthlyUsed++

	// 异步保存
	go m.Save()

	return nil
}

// GetQuotaStatus 获取配额状态
func (m *Manager) GetQuotaStatus(engine ocr.EngineType) *QuotaStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	quota, ok := m.quotas[engine]
	if !ok {
		return nil
	}

	return &QuotaStatus{
		Engine:          engine,
		DailyUsed:       quota.DailyUsed,
		DailyLimit:      quota.DailyLimit,
		DailyRemaining:  max(0, quota.DailyLimit-quota.DailyUsed),
		MonthlyUsed:     quota.MonthlyUsed,
		MonthlyLimit:    quota.MonthlyLimit,
		MonthlyRemaining: max(0, quota.MonthlyLimit-quota.MonthlyUsed),
	}
}

// GetAllQuotaStatus 获取所有配额状态
func (m *Manager) GetAllQuotaStatus() []*QuotaStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*QuotaStatus
	for engine, quota := range m.quotas {
		result = append(result, &QuotaStatus{
			Engine:           engine,
			DailyUsed:        quota.DailyUsed,
			DailyLimit:       quota.DailyLimit,
			DailyRemaining:   max(0, quota.DailyLimit-quota.DailyUsed),
			MonthlyUsed:      quota.MonthlyUsed,
			MonthlyLimit:     quota.MonthlyLimit,
			MonthlyRemaining: max(0, quota.MonthlyLimit-quota.MonthlyUsed),
		})
	}
	return result
}

// QuotaStatus 配额状态
type QuotaStatus struct {
	Engine           ocr.EngineType `json:"engine"`
	DailyUsed        int            `json:"daily_used"`
	DailyLimit       int            `json:"daily_limit"`
	DailyRemaining   int            `json:"daily_remaining"`
	MonthlyUsed      int            `json:"monthly_used"`
	MonthlyLimit     int            `json:"monthly_limit"`
	MonthlyRemaining int            `json:"monthly_remaining"`
}

// checkAndReset 检查并重置配额
func (m *Manager) checkAndReset(quota *EngineQuota) {
	now := time.Now()

	// 检查日重置
	if now.Day() != quota.LastDay {
		quota.DailyUsed = 0
		quota.LastDay = now.Day()
	}

	// 检查月重置
	if int(now.Month()) != quota.LastMonth {
		quota.MonthlyUsed = 0
		quota.LastMonth = int(now.Month())
	}

	quota.LastReset = now
}

// Save 保存配额到文件
func (m *Manager) Save() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.savePath == "" {
		return nil
	}

	dir := filepath.Dir(m.savePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(m.quotas, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(m.savePath, data, 0644)
}

// Load 从文件加载配额
func (m *Manager) Load() error {
	if m.savePath == "" {
		return nil
	}

	data, err := os.ReadFile(m.savePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var quotas map[ocr.EngineType]*EngineQuota
	if err := json.Unmarshal(data, &quotas); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for engine, quota := range quotas {
		if existing, ok := m.quotas[engine]; ok {
			// 保留限制配置，更新使用量
			existing.DailyUsed = quota.DailyUsed
			existing.MonthlyUsed = quota.MonthlyUsed
			existing.LastDay = quota.LastDay
			existing.LastMonth = quota.LastMonth
			existing.LastReset = quota.LastReset
		}
	}

	return nil
}

// SetDailyLimit 设置日配额
func (m *Manager) SetDailyLimit(engine ocr.EngineType, limit int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if quota, ok := m.quotas[engine]; ok {
		quota.DailyLimit = limit
	} else {
		m.quotas[engine] = &EngineQuota{
			Engine:     engine,
			DailyLimit: limit,
			LastReset:  time.Now(),
			LastDay:    time.Now().Day(),
			LastMonth:  int(time.Now().Month()),
		}
	}
}

// SetMonthlyLimit 设置月配额
func (m *Manager) SetMonthlyLimit(engine ocr.EngineType, limit int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if quota, ok := m.quotas[engine]; ok {
		quota.MonthlyLimit = limit
	} else {
		m.quotas[engine] = &EngineQuota{
			Engine:       engine,
			MonthlyLimit: limit,
			LastReset:    time.Now(),
			LastDay:      time.Now().Day(),
			LastMonth:    int(time.Now().Month()),
		}
	}
}

// ResetQuota 重置配额
func (m *Manager) ResetQuota(engine ocr.EngineType, resetDaily, resetMonthly bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if quota, ok := m.quotas[engine]; ok {
		if resetDaily {
			quota.DailyUsed = 0
		}
		if resetMonthly {
			quota.MonthlyUsed = 0
		}
		quota.LastReset = time.Now()
	}
}

// WithContext 包装引擎调用，自动检查和消耗配额
func (m *Manager) WithContext(ctx context.Context, engine ocr.EngineType, fn func() error) error {
	// 检查配额
	ok, msg := m.CheckQuota(engine)
	if !ok {
		return fmt.Errorf("quota exceeded: %s", msg)
	}

	// 执行操作
	if err := fn(); err != nil {
		return err
	}

	// 消耗配额
	return m.UseQuota(engine)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
