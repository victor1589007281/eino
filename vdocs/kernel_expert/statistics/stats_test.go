package statistics

import (
	"testing"
	"time"
)

func TestTokenCounter_Record(t *testing.T) {
	counter := NewTokenCounter()

	// 记录使用
	counter.Record("gpt-4-turbo", 1000, 500)
	counter.Record("gpt-4-turbo", 2000, 1000)
	counter.Record("qwen-turbo", 500, 200)

	// 验证统计
	stats := counter.GetStats("gpt-4-turbo")
	if stats == nil {
		t.Fatal("Expected stats for gpt-4-turbo")
	}

	if stats.InputTokens != 3000 {
		t.Errorf("Expected 3000 input tokens, got %d", stats.InputTokens)
	}
	if stats.OutputTokens != 1500 {
		t.Errorf("Expected 1500 output tokens, got %d", stats.OutputTokens)
	}
	if stats.RequestCount != 2 {
		t.Errorf("Expected 2 requests, got %d", stats.RequestCount)
	}

	// 验证成本计算
	expectedCost := (3000.0/1000.0)*0.01 + (1500.0/1000.0)*0.03 // gpt-4-turbo pricing
	t.Logf("Expected cost %f, got %f", expectedCost, stats.TotalCost)
	// Use tolerance for floating point comparison
	if diff := stats.TotalCost - expectedCost; diff < -0.001 || diff > 0.001 {
		t.Errorf("Cost mismatch: expected %f, got %f", expectedCost, stats.TotalCost)
	}
}

func TestTokenCounter_GetTotals(t *testing.T) {
	counter := NewTokenCounter()

	counter.Record("gpt-4-turbo", 1000, 500)
	counter.Record("qwen-turbo", 2000, 1000)

	input, output, total := counter.GetTotalTokens()
	if input != 3000 {
		t.Errorf("Expected 3000 total input, got %d", input)
	}
	if output != 1500 {
		t.Errorf("Expected 1500 total output, got %d", output)
	}
	if total != 4500 {
		t.Errorf("Expected 4500 total, got %d", total)
	}
}

func TestTokenCounter_Reset(t *testing.T) {
	counter := NewTokenCounter()

	counter.Record("gpt-4-turbo", 1000, 500)
	counter.Reset()

	stats := counter.GetStats("gpt-4-turbo")
	if stats != nil {
		t.Error("Expected nil stats after reset")
	}

	_, _, total := counter.GetTotalTokens()
	if total != 0 {
		t.Error("Expected 0 total after reset")
	}
}

func TestCacheStatsCollector(t *testing.T) {
	collector := NewCacheStatsCollector()

	// 记录命中和未命中
	collector.RecordHit("L1", "search")
	collector.RecordHit("L1", "search")
	collector.RecordMiss("L1", "search")
	collector.RecordHit("L2", "function")
	collector.RecordMiss("L2", "function")

	// 验证命中率
	hitRate := collector.HitRate()
	expectedRate := 3.0 / 5.0
	if hitRate != expectedRate {
		t.Errorf("Expected hit rate %f, got %f", expectedRate, hitRate)
	}

	// 验证统计
	stats := collector.GetStats()
	if stats["total_hits"].(int64) != 3 {
		t.Errorf("Expected 3 total hits, got %d", stats["total_hits"])
	}
	if stats["total_misses"].(int64) != 2 {
		t.Errorf("Expected 2 total misses, got %d", stats["total_misses"])
	}
}

func TestPerformanceCollector_ResponseTime(t *testing.T) {
	collector := NewPerformanceCollector()

	// 记录响应时间
	durations := []time.Duration{
		100 * time.Millisecond,
		200 * time.Millisecond,
		150 * time.Millisecond,
		500 * time.Millisecond,
		50 * time.Millisecond,
	}

	for _, d := range durations {
		collector.RecordResponseTime(d)
	}

	stats := collector.GetLatencyStats()

	if stats["count"] != 5.0 {
		t.Errorf("Expected count 5, got %f", stats["count"])
	}

	expectedAvg := (0.1 + 0.2 + 0.15 + 0.5 + 0.05) / 5.0
	if abs(stats["avg"]-expectedAvg) > 0.001 {
		t.Errorf("Expected avg %f, got %f", expectedAvg, stats["avg"])
	}

	if stats["min"] != 0.05 {
		t.Errorf("Expected min 0.05, got %f", stats["min"])
	}

	if stats["max"] != 0.5 {
		t.Errorf("Expected max 0.5, got %f", stats["max"])
	}
}

func TestPerformanceCollector_ToolCalls(t *testing.T) {
	collector := NewPerformanceCollector()

	// 记录工具调用
	collector.RecordToolCall("grep", 100*time.Millisecond, true)
	collector.RecordToolCall("grep", 150*time.Millisecond, true)
	collector.RecordToolCall("grep", 200*time.Millisecond, false)
	collector.RecordToolCall("search", 300*time.Millisecond, true)

	// 验证grep统计
	grepStats := collector.toolCalls["grep"]
	if grepStats.CallCount != 3 {
		t.Errorf("Expected 3 calls, got %d", grepStats.CallCount)
	}
	if grepStats.SuccessCount != 2 {
		t.Errorf("Expected 2 successes, got %d", grepStats.SuccessCount)
	}
	if grepStats.ErrorCount != 1 {
		t.Errorf("Expected 1 error, got %d", grepStats.ErrorCount)
	}
}

func TestBusinessCollector(t *testing.T) {
	collector := NewBusinessCollector()

	// 记录查询
	collector.RecordQuery("concept", true, "user1")
	collector.RecordQuery("function", true, "user1")
	collector.RecordQuery("callchain", false, "user2")
	collector.RecordQuery("concept", true, "user3")

	// 验证成功率
	successRate := collector.SuccessRate()
	expectedRate := 3.0 / 4.0
	if successRate != expectedRate {
		t.Errorf("Expected success rate %f, got %f", expectedRate, successRate)
	}

	// 验证统计
	stats := collector.GetStats()
	if stats["query_count"].(int64) != 4 {
		t.Errorf("Expected 4 queries, got %d", stats["query_count"])
	}
	if stats["unique_users"].(int) != 3 {
		t.Errorf("Expected 3 unique users, got %d", stats["unique_users"])
	}

	// 验证意图分布
	intentDist := stats["intent_dist"].(map[string]int64)
	if intentDist["concept"] != 2 {
		t.Errorf("Expected 2 concept queries, got %d", intentDist["concept"])
	}
}

func TestStatsCollector_GenerateReport(t *testing.T) {
	collector := NewStatsCollector()

	// 记录一些数据
	collector.Token().Record("gpt-4-turbo", 1000, 500)
	collector.Cache().RecordHit("L1", "search")
	collector.Performance().RecordResponseTime(100 * time.Millisecond)
	collector.Business().RecordQuery("concept", true, "user1")

	// 生成报告
	report := collector.GenerateReport()

	if report == nil {
		t.Fatal("Expected report")
	}

	if report.Summary == nil {
		t.Error("Expected summary in report")
	}

	if report.Summary.TotalQueries != 1 {
		t.Errorf("Expected 1 query in summary, got %d", report.Summary.TotalQueries)
	}
}

func TestReport_ExportJSON(t *testing.T) {
	collector := NewStatsCollector()
	collector.Token().Record("gpt-4-turbo", 1000, 500)

	report := collector.GenerateReport()

	data, err := report.ExportJSON()
	if err != nil {
		t.Fatalf("ExportJSON failed: %v", err)
	}

	if len(data) == 0 {
		t.Error("Expected non-empty JSON")
	}
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func BenchmarkTokenCounter_Record(b *testing.B) {
	counter := NewTokenCounter()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		counter.Record("gpt-4-turbo", 1000, 500)
	}
}

func BenchmarkCacheStatsCollector_RecordHit(b *testing.B) {
	collector := NewCacheStatsCollector()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		collector.RecordHit("L1", "search")
	}
}
