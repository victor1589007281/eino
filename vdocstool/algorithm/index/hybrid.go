// Package index 混合检索实现
package index

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

// HybridSearcher 混合检索器
type HybridSearcher struct {
	esIndex     InvertedIndex
	milvusIndex VectorIndex
	graphIndex  GraphIndex
	
	// 可选的重排序器
	reranker    Reranker
}

// Reranker 重排序接口
type Reranker interface {
	// Rerank 重排序
	Rerank(ctx context.Context, query string, hits []*SearchHit, topK int) ([]*SearchHit, error)
}

// NewHybridSearcher 创建混合检索器
func NewHybridSearcher(es InvertedIndex, milvus VectorIndex, graph GraphIndex) *HybridSearcher {
	return &HybridSearcher{
		esIndex:     es,
		milvusIndex: milvus,
		graphIndex:  graph,
	}
}

// SetReranker 设置重排序器
func (h *HybridSearcher) SetReranker(reranker Reranker) {
	h.reranker = reranker
}

// Search 混合搜索
func (h *HybridSearcher) Search(ctx context.Context, query *SearchQuery) (*SearchResult, error) {
	start := time.Now()

	var wg sync.WaitGroup
	var mu sync.Mutex

	var keywordHits []*SearchHit
	var vectorHits []*SearchHit
	var graphHits []*SearchHit
	var keywordErr, vectorErr, graphErr error

	// 并行执行各种搜索

	// 1. 关键词搜索
	if query.UseKeyword && h.esIndex != nil && query.Text != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			keywordHits, keywordErr = h.esIndex.SearchKeyword(ctx, query.Text, query.TopK*2)
		}()
	}

	// 2. 向量搜索
	if query.UseVector && h.milvusIndex != nil && len(query.Vector) > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			filter := ""
			if query.Filters != nil && query.Filters.SessionID != "" {
				filter = `session_id == "` + query.Filters.SessionID + `"`
			}
			vectorHits, vectorErr = h.milvusIndex.SearchVector(ctx, query.Vector, query.TopK*2, filter)
		}()
	}

	// 3. 图搜索
	if query.UseGraph && h.graphIndex != nil && query.Text != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			graphHits, graphErr = h.searchGraph(ctx, query.Text, query.TopK)
		}()
	}

	wg.Wait()

	// 处理错误
	if keywordErr != nil && vectorErr != nil && graphErr != nil {
		if keywordErr != nil {
			return nil, keywordErr
		}
		if vectorErr != nil {
			return nil, vectorErr
		}
		return nil, graphErr
	}

	// 融合结果
	hits := h.fuseResults(keywordHits, vectorHits, graphHits, query.TopK)

	// 可选的重排序
	if h.reranker != nil && len(hits) > 0 && query.Text != "" {
		mu.Lock()
		rerankedHits, err := h.reranker.Rerank(ctx, query.Text, hits, query.TopK)
		mu.Unlock()
		if err == nil {
			hits = rerankedHits
		}
	}

	return &SearchResult{
		Hits:   hits,
		Total:  int64(len(hits)),
		TookMs: time.Since(start).Milliseconds(),
	}, nil
}

// searchGraph 图搜索
func (h *HybridSearcher) searchGraph(ctx context.Context, query string, topK int) ([]*SearchHit, error) {
	if h.graphIndex == nil {
		return nil, nil
	}

	// 从查询中提取可能的实体名称
	// 这里使用简单的分词方法，实际应该使用 NER
	words := extractSearchTerms(query)
	
	var allRelations []*Relation
	seenPairs := make(map[string]bool)

	// 对每个词查询相关的实体和关系
	for _, word := range words {
		if len(word) < 2 {
			continue
		}

		// 查询实体
		entities, err := h.graphIndex.QueryEntities(ctx, word, 5)
		if err != nil {
			continue
		}

		// 对每个实体查询关系
		for _, entity := range entities {
			relations, err := h.graphIndex.QueryRelations(ctx, entity.Name, 2)
			if err != nil {
				continue
			}

			for _, rel := range relations {
				// 去重
				key := fmt.Sprintf("%s-%s-%s", rel.FromEntity, rel.RelationType, rel.ToEntity)
				if seenPairs[key] {
					continue
				}
				seenPairs[key] = true
				allRelations = append(allRelations, rel)
			}
		}
	}

	// 将关系转换为搜索结果
	var hits []*SearchHit
	for _, rel := range allRelations {
		// 计算相关性分数
		score := rel.Weight
		if score == 0 {
			score = 0.5
		}

		hits = append(hits, &SearchHit{
			ID:      fmt.Sprintf("graph_%s_%s", rel.FromEntity, rel.ToEntity),
			Score:   score,
			Sources: []string{"graph"},
			Source: &Document{
				ID:      fmt.Sprintf("graph_%s_%s", rel.FromEntity, rel.ToEntity),
				Type:    "graph",
				Title:   fmt.Sprintf("%s -> %s -> %s", rel.FromEntity, rel.RelationType, rel.ToEntity),
				Summary: fmt.Sprintf("关系: %s %s %s", rel.FromEntity, rel.RelationType, rel.ToEntity),
			},
		})

		if len(hits) >= topK {
			break
		}
	}

	return hits, nil
}

// extractSearchTerms 提取搜索词
func extractSearchTerms(query string) []string {
	// 简单分词
	var terms []string
	var current []rune
	
	for _, r := range query {
		if r == ' ' || r == ',' || r == '.' || r == '?' || r == '!' ||
			r == '，' || r == '。' || r == '？' || r == '！' {
			if len(current) > 0 {
				terms = append(terms, string(current))
				current = nil
			}
		} else {
			current = append(current, r)
		}
	}
	
	if len(current) > 0 {
		terms = append(terms, string(current))
	}
	
	return terms
}

// fuseResults RRF 融合结果
// RRF (Reciprocal Rank Fusion) 是一种简单有效的结果融合算法
func (h *HybridSearcher) fuseResults(keyword, vector, graph []*SearchHit, topK int) []*SearchHit {
	const k = 60 // RRF 常数

	scores := make(map[string]float64)
	metadata := make(map[string]*SearchHit)

	// 处理关键词结果
	for i, hit := range keyword {
		scores[hit.ID] += 1.0 / float64(k+i+1)
		if existing, ok := metadata[hit.ID]; ok {
			existing.Sources = append(existing.Sources, "keyword")
			existing.Highlights = hit.Highlights
		} else {
			metadata[hit.ID] = &SearchHit{
				ID:         hit.ID,
				Score:      hit.Score,
				Source:     hit.Source,
				Highlights: hit.Highlights,
				Sources:    []string{"keyword"},
			}
		}
	}

	// 处理向量结果
	for i, hit := range vector {
		scores[hit.ID] += 1.0 / float64(k+i+1)
		if existing, ok := metadata[hit.ID]; ok {
			existing.Sources = append(existing.Sources, "vector")
			existing.VectorDistance = hit.VectorDistance
		} else {
			metadata[hit.ID] = &SearchHit{
				ID:             hit.ID,
				VectorDistance: hit.VectorDistance,
				Sources:        []string{"vector"},
			}
		}
	}

	// 处理图结果
	for i, hit := range graph {
		scores[hit.ID] += 1.0 / float64(k+i+1)
		if existing, ok := metadata[hit.ID]; ok {
			existing.Sources = append(existing.Sources, "graph")
		} else {
			metadata[hit.ID] = &SearchHit{
				ID:      hit.ID,
				Source:  hit.Source,
				Sources: []string{"graph"},
			}
		}
	}

	// 按融合分数排序
	type scoredHit struct {
		id    string
		score float64
	}
	var sortedHits []scoredHit
	for id, score := range scores {
		sortedHits = append(sortedHits, scoredHit{id, score})
	}
	sort.Slice(sortedHits, func(i, j int) bool {
		return sortedHits[i].score > sortedHits[j].score
	})

	// 取 topK
	if len(sortedHits) > topK {
		sortedHits = sortedHits[:topK]
	}

	// 构建结果
	results := make([]*SearchHit, 0, len(sortedHits))
	for _, sh := range sortedHits {
		hit := metadata[sh.id]
		hit.FusedScore = sh.score
		results = append(results, hit)
	}

	return results
}

// ReRank 重排序（使用简单的基于词重叠的重排序）
func (h *HybridSearcher) ReRank(ctx context.Context, query string, hits []*SearchHit) ([]*SearchHit, error) {
	if h.reranker != nil {
		return h.reranker.Rerank(ctx, query, hits, len(hits))
	}
	
	// 使用简单的基于词重叠的重排序
	return h.simpleRerank(query, hits), nil
}

// simpleRerank 简单重排序
func (h *HybridSearcher) simpleRerank(query string, hits []*SearchHit) []*SearchHit {
	queryTerms := extractSearchTerms(query)
	queryTermSet := make(map[string]bool)
	for _, term := range queryTerms {
		queryTermSet[term] = true
	}

	// 计算每个结果的重排序分数
	for _, hit := range hits {
		if hit.Source == nil {
			continue
		}

		// 计算词重叠
		docText := hit.Source.Title + " " + hit.Source.Summary + " " + hit.Source.Content
		docTerms := extractSearchTerms(docText)
		
		overlap := 0
		for _, term := range docTerms {
			if queryTermSet[term] {
				overlap++
			}
		}

		// 结合原始分数和词重叠
		if len(queryTerms) > 0 {
			overlapScore := float64(overlap) / float64(len(queryTerms))
			hit.FusedScore = hit.FusedScore*0.7 + overlapScore*0.3
		}
	}

	// 重新排序
	sort.Slice(hits, func(i, j int) bool {
		return hits[i].FusedScore > hits[j].FusedScore
	})

	return hits
}

// CrossEncoderReranker Cross-Encoder 重排序器
type CrossEncoderReranker struct {
	// 可以集成 Cross-Encoder 模型进行精确重排序
	// 例如使用 sentence-transformers/cross-encoder 模型
	enabled bool
}

// NewCrossEncoderReranker 创建 Cross-Encoder 重排序器
func NewCrossEncoderReranker() *CrossEncoderReranker {
	return &CrossEncoderReranker{
		enabled: false, // 默认禁用，需要模型支持
	}
}

// Rerank 重排序
func (r *CrossEncoderReranker) Rerank(ctx context.Context, query string, hits []*SearchHit, topK int) ([]*SearchHit, error) {
	if !r.enabled || len(hits) == 0 {
		return hits, nil
	}

	// Cross-Encoder 重排序实现
	// 需要对每个 (query, document) 对进行评分
	// 
	// 示例代码 (需要模型服务):
	// pairs := make([][]string, len(hits))
	// for i, hit := range hits {
	//     docText := hit.Source.Title + " " + hit.Source.Summary
	//     pairs[i] = []string{query, docText}
	// }
	// 
	// scores, err := r.model.Score(ctx, pairs)
	// if err != nil {
	//     return hits, err
	// }
	// 
	// for i, score := range scores {
	//     hits[i].FusedScore = score
	// }
	// 
	// sort.Slice(hits, func(i, j int) bool {
	//     return hits[i].FusedScore > hits[j].FusedScore
	// })

	return hits, nil
}

// LLMReranker LLM 重排序器
type LLMReranker struct {
	// 使用 LLM 进行重排序
	// 可以提供更智能的相关性判断
	enabled bool
}

// NewLLMReranker 创建 LLM 重排序器
func NewLLMReranker() *LLMReranker {
	return &LLMReranker{
		enabled: false,
	}
}

// Rerank 使用 LLM 重排序
func (r *LLMReranker) Rerank(ctx context.Context, query string, hits []*SearchHit, topK int) ([]*SearchHit, error) {
	if !r.enabled || len(hits) == 0 {
		return hits, nil
	}

	// LLM 重排序实现
	// 构建 prompt 让 LLM 对结果进行排序
	// 
	// 示例 prompt:
	// ```
	// 请根据与查询 "{query}" 的相关性，对以下结果进行排序：
	// 1. {doc1_title}: {doc1_summary}
	// 2. {doc2_title}: {doc2_summary}
	// ...
	// 
	// 返回排序后的编号，最相关的在前，格式: [1, 3, 2, ...]
	// ```

	return hits, nil
}
