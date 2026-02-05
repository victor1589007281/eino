// Package retrieval 重排序器
package retrieval

import (
	"context"
	"math"
	"time"
)

// Reranker 重排序器
type Reranker struct {
	similarityThreshold float64
	timeDecayFactor     float64
}

// NewReranker 创建重排序器
func NewReranker(threshold, decay float64) *Reranker {
	if threshold <= 0 {
		threshold = 0.3
	}
	if decay <= 0 {
		decay = 0.95
	}
	return &Reranker{
		similarityThreshold: threshold,
		timeDecayFactor:     decay,
	}
}

// Rerank 重排序
func (r *Reranker) Rerank(ctx context.Context, items []*ContextItem, query string) []*ContextItem {
	if len(items) == 0 {
		return items
	}

	// 计算精排分数
	for _, item := range items {
		// 交叉编码器精排（简化实现）
		crossScore := r.crossEncoderScore(query, item.Content)

		// 时间衰减
		timeDecay := r.calculateTimeDecay(item)

		// 综合分数
		item.Relevance = item.Relevance*0.4 + crossScore*0.4 + timeDecay*0.2
	}

	// 排序
	r.sortByRelevance(items)

	// 去重（保留多样性）
	items = r.deduplicate(items)

	// 过滤低相关度
	items = r.filterByThreshold(items)

	return items
}

// crossEncoderScore 交叉编码器打分（简化实现）
func (r *Reranker) crossEncoderScore(query, content string) float64 {
	// 简化实现：基于词汇重叠和位置
	queryWords := tokenize(query)
	contentWords := tokenize(content)

	if len(queryWords) == 0 || len(contentWords) == 0 {
		return 0
	}

	// 计算匹配词在内容中的位置加权
	querySet := make(map[string]bool)
	for _, w := range queryWords {
		querySet[w] = true
	}

	score := 0.0
	for i, w := range contentWords {
		if querySet[w] {
			// 位置越靠前权重越高
			positionWeight := 1.0 - float64(i)/(float64(len(contentWords))+1)
			score += positionWeight
		}
	}

	// 归一化
	maxScore := float64(len(queryWords))
	if maxScore > 0 {
		score = score / maxScore
	}

	return math.Min(score, 1.0)
}

// calculateTimeDecay 计算时间衰减
func (r *Reranker) calculateTimeDecay(item *ContextItem) float64 {
	// 简化实现：假设所有 L1 内容都是最新的
	if item.Source == "L1" {
		return 1.0
	}

	// L2/L3 内容应用衰减
	// 实际应该基于消息时间戳
	return r.timeDecayFactor
}

// sortByRelevance 按相关度排序
func (r *Reranker) sortByRelevance(items []*ContextItem) {
	for i := 0; i < len(items)-1; i++ {
		for j := i + 1; j < len(items); j++ {
			if items[j].Relevance > items[i].Relevance {
				items[i], items[j] = items[j], items[i]
			}
		}
	}
}

// deduplicate 去重（保留多样性）
func (r *Reranker) deduplicate(items []*ContextItem) []*ContextItem {
	if len(items) <= 1 {
		return items
	}

	var result []*ContextItem
	contentHashes := make(map[string]bool)

	for _, item := range items {
		// 简化的内容哈希（取前100字符）
		hash := item.Content
		if len(hash) > 100 {
			hash = hash[:100]
		}

		// 检查是否已有相似内容
		isDuplicate := false
		for existingHash := range contentHashes {
			if r.isSimilarContent(hash, existingHash) {
				isDuplicate = true
				break
			}
		}

		if !isDuplicate {
			contentHashes[hash] = true
			result = append(result, item)
		}
	}

	return result
}

// isSimilarContent 检查内容是否相似
func (r *Reranker) isSimilarContent(a, b string) bool {
	// 简化实现：如果超过70%的词重叠，认为相似
	aWords := tokenize(a)
	bWords := tokenize(b)

	if len(aWords) == 0 || len(bWords) == 0 {
		return false
	}

	aSet := make(map[string]bool)
	for _, w := range aWords {
		aSet[w] = true
	}

	overlap := 0
	for _, w := range bWords {
		if aSet[w] {
			overlap++
		}
	}

	minLen := len(aWords)
	if len(bWords) < minLen {
		minLen = len(bWords)
	}

	return float64(overlap)/float64(minLen) > 0.7
}

// filterByThreshold 按阈值过滤
func (r *Reranker) filterByThreshold(items []*ContextItem) []*ContextItem {
	var result []*ContextItem
	for _, item := range items {
		if item.Relevance >= r.similarityThreshold {
			result = append(result, item)
		}
	}
	return result
}

// DiversityReranker 多样性重排序器
type DiversityReranker struct {
	maxSameSource int // 同一来源最多保留数量
}

// NewDiversityReranker 创建多样性重排序器
func NewDiversityReranker(maxSameSource int) *DiversityReranker {
	if maxSameSource <= 0 {
		maxSameSource = 3
	}
	return &DiversityReranker{
		maxSameSource: maxSameSource,
	}
}

// Rerank 多样性重排序
func (d *DiversityReranker) Rerank(items []*ContextItem) []*ContextItem {
	sourceCount := make(map[string]int)
	var result []*ContextItem

	for _, item := range items {
		source := item.Source
		if sourceCount[source] < d.maxSameSource {
			result = append(result, item)
			sourceCount[source]++
		}
	}

	return result
}

// TemporalReranker 时间重排序器
type TemporalReranker struct {
	halfLifeHours float64 // 半衰期（小时）
}

// NewTemporalReranker 创建时间重排序器
func NewTemporalReranker(halfLifeHours float64) *TemporalReranker {
	if halfLifeHours <= 0 {
		halfLifeHours = 24 // 默认24小时半衰期
	}
	return &TemporalReranker{
		halfLifeHours: halfLifeHours,
	}
}

// ApplyDecay 应用时间衰减
func (t *TemporalReranker) ApplyDecay(items []*ContextItem, timestamps []time.Time) {
	now := time.Now()

	for i, item := range items {
		if i < len(timestamps) {
			age := now.Sub(timestamps[i]).Hours()
			decay := math.Pow(0.5, age/t.halfLifeHours)
			item.Relevance *= decay
		}
	}
}
