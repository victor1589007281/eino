// Package main provides the entry point for MySQL Expert Agent.
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"

	"github.com/cloudwego/eino/vdocsMysql/agent"
	"github.com/cloudwego/eino/vdocsMysql/config"
)

func main() {
	// Parse command line flags
	configPath := flag.String("config", "", "Path to config file")
	interactive := flag.Bool("i", false, "Run in interactive mode")
	query := flag.String("q", "", "Single query to process")
	flag.Parse()

	// Load configuration
	var cfg *config.Config
	var err error

	if *configPath != "" {
		cfg, err = config.LoadConfig(*configPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
			os.Exit(1)
		}
	} else {
		cfg = config.DefaultConfig()
	}

	// Check for API key from default provider
	apiKeySet := false
	if cfg.LLM.DefaultProvider != "" {
		if provider, ok := cfg.LLM.Providers[cfg.LLM.DefaultProvider]; ok {
			var apiKey string
			if provider.APIKeyEnv != "" {
				apiKey = os.Getenv(provider.APIKeyEnv)
			}
			if apiKey == "" {
				apiKey = provider.APIKey
			}
			if apiKey != "" {
				apiKeySet = true
			} else {
				fmt.Fprintf(os.Stderr, "Warning: API key not set for provider %s.\n", cfg.LLM.DefaultProvider)
			}
		}
	}
	_ = apiKeySet // Will be used for full agent creation

	ctx := context.Background()

	// Create master agent (without chat model for now - demo mode)
	var masterAgent *agent.MasterAgent
	// Note: Full agent creation requires a proper ToolCallingChatModel implementation.
	// For testing, we'll use demo mode.

	// Run mode
	if *query != "" {
		// Single query mode
		processQuery(ctx, masterAgent, *query, cfg)
	} else if *interactive {
		// Interactive mode
		runInteractiveMode(ctx, masterAgent, cfg)
	} else {
		// Default: show help
		printUsage()
	}
}


// processQuery processes a single query.
func processQuery(ctx context.Context, masterAgent *agent.MasterAgent, query string, cfg *config.Config) {
	fmt.Println("\n处理查询:", query)
	fmt.Println(strings.Repeat("-", 60))

	// Classify intent
	intent := agent.ClassifyIntent(query)
	fmt.Printf("识别意图: %s\n\n", intent)

	// In real implementation, run the agent
	if masterAgent == nil {
		// Demo output
		printDemoOutput(query, cfg)
		return
	}

	// Create input
	input := &adk.AgentInput{
		Messages: []*schema.Message{
			schema.UserMessage(query),
		},
		EnableStreaming: cfg.Agent.EnableStreaming,
	}

	// Run agent
	iter := masterAgent.Run(ctx, input)

	// Process events
	for {
		event, ok := iter.Next()
		if !ok {
			break
		}

		if event.Err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", event.Err)
			continue
		}

		if event.Output != nil && event.Output.MessageOutput != nil {
			msg, err := event.Output.MessageOutput.GetMessage()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error getting message: %v\n", err)
				continue
			}
			if msg != nil {
				fmt.Println(msg.Content)
			}
		}
	}
}

// runInteractiveMode runs the agent in interactive mode.
func runInteractiveMode(ctx context.Context, masterAgent *agent.MasterAgent, cfg *config.Config) {
	fmt.Println("MySQL内核专家Agent - 交互模式")
	fmt.Println("输入 'quit' 或 'exit' 退出")
	fmt.Println(strings.Repeat("=", 60))

	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Print("\n> ")
		input, err := reader.ReadString('\n')
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading input: %v\n", err)
			continue
		}

		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		if input == "quit" || input == "exit" {
			fmt.Println("再见!")
			break
		}

		if input == "help" {
			printHelp()
			continue
		}

		processQuery(ctx, masterAgent, input, cfg)
	}
}

// printUsage prints usage information.
func printUsage() {
	fmt.Println(`MySQL内核专家Agent

用法:
  mysql-expert [选项]

选项:
  -config string  配置文件路径
  -i              交互模式
  -q string       单次查询

示例:
  mysql-expert -i                     # 交互模式
  mysql-expert -q "mysql_parse函数的调用链是什么?"
  mysql-expert -config config.yaml -i

支持的问题类型:
  - 代码搜索: "找到处理SELECT的函数"
  - 原理解释: "InnoDB如何实现MVCC"
  - 调用链分析: "mysql_execute_command的调用链"
  - 性能分析: "查询执行的瓶颈在哪里"
  - 架构理解: "MySQL的线程模型"
  - 负载模拟: "模拟高并发SELECT场景"`)
}

// printHelp prints help for interactive mode.
func printHelp() {
	fmt.Print(`
可用命令:
  help    - 显示帮助
  quit    - 退出
  exit    - 退出

可以直接输入MySQL内核相关问题，例如:
  - mysql_parse函数的调用链是什么?
  - InnoDB是如何实现行锁的?
  - Buffer Pool的刷脏页机制是什么?
  - 查找trx_commit函数的定义
`)
}

// printDemoOutput prints demo output when agent is not available.
func printDemoOutput(query string, cfg *config.Config) {
	fmt.Println("\n[Demo模式 - 实际实现需要配置LLM API]")
	fmt.Println()

	intent := agent.ClassifyIntent(query)

	switch intent {
	case agent.IntentCallChain:
		fmt.Println("## 调用链分析")
		fmt.Println()
		fmt.Println("```")
		fmt.Println("mysql_parse - sql/sql_parse.cc:5421")
		fmt.Println("├── MYSQLparse - sql/sql_yacc.cc")
		fmt.Println("│   └── lex_start - sql/sql_lex.cc")
		fmt.Println("└── mysql_execute_command - sql/sql_parse.cc:2845")
		fmt.Println("    ├── execute_sqlcom_select - sql/sql_select.cc")
		fmt.Println("    └── Sql_cmd_dml::execute - sql/sql_select.cc")
		fmt.Println("```")

	case agent.IntentCodeSearch:
		fmt.Printf("\n在 %s 中搜索相关代码...\n", cfg.Source.Path)
		fmt.Println("\n### 搜索结果")
		fmt.Println()
		fmt.Println("```cpp")
		fmt.Println("// sql/sql_parse.cc:2845")
		fmt.Println("bool mysql_execute_command(THD *thd, bool first_level) {")
		fmt.Println("  // ... 命令执行入口")
		fmt.Println("}")
		fmt.Println("```")

	default:
		fmt.Println("## 分析结果")
		fmt.Println()
		fmt.Printf("问题类型: %s\n", intent)
		fmt.Println()
		fmt.Println("需要配置LLM API才能获得详细分析结果。")
	}

	fmt.Println()
	fmt.Println("---")
	fmt.Printf("源码路径: %s\n", cfg.Source.Path)
	fmt.Printf("输出模式: %s\n", cfg.Output.Mode)
}
