// Package router 配额管理器
package router

import (
	"encoding/json"
	"os"
	"sync"
	"time"
)

// QuotaStatus 配额状态
type QuotaStatus string

const (
	QuotaHealthy   QuotaStatus = "healthy"   // < 70%
	QuotaWarning   QuotaStatus = "warning"   // 70% - 90%
	QuotaCritical  QuotaStatus = "critical"  // 90% - 100%
	QuotaExhausted QuotaStatus = "exhausted" // >= 100%
)

// EngineQuota 引擎配额
type EngineQuota struct {
	EngineName string `json:"engine_name"`
	
	// 配额设置
	MonthlyLimit int64 `json:"monthly_limit"` // 月度限额，0表示无限制
	DailyLimit   int64 `json:"daily_limit"`   // 日限额，0表示无限制
	
	// 使用统计
	MonthlyUsed   int64     `json:"monthly_used"`
	DailyUsed     int64     `json:"daily_used"`
	LastResetTime time.Time `json:"last_reset_time"`
	DailyResetTime time.Time `json:"daily_reset_time"`
	
	// 阈值设置
	WarningThreshold  float64 `json:"warning_threshold"`  // 告警阈值 (0.7)
	CriticalThreshold float64 `json:"critical_threshold"` // 临界阈值 (0.9)
	
	// 成本信息
	CostPerQuery float64 `json:"cost_per_query"` // 每次查询成本
	IsFree       bool    `json:"is_free"`        // 是否免费
	
	mu sync.RWMutex
}

// QuotaManager 配额管理器
type QuotaManager struct {
	quotas       map[string]*EngineQuota
	mu           sync.RWMutex
	
	// 持久化路径
	statePath    string
	
	// 自动保存
	saveInterval time.Duration
	stopCh       chan struct{}
}

// QuotaConfig 配额配置
type QuotaConfig struct {
	MonthlyLimit      int64   `json:"monthly_limit"`
	DailyLimit        int64   `json:"daily_limit"`
	CostPerQuery      float64 `json:"cost_per_query"`
	IsFree            bool    `json:"is_free"`
	WarningThreshold  float64 `json:"warning_threshold"`
	CriticalThreshold float64 `json:"critical_threshold"`
}

// DefaultQuotaConfigs 默认配额配置
var DefaultQuotaConfigs = map[string]QuotaConfig{
	"serper": {
		MonthlyLimit:      2500,
		DailyLimit:        100,
		CostPerQuery:      0.001,
		IsFree:            false,
		WarningThreshold:  0.7,
		CriticalThreshold: 0.9,
	},
	"tavily": {
		MonthlyLimit:      1000,
		DailyLimit:        50,
		CostPerQuery:      0.001,
		IsFree:            false,
		WarningThreshold:  0.7,
		CriticalThreshold: 0.9,
	},
	"exa": {
		MonthlyLimit:      1000,
		DailyLimit:        50,
		CostPerQuery:      0.001,
		IsFree:            false,
		WarningThreshold:  0.7,
		CriticalThreshold: 0.9,
	},
	"brave": {
		MonthlyLimit:      2000,
		DailyLimit:        100,
		CostPerQuery:      0.0005,
		IsFree:            false,
		WarningThreshold:  0.7,
		CriticalThreshold: 0.9,
	},
	"duckduckgo": {
		MonthlyLimit:      0, // 无限制
		DailyLimit:        0,
		CostPerQuery:      0,
		IsFree:            true,
		WarningThreshold:  1.0,
		CriticalThreshold: 1.0,
	},
	"searxng": {
		MonthlyLimit:      0,
		DailyLimit:        0,
		CostPerQuery:      0,
		IsFree:            true,
		WarningThreshold:  1.0,
		CriticalThreshold: 1.0,
	},
	"bing": {
		MonthlyLimit:      0,
		DailyLimit:        0,
		CostPerQuery:      0,
		IsFree:            true,
		WarningThreshold:  1.0,
		CriticalThreshold: 1.0,
	},
	"baidu": {
		MonthlyLimit:      0,
		DailyLimit:        0,
		CostPerQuery:      0,
		IsFree:            true,
		WarningThreshold:  1.0,
		CriticalThreshold: 1.0,
	},
}

// NewQuotaManager 创建配额管理器
func NewQuotaManager(statePath string) *QuotaManager {
	qm := &QuotaManager{
		quotas:       make(map[string]*EngineQuota),
		statePath:    statePath,
		saveInterval: time.Minute,
		stopCh:       make(chan struct{}),
	}
	
	// 初始化默认配额
	for name, cfg := range DefaultQuotaConfigs {
		qm.quotas[name] = &EngineQuota{
			EngineName:        name,
			MonthlyLimit:      cfg.MonthlyLimit,
			DailyLimit:        cfg.DailyLimit,
			CostPerQuery:      cfg.CostPerQuery,
			IsFree:            cfg.IsFree,
			WarningThreshold:  cfg.WarningThreshold,
			CriticalThreshold: cfg.CriticalThreshold,
			LastResetTime:     time.Now(),
			DailyResetTime:    time.Now(),
		}
	}
	
	// 尝试加载持久化状态
	qm.loadState()
	
	return qm
}

// Start 启动配额管理器
func (qm *QuotaManager) Start() {
	go qm.autoSaveLoop()
	go qm.autoResetLoop()
}

// Stop 停止配额管理器
func (qm *QuotaManager) Stop() {
	close(qm.stopCh)
	qm.saveState()
}

// IncrementUsage 增加使用量
func (qm *QuotaManager) IncrementUsage(engineName string) {
	qm.mu.RLock()
	quota, ok := qm.quotas[engineName]
	qm.mu.RUnlock()
	
	if !ok {
		return
	}
	
	quota.mu.Lock()
	defer quota.mu.Unlock()
	
	quota.MonthlyUsed++
	quota.DailyUsed++
}

// GetQuota 获取配额信息
func (qm *QuotaManager) GetQuota(engineName string) *EngineQuota {
	qm.mu.RLock()
	defer qm.mu.RUnlock()
	return qm.quotas[engineName]
}

// GetQuotaHealth 获取配额健康度 (0-1)
func (qm *QuotaManager) GetQuotaHealth(engineName string) float64 {
	qm.mu.RLock()
	quota, ok := qm.quotas[engineName]
	qm.mu.RUnlock()
	
	if !ok {
		return 1.0
	}
	
	return quota.GetHealth()
}

// GetHealth 获取健康度
func (q *EngineQuota) GetHealth() float64 {
	q.mu.RLock()
	defer q.mu.RUnlock()
	
	if q.MonthlyLimit <= 0 {
		return 1.0 // 无限制
	}
	
	usageRatio := float64(q.MonthlyUsed) / float64(q.MonthlyLimit)
	
	// 非线性衰减
	if usageRatio < 0.5 {
		return 1.0
	} else if usageRatio < 0.7 {
		return 1.0 - (usageRatio-0.5)*0.5 // 0.9 - 1.0
	} else if usageRatio < 0.9 {
		return 0.9 - (usageRatio-0.7)*1.5 // 0.6 - 0.9
	} else if usageRatio < 1.0 {
		return 0.6 - (usageRatio-0.9)*5.0 // 0.1 - 0.6
	}
	return 0.0 // 已耗尽
}

// GetStatus 获取配额状态
func (q *EngineQuota) GetStatus() QuotaStatus {
	q.mu.RLock()
	defer q.mu.RUnlock()
	
	if q.MonthlyLimit <= 0 {
		return QuotaHealthy
	}
	
	usageRatio := float64(q.MonthlyUsed) / float64(q.MonthlyLimit)
	
	if usageRatio >= 1.0 {
		return QuotaExhausted
	} else if usageRatio >= q.CriticalThreshold {
		return QuotaCritical
	} else if usageRatio >= q.WarningThreshold {
		return QuotaWarning
	}
	return QuotaHealthy
}

// IsAvailable 检查是否可用
func (qm *QuotaManager) IsAvailable(engineName string) bool {
	qm.mu.RLock()
	quota, ok := qm.quotas[engineName]
	qm.mu.RUnlock()
	
	if !ok {
		return true // 未配置的引擎默认可用
	}
	
	return quota.GetStatus() != QuotaExhausted
}

// GetAllQuotaStatus 获取所有引擎配额状态
func (qm *QuotaManager) GetAllQuotaStatus() map[string]*QuotaInfo {
	qm.mu.RLock()
	defer qm.mu.RUnlock()
	
	result := make(map[string]*QuotaInfo)
	for name, quota := range qm.quotas {
		quota.mu.RLock()
		info := &QuotaInfo{
			EngineName:   name,
			MonthlyLimit: quota.MonthlyLimit,
			MonthlyUsed:  quota.MonthlyUsed,
			DailyLimit:   quota.DailyLimit,
			DailyUsed:    quota.DailyUsed,
			Status:       quota.GetStatus(),
			Health:       quota.GetHealth(),
			IsFree:       quota.IsFree,
			UsagePercent: 0,
		}
		if quota.MonthlyLimit > 0 {
			info.UsagePercent = float64(quota.MonthlyUsed) / float64(quota.MonthlyLimit) * 100
		}
		quota.mu.RUnlock()
		result[name] = info
	}
	
	return result
}

// QuotaInfo 配额信息
type QuotaInfo struct {
	EngineName   string      `json:"engine_name"`
	MonthlyLimit int64       `json:"monthly_limit"`
	MonthlyUsed  int64       `json:"monthly_used"`
	DailyLimit   int64       `json:"daily_limit"`
	DailyUsed    int64       `json:"daily_used"`
	Status       QuotaStatus `json:"status"`
	Health       float64     `json:"health"`
	IsFree       bool        `json:"is_free"`
	UsagePercent float64     `json:"usage_percent"`
}

// autoSaveLoop 自动保存循环
func (qm *QuotaManager) autoSaveLoop() {
	ticker := time.NewTicker(qm.saveInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			qm.saveState()
		case <-qm.stopCh:
			return
		}
	}
}

// autoResetLoop 自动重置循环
func (qm *QuotaManager) autoResetLoop() {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			qm.checkAndReset()
		case <-qm.stopCh:
			return
		}
	}
}

// checkAndReset 检查并重置配额
func (qm *QuotaManager) checkAndReset() {
	now := time.Now()
	
	qm.mu.RLock()
	defer qm.mu.RUnlock()
	
	for _, quota := range qm.quotas {
		quota.mu.Lock()
		
		// 检查月度重置
		if now.Month() != quota.LastResetTime.Month() || now.Year() != quota.LastResetTime.Year() {
			quota.MonthlyUsed = 0
			quota.LastResetTime = now
		}
		
		// 检查日重置
		if now.Day() != quota.DailyResetTime.Day() || now.Month() != quota.DailyResetTime.Month() {
			quota.DailyUsed = 0
			quota.DailyResetTime = now
		}
		
		quota.mu.Unlock()
	}
}

// saveState 保存状态
func (qm *QuotaManager) saveState() {
	if qm.statePath == "" {
		return
	}
	
	qm.mu.RLock()
	defer qm.mu.RUnlock()
	
	state := make(map[string]*QuotaState)
	for name, quota := range qm.quotas {
		quota.mu.RLock()
		state[name] = &QuotaState{
			MonthlyUsed:    quota.MonthlyUsed,
			DailyUsed:      quota.DailyUsed,
			LastResetTime:  quota.LastResetTime,
			DailyResetTime: quota.DailyResetTime,
		}
		quota.mu.RUnlock()
	}
	
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return
	}
	
	os.WriteFile(qm.statePath, data, 0644)
}

// loadState 加载状态
func (qm *QuotaManager) loadState() {
	if qm.statePath == "" {
		return
	}
	
	data, err := os.ReadFile(qm.statePath)
	if err != nil {
		return
	}
	
	var state map[string]*QuotaState
	if err := json.Unmarshal(data, &state); err != nil {
		return
	}
	
	qm.mu.Lock()
	defer qm.mu.Unlock()
	
	for name, s := range state {
		if quota, ok := qm.quotas[name]; ok {
			quota.mu.Lock()
			quota.MonthlyUsed = s.MonthlyUsed
			quota.DailyUsed = s.DailyUsed
			quota.LastResetTime = s.LastResetTime
			quota.DailyResetTime = s.DailyResetTime
			quota.mu.Unlock()
		}
	}
}

// QuotaState 配额状态（用于持久化）
type QuotaState struct {
	MonthlyUsed    int64     `json:"monthly_used"`
	DailyUsed      int64     `json:"daily_used"`
	LastResetTime  time.Time `json:"last_reset_time"`
	DailyResetTime time.Time `json:"daily_reset_time"`
}

// UpdateQuotaConfig 更新配额配置
func (qm *QuotaManager) UpdateQuotaConfig(engineName string, cfg QuotaConfig) {
	qm.mu.Lock()
	defer qm.mu.Unlock()
	
	if quota, ok := qm.quotas[engineName]; ok {
		quota.mu.Lock()
		quota.MonthlyLimit = cfg.MonthlyLimit
		quota.DailyLimit = cfg.DailyLimit
		quota.CostPerQuery = cfg.CostPerQuery
		quota.IsFree = cfg.IsFree
		quota.WarningThreshold = cfg.WarningThreshold
		quota.CriticalThreshold = cfg.CriticalThreshold
		quota.mu.Unlock()
	} else {
		qm.quotas[engineName] = &EngineQuota{
			EngineName:        engineName,
			MonthlyLimit:      cfg.MonthlyLimit,
			DailyLimit:        cfg.DailyLimit,
			CostPerQuery:      cfg.CostPerQuery,
			IsFree:            cfg.IsFree,
			WarningThreshold:  cfg.WarningThreshold,
			CriticalThreshold: cfg.CriticalThreshold,
			LastResetTime:     time.Now(),
			DailyResetTime:    time.Now(),
		}
	}
}

// GetCost 获取引擎成本
func (qm *QuotaManager) GetCost(engineName string) float64 {
	qm.mu.RLock()
	quota, ok := qm.quotas[engineName]
	qm.mu.RUnlock()
	
	if !ok {
		return 0
	}
	
	quota.mu.RLock()
	defer quota.mu.RUnlock()
	return quota.CostPerQuery
}

// IsFree 检查是否免费
func (qm *QuotaManager) IsFree(engineName string) bool {
	qm.mu.RLock()
	quota, ok := qm.quotas[engineName]
	qm.mu.RUnlock()
	
	if !ok {
		return true
	}
	
	quota.mu.RLock()
	defer quota.mu.RUnlock()
	return quota.IsFree
}
