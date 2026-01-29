// Package main 微信公众号文章润色专家Agent入口
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cloudwego/eino/vdocswx/agent"
	"github.com/cloudwego/eino/vdocswx/agent/grammar"
	"github.com/cloudwego/eino/vdocswx/agent/logic"
	"github.com/cloudwego/eino/vdocswx/agent/master"
	"github.com/cloudwego/eino/vdocswx/agent/structure"
	"github.com/cloudwego/eino/vdocswx/agent/style"
	"github.com/cloudwego/eino/vdocswx/api"
	"github.com/cloudwego/eino/vdocswx/cache"
	"github.com/cloudwego/eino/vdocswx/config"
	"github.com/cloudwego/eino/vdocswx/index/inverted"
	"github.com/cloudwego/eino/vdocswx/llm"
	"github.com/cloudwego/eino/vdocswx/skills"
	"github.com/cloudwego/eino/vdocswx/stats"
	"github.com/cloudwego/eino/vdocswx/storage"
)

// 版本信息（通过ldflags注入）
var (
	Version   = "dev"
	GitCommit = "unknown"
	BuildDate = "unknown"
	GoVersion = "unknown"
)

func main() {
	// 命令行参数
	var (
		configPath  = flag.String("config", "config.yaml", "配置文件路径")
		showVersion = flag.Bool("version", false, "显示版本信息")
		command     = flag.String("cmd", "serve", "执行命令: serve, polish, init")
		inputFile   = flag.String("input", "", "输入文件路径（polish命令使用）")
		outputFile  = flag.String("output", "", "输出文件路径（polish命令使用）")
	)
	flag.Parse()

	// 显示版本信息
	if *showVersion {
		printVersion()
		return
	}

	// 加载配置
	cfg, err := loadConfig(*configPath)
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// 根据命令执行
	switch *command {
	case "serve":
		runServer(cfg)
	case "polish":
		runPolish(cfg, *inputFile, *outputFile)
	case "init":
		runInit(cfg)
	case "version":
		printVersion()
	default:
		log.Fatalf("未知命令: %s", *command)
	}
}

// printVersion 打印版本信息
func printVersion() {
	fmt.Printf("微信公众号文章润色专家 Agent\n")
	fmt.Printf("版本: %s\n", Version)
	fmt.Printf("Git Commit: %s\n", GitCommit)
	fmt.Printf("构建日期: %s\n", BuildDate)
	fmt.Printf("Go版本: %s\n", GoVersion)
}

// loadConfig 加载配置
func loadConfig(path string) (*config.Config, error) {
	// 检查配置文件是否存在
	if _, err := os.Stat(path); os.IsNotExist(err) {
		log.Printf("配置文件不存在，使用默认配置")
		return config.DefaultConfig(), nil
	}

	return config.LoadConfig(path)
}

// runServer 启动HTTP服务
func runServer(cfg *config.Config) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 初始化组件
	app, err := initializeApp(ctx, cfg)
	if err != nil {
		log.Fatalf("初始化应用失败: %v", err)
	}
	defer app.Close()

	// 创建HTTP服务器
	server := api.NewServer(cfg, app.masterAgent, app.storage, app.statsCollector)

	// 启动服务器
	go func() {
		addr := fmt.Sprintf(":%d", cfg.Server.Port)
		log.Printf("启动HTTP服务器，监听端口: %s", addr)
		if err := server.Start(addr); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP服务器错误: %v", err)
		}
	}()

	// 启动统计报告
	go app.statsCollector.StartReporting(ctx)

	// 等待中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("正在关闭服务器...")

	// 优雅关闭
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("服务器关闭错误: %v", err)
	}

	log.Println("服务器已关闭")
}

// runPolish 执行单次润色
func runPolish(cfg *config.Config, inputPath, outputPath string) {
	if inputPath == "" {
		log.Fatal("请指定输入文件路径 (-input)")
	}

	ctx := context.Background()

	// 初始化组件
	app, err := initializeApp(ctx, cfg)
	if err != nil {
		log.Fatalf("初始化应用失败: %v", err)
	}
	defer app.Close()

	// 读取输入文件
	content, err := os.ReadFile(inputPath)
	if err != nil {
		log.Fatalf("读取输入文件失败: %v", err)
	}

	// 创建输入
	input := &agent.ArticleInput{
		Content:  string(content),
		FilePath: inputPath,
		Type:     agent.ArticleTypeUnknown,
	}

	// 执行润色
	log.Println("开始润色文章...")
	result, err := app.masterAgent.Polish(ctx, input)
	if err != nil {
		log.Fatalf("润色失败: %v", err)
	}

	// 输出结果
	if outputPath == "" {
		outputPath = inputPath + ".polished.md"
	}

	if err := os.WriteFile(outputPath, []byte(result.PolishedContent), 0644); err != nil {
		log.Fatalf("写入输出文件失败: %v", err)
	}

	// 打印统计信息
	log.Printf("润色完成！")
	log.Printf("原始字数: %d", result.Statistics.OriginalCharCount)
	log.Printf("润色后字数: %d", result.Statistics.PolishedCharCount)
	log.Printf("修改数量: %d", result.Statistics.TotalChanges)
	log.Printf("输出文件: %s", outputPath)

	// 打印修改详情
	if len(result.Changes) > 0 {
		log.Println("\n修改详情:")
		for i, change := range result.Changes {
			if change.Original != "" {
				log.Printf("%d. [%s] %s -> %s (%s)",
					i+1, change.Type, change.Original, change.Modified, change.Reason)
			} else {
				log.Printf("%d. [%s] 建议: %s", i+1, change.Type, change.Reason)
			}
		}
	}
}

// runInit 初始化系统
func runInit(cfg *config.Config) {
	ctx := context.Background()

	log.Println("初始化系统...")

	// 创建必要目录
	dirs := []string{
		cfg.Input.MarkdownFilePath,
		cfg.Output.WorkDirectory,
		cfg.Index.Path,
		cfg.Storage.LocalDBPath,
		cfg.Skills.Directory,
	}

	for _, dir := range dirs {
		if dir != "" {
			if err := os.MkdirAll(dir, 0755); err != nil {
				log.Printf("创建目录失败 %s: %v", dir, err)
			} else {
				log.Printf("创建目录: %s", dir)
			}
		}
	}

	// 初始化数据库
	dbPath := cfg.Storage.LocalDBPath
	if dbPath == "" {
		dbPath = "./data"
	}
	os.MkdirAll(dbPath, 0755)
	
	db, err := storage.NewSQLiteStorage(dbPath + "/vdocswx.db")
	if err != nil {
		log.Fatalf("初始化数据库失败: %v", err)
	}
	db.Close()
	log.Println("数据库初始化完成")

	// 加载技能
	if cfg.Skills.Directory != "" {
		skillLoader := skills.NewSkillLoader(cfg.Skills.Directory)
		if err := skillLoader.LoadAll(ctx); err != nil {
			log.Printf("加载技能失败: %v", err)
		} else {
			log.Printf("加载技能: %d 个", len(skillLoader.GetAllSkills()))
		}
	}

	log.Println("系统初始化完成！")
}

// App 应用实例
type App struct {
	cfg            *config.Config
	llmManager     *llm.LLMManager
	masterAgent    *master.MasterAgent
	storage        *storage.SQLiteStorage
	cache          cache.Cache
	index          *inverted.InvertedIndex
	statsCollector *stats.Collector
	skillLoader    *skills.SkillLoader
}

// Close 关闭应用
func (app *App) Close() {
	if app.storage != nil {
		app.storage.Close()
	}
}

// initializeApp 初始化应用
func initializeApp(ctx context.Context, cfg *config.Config) (*App, error) {
	app := &App{cfg: cfg}

	// 初始化统计收集器
	app.statsCollector = stats.NewCollector(&cfg.Stats)

	// 初始化存储
	dbPath := cfg.Storage.LocalDBPath
	if dbPath == "" {
		dbPath = "./data"
	}
	os.MkdirAll(dbPath, 0755)
	
	var err error
	app.storage, err = storage.NewSQLiteStorage(dbPath + "/vdocswx.db")
	if err != nil {
		return nil, fmt.Errorf("初始化存储失败: %w", err)
	}

	// 初始化缓存
	app.cache, err = cache.NewLRUCache(&cfg.Cache, app.statsCollector, "main")
	if err != nil {
		return nil, fmt.Errorf("初始化缓存失败: %w", err)
	}

	// 初始化索引
	app.index = inverted.NewInvertedIndex()

	// 初始化LLM管理器
	app.llmManager, err = llm.NewLLMManager(&cfg.LLM)
	if err != nil {
		log.Printf("初始化LLM管理器警告: %v (将使用规则引擎模式)", err)
		app.llmManager = nil
	}

	// 初始化技能加载器
	if cfg.Skills.Directory != "" {
		app.skillLoader = skills.NewSkillLoader(cfg.Skills.Directory)
		if err := app.skillLoader.LoadAll(ctx); err != nil {
			log.Printf("加载技能警告: %v", err)
		}
	}

	// 初始化子Agent
	subAgents := map[string]agent.SubAgent{
		"grammar":   grammar.NewGrammarAgent(app.llmManager),
		"style":     style.NewStyleAgent(app.llmManager),
		"logic":     logic.NewLogicAgent(app.llmManager),
		"structure": structure.NewStructureAgent(app.llmManager),
	}

	// 初始化主Agent
	app.masterAgent, err = master.NewMasterAgent(cfg, app.llmManager, subAgents, app.cache, app.statsCollector)
	if err != nil {
		return nil, fmt.Errorf("初始化主Agent失败: %w", err)
	}

	return app, nil
}
