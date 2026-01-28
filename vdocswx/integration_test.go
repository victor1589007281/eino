// +build integration

package vdocswx

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
	"github.com/cloudwego/eino/vdocswx/stats"
	"github.com/cloudwego/eino/vdocswx/storage"
)

// 集成测试需要运行: go test -tags=integration -v ./...

func TestIntegration_PolishAPI(t *testing.T) {
	// 跳过如果没有设置LLM API Key
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// 初始化配置
	cfg := config.DefaultConfig()

	// 初始化组件
	ctx := context.Background()
	statsCollector := stats.NewCollector(&cfg.Stats)

	// 使用临时数据库
	db, err := storage.NewSQLiteStorage(":memory:")
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer db.Close()

	// 初始化缓存
	cacheImpl, err := cache.NewLRUCache(&cfg.Cache, statsCollector, "test")
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	// 初始化子Agent（不使用真实LLM）
	subAgents := map[string]agent.SubAgent{
		"grammar":   grammar.NewGrammarAgent(nil),
		"style":     style.NewStyleAgent(nil),
		"logic":     logic.NewLogicAgent(nil),
		"structure": structure.NewStructureAgent(nil),
	}

	// 初始化主Agent
	masterAgent, err := master.NewMasterAgent(cfg, nil, subAgents, cacheImpl, statsCollector)
	if err != nil {
		t.Fatalf("Failed to create master agent: %v", err)
	}

	// 创建HTTP服务器
	server := api.NewServer(cfg, masterAgent, db, statsCollector)

	// 使用httptest创建测试服务器
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 路由处理
		switch r.URL.Path {
		case "/health":
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
		case "/api/v1/polish":
			handlePolishTest(w, r, masterAgent)
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	// 测试健康检查
	t.Run("HealthCheck", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/health")
		if err != nil {
			t.Fatalf("Health check failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}
	})

	// 测试润色API
	t.Run("PolishAPI", func(t *testing.T) {
		reqBody := `{
			"content": "这是一个帐号登陆的功能,用户可以做为管理员登陆系统.",
			"type": "tech"
		}`

		resp, err := http.Post(ts.URL+"/api/v1/polish", "application/json", strings.NewReader(reqBody))
		if err != nil {
			t.Fatalf("Polish API failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Errorf("Expected status 200, got %d: %s", resp.StatusCode, string(body))
		}

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		// 验证响应包含必要字段
		if _, ok := result["polished_content"]; !ok {
			t.Error("Response missing 'polished_content' field")
		}
	})
}

func handlePolishTest(w http.ResponseWriter, r *http.Request, masterAgent *master.MasterAgent) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Content     string `json:"content"`
		Title       string `json:"title"`
		Type        string `json:"type"`
		UserContext string `json:"user_context"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	input := &agent.ArticleInput{
		Content:     req.Content,
		Title:       req.Title,
		Type:        agent.ArticleType(req.Type),
		UserContext: req.UserContext,
	}

	if input.Type == "" {
		input.Type = agent.ArticleTypeUnknown
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	result, err := masterAgent.Polish(ctx, input)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"polished_content": result.PolishedContent,
		"changes":          result.Changes,
		"statistics":       result.Statistics,
	})
}

func TestIntegration_EndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	cfg := config.DefaultConfig()

	// 初始化存储
	db, err := storage.NewSQLiteStorage(":memory:")
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer db.Close()

	// 测试文章存储
	t.Run("ArticleStorage", func(t *testing.T) {
		article := &storage.Article{
			ID:      "test-article-1",
			Title:   "测试文章",
			Content: "这是测试内容",
			Type:    "tech",
		}

		if err := db.SaveArticle(ctx, article); err != nil {
			t.Fatalf("Failed to save article: %v", err)
		}

		retrieved, err := db.GetArticle(ctx, "test-article-1")
		if err != nil {
			t.Fatalf("Failed to get article: %v", err)
		}

		if retrieved == nil {
			t.Fatal("Article not found")
		}

		if retrieved.Title != article.Title {
			t.Errorf("Title mismatch: expected '%s', got '%s'", article.Title, retrieved.Title)
		}
	})

	// 测试润色记录存储
	t.Run("PolishRecordStorage", func(t *testing.T) {
		record := &storage.PolishRecord{
			ID:              "record-1",
			ArticleID:       "test-article-1",
			OriginalContent: "原始内容",
			PolishedContent: "润色后内容",
			TokensUsed:      100,
		}

		if err := db.SavePolishRecord(ctx, record); err != nil {
			t.Fatalf("Failed to save polish record: %v", err)
		}

		records, err := db.GetPolishRecords(ctx, "test-article-1")
		if err != nil {
			t.Fatalf("Failed to get polish records: %v", err)
		}

		if len(records) == 0 {
			t.Fatal("No polish records found")
		}
	})

	// 测试缓存
	t.Run("CacheStorage", func(t *testing.T) {
		key := "test-cache-key"
		value := "test-cache-value"

		if err := db.SaveCacheEntry(ctx, key, value, nil); err != nil {
			t.Fatalf("Failed to save cache entry: %v", err)
		}

		retrieved, found, err := db.GetCacheEntry(ctx, key)
		if err != nil {
			t.Fatalf("Failed to get cache entry: %v", err)
		}

		if !found {
			t.Fatal("Cache entry not found")
		}

		if retrieved != value {
			t.Errorf("Value mismatch: expected '%s', got '%s'", value, retrieved)
		}
	})
}

func TestIntegration_IndexSystem(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()

	// 使用内存中的倒排索引
	idx := inverted.NewInvertedIndex()

	// 添加测试文档
	docs := []struct {
		id      string
		title   string
		content string
	}{
		{"doc1", "Go语言教程", "Go语言是一种高效的编程语言，适合构建微服务和云原生应用"},
		{"doc2", "Python数据分析", "Python是数据科学领域最流行的编程语言之一"},
		{"doc3", "微信公众号运营", "微信公众号是企业营销的重要渠道"},
	}

	for _, d := range docs {
		doc := &inverted.Document{
			ID:      d.id,
			Title:   d.title,
			Content: d.content,
		}
		if err := idx.AddDocument(ctx, doc); err != nil {
			t.Fatalf("Failed to add document: %v", err)
		}
	}

	// 搜索测试
	t.Run("SearchProgrammingLanguage", func(t *testing.T) {
		results, err := idx.Search(ctx, "编程语言", 10)
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		if len(results) < 2 {
			t.Errorf("Expected at least 2 results, got %d", len(results))
		}
	})

	t.Run("SearchWeChat", func(t *testing.T) {
		results, err := idx.Search(ctx, "微信", 10)
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		if len(results) == 0 {
			t.Error("Expected at least 1 result for '微信'")
		}
	})
}

// 需要导入
import (
	"github.com/cloudwego/eino/vdocswx/index/inverted"
)
