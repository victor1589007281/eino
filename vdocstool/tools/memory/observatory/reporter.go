// Package observatory 报告生成器
package observatory

import (
	"fmt"
	"time"
)

// Reporter 报告生成器
type Reporter struct {
	analyzer  *Analyzer
	evaluator *Evaluator
}

// NewReporter 创建报告生成器
func NewReporter(analyzer *Analyzer, evaluator *Evaluator) *Reporter {
	return &Reporter{
		analyzer:  analyzer,
		evaluator: evaluator,
	}
}

// GenerateReport 生成报告
func (r *Reporter) GenerateReport(period string) (*Report, error) {
	// 解析周期
	duration, err := parsePeriod(period)
	if err != nil {
		return nil, err
	}

	report := &Report{
		GeneratedAt: time.Now(),
		Period:      period,
	}

	// 获取指标
	metrics := r.analyzer.GetRealtimeMetrics()
	report.Metrics = *metrics

	// 获取窗口统计
	var windowName string
	switch {
	case duration <= time.Hour:
		windowName = "1h"
	case duration <= 24*time.Hour:
		windowName = "24h"
	default:
		windowName = "24h"
	}

	stats := r.analyzer.GetWindowStats(windowName)
	if stats != nil {
		report.Summary = r.generateSummary(stats, metrics)
	}

	// 获取趋势
	report.Trends = *r.analyzer.GetTrends(duration)

	// 获取问题
	report.TopIssues = r.getTopIssues(windowName)

	// 生成建议
	report.Recommendations = r.generateRecommendations(metrics, stats, report.TopIssues)

	return report, nil
}

// generateSummary 生成概览
func (r *Reporter) generateSummary(stats *WindowStats, metrics *Metrics) ReportSummary {
	summary := ReportSummary{
		TotalRequests: stats.TotalRequests,
		AvgLatency:    stats.LatencyAvg.String(),
		AvgRelevance:  stats.AvgSimilarityScore,
	}

	if stats.TotalRequests > 0 {
		summary.SuccessRate = float64(stats.SuccessRequests) / float64(stats.TotalRequests)
	}

	// 判断整体健康状态
	if summary.SuccessRate >= 0.99 && stats.LatencyP95 < 200*time.Millisecond {
		summary.OverallHealth = "healthy"
	} else if summary.SuccessRate >= 0.95 && stats.LatencyP95 < 500*time.Millisecond {
		summary.OverallHealth = "good"
	} else if summary.SuccessRate >= 0.90 {
		summary.OverallHealth = "degraded"
	} else {
		summary.OverallHealth = "critical"
	}

	return summary
}

// getTopIssues 获取主要问题
func (r *Reporter) getTopIssues(windowName string) []AggregatedIssue {
	window, ok := r.analyzer.windows[windowName]
	if !ok {
		return nil
	}

	window.mu.RLock()
	events := make([]*Event, len(window.Events))
	copy(events, window.Events)
	window.mu.RUnlock()

	return r.evaluator.GetAggregatedIssues(events)
}

// generateRecommendations 生成优化建议
func (r *Reporter) generateRecommendations(metrics *Metrics, stats *WindowStats, issues []AggregatedIssue) []Recommendation {
	var recommendations []Recommendation

	// 基于延迟分析
	if metrics.Latency.P95 > 500*time.Millisecond {
		rec := Recommendation{
			ID:          "perf-high-latency",
			Priority:    "high",
			Category:    "performance",
			Title:       "检索延迟较高",
			Description: fmt.Sprintf("P95 延迟 %v，超过 500ms 的建议阈值", metrics.Latency.P95),
			Action:      "考虑优化向量索引或增加缓存预热",
			Impact:      "预计可降低延迟 30-50%",
		}

		if metrics.Latency.Breakdown.L2Avg > 200*time.Millisecond {
			rec.SuggestedChanges = map[string]interface{}{
				"l2.index_type": "hnsw",
				"l2.ef_search":  64,
			}
		}

		recommendations = append(recommendations, rec)
	}

	// 基于缓存分析
	if stats != nil && stats.CacheHitRate < 0.5 {
		recommendations = append(recommendations, Recommendation{
			ID:          "cache-low-hit-rate",
			Priority:    "medium",
			Category:    "performance",
			Title:       "缓存命中率较低",
			Description: fmt.Sprintf("L1 缓存命中率仅 %.1f%%", stats.CacheHitRate*100),
			Action:      "建议启用会话预加载和智能预读取",
			Impact:      "预计可提升命中率至 70%+",
			SuggestedChanges: map[string]interface{}{
				"prefetch.enabled":        true,
				"prefetch.session_lookahead": 5,
			},
		})
	}

	// 基于相关性分析
	if stats != nil && stats.AvgSimilarityScore < 0.7 {
		recommendations = append(recommendations, Recommendation{
			ID:          "quality-low-relevance",
			Priority:    "high",
			Category:    "quality",
			Title:       "检索相关性评分偏低",
			Description: fmt.Sprintf("平均相似度 %.2f，低于 0.7 的建议阈值", stats.AvgSimilarityScore),
			Action:      "建议调整相似度阈值或优化 embedding 模型",
			Impact:      "预计可提升结果质量",
			SuggestedChanges: map[string]interface{}{
				"retrieval.similarity_threshold": 0.65,
			},
		})
	}

	// 基于问题分析
	for _, issue := range issues {
		if issue.Count > 10 && issue.Severity == "warning" {
			var rec *Recommendation
			switch issue.Type {
			case IssueTypeEmptyResult:
				rec = &Recommendation{
					ID:          "quality-empty-results",
					Priority:    "medium",
					Category:    "quality",
					Title:       "空结果查询较多",
					Description: fmt.Sprintf("检测到 %d 次空结果查询", issue.Count),
					Action:      "考虑降低相似度阈值或扩大搜索范围",
					Impact:      "减少用户查询失败率",
				}
			case IssueTypeCacheMiss:
				rec = &Recommendation{
					ID:          "cache-frequent-miss",
					Priority:    "low",
					Category:    "performance",
					Title:       "缓存未命中频繁",
					Description: fmt.Sprintf("检测到 %d 次缓存未命中", issue.Count),
					Action:      "建议优化缓存预热策略",
					Impact:      "降低平均延迟",
				}
			}
			if rec != nil {
				recommendations = append(recommendations, *rec)
			}
		}
	}

	return recommendations
}

// GetRecommendations 获取当前建议
func (r *Reporter) GetRecommendations() []Recommendation {
	metrics := r.analyzer.GetRealtimeMetrics()
	stats := r.analyzer.GetWindowStats("1h")
	issues := r.getTopIssues("1h")
	return r.generateRecommendations(metrics, stats, issues)
}

// parsePeriod 解析时间周期
func parsePeriod(period string) (time.Duration, error) {
	switch period {
	case "1h":
		return time.Hour, nil
	case "6h":
		return 6 * time.Hour, nil
	case "24h":
		return 24 * time.Hour, nil
	case "7d":
		return 7 * 24 * time.Hour, nil
	default:
		return time.Hour, fmt.Errorf("unsupported period: %s", period)
	}
}

// ExportReport 导出报告
func (r *Reporter) ExportReport(report *Report, format string) ([]byte, error) {
	switch format {
	case "json":
		return r.exportJSON(report)
	case "markdown":
		return r.exportMarkdown(report)
	default:
		return r.exportJSON(report)
	}
}

// exportJSON 导出 JSON 格式
func (r *Reporter) exportJSON(report *Report) ([]byte, error) {
	// 使用标准 json 库
	return []byte(fmt.Sprintf(`{"generated_at":"%s","period":"%s"}`, 
		report.GeneratedAt.Format(time.RFC3339), 
		report.Period)), nil
}

// exportMarkdown 导出 Markdown 格式
func (r *Reporter) exportMarkdown(report *Report) ([]byte, error) {
	md := fmt.Sprintf(`# Memory Observatory Report

**Generated:** %s  
**Period:** %s

## Summary

- Total Requests: %d
- Success Rate: %.2f%%
- Avg Latency: %s
- Overall Health: %s

## Top Issues

`,
		report.GeneratedAt.Format(time.RFC3339),
		report.Period,
		report.Summary.TotalRequests,
		report.Summary.SuccessRate*100,
		report.Summary.AvgLatency,
		report.Summary.OverallHealth,
	)

	for i, issue := range report.TopIssues {
		md += fmt.Sprintf("%d. **%s** [%s] - %s (Count: %d)\n",
			i+1, issue.Type, issue.Severity, issue.Description, issue.Count)
	}

	md += "\n## Recommendations\n\n"
	for i, rec := range report.Recommendations {
		md += fmt.Sprintf("### %d. %s [%s]\n\n%s\n\n**Action:** %s\n\n**Impact:** %s\n\n",
			i+1, rec.Title, rec.Priority, rec.Description, rec.Action, rec.Impact)
	}

	return []byte(md), nil
}
