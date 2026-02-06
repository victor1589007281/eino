// Package router 质量追踪器
package router

import (
	"sync"
	"time"
)

// QualityTracker 质量追踪器
type QualityTracker struct {
	// 滑动窗口存储
	windows map[string]*SlidingWindow
	
	mu sync.RWMutex
}

// SlidingWindow 滑动窗口
type SlidingWindow struct {
	Size    int
	Records []*SearchRecord
	Index   int
	Count   int
	
	mu sync.RWMutex
}

// SearchRecord 搜索记录
type SearchRecord struct {
	Timestamp   time.Time
	QueryType   QueryType
	ResultCount int
	Latency     time.Duration
	Success     bool
	ErrorType   string
	
	// 质量反馈 (异步更新)
	UserClicked  bool
	ClickedIndex int
	DwellTime    float64
}

// QualityMetrics 质量指标
type QualityMetrics struct {
	// 基础指标
	SuccessRate   float64       `json:"success_rate"`
	AvgResultCount float64      `json:"avg_result_count"`
	LatencyP50    time.Duration `json:"latency_p50"`
	LatencyP95    time.Duration `json:"latency_p95"`
	LatencyAvg    time.Duration `json:"latency_avg"`
	
	// 质量指标
	RelevanceScore  float64 `json:"relevance_score"`
	DiversityScore  float64 `json:"diversity_score"`
	FreshnessScore  float64 `json:"freshness_score"`
	
	// 稳定性指标
	QualityVariance  float64 `json:"quality_variance"`
	ConsecutiveFails int     `json:"consecutive_fails"`
	
	// 查询类型分布
	QueryTypeDistribution map[QueryType]int `json:"query_type_distribution"`
	
	// 时间窗口
	WindowSize   int       `json:"window_size"`
	WindowStart  time.Time `json:"window_start"`
	TotalRecords int       `json:"total_records"`
}

// NewQualityTracker 创建质量追踪器
func NewQualityTracker() *QualityTracker {
	return &QualityTracker{
		windows: make(map[string]*SlidingWindow),
	}
}

// NewSlidingWindow 创建滑动窗口
func NewSlidingWindow(size int) *SlidingWindow {
	return &SlidingWindow{
		Size:    size,
		Records: make([]*SearchRecord, size),
		Index:   0,
		Count:   0,
	}
}

// Record 记录搜索结果
func (qt *QualityTracker) Record(engineName string, record *SearchRecord) {
	qt.mu.Lock()
	window, ok := qt.windows[engineName]
	if !ok {
		window = NewSlidingWindow(100) // 保留最近100条记录
		qt.windows[engineName] = window
	}
	qt.mu.Unlock()
	
	window.Add(record)
}

// Add 添加记录到窗口
func (sw *SlidingWindow) Add(record *SearchRecord) {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	
	sw.Records[sw.Index] = record
	sw.Index = (sw.Index + 1) % sw.Size
	if sw.Count < sw.Size {
		sw.Count++
	}
}

// GetMetrics 获取引擎质量指标
func (qt *QualityTracker) GetMetrics(engineName string) *QualityMetrics {
	qt.mu.RLock()
	window, ok := qt.windows[engineName]
	qt.mu.RUnlock()
	
	if !ok {
		return &QualityMetrics{
			SuccessRate:    0.8, // 默认值
			RelevanceScore: 0.7,
		}
	}
	
	return window.CalculateMetrics()
}

// CalculateMetrics 计算指标
func (sw *SlidingWindow) CalculateMetrics() *QualityMetrics {
	sw.mu.RLock()
	defer sw.mu.RUnlock()
	
	metrics := &QualityMetrics{
		QueryTypeDistribution: make(map[QueryType]int),
		WindowSize:            sw.Size,
		TotalRecords:          sw.Count,
	}
	
	if sw.Count == 0 {
		metrics.SuccessRate = 0.8
		metrics.RelevanceScore = 0.7
		return metrics
	}
	
	var successCount int
	var totalResultCount int
	var latencies []time.Duration
	var consecutiveFails int
	var maxConsecutiveFails int
	
	// 收集数据
	for i := 0; i < sw.Count; i++ {
		record := sw.Records[i]
		if record == nil {
			continue
		}
		
		if record.Success {
			successCount++
			consecutiveFails = 0
		} else {
			consecutiveFails++
			if consecutiveFails > maxConsecutiveFails {
				maxConsecutiveFails = consecutiveFails
			}
		}
		
		totalResultCount += record.ResultCount
		latencies = append(latencies, record.Latency)
		metrics.QueryTypeDistribution[record.QueryType]++
		
		if metrics.WindowStart.IsZero() || record.Timestamp.Before(metrics.WindowStart) {
			metrics.WindowStart = record.Timestamp
		}
	}
	
	// 计算成功率
	metrics.SuccessRate = float64(successCount) / float64(sw.Count)
	
	// 计算平均结果数
	if successCount > 0 {
		metrics.AvgResultCount = float64(totalResultCount) / float64(successCount)
	}
	
	// 计算延迟指标
	if len(latencies) > 0 {
		metrics.LatencyP50 = percentile(latencies, 0.50)
		metrics.LatencyP95 = percentile(latencies, 0.95)
		metrics.LatencyAvg = average(latencies)
	}
	
	metrics.ConsecutiveFails = maxConsecutiveFails
	
	// 计算相关性评分 (基于结果数和成功率的综合评估)
	resultScore := min(metrics.AvgResultCount/10.0, 1.0)
	metrics.RelevanceScore = metrics.SuccessRate*0.6 + resultScore*0.4
	
	// 计算质量波动
	metrics.QualityVariance = sw.calculateVariance()
	
	return metrics
}

// calculateVariance 计算质量波动
func (sw *SlidingWindow) calculateVariance() float64 {
	if sw.Count < 2 {
		return 0
	}
	
	var successRates []float64
	windowSize := 10
	
	for i := 0; i <= sw.Count-windowSize && i < sw.Count; i += windowSize {
		successCount := 0
		for j := i; j < i+windowSize && j < sw.Count; j++ {
			if sw.Records[j] != nil && sw.Records[j].Success {
				successCount++
			}
		}
		successRates = append(successRates, float64(successCount)/float64(windowSize))
	}
	
	if len(successRates) < 2 {
		return 0
	}
	
	// 计算方差
	var sum, sumSq float64
	for _, r := range successRates {
		sum += r
		sumSq += r * r
	}
	mean := sum / float64(len(successRates))
	variance := sumSq/float64(len(successRates)) - mean*mean
	
	return variance
}

// GetSuccessRate 获取成功率
func (qt *QualityTracker) GetSuccessRate(engineName string) float64 {
	metrics := qt.GetMetrics(engineName)
	return metrics.SuccessRate
}

// GetLatencyP95 获取 P95 延迟
func (qt *QualityTracker) GetLatencyP95(engineName string) time.Duration {
	metrics := qt.GetMetrics(engineName)
	return metrics.LatencyP95
}

// GetConsecutiveFails 获取连续失败次数
func (qt *QualityTracker) GetConsecutiveFails(engineName string) int {
	qt.mu.RLock()
	window, ok := qt.windows[engineName]
	qt.mu.RUnlock()
	
	if !ok {
		return 0
	}
	
	window.mu.RLock()
	defer window.mu.RUnlock()
	
	// 从最新记录向前计算连续失败
	consecutiveFails := 0
	idx := (window.Index - 1 + window.Size) % window.Size
	
	for i := 0; i < window.Count; i++ {
		record := window.Records[idx]
		if record == nil || record.Success {
			break
		}
		consecutiveFails++
		idx = (idx - 1 + window.Size) % window.Size
	}
	
	return consecutiveFails
}

// RecordFeedback 记录用户反馈
func (qt *QualityTracker) RecordFeedback(engineName string, clicked bool, clickedIndex int, dwellTime float64) {
	qt.mu.RLock()
	window, ok := qt.windows[engineName]
	qt.mu.RUnlock()
	
	if !ok || window.Count == 0 {
		return
	}
	
	window.mu.Lock()
	defer window.mu.Unlock()
	
	// 更新最近一条记录的反馈
	idx := (window.Index - 1 + window.Size) % window.Size
	if window.Records[idx] != nil {
		window.Records[idx].UserClicked = clicked
		window.Records[idx].ClickedIndex = clickedIndex
		window.Records[idx].DwellTime = dwellTime
	}
}

// percentile 计算百分位数
func percentile(values []time.Duration, p float64) time.Duration {
	if len(values) == 0 {
		return 0
	}
	
	// 简单排序
	sorted := make([]time.Duration, len(values))
	copy(sorted, values)
	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i] > sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	
	idx := int(float64(len(sorted)-1) * p)
	return sorted[idx]
}

// average 计算平均值
func average(values []time.Duration) time.Duration {
	if len(values) == 0 {
		return 0
	}
	
	var sum time.Duration
	for _, v := range values {
		sum += v
	}
	return sum / time.Duration(len(values))
}

// min 返回较小值
func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
