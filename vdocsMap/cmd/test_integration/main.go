/*
 * Copyright 2025 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"go.uber.org/zap"

	"github.com/cloudwego/eino/vdocsMap/agent"
	"github.com/cloudwego/eino/vdocsMap/config"
	"github.com/cloudwego/eino/vdocsMap/llm"
)

var (
	configPath = flag.String("config", "config/config-dev.yaml", "配置文件路径")
	testCase   = flag.String("test", "all", "测试用例：all, image, gif, sticker, prompt, style")
)

func main() {
	flag.Parse()

	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	logger.Info("开始集成测试", zap.String("test_case", *testCase))

	// 加载配置
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		logger.Fatal("加载配置失败", zap.Error(err))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// 创建 LLM 模型
	chatModel, err := llm.NewChatModel(ctx, &cfg.LLM)
	if err != nil {
		logger.Fatal("创建 LLM 模型失败", zap.Error(err))
	}

	// 创建 Agent
	imageAgent, err := agent.NewImageGenAgent(ctx, cfg, chatModel)
	if err != nil {
		logger.Fatal("创建 Agent 失败", zap.Error(err))
	}
	defer imageAgent.Close()

	results := make(map[string]*TestResult)

	switch *testCase {
	case "all":
		results["image"] = testImageGeneration(ctx, imageAgent, logger)
		results["gif"] = testGIFGeneration(ctx, imageAgent, logger)
		results["sticker"] = testStickerGeneration(ctx, imageAgent, logger)
		results["prompt"] = testPromptOptimization(ctx, imageAgent, logger)
		results["style"] = testStyleAnalysis(ctx, imageAgent, logger)
	case "image":
		results["image"] = testImageGeneration(ctx, imageAgent, logger)
	case "gif":
		results["gif"] = testGIFGeneration(ctx, imageAgent, logger)
	case "sticker":
		results["sticker"] = testStickerGeneration(ctx, imageAgent, logger)
	case "prompt":
		results["prompt"] = testPromptOptimization(ctx, imageAgent, logger)
	case "style":
		results["style"] = testStyleAnalysis(ctx, imageAgent, logger)
	default:
		logger.Fatal("未知的测试用例", zap.String("test", *testCase))
	}

	// 输出测试结果
	printResults(results, logger)
}

// TestResult 测试结果
type TestResult struct {
	Name      string        `json:"name"`
	Success   bool          `json:"success"`
	Duration  time.Duration `json:"duration"`
	Error     string        `json:"error,omitempty"`
	Output    string        `json:"output,omitempty"`
}

// testImageGeneration 测试图片生成
func testImageGeneration(ctx context.Context, agent *agent.ImageGenAgent, logger *zap.Logger) *TestResult {
	result := &TestResult{Name: "图片生成测试"}
	start := time.Now()

	logger.Info("开始测试图片生成")

	prompt := "请帮我生成一张卡通风格的可爱猫咪图片，背景是蓝天白云"

	response, err := agent.Run(ctx, prompt)
	if err != nil {
		result.Error = err.Error()
		result.Duration = time.Since(start)
		logger.Error("图片生成测试失败", zap.Error(err))
		return result
	}

	result.Success = true
	result.Duration = time.Since(start)
	result.Output = response.FinalResponse

	logger.Info("图片生成测试完成",
		zap.Duration("duration", result.Duration),
		zap.Int("image_count", len(response.ImagePaths)))

	return result
}

// testGIFGeneration 测试动图生成
func testGIFGeneration(ctx context.Context, agent *agent.ImageGenAgent, logger *zap.Logger) *TestResult {
	result := &TestResult{Name: "动图生成测试"}
	start := time.Now()

	logger.Info("开始测试动图生成")

	prompt := "请帮我生成一个可爱小猫跳跃的动图"

	response, err := agent.Run(ctx, prompt)
	if err != nil {
		result.Error = err.Error()
		result.Duration = time.Since(start)
		logger.Error("动图生成测试失败", zap.Error(err))
		return result
	}

	result.Success = true
	result.Duration = time.Since(start)
	result.Output = response.FinalResponse

	logger.Info("动图生成测试完成", zap.Duration("duration", result.Duration))

	return result
}

// testStickerGeneration 测试表情包生成
func testStickerGeneration(ctx context.Context, agent *agent.ImageGenAgent, logger *zap.Logger) *TestResult {
	result := &TestResult{Name: "表情包生成测试"}
	start := time.Now()

	logger.Info("开始测试表情包生成")

	prompt := "请帮我生成一个惊讶表情的猫咪表情包，上面写着'什么？！'"

	response, err := agent.Run(ctx, prompt)
	if err != nil {
		result.Error = err.Error()
		result.Duration = time.Since(start)
		logger.Error("表情包生成测试失败", zap.Error(err))
		return result
	}

	result.Success = true
	result.Duration = time.Since(start)
	result.Output = response.FinalResponse

	logger.Info("表情包生成测试完成", zap.Duration("duration", result.Duration))

	return result
}

// testPromptOptimization 测试提示词优化
func testPromptOptimization(ctx context.Context, agent *agent.ImageGenAgent, logger *zap.Logger) *TestResult {
	result := &TestResult{Name: "提示词优化测试"}
	start := time.Now()

	logger.Info("开始测试提示词优化")

	prompt := "请帮我优化这个图片描述：一只猫"

	response, err := agent.Run(ctx, prompt)
	if err != nil {
		result.Error = err.Error()
		result.Duration = time.Since(start)
		logger.Error("提示词优化测试失败", zap.Error(err))
		return result
	}

	result.Success = true
	result.Duration = time.Since(start)
	result.Output = response.FinalResponse

	logger.Info("提示词优化测试完成", zap.Duration("duration", result.Duration))

	return result
}

// testStyleAnalysis 测试风格分析
func testStyleAnalysis(ctx context.Context, agent *agent.ImageGenAgent, logger *zap.Logger) *TestResult {
	result := &TestResult{Name: "风格分析测试"}
	start := time.Now()

	logger.Info("开始测试风格分析")

	prompt := "请帮我分析一下'日系动漫风格的少女在樱花树下'这个描述适合用什么风格和尺寸"

	response, err := agent.Run(ctx, prompt)
	if err != nil {
		result.Error = err.Error()
		result.Duration = time.Since(start)
		logger.Error("风格分析测试失败", zap.Error(err))
		return result
	}

	result.Success = true
	result.Duration = time.Since(start)
	result.Output = response.FinalResponse

	logger.Info("风格分析测试完成", zap.Duration("duration", result.Duration))

	return result
}

// printResults 打印测试结果
func printResults(results map[string]*TestResult, logger *zap.Logger) {
	fmt.Println("\n" + "=" + string(make([]byte, 59)))
	fmt.Println("  📊 集成测试结果汇总")
	fmt.Println("=" + string(make([]byte, 59)))

	successCount := 0
	totalDuration := time.Duration(0)

	for _, result := range results {
		status := "❌ 失败"
		if result.Success {
			status = "✅ 成功"
			successCount++
		}
		totalDuration += result.Duration

		fmt.Printf("\n📝 %s: %s\n", result.Name, status)
		fmt.Printf("   耗时: %v\n", result.Duration)
		if result.Error != "" {
			fmt.Printf("   错误: %s\n", result.Error)
		}
	}

	fmt.Println("\n" + "-" + string(make([]byte, 59)))
	fmt.Printf("总计: %d/%d 通过, 总耗时: %v\n", successCount, len(results), totalDuration)
	fmt.Println("-" + string(make([]byte, 59)))

	// 输出 JSON 格式结果
	jsonBytes, _ := json.MarshalIndent(results, "", "  ")
	logger.Info("测试结果 JSON", zap.String("results", string(jsonBytes)))

	// 设置退出码
	if successCount != len(results) {
		os.Exit(1)
	}
}
