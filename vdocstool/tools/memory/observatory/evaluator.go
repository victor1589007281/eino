// Package observatory 质量评估器
package observatory

import (
	"strings"
	"time"
)

// Evaluator 质量评估器
type Evaluator struct {
	// 规则评估器
	rules []EvaluationRule
}

// EvaluationRule 评估规则
type EvaluationRule struct {
	Name      string
	Type      IssueType
	Severity  string
	Condition func(*Event) bool
	Message   func(*Event) string
}

// NewEvaluator 创建评估器
func NewEvaluator() *Evaluator {
	e := &Evaluator{}
	e.initRules()
	return e
}

// initRules 初始化规则
func (e *Evaluator) initRules() {
	e.rules = []EvaluationRule{
		{
			Name:     "high_latency",
			Type:     IssueTypeHighLatency,
			Severity: "warning",
			Condition: func(event *Event) bool {
				return event.Latency.Total > 500*time.Millisecond
			},
			Message: func(event *Event) string {
				return "检索延迟过高: " + event.Latency.Total.String()
			},
		},
		{
			Name:     "critical_latency",
			Type:     IssueTypeHighLatency,
			Severity: "critical",
			Condition: func(event *Event) bool {
				return event.Latency.Total > 2*time.Second
			},
			Message: func(event *Event) string {
				return "检索延迟严重超标: " + event.Latency.Total.String()
			},
		},
		{
			Name:     "empty_result",
			Type:     IssueTypeEmptyResult,
			Severity: "warning",
			Condition: func(event *Event) bool {
				return len(event.RetrievedChunks) == 0 && event.Error == ""
			},
			Message: func(event *Event) string {
				return "检索结果为空"
			},
		},
		{
			Name:     "low_relevance",
			Type:     IssueTypeLowRelevance,
			Severity: "warning",
			Condition: func(event *Event) bool {
				if len(event.RetrievedChunks) == 0 {
					return false
				}
				avgScore := 0.0
				for _, chunk := range event.RetrievedChunks {
					avgScore += chunk.SimilarityScore
				}
				avgScore /= float64(len(event.RetrievedChunks))
				return avgScore < 0.5
			},
			Message: func(event *Event) string {
				return "检索结果相关性较低"
			},
		},
		{
			Name:     "token_waste",
			Type:     IssueTypeTokenWaste,
			Severity: "info",
			Condition: func(event *Event) bool {
				// 如果召回很多内容但总 token 很少，可能存在重复或低质量内容
				if len(event.RetrievedChunks) > 5 && event.TotalTokens < 500 {
					return true
				}
				return false
			},
			Message: func(event *Event) string {
				return "召回内容可能存在浪费"
			},
		},
		{
			Name:     "cache_miss",
			Type:     IssueTypeCacheMiss,
			Severity: "info",
			Condition: func(event *Event) bool {
				// 如果没有 L1 命中，可能是缓存未预热
				for _, chunk := range event.RetrievedChunks {
					if chunk.Tier == "L1" {
						return false
					}
				}
				return len(event.RetrievedChunks) > 0
			},
			Message: func(event *Event) string {
				return "L1 缓存未命中"
			},
		},
	}
}

// Evaluate 评估事件
func (e *Evaluator) Evaluate(event *Event) *EvaluationResult {
	result := &EvaluationResult{
		RequestID: event.RequestID,
	}

	// 计算相关性评分
	result.Scores = e.calculateScores(event)
	result.RelevanceScore = e.calculateOverallRelevance(result.Scores)

	// 检测问题
	for _, rule := range e.rules {
		if rule.Condition(event) {
			result.Issues = append(result.Issues, Issue{
				Type:     rule.Type,
				Severity: rule.Severity,
				Message:  rule.Message(event),
			})
		}
	}

	return result
}

// calculateScores 计算各项评分
func (e *Evaluator) calculateScores(event *Event) ScoreDetails {
	scores := ScoreDetails{}

	if len(event.RetrievedChunks) == 0 {
		return scores
	}

	// 语义匹配度 - 基于平均相似度
	var totalSimilarity float64
	for _, chunk := range event.RetrievedChunks {
		totalSimilarity += chunk.SimilarityScore
	}
	scores.SemanticMatch = totalSimilarity / float64(len(event.RetrievedChunks))

	// 关键词覆盖 - 简化实现
	scores.KeywordCoverage = e.calculateKeywordCoverage(event)

	// 主题对齐 - 基于 Top 结果的相似度
	if len(event.RetrievedChunks) > 0 {
		scores.TopicAlignment = event.RetrievedChunks[0].SimilarityScore
	}

	// 时效性评分 - 简化实现
	scores.FreshnessScore = 0.8

	// 多样性评分 - 基于不同层级的分布
	tierCounts := make(map[string]int)
	for _, chunk := range event.RetrievedChunks {
		tierCounts[chunk.Tier]++
	}
	scores.DiversityScore = float64(len(tierCounts)) / 3.0

	return scores
}

// calculateKeywordCoverage 计算关键词覆盖度
func (e *Evaluator) calculateKeywordCoverage(event *Event) float64 {
	if event.Query == "" {
		return 0
	}

	// 简化实现：检查查询中的词是否在结果中出现
	queryWords := strings.Fields(strings.ToLower(event.Query))
	if len(queryWords) == 0 {
		return 0
	}

	// 这里简化为返回固定值，实际需要检查召回内容
	return 0.7
}

// calculateOverallRelevance 计算综合相关性
func (e *Evaluator) calculateOverallRelevance(scores ScoreDetails) float64 {
	// 加权平均
	weights := map[string]float64{
		"semantic":   0.35,
		"keyword":    0.25,
		"topic":      0.20,
		"freshness":  0.10,
		"diversity":  0.10,
	}

	score := weights["semantic"]*scores.SemanticMatch +
		weights["keyword"]*scores.KeywordCoverage +
		weights["topic"]*scores.TopicAlignment +
		weights["freshness"]*scores.FreshnessScore +
		weights["diversity"]*scores.DiversityScore

	return score
}

// EvaluateBatch 批量评估
func (e *Evaluator) EvaluateBatch(events []*Event) []*EvaluationResult {
	results := make([]*EvaluationResult, len(events))
	for i, event := range events {
		results[i] = e.Evaluate(event)
	}
	return results
}

// GetAggregatedIssues 获取聚合问题
func (e *Evaluator) GetAggregatedIssues(events []*Event) []AggregatedIssue {
	issueMap := make(map[IssueType]*AggregatedIssue)

	for _, event := range events {
		result := e.Evaluate(event)
		for _, issue := range result.Issues {
			if agg, ok := issueMap[issue.Type]; ok {
				agg.Count++
				if event.Timestamp.Before(agg.FirstSeen) {
					agg.FirstSeen = event.Timestamp
				}
				if event.Timestamp.After(agg.LastSeen) {
					agg.LastSeen = event.Timestamp
				}
			} else {
				issueMap[issue.Type] = &AggregatedIssue{
					Type:        issue.Type,
					Count:       1,
					Severity:    issue.Severity,
					Description: issue.Message,
					FirstSeen:   event.Timestamp,
					LastSeen:    event.Timestamp,
				}
			}
		}
	}

	// 转换为列表
	var issues []AggregatedIssue
	for _, agg := range issueMap {
		issues = append(issues, *agg)
	}

	return issues
}
