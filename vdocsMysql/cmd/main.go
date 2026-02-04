// Package main provides the entry point for MySQL Expert Agent.
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"github.com/cloudwego/eino/vdocsMysql/agent"
	"github.com/cloudwego/eino/vdocsMysql/config"
	"github.com/cloudwego/eino/vdocsMysql/llm"
)

func main() {
	// Parse command line flags
	configPath := flag.String("config", "", "Path to config file")
	interactive := flag.Bool("i", false, "Run in interactive mode")
	query := flag.String("q", "", "Single query to process")
	serverMode := flag.Bool("server", false, "Run in server mode")
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

	// Initialize LLM
	var chatModel model.ToolCallingChatModel
	if cfg.LLM.DefaultProvider != "" {
		if providerCfg, ok := cfg.LLM.Providers[cfg.LLM.DefaultProvider]; ok {
			// Create OpenAI provider
			openaiCfg := &llm.OpenAIConfig{
				BaseURL:   providerCfg.BaseURL,
				APIKey:    providerCfg.APIKey,
				APIKeyEnv: providerCfg.APIKeyEnv,
				Timeout:   providerCfg.Timeout,
			}

			provider, err := llm.NewOpenAIProvider(openaiCfg)
			if err != nil {
				log.Printf("Failed to create LLM provider: %v", err)
			} else {
				// Create chat model adapter
				modelName := cfg.LLM.DefaultModel
				if modelName == "" {
					// Fallback to first model in config
					for name := range providerCfg.Models {
						modelName = name
						break
					}
				}

				chatModel, err = llm.NewChatModel(context.Background(), provider, modelName)
				if err != nil {
					log.Printf("Failed to create chat model: %v", err)
				}
			}
		}
	}

	if chatModel == nil {
		log.Println("Warning: No valid LLM provider configured. Running in demo mode.")
	}

	// Ensure tags file exists
	if err := ensureTagsFile(cfg); err != nil {
		log.Printf("Failed to generate tags file: %v", err)
	}

	ctx := context.Background()

	// Create master agent
	var masterAgent *agent.MasterAgent
	if chatModel != nil {
		masterAgent, err = agent.NewMasterAgent(ctx, &agent.MasterAgentConfig{
			Config:    cfg,
			ChatModel: chatModel,
		})
		if err != nil {
			log.Printf("Failed to create master agent: %v", err)
			masterAgent = nil
		}
	}

	// Run mode
	if *serverMode {
		if err := agent.RunServerMode(ctx, masterAgent, cfg); err != nil {
			log.Fatalf("Server failed: %v", err)
		}
	} else if *query != "" {
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

// ensureTagsFile ensures the ctags file exists.
func ensureTagsFile(cfg *config.Config) error {
	indexPath := cfg.Index.Path
	if err := os.MkdirAll(indexPath, 0755); err != nil {
		return fmt.Errorf("failed to create index directory: %w", err)
	}

	tagsPath := filepath.Join(indexPath, "tags")
	if _, err := os.Stat(tagsPath); err == nil {
		return nil // exists
	}

	log.Printf("Generating tags file at %s from %s...", tagsPath, cfg.Source.Path)
	cmd := exec.Command("ctags", "-R", "-f", tagsPath, cfg.Source.Path)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
