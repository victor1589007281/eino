package router

import (
	"context"
	"testing"
	"time"

	"github.com/cloudwego/eino/vdocstool/config"
	"github.com/cloudwego/eino/vdocstool/tools/search/engines"
)

// MockEngine 模拟搜索引擎
type MockEngine struct {
	name        string
	priority    int
	shouldFail  bool
	failCount   int
	searchDelay time.Duration
}

func NewMockEngine(name string, priority int) *MockEngine {
	return &MockEngine{
		name:     name,
		priority: priority,
	}
}

func (e *MockEngine) Name() string {
	return e.name
}

func (e *MockEngine) Priority() int {
	return e.priority
}

func (e *MockEngine) Search(ctx context.Context, req *engines.SearchRequest) ([]*engines.SearchResult, error) {
	if e.searchDelay > 0 {
		time.Sleep(e.searchDelay)
	}

	if e.shouldFail {
		e.failCount++
		return nil, context.DeadlineExceeded
	}

	return []*engines.SearchResult{
		{
			Title:   "Test Result from " + e.name,
			URL:     "https://example.com",
			Snippet: "Test snippet",
		},
	}, nil
}

func (e *MockEngine) HealthCheck(ctx context.Context) error {
	if e.shouldFail {
		return context.DeadlineExceeded
	}
	return nil
}

func TestAdaptiveRouter_SelectEngine(t *testing.T) {
	cfg := config.RouterConfig{
		Strategy:                "health_first",
		CircuitBreakerThreshold: 3,
		CircuitBreakerTimeout:   60 * time.Second,
		HealthCheckInterval:     30 * time.Second,
	}

	router := NewAdaptiveRouter(cfg)

	// 注册引擎
	engine1 := NewMockEngine("engine1", 1)
	engine2 := NewMockEngine("engine2", 2)
	router.RegisterEngine(engine1)
	router.RegisterEngine(engine2)

	ctx := context.Background()

	// 测试健康优先策略
	selected, err := router.SelectEngine(ctx, "")
	if err != nil {
		t.Fatalf("SelectEngine failed: %v", err)
	}

	if selected.Name() != "engine1" {
		t.Errorf("Expected engine1 (priority 1), got %s", selected.Name())
	}

	// 测试指定首选引擎
	selected, err = router.SelectEngine(ctx, "engine2")
	if err != nil {
		t.Fatalf("SelectEngine failed: %v", err)
	}

	if selected.Name() != "engine2" {
		t.Errorf("Expected engine2 (preferred), got %s", selected.Name())
	}
}

func TestAdaptiveRouter_Fallback(t *testing.T) {
	cfg := config.RouterConfig{
		Strategy:                "health_first",
		CircuitBreakerThreshold: 3,
		CircuitBreakerTimeout:   60 * time.Second,
		HealthCheckInterval:     30 * time.Second,
	}

	router := NewAdaptiveRouter(cfg)

	// 注册引擎，第一个会失败
	engine1 := NewMockEngine("engine1", 1)
	engine1.shouldFail = true
	engine2 := NewMockEngine("engine2", 2)
	router.RegisterEngine(engine1)
	router.RegisterEngine(engine2)

	ctx := context.Background()

	// 执行搜索，应该降级到 engine2
	result, err := router.Route(ctx, &engines.SearchRequest{
		Query:      "test",
		MaxResults: 5,
	}, "engine1", true)

	if err != nil {
		t.Fatalf("Route failed: %v", err)
	}

	if !result.FallbackTriggered {
		t.Error("Expected fallback to be triggered")
	}

	if result.EngineUsed != "engine2" {
		t.Errorf("Expected engine2 after fallback, got %s", result.EngineUsed)
	}
}

func TestAdaptiveRouter_CircuitBreaker(t *testing.T) {
	cfg := config.RouterConfig{
		Strategy:                "health_first",
		CircuitBreakerThreshold: 2, // 连续2次失败就熔断
		CircuitBreakerTimeout:   1 * time.Second,
		HealthCheckInterval:     30 * time.Second,
	}

	router := NewAdaptiveRouter(cfg)

	engine1 := NewMockEngine("engine1", 1)
	engine1.shouldFail = true
	router.RegisterEngine(engine1)

	// 模拟连续失败
	for i := 0; i < 3; i++ {
		router.recordFailure("engine1")
	}

	// 检查引擎状态
	state := router.GetEngineState("engine1")
	if state.Status != engines.StatusCircuitOpen {
		t.Errorf("Expected circuit open, got %v", state.Status)
	}
}

func TestHealthChecker(t *testing.T) {
	cfg := config.RouterConfig{
		Strategy:                "health_first",
		CircuitBreakerThreshold: 3,
		CircuitBreakerTimeout:   60 * time.Second,
		HealthCheckInterval:     100 * time.Millisecond,
	}

	router := NewAdaptiveRouter(cfg)

	engine := NewMockEngine("test_engine", 1)
	router.RegisterEngine(engine)

	// 启动路由器（包括健康检查）
	router.Start()
	defer router.Stop()

	// 等待健康检查完成
	time.Sleep(200 * time.Millisecond)

	// 验证引擎状态
	state := router.GetEngineState("test_engine")
	if state == nil {
		t.Fatal("Engine state not found")
	}

	if state.Status != engines.StatusHealthy {
		t.Errorf("Expected healthy status, got %v", state.Status)
	}
}

func TestFallbackStrategy(t *testing.T) {
	cfg := config.RouterConfig{
		Strategy:                "health_first",
		CircuitBreakerThreshold: 3,
		CircuitBreakerTimeout:   60 * time.Second,
	}

	router := NewAdaptiveRouter(cfg)

	// 注册多个引擎
	engine1 := NewMockEngine("engine1", 1)
	engine1.shouldFail = true
	engine2 := NewMockEngine("engine2", 2)
	engine2.shouldFail = true
	engine3 := NewMockEngine("engine3", 3)

	router.RegisterEngine(engine1)
	router.RegisterEngine(engine2)
	router.RegisterEngine(engine3)

	ctx := context.Background()
	strategy := NewFallbackStrategy(router)

	result, err := strategy.ExecuteWithFallback(ctx, &engines.SearchRequest{
		Query:      "test",
		MaxResults: 5,
	}, "engine1")

	if err != nil {
		t.Fatalf("ExecuteWithFallback failed: %v", err)
	}

	if result.FinalEngine != "engine3" {
		t.Errorf("Expected engine3 as final engine, got %s", result.FinalEngine)
	}

	if !result.FallbackUsed {
		t.Error("Expected fallback to be used")
	}

	if len(result.Attempts) != 3 {
		t.Errorf("Expected 3 attempts, got %d", len(result.Attempts))
	}
}
