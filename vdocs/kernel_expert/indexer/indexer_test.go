package indexer

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestIndexPersistence_SaveAndLoad(t *testing.T) {
	// 创建临时目录
	tmpDir, err := os.MkdirTemp("", "index_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// 创建持久化管理器
	persistence, err := NewIndexPersistence(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create persistence: %v", err)
	}

	// 创建测试索引
	idx := &Index{
		Meta: &IndexMeta{
			Version:        "1.0.0",
			SourceHash:     "abc123",
			CreatedAt:      time.Now(),
			LastUpdated:    time.Now(),
			FileCount:      100,
			TotalSize:      1024000,
			IndexerVersion: IndexerVersion,
		},
		InvertedIndex: map[string]*PostingList{
			"fork": {
				Term:     "fork",
				DocFreq:  10,
				Postings: []*Posting{},
			},
		},
		FunctionSummaries: map[string]*FunctionSummary{
			"do_fork": {
				Name:       "do_fork",
				File:       "kernel/fork.c",
				StartLine:  100,
				EndLine:    200,
				Signature:  "long do_fork(unsigned long clone_flags)",
				ReturnType: "long",
				Parameters: []string{"unsigned long clone_flags"},
			},
		},
		SymbolTable: map[string]*Symbol{
			"task_struct": {
				Name: "task_struct",
				Kind: SymbolStruct,
				File: "include/linux/sched.h",
				Line: 500,
			},
		},
		CallGraph:  NewCallGraph(),
		FileHashes: map[string]string{"kernel/fork.c": "hash123"},
	}

	// 保存索引
	if err := persistence.SaveIndex(idx); err != nil {
		t.Fatalf("Failed to save index: %v", err)
	}

	// 验证索引存在
	if !persistence.IndexExists() {
		t.Error("Index should exist after save")
	}

	// 加载索引
	loaded, err := persistence.LoadIndex()
	if err != nil {
		t.Fatalf("Failed to load index: %v", err)
	}

	// 验证元信息
	if loaded.Meta.Version != idx.Meta.Version {
		t.Errorf("Meta version mismatch: %s != %s", loaded.Meta.Version, idx.Meta.Version)
	}
	if loaded.Meta.FileCount != idx.Meta.FileCount {
		t.Errorf("Meta file count mismatch: %d != %d", loaded.Meta.FileCount, idx.Meta.FileCount)
	}
}

func TestIndexPersistence_Delete(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "index_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	persistence, err := NewIndexPersistence(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create persistence: %v", err)
	}

	// 创建并保存索引
	idx := &Index{
		Meta: &IndexMeta{
			Version:        "1.0.0",
			IndexerVersion: IndexerVersion,
		},
		InvertedIndex:     make(map[string]*PostingList),
		FunctionSummaries: make(map[string]*FunctionSummary),
		SymbolTable:       make(map[string]*Symbol),
		CallGraph:         NewCallGraph(),
		FileHashes:        make(map[string]string),
	}
	persistence.SaveIndex(idx)

	// 删除索引
	if err := persistence.DeleteIndex(); err != nil {
		t.Fatalf("Failed to delete index: %v", err)
	}

	// 验证已删除
	if persistence.IndexExists() {
		t.Error("Index should not exist after delete")
	}
}

func TestInvertedIndex_Search(t *testing.T) {
	ii := NewInvertedIndex()

	// 添加文档
	ii.AddDocument("doc1", "linux kernel fork process", 100)
	ii.AddDocument("doc2", "linux scheduler process", 100)
	ii.AddDocument("doc3", "memory allocation kernel", 100)

	// 搜索测试
	results := ii.Search([]string{"linux"}, "OR", 10)
	if len(results) != 2 {
		t.Errorf("Expected 2 results for 'linux', got %d", len(results))
	}

	results = ii.Search([]string{"kernel"}, "OR", 10)
	if len(results) != 2 {
		t.Errorf("Expected 2 results for 'kernel', got %d", len(results))
	}

	results = ii.Search([]string{"nonexistent"}, "OR", 10)
	if len(results) != 0 {
		t.Errorf("Expected 0 results for 'nonexistent', got %d", len(results))
	}
}

func TestInvertedIndex_MultiTermSearch(t *testing.T) {
	ii := NewInvertedIndex()

	ii.AddDocument("doc1", "linux kernel fork", 100)
	ii.AddDocument("doc2", "linux scheduler", 100)
	ii.AddDocument("doc3", "kernel memory", 100)

	// AND搜索
	results := ii.Search([]string{"linux", "kernel"}, "AND", 10)
	if len(results) != 1 {
		t.Errorf("Expected 1 result for AND search, got %d", len(results))
	}

	// OR搜索
	results = ii.Search([]string{"linux", "memory"}, "OR", 10)
	if len(results) != 3 {
		t.Errorf("Expected 3 results for OR search, got %d", len(results))
	}
}

func TestFunctionSummaryIndex(t *testing.T) {
	fsi := NewFunctionSummaryIndex()

	// 添加函数
	fsi.Add(&FunctionSummary{
		Name:       "do_fork",
		File:       "kernel/fork.c",
		StartLine:  100,
		EndLine:    200,
		Signature:  "long do_fork(unsigned long flags)",
		ReturnType: "long",
		Parameters: []string{"unsigned long flags"},
		CallCount:  50,
	})

	fsi.Add(&FunctionSummary{
		Name:       "copy_process",
		File:       "kernel/fork.c",
		StartLine:  300,
		EndLine:    500,
		Signature:  "struct task_struct *copy_process(unsigned long flags)",
		ReturnType: "struct task_struct *",
		Parameters: []string{"unsigned long flags"},
		CallCount:  30,
	})

	// 获取函数
	summary := fsi.Get("do_fork")
	if summary == nil {
		t.Fatal("Expected to find do_fork")
	}
	if summary.StartLine != 100 {
		t.Errorf("Expected StartLine 100, got %d", summary.StartLine)
	}

	// 按文件查询
	funcs := fsi.GetByFile("kernel/fork.c")
	if len(funcs) != 2 {
		t.Errorf("Expected 2 functions in fork.c, got %d", len(funcs))
	}

	// 不存在的函数
	summary = fsi.Get("nonexistent")
	if summary != nil {
		t.Error("Expected nil for nonexistent function")
	}
}

func TestCallGraph_Operations(t *testing.T) {
	cg := NewCallGraph()

	// 添加节点
	cg.AddNode(&CallGraphNode{Function: "do_fork", File: "kernel/fork.c"})
	cg.AddNode(&CallGraphNode{Function: "copy_process", File: "kernel/fork.c"})
	cg.AddNode(&CallGraphNode{Function: "sys_fork", File: "kernel/sys.c"})

	// 添加边
	cg.AddEdge(&CallEdge{Caller: "sys_fork", Callee: "do_fork"})
	cg.AddEdge(&CallEdge{Caller: "do_fork", Callee: "copy_process"})

	// 验证调用者
	callers := cg.GetCallers("do_fork")
	if len(callers) != 1 {
		t.Errorf("Expected 1 caller, got %d", len(callers))
	}

	// 验证被调用者
	callees := cg.GetCallees("do_fork")
	if len(callees) != 1 {
		t.Errorf("Expected 1 callee, got %d", len(callees))
	}
}

func TestIndexManager_Lifecycle(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "manager_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// 创建测试源码目录
	sourceDir := filepath.Join(tmpDir, "source")
	os.MkdirAll(sourceDir, 0755)
	os.WriteFile(filepath.Join(sourceDir, "test.c"), []byte("void foo() {}"), 0644)

	indexDir := filepath.Join(tmpDir, "index")

	config := &IndexConfig{
		AutoBuild:            false,
		IncrementalThreshold: 100,
		RebuildCron:          "",
	}

	manager, err := NewIndexManager(sourceDir, indexDir, config)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	// 初始状态检查
	if manager.IsReady() {
		t.Error("Manager should not be ready before start")
	}

	if manager.IsBuilding() {
		t.Error("Manager should not be building initially")
	}
}

func TestIndexBuilder_Build(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "builder_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// 创建测试源码
	sourceDir := filepath.Join(tmpDir, "source")
	indexDir := filepath.Join(tmpDir, "index")
	os.MkdirAll(sourceDir, 0755)
	os.MkdirAll(indexDir, 0755)

	// 创建简单的C文件
	cCode := `
#include <stdio.h>

void helper_func() {
    printf("helper\n");
}

int main_func(int argc, char *argv[]) {
    helper_func();
    return 0;
}
`
	os.WriteFile(filepath.Join(sourceDir, "main.c"), []byte(cCode), 0644)

	builder := NewIndexBuilder(sourceDir, indexDir, 4)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err = builder.Build(ctx)
	if err != nil {
		t.Fatalf("Failed to build index: %v", err)
	}

	idx := builder.GetIndex()
	if idx == nil {
		t.Fatal("Index should not be nil")
	}

	if idx.Meta == nil {
		t.Fatal("Index meta should not be nil")
	}
}

func TestCalculateSourceHash(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "hash_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// 创建测试文件
	os.WriteFile(filepath.Join(tmpDir, "test.c"), []byte("int main() {}"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "test.h"), []byte("#ifndef TEST_H"), 0644)

	hash1, err := CalculateSourceHash(tmpDir)
	if err != nil {
		t.Fatalf("Failed to calculate hash: %v", err)
	}

	if hash1 == "" {
		t.Error("Hash should not be empty")
	}

	// 相同内容应该产生相同哈希
	hash2, err := CalculateSourceHash(tmpDir)
	if err != nil {
		t.Fatalf("Failed to calculate hash: %v", err)
	}

	if hash1 != hash2 {
		t.Error("Same content should produce same hash")
	}
}

func BenchmarkInvertedIndex_Search(b *testing.B) {
	ii := NewInvertedIndex()

	// 添加大量文档
	for i := 0; i < 10000; i++ {
		ii.AddDocument(string(rune(i)), "linux kernel test", 100)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ii.Search([]string{"linux"}, "OR", 10)
	}
}

func BenchmarkInvertedIndex_Add(b *testing.B) {
	ii := NewInvertedIndex()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ii.AddDocument(string(rune(i)), "linux kernel test benchmark", 100)
	}
}
