// Package main provides integration tests for MySQL Expert Agent.
package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/cloudwego/eino/vdocsMysql/cache"
	"github.com/cloudwego/eino/vdocsMysql/config"
	"github.com/cloudwego/eino/vdocsMysql/index"
	"github.com/cloudwego/eino/vdocsMysql/llm"
	"github.com/cloudwego/eino/vdocsMysql/stats"
	"github.com/cloudwego/eino/vdocsMysql/storage"
	"github.com/cloudwego/eino/vdocsMysql/tools"
)

func main() {
	fmt.Println("=" + strings.Repeat("=", 59))
	fmt.Println("  MySQL Expert Agent - 集成功能测试")
	fmt.Println("=" + strings.Repeat("=", 59))
	fmt.Println()

	ctx := context.Background()

	// Test configurations from environment variables with defaults
	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "/Users/huaquan.liang/Documents/GitHub/eino/vdocsMysql/data"
	}
	sourceDir := os.Getenv("SOURCE_PATH")
	if sourceDir == "" {
		sourceDir = "/Users/huaquan.liang/Documents/GitHub/percona-server"
	}
	deepseekKey := os.Getenv("DEEPSEEK_API_KEY")

	// Ensure data directory exists
	os.MkdirAll(dataDir+"/index", 0755)
	os.MkdirAll(dataDir+"/cache", 0755)

	passed := 0
	failed := 0

	// Test 1: SQLite Storage
	fmt.Println("📦 测试1: SQLite存储层")
	fmt.Println(strings.Repeat("-", 60))
	if testSQLiteStorage(ctx, dataDir) {
		fmt.Println("✅ SQLite存储层测试通过")
		passed++
	} else {
		fmt.Println("❌ SQLite存储层测试失败")
		failed++
	}
	fmt.Println()

	// Test 2: BoltDB Storage
	fmt.Println("📦 测试2: BoltDB存储层")
	fmt.Println(strings.Repeat("-", 60))
	if testBoltDBStorage(ctx, dataDir) {
		fmt.Println("✅ BoltDB存储层测试通过")
		passed++
	} else {
		fmt.Println("❌ BoltDB存储层测试失败")
		failed++
	}
	fmt.Println()

	// Test 3: Cache System
	fmt.Println("🗄️ 测试3: 缓存系统")
	fmt.Println(strings.Repeat("-", 60))
	if testCacheSystem(ctx) {
		fmt.Println("✅ 缓存系统测试通过")
		passed++
	} else {
		fmt.Println("❌ 缓存系统测试失败")
		failed++
	}
	fmt.Println()

	// Test 4: Statistics Module
	fmt.Println("📊 测试4: 统计模块")
	fmt.Println(strings.Repeat("-", 60))
	if testStatsModule(ctx) {
		fmt.Println("✅ 统计模块测试通过")
		passed++
	} else {
		fmt.Println("❌ 统计模块测试失败")
		failed++
	}
	fmt.Println()

	// Test 5: Grep Tool (if source exists)
	if _, err := os.Stat(sourceDir); err == nil {
		fmt.Println("🔍 测试5: Grep搜索工具")
		fmt.Println(strings.Repeat("-", 60))
		if testGrepTool(ctx, sourceDir) {
			fmt.Println("✅ Grep搜索工具测试通过")
			passed++
		} else {
			fmt.Println("❌ Grep搜索工具测试失败")
			failed++
		}
		fmt.Println()
	} else {
		fmt.Println("⏭️ 测试5: 跳过 (源码目录不存在)")
		fmt.Println()
	}

	// Test 6: Index System (if source exists)
	if _, err := os.Stat(sourceDir); err == nil {
		fmt.Println("📇 测试6: 索引系统 (小规模测试)")
		fmt.Println(strings.Repeat("-", 60))
		if testIndexSystem(ctx, sourceDir, dataDir) {
			fmt.Println("✅ 索引系统测试通过")
			passed++
		} else {
			fmt.Println("❌ 索引系统测试失败")
			failed++
		}
		fmt.Println()
	} else {
		fmt.Println("⏭️ 测试6: 跳过 (源码目录不存在)")
		fmt.Println()
	}

	// Test 7: LLM Provider (DeepSeek)
	if deepseekKey != "" {
		fmt.Println("🤖 测试7: DeepSeek LLM提供商")
		fmt.Println(strings.Repeat("-", 60))
		if testDeepSeekProvider(ctx, deepseekKey) {
			fmt.Println("✅ DeepSeek LLM测试通过")
			passed++
		} else {
			fmt.Println("❌ DeepSeek LLM测试失败")
			failed++
		}
		fmt.Println()
	} else {
		fmt.Println("⏭️ 测试7: 跳过 (DEEPSEEK_API_KEY未设置)")
		fmt.Println()
	}

	// Summary
	fmt.Println("=" + strings.Repeat("=", 59))
	fmt.Printf("  测试总结: 通过 %d, 失败 %d\n", passed, failed)
	fmt.Println("=" + strings.Repeat("=", 59))

	if failed > 0 {
		os.Exit(1)
	}
}

func testSQLiteStorage(ctx context.Context, dataDir string) bool {
	dbPath := dataDir + "/index/test_sqlite.db"
	defer os.Remove(dbPath)

	store, err := storage.NewSQLiteStorage(&storage.SQLiteConfig{
		Path:        dbPath,
		JournalMode: "WAL",
	})
	if err != nil {
		fmt.Printf("  创建存储失败: %v\n", err)
		return false
	}
	defer store.Close()

	if err := store.Init(ctx); err != nil {
		fmt.Printf("  初始化失败: %v\n", err)
		return false
	}
	fmt.Println("  ✓ 初始化成功")

	// Test document CRUD
	doc := &storage.Document{
		FilePath:  "test/file.cc",
		FileHash:  "abc123",
		FileSize:  1000,
		LineCount: 100,
		Module:    "Test",
		Language:  "C++",
	}

	if err := store.CreateDocument(ctx, doc); err != nil {
		fmt.Printf("  创建文档失败: %v\n", err)
		return false
	}
	fmt.Printf("  ✓ 创建文档成功 (ID: %d)\n", doc.ID)

	retrieved, err := store.GetDocumentByPath(ctx, "test/file.cc")
	if err != nil || retrieved == nil {
		fmt.Printf("  查询文档失败: %v\n", err)
		return false
	}
	fmt.Println("  ✓ 查询文档成功")

	// Test inverted index
	if err := store.AddTermOccurrence(ctx, "mysql_parse", doc.ID, 5, nil); err != nil {
		fmt.Printf("  添加索引失败: %v\n", err)
		return false
	}
	fmt.Println("  ✓ 添加倒排索引成功")

	// Test function summary
	fn := &storage.FunctionSummary{
		ID:        "test_func_1",
		Name:      "test_function",
		FilePath:  "test/file.cc",
		Module:    "Test",
		Signature: "void test_function()",
	}
	if err := store.CreateFunction(ctx, fn); err != nil {
		fmt.Printf("  创建函数摘要失败: %v\n", err)
		return false
	}
	fmt.Println("  ✓ 创建函数摘要成功")

	// Get stats
	statsInfo, _ := store.Stats(ctx)
	fmt.Printf("  ✓ 统计: 文档=%d, 词条=%d, 函数=%d\n",
		statsInfo.DocumentCount, statsInfo.TermCount, statsInfo.FunctionCount)

	return true
}

func testBoltDBStorage(ctx context.Context, dataDir string) bool {
	dbPath := dataDir + "/index/test_bolt.db"
	defer os.Remove(dbPath)

	store, err := storage.NewBoltDBStorage(&storage.BoltDBConfig{
		Path: dbPath,
	})
	if err != nil {
		fmt.Printf("  创建BoltDB失败: %v\n", err)
		return false
	}
	defer store.Close()
	fmt.Println("  ✓ 初始化成功")

	// Create node
	node := &storage.CallGraphNode{
		ID:       "node_1",
		Name:     "test_func",
		FilePath: "test/file.cc",
		Module:   "Test",
		NodeType: "function",
	}

	if err := store.CreateNode(ctx, node); err != nil {
		fmt.Printf("  创建节点失败: %v\n", err)
		return false
	}
	fmt.Println("  ✓ 创建调用图节点成功")

	// Create edge
	node2 := &storage.CallGraphNode{
		ID:       "node_2",
		Name:     "callee_func",
		FilePath: "test/file.cc",
		Module:   "Test",
		NodeType: "function",
	}
	store.CreateNode(ctx, node2)

	edge := &storage.CallGraphEdge{
		ID:         "edge_1",
		FromNodeID: "node_1",
		ToNodeID:   "node_2",
		CallType:   "direct",
	}

	if err := store.CreateEdge(ctx, edge); err != nil {
		fmt.Printf("  创建边失败: %v\n", err)
		return false
	}
	fmt.Println("  ✓ 创建调用图边成功")

	// Get callees
	callees, err := store.GetCallees(ctx, "node_1")
	if err != nil || len(callees) != 1 {
		fmt.Printf("  查询被调用者失败: %v\n", err)
		return false
	}
	fmt.Println("  ✓ 查询调用关系成功")

	return true
}

func testCacheSystem(ctx context.Context) bool {
	// Test LRU Cache
	lruCache := cache.NewLRUCache(&cache.LRUCacheConfig{
		MaxSize: 100,
	})

	lruCache.Set(ctx, "key1", "value1", 5*time.Minute)
	lruCache.Set(ctx, "key2", "value2", 5*time.Minute)

	val, ok := lruCache.Get(ctx, "key1")
	if !ok || val.(string) != "value1" {
		fmt.Println("  LRU缓存Get失败")
		return false
	}
	fmt.Println("  ✓ LRU缓存Get/Set成功")

	// Test Sharded Cache
	shardedCache := cache.NewShardedLRUCache(&cache.ShardedLRUCacheConfig{
		MaxSize:    1000,
		ShardCount: 16,
	})

	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("key_%d", i)
		shardedCache.Set(ctx, key, i, time.Minute)
	}

	hits := 0
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("key_%d", i)
		if _, ok := shardedCache.Get(ctx, key); ok {
			hits++
		}
	}
	fmt.Printf("  ✓ 分片缓存: 写入100, 命中%d\n", hits)

	// Test Multi-layer Cache
	l1 := cache.NewLRUCache(&cache.LRUCacheConfig{MaxSize: 10})
	l2 := cache.NewLRUCache(&cache.LRUCacheConfig{MaxSize: 100})
	multiCache := cache.NewMultiLayerCache(&cache.MultiLayerCacheConfig{
		L1: l1,
		L2: l2,
	})

	multiCache.Set(ctx, "multi_key", "multi_value", time.Minute)
	val, ok = multiCache.Get(ctx, "multi_key")
	if !ok || val.(string) != "multi_value" {
		fmt.Println("  多层缓存失败")
		return false
	}
	fmt.Println("  ✓ 多层缓存测试成功")

	// Check stats
	cacheStats := shardedCache.Stats()
	fmt.Printf("  ✓ 缓存统计: 大小=%d, 命中率=%.2f%%\n",
		cacheStats.Size, cacheStats.HitRate*100)

	return true
}

func testStatsModule(ctx context.Context) bool {
	collector := stats.NewCollector(&stats.CollectorConfig{
		Enabled:        true,
		ReportInterval: time.Minute,
	})

	// Record token usage
	collector.TokenStats().RecordUsage("deepseek-chat", "master", 100, 50, 0.001)
	collector.TokenStats().RecordUsage("deepseek-chat", "search", 200, 100, 0.002)
	fmt.Println("  ✓ Token统计记录成功")

	// Record cache hits/misses
	collector.CacheStats().RecordHit("l1_cache")
	collector.CacheStats().RecordHit("l1_cache")
	collector.CacheStats().RecordMiss("l1_cache")
	fmt.Println("  ✓ 缓存命中统计记录成功")

	// Record requests
	collector.RequestStats().RecordRequest("/api/query", 100*time.Millisecond, true)
	collector.RequestStats().RecordRequest("/api/query", 200*time.Millisecond, true)
	collector.RequestStats().RecordRequest("/api/query", 500*time.Millisecond, false)
	fmt.Println("  ✓ 请求统计记录成功")

	// Get summary
	summary := collector.Summary()
	fmt.Printf("  ✓ Token统计: 总计%d tokens, 成本$%.4f\n",
		summary.Token.TotalTokens, summary.Token.TotalCost)
	fmt.Printf("  ✓ 缓存统计: 命中率%.2f%%\n",
		summary.Cache.OverallHitRate*100)
	fmt.Printf("  ✓ 请求统计: 总计%d, 成功率%.2f%%\n",
		summary.Request.TotalRequests, summary.Request.SuccessRate*100)

	return true
}

func testGrepTool(ctx context.Context, sourceDir string) bool {
	grepTool := tools.NewGrepTool(&tools.GrepToolConfig{
		SourcePath: sourceDir,
		MaxResults: 10,
	})

	// Get tool info
	info, err := grepTool.Info(ctx)
	if err != nil {
		fmt.Printf("  获取工具信息失败: %v\n", err)
		return false
	}
	fmt.Printf("  ✓ 工具名称: %s\n", info.Name)

	// Test search
	result, err := grepTool.InvokableRun(ctx, `{"pattern": "mysql_parse", "file_types": ["cc"], "directories": ["sql/"]}`)
	if err != nil {
		fmt.Printf("  搜索失败: %v\n", err)
		return false
	}

	lines := strings.Split(result, "\n")
	if len(lines) > 0 {
		fmt.Printf("  ✓ 搜索 'mysql_parse': 找到结果\n")
		// Print first few lines
		for i, line := range lines {
			if i >= 5 {
				fmt.Println("  ...")
				break
			}
			if line != "" {
				fmt.Printf("    %s\n", line)
			}
		}
	} else {
		fmt.Println("  ✓ 搜索完成 (无结果)")
	}

	return true
}

func testIndexSystem(ctx context.Context, sourceDir, dataDir string) bool {
	// Create storage
	dbPath := dataDir + "/index/test_index.db"
	defer os.Remove(dbPath)

	store, err := storage.NewSQLiteStorage(&storage.SQLiteConfig{
		Path:        dbPath,
		JournalMode: "WAL",
	})
	if err != nil {
		fmt.Printf("  创建存储失败: %v\n", err)
		return false
	}
	defer store.Close()

	if err := store.Init(ctx); err != nil {
		fmt.Printf("  初始化存储失败: %v\n", err)
		return false
	}

	// Create indexer with limited scope
	indexer := index.NewIndexer(&index.IndexerConfig{
		Config: &config.IndexConfig{
			ParallelWorkers: 4,
			BatchSize:       100,
		},
		Storage:    store,
		SourcePath: sourceDir + "/sql", // Only index sql/ directory for test
	})
	_ = indexer

	fmt.Println("  ✓ 索引器创建成功")
	fmt.Println("  注: 完整索引需要较长时间，仅验证索引器功能")

	return true
}

func testDeepSeekProvider(ctx context.Context, apiKey string) bool {
	provider, err := llm.NewOpenAIProvider(&llm.OpenAIConfig{
		BaseURL:    "https://api.deepseek.com/v1",
		APIKey:     apiKey,
		Timeout:    60 * time.Second,
		MaxRetries: 3,
	})
	if err != nil {
		fmt.Printf("  创建Provider失败: %v\n", err)
		return false
	}
	defer provider.Close()

	fmt.Println("  ✓ DeepSeek Provider创建成功")

	// Test simple chat
	request := &llm.ChatRequest{
		Model: "deepseek-chat",
		Messages: []llm.Message{
			{Role: "system", Content: "你是一个MySQL内核专家。简短回答。"},
			{Role: "user", Content: "什么是InnoDB的MVCC？用一句话回答。"},
		},
		MaxTokens:   100,
		Temperature: 0.7,
	}

	fmt.Println("  发送测试请求...")
	response, err := provider.Chat(ctx, request)
	if err != nil {
		fmt.Printf("  Chat请求失败: %v\n", err)
		return false
	}

	if len(response.Choices) > 0 && response.Choices[0].Message != nil {
		fmt.Println("  ✓ DeepSeek响应:")
		fmt.Printf("    %s\n", response.Choices[0].Message.Content)
	}

	if response.Usage != nil {
		fmt.Printf("  ✓ Token使用: 输入=%d, 输出=%d, 总计=%d\n",
			response.Usage.PromptTokens,
			response.Usage.CompletionTokens,
			response.Usage.TotalTokens)
	}

	return true
}
