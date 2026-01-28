// Package tools 测试辅助函数
package tools

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// SearchResult 搜索结果
type SearchResult struct {
	File    string
	Line    int
	Column  int
	Content string
	Context []string
}

// GrepOptions grep选项
type GrepOptions struct {
	CaseInsensitive bool
	FilePattern     string
	MaxResults      int
	Context         int
	UseRegex        bool
}

// GrepToolSimple 简化的grep工具 (for tests)
type GrepToolSimple struct {
	sourcePath string
}

// NewGrepToolSimple 创建简化grep工具
func NewGrepToolSimple(sourcePath string) *GrepToolSimple {
	return &GrepToolSimple{sourcePath: sourcePath}
}

// Search 搜索
func (t *GrepToolSimple) Search(ctx context.Context, pattern string, opts *GrepOptions) ([]*SearchResult, error) {
	if opts == nil {
		opts = &GrepOptions{MaxResults: 100}
	}

	results := make([]*SearchResult, 0)

	err := filepath.Walk(t.sourcePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}

		// 检查文件类型
		if opts.FilePattern != "" {
			matched, _ := filepath.Match(opts.FilePattern, info.Name())
			if !matched {
				return nil
			}
		}

		// 读取并搜索文件
		matches := t.searchFile(path, pattern, opts)
		results = append(results, matches...)

		if opts.MaxResults > 0 && len(results) >= opts.MaxResults {
			return filepath.SkipDir
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	if opts.MaxResults > 0 && len(results) > opts.MaxResults {
		results = results[:opts.MaxResults]
	}

	return results, nil
}

func (t *GrepToolSimple) searchFile(path, pattern string, opts *GrepOptions) []*SearchResult {
	results := make([]*SearchResult, 0)

	file, err := os.Open(path)
	if err != nil {
		return results
	}
	defer file.Close()

	var re *regexp.Regexp
	if opts.UseRegex {
		flags := ""
		if opts.CaseInsensitive {
			flags = "(?i)"
		}
		re, _ = regexp.Compile(flags + pattern)
	}

	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		var matched bool
		if opts.UseRegex && re != nil {
			matched = re.MatchString(line)
		} else {
			searchLine := line
			searchPattern := pattern
			if opts.CaseInsensitive {
				searchLine = strings.ToLower(line)
				searchPattern = strings.ToLower(pattern)
			}
			matched = strings.Contains(searchLine, searchPattern)
		}

		if matched {
			relPath, _ := filepath.Rel(t.sourcePath, path)
			results = append(results, &SearchResult{
				File:    relPath,
				Line:    lineNum,
				Content: line,
			})
		}
	}

	return results
}

// MockSearchIndex 模拟搜索索引
type MockSearchIndex struct {
	terms map[string][]SearchResult
}

// Search 搜索
func (m *MockSearchIndex) Search(term string) []SearchResult {
	return m.terms[term]
}

// MockFunctionIndex 模拟函数索引
type MockFunctionIndex struct {
	functions map[string]*FunctionInfoTest
}

// FunctionInfoTest 函数信息 (for tests)
type FunctionInfoTest struct {
	Name       string
	File       string
	StartLine  int
	EndLine    int
	Signature  string
	Summary    string
	Parameters []string
	ReturnType string
}

// Get 获取
func (m *MockFunctionIndex) Get(name string) *FunctionInfoTest {
	return m.functions[name]
}

// MockCallGraph 模拟调用图
type MockCallGraph struct {
	nodes map[string]*CallNodeTest
}

// CallNodeTest 调用节点 (for tests)
type CallNodeTest struct {
	Function string
	File     string
	Callers  []string
	Callees  []string
}

// GetCallers 获取调用者
func (m *MockCallGraph) GetCallers(name string) []string {
	if node := m.nodes[name]; node != nil {
		return node.Callers
	}
	return nil
}

// GetCallees 获取被调用者
func (m *MockCallGraph) GetCallees(name string) []string {
	if node := m.nodes[name]; node != nil {
		return node.Callees
	}
	return nil
}

// MockTool 模拟工具
type MockTool struct {
	name string
}

// Name 工具名
func (m *MockTool) Name() string {
	return m.name
}

// Description 描述
func (m *MockTool) Description() string {
	return "Mock tool"
}

// Execute 执行
func (m *MockTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	return nil, nil
}
