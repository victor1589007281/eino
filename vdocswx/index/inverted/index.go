// Package inverted 实现倒排索引
package inverted

import (
	"context"
	"regexp"
	"sort"
	"strings"
	"sync"
	"unicode"
)

// InvertedIndex 倒排索引
type InvertedIndex struct {
	index     map[string][]Posting // term -> postings
	documents map[string]*Document // docID -> document
	mu        sync.RWMutex
	stopWords map[string]bool
	analyzer  *Analyzer
}

// Posting 倒排记录
type Posting struct {
	DocID     string
	Positions []int
	TF        float64 // Term Frequency
}

// Document 文档
type Document struct {
	ID       string
	Title    string
	Content  string
	Metadata map[string]string
	Terms    map[string]int // term -> count
	Length   int            // 文档长度（词数）
}

// SearchResult 搜索结果
type SearchResult struct {
	DocID     string
	Score     float64
	Positions []int
	Snippet   string
}

// Analyzer 分析器
type Analyzer struct {
	stopWords map[string]bool
}

// NewInvertedIndex 创建倒排索引
func NewInvertedIndex() *InvertedIndex {
	idx := &InvertedIndex{
		index:     make(map[string][]Posting),
		documents: make(map[string]*Document),
		stopWords: make(map[string]bool),
	}
	idx.initStopWords()
	idx.analyzer = &Analyzer{stopWords: idx.stopWords}
	return idx
}

// initStopWords 初始化停用词
func (idx *InvertedIndex) initStopWords() {
	// 中文停用词
	chineseStopWords := []string{
		"的", "了", "是", "在", "我", "有", "和", "就", "不", "人",
		"都", "一", "一个", "上", "也", "很", "到", "说", "要", "去",
		"你", "会", "着", "没有", "看", "好", "自己", "这", "那", "里",
	}
	
	// 英文停用词
	englishStopWords := []string{
		"the", "a", "an", "and", "or", "but", "in", "on", "at", "to",
		"for", "of", "with", "by", "from", "as", "is", "was", "are", "were",
		"been", "be", "have", "has", "had", "do", "does", "did", "will", "would",
	}
	
	for _, word := range chineseStopWords {
		idx.stopWords[word] = true
	}
	for _, word := range englishStopWords {
		idx.stopWords[strings.ToLower(word)] = true
	}
}

// AddDocument 添加文档到索引
func (idx *InvertedIndex) AddDocument(ctx context.Context, doc *Document) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	
	// 分析文档
	terms := idx.analyzer.Analyze(doc.Content)
	doc.Terms = make(map[string]int)
	doc.Length = len(terms)
	
	// 记录词项位置
	termPositions := make(map[string][]int)
	for i, term := range terms {
		doc.Terms[term]++
		termPositions[term] = append(termPositions[term], i)
	}
	
	// 更新倒排索引
	for term, positions := range termPositions {
		posting := Posting{
			DocID:     doc.ID,
			Positions: positions,
			TF:        float64(len(positions)) / float64(doc.Length),
		}
		idx.index[term] = append(idx.index[term], posting)
	}
	
	// 存储文档
	idx.documents[doc.ID] = doc
	
	return nil
}

// RemoveDocument 从索引中删除文档
func (idx *InvertedIndex) RemoveDocument(ctx context.Context, docID string) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	
	doc, ok := idx.documents[docID]
	if !ok {
		return nil
	}
	
	// 从倒排索引中删除
	for term := range doc.Terms {
		postings := idx.index[term]
		newPostings := make([]Posting, 0, len(postings)-1)
		for _, p := range postings {
			if p.DocID != docID {
				newPostings = append(newPostings, p)
			}
		}
		if len(newPostings) > 0 {
			idx.index[term] = newPostings
		} else {
			delete(idx.index, term)
		}
	}
	
	// 删除文档
	delete(idx.documents, docID)
	
	return nil
}

// Search 搜索
func (idx *InvertedIndex) Search(ctx context.Context, query string, topK int) ([]SearchResult, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	
	// 分析查询
	queryTerms := idx.analyzer.Analyze(query)
	if len(queryTerms) == 0 {
		return nil, nil
	}
	
	// 计算文档得分
	docScores := make(map[string]float64)
	docPositions := make(map[string][]int)
	
	for _, term := range queryTerms {
		postings, ok := idx.index[term]
		if !ok {
			continue
		}
		
		// 计算IDF
		idf := idx.calculateIDF(term)
		
		for _, posting := range postings {
			// TF-IDF得分
			score := posting.TF * idf
			docScores[posting.DocID] += score
			docPositions[posting.DocID] = append(docPositions[posting.DocID], posting.Positions...)
		}
	}
	
	// 排序结果
	results := make([]SearchResult, 0, len(docScores))
	for docID, score := range docScores {
		doc := idx.documents[docID]
		snippet := idx.generateSnippet(doc.Content, docPositions[docID])
		
		results = append(results, SearchResult{
			DocID:     docID,
			Score:     score,
			Positions: docPositions[docID],
			Snippet:   snippet,
		})
	}
	
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})
	
	if topK > 0 && len(results) > topK {
		results = results[:topK]
	}
	
	return results, nil
}

// calculateIDF 计算IDF
func (idx *InvertedIndex) calculateIDF(term string) float64 {
	postings := idx.index[term]
	if len(postings) == 0 {
		return 0
	}
	
	numDocs := float64(len(idx.documents))
	docFreq := float64(len(postings))
	
	// IDF = log(N / df)
	return 1.0 + (numDocs / docFreq)
}

// generateSnippet 生成摘要
func (idx *InvertedIndex) generateSnippet(content string, positions []int) string {
	if len(positions) == 0 {
		// 返回前100个字符
		runes := []rune(content)
		if len(runes) > 100 {
			return string(runes[:100]) + "..."
		}
		return content
	}
	
	// 找到第一个匹配位置附近的文本
	runes := []rune(content)
	pos := positions[0]
	
	// 扩展窗口
	start := pos - 30
	if start < 0 {
		start = 0
	}
	end := pos + 70
	if end > len(runes) {
		end = len(runes)
	}
	
	snippet := string(runes[start:end])
	if start > 0 {
		snippet = "..." + snippet
	}
	if end < len(runes) {
		snippet = snippet + "..."
	}
	
	return snippet
}

// GetDocument 获取文档
func (idx *InvertedIndex) GetDocument(docID string) (*Document, bool) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	
	doc, ok := idx.documents[docID]
	return doc, ok
}

// GetDocumentCount 获取文档数量
func (idx *InvertedIndex) GetDocumentCount() int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return len(idx.documents)
}

// GetTermCount 获取词项数量
func (idx *InvertedIndex) GetTermCount() int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return len(idx.index)
}

// Analyze 分析文本
func (a *Analyzer) Analyze(text string) []string {
	terms := make([]string, 0)
	
	// 转小写
	text = strings.ToLower(text)
	
	// 移除Markdown标记
	text = regexp.MustCompile(`[#*_\[\]()!`+"`"+`]`).ReplaceAllString(text, " ")
	
	// 分词
	var currentWord strings.Builder
	
	for _, r := range text {
		if unicode.Is(unicode.Han, r) {
			// 中文字符：先保存当前英文词
			if currentWord.Len() > 0 {
				word := currentWord.String()
				if !a.stopWords[word] && len(word) >= 2 {
					terms = append(terms, word)
				}
				currentWord.Reset()
			}
			// 每个中文字作为一个term
			char := string(r)
			if !a.stopWords[char] {
				terms = append(terms, char)
			}
		} else if unicode.IsLetter(r) || unicode.IsDigit(r) {
			currentWord.WriteRune(r)
		} else {
			// 分隔符
			if currentWord.Len() > 0 {
				word := currentWord.String()
				if !a.stopWords[word] && len(word) >= 2 {
					terms = append(terms, word)
				}
				currentWord.Reset()
			}
		}
	}
	
	// 处理最后一个词
	if currentWord.Len() > 0 {
		word := currentWord.String()
		if !a.stopWords[word] && len(word) >= 2 {
			terms = append(terms, word)
		}
	}
	
	return terms
}
