// Package market 市场服务测试
package market

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cloudwego/eino/vdocstool/tools/finance/types"
)

func TestServiceCreation(t *testing.T) {
	service := NewService()
	if service == nil {
		t.Fatal("NewService returned nil")
	}
	if service.client == nil {
		t.Error("Service should have an HTTP client")
	}
}

func TestGetMarketOverviewWithMock(t *testing.T) {
	// 创建模拟服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 返回模拟的新浪指数数据
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`var hq_str_sh000001="上证指数,3050.00,3040.00,3055.50,3060.00,3020.00,3055.50,3055.50,500000000,650000000000,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0";`))
	}))
	defer server.Close()

	// 注意：由于Service使用硬编码的URL，我们只能测试创建和基本功能
	service := NewService()
	if service == nil {
		t.Fatal("Service should be created")
	}
}

func TestGetMarketOverviewCNMarket(t *testing.T) {
	// 由于实际调用需要网络，这里只测试能够调用不panic
	service := NewService()
	ctx := context.Background()

	// 实际测试会调用真实API，在单测中跳过
	t.Skip("Skipping network-dependent test in unit tests")

	_, err := service.GetMarketOverview(ctx, types.MarketSH)
	if err != nil {
		t.Logf("GetMarketOverview returned error (expected in offline mode): %v", err)
	}
}

func TestGetSectors(t *testing.T) {
	service := NewService()
	ctx := context.Background()

	// 跳过网络依赖测试
	t.Skip("Skipping network-dependent test in unit tests")

	sectors, err := service.GetSectors(ctx, "industry", 10)
	if err != nil {
		t.Logf("GetSectors returned error (expected in offline mode): %v", err)
		return
	}

	if len(sectors) > 10 {
		t.Errorf("Expected at most 10 sectors, got %d", len(sectors))
	}
}

func TestGetNorthFlow(t *testing.T) {
	service := NewService()
	ctx := context.Background()

	// 跳过网络依赖测试
	t.Skip("Skipping network-dependent test in unit tests")

	flow, err := service.GetNorthFlow(ctx)
	if err != nil {
		t.Logf("GetNorthFlow returned error (expected in offline mode): %v", err)
		return
	}

	if flow == nil {
		t.Error("Expected flow data, got nil")
	}
}

func TestParseIndexResponse(t *testing.T) {
	service := NewService()

	testCases := []struct {
		name     string
		body     string
		code     string
		expected string
		hasError bool
	}{
		{
			name:     "valid response",
			body:     `var hq_str_sh000001="上证指数,3050.00,3040.00,3055.50,3060.00,3020.00,3055.50,3055.50,500000000,650000000000,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0";`,
			code:     "sh000001",
			expected: "上证指数",
			hasError: false,
		},
		{
			name:     "empty response",
			body:     `var hq_str_sh000001="";`,
			code:     "sh000001",
			expected: "",
			hasError: true,
		},
		{
			name:     "invalid format",
			body:     `invalid data`,
			code:     "sh000001",
			expected: "",
			hasError: true,
		},
		{
			name:     "insufficient fields",
			body:     `var hq_str_sh000001="name,1,2,3";`,
			code:     "sh000001",
			expected: "",
			hasError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			index, err := service.parseIndexResponse(tc.body, tc.code)

			if tc.hasError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if index.Name != tc.expected {
				t.Errorf("Expected name '%s', got '%s'", tc.expected, index.Name)
			}
		})
	}
}

func TestParseTopListResponse(t *testing.T) {
	service := NewService()

	validJSON := `{
		"data": {
			"diff": [
				{"f2": 10.5, "f3": 5.2, "f4": 0.52, "f5": 1000000, "f6": 10500000, "f12": "000001", "f14": "平安银行"},
				{"f2": 20.3, "f3": 3.1, "f4": 0.61, "f5": 2000000, "f6": 40600000, "f12": "600519", "f14": "贵州茅台"}
			]
		}
	}`

	quotes, err := service.parseTopListResponse([]byte(validJSON), types.MarketSH)
	if err != nil {
		t.Fatalf("parseTopListResponse failed: %v", err)
	}

	if len(quotes) != 2 {
		t.Errorf("Expected 2 quotes, got %d", len(quotes))
	}

	if quotes[0].Symbol != "000001" {
		t.Errorf("Expected symbol '000001', got '%s'", quotes[0].Symbol)
	}

	if quotes[0].Name != "平安银行" {
		t.Errorf("Expected name '平安银行', got '%s'", quotes[0].Name)
	}

	if quotes[0].Price != 10.5 {
		t.Errorf("Expected price 10.5, got %f", quotes[0].Price)
	}

	// 测试无效JSON
	_, err = service.parseTopListResponse([]byte("invalid json"), types.MarketSH)
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}

	// 测试空数据
	emptyJSON := `{"data": {"diff": []}}`
	quotes, err = service.parseTopListResponse([]byte(emptyJSON), types.MarketSH)
	if err != nil {
		t.Fatalf("parseTopListResponse failed for empty data: %v", err)
	}
	if len(quotes) != 0 {
		t.Errorf("Expected 0 quotes for empty data, got %d", len(quotes))
	}
}

func TestParseSectorResponse(t *testing.T) {
	service := NewService()

	validJSON := `{
		"data": {
			"diff": [
				{"f3": 2.5, "f12": "BK0475", "f14": "银行", "f128": "招商银行", "f140": "600036", "f141": 3.2},
				{"f3": 1.8, "f12": "BK0476", "f14": "保险", "f128": "中国平安", "f140": "601318", "f141": 2.1}
			]
		}
	}`

	sectors, err := service.parseSectorResponse([]byte(validJSON))
	if err != nil {
		t.Fatalf("parseSectorResponse failed: %v", err)
	}

	if len(sectors) != 2 {
		t.Errorf("Expected 2 sectors, got %d", len(sectors))
	}

	// 应该按涨跌幅排序
	if sectors[0].ChangePct < sectors[1].ChangePct {
		t.Error("Sectors should be sorted by change percent descending")
	}

	if sectors[0].Name != "银行" {
		t.Errorf("Expected first sector '银行', got '%s'", sectors[0].Name)
	}

	// 测试无效JSON
	_, err = service.parseSectorResponse([]byte("invalid json"))
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestCaching(t *testing.T) {
	service := NewService()

	// 手动设置缓存
	overview := &types.MarketOverview{
		Market:     types.MarketSH,
		IndexName:  "上证指数",
		IndexPrice: 3050.50,
	}
	service.cache.Store("overview_sh", &cachedOverview{
		data: overview,
		time: time.Now(),
	})

	// 验证缓存存在
	cached, ok := service.cache.Load("overview_sh")
	if !ok {
		t.Error("Cache should contain the overview")
	}

	cachedOverview, ok := cached.(*cachedOverview)
	if !ok {
		t.Error("Cached value should be *cachedOverview")
	}

	if cachedOverview.data.IndexName != "上证指数" {
		t.Errorf("Expected cached index name '上证指数', got '%s'", cachedOverview.data.IndexName)
	}
}

func TestNorthFlowData(t *testing.T) {
	flow := &NorthFlowData{
		SHConnect: 10.5,
		SZConnect: 8.3,
		Total:     18.8,
	}

	if flow.SHConnect+flow.SZConnect != flow.Total {
		t.Logf("Note: SH(%f) + SZ(%f) may not equal Total(%f) due to rounding",
			flow.SHConnect, flow.SZConnect, flow.Total)
	}
}
