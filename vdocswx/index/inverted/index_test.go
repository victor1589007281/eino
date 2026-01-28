package inverted

import (
	"context"
	"testing"
)

func TestNewInvertedIndex(t *testing.T) {
	idx := NewInvertedIndex()
	if idx == nil {
		t.Fatal("Expected non-nil index")
	}
	
	if idx.GetDocumentCount() != 0 {
		t.Errorf("Expected 0 documents, got %d", idx.GetDocumentCount())
	}
	
	if idx.GetTermCount() != 0 {
		t.Errorf("Expected 0 terms, got %d", idx.GetTermCount())
	}
}

func TestInvertedIndex_AddDocument(t *testing.T) {
	idx := NewInvertedIndex()
	ctx := context.Background()
	
	doc := &Document{
		ID:      "doc1",
		Title:   "测试文档",
		Content: "这是一个测试文档，用于测试倒排索引功能。",
	}
	
	err := idx.AddDocument(ctx, doc)
	if err != nil {
		t.Fatalf("AddDocument failed: %v", err)
	}
	
	if idx.GetDocumentCount() != 1 {
		t.Errorf("Expected 1 document, got %d", idx.GetDocumentCount())
	}
	
	// 验证文档可以被检索
	retrieved, ok := idx.GetDocument("doc1")
	if !ok {
		t.Fatal("Document not found")
	}
	
	if retrieved.Title != doc.Title {
		t.Errorf("Expected title '%s', got '%s'", doc.Title, retrieved.Title)
	}
}

func TestInvertedIndex_RemoveDocument(t *testing.T) {
	idx := NewInvertedIndex()
	ctx := context.Background()
	
	doc := &Document{
		ID:      "doc1",
		Title:   "测试文档",
		Content: "这是一个测试文档",
	}
	
	idx.AddDocument(ctx, doc)
	
	if idx.GetDocumentCount() != 1 {
		t.Fatalf("Expected 1 document, got %d", idx.GetDocumentCount())
	}
	
	err := idx.RemoveDocument(ctx, "doc1")
	if err != nil {
		t.Fatalf("RemoveDocument failed: %v", err)
	}
	
	if idx.GetDocumentCount() != 0 {
		t.Errorf("Expected 0 documents after removal, got %d", idx.GetDocumentCount())
	}
	
	// 验证文档已被删除
	_, ok := idx.GetDocument("doc1")
	if ok {
		t.Error("Document should not exist after removal")
	}
}

func TestInvertedIndex_Search(t *testing.T) {
	idx := NewInvertedIndex()
	ctx := context.Background()
	
	// 添加多个文档
	docs := []*Document{
		{ID: "doc1", Title: "Python教程", Content: "Python是一种流行的编程语言，适合初学者学习"},
		{ID: "doc2", Title: "Go教程", Content: "Go语言是Google开发的编程语言，性能优秀"},
		{ID: "doc3", Title: "美食指南", Content: "这是一份美食推荐指南，介绍各地特色美食"},
	}
	
	for _, doc := range docs {
		if err := idx.AddDocument(ctx, doc); err != nil {
			t.Fatalf("AddDocument failed: %v", err)
		}
	}
	
	// 搜索"编程语言"
	results, err := idx.Search(ctx, "编程语言", 10)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	
	// 应该找到doc1和doc2
	if len(results) < 2 {
		t.Errorf("Expected at least 2 results for '编程语言', got %d", len(results))
	}
	
	// 搜索"美食"
	results, err = idx.Search(ctx, "美食", 10)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	
	// 应该找到doc3
	if len(results) == 0 {
		t.Error("Expected at least 1 result for '美食'")
	}
}

func TestInvertedIndex_Search_EmptyQuery(t *testing.T) {
	idx := NewInvertedIndex()
	ctx := context.Background()
	
	results, err := idx.Search(ctx, "", 10)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	
	if results != nil && len(results) != 0 {
		t.Errorf("Expected empty results for empty query, got %d results", len(results))
	}
}

func TestInvertedIndex_Search_TopK(t *testing.T) {
	idx := NewInvertedIndex()
	ctx := context.Background()
	
	// 添加多个文档
	for i := 0; i < 10; i++ {
		doc := &Document{
			ID:      fmt.Sprintf("doc%d", i),
			Title:   "测试文档",
			Content: "这是一个测试文档内容",
		}
		idx.AddDocument(ctx, doc)
	}
	
	// 搜索并限制结果数量
	results, err := idx.Search(ctx, "测试", 3)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	
	if len(results) > 3 {
		t.Errorf("Expected at most 3 results with topK=3, got %d", len(results))
	}
}

func TestAnalyzer_Analyze(t *testing.T) {
	analyzer := &Analyzer{stopWords: make(map[string]bool)}
	analyzer.stopWords["的"] = true
	analyzer.stopWords["是"] = true
	analyzer.stopWords["the"] = true
	
	tests := []struct {
		name     string
		input    string
		minTerms int
	}{
		{
			name:     "Chinese text",
			input:    "这是一个测试",
			minTerms: 2, // 去除停用词后
		},
		{
			name:     "English text",
			input:    "This is a test document",
			minTerms: 2,
		},
		{
			name:     "Mixed text",
			input:    "这是一个Python测试",
			minTerms: 3,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			terms := analyzer.Analyze(tt.input)
			if len(terms) < tt.minTerms {
				t.Errorf("Expected at least %d terms, got %d: %v", tt.minTerms, len(terms), terms)
			}
		})
	}
}

// 需要导入fmt包
import "fmt"

func TestInvertedIndex_Concurrent(t *testing.T) {
	idx := NewInvertedIndex()
	ctx := context.Background()
	
	// 并发添加文档
	done := make(chan bool, 10)
	
	for i := 0; i < 10; i++ {
		go func(id int) {
			doc := &Document{
				ID:      fmt.Sprintf("doc%d", id),
				Title:   fmt.Sprintf("文档%d", id),
				Content: fmt.Sprintf("这是文档%d的内容", id),
			}
			idx.AddDocument(ctx, doc)
			done <- true
		}(i)
	}
	
	// 等待所有goroutine完成
	for i := 0; i < 10; i++ {
		<-done
	}
	
	if idx.GetDocumentCount() != 10 {
		t.Errorf("Expected 10 documents, got %d", idx.GetDocumentCount())
	}
}
