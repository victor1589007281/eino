// Package alert 价格提醒系统
package alert

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cloudwego/eino/vdocstool/tools/finance/types"
)

// 全局ID计数器
var alertIDCounter int64

// AlertType 提醒类型
type AlertType string

const (
	AlertTypePriceAbove  AlertType = "price_above"  // 价格高于
	AlertTypePriceBelow  AlertType = "price_below"  // 价格低于
	AlertTypeChangeAbove AlertType = "change_above" // 涨幅高于
	AlertTypeChangeBelow AlertType = "change_below" // 跌幅低于
	AlertTypeVolumeAbove AlertType = "volume_above" // 成交量高于
)

// AlertStatus 提醒状态
type AlertStatus string

const (
	AlertStatusActive    AlertStatus = "active"    // 激活
	AlertStatusTriggered AlertStatus = "triggered" // 已触发
	AlertStatusDisabled  AlertStatus = "disabled"  // 已禁用
)

// Manager 提醒管理器
type Manager struct {
	alerts    map[int64]*types.Alert          // alertID -> alert
	userIndex map[string]map[int64]bool       // userID -> alertIDs
	symbolIndex map[string]map[int64]bool     // symbol_market -> alertIDs
	mu        sync.RWMutex
	handlers  []AlertHandler
	running   bool
	stopCh    chan struct{}
}

// AlertHandler 提醒处理器
type AlertHandler func(alert *types.Alert, quote *types.Quote)

// NewManager 创建提醒管理器
func NewManager() *Manager {
	return &Manager{
		alerts:      make(map[int64]*types.Alert),
		userIndex:   make(map[string]map[int64]bool),
		symbolIndex: make(map[string]map[int64]bool),
		handlers:    make([]AlertHandler, 0),
		stopCh:      make(chan struct{}),
	}
}

// AddHandler 添加提醒处理器
func (m *Manager) AddHandler(handler AlertHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.handlers = append(m.handlers, handler)
}

// CreateAlert 创建提醒
func (m *Manager) CreateAlert(ctx context.Context, alert *types.Alert) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	alert.ID = atomic.AddInt64(&alertIDCounter, 1)
	alert.Status = string(AlertStatusActive)
	alert.CreatedAt = time.Now()
	alert.UpdatedAt = time.Now()

	m.alerts[alert.ID] = alert

	// 更新用户索引
	if _, ok := m.userIndex[alert.UserID]; !ok {
		m.userIndex[alert.UserID] = make(map[int64]bool)
	}
	m.userIndex[alert.UserID][alert.ID] = true

	// 更新代码索引
	key := fmt.Sprintf("%s_%s", alert.Symbol, alert.Market)
	if _, ok := m.symbolIndex[key]; !ok {
		m.symbolIndex[key] = make(map[int64]bool)
	}
	m.symbolIndex[key][alert.ID] = true

	return nil
}

// GetAlert 获取提醒
func (m *Manager) GetAlert(ctx context.Context, alertID int64) (*types.Alert, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if alert, ok := m.alerts[alertID]; ok {
		return alert, nil
	}
	return nil, fmt.Errorf("alert %d not found", alertID)
}

// GetUserAlerts 获取用户所有提醒
func (m *Manager) GetUserAlerts(ctx context.Context, userID string) ([]*types.Alert, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	alerts := make([]*types.Alert, 0)
	if alertIDs, ok := m.userIndex[userID]; ok {
		for alertID := range alertIDs {
			if alert, ok := m.alerts[alertID]; ok {
				alerts = append(alerts, alert)
			}
		}
	}
	return alerts, nil
}

// GetActiveAlerts 获取活跃提醒
func (m *Manager) GetActiveAlerts(ctx context.Context, userID string) ([]*types.Alert, error) {
	allAlerts, err := m.GetUserAlerts(ctx, userID)
	if err != nil {
		return nil, err
	}

	activeAlerts := make([]*types.Alert, 0)
	for _, alert := range allAlerts {
		if alert.Status == string(AlertStatusActive) {
			activeAlerts = append(activeAlerts, alert)
		}
	}
	return activeAlerts, nil
}

// UpdateAlert 更新提醒
func (m *Manager) UpdateAlert(ctx context.Context, alertID int64, threshold float64, alertType string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	alert, ok := m.alerts[alertID]
	if !ok {
		return fmt.Errorf("alert %d not found", alertID)
	}

	if threshold > 0 {
		alert.Threshold = threshold
	}
	if alertType != "" {
		alert.AlertType = alertType
	}
	alert.UpdatedAt = time.Now()

	return nil
}

// DisableAlert 禁用提醒
func (m *Manager) DisableAlert(ctx context.Context, alertID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	alert, ok := m.alerts[alertID]
	if !ok {
		return fmt.Errorf("alert %d not found", alertID)
	}

	alert.Status = string(AlertStatusDisabled)
	alert.UpdatedAt = time.Now()
	return nil
}

// EnableAlert 启用提醒
func (m *Manager) EnableAlert(ctx context.Context, alertID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	alert, ok := m.alerts[alertID]
	if !ok {
		return fmt.Errorf("alert %d not found", alertID)
	}

	alert.Status = string(AlertStatusActive)
	alert.UpdatedAt = time.Now()
	return nil
}

// DeleteAlert 删除提醒
func (m *Manager) DeleteAlert(ctx context.Context, alertID int64, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	alert, ok := m.alerts[alertID]
	if !ok {
		return fmt.Errorf("alert %d not found", alertID)
	}

	if alert.UserID != userID {
		return fmt.Errorf("unauthorized")
	}

	// 删除索引
	delete(m.alerts, alertID)
	if userAlerts, ok := m.userIndex[userID]; ok {
		delete(userAlerts, alertID)
	}
	key := fmt.Sprintf("%s_%s", alert.Symbol, alert.Market)
	if symbolAlerts, ok := m.symbolIndex[key]; ok {
		delete(symbolAlerts, alertID)
	}

	return nil
}

// CheckQuote 检查行情是否触发提醒
func (m *Manager) CheckQuote(ctx context.Context, quote *types.Quote) []*types.Alert {
	m.mu.Lock()
	defer m.mu.Unlock()

	triggered := make([]*types.Alert, 0)

	key := fmt.Sprintf("%s_%s", quote.Symbol, quote.Market)
	alertIDs, ok := m.symbolIndex[key]
	if !ok {
		return triggered
	}

	for alertID := range alertIDs {
		alert, ok := m.alerts[alertID]
		if !ok || alert.Status != string(AlertStatusActive) {
			continue
		}

		if m.checkCondition(alert, quote) {
			now := time.Now()
			alert.Status = string(AlertStatusTriggered)
			alert.TriggeredAt = &now
			alert.TriggeredValue = quote.Price
			alert.CurrentValue = quote.Price
			alert.UpdatedAt = now

			triggered = append(triggered, alert)

			// 调用处理器
			for _, handler := range m.handlers {
				go handler(alert, quote)
			}
		}
	}

	return triggered
}

// checkCondition 检查条件是否满足
func (m *Manager) checkCondition(alert *types.Alert, quote *types.Quote) bool {
	switch AlertType(alert.AlertType) {
	case AlertTypePriceAbove:
		return quote.Price >= alert.Threshold
	case AlertTypePriceBelow:
		return quote.Price <= alert.Threshold
	case AlertTypeChangeAbove:
		return quote.ChangePct >= alert.Threshold
	case AlertTypeChangeBelow:
		return quote.ChangePct <= -alert.Threshold
	case AlertTypeVolumeAbove:
		return float64(quote.Volume) >= alert.Threshold
	}
	return false
}

// CheckQuotes 批量检查行情
func (m *Manager) CheckQuotes(ctx context.Context, quotes []*types.Quote) []*types.Alert {
	allTriggered := make([]*types.Alert, 0)
	for _, quote := range quotes {
		triggered := m.CheckQuote(ctx, quote)
		allTriggered = append(allTriggered, triggered...)
	}
	return allTriggered
}

// GetSymbolAlerts 获取某个标的的所有提醒
func (m *Manager) GetSymbolAlerts(ctx context.Context, symbol string, market types.Market) ([]*types.Alert, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	alerts := make([]*types.Alert, 0)
	key := fmt.Sprintf("%s_%s", symbol, market)
	if alertIDs, ok := m.symbolIndex[key]; ok {
		for alertID := range alertIDs {
			if alert, ok := m.alerts[alertID]; ok {
				alerts = append(alerts, alert)
			}
		}
	}
	return alerts, nil
}

// GetTriggeredAlerts 获取已触发的提醒
func (m *Manager) GetTriggeredAlerts(ctx context.Context, userID string) ([]*types.Alert, error) {
	allAlerts, err := m.GetUserAlerts(ctx, userID)
	if err != nil {
		return nil, err
	}

	triggered := make([]*types.Alert, 0)
	for _, alert := range allAlerts {
		if alert.Status == string(AlertStatusTriggered) {
			triggered = append(triggered, alert)
		}
	}
	return triggered, nil
}

// ResetAlert 重置提醒(触发后重新激活)
func (m *Manager) ResetAlert(ctx context.Context, alertID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	alert, ok := m.alerts[alertID]
	if !ok {
		return fmt.Errorf("alert %d not found", alertID)
	}

	alert.Status = string(AlertStatusActive)
	alert.TriggeredAt = nil
	alert.TriggeredValue = 0
	alert.UpdatedAt = time.Now()

	return nil
}

// Stats 获取统计信息
func (m *Manager) Stats() *AlertStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &AlertStats{
		TotalAlerts:     len(m.alerts),
		ActiveAlerts:    0,
		TriggeredAlerts: 0,
		DisabledAlerts:  0,
		UserCount:       len(m.userIndex),
		SymbolCount:     len(m.symbolIndex),
	}

	for _, alert := range m.alerts {
		switch AlertStatus(alert.Status) {
		case AlertStatusActive:
			stats.ActiveAlerts++
		case AlertStatusTriggered:
			stats.TriggeredAlerts++
		case AlertStatusDisabled:
			stats.DisabledAlerts++
		}
	}

	return stats
}

// AlertStats 提醒统计
type AlertStats struct {
	TotalAlerts     int `json:"total_alerts"`
	ActiveAlerts    int `json:"active_alerts"`
	TriggeredAlerts int `json:"triggered_alerts"`
	DisabledAlerts  int `json:"disabled_alerts"`
	UserCount       int `json:"user_count"`
	SymbolCount     int `json:"symbol_count"`
}
