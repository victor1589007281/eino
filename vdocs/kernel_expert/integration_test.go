// Package main 集成测试
package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"kernel_expert/cache"
	"kernel_expert/interaction"
	"kernel_expert/models"
	"kernel_expert/statistics"
)

// ============================================================
// 缓存系统集成测试
// ============================================================

func TestIntegration_CacheSystem(t *testing.T) {
	// 创建多级缓存
	l1Cache := cache.NewLRUCache(100, time.Hour)
	mlCache := cache.NewMultiLevelCache(l1Cache, nil, nil)
	statsCollector := statistics.NewStatsCollector()

	ctx := context.Background()

	t.Run("CacheSetAndGet", func(t *testing.T) {
		entry := &cache.CacheEntry{
			Key:       "test:key1",
			Value:     "test value",
			Type:      cache.CacheTypeSearch,
			CreatedAt: time.Now(),
			Size:      10,
		}

		err := mlCache.Set(ctx, entry)
		if err != nil {
			t.Fatalf("Cache set failed: %v", err)
		}

		got, err := mlCache.Get(ctx, "test:key1")
		if err != nil {
			t.Fatalf("Cache get failed: %v", err)
		}

		if got == nil {
			t.Fatal("Expected cached entry")
		}

		if got.Value != "test value" {
			t.Errorf("Value mismatch: got %v", got.Value)
		}
	})

	t.Run("CacheWithStats", func(t *testing.T) {
		// 模拟缓存命中和未命中
		for i := 0; i < 10; i++ {
			key := "stats:key" + string(rune('0'+i%5))
			if _, err := mlCache.Get(ctx, key); err != nil {
				statsCollector.Cache().RecordMiss("L1", "search")

				// 设置缓存
				entry := &cache.CacheEntry{Key: key, Value: i, Size: 1}
				mlCache.Set(ctx, entry)
			} else {
				statsCollector.Cache().RecordHit("L1", "search")
			}
		}

		// 验证统计
		stats := statsCollector.Cache().GetStats()
		if stats["total_hits"].(int64)+stats["total_misses"].(int64) != 10 {
			t.Error("Stats count mismatch")
		}
	})

	t.Run("CacheKeyGeneration", func(t *testing.T) {
		keyGen := &cache.CacheKeyGenerator{}

		key1 := keyGen.SearchKey("pattern1", nil)
		key2 := keyGen.SearchKey("pattern1", nil)
		key3 := keyGen.SearchKey("pattern2", nil)

		if key1 != key2 {
			t.Error("Same input should generate same key")
		}
		if key1 == key3 {
			t.Error("Different input should generate different key")
		}
	})

	t.Run("CacheInvalidation", func(t *testing.T) {
		// 设置带标签的缓存
		entry := &cache.CacheEntry{
			Key:   "invalidate:test",
			Value: "will be deleted",
			Tags:  []string{"file:fork.c"},
			Size:  10,
		}
		l1Cache.Set(ctx, entry)

		// 通过标签删除
		l1Cache.DeleteByTag(ctx, "file:fork.c")

		got, _ := l1Cache.Get(ctx, "invalidate:test")
		if got != nil {
			t.Error("Entry should be invalidated")
		}
	})
}

// ============================================================
// 模型路由集成测试
// ============================================================

func TestIntegration_ModelRouting(t *testing.T) {
	registry := models.NewModelRegistry()
	registry.InitDefaultModels()

	t.Run("ComplexityBasedRouting", func(t *testing.T) {
		router := models.NewComplexityRouter(registry)
		ctx := context.Background()

		testCases := []struct {
			complexity models.TaskComplexity
			expected   []string
		}{
			{models.ComplexitySimple, []string{"gpt-4o-mini", "qwen-turbo", "claude-3-haiku"}},
			{models.ComplexityModerate, []string{"gpt-4o", "qwen-plus", "claude-3-sonnet"}},
			{models.ComplexityComplex, []string{"gpt-4-turbo", "qwen-max", "claude-3-sonnet"}},
			{models.ComplexityExpert, []string{"gpt-4-turbo", "claude-3-opus", "qwen-max"}},
		}

		for _, tc := range testCases {
			task := &models.Task{Complexity: tc.complexity}
			model, err := router.Route(ctx, task)
			if err != nil {
				t.Fatalf("Route failed for complexity %d: %v", tc.complexity, err)
			}

			found := false
			for _, expected := range tc.expected {
				if model.Name == expected {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Unexpected model %s for complexity %d", model.Name, tc.complexity)
			}
		}
	})

	t.Run("CostBasedRouting", func(t *testing.T) {
		router := models.NewCostRouter(registry)
		ctx := context.Background()

		task := &models.Task{InputTokens: 1000}
		model, err := router.Route(ctx, task)
		if err != nil {
			t.Fatalf("Route failed: %v", err)
		}

		// 应该选择低成本模型
		if model.InputPrice > 0.01 {
			t.Logf("Selected model %s with input price %f", model.Name, model.InputPrice)
		}
	})

	t.Run("CompositeRouting", func(t *testing.T) {
		complexityRouter := models.NewComplexityRouter(registry)
		costRouter := models.NewCostRouter(registry)
		composite := models.NewCompositeRouter(complexityRouter, costRouter)

		ctx := context.Background()
		task := &models.Task{
			Query:  "详细分析Linux调度器架构",
			Intent: "architecture",
		}

		model, err := composite.Route(ctx, task)
		if err != nil {
			t.Fatalf("Route failed: %v", err)
		}

		if model == nil {
			t.Fatal("Expected model")
		}
	})

	t.Run("ComplexityEvaluation", func(t *testing.T) {
		evaluator := models.NewComplexityEvaluator()

		tests := []struct {
			query    string
			intent   string
			minLevel models.TaskComplexity
		}{
			{"什么是进程", "concept", models.ComplexitySimple},
			{"分析fork函数", "function", models.ComplexityModerate},
			{"深入分析调度器架构", "architecture", models.ComplexityComplex},
		}

		for _, tc := range tests {
			level := evaluator.Evaluate(tc.query, tc.intent)
			if level < tc.minLevel {
				t.Errorf("Query '%s' should be at least complexity %d, got %d",
					tc.query, tc.minLevel, level)
			}
		}
	})
}

// ============================================================
// 统计系统集成测试
// ============================================================

func TestIntegration_StatisticsSystem(t *testing.T) {
	collector := statistics.NewStatsCollector()

	t.Run("TokenTracking", func(t *testing.T) {
		// 模拟多个模型的使用
		collector.Token().Record("gpt-4-turbo", 1000, 500)
		collector.Token().Record("gpt-4-turbo", 2000, 1000)
		collector.Token().Record("qwen-plus", 500, 200)

		// 验证总量
		input, output, total := collector.Token().GetTotalTokens()
		if total != 5200 {
			t.Errorf("Expected 5200 total tokens, got %d", total)
		}
		if input != 3500 {
			t.Errorf("Expected 3500 input tokens, got %d", input)
		}
		if output != 1700 {
			t.Errorf("Expected 1700 output tokens, got %d", output)
		}

		// 验证成本计算
		cost := collector.Token().GetTotalCost()
		if cost <= 0 {
			t.Error("Expected positive cost")
		}
	})

	t.Run("PerformanceTracking", func(t *testing.T) {
		// 记录响应时间
		collector.Performance().RecordResponseTime(100 * time.Millisecond)
		collector.Performance().RecordResponseTime(200 * time.Millisecond)
		collector.Performance().RecordResponseTime(150 * time.Millisecond)

		// 记录工具调用
		collector.Performance().RecordToolCall("grep", 50*time.Millisecond, true)
		collector.Performance().RecordToolCall("grep", 60*time.Millisecond, false)

		stats := collector.Performance().GetLatencyStats()
		if stats["count"] != 3 {
			t.Errorf("Expected 3 response times, got %f", stats["count"])
		}
	})

	t.Run("BusinessMetrics", func(t *testing.T) {
		collector.Business().RecordQuery("function", true, "user1")
		collector.Business().RecordQuery("architecture", true, "user1")
		collector.Business().RecordQuery("concept", false, "user2")

		rate := collector.Business().SuccessRate()
		expected := 2.0 / 3.0
		if rate != expected {
			t.Errorf("Expected success rate %f, got %f", expected, rate)
		}
	})

	t.Run("ReportGeneration", func(t *testing.T) {
		report := collector.GenerateReport()

		if report == nil {
			t.Fatal("Expected report")
		}

		if report.Summary == nil {
			t.Error("Expected summary")
		}

		data, err := report.ExportJSON()
		if err != nil {
			t.Fatalf("Export failed: %v", err)
		}

		if len(data) == 0 {
			t.Error("Expected non-empty JSON")
		}
	})
}

// ============================================================
// Agent交互协议集成测试
// ============================================================

func TestIntegration_AgentInteraction(t *testing.T) {
	t.Run("A2AProtocol", func(t *testing.T) {
		// 创建服务器
		card := &interaction.AgentCard{
			ID:          "kernel-expert",
			Name:        "Linux Kernel Expert",
			Description: "Expert agent for kernel analysis",
			Version:     "1.0.0",
			Capabilities: []interaction.Capability{
				{Name: "analyze_function", Description: "Analyze kernel function"},
				{Name: "search_code", Description: "Search kernel code"},
			},
		}

		server := interaction.NewA2AServer(card)

		// 注册处理器
		server.RegisterHandler("analyze_function", func(ctx context.Context, input map[string]interface{}) (*interaction.A2AResponse, error) {
			funcName := input["function_name"].(string)
			return &interaction.A2AResponse{
				Success: true,
				Output: map[string]interface{}{
					"function": funcName,
					"analysis": "Function analysis result",
				},
			}, nil
		})

		// 创建HTTP服务器
		ts := httptest.NewServer(server)
		defer ts.Close()

		// 创建客户端
		client := interaction.NewA2AClient()
		client.RegisterAgent(&interaction.AgentCard{
			ID:   "kernel-expert",
			Name: "Linux Kernel Expert",
			Endpoints: []interaction.Endpoint{
				{Protocol: "a2a", URL: ts.URL, Priority: 1},
			},
			Capabilities: []interaction.Capability{
				{Name: "analyze_function"},
			},
		})

		// 测试调用
		ctx := context.Background()
		resp, err := client.Call(ctx, "kernel-expert", "analyze_function", map[string]interface{}{
			"function_name": "do_fork",
		})

		if err != nil {
			t.Fatalf("A2A call failed: %v", err)
		}

		if !resp.Success {
			t.Error("Expected successful response")
		}

		if resp.Output["function"] != "do_fork" {
			t.Error("Output mismatch")
		}
	})

	t.Run("MCPProtocol", func(t *testing.T) {
		server := interaction.NewMCPServer("kernel-expert", "1.0.0")

		// 注册工具
		server.RegisterTool(&interaction.MCPTool{
			Name:        "search_code",
			Description: "Search kernel source code",
		}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			query := args["query"].(string)
			return map[string]interface{}{
				"query":   query,
				"results": []string{"kernel/fork.c:100", "kernel/fork.c:200"},
			}, nil
		})

		// 注册资源
		server.RegisterResource(&interaction.MCPResource{
			URI:         "kernel://source/fork.c",
			Name:        "fork.c",
			Description: "Linux kernel fork implementation",
		}, func(ctx context.Context, uri string) ([]byte, string, error) {
			return []byte("/* fork.c source */"), "text/plain", nil
		})

		// 创建HTTP服务器
		ts := httptest.NewServer(server)
		defer ts.Close()

		// 测试工具列表
		resp, err := http.Post(ts.URL, "application/json", strings.NewReader(`{
			"jsonrpc": "2.0",
			"id": 1,
			"method": "tools/list"
		}`))
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected 200, got %d", resp.StatusCode)
		}

		// 测试调用工具
		ctx := context.Background()
		result, err := server.CallTool(ctx, "search_code", map[string]interface{}{
			"query": "do_fork",
		})

		if err != nil {
			t.Fatalf("CallTool failed: %v", err)
		}

		resultMap := result.(map[string]interface{})
		if resultMap["query"] != "do_fork" {
			t.Error("Tool result mismatch")
		}
	})

	t.Run("AgentDiscovery", func(t *testing.T) {
		client := interaction.NewA2AClient()

		// 注册多个agent
		client.RegisterAgent(&interaction.AgentCard{
			ID:           "agent1",
			Capabilities: []interaction.Capability{{Name: "search"}},
		})
		client.RegisterAgent(&interaction.AgentCard{
			ID:           "agent2",
			Capabilities: []interaction.Capability{{Name: "analyze"}, {Name: "search"}},
		})
		client.RegisterAgent(&interaction.AgentCard{
			ID:           "agent3",
			Capabilities: []interaction.Capability{{Name: "generate"}},
		})

		// 发现具有search能力的agent
		agents := client.DiscoverAgents("search")
		if len(agents) != 2 {
			t.Errorf("Expected 2 agents with search capability, got %d", len(agents))
		}
	})
}

// ============================================================
// 完整流程集成测试
// ============================================================

func TestIntegration_FullPipeline(t *testing.T) {
	ctx := context.Background()

	// 初始化所有组件
	statsCollector := statistics.NewStatsCollector()
	l1Cache := cache.NewLRUCache(100, time.Hour)
	mlCache := cache.NewMultiLevelCache(l1Cache, nil, nil)
	registry := models.NewModelRegistry()
	registry.InitDefaultModels()
	router := models.NewComplexityRouter(registry)

	// 模拟完整的请求处理流程
	query := "解释Linux进程调度器的工作原理"
	intent := "architecture"

	startTime := time.Now()

	// 1. 检查缓存
	keyGen := &cache.CacheKeyGenerator{}
	cacheKey := keyGen.SearchKey(query, nil)

	cachedResult, _ := mlCache.Get(ctx, cacheKey)
	if cachedResult != nil {
		statsCollector.Cache().RecordHit("L1", "search")
		t.Log("Cache hit")
	} else {
		statsCollector.Cache().RecordMiss("L1", "search")

		// 2. 路由选择模型
		task := &models.Task{
			Query:  query,
			Intent: intent,
		}
		model, err := router.Route(ctx, task)
		if err != nil {
			t.Fatalf("Route failed: %v", err)
		}
		t.Logf("Selected model: %s", model.Name)

		// 3. 模拟模型调用
		inputTokens := 800
		outputTokens := 1500
		statsCollector.Token().Record(model.Name, inputTokens, outputTokens)

		// 4. 缓存结果
		result := &cache.CacheEntry{
			Key:   cacheKey,
			Value: "Analysis result: Linux scheduler...",
			Type:  cache.CacheTypeSearch,
			Size:  100,
			Tags:  []string{"query", "scheduler"},
		}
		mlCache.Set(ctx, result)
	}

	// 5. 记录性能
	duration := time.Since(startTime)
	statsCollector.Performance().RecordResponseTime(duration)

	// 6. 记录业务统计
	statsCollector.Business().RecordQuery(intent, true, "test-user")

	// 7. 生成报告
	report := statsCollector.GenerateReport()

	// 验证结果
	if report.Summary.TotalQueries < 1 {
		t.Error("Expected at least 1 query")
	}

	t.Logf("Pipeline completed in %v", duration)
	t.Logf("Total cost: $%.4f", statsCollector.Token().GetTotalCost())
}

// ============================================================
// 索引系统集成测试
// ============================================================

func TestIntegration_IndexSystem(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "index_integration")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// 创建测试源码
	sourceDir := filepath.Join(tmpDir, "source")
	os.MkdirAll(sourceDir, 0755)

	// 创建测试C文件
	testFiles := map[string]string{
		"fork.c": `
#include <linux/sched.h>

long do_fork(unsigned long clone_flags) {
    struct task_struct *p;
    p = copy_process(clone_flags);
    wake_up_new_task(p);
    return p->pid;
}

struct task_struct *copy_process(unsigned long flags) {
    // Implementation
    return NULL;
}
`,
		"sched.c": `
#include <linux/sched.h>

void schedule(void) {
    struct task_struct *next;
    next = pick_next_task();
    context_switch(next);
}

void wake_up_new_task(struct task_struct *p) {
    enqueue_task(p);
}
`,
	}

	for name, content := range testFiles {
		os.WriteFile(filepath.Join(sourceDir, name), []byte(content), 0644)
	}

	t.Run("IndexCreation", func(t *testing.T) {
		// 验证文件已创建
		files, _ := os.ReadDir(sourceDir)
		if len(files) != 2 {
			t.Errorf("Expected 2 files, got %d", len(files))
		}
	})
}

// ============================================================
// 性能基准测试
// ============================================================

func BenchmarkIntegration_CacheOperations(b *testing.B) {
	l1Cache := cache.NewLRUCache(1000, time.Hour)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := "bench:key" + string(rune(i%1000))

		if _, err := l1Cache.Get(ctx, key); err != nil {
			l1Cache.Set(ctx, &cache.CacheEntry{
				Key:   key,
				Value: i,
				Size:  1,
			})
		}
	}
}

func BenchmarkIntegration_ModelRouting(b *testing.B) {
	registry := models.NewModelRegistry()
	registry.InitDefaultModels()
	router := models.NewComplexityRouter(registry)
	ctx := context.Background()

	task := &models.Task{Complexity: models.ComplexityModerate}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		router.Route(ctx, task)
	}
}

func BenchmarkIntegration_StatisticsRecording(b *testing.B) {
	collector := statistics.NewStatsCollector()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		collector.Token().Record("gpt-4-turbo", 1000, 500)
		collector.Cache().RecordHit("L1", "search")
		collector.Performance().RecordResponseTime(100 * time.Millisecond)
		collector.Business().RecordQuery("function", true, "user")
	}
}

func BenchmarkIntegration_A2ARequest(b *testing.B) {
	card := &interaction.AgentCard{ID: "test-agent"}
	server := interaction.NewA2AServer(card)
	server.RegisterHandler("test", func(ctx context.Context, input map[string]interface{}) (*interaction.A2AResponse, error) {
		return &interaction.A2AResponse{Success: true}, nil
	})

	ctx := context.Background()
	msg := &interaction.A2AMessage{
		ID:   "msg-001",
		Type: interaction.A2AMessageRequest,
		From: "client",
		Payload: &interaction.A2ARequest{
			Capability: "test",
			Input:      map[string]interface{}{},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		server.HandleRequest(ctx, msg)
	}
}

// ============================================================
// 端到端测试
// ============================================================

func TestE2E_QueryToResponse(t *testing.T) {
	// 这是一个端到端测试，模拟从用户查询到最终响应的完整流程
	ctx := context.Background()

	// 初始化系统
	statsCollector := statistics.NewStatsCollector()
	l1Cache := cache.NewLRUCache(100, time.Hour)
	registry := models.NewModelRegistry()
	registry.InitDefaultModels()

	// 模拟用户查询
	queries := []struct {
		query  string
		intent string
	}{
		{"什么是Linux进程", "concept"},
		{"fork函数如何实现", "function"},
		{"调度器的架构设计", "architecture"},
	}

	for _, q := range queries {
		t.Run(q.query, func(t *testing.T) {
			// 检查缓存
			keyGen := &cache.CacheKeyGenerator{}
			cacheKey := keyGen.SearchKey(q.query, nil)

			_, _ = l1Cache.Get(ctx, cacheKey)

			// 选择模型
			evaluator := models.NewComplexityEvaluator()
			complexity := evaluator.Evaluate(q.query, q.intent)

			router := models.NewComplexityRouter(registry)
			model, err := router.Route(ctx, &models.Task{Complexity: complexity})
			if err != nil {
				t.Fatalf("Route failed: %v", err)
			}

			// 记录统计
			statsCollector.Token().Record(model.Name, 500, 800)
			statsCollector.Business().RecordQuery(q.intent, true, "test")

			t.Logf("Query: %s -> Model: %s (Complexity: %d)", q.query, model.Name, complexity)
		})
	}

	// 验证总体统计
	report := statsCollector.GenerateReport()
	if report.Summary.TotalQueries != int64(len(queries)) {
		t.Errorf("Expected %d queries, got %d", len(queries), report.Summary.TotalQueries)
	}

	// 打印报告
	data, _ := json.MarshalIndent(report.Summary, "", "  ")
	t.Logf("Summary:\n%s", string(data))
}
