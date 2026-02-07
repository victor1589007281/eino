//go:build integration

// Package integration 财经工具集成测试
package integration

import (
	"context"
	"testing"
	"time"

	"github.com/cloudwego/eino/vdocstool/tools/finance"
	"github.com/cloudwego/eino/vdocstool/tools/finance/service"
	"github.com/cloudwego/eino/vdocstool/tools/finance/sources/eastmoney"
	"github.com/cloudwego/eino/vdocstool/tools/finance/sources/sina"
)

// TestEastMoneyQuote 测试东方财富行情获取
func TestEastMoneyQuote(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	source := eastmoney.New(nil)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 测试获取贵州茅台行情
	quote, err := source.GetQuote(ctx, "600519")
	if err != nil {
		t.Fatalf("Failed to get quote: %v", err)
	}

	if quote == nil {
		t.Fatal("Quote is nil")
	}

	t.Logf("茅台行情: %s (%s) 价格: %.2f 涨跌: %.2f%%", 
		quote.Name, quote.Symbol, quote.Price, quote.ChangePct)

	// 验证数据
	if quote.Symbol != "600519" {
		t.Errorf("Expected symbol 600519, got %s", quote.Symbol)
	}
	if quote.Price <= 0 {
		t.Error("Expected positive price")
	}
	if quote.Name == "" {
		t.Error("Expected non-empty name")
	}
}

// TestEastMoneySearch 测试东方财富搜索
func TestEastMoneySearch(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	source := eastmoney.New(nil)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 测试搜索
	results, err := source.Search(ctx, "茅台")
	if err != nil {
		t.Fatalf("Failed to search: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("Expected search results")
	}

	t.Logf("搜索 '茅台' 找到 %d 个结果", len(results))
	for i, r := range results {
		if i >= 5 {
			break
		}
		t.Logf("  %d. %s (%s) - %s", i+1, r.Name, r.Symbol, r.Market)
	}
}

// TestEastMoneyKLine 测试东方财富K线获取
func TestEastMoneyKLine(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	source := eastmoney.New(nil)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 测试获取日K线
	klines, err := source.GetKLine(ctx, "600519", "1d", 30)
	if err != nil {
		t.Fatalf("Failed to get kline: %v", err)
	}

	if len(klines) == 0 {
		t.Fatal("Expected kline data")
	}

	t.Logf("获取到 %d 条K线数据", len(klines))
	// 显示最近5条
	for i := len(klines) - 5; i < len(klines); i++ {
		if i < 0 {
			continue
		}
		k := klines[i]
		t.Logf("  %s: 开%.2f 高%.2f 低%.2f 收%.2f", 
			k.Timestamp.Format("2006-01-02"), k.Open, k.High, k.Low, k.Close)
	}
}

// TestSinaQuote 测试新浪财经行情获取
func TestSinaQuote(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	source := sina.New(nil)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 测试获取上证指数
	quote, err := source.GetQuote(ctx, "sh000001")
	if err != nil {
		t.Fatalf("Failed to get quote: %v", err)
	}

	if quote == nil {
		t.Fatal("Quote is nil")
	}

	t.Logf("上证指数: %s 价格: %.2f 涨跌: %.2f%%", 
		quote.Name, quote.Price, quote.ChangePct)

	if quote.Price <= 0 {
		t.Error("Expected positive price")
	}
}

// TestSinaBatchQuotes 测试新浪批量行情
func TestSinaBatchQuotes(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	source := sina.New(nil)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 测试批量获取
	symbols := []string{"sh600519", "sz000001", "sh000001"}
	quotes, err := source.GetQuotes(ctx, symbols)
	if err != nil {
		t.Fatalf("Failed to get quotes: %v", err)
	}

	t.Logf("获取到 %d 条行情数据", len(quotes))
	for _, q := range quotes {
		t.Logf("  %s (%s): %.2f (%.2f%%)", q.Name, q.Symbol, q.Price, q.ChangePct)
	}
}

// TestFinanceService 测试财经服务
func TestFinanceService(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// 创建服务 (不使用Redis)
	svc, err := service.NewFinanceService(&service.ServiceConfig{
		EnableCache:  true,
		EnableRouter: true,
	})
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}
	defer svc.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// 测试获取行情
	quote, err := svc.GetQuote(ctx, "600519")
	if err != nil {
		t.Fatalf("Failed to get quote: %v", err)
	}

	t.Logf("服务获取行情: %s (%.2f)", quote.Name, quote.Price)

	// 测试搜索
	results, err := svc.Search(ctx, "银行")
	if err != nil {
		t.Fatalf("Failed to search: %v", err)
	}

	t.Logf("搜索 '银行' 找到 %d 个结果", len(results))

	// 测试K线
	klines, err := svc.GetKLine(ctx, "600519", "1d", 10)
	if err != nil {
		t.Fatalf("Failed to get kline: %v", err)
	}

	t.Logf("获取到 %d 条K线", len(klines))
}

// TestFinanceTool 测试财经MCP工具
func TestFinanceTool(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// 创建工具
	tool, err := finance.NewTool(nil)
	if err != nil {
		t.Fatalf("Failed to create tool: %v", err)
	}
	defer tool.Close()

	t.Log("财经工具创建成功")

	// 检查Skills
	skills := finance.GetSkillsYAML()
	if skills == "" {
		t.Error("Expected non-empty skills YAML")
	}
	t.Log("Skills配置加载成功")
}

// TestRouterFailover 测试路由器故障转移
func TestRouterFailover(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// 创建服务
	svc, err := service.NewFinanceService(&service.ServiceConfig{
		EnableCache:  true,
		EnableRouter: true,
	})
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}
	defer svc.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 连续获取行情，验证路由稳定性
	for i := 0; i < 3; i++ {
		quote, err := svc.GetQuote(ctx, "600519")
		if err != nil {
			t.Logf("Request %d failed: %v", i+1, err)
		} else {
			t.Logf("Request %d success: %s = %.2f (source: %s)", 
				i+1, quote.Symbol, quote.Price, quote.Source)
		}
		time.Sleep(500 * time.Millisecond)
	}

	// 获取路由器状态
	stats := svc.GetRouterStats()
	t.Logf("路由器状态: 总数据源=%d, 健康数据源=%d", 
		stats.TotalSources, stats.HealthySources)

	// 获取健康状态
	health := svc.GetSourceHealth()
	for name, h := range health {
		t.Logf("  %s: %s (成功率: %.2f%%)", name, h.Status, h.SuccessRate*100)
	}
}
