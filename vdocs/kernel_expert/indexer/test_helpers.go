// Package indexer 测试辅助函数
package indexer

// SearchSingle 单词搜索 (wrapper for tests)
func (idx *InvertedIndex) SearchSingle(term string) []*SearchResult {
	results := idx.Search([]string{term}, "OR", 100)
	return results
}

// AddDocumentSimple 简单添加文档 (for tests)
func (idx *InvertedIndex) AddDocumentSimple(docID string, terms []string) {
	content := ""
	for _, t := range terms {
		content += t + " "
	}
	idx.AddDocument(docID, content, int64(len(content)))
}

// SearchMultiTerms 多词搜索 (for tests)
func (idx *InvertedIndex) SearchMultiTerms(terms []string, andOperator bool) []*SearchResult {
	op := "OR"
	if andOperator {
		op = "AND"
	}
	return idx.Search(terms, op, 100)
}

// FunctionSummaryIndex 函数摘要索引
type FunctionSummaryIndex struct {
	functions map[string]*FunctionSummary
}

// NewFunctionSummaryIndex 创建函数摘要索引
func NewFunctionSummaryIndex() *FunctionSummaryIndex {
	return &FunctionSummaryIndex{
		functions: make(map[string]*FunctionSummary),
	}
}

// Add 添加函数摘要
func (fsi *FunctionSummaryIndex) Add(summary *FunctionSummary) {
	fsi.functions[summary.Name] = summary
}

// Get 获取函数摘要
func (fsi *FunctionSummaryIndex) Get(name string) *FunctionSummary {
	return fsi.functions[name]
}

// GetByFile 按文件获取函数列表
func (fsi *FunctionSummaryIndex) GetByFile(file string) []*FunctionSummary {
	result := make([]*FunctionSummary, 0)
	for _, f := range fsi.functions {
		if f.File == file {
			result = append(result, f)
		}
	}
	return result
}
