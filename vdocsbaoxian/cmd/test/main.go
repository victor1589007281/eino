// Package main provides integration tests for Insurance Expert Agent.
package main

import (
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

type TestResult struct {
	Name    string
	Passed  bool
	Message string
	Time    time.Duration
}

func main() {
	fmt.Println("========================================")
	fmt.Println("  保险专家Agent 集成测试")
	fmt.Println("========================================")
	fmt.Println()

	apiKey := os.Getenv("DEEPSEEK_API_KEY")
	if apiKey == "" {
		fmt.Println("❌ 错误: 请设置 DEEPSEEK_API_KEY 环境变量")
		os.Exit(1)
	}

	results := []TestResult{}
	ctx := context.Background()

	// 测试1: 配置模块
	results = append(results, testConfig())

	// 测试2: 缓存模块
	results = append(results, testCache(ctx))

	// 测试3: 路由模块
	results = append(results, testRouter(ctx))

	// 测试4: 统计模块
	results = append(results, testStats())

	// 测试5: 输出格式化
	results = append(results, testOutput())

	// 测试6: DeepSeek API
	results = append(results, testDeepSeekAPI(ctx, apiKey))

	// 测试7: 保险问答
	results = append(results, testInsuranceQA(ctx, apiKey))

	// 打印结果
	fmt.Println()
	fmt.Println("========================================")
	fmt.Println("  测试结果汇总")
	fmt.Println("========================================")
	
	passed := 0
	failed := 0
	for _, r := range results {
		status := "✅ PASS"
		if !r.Passed {
			status = "❌ FAIL"
			failed++
		} else {
			passed++
		}
		fmt.Printf("%s  %-25s  %v  %s\n", status, r.Name, r.Time.Round(time.Millisecond), r.Message)
	}

	fmt.Println()
	fmt.Printf("总计: %d 测试, %d 通过, %d 失败\n", len(results), passed, failed)

	if failed > 0 {
		os.Exit(1)
	}
}

func testConfig() TestResult {
	start := time.Now()
	cfg := config.DefaultConfig()
	
	if cfg.LLM.DefaultProvider == "" {
		return TestResult{"Config", false, "DefaultProvider为空", time.Since(start)}
	}
	if cfg.Cache.L1.MaxSize <= 0 {
		return TestResult{"Config", false, "缓存配置无效", time.Since(start)}
	}
	
	return TestResult{"Config", true, "配置加载正常", time.Since(start)}
}

func testCache(ctx context.Context) TestResult {
	start := time.Now()
	
	l1 := cache.NewLRUCache(100, time.Minute)
	l2 := cache.NewLRUCache(1000, time.Hour)
	multi := cache.NewMultiLevelCache([]cache.Cache{l1, l2}, nil)

	// 写入测试
	err := multi.Set(ctx, "key1", "value1", 0)
	if err != nil {
		return TestResult{"Cache", false, fmt.Sprintf("写入失败: %v", err), time.Since(start)}
	}

	// 读取测试
	val, ok := multi.Get(ctx, "key1")
	if !ok {
		return TestResult{"Cache", false, "读取失败", time.Since(start)}
	}
	if val != "value1" {
		return TestResult{"Cache", false, "值不匹配", time.Since(start)}
	}

	return TestResult{"Cache", true, "缓存功能正常", time.Since(start)}
}

func testRouter(ctx context.Context) TestResult {
	start := time.Now()
	
	cfg := router.DefaultRouterConfig()
	r := router.NewRouter(cfg)

	model, err := r.SelectModel(ctx, router.TaskTypeSimple)
	if err != nil {
		return TestResult{"Router", false, fmt.Sprintf("路由失败: %v", err), time.Since(start)}
	}
	if model.Model == "" {
		return TestResult{"Router", false, "模型为空", time.Since(start)}
	}

	return TestResult{"Router", true, fmt.Sprintf("选择模型: %s", model.Model), time.Since(start)}
}

func testStats() TestResult {
	start := time.Now()
	
	s := stats.NewStats()
	s.RecordTokenUsage("test", 100, 50)
	s.RecordCacheHit(1, "test")
	s.RecordQuery(true, time.Millisecond)

	summary := s.GetSummary()
	if summary.TokenStats.TotalTokens != 150 {
		return TestResult{"Stats", false, "Token统计错误", time.Since(start)}
	}

	return TestResult{"Stats", true, "统计功能正常", time.Since(start)}
}

func testOutput() TestResult {
	start := time.Now()
	
	result := &output.AnalysisResult{
		Query:      "测试问题",
		Summary:    "测试摘要",
		Confidence: 0.95,
		TokensUsed: 100,
		Timestamp:  time.Now(),
	}

	formatter := output.NewSummaryFormatter(1000)
	out, err := formatter.Format(result)
	if err != nil {
		return TestResult{"Output", false, fmt.Sprintf("格式化失败: %v", err), time.Since(start)}
	}
	if len(out) == 0 {
		return TestResult{"Output", false, "输出为空", time.Since(start)}
	}

	return TestResult{"Output", true, fmt.Sprintf("生成%d字符", len(out)), time.Since(start)}
}

func testDeepSeekAPI(ctx context.Context, apiKey string) TestResult {
	start := time.Now()

	resp, err := callDeepSeek(ctx, apiKey, "你好")
	if err != nil {
		return TestResult{"DeepSeek API", false, fmt.Sprintf("调用失败: %v", err), time.Since(start)}
	}
	if len(resp) == 0 {
		return TestResult{"DeepSeek API", false, "响应为空", time.Since(start)}
	}

	return TestResult{"DeepSeek API", true, fmt.Sprintf("响应%d字符", len(resp)), time.Since(start)}
}

func testInsuranceQA(ctx context.Context, apiKey string) TestResult {
	start := time.Now()

	resp, err := callDeepSeek(ctx, apiKey, "什么是保险等待期？用一句话回答。")
	if err != nil {
		return TestResult{"Insurance QA", false, fmt.Sprintf("调用失败: %v", err), time.Since(start)}
	}
	
	// 检查回答是否包含相关关键词
	keywords := []string{"等待期", "观察期", "保险"}
	found := false
	for _, kw := range keywords {
		if strings.Contains(resp, kw) {
			found = true
			break
		}
	}
	
	if !found {
		return TestResult{"Insurance QA", false, "回答不相关", time.Since(start)}
	}

	return TestResult{"Insurance QA", true, "问答正常", time.Since(start)}
}

type DeepSeekRequest struct {
	Model    string        `json:"model"`
	Messages []DeepSeekMsg `json:"messages"`
	Stream   bool          `json:"stream"`
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
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func callDeepSeek(ctx context.Context, apiKey, prompt string) (string, error) {
	reqBody := DeepSeekRequest{
		Model: "deepseek-chat",
		Messages: []DeepSeekMsg{
			{Role: "user", Content: prompt},
		},
		Stream: false,
	}

	jsonData, _ := json.Marshal(reqBody)

	req, _ := http.NewRequestWithContext(ctx, "POST", "https://api.deepseek.com/chat/completions", strings.NewReader(string(jsonData)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result DeepSeekResponse
	json.Unmarshal(body, &result)

	if result.Error != nil {
		return "", fmt.Errorf(result.Error.Message)
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("无响应")
	}

	return result.Choices[0].Message.Content, nil
}
