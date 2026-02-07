package portfolio

import (
	"context"
	"testing"

	"github.com/cloudwego/eino/vdocstool/tools/finance/types"
)

func TestCreatePortfolio(t *testing.T) {
	manager := NewManager()
	ctx := context.Background()

	// 创建组合
	p, err := manager.CreatePortfolio(ctx, "user1", "我的组合", "测试组合", "CNY", 100000)
	if err != nil {
		t.Fatalf("CreatePortfolio failed: %v", err)
	}

	if p.Name != "我的组合" {
		t.Errorf("Portfolio name should be '我的组合', got '%s'", p.Name)
	}

	if p.UserID != "user1" {
		t.Errorf("Portfolio UserID should be 'user1', got '%s'", p.UserID)
	}

	if p.InitialCapital != 100000 {
		t.Errorf("Portfolio InitialCapital should be 100000, got %f", p.InitialCapital)
	}

	// 不能重复创建
	_, err = manager.CreatePortfolio(ctx, "user1", "我的组合", "测试组合", "CNY", 100000)
	if err == nil {
		t.Error("Should not be able to create duplicate portfolio")
	}
}

func TestGetPortfolio(t *testing.T) {
	manager := NewManager()
	ctx := context.Background()

	// 创建组合
	_, err := manager.CreatePortfolio(ctx, "user1", "测试组合", "", "CNY", 0)
	if err != nil {
		t.Fatalf("CreatePortfolio failed: %v", err)
	}

	// 获取组合
	p, err := manager.GetPortfolio(ctx, "user1", "测试组合")
	if err != nil {
		t.Fatalf("GetPortfolio failed: %v", err)
	}

	if p.Name != "测试组合" {
		t.Errorf("Portfolio name should be '测试组合', got '%s'", p.Name)
	}

	// 获取不存在的组合
	_, err = manager.GetPortfolio(ctx, "user1", "不存在")
	if err == nil {
		t.Error("Should not be able to get non-existent portfolio")
	}
}

func TestGetPortfolios(t *testing.T) {
	manager := NewManager()
	ctx := context.Background()

	// 创建多个组合
	manager.CreatePortfolio(ctx, "user1", "组合1", "", "CNY", 0)
	manager.CreatePortfolio(ctx, "user1", "组合2", "", "CNY", 0)
	manager.CreatePortfolio(ctx, "user2", "组合3", "", "CNY", 0)

	// 获取user1的组合
	portfolios, err := manager.GetPortfolios(ctx, "user1")
	if err != nil {
		t.Fatalf("GetPortfolios failed: %v", err)
	}

	if len(portfolios) != 2 {
		t.Errorf("User1 should have 2 portfolios, got %d", len(portfolios))
	}

	// 获取user2的组合
	portfolios, err = manager.GetPortfolios(ctx, "user2")
	if err != nil {
		t.Fatalf("GetPortfolios failed: %v", err)
	}

	if len(portfolios) != 1 {
		t.Errorf("User2 should have 1 portfolio, got %d", len(portfolios))
	}
}

func TestDeletePortfolio(t *testing.T) {
	manager := NewManager()
	ctx := context.Background()

	// 创建组合
	manager.CreatePortfolio(ctx, "user1", "待删除", "", "CNY", 0)

	// 删除组合
	err := manager.DeletePortfolio(ctx, "user1", "待删除")
	if err != nil {
		t.Fatalf("DeletePortfolio failed: %v", err)
	}

	// 再次获取应该失败
	_, err = manager.GetPortfolio(ctx, "user1", "待删除")
	if err == nil {
		t.Error("Should not be able to get deleted portfolio")
	}

	// 删除不存在的组合
	err = manager.DeletePortfolio(ctx, "user1", "不存在")
	if err == nil {
		t.Error("Should not be able to delete non-existent portfolio")
	}
}

func TestAddPosition(t *testing.T) {
	manager := NewManager()
	ctx := context.Background()

	// 创建组合
	manager.CreatePortfolio(ctx, "user1", "持仓测试", "", "CNY", 100000)

	// 添加持仓
	pos := &types.Position{
		Symbol:   "600519",
		Market:   types.MarketSH,
		Quantity: 100,
		AvgCost:  1800,
	}
	err := manager.AddPosition(ctx, "user1", "持仓测试", pos)
	if err != nil {
		t.Fatalf("AddPosition failed: %v", err)
	}

	// 检查持仓
	p, _ := manager.GetPortfolio(ctx, "user1", "持仓测试")
	if len(p.Positions) != 1 {
		t.Errorf("Portfolio should have 1 position, got %d", len(p.Positions))
	}

	if p.Positions[0].Symbol != "600519" {
		t.Errorf("Position symbol should be '600519', got '%s'", p.Positions[0].Symbol)
	}

	// 再次添加同一股票应该合并
	pos2 := &types.Position{
		Symbol:   "600519",
		Market:   types.MarketSH,
		Quantity: 100,
		AvgCost:  1900,
	}
	err = manager.AddPosition(ctx, "user1", "持仓测试", pos2)
	if err != nil {
		t.Fatalf("AddPosition failed: %v", err)
	}

	p, _ = manager.GetPortfolio(ctx, "user1", "持仓测试")
	if len(p.Positions) != 1 {
		t.Errorf("Portfolio should still have 1 position after merge, got %d", len(p.Positions))
	}

	if p.Positions[0].Quantity != 200 {
		t.Errorf("Position quantity should be 200 after merge, got %f", p.Positions[0].Quantity)
	}

	// 平均成本应该更新
	expectedAvgCost := (100*1800 + 100*1900) / 200.0
	if p.Positions[0].AvgCost != expectedAvgCost {
		t.Errorf("Position avg cost should be %f, got %f", expectedAvgCost, p.Positions[0].AvgCost)
	}
}

func TestUpdatePosition(t *testing.T) {
	manager := NewManager()
	ctx := context.Background()

	// 创建组合并添加持仓
	manager.CreatePortfolio(ctx, "user1", "更新测试", "", "CNY", 100000)
	manager.AddPosition(ctx, "user1", "更新测试", &types.Position{
		Symbol:   "600519",
		Market:   types.MarketSH,
		Quantity: 100,
		AvgCost:  1800,
	})

	// 更新持仓
	err := manager.UpdatePosition(ctx, "user1", "更新测试", "600519", types.MarketSH, 50, 1850)
	if err != nil {
		t.Fatalf("UpdatePosition failed: %v", err)
	}

	p, _ := manager.GetPortfolio(ctx, "user1", "更新测试")
	if p.Positions[0].Quantity != 50 {
		t.Errorf("Position quantity should be 50 after update, got %f", p.Positions[0].Quantity)
	}

	if p.Positions[0].AvgCost != 1850 {
		t.Errorf("Position avg cost should be 1850 after update, got %f", p.Positions[0].AvgCost)
	}
}

func TestRemovePosition(t *testing.T) {
	manager := NewManager()
	ctx := context.Background()

	// 创建组合并添加持仓
	manager.CreatePortfolio(ctx, "user1", "删除测试", "", "CNY", 100000)
	manager.AddPosition(ctx, "user1", "删除测试", &types.Position{
		Symbol:   "600519",
		Market:   types.MarketSH,
		Quantity: 100,
		AvgCost:  1800,
	})

	// 删除持仓
	err := manager.RemovePosition(ctx, "user1", "删除测试", "600519", types.MarketSH)
	if err != nil {
		t.Fatalf("RemovePosition failed: %v", err)
	}

	p, _ := manager.GetPortfolio(ctx, "user1", "删除测试")
	if len(p.Positions) != 0 {
		t.Errorf("Portfolio should have 0 positions after remove, got %d", len(p.Positions))
	}
}

func TestRecordTransaction(t *testing.T) {
	manager := NewManager()
	ctx := context.Background()

	// 创建组合
	manager.CreatePortfolio(ctx, "user1", "交易测试", "", "CNY", 100000)

	// 买入交易
	tx := &types.Transaction{
		Symbol:    "600519",
		Market:    types.MarketSH,
		TradeType: "buy",
		Quantity:  100,
		Price:     1800,
		Fee:       5,
	}
	err := manager.RecordTransaction(ctx, "user1", "交易测试", tx)
	if err != nil {
		t.Fatalf("RecordTransaction failed: %v", err)
	}

	p, _ := manager.GetPortfolio(ctx, "user1", "交易测试")
	if len(p.Positions) != 1 {
		t.Errorf("Portfolio should have 1 position after buy, got %d", len(p.Positions))
	}

	// 卖出交易
	tx2 := &types.Transaction{
		Symbol:    "600519",
		Market:    types.MarketSH,
		TradeType: "sell",
		Quantity:  50,
		Price:     1900,
		Fee:       5,
	}
	err = manager.RecordTransaction(ctx, "user1", "交易测试", tx2)
	if err != nil {
		t.Fatalf("RecordTransaction failed: %v", err)
	}

	p, _ = manager.GetPortfolio(ctx, "user1", "交易测试")
	if p.Positions[0].Quantity != 50 {
		t.Errorf("Position quantity should be 50 after sell, got %f", p.Positions[0].Quantity)
	}
}

func TestUpdatePrices(t *testing.T) {
	manager := NewManager()
	ctx := context.Background()

	// 创建组合并添加持仓
	manager.CreatePortfolio(ctx, "user1", "价格测试", "", "CNY", 100000)
	manager.AddPosition(ctx, "user1", "价格测试", &types.Position{
		Symbol:   "600519",
		Market:   types.MarketSH,
		Quantity: 100,
		AvgCost:  1800,
	})

	// 更新价格
	quotes := map[string]*types.Quote{
		"600519_sh": {
			Symbol: "600519",
			Market: types.MarketSH,
			Price:  1900,
		},
	}
	err := manager.UpdatePrices(ctx, "user1", "价格测试", quotes)
	if err != nil {
		t.Fatalf("UpdatePrices failed: %v", err)
	}

	p, _ := manager.GetPortfolio(ctx, "user1", "价格测试")
	pos := p.Positions[0]

	if pos.CurrentPrice != 1900 {
		t.Errorf("Position current price should be 1900, got %f", pos.CurrentPrice)
	}

	expectedMarketValue := 100 * 1900.0
	if pos.MarketValue != expectedMarketValue {
		t.Errorf("Position market value should be %f, got %f", expectedMarketValue, pos.MarketValue)
	}

	expectedProfitLoss := 100.0 * (1900 - 1800)
	if pos.ProfitLoss != expectedProfitLoss {
		t.Errorf("Position profit loss should be %f, got %f", expectedProfitLoss, pos.ProfitLoss)
	}
}

func TestGetPortfolioSummary(t *testing.T) {
	manager := NewManager()
	ctx := context.Background()

	// 创建组合并添加持仓
	manager.CreatePortfolio(ctx, "user1", "概要测试", "", "CNY", 100000)
	manager.AddPosition(ctx, "user1", "概要测试", &types.Position{
		Symbol:    "600519",
		Market:    types.MarketSH,
		AssetType: types.AssetTypeStock,
		Quantity:  100,
		AvgCost:   1800,
	})
	manager.AddPosition(ctx, "user1", "概要测试", &types.Position{
		Symbol:    "510300",
		Market:    types.MarketSH,
		AssetType: types.AssetTypeETF,
		Quantity:  1000,
		AvgCost:   4,
	})

	// 获取概要
	summary, err := manager.GetPortfolioSummary(ctx, "user1", "概要测试")
	if err != nil {
		t.Fatalf("GetPortfolioSummary failed: %v", err)
	}

	if summary.PositionCount != 2 {
		t.Errorf("Summary position count should be 2, got %d", summary.PositionCount)
	}

	expectedTotalCost := 100*1800 + 1000*4.0
	if summary.TotalCost != expectedTotalCost {
		t.Errorf("Summary total cost should be %f, got %f", expectedTotalCost, summary.TotalCost)
	}
}
