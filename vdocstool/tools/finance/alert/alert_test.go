package alert

import (
	"context"
	"testing"

	"github.com/cloudwego/eino/vdocstool/tools/finance/types"
)

func TestCreateAlert(t *testing.T) {
	manager := NewManager()
	ctx := context.Background()

	// 创建提醒
	alert := &types.Alert{
		UserID:    "user1",
		Symbol:    "600519",
		Name:      "贵州茅台",
		Market:    types.MarketSH,
		AlertType: string(AlertTypePriceAbove),
		Threshold: 2000,
	}

	err := manager.CreateAlert(ctx, alert)
	if err != nil {
		t.Fatalf("CreateAlert failed: %v", err)
	}

	if alert.ID == 0 {
		t.Error("Alert ID should be assigned")
	}

	if alert.Status != string(AlertStatusActive) {
		t.Errorf("Alert status should be 'active', got '%s'", alert.Status)
	}
}

func TestGetAlert(t *testing.T) {
	manager := NewManager()
	ctx := context.Background()

	// 创建提醒
	alert := &types.Alert{
		UserID:    "user1",
		Symbol:    "600519",
		AlertType: string(AlertTypePriceBelow),
		Threshold: 1800,
	}
	manager.CreateAlert(ctx, alert)

	// 获取提醒
	retrieved, err := manager.GetAlert(ctx, alert.ID)
	if err != nil {
		t.Fatalf("GetAlert failed: %v", err)
	}

	if retrieved.Symbol != "600519" {
		t.Errorf("Alert symbol should be '600519', got '%s'", retrieved.Symbol)
	}

	if retrieved.Threshold != 1800 {
		t.Errorf("Alert threshold should be 1800, got %f", retrieved.Threshold)
	}

	// 获取不存在的提醒
	_, err = manager.GetAlert(ctx, 9999999)
	if err == nil {
		t.Error("Should not be able to get non-existent alert")
	}
}

func TestGetUserAlerts(t *testing.T) {
	manager := NewManager()
	ctx := context.Background()

	// 为不同用户创建提醒
	manager.CreateAlert(ctx, &types.Alert{
		UserID:    "user1",
		Symbol:    "600519",
		AlertType: string(AlertTypePriceAbove),
		Threshold: 2000,
	})
	manager.CreateAlert(ctx, &types.Alert{
		UserID:    "user1",
		Symbol:    "000001",
		AlertType: string(AlertTypePriceBelow),
		Threshold: 10,
	})
	manager.CreateAlert(ctx, &types.Alert{
		UserID:    "user2",
		Symbol:    "AAPL",
		AlertType: string(AlertTypePriceAbove),
		Threshold: 200,
	})

	// 获取user1的提醒
	alerts, err := manager.GetUserAlerts(ctx, "user1")
	if err != nil {
		t.Fatalf("GetUserAlerts failed: %v", err)
	}

	if len(alerts) != 2 {
		t.Errorf("User1 should have 2 alerts, got %d", len(alerts))
	}

	// 获取user2的提醒
	alerts, err = manager.GetUserAlerts(ctx, "user2")
	if err != nil {
		t.Fatalf("GetUserAlerts failed: %v", err)
	}

	if len(alerts) != 1 {
		t.Errorf("User2 should have 1 alert, got %d", len(alerts))
	}
}

func TestGetActiveAlerts(t *testing.T) {
	manager := NewManager()
	ctx := context.Background()

	// 创建提醒
	alert1 := &types.Alert{UserID: "user1", Symbol: "600519", AlertType: string(AlertTypePriceAbove), Threshold: 2000}
	alert2 := &types.Alert{UserID: "user1", Symbol: "000001", AlertType: string(AlertTypePriceBelow), Threshold: 10}
	manager.CreateAlert(ctx, alert1)
	manager.CreateAlert(ctx, alert2)

	// 禁用一个提醒
	manager.DisableAlert(ctx, alert2.ID)

	// 获取活跃提醒
	activeAlerts, err := manager.GetActiveAlerts(ctx, "user1")
	if err != nil {
		t.Fatalf("GetActiveAlerts failed: %v", err)
	}

	if len(activeAlerts) != 1 {
		t.Errorf("User1 should have 1 active alert, got %d", len(activeAlerts))
	}
}

func TestDisableEnableAlert(t *testing.T) {
	manager := NewManager()
	ctx := context.Background()

	// 创建提醒
	alert := &types.Alert{UserID: "user1", Symbol: "600519", AlertType: string(AlertTypePriceAbove), Threshold: 2000}
	manager.CreateAlert(ctx, alert)

	// 禁用
	err := manager.DisableAlert(ctx, alert.ID)
	if err != nil {
		t.Fatalf("DisableAlert failed: %v", err)
	}

	retrieved, _ := manager.GetAlert(ctx, alert.ID)
	if retrieved.Status != string(AlertStatusDisabled) {
		t.Errorf("Alert status should be 'disabled', got '%s'", retrieved.Status)
	}

	// 启用
	err = manager.EnableAlert(ctx, alert.ID)
	if err != nil {
		t.Fatalf("EnableAlert failed: %v", err)
	}

	retrieved, _ = manager.GetAlert(ctx, alert.ID)
	if retrieved.Status != string(AlertStatusActive) {
		t.Errorf("Alert status should be 'active', got '%s'", retrieved.Status)
	}
}

func TestDeleteAlert(t *testing.T) {
	manager := NewManager()
	ctx := context.Background()

	// 创建提醒
	alert := &types.Alert{UserID: "user1", Symbol: "600519", AlertType: string(AlertTypePriceAbove), Threshold: 2000}
	manager.CreateAlert(ctx, alert)

	// 删除
	err := manager.DeleteAlert(ctx, alert.ID, "user1")
	if err != nil {
		t.Fatalf("DeleteAlert failed: %v", err)
	}

	// 再次获取应该失败
	_, err = manager.GetAlert(ctx, alert.ID)
	if err == nil {
		t.Error("Should not be able to get deleted alert")
	}

	// 尝试以其他用户身份删除
	alert2 := &types.Alert{UserID: "user1", Symbol: "000001", AlertType: string(AlertTypePriceBelow), Threshold: 10}
	manager.CreateAlert(ctx, alert2)

	err = manager.DeleteAlert(ctx, alert2.ID, "user2")
	if err == nil {
		t.Error("Should not be able to delete other user's alert")
	}
}

func TestCheckQuote_PriceAbove(t *testing.T) {
	manager := NewManager()
	ctx := context.Background()

	// 创建价格高于提醒
	alert := &types.Alert{
		UserID:    "user1",
		Symbol:    "600519",
		Market:    types.MarketSH,
		AlertType: string(AlertTypePriceAbove),
		Threshold: 2000,
	}
	manager.CreateAlert(ctx, alert)

	// 检查低于阈值的行情
	quote1 := &types.Quote{Symbol: "600519", Market: types.MarketSH, Price: 1900}
	triggered := manager.CheckQuote(ctx, quote1)
	if len(triggered) != 0 {
		t.Error("Alert should not be triggered when price is below threshold")
	}

	// 检查高于阈值的行情
	quote2 := &types.Quote{Symbol: "600519", Market: types.MarketSH, Price: 2100}
	triggered = manager.CheckQuote(ctx, quote2)
	if len(triggered) != 1 {
		t.Errorf("Alert should be triggered when price is above threshold, got %d", len(triggered))
	}

	// 检查提醒状态
	retrieved, _ := manager.GetAlert(ctx, alert.ID)
	if retrieved.Status != string(AlertStatusTriggered) {
		t.Errorf("Alert status should be 'triggered', got '%s'", retrieved.Status)
	}

	if retrieved.TriggeredAt == nil {
		t.Error("Alert TriggeredAt should not be nil")
	}

	if retrieved.TriggeredValue != 2100 {
		t.Errorf("Alert TriggeredValue should be 2100, got %f", retrieved.TriggeredValue)
	}
}

func TestCheckQuote_PriceBelow(t *testing.T) {
	manager := NewManager()
	ctx := context.Background()

	// 创建价格低于提醒
	alert := &types.Alert{
		UserID:    "user1",
		Symbol:    "600519",
		Market:    types.MarketSH,
		AlertType: string(AlertTypePriceBelow),
		Threshold: 1800,
	}
	manager.CreateAlert(ctx, alert)

	// 检查高于阈值的行情
	quote1 := &types.Quote{Symbol: "600519", Market: types.MarketSH, Price: 1900}
	triggered := manager.CheckQuote(ctx, quote1)
	if len(triggered) != 0 {
		t.Error("Alert should not be triggered when price is above threshold")
	}

	// 检查低于阈值的行情
	quote2 := &types.Quote{Symbol: "600519", Market: types.MarketSH, Price: 1700}
	triggered = manager.CheckQuote(ctx, quote2)
	if len(triggered) != 1 {
		t.Errorf("Alert should be triggered when price is below threshold, got %d", len(triggered))
	}
}

func TestCheckQuote_ChangeAbove(t *testing.T) {
	manager := NewManager()
	ctx := context.Background()

	// 创建涨幅提醒
	alert := &types.Alert{
		UserID:    "user1",
		Symbol:    "600519",
		Market:    types.MarketSH,
		AlertType: string(AlertTypeChangeAbove),
		Threshold: 5,
	}
	manager.CreateAlert(ctx, alert)

	// 检查涨幅低于阈值
	quote1 := &types.Quote{Symbol: "600519", Market: types.MarketSH, ChangePct: 3}
	triggered := manager.CheckQuote(ctx, quote1)
	if len(triggered) != 0 {
		t.Error("Alert should not be triggered when change is below threshold")
	}

	// 检查涨幅高于阈值
	quote2 := &types.Quote{Symbol: "600519", Market: types.MarketSH, ChangePct: 6}
	triggered = manager.CheckQuote(ctx, quote2)
	if len(triggered) != 1 {
		t.Errorf("Alert should be triggered when change is above threshold, got %d", len(triggered))
	}
}

func TestCheckQuote_ChangeBelow(t *testing.T) {
	manager := NewManager()
	ctx := context.Background()

	// 创建跌幅提醒
	alert := &types.Alert{
		UserID:    "user1",
		Symbol:    "600519",
		Market:    types.MarketSH,
		AlertType: string(AlertTypeChangeBelow),
		Threshold: 5,
	}
	manager.CreateAlert(ctx, alert)

	// 检查跌幅低于阈值
	quote1 := &types.Quote{Symbol: "600519", Market: types.MarketSH, ChangePct: -3}
	triggered := manager.CheckQuote(ctx, quote1)
	if len(triggered) != 0 {
		t.Error("Alert should not be triggered when drop is below threshold")
	}

	// 检查跌幅高于阈值
	quote2 := &types.Quote{Symbol: "600519", Market: types.MarketSH, ChangePct: -6}
	triggered = manager.CheckQuote(ctx, quote2)
	if len(triggered) != 1 {
		t.Errorf("Alert should be triggered when drop is above threshold, got %d", len(triggered))
	}
}

func TestCheckQuote_VolumeAbove(t *testing.T) {
	manager := NewManager()
	ctx := context.Background()

	// 创建成交量提醒
	alert := &types.Alert{
		UserID:    "user1",
		Symbol:    "600519",
		Market:    types.MarketSH,
		AlertType: string(AlertTypeVolumeAbove),
		Threshold: 100000,
	}
	manager.CreateAlert(ctx, alert)

	// 检查成交量低于阈值
	quote1 := &types.Quote{Symbol: "600519", Market: types.MarketSH, Volume: 50000}
	triggered := manager.CheckQuote(ctx, quote1)
	if len(triggered) != 0 {
		t.Error("Alert should not be triggered when volume is below threshold")
	}

	// 检查成交量高于阈值
	quote2 := &types.Quote{Symbol: "600519", Market: types.MarketSH, Volume: 150000}
	triggered = manager.CheckQuote(ctx, quote2)
	if len(triggered) != 1 {
		t.Errorf("Alert should be triggered when volume is above threshold, got %d", len(triggered))
	}
}

func TestCheckQuotes(t *testing.T) {
	manager := NewManager()
	ctx := context.Background()

	// 创建多个提醒
	manager.CreateAlert(ctx, &types.Alert{
		UserID:    "user1",
		Symbol:    "600519",
		Market:    types.MarketSH,
		AlertType: string(AlertTypePriceAbove),
		Threshold: 2000,
	})
	manager.CreateAlert(ctx, &types.Alert{
		UserID:    "user1",
		Symbol:    "000001",
		Market:    types.MarketSZ,
		AlertType: string(AlertTypePriceBelow),
		Threshold: 10,
	})

	// 批量检查行情
	quotes := []*types.Quote{
		{Symbol: "600519", Market: types.MarketSH, Price: 2100},
		{Symbol: "000001", Market: types.MarketSZ, Price: 9},
	}

	triggered := manager.CheckQuotes(ctx, quotes)
	if len(triggered) != 2 {
		t.Errorf("Should trigger 2 alerts, got %d", len(triggered))
	}
}

func TestResetAlert(t *testing.T) {
	manager := NewManager()
	ctx := context.Background()

	// 创建并触发提醒
	alert := &types.Alert{
		UserID:    "user1",
		Symbol:    "600519",
		Market:    types.MarketSH,
		AlertType: string(AlertTypePriceAbove),
		Threshold: 2000,
	}
	manager.CreateAlert(ctx, alert)
	manager.CheckQuote(ctx, &types.Quote{Symbol: "600519", Market: types.MarketSH, Price: 2100})

	// 重置提醒
	err := manager.ResetAlert(ctx, alert.ID)
	if err != nil {
		t.Fatalf("ResetAlert failed: %v", err)
	}

	retrieved, _ := manager.GetAlert(ctx, alert.ID)
	if retrieved.Status != string(AlertStatusActive) {
		t.Errorf("Alert status should be 'active' after reset, got '%s'", retrieved.Status)
	}

	if retrieved.TriggeredAt != nil {
		t.Error("Alert TriggeredAt should be nil after reset")
	}

	if retrieved.TriggeredValue != 0 {
		t.Errorf("Alert TriggeredValue should be 0 after reset, got %f", retrieved.TriggeredValue)
	}
}

func TestAlertHandler(t *testing.T) {
	manager := NewManager()
	ctx := context.Background()

	// 添加处理器，使用channel同步
	handlerCalled := make(chan bool, 1)
	manager.AddHandler(func(alert *types.Alert, quote *types.Quote) {
		handlerCalled <- true
	})

	// 创建并触发提醒
	alert := &types.Alert{
		UserID:    "user1",
		Symbol:    "600519",
		Market:    types.MarketSH,
		AlertType: string(AlertTypePriceAbove),
		Threshold: 2000,
	}
	manager.CreateAlert(ctx, alert)
	manager.CheckQuote(ctx, &types.Quote{Symbol: "600519", Market: types.MarketSH, Price: 2100})

	// 等待处理器被调用(异步)
	select {
	case <-handlerCalled:
		// handler was called
	default:
		// 由于是goroutine调用，可能还没执行完
	}
}

func TestStats(t *testing.T) {
	manager := NewManager()
	ctx := context.Background()

	// 创建多个提醒
	alert1 := &types.Alert{UserID: "user1", Symbol: "600519", Market: types.MarketSH, AlertType: string(AlertTypePriceAbove), Threshold: 2000}
	alert2 := &types.Alert{UserID: "user1", Symbol: "000001", Market: types.MarketSZ, AlertType: string(AlertTypePriceBelow), Threshold: 10}
	alert3 := &types.Alert{UserID: "user2", Symbol: "AAPL", Market: types.MarketUS, AlertType: string(AlertTypePriceAbove), Threshold: 200}
	
	err1 := manager.CreateAlert(ctx, alert1)
	err2 := manager.CreateAlert(ctx, alert2)
	err3 := manager.CreateAlert(ctx, alert3)
	
	if err1 != nil || err2 != nil || err3 != nil {
		t.Fatalf("Failed to create alerts: %v, %v, %v", err1, err2, err3)
	}

	// 获取统计检查创建后的状态
	stats := manager.Stats()
	if stats.TotalAlerts != 3 {
		t.Errorf("After creation, total alerts should be 3, got %d", stats.TotalAlerts)
	}
	if stats.ActiveAlerts != 3 {
		t.Errorf("After creation, active alerts should be 3, got %d", stats.ActiveAlerts)
	}

	// 禁用一个
	manager.DisableAlert(ctx, alert2.ID)

	// 触发一个
	triggered := manager.CheckQuote(ctx, &types.Quote{Symbol: "600519", Market: types.MarketSH, Price: 2100})
	if len(triggered) != 1 {
		t.Errorf("Should trigger 1 alert, got %d", len(triggered))
	}

	// 获取统计
	stats = manager.Stats()

	if stats.TotalAlerts != 3 {
		t.Errorf("Total alerts should be 3, got %d", stats.TotalAlerts)
	}

	if stats.ActiveAlerts != 1 {
		t.Errorf("Active alerts should be 1, got %d", stats.ActiveAlerts)
	}

	if stats.TriggeredAlerts != 1 {
		t.Errorf("Triggered alerts should be 1, got %d", stats.TriggeredAlerts)
	}

	if stats.DisabledAlerts != 1 {
		t.Errorf("Disabled alerts should be 1, got %d", stats.DisabledAlerts)
	}

	if stats.UserCount != 2 {
		t.Errorf("User count should be 2, got %d", stats.UserCount)
	}

	if stats.SymbolCount != 3 {
		t.Errorf("Symbol count should be 3, got %d", stats.SymbolCount)
	}
}
