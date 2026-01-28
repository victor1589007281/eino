// +build integration

package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cloudwego/eino/vdocsbaoxian/cache"
	"github.com/cloudwego/eino/vdocsbaoxian/config"
	"github.com/cloudwego/eino/vdocsbaoxian/output"
	"github.com/cloudwego/eino/vdocsbaoxian/router"
	"github.com/cloudwego/eino/vdocsbaoxian/stats"
	"github.com/cloudwego/eino/vdocsbaoxian/storage"
)

// TestIntegration_CacheAndStorage tests cache and storage integration.
func TestIntegration_CacheAndStorage(t *testing.T) {
	tmpDir := t.TempDir()
	
	// Create storage
	storagePath := filepath.Join(tmpDir, "test.db")
	store, err := storage.NewSQLiteStorage(storagePath)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer store.Close()
	
	// Create cache
	l1Cache := cache.NewLRUCache(100, time.Minute)
	l2Cache := cache.NewLRUCache(1000, time.Hour)
	
	statsCollector := stats.NewStats()
	
	multiCache := cache.NewMultiLevelCache(
		[]cache.Cache{l1Cache, l2Cache},
		func(level int, hit bool) {
			if hit {
				statsCollector.RecordCacheHit(level, "test")
			} else {
				statsCollector.RecordCacheMiss(level, "test")
			}
		},
	)
	
	ctx := context.Background()
	
	// Store data
	testData := []byte(`{"key": "value"}`)
	err = store.Set(ctx, "test_collection", "doc1", testData)
	if err != nil {
		t.Fatalf("Storage set failed: %v", err)
	}
	
	// Retrieve and cache
	data, err := store.Get(ctx, "test_collection", "doc1")
	if err != nil {
		t.Fatalf("Storage get failed: %v", err)
	}
	
	// Cache the data
	err = multiCache.Set(ctx, "test_collection:doc1", data, 0)
	if err != nil {
		t.Fatalf("Cache set failed: %v", err)
	}
	
	// Get from cache
	cached, ok := multiCache.Get(ctx, "test_collection:doc1")
	if !ok {
		t.Fatal("Cache get failed")
	}
	
	if string(cached.([]byte)) != string(testData) {
		t.Error("Cached data doesn't match")
	}
	
	// Check stats
	summary := statsCollector.GetSummary()
	if summary.CacheStats.TotalHits == 0 {
		t.Error("Expected cache hits")
	}
}

// TestIntegration_RouterAndStats tests router and stats integration.
func TestIntegration_RouterAndStats(t *testing.T) {
	routerConfig := router.DefaultRouterConfig()
	r := router.NewRouter(routerConfig)
	
	statsCollector := stats.NewStats()
	
	// Connect stats
	r.SetStatsFunc(func(provider string, success bool, tokens int64, duration time.Duration) {
		if success {
			statsCollector.RecordTokenUsage(provider, tokens, tokens/2)
		} else {
			statsCollector.RecordProviderError(provider)
		}
	})
	
	ctx := context.Background()
	
	// Select model for different tasks
	tasks := []router.TaskType{
		router.TaskTypeSimple,
		router.TaskTypeReasoning,
		router.TaskTypeAnalysis,
	}
	
	for _, task := range tasks {
		model, err := r.SelectModel(ctx, task)
		if err != nil {
			t.Errorf("SelectModel for %s failed: %v", task, err)
			continue
		}
		
		// Simulate usage
		r.RecordUsage(model.Model, true, 100, 50*time.Millisecond)
	}
	
	// Check stats
	tokenStats := statsCollector.GetTokenStats()
	if tokenStats.TotalTokens == 0 {
		t.Error("Expected token usage recorded")
	}
	
	modelStats := r.GetModelStats()
	if len(modelStats) == 0 {
		t.Error("Expected model stats")
	}
}

// TestIntegration_OutputGeneration tests output generation.
func TestIntegration_OutputGeneration(t *testing.T) {
	// Create analysis result
	result := &output.AnalysisResult{
		Query:   "什么是保险等待期？",
		Summary: "保险等待期是指保险合同生效后的一段时间内，保险公司不承担保险责任的期间。",
		Details: "详细说明：等待期设置的目的是防止逆选择...",
		LegalBasis: []output.LegalReference{
			{
				Name:       "健康保险管理办法",
				Article:    "第17条",
				Content:    "保险公司不得在短期健康保险产品中设置等待期超过180天。",
				EffectDate: "2019-12-01",
				Status:     "现行有效",
			},
		},
		ProductTerms: []output.ProductTerm{
			{
				ProductName: "某某重疾险",
				TermSection: "等待期条款",
				Content:     "本合同等待期为90天...",
				Version:     "2024版",
			},
		},
		Diagrams: []output.Diagram{
			{
				Type:    "flowchart",
				Title:   "等待期判断流程",
				Content: "flowchart TD\n    A[出险] --> B{等待期内?}\n    B -->|是| C[不赔付]\n    B -->|否| D[正常理赔]",
			},
		},
		Tables: []output.Table{
			{
				Title:   "常见险种等待期",
				Headers: []string{"险种", "等待期"},
				Rows: [][]string{
					{"重疾险", "90-180天"},
					{"医疗险", "30-60天"},
					{"寿险（疾病身故）", "90-180天"},
				},
			},
		},
		Warnings:   []string{"具体等待期请以产品条款为准"},
		Confidence: 0.92,
		TokensUsed: 1500,
		Sources:    []string{"健康保险管理办法", "某某重疾险条款"},
		Timestamp:  time.Now(),
	}
	
	// Test summary format
	summaryFormatter := output.NewSummaryFormatter(2000)
	summary, err := summaryFormatter.Format(result)
	if err != nil {
		t.Fatalf("Summary format failed: %v", err)
	}
	if len(summary) == 0 {
		t.Error("Empty summary output")
	}
	
	// Test document format
	docFormatter := output.NewDocumentFormatter(nil)
	doc, err := docFormatter.Format(result)
	if err != nil {
		t.Fatalf("Document format failed: %v", err)
	}
	if len(doc) == 0 {
		t.Error("Empty document output")
	}
	
	// Check document contains all sections
	expectedSections := []string{
		"保险专家分析报告",
		"问题概述",
		"分析摘要",
		"法规依据",
		"产品条款参考",
		"流程图表",
		"数据表格",
		"元数据",
	}
	
	for _, section := range expectedSections {
		if !contains(doc, section) {
			t.Errorf("Document missing section: %s", section)
		}
	}
}

// TestIntegration_ConfigLoadSave tests config load and save.
func TestIntegration_ConfigLoadSave(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	
	// Create and modify config
	cfg := config.DefaultConfig()
	cfg.Log.Level = "debug"
	cfg.Cache.L1.Capacity = 5000
	cfg.LLM.DefaultProvider = "qwen"
	
	// Save
	err := config.SaveConfig(cfg, configPath)
	if err != nil {
		t.Fatalf("SaveConfig failed: %v", err)
	}
	
	// Verify file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatal("Config file not created")
	}
	
	// Load
	loaded, err := config.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	
	// Verify values
	if loaded.Log.Level != "debug" {
		t.Errorf("Expected log level 'debug', got '%s'", loaded.Log.Level)
	}
	if loaded.Cache.L1.Capacity != 5000 {
		t.Errorf("Expected L1 capacity 5000, got %d", loaded.Cache.L1.Capacity)
	}
	if loaded.LLM.DefaultProvider != "qwen" {
		t.Errorf("Expected provider 'qwen', got '%s'", loaded.LLM.DefaultProvider)
	}
}

// TestIntegration_DocumentStore tests document store operations.
func TestIntegration_DocumentStore(t *testing.T) {
	tmpDir := t.TempDir()
	storagePath := filepath.Join(tmpDir, "docs.db")
	
	store, err := storage.NewSQLiteStorage(storagePath)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer store.Close()
	
	docStore := storage.NewDocumentStore(store)
	ctx := context.Background()
	
	// Create law document
	lawDoc := &storage.Document{
		ID:         "law001",
		Collection: "laws",
		Content:    "保险法第16条内容...",
		Metadata: map[string]interface{}{
			"law_name": "保险法",
			"article":  "第16条",
		},
		Timeliness: &storage.TimelinessInfo{
			Source:      "全国人大",
			PublishDate: time.Date(2015, 4, 24, 0, 0, 0, 0, time.Local),
			EffectDate:  time.Date(2015, 4, 24, 0, 0, 0, 0, time.Local),
			IsValid:     true,
		},
	}
	
	// Save
	err = docStore.SaveDocument(ctx, lawDoc)
	if err != nil {
		t.Fatalf("SaveDocument failed: %v", err)
	}
	
	// Get
	retrieved, err := docStore.GetDocument(ctx, "laws", "law001")
	if err != nil {
		t.Fatalf("GetDocument failed: %v", err)
	}
	if retrieved == nil {
		t.Fatal("Document not found")
	}
	if retrieved.ID != "law001" {
		t.Errorf("Expected ID 'law001', got '%s'", retrieved.ID)
	}
	
	// Check timeliness
	isValid := docStore.CheckTimeliness(retrieved)
	if !isValid {
		t.Error("Document should be valid")
	}
	
	// List
	docs, err := docStore.ListDocuments(ctx, "laws", "", 10)
	if err != nil {
		t.Fatalf("ListDocuments failed: %v", err)
	}
	if len(docs) != 1 {
		t.Errorf("Expected 1 document, got %d", len(docs))
	}
	
	// Delete
	err = docStore.DeleteDocument(ctx, "laws", "law001")
	if err != nil {
		t.Fatalf("DeleteDocument failed: %v", err)
	}
	
	// Verify deletion
	deleted, _ := docStore.GetDocument(ctx, "laws", "law001")
	if deleted != nil {
		t.Error("Document should be deleted")
	}
}

// TestIntegration_StatsCollection tests comprehensive stats collection.
func TestIntegration_StatsCollection(t *testing.T) {
	s := stats.NewStats()
	
	// Set daily budget
	s.SetDailyBudget(100000)
	
	// Simulate operations
	providers := []string{"deepseek", "qwen", "glm"}
	
	for i := 0; i < 100; i++ {
		provider := providers[i%len(providers)]
		s.RecordTokenUsage(provider, 100, 50)
		
		if i%10 == 0 {
			s.RecordProviderError(provider)
		}
		
		// Cache operations
		if i%3 == 0 {
			s.RecordCacheHit(1, "index")
		} else {
			s.RecordCacheMiss(1, "index")
		}
		
		// Query operations
		s.RecordQuery(i%5 != 0, time.Duration(i)*time.Millisecond)
		
		// Sub-agent operations
		agents := []string{"LegalAgent", "ProductAgent", "ClaimAgent"}
		agentName := agents[i%len(agents)]
		s.RecordSubAgentExecution(agentName, true, time.Duration(i+1)*time.Millisecond)
	}
	
	// Get summary
	summary := s.GetSummary()
	
	// Verify token stats
	if summary.TokenStats.TotalTokens != 15000 { // 100 * 150
		t.Errorf("Expected TotalTokens 15000, got %d", summary.TokenStats.TotalTokens)
	}
	
	// Verify cache hit rate
	expectedHitRate := float64(34) / float64(100) // ~34 hits out of 100
	if summary.CacheHitRate < 0.3 || summary.CacheHitRate > 0.4 {
		t.Errorf("Expected hit rate ~0.34, got %.2f", summary.CacheHitRate)
	}
	
	// Verify agent stats
	if summary.AgentStats.TotalQueries != 100 {
		t.Errorf("Expected 100 queries, got %d", summary.AgentStats.TotalQueries)
	}
	
	// Verify budget usage
	if summary.TokenBudgetUsage <= 0 {
		t.Error("Expected positive budget usage")
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
