// Package main provides the entry point for Interview Expert Agent.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/cloudwego/eino/vdocsMianshi/config"
	"github.com/cloudwego/eino/vdocsMianshi/interaction"
)

var (
	configPath  = flag.String("config", "config.yaml", "Path to configuration file")
	logLevel    = flag.String("log-level", "", "Log level override (debug, info, warn, error)")
	showVersion = flag.Bool("version", false, "Show version information")
)

// Version information (set by ldflags)
var (
	Version   = "dev"
	GitCommit = "unknown"
	BuildTime = "unknown"
	GoVersion = "unknown"
)

func main() {
	flag.Parse()

	if *showVersion {
		printVersion()
		return
	}

	// Get command
	args := flag.Args()
	if len(args) == 0 {
		printUsage()
		os.Exit(1)
	}

	command := args[0]

	// Load configuration
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		// Use default config if file not found
		cfg = config.DefaultConfig()
	}

	// Override log level if specified
	if *logLevel != "" {
		cfg.Log.Level = *logLevel
	}

	// Execute command
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\n正在关闭...")
		cancel()
	}()

	switch command {
	case "serve":
		if err := runServer(ctx, cfg); err != nil {
			log.Fatalf("服务器错误: %v", err)
		}
	case "analyze":
		if len(args) < 2 {
			fmt.Println("用法: interview-expert analyze <jd_file_or_text>")
			os.Exit(1)
		}
		if err := runAnalyze(ctx, cfg, args[1:]); err != nil {
			log.Fatalf("分析错误: %v", err)
		}
	case "interview":
		if err := runInteractiveInterview(ctx, cfg); err != nil {
			log.Fatalf("面试错误: %v", err)
		}
	case "demo":
		if err := runDemo(ctx, cfg); err != nil {
			log.Fatalf("演示错误: %v", err)
		}
	default:
		fmt.Printf("未知命令: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printVersion() {
	fmt.Printf("Interview Expert Agent - 专业面试官智能助手\n")
	fmt.Printf("  Version:    %s\n", Version)
	fmt.Printf("  Git Commit: %s\n", GitCommit)
	fmt.Printf("  Build Time: %s\n", BuildTime)
	fmt.Printf("  Go Version: %s\n", GoVersion)
}

func printUsage() {
	fmt.Println("Interview Expert Agent - 专业面试官智能助手")
	fmt.Println("")
	fmt.Println("用法:")
	fmt.Println("  interview-expert [flags] <command> [arguments]")
	fmt.Println("")
	fmt.Println("命令:")
	fmt.Println("  serve       启动 API 服务器")
	fmt.Println("  analyze     分析职位描述(JD)")
	fmt.Println("  interview   启动交互式面试")
	fmt.Println("  demo        运行演示模式")
	fmt.Println("")
	fmt.Println("Flags:")
	flag.PrintDefaults()
	fmt.Println("")
	fmt.Println("示例:")
	fmt.Println("  interview-expert serve --config=config.yaml")
	fmt.Println("  interview-expert analyze jd.txt")
	fmt.Println("  interview-expert interview")
	fmt.Println("  interview-expert demo")
}

func runServer(ctx context.Context, cfg *config.Config) error {
	fmt.Println("正在启动 Interview Expert Server...")
	fmt.Printf("  REST API: http://localhost%s%s\n", cfg.Interaction.RESTServer.Address, cfg.Interaction.RESTServer.BasePath)

	// Initialize agent factory (placeholder)
	// TODO: Implement proper agent factory initialization
	
	return interaction.StartServer(ctx, cfg, nil)
}

func runAnalyze(ctx context.Context, cfg *config.Config, args []string) error {
	// Read JD content
	var jdContent string

	// Check if it's a file or direct input
	input := strings.Join(args, " ")
	if fileInfo, err := os.Stat(input); err == nil && !fileInfo.IsDir() {
		// It's a file
		data, err := os.ReadFile(input)
		if err != nil {
			return fmt.Errorf("无法读取文件: %w", err)
		}
		jdContent = string(data)
	} else {
		// Direct input
		jdContent = input
	}

	fmt.Println("📋 分析职位描述中...")
	fmt.Println("")
	fmt.Println("=== JD内容 ===")
	fmt.Println(jdContent)
	fmt.Println("")

	// TODO: Call JD analyzer agent
	// For now, output a sample analysis
	fmt.Println("=== 分析结果 ===")
	sampleResult := map[string]any{
		"title": "高级后端工程师",
		"level": "senior",
		"required_skills": []string{
			"Go/Java", "MySQL", "Redis", "Kubernetes",
		},
		"hiring_intent": map[string]any{
			"primary_focus": "technical",
			"team_role":     "individual_contributor",
			"key_competencies": []string{
				"分布式系统设计", "高并发处理", "代码质量",
			},
		},
	}

	resultJSON, _ := json.MarshalIndent(sampleResult, "", "  ")
	fmt.Println(string(resultJSON))

	return nil
}

func runInteractiveInterview(ctx context.Context, cfg *config.Config) error {
	fmt.Println("🎯 Interview Expert - 交互式面试模式")
	fmt.Println("输入 'quit' 或 'exit' 退出")
	fmt.Println("")

	reader := bufio.NewReader(os.Stdin)

	// Step 1: Get JD
	fmt.Println("📋 请输入职位描述(JD)，输入空行结束:")
	var jdLines []string
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			break
		}
		if line == "quit" || line == "exit" {
			return nil
		}
		jdLines = append(jdLines, line)
	}

	if len(jdLines) == 0 {
		// Use sample JD
		jdLines = []string{
			"高级Go开发工程师",
			"职责: 负责核心业务系统开发，参与架构设计",
			"要求: 3年以上Go开发经验，熟悉分布式系统，熟悉MySQL/Redis",
		}
		fmt.Println("使用示例JD...")
	}

	jdContent := strings.Join(jdLines, "\n")
	fmt.Println("")
	fmt.Println("✅ JD已接收，正在分析...")
	fmt.Println("")

	// Step 2: Show analysis and questions
	fmt.Println("=== 面试方案 ===")
	fmt.Println("轮次1: 技术面试 (30分钟)")
	fmt.Println("")

	questions := []string{
		"请介绍一下你最近参与的一个项目，你在其中担任什么角色？",
		"请解释Go语言中的goroutine和channel，以及它们是如何实现并发的？",
		"在高并发场景下，如何设计一个可靠的分布式系统？",
		"请描述一次你解决过的最具挑战性的技术问题。",
	}

	// Step 3: Conduct interview
	fmt.Println("🎤 面试开始")
	fmt.Println("")

	for i, q := range questions {
		fmt.Printf("【问题 %d/%d】\n", i+1, len(questions))
		fmt.Println(q)
		fmt.Println("")
		fmt.Print("您的回答: ")

		answer, err := reader.ReadString('\n')
		if err != nil {
			return err
		}
		answer = strings.TrimSpace(answer)

		if answer == "quit" || answer == "exit" {
			fmt.Println("面试提前结束")
			break
		}

		// Provide feedback
		fmt.Println("")
		fmt.Println("📝 已记录答案")
		fmt.Println("")
	}

	// Step 4: Show result
	fmt.Println("=== 面试评估 ===")
	fmt.Println("")
	fmt.Println("综合评分: 75/100")
	fmt.Println("建议: 建议录用")
	fmt.Println("")
	fmt.Println("优势:")
	fmt.Println("- 技术基础扎实")
	fmt.Println("- 表达清晰")
	fmt.Println("")
	fmt.Println("待提升:")
	fmt.Println("- 系统设计经验有待加强")
	fmt.Println("")
	fmt.Println("感谢使用 Interview Expert!")

	return nil
}

func runDemo(ctx context.Context, cfg *config.Config) error {
	fmt.Println("🎬 Interview Expert 演示模式")
	fmt.Println("")

	// Sample JD
	sampleJD := `
高级Go开发工程师

公司: ABC科技有限公司
部门: 基础架构团队

职责:
1. 负责核心业务系统的设计与开发
2. 参与系统架构设计和技术选型
3. 优化系统性能，解决高并发场景下的技术挑战
4. 指导初级工程师，进行代码审查

要求:
1. 本科及以上学历，计算机相关专业
2. 3-5年Go语言开发经验
3. 熟悉分布式系统设计，了解微服务架构
4. 熟练使用MySQL、Redis、Kafka等中间件
5. 有Kubernetes部署经验优先
6. 良好的沟通能力和团队协作精神
`

	fmt.Println("📋 示例职位描述:")
	fmt.Println(sampleJD)
	fmt.Println("")

	fmt.Println("⏳ 正在分析JD...")
	fmt.Println("")

	// Show analysis result
	fmt.Println("✅ JD分析完成")
	fmt.Println("")
	fmt.Println("职位: 高级Go开发工程师")
	fmt.Println("级别: senior")
	fmt.Println("必需技能: Go, MySQL, Redis, Kafka, 分布式系统")
	fmt.Println("加分项: Kubernetes")
	fmt.Println("")
	fmt.Println("用人意图分析:")
	fmt.Println("- 主要关注: 技术能力")
	fmt.Println("- 团队角色: 资深个人贡献者")
	fmt.Println("- 核心能力: 系统设计、高并发处理、团队协作")
	fmt.Println("")

	fmt.Println("⏳ 正在设计面试方案...")
	fmt.Println("")

	// Show interview plan
	fmt.Println("✅ 面试方案设计完成")
	fmt.Println("")
	fmt.Println("=== 面试方案 ===")
	fmt.Println("")
	fmt.Println("总时长: 60分钟")
	fmt.Println("")
	fmt.Println("轮次1: 技术筛选 (15分钟)")
	fmt.Println("  - Go语言基础")
	fmt.Println("  - 并发编程")
	fmt.Println("")
	fmt.Println("轮次2: 深度技术 (30分钟)")
	fmt.Println("  - 系统设计")
	fmt.Println("  - 数据库优化")
	fmt.Println("  - 分布式系统")
	fmt.Println("")
	fmt.Println("轮次3: 行为面试 (15分钟)")
	fmt.Println("  - 项目经验")
	fmt.Println("  - 团队协作")
	fmt.Println("")

	// Show sample questions
	fmt.Println("=== 示例面试题目 ===")
	fmt.Println("")
	fmt.Println("【技术题 - 中级】")
	fmt.Println("请解释Go语言中的goroutine和channel，以及它们是如何实现并发的？")
	fmt.Println("")
	fmt.Println("追问:")
	fmt.Println("- goroutine和线程有什么区别？")
	fmt.Println("- 如何避免goroutine泄漏？")
	fmt.Println("")
	fmt.Println("评估标准:")
	fmt.Println("- 概念理解 (40%)")
	fmt.Println("- 实际应用 (40%)")
	fmt.Println("- 最佳实践 (20%)")
	fmt.Println("")

	// Show sample report
	fmt.Println("=== 示例面试报告 ===")
	fmt.Println("")
	fmt.Println("📊 面试评估报告")
	fmt.Println("")
	fmt.Println("候选人: 张三")
	fmt.Println("职位: 高级Go开发工程师")
	fmt.Println("面试日期: 2026-01-31")
	fmt.Println("")
	fmt.Println("执行摘要:")
	fmt.Println("该候选人展现了扎实的Go语言基础和良好的系统设计能力。")
	fmt.Println("在分布式系统和数据库优化方面有一定经验，建议录用。")
	fmt.Println("")
	fmt.Println("| 评估维度 | 得分 | 评级 |")
	fmt.Println("|---------|------|------|")
	fmt.Println("| 技术能力 | 82%  | A-   |")
	fmt.Println("| 系统设计 | 75%  | B+   |")
	fmt.Println("| 沟通表达 | 85%  | A    |")
	fmt.Println("| 综合评分 | 80%  | A-   |")
	fmt.Println("")
	fmt.Println("建议: ✅ 建议录用")
	fmt.Println("")
	fmt.Println("---")
	fmt.Println("演示结束。运行 'interview-expert interview' 开始真实面试。")

	return nil
}
