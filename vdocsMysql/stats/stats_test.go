package stats

import (
	"context"
	"testing"
	"time"
)

func TestTokenStatsCollector(t *testing.T) {
	collector := NewTokenStatsCollector()

	// Test RecordUsage
	t.Run("RecordUsage", func(t *testing.T) {
		collector.RecordUsage("gpt-4", "master_agent", 100, 50, 0.003)
		collector.RecordUsage("gpt-4", "master_agent", 200, 100, 0.006)
		collector.RecordUsage("gpt-3.5", "code_search", 500, 200, 0.001)

		summary := collector.Summary()

		if summary.TotalPromptTokens != 800 {
			t.Errorf("Expected 800 prompt tokens, got %d", summary.TotalPromptTokens)
		}
		if summary.TotalCompletionTokens != 350 {
			t.Errorf("Expected 350 completion tokens, got %d", summary.TotalCompletionTokens)
		}
		if summary.TotalTokens != 1150 {
			t.Errorf("Expected 1150 total tokens, got %d", summary.TotalTokens)
		}
		if summary.TotalCost != 0.01 {
			t.Errorf("Expected 0.01 cost, got %f", summary.TotalCost)
		}
	})

	// Test model stats
	t.Run("ByModel", func(t *testing.T) {
		summary := collector.Summary()

		if len(summary.ByModel) != 2 {
			t.Errorf("Expected 2 models, got %d", len(summary.ByModel))
		}

		var gpt4Stats *ModelTokenStats
		for _, m := range summary.ByModel {
			if m.Model == "gpt-4" {
				gpt4Stats = m
				break
			}
		}

		if gpt4Stats == nil {
			t.Fatal("Expected gpt-4 stats")
		}
		if gpt4Stats.RequestCount != 2 {
			t.Errorf("Expected 2 requests for gpt-4, got %d", gpt4Stats.RequestCount)
		}
	})

	// Test agent stats
	t.Run("ByAgent", func(t *testing.T) {
		summary := collector.Summary()

		if len(summary.ByAgent) != 2 {
			t.Errorf("Expected 2 agents, got %d", len(summary.ByAgent))
		}

		var masterStats *AgentTokenStats
		for _, a := range summary.ByAgent {
			if a.Agent == "master_agent" {
				masterStats = a
				break
			}
		}

		if masterStats == nil {
			t.Fatal("Expected master_agent stats")
		}
		if masterStats.TotalTokens != 450 {
			t.Errorf("Expected 450 tokens for master_agent, got %d", masterStats.TotalTokens)
		}
	})

	// Test daily budget
	t.Run("DailyBudget", func(t *testing.T) {
		collector.SetDailyBudget(10000)

		remaining := collector.RemainingBudget()
		if remaining != 10000-1150 {
			t.Errorf("Expected %d remaining, got %d", 10000-1150, remaining)
		}

		summary := collector.Summary()
		if summary.DailyBudget != 10000 {
			t.Errorf("Expected 10000 budget, got %d", summary.DailyBudget)
		}
		if summary.DailyUsed != 1150 {
			t.Errorf("Expected 1150 used, got %d", summary.DailyUsed)
		}
	})
}

func TestCacheStatsCollector(t *testing.T) {
	collector := NewCacheStatsCollector()

	// Test RecordHit and RecordMiss
	t.Run("HitAndMiss", func(t *testing.T) {
		for i := 0; i < 80; i++ {
			collector.RecordHit("l1_cache")
		}
		for i := 0; i < 20; i++ {
			collector.RecordMiss("l1_cache")
		}

		hitRate := collector.GetHitRate("l1_cache")
		if hitRate != 0.8 {
			t.Errorf("Expected 0.8 hit rate, got %f", hitRate)
		}
	})

	// Test multiple caches
	t.Run("MultipleCaches", func(t *testing.T) {
		collector.RecordHit("l2_cache")
		collector.RecordMiss("l2_cache")
		collector.RecordMiss("l2_cache")

		l2Rate := collector.GetHitRate("l2_cache")
		expectedRate := 1.0 / 3.0
		if l2Rate < expectedRate-0.01 || l2Rate > expectedRate+0.01 {
			t.Errorf("Expected ~0.33 hit rate, got %f", l2Rate)
		}
	})

	// Test RecordEviction
	t.Run("Eviction", func(t *testing.T) {
		collector.RecordEviction("l1_cache")
		collector.RecordEviction("l1_cache")

		summary := collector.Summary()
		var l1Metrics *CacheMetrics
		for _, m := range summary.Caches {
			if m.Name == "l1_cache" {
				l1Metrics = m
				break
			}
		}

		if l1Metrics == nil {
			t.Fatal("Expected l1_cache metrics")
		}
		if l1Metrics.Evictions != 2 {
			t.Errorf("Expected 2 evictions, got %d", l1Metrics.Evictions)
		}
	})

	// Test UpdateSize
	t.Run("UpdateSize", func(t *testing.T) {
		collector.UpdateSize("l1_cache", 1000)

		summary := collector.Summary()
		var l1Metrics *CacheMetrics
		for _, m := range summary.Caches {
			if m.Name == "l1_cache" {
				l1Metrics = m
				break
			}
		}

		if l1Metrics.Size != 1000 {
			t.Errorf("Expected size 1000, got %d", l1Metrics.Size)
		}
	})

	// Test Summary
	t.Run("Summary", func(t *testing.T) {
		summary := collector.Summary()

		if summary.TotalHits < 80 {
			t.Errorf("Expected at least 80 total hits, got %d", summary.TotalHits)
		}
		if summary.TotalMisses < 20 {
			t.Errorf("Expected at least 20 total misses, got %d", summary.TotalMisses)
		}
		if summary.OverallHitRate < 0.7 || summary.OverallHitRate > 1.0 {
			t.Errorf("Unexpected overall hit rate: %f", summary.OverallHitRate)
		}
	})
}

func TestRequestStatsCollector(t *testing.T) {
	collector := NewRequestStatsCollector()

	// Test RecordRequest
	t.Run("RecordRequest", func(t *testing.T) {
		collector.RecordRequest("/api/query", 100*time.Millisecond, true)
		collector.RecordRequest("/api/query", 200*time.Millisecond, true)
		collector.RecordRequest("/api/query", 300*time.Millisecond, false)
		collector.RecordRequest("/api/search", 50*time.Millisecond, true)

		summary := collector.Summary()

		if summary.TotalRequests != 4 {
			t.Errorf("Expected 4 total requests, got %d", summary.TotalRequests)
		}
		if summary.SuccessRequests != 3 {
			t.Errorf("Expected 3 success requests, got %d", summary.SuccessRequests)
		}
		if summary.FailedRequests != 1 {
			t.Errorf("Expected 1 failed request, got %d", summary.FailedRequests)
		}
		if summary.SuccessRate != 0.75 {
			t.Errorf("Expected 0.75 success rate, got %f", summary.SuccessRate)
		}
	})

	// Test latency stats
	t.Run("LatencyStats", func(t *testing.T) {
		summary := collector.Summary()

		if summary.MaxLatency != 300*time.Millisecond {
			t.Errorf("Expected 300ms max latency, got %v", summary.MaxLatency)
		}
		if summary.MinLatency != 50*time.Millisecond {
			t.Errorf("Expected 50ms min latency, got %v", summary.MinLatency)
		}

		expectedAvg := (100 + 200 + 300 + 50) * time.Millisecond / 4
		if summary.AverageLatency != expectedAvg {
			t.Errorf("Expected %v avg latency, got %v", expectedAvg, summary.AverageLatency)
		}
	})

	// Test by endpoint
	t.Run("ByEndpoint", func(t *testing.T) {
		summary := collector.Summary()

		if len(summary.ByEndpoint) != 2 {
			t.Errorf("Expected 2 endpoints, got %d", len(summary.ByEndpoint))
		}

		var queryStats *EndpointStats
		for _, e := range summary.ByEndpoint {
			if e.Endpoint == "/api/query" {
				queryStats = e
				break
			}
		}

		if queryStats == nil {
			t.Fatal("Expected /api/query stats")
		}
		if queryStats.Requests != 3 {
			t.Errorf("Expected 3 requests for /api/query, got %d", queryStats.Requests)
		}
		if queryStats.Successes != 2 {
			t.Errorf("Expected 2 successes for /api/query, got %d", queryStats.Successes)
		}
		if queryStats.MaxLatency != 300*time.Millisecond {
			t.Errorf("Expected 300ms max latency for /api/query, got %v", queryStats.MaxLatency)
		}
	})
}

func TestCollector(t *testing.T) {
	var loggedMessages []string
	logger := func(format string, args ...interface{}) {
		// Just accumulate for test
		loggedMessages = append(loggedMessages, format)
	}

	collector := NewCollector(&CollectorConfig{
		Enabled:        true,
		ReportInterval: 100 * time.Millisecond,
		Exporters:      []Exporter{NewLogExporter(logger)},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Record some stats
	collector.TokenStats().RecordUsage("gpt-4", "agent", 100, 50, 0.003)
	collector.CacheStats().RecordHit("cache")
	collector.RequestStats().RecordRequest("/api", 100*time.Millisecond, true)

	// Start collector
	collector.Start(ctx)

	// Wait for at least one export
	time.Sleep(150 * time.Millisecond)

	// Stop collector
	collector.Stop()

	// Check logs were generated
	if len(loggedMessages) == 0 {
		t.Error("Expected log messages from exporter")
	}
}

func TestJSONExporter(t *testing.T) {
	var exported []byte
	exporter := NewJSONExporter(func(data []byte) {
		exported = data
	})

	summary := &StatsSummary{
		Timestamp: time.Now(),
		Token: &TokenStatsSummary{
			TotalTokens: 1000,
		},
		Cache: &CacheStatsSummary{
			TotalHits: 100,
		},
		Request: &RequestStatsSummary{
			TotalRequests: 50,
		},
	}

	exporter.Export(summary)

	if len(exported) == 0 {
		t.Error("Expected JSON export")
	}
}

func TestPrometheusExporter(t *testing.T) {
	exporter := NewPrometheusExporter(9999)

	summary := &StatsSummary{
		Timestamp: time.Now(),
		Token: &TokenStatsSummary{
			TotalPromptTokens:     500,
			TotalCompletionTokens: 300,
			TotalCost:             0.01,
		},
		Cache: &CacheStatsSummary{
			TotalHits:      80,
			TotalMisses:    20,
			OverallHitRate: 0.8,
		},
		Request: &RequestStatsSummary{
			TotalRequests:   100,
			SuccessRequests: 95,
			FailedRequests:  5,
			AverageLatency:  100 * time.Millisecond,
			MaxLatency:      500 * time.Millisecond,
		},
	}

	exporter.Export(summary)

	// Note: We don't start the server in unit tests
	// The exporter stores the summary and handleMetrics would format it
}

func TestCollectorSummary(t *testing.T) {
	collector := NewCollector(&CollectorConfig{
		Enabled: true,
	})

	// Add some data
	collector.TokenStats().RecordUsage("model", "agent", 100, 50, 0.01)
	collector.CacheStats().RecordHit("cache")
	collector.CacheStats().RecordMiss("cache")
	collector.RequestStats().RecordRequest("/api", 100*time.Millisecond, true)

	summary := collector.Summary()

	if summary.Token == nil {
		t.Error("Token summary should not be nil")
	}
	if summary.Cache == nil {
		t.Error("Cache summary should not be nil")
	}
	if summary.Request == nil {
		t.Error("Request summary should not be nil")
	}
	if summary.Timestamp.IsZero() {
		t.Error("Timestamp should be set")
	}
}

func BenchmarkTokenStatsCollector(b *testing.B) {
	collector := NewTokenStatsCollector()

	b.Run("RecordUsage", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			collector.RecordUsage("gpt-4", "agent", 100, 50, 0.003)
		}
	})

	b.Run("Summary", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			collector.Summary()
		}
	})
}

func BenchmarkCacheStatsCollector(b *testing.B) {
	collector := NewCacheStatsCollector()

	b.Run("RecordHit", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			collector.RecordHit("cache")
		}
	})

	b.Run("GetHitRate", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			collector.GetHitRate("cache")
		}
	})
}

func BenchmarkRequestStatsCollector(b *testing.B) {
	collector := NewRequestStatsCollector()

	b.Run("RecordRequest", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			collector.RecordRequest("/api", 100*time.Millisecond, true)
		}
	})
}
