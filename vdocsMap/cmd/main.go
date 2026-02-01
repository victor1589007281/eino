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
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"go.uber.org/zap"

	"github.com/cloudwego/eino/vdocsMap/agent"
	"github.com/cloudwego/eino/vdocsMap/config"
	"github.com/cloudwego/eino/vdocsMap/llm"
)

var (
	configPath = flag.String("config", "config/config-dev.yaml", "配置文件路径")
	mode       = flag.String("mode", "cli", "运行模式：cli（命令行）、server（HTTP服务）")
)

func main() {
	flag.Parse()

	// 初始化日志
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	// 加载配置
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		logger.Fatal("加载配置失败", zap.Error(err))
	}

	// 创建上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 处理中断信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		logger.Info("收到中断信号，正在退出...")
		cancel()
	}()

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

	logger.Info("文生图 Agent 启动成功",
		zap.String("name", cfg.Agent.Name),
		zap.String("mode", *mode))

	switch *mode {
	case "cli":
		runCLI(ctx, imageAgent, logger)
	case "server":
		runServer(ctx, cfg, imageAgent, logger)
	default:
		logger.Fatal("未知的运行模式", zap.String("mode", *mode))
	}
}

// runCLI 运行命令行交互模式
func runCLI(ctx context.Context, imageAgent *agent.ImageGenAgent, logger *zap.Logger) {
	fmt.Println("=" + strings.Repeat("=", 59))
	fmt.Println("  🎨 文生图 Agent - 交互模式")
	fmt.Println("  输入您的需求，我将为您生成图片")
	fmt.Println("  支持：静态图片、动图(GIF)、表情包")
	fmt.Println("  输入 'exit' 或 'quit' 退出")
	fmt.Println("=" + strings.Repeat("=", 59))
	fmt.Println()

	scanner := bufio.NewScanner(os.Stdin)

	for {
		fmt.Print("🎨 请输入需求 > ")
		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		if input == "exit" || input == "quit" {
			fmt.Println("再见！👋")
			break
		}

		// 检查上下文是否已取消
		select {
		case <-ctx.Done():
			return
		default:
		}

		// 处理用户请求
		fmt.Println("\n⏳ 正在处理您的请求...")

		response, err := imageAgent.Run(ctx, input)
		if err != nil {
			logger.Error("处理请求失败", zap.Error(err))
			fmt.Printf("❌ 处理失败: %v\n\n", err)
			continue
		}

		// 显示结果
		fmt.Println("\n" + strings.Repeat("-", 60))
		fmt.Println("📝 处理结果:")
		fmt.Println(response.FinalResponse)

		if len(response.ImagePaths) > 0 {
			fmt.Println("\n🖼️ 生成的图片:")
			for i, path := range response.ImagePaths {
				fmt.Printf("  %d. %s\n", i+1, path)
			}
		}

		if len(response.ToolCalls) > 0 {
			fmt.Println("\n🔧 工具调用记录:")
			for _, tc := range response.ToolCalls {
				fmt.Printf("  - %s\n", tc.ToolName)
			}
		}

		fmt.Println(strings.Repeat("-", 60) + "\n")
	}
}

// runServer 运行 HTTP 服务模式
func runServer(ctx context.Context, cfg *config.Config, imageAgent *agent.ImageGenAgent, logger *zap.Logger) {
	// TODO: 实现 HTTP 服务
	// 可以使用 gin、fiber 或标准库 net/http
	logger.Info("HTTP 服务模式尚未实现",
		zap.String("host", cfg.Server.Host),
		zap.Int("port", cfg.Server.Port))

	// 等待中断信号
	<-ctx.Done()
}
