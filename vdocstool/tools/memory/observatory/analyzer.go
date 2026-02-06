// Package observatory 指标分析器
package observatory

import (
	"sort"
	"sync"
	"time"
)

// Analyzer 指标分析器
type Analyzer struct {
	// 滑动窗口
	windows map[string]*MetricsWindow

	// 实时指标
	realtimeMetrics *Metrics

	// 统计数据
	totalRequests   int64
	successRequests int64
	failedRequests  int64

	// 延迟数据
	latencies []time.Duration

	// 控制
	stopCh chan struct{}

	mu sync.RWMutex
}

// MetricsWindow 指标窗口
type MetricsWindow struct {
	Name       string
	Duration   time.Duration
	Events     []*Event
	StartTime  time.Time
	Stats      *WindowStats
	MaxEvents  int

	mu sync.RWMutex
}

// WindowStats 窗口统计
type WindowStats struct {
	TotalRequests   int64
	SuccessRequests int64
	FailedRequests  int64

	LatencyP50 time.Duration
	LatencyP95 time.Duration
	LatencyP99 time.Duration
	LatencyAvg time.Duration

	L1Hits       int64
	L2Hits       int64
	L3Hits       int64
	CacheHitRate float64

	AvgSimilarityScore float64
	AvgRetrievedCount  float64
	AvgTokenUsage      float64

	AvgUtilizationRate float64
	WasteRate          float64
}

// NewAnalyzer 创建分析器
func NewAnalyzer(windowSizes []string) *Analyzer {
	a := &Analyzer{
		windows:         make(map[string]*MetricsWindow),
		realtimeMetrics: &Metrics{},
		stopCh:          make(chan struct{}),
	}

	// 创建窗口
	windowConfigs := map[string]struct {
		duration  time.Duration
		maxEvents int
	}{
		"1m":  {time.Minute, 1000},
		"5m":  {5 * time.Minute, 5000},
		"1h":  {time.Hour, 10000},
		"24h": {24 * time.Hour, 50000},
	}

	for _, size := range windowSizes {
		if cfg, ok := windowConfigs[size]; ok {
			a.windows[size] = &MetricsWindow{
				Name:      size,
				Duration:  cfg.duration,
				MaxEvents: cfg.maxEvents,
				Events:    make([]*Event, 0, cfg.maxEvents),
				StartTime: time.Now(),
			}
		}
	}

	return a
}

// ProcessEvent 处理事件
func (a *Analyzer) ProcessEvent(event *Event) {
	a.mu.Lock()
	a.totalRequests++

	if event.Error == "" {
		a.successRequests++
	} else {
		a.failedRequests++
	}

	a.latencies = append(a.latencies, event.Latency.Total)
	if len(a.latencies) > 10000 {
		a.latencies = a.latencies[len(a.latencies)-10000:]
	}
	a.mu.Unlock()

	// 添加到各个窗口
	for _, window := range a.windows {
		window.AddEvent(event)
	}

	// 更新实时指标
	a.updateRealtimeMetrics(event)
}

// AddEvent 添加事件到窗口
func (w *MetricsWindow) AddEvent(event *Event) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// 清理过期事件
	cutoff := time.Now().Add(-w.Duration)
	validEvents := make([]*Event, 0, len(w.Events))
	for _, e := range w.Events {
		if e.Timestamp.After(cutoff) {
			validEvents = append(validEvents, e)
		}
	}
	w.Events = validEvents

	// 添加新事件
	if len(w.Events) < w.MaxEvents {
		w.Events = append(w.Events, event)
	}
}

// updateRealtimeMetrics 更新实时指标
func (a *Analyzer) updateRealtimeMetrics(event *Event) {
	a.mu.Lock()
	defer a.mu.Unlock()

	metrics := a.realtimeMetrics

	// 更新延迟
	if len(a.latencies) > 0 {
		metrics.Latency.P50 = percentile(a.latencies, 0.50)
		metrics.Latency.P95 = percentile(a.latencies, 0.95)
		metrics.Latency.P99 = percentile(a.latencies, 0.99)
		metrics.Latency.Avg = average(a.latencies)
	}

	// 更新质量
	if a.totalRequests > 0 {
		metrics.Quality.SuccessRate = float64(a.successRequests) / float64(a.totalRequests)
		metrics.Quality.ErrorRate = float64(a.failedRequests) / float64(a.totalRequests)
	}

	// 更新时间戳
	metrics.Timestamp = time.Now()
}

// GetRealtimeMetrics 获取实时指标
func (a *Analyzer) GetRealtimeMetrics() *Metrics {
	a.mu.RLock()
	defer a.mu.RUnlock()

	// 复制指标
	metrics := *a.realtimeMetrics
	return &metrics
}

// GetWindowStats 获取窗口统计
func (a *Analyzer) GetWindowStats(windowName string) *WindowStats {
	window, ok := a.windows[windowName]
	if !ok {
		return nil
	}

	return window.CalculateStats()
}

// CalculateStats 计算窗口统计
func (w *MetricsWindow) CalculateStats() *WindowStats {
	w.mu.RLock()
	defer w.mu.RUnlock()

	stats := &WindowStats{}

	if len(w.Events) == 0 {
		return stats
	}

	var latencies []time.Duration
	var similarityScores []float64
	var retrievedCounts []int
	var tokenUsages []int

	for _, event := range w.Events {
		stats.TotalRequests++
		if event.Error == "" {
			stats.SuccessRequests++
		} else {
			stats.FailedRequests++
		}

		latencies = append(latencies, event.Latency.Total)

		// 统计各层命中
		for _, chunk := range event.RetrievedChunks {
			switch chunk.Tier {
			case "L1":
				stats.L1Hits++
			case "L2":
				stats.L2Hits++
			case "L3":
				stats.L3Hits++
			}
			similarityScores = append(similarityScores, chunk.SimilarityScore)
		}

		retrievedCounts = append(retrievedCounts, len(event.RetrievedChunks))
		tokenUsages = append(tokenUsages, event.TotalTokens)
	}

	// 计算延迟
	if len(latencies) > 0 {
		stats.LatencyP50 = percentile(latencies, 0.50)
		stats.LatencyP95 = percentile(latencies, 0.95)
		stats.LatencyP99 = percentile(latencies, 0.99)
		stats.LatencyAvg = average(latencies)
	}

	// 计算缓存命中率
	totalHits := stats.L1Hits + stats.L2Hits + stats.L3Hits
	if totalHits > 0 {
		stats.CacheHitRate = float64(stats.L1Hits) / float64(totalHits)
	}

	// 计算平均相似度
	if len(similarityScores) > 0 {
		var sum float64
		for _, s := range similarityScores {
			sum += s
		}
		stats.AvgSimilarityScore = sum / float64(len(similarityScores))
	}

	// 计算平均召回数
	if len(retrievedCounts) > 0 {
		var sum int
		for _, c := range retrievedCounts {
			sum += c
		}
		stats.AvgRetrievedCount = float64(sum) / float64(len(retrievedCounts))
	}

	// 计算平均 token 使用
	if len(tokenUsages) > 0 {
		var sum int
		for _, t := range tokenUsages {
			sum += t
		}
		stats.AvgTokenUsage = float64(sum) / float64(len(tokenUsages))
	}

	return stats
}

// StartBackgroundTasks 启动后台任务
func (a *Analyzer) StartBackgroundTasks() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			a.cleanupOldData()
		case <-a.stopCh:
			return
		}
	}
}

// Stop 停止分析器
func (a *Analyzer) Stop() {
	close(a.stopCh)
}

// cleanupOldData 清理旧数据
func (a *Analyzer) cleanupOldData() {
	for _, window := range a.windows {
		window.cleanup()
	}
}

// cleanup 清理窗口中的过期数据
func (w *MetricsWindow) cleanup() {
	w.mu.Lock()
	defer w.mu.Unlock()

	cutoff := time.Now().Add(-w.Duration)
	validEvents := make([]*Event, 0, len(w.Events))
	for _, e := range w.Events {
		if e.Timestamp.After(cutoff) {
			validEvents = append(validEvents, e)
		}
	}
	w.Events = validEvents
}

// GetTrends 获取趋势数据
func (a *Analyzer) GetTrends(period time.Duration) *ReportTrends {
	trends := &ReportTrends{}

	// 获取对应窗口
	var windowName string
	switch {
	case period <= time.Hour:
		windowName = "1h"
	case period <= 24*time.Hour:
		windowName = "24h"
	default:
		windowName = "24h"
	}

	window, ok := a.windows[windowName]
	if !ok {
		return trends
	}

	window.mu.RLock()
	defer window.mu.RUnlock()

	// 按时间分组计算趋势
	bucketDuration := period / 24
	buckets := make(map[int64]struct {
		latencies []time.Duration
		qualities []float64
		count     int
	})

	for _, event := range window.Events {
		bucketKey := event.Timestamp.Unix() / int64(bucketDuration.Seconds())
		bucket := buckets[bucketKey]
		bucket.latencies = append(bucket.latencies, event.Latency.Total)
		if event.Error == "" {
			bucket.qualities = append(bucket.qualities, 1.0)
		} else {
			bucket.qualities = append(bucket.qualities, 0.0)
		}
		bucket.count++
		buckets[bucketKey] = bucket
	}

	// 转换为趋势点
	var keys []int64
	for k := range buckets {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return keys[i] < keys[j]
	})

	for _, key := range keys {
		bucket := buckets[key]
		timestamp := time.Unix(key*int64(bucketDuration.Seconds()), 0)

		// 延迟趋势
		if len(bucket.latencies) > 0 {
			trends.LatencyTrend = append(trends.LatencyTrend, TrendPoint{
				Timestamp: timestamp,
				Value:     float64(average(bucket.latencies).Milliseconds()),
			})
		}

		// 质量趋势
		if len(bucket.qualities) > 0 {
			var sum float64
			for _, q := range bucket.qualities {
				sum += q
			}
			trends.QualityTrend = append(trends.QualityTrend, TrendPoint{
				Timestamp: timestamp,
				Value:     sum / float64(len(bucket.qualities)),
			})
		}

		// 吞吐量趋势
		trends.ThroughputTrend = append(trends.ThroughputTrend, TrendPoint{
			Timestamp: timestamp,
			Value:     float64(bucket.count) / bucketDuration.Seconds(),
		})
	}

	return trends
}

// percentile 计算百分位数
func percentile(values []time.Duration, p float64) time.Duration {
	if len(values) == 0 {
		return 0
	}

	sorted := make([]time.Duration, len(values))
	copy(sorted, values)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i] < sorted[j]
	})

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
