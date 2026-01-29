// Package main provides a demo for Insurance Expert Agent.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/cloudwego/eino/vdocsbaoxian/cache"
	"github.com/cloudwego/eino/vdocsbaoxian/config"
	"github.com/cloudwego/eino/vdocsbaoxian/output"
	"github.com/cloudwego/eino/vdocsbaoxian/router"
	"github.com/cloudwego/eino/vdocsbaoxian/stats"
)

func main() {
	fmt.Println("========================================")
	fmt.Println("  保险专家Agent 集成测试演示")
	fmt.Println("  Insurance Expert Agent Demo")
	fmt.Println("========================================")
	fmt.Println()

	// 检查API密钥
	apiKey := os.Getenv("DEEPSEEK_API_KEY")
	if apiKey == "" {
		fmt.Println("❌ 错误: 请设置 DEEPSEEK_API_KEY 环境变量")
		fmt.Println("   示例: export DEEPSEEK_API_KEY=sk-xxx")
		os.Exit(1)
	}
	fmt.Println("✅ DeepSeek API Key 已配置")

	// 初始化配置
	cfg := config.DefaultConfig()
	fmt.Println("✅ 配置加载成功")

	// 初始化统计模块
	statsCollector := stats.NewStats()
	statsCollector.SetDailyBudget(100000) // 设置每日token预算
	fmt.Println("✅ 统计模块初始化成功")

	// 初始化缓存
	l1Cache := cache.NewLRUCache(1000, 5*time.Minute)
	l2Cache := cache.NewLRUCache(10000, 30*time.Minute)
	multiCache := cache.NewMultiLevelCache(
		[]cache.Cache{l1Cache, l2Cache},
		func(level int, hit bool) {
			if hit {
				statsCollector.RecordCacheHit(level, "query")
			} else {
				statsCollector.RecordCacheMiss(level, "query")
			}
		},
	)
	fmt.Println("✅ 多级缓存初始化成功")

	// 初始化路由
	routerCfg := router.DefaultRouterConfig()
	// 配置DeepSeek
	routerCfg.Models["deepseek-chat"] = router.ModelConfig{
		Provider:     router.ProviderDeepSeek,
		Model:        "deepseek-chat",
		MaxTokens:    4096,
		Temperature:  0.7,
		Timeout:      60 * time.Second,
		RateLimit:    60,
		CostPerToken: 0.001,
		Priority:     1,
		TaskTypes:    []router.TaskType{router.TaskTypeSimple, router.TaskTypeReasoning, router.TaskTypeAnalysis},
		Enabled:      true,
	}
	modelRouter := router.NewRouter(routerCfg)
	modelRouter.SetStatsFunc(func(provider string, success bool, tokens int64, duration time.Duration) {
		statsCollector.RecordTokenUsage(provider, tokens, tokens/2)
	})
	fmt.Println("✅ 模型路由初始化成功")

	// 初始化输出格式化器
	summaryFormatter := output.NewSummaryFormatter(2000)
	docFormatter := output.NewDocumentFormatter(nil)
	fmt.Println("✅ 输出格式化器初始化成功")
	fmt.Println()

	// 运行集成测试
	fmt.Println("========================================")
	fmt.Println("  开始集成测试")
	fmt.Println("========================================")
	fmt.Println()

	ctx := context.Background()

	// 测试1: 缓存功能
	fmt.Println("📋 测试1: 缓存功能")
	testCache(ctx, multiCache)
	fmt.Println()

	// 测试2: 模型路由
	fmt.Println("📋 测试2: 模型路由")
	testRouter(ctx, modelRouter)
	fmt.Println()

	// 测试3: 输出格式化
	fmt.Println("📋 测试3: 输出格式化")
	testOutput(summaryFormatter, docFormatter)
	fmt.Println()

	// 测试4: DeepSeek API调用
	fmt.Println("📋 测试4: DeepSeek API调用")
	testDeepSeekAPI(ctx, apiKey, statsCollector)
	fmt.Println()

	// 测试5: 保险问答演示
	fmt.Println("📋 测试5: 保险问答演示")
	testInsuranceQA(ctx, apiKey, statsCollector)
	fmt.Println()

	// 打印统计信息
	fmt.Println("========================================")
	fmt.Println("  统计信息")
	fmt.Println("========================================")
	printStats(statsCollector)
	fmt.Println()

	// 交互模式
	fmt.Println("========================================")
	fmt.Println("  进入交互模式 (输入 'exit' 退出)")
	fmt.Println("========================================")
	interactiveMode(ctx, apiKey, statsCollector, multiCache, cfg)
}

func testCache(ctx context.Context, cache *cache.MultiLevelCache) {
	// 写入测试
	err := cache.Set(ctx, "test_key", "test_value", 0)
	if err != nil {
		fmt.Printf("   ❌ 缓存写入失败: %v\n", err)
		return
	}
	fmt.Println("   ✅ 缓存写入成功")

	// 读取测试
	value, ok := cache.Get(ctx, "test_key")
	if !ok {
		fmt.Println("   ❌ 缓存读取失败")
		return
	}
	fmt.Printf("   ✅ 缓存读取成功: %v\n", value)

	// 再次读取（应该命中L1缓存）
	_, ok = cache.Get(ctx, "test_key")
	if ok {
		fmt.Println("   ✅ L1缓存命中")
	}
}

func testRouter(ctx context.Context, r *router.Router) {
	// 简单任务路由
	model, err := r.SelectModel(ctx, router.TaskTypeSimple)
	if err != nil {
		fmt.Printf("   ❌ 简单任务路由失败: %v\n", err)
	} else {
		fmt.Printf("   ✅ 简单任务路由: %s\n", model.Model)
	}

	// 推理任务路由
	model, err = r.SelectModel(ctx, router.TaskTypeReasoning)
	if err != nil {
		fmt.Printf("   ❌ 推理任务路由失败: %v\n", err)
	} else {
		fmt.Printf("   ✅ 推理任务路由: %s\n", model.Model)
	}
}

func testOutput(summary *output.SummaryFormatter, doc *output.DocumentFormatter) {
	result := &output.AnalysisResult{
		Query:      "什么是保险等待期？",
		Summary:    "保险等待期是指保险合同生效后的一段时间内，保险公司不承担保险责任的期间。",
		Details:    "详细说明...",
		Confidence: 0.95,
		TokensUsed: 100,
		Timestamp:  time.Now(),
	}

	// 测试摘要格式
	summaryOutput, err := summary.Format(result)
	if err != nil {
		fmt.Printf("   ❌ 摘要格式化失败: %v\n", err)
	} else {
		fmt.Printf("   ✅ 摘要格式化成功 (%d 字符)\n", len(summaryOutput))
	}

	// 测试文档格式
	docOutput, err := doc.Format(result)
	if err != nil {
		fmt.Printf("   ❌ 文档格式化失败: %v\n", err)
	} else {
		fmt.Printf("   ✅ 文档格式化成功 (%d 字符)\n", len(docOutput))
	}
}

func testDeepSeekAPI(ctx context.Context, apiKey string, statsCollector *stats.Stats) {
	start := time.Now()

	resp, err := callDeepSeek(ctx, apiKey, "你好，请用一句话介绍你自己。")
	if err != nil {
		fmt.Printf("   ❌ API调用失败: %v\n", err)
		return
	}

	duration := time.Since(start)
	fmt.Printf("   ✅ API调用成功 (耗时: %v)\n", duration)
	fmt.Printf("   📝 回复: %s\n", truncate(resp, 100))

	statsCollector.RecordQuery(true, duration)
}

func testInsuranceQA(ctx context.Context, apiKey string, statsCollector *stats.Stats) {
	questions := []string{
		"什么是保险等待期？简要回答。",
		"保险法第16条的主要内容是什么？简要回答。",
	}

	for i, q := range questions {
		fmt.Printf("   问题%d: %s\n", i+1, q)
		start := time.Now()

		resp, err := callDeepSeek(ctx, apiKey, q)
		if err != nil {
			fmt.Printf("   ❌ 回答失败: %v\n", err)
			continue
		}

		duration := time.Since(start)
		fmt.Printf("   ✅ 回答成功 (耗时: %v)\n", duration)
		fmt.Printf("   📝 回答: %s\n\n", truncate(resp, 200))

		statsCollector.RecordQuery(true, duration)
	}
}

func printStats(s *stats.Stats) {
	summary := s.GetSummary()

	fmt.Printf("   运行时间: %v\n", summary.Uptime)
	fmt.Printf("   Token统计:\n")
	fmt.Printf("     - 总输入: %d\n", summary.TokenStats.TotalInputTokens)
	fmt.Printf("     - 总输出: %d\n", summary.TokenStats.TotalOutputTokens)
	fmt.Printf("   缓存统计:\n")
	fmt.Printf("     - 命中率: %.2f%%\n", summary.CacheHitRate*100)
	fmt.Printf("     - L1命中: %d\n", summary.CacheStats.L1Hits)
	fmt.Printf("     - L1未命中: %d\n", summary.CacheStats.L1Misses)
	fmt.Printf("   查询统计:\n")
	fmt.Printf("     - 总查询: %d\n", summary.AgentStats.TotalQueries)
	fmt.Printf("     - 成功: %d\n", summary.AgentStats.SuccessfulQueries)
}

func interactiveMode(ctx context.Context, apiKey string, statsCollector *stats.Stats, cache *cache.MultiLevelCache, cfg *config.Config) {
	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Print("\n🤖 请输入保险相关问题: ")
		input, err := reader.ReadString('\n')
		if err != nil {
			break
		}

		input = strings.TrimSpace(input)
		if input == "exit" || input == "quit" || input == "退出" {
			fmt.Println("👋 再见！")
			break
		}

		if input == "stats" || input == "统计" {
			printStats(statsCollector)
			continue
		}

		if input == "" {
			continue
		}

		// 检查缓存
		cacheKey := "qa:" + input
		if cached, ok := cache.Get(ctx, cacheKey); ok {
			fmt.Println("📦 [缓存命中]")
			fmt.Printf("📝 回答: %s\n", cached)
			continue
		}

		// 调用API
		fmt.Println("⏳ 正在思考...")
		start := time.Now()

		// 构建保险专家提示词
		prompt := fmt.Sprintf(`你是一位专业的保险专家，请根据以下问题给出专业、准确的回答。
请确保回答：
1. 基于保险法律法规
2. 参考保险产品条款
3. 结合实际理赔案例

问题: %s

请给出详细但简洁的回答：`, input)

		resp, err := callDeepSeek(ctx, apiKey, prompt)
		if err != nil {
			fmt.Printf("❌ 回答失败: %v\n", err)
			continue
		}

		duration := time.Since(start)
		fmt.Printf("⏱️  耗时: %v\n", duration)
		fmt.Printf("📝 回答:\n%s\n", resp)

		// 存入缓存
		cache.Set(ctx, cacheKey, resp, 10*time.Minute)

		statsCollector.RecordQuery(true, duration)
	}
}

// DeepSeek API调用
type DeepSeekRequest struct {
	Model    string          `json:"model"`
	Messages []DeepSeekMsg   `json:"messages"`
	Stream   bool            `json:"stream"`
}

type DeepSeekMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type DeepSeekResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func callDeepSeek(ctx context.Context, apiKey, prompt string) (string, error) {
	reqBody := DeepSeekRequest{
		Model: "deepseek-chat",
		Messages: []DeepSeekMsg{
			{Role: "system", Content: "你是一位专业的保险专家，精通保险法律法规、产品条款和理赔实务。"},
			{Role: "user", Content: prompt},
		},
		Stream: false,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.deepseek.com/chat/completions", strings.NewReader(string(jsonData)))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var result DeepSeekResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("解析响应失败: %v, body: %s", err, string(body))
	}

	if result.Error != nil {
		return "", fmt.Errorf("API错误: %s", result.Error.Message)
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("无响应内容")
	}

	return result.Choices[0].Message.Content, nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
