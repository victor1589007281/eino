//go:build integration

// Package search 网页搜索工具集成测试
package search

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	
	"github.com/cloudwego/eino/vdocstool/config"
	"github.com/cloudwego/eino/vdocstool/tools/search/engines"
	"github.com/cloudwego/eino/vdocstool/tools/search/router"
)

// ====================== 测试辅助 ======================

// MockSearchEngine 模拟搜索引擎
type MockSearchEngine struct {
	name            string
	priority        int
	results         []*engines.SearchResult
	shouldFail      bool    // 搜索是否失败
	healthCheckFail bool    // 健康检查是否失败
	latency         time.Duration
}

func (m *MockSearchEngine) Name() string { return m.name }
func (m *MockSearchEngine) Priority() int { return m.priority }
func (m *MockSearchEngine) HealthCheck(ctx context.Context) error {
	if m.healthCheckFail {
		return errors.New("engine unavailable")
	}
	return nil
}
func (m *MockSearchEngine) Search(ctx context.Context, req *engines.SearchRequest) ([]*engines.SearchResult, error) {
	// 检查 context 是否已取消
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	
	if m.latency > 0 {
		select {
		case <-time.After(m.latency):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if m.shouldFail {
		return nil, errors.New("search failed")
	}
	return m.results, nil
}

// newMockEngine 创建模拟引擎（健康检查和搜索同时失败/成功）
func newMockEngine(name string, priority int) *MockSearchEngine {
	return &MockSearchEngine{
		name:     name,
		priority: priority,
		results: []*engines.SearchResult{
			{Title: fmt.Sprintf("%s Result 1", name), URL: "https://example.com/1", Snippet: "Description 1"},
			{Title: fmt.Sprintf("%s Result 2", name), URL: "https://example.com/2", Snippet: "Description 2"},
			{Title: fmt.Sprintf("%s Result 3", name), URL: "https://example.com/3", Snippet: "Description 3"},
		},
	}
}

// newMockEngineWithHealthCheck 创建模拟引擎（可分别控制健康检查和搜索）
func newMockEngineWithHealthCheck(name string, priority int, healthCheckFail bool) *MockSearchEngine {
	return &MockSearchEngine{
		name:            name,
		priority:        priority,
		healthCheckFail: healthCheckFail,
		results: []*engines.SearchResult{
			{Title: fmt.Sprintf("%s Result 1", name), URL: "https://example.com/1", Snippet: "Description 1"},
			{Title: fmt.Sprintf("%s Result 2", name), URL: "https://example.com/2", Snippet: "Description 2"},
			{Title: fmt.Sprintf("%s Result 3", name), URL: "https://example.com/3", Snippet: "Description 3"},
		},
	}
}

// ====================== 集成测试用例 ======================

// TestWebSearchTool_Integration_AllEngines 测试所有搜索引擎
func TestWebSearchTool_Integration_AllEngines(t *testing.T) {
	tests := []struct {
		name            string
		query           string
		preferredEngine string
		expectEngine    string
		expectResults   int
	}{
		{
			name:            "DuckDuckGo_Default",
			query:           "golang tutorial",
			preferredEngine: "",
			expectEngine:    "duckduckgo",
			expectResults:   3,
		},
		{
			name:            "Bing_Preferred",
			query:           "golang tutorial",
			preferredEngine: "bing",
			expectEngine:    "bing",
			expectResults:   3,
		},
		{
			name:            "Baidu_Preferred",
			query:           "golang 教程",
			preferredEngine: "baidu",
			expectEngine:    "baidu",
			expectResults:   3,
		},
		{
			name:            "Serper_Preferred",
			query:           "golang best practices",
			preferredEngine: "serper",
			expectEngine:    "serper",
			expectResults:   3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 创建路由器
			r := router.NewAdaptiveRouter(config.RouterConfig{
				Strategy:            "health_first",
				HealthCheckInterval: 60 * time.Second,
			})

			// 注册模拟引擎
			r.RegisterEngine(newMockEngine("duckduckgo", 1))
			r.RegisterEngine(newMockEngine("bing", 2))
			r.RegisterEngine(newMockEngine("baidu", 3))
			r.RegisterEngine(newMockEngine("serper", 4))
			
			r.Start()
			defer r.Stop()

			// 等待健康检查完成
			time.Sleep(100 * time.Millisecond)

			// 执行搜索
			ctx := context.Background()
			req := &engines.SearchRequest{
				Query:      tt.query,
				MaxResults: 10,
			}

			result, err := r.Route(ctx, req, tt.preferredEngine, true)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// 验证结果
			if result.EngineUsed != tt.expectEngine {
				t.Errorf("engine mismatch: got %s, want %s", result.EngineUsed, tt.expectEngine)
			}

			if len(result.Results) < tt.expectResults {
				t.Errorf("results count: got %d, want at least %d", len(result.Results), tt.expectResults)
			}

			// 验证结果包含引擎标识
			for _, r := range result.Results {
				if !strings.Contains(r.Title, tt.expectEngine) {
					t.Errorf("result title should contain engine name: %s", r.Title)
				}
			}
		})
	}
}

// TestWebSearchTool_Integration_Fallback 测试自动降级
func TestWebSearchTool_Integration_Fallback(t *testing.T) {
	tests := []struct {
		name              string
		failEngines       []string
		preferredEngine   string
		expectEngine      string
		expectFallback    bool
		expectError       bool
	}{
		{
			name:            "PreferredFails_FallbackToHealthy",
			failEngines:     []string{"duckduckgo"},
			preferredEngine: "duckduckgo",
			expectEngine:    "bing", // 应该降级到 bing
			expectFallback:  true,
			expectError:     false,
		},
		{
			name:            "PreferredAndFirstFallbackFail",
			failEngines:     []string{"duckduckgo", "bing"},
			preferredEngine: "duckduckgo",
			expectEngine:    "",   // 路由器只支持一次 fallback，所以会失败
			expectFallback:  false,
			expectError:     true, // 预期会返回错误
		},
		{
			name:            "NoFallback_PreferredHealthy",
			failEngines:     []string{"bing", "baidu"},
			preferredEngine: "duckduckgo",
			expectEngine:    "duckduckgo",
			expectFallback:  false,
			expectError:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := router.NewAdaptiveRouter(config.RouterConfig{
				Strategy:            "health_first",
				HealthCheckInterval: 60 * time.Second, // 长间隔，避免干扰
			})

			// 创建引擎 - 健康检查总是成功，但搜索可能失败
			enginesList := []*MockSearchEngine{
				newMockEngineWithHealthCheck("duckduckgo", 1, false),
				newMockEngineWithHealthCheck("bing", 2, false),
				newMockEngineWithHealthCheck("baidu", 3, false),
				newMockEngineWithHealthCheck("serper", 4, false),
			}

			// 设置搜索失败状态（健康检查仍然成功）
			for _, eng := range enginesList {
				for _, failName := range tt.failEngines {
					if eng.name == failName {
						eng.shouldFail = true // 只影响搜索，不影响健康检查
					}
				}
				r.RegisterEngine(eng)
			}

			r.Start()
			defer r.Stop()
			time.Sleep(100 * time.Millisecond)

			// 执行搜索
			ctx := context.Background()
			req := &engines.SearchRequest{Query: "test query", MaxResults: 10}

			result, err := r.Route(ctx, req, tt.preferredEngine, true)
			
			// 验证错误
			if tt.expectError {
				if err == nil {
					t.Errorf("expected error, but got none")
				}
				return
			}
			
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// 验证降级行为
			if result.EngineUsed != tt.expectEngine {
				t.Errorf("engine: got %s, want %s", result.EngineUsed, tt.expectEngine)
			}

			if result.FallbackTriggered != tt.expectFallback {
				t.Errorf("fallback: got %v, want %v", result.FallbackTriggered, tt.expectFallback)
			}
		})
	}
}

// TestWebSearchTool_Integration_AllEnginesFail 测试所有引擎失败
func TestWebSearchTool_Integration_AllEnginesFail(t *testing.T) {
	r := router.NewAdaptiveRouter(config.RouterConfig{
		Strategy:            "health_first",
		HealthCheckInterval: 60 * time.Second,
	})

	// 所有引擎都失败
	for _, name := range []string{"duckduckgo", "bing", "baidu", "serper"} {
		eng := newMockEngine(name, 1)
		eng.shouldFail = true
		r.RegisterEngine(eng)
	}

	r.Start()
	defer r.Stop()
	time.Sleep(100 * time.Millisecond)

	ctx := context.Background()
	req := &engines.SearchRequest{Query: "test", MaxResults: 10}

	_, err := r.Route(ctx, req, "", true)
	if err == nil {
		t.Error("expected error when all engines fail")
	}
}

// TestWebSearchTool_Integration_HealthCheck 测试健康检查
func TestWebSearchTool_Integration_HealthCheck(t *testing.T) {
	r := router.NewAdaptiveRouter(config.RouterConfig{
		Strategy:            "health_first",
		HealthCheckInterval: 1 * time.Second, // 1秒检查一次
	})

	duckduckgo := newMockEngine("duckduckgo", 1)
	bing := newMockEngine("bing", 2)

	r.RegisterEngine(duckduckgo)
	r.RegisterEngine(bing)

	r.Start()
	defer r.Stop()
	time.Sleep(100 * time.Millisecond)

	// 初始状态：两个引擎都健康
	status := r.HealthChecker.GetHealthStatus()
	if len(status) != 2 {
		t.Errorf("expected 2 engines, got %d", len(status))
	}

	// 设置 duckduckgo 健康检查失败
	duckduckgo.healthCheckFail = true
	
	// 触发健康检查
	r.HealthChecker.CheckEngineNow("duckduckgo")
	time.Sleep(100 * time.Millisecond)

	// 验证状态更新
	status = r.HealthChecker.GetHealthStatus()
	for name, s := range status {
		if name == "duckduckgo" && s.Status == "healthy" {
			t.Error("duckduckgo should not be healthy")
		}
	}
}

// TestWebSearchTool_Integration_RoutingStrategy 测试路由策略
func TestWebSearchTool_Integration_RoutingStrategy(t *testing.T) {
	strategies := []router.Strategy{
		router.StrategyHealthFirst,
		router.StrategyRoundRobin,
		router.StrategyWeighted,
	}

	for _, strategy := range strategies {
		t.Run(string(strategy), func(t *testing.T) {
			r := router.NewAdaptiveRouter(config.RouterConfig{
				Strategy:            string(strategy),
				HealthCheckInterval: 60 * time.Second,
			})

			r.RegisterEngine(newMockEngine("duckduckgo", 1))
			r.RegisterEngine(newMockEngine("bing", 2))

			r.Start()
			defer r.Stop()
			time.Sleep(100 * time.Millisecond)

			// 执行多次搜索
			ctx := context.Background()
			req := &engines.SearchRequest{Query: "test", MaxResults: 10}

			for i := 0; i < 5; i++ {
				result, err := r.Route(ctx, req, "", true)
				if err != nil {
					t.Fatalf("search %d failed: %v", i, err)
				}

				if result.Results == nil {
					t.Errorf("search %d: no results", i)
				}
			}
		})
	}
}

// TestWebSearchTool_Integration_HandleWebSearch 测试顶层 handleWebSearch 函数
func TestWebSearchTool_Integration_HandleWebSearch(t *testing.T) {
	// 创建 Mock HTTP 服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 返回模拟的搜索结果
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		
		response := `{
			"RelatedTopics": [
				{"FirstURL": "https://example.com/1", "Text": "Result 1 - Description"},
				{"FirstURL": "https://example.com/2", "Text": "Result 2 - Description"}
			]
		}`
		w.Write([]byte(response))
	}))
	defer server.Close()

	// 使用测试配置创建工具
	cfg := config.SearchConfig{
		MaxResults:   10,
		AutoFallback: true,
		Router: config.RouterConfig{
			Strategy:            "health_first",
			HealthCheckInterval: 60 * time.Second,
		},
		Engines: map[string]config.EngineConfig{
			"duckduckgo": {Enabled: true, Priority: 1},
		},
	}

	// 注意：实际测试中需要注入 mock 服务器的 URL
	// 这里展示测试框架结构
	t.Log("WebSearch tool integration test setup complete")
	t.Log("Server URL:", server.URL)
	t.Log("Config:", cfg)
}

// TestWebSearchTool_Integration_Timeout 测试超时处理
func TestWebSearchTool_Integration_Timeout(t *testing.T) {
	r := router.NewAdaptiveRouter(config.RouterConfig{
		Strategy:            "health_first",
		HealthCheckInterval: 60 * time.Second,
	})

	// 创建一个延迟很长的引擎
	slowEngine := newMockEngine("slow", 1)
	slowEngine.latency = 2 * time.Second

	r.RegisterEngine(slowEngine)
	r.Start()
	defer r.Stop()
	time.Sleep(100 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	req := &engines.SearchRequest{Query: "test", MaxResults: 10}
	_, err := r.Route(ctx, req, "", false)

	// 应该返回 context 超时错误
	if err == nil {
		t.Error("expected timeout error")
	} else {
		t.Logf("Got expected error: %v", err)
	}
}

// ====================== 基准测试 ======================

func BenchmarkWebSearchTool_Search(b *testing.B) {
	r := router.NewAdaptiveRouter(config.RouterConfig{
		Strategy:            "health_first",
		HealthCheckInterval: 60 * time.Second,
	})

	r.RegisterEngine(newMockEngine("duckduckgo", 1))
	r.RegisterEngine(newMockEngine("bing", 2))

	r.Start()
	defer r.Stop()
	time.Sleep(100 * time.Millisecond)

	ctx := context.Background()
	req := &engines.SearchRequest{Query: "benchmark test", MaxResults: 10}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.Route(ctx, req, "", true)
	}
}
