package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestGrepToolSimple_Search(t *testing.T) {
	// 创建临时目录和测试文件
	tmpDir, err := os.MkdirTemp("", "grep_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// 创建测试文件
	testContent := `
#include <linux/sched.h>

void do_fork(void) {
    // Fork implementation
    copy_process();
}

void copy_process(void) {
    // Copy process
}
`
	os.WriteFile(filepath.Join(tmpDir, "fork.c"), []byte(testContent), 0644)

	tool := NewGrepToolSimple(tmpDir)
	ctx := context.Background()

	// 测试基本搜索
	results, err := tool.Search(ctx, "do_fork", &GrepOptions{})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) == 0 {
		t.Error("Expected some results")
	}

	// 验证结果包含文件和行号
	for _, r := range results {
		if r.File == "" {
			t.Error("File should not be empty")
		}
		if r.Line <= 0 {
			t.Error("Line number should be positive")
		}
	}
}

func TestGrepToolSimple_SearchWithOptions(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "grep_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// 创建多个测试文件
	os.WriteFile(filepath.Join(tmpDir, "test1.c"), []byte("int foo() { return 0; }"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "test2.c"), []byte("int FOO() { return 1; }"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "test.h"), []byte("int foo();"), 0644)

	tool := NewGrepToolSimple(tmpDir)
	ctx := context.Background()

	// 测试大小写不敏感搜索
	results, err := tool.Search(ctx, "foo", &GrepOptions{CaseInsensitive: true})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) < 2 {
		t.Errorf("Expected at least 2 results with case insensitive, got %d", len(results))
	}

	// 测试文件类型过滤
	results, err = tool.Search(ctx, "foo", &GrepOptions{FilePattern: "*.c"})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	for _, r := range results {
		if filepath.Ext(r.File) != ".c" {
			t.Errorf("Expected .c file, got %s", r.File)
		}
	}

	// 测试限制结果数
	results, err = tool.Search(ctx, "foo", &GrepOptions{MaxResults: 1, CaseInsensitive: true})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) > 1 {
		t.Errorf("Expected max 1 result, got %d", len(results))
	}
}

func TestGrepToolSimple_SearchRegex(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "grep_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	os.WriteFile(filepath.Join(tmpDir, "test.c"), []byte(`
void func1() {}
void func2() {}
void helper() {}
`), 0644)

	tool := NewGrepToolSimple(tmpDir)
	ctx := context.Background()

	// 测试正则表达式搜索
	results, err := tool.Search(ctx, "func[0-9]+", &GrepOptions{UseRegex: true})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("Expected 2 results for regex, got %d", len(results))
	}
}

func TestMockSearchIndex(t *testing.T) {
	mockIndex := &MockSearchIndex{
		terms: map[string][]SearchResult{
			"fork": {
				{File: "kernel/fork.c", Line: 100, Content: "do_fork implementation"},
				{File: "kernel/fork.c", Line: 200, Content: "copy_process"},
			},
		},
	}

	results := mockIndex.Search("fork")
	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}
}

func TestMockFunctionIndex(t *testing.T) {
	mockIndex := &MockFunctionIndex{
		functions: map[string]*FunctionInfoTest{
			"do_fork": {
				Name:       "do_fork",
				File:       "kernel/fork.c",
				StartLine:  100,
				EndLine:    200,
				Signature:  "long do_fork(unsigned long clone_flags)",
				Summary:    "Create a new process",
				Parameters: []string{"clone_flags: flags for cloning"},
				ReturnType: "long",
			},
		},
	}

	info := mockIndex.Get("do_fork")
	if info == nil {
		t.Fatal("Expected function info")
	}

	if info.Name != "do_fork" {
		t.Errorf("Expected name 'do_fork', got '%s'", info.Name)
	}
}

func TestMockCallGraph(t *testing.T) {
	mockGraph := &MockCallGraph{
		nodes: map[string]*CallNodeTest{
			"do_fork": {
				Function: "do_fork",
				File:     "kernel/fork.c",
				Callers:  []string{"sys_fork", "sys_clone"},
				Callees:  []string{"copy_process", "wake_up_new_task"},
			},
		},
	}

	// 测试获取调用者
	callers := mockGraph.GetCallers("do_fork")
	if len(callers) != 2 {
		t.Errorf("Expected 2 callers, got %d", len(callers))
	}

	// 测试获取被调用者
	callees := mockGraph.GetCallees("do_fork")
	if len(callees) != 2 {
		t.Errorf("Expected 2 callees, got %d", len(callees))
	}
}

func TestGrepOptions(t *testing.T) {
	opts := &GrepOptions{
		CaseInsensitive: true,
		FilePattern:     "*.c",
		MaxResults:      100,
		Context:         3,
		UseRegex:        true,
	}

	if !opts.CaseInsensitive {
		t.Error("CaseInsensitive should be true")
	}
	if opts.FilePattern != "*.c" {
		t.Error("FilePattern mismatch")
	}
}

func TestSearchResult(t *testing.T) {
	result := &SearchResult{
		File:    "kernel/fork.c",
		Line:    100,
		Column:  5,
		Content: "void do_fork(void)",
		Context: []string{"// Previous line", "void do_fork(void)", "// Next line"},
	}

	if result.File == "" {
		t.Error("File should not be empty")
	}
	if result.Line <= 0 {
		t.Error("Line should be positive")
	}
}

func BenchmarkGrepToolSimple_Search(b *testing.B) {
	tmpDir, _ := os.MkdirTemp("", "grep_bench")
	defer os.RemoveAll(tmpDir)

	// 创建测试文件
	content := make([]byte, 10000)
	for i := range content {
		content[i] = 'a'
	}
	for i := 0; i < 10; i++ {
		os.WriteFile(filepath.Join(tmpDir, "test"+string(rune('0'+i%10))+".c"), content, 0644)
	}

	tool := NewGrepToolSimple(tmpDir)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tool.Search(ctx, "aaa", &GrepOptions{MaxResults: 10})
	}
}
