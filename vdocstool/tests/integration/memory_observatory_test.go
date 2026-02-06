// Package integration 集成测试
package integration

import (
	"testing"
	"time"

	"github.com/cloudwego/eino/vdocstool/tools/memory/observatory"
)

// TestObservatorySystem_Integration 旁观者系统集成测试
func TestObservatorySystem_Integration(t *testing.T) {
	// 创建所有组件
	collector := observatory.NewCollector(1000, 1.0) // bufferSize, sampleRate

	analyzer := observatory.NewAnalyzer([]string{"1m", "5m", "1h"})

	evaluator := observatory.NewEvaluator()

	reporter := observatory.NewReporter(analyzer, evaluator)

	// 模拟一系列检索事件
	t.Run("SimulateRetrievalWorkload", func(t *testing.T) {
		// 模拟100个检索事件
		for i := 0; i < 100; i++ {
			event := generateMockEvent(i)

			// 收集事件
			collector.Collect(event)

			// 分析事件
			analyzer.ProcessEvent(event)

			// 评估事件
			evaluator.Evaluate(event)
		}

		// 验证收集器
		events := collector.GetEvents(200)
		if len(events) != 100 {
			t.Errorf("Expected 100 events, got %d", len(events))
		}

		// 验证分析器
		metrics := analyzer.GetRealtimeMetrics()
		if metrics == nil {
			t.Error("GetRealtimeMetrics returned nil")
		}

		t.Logf("Realtime Metrics collected successfully")
	})

	// 生成报告
	t.Run("GenerateReport", func(t *testing.T) {
		report, err := reporter.GenerateReport("1h")
		if err != nil {
			t.Fatalf("GenerateReport failed: %v", err)
		}

		t.Logf("Observatory Report:")
		t.Logf("  Generated At: %v", report.GeneratedAt)
		t.Logf("  Period: %s", report.Period)

		if len(report.TopIssues) > 0 {
			t.Logf("  Top Issues: %d", len(report.TopIssues))
		}

		if len(report.Recommendations) > 0 {
			t.Logf("  Recommendations: %d", len(report.Recommendations))
		}
	})

	// 测试窗口统计
	t.Run("WindowStats", func(t *testing.T) {
		windows := []string{"1m", "5m", "1h"}
		for _, w := range windows {
			stats := analyzer.GetWindowStats(w)
			if stats != nil {
				t.Logf("Window %s stats: %d requests", w, stats.TotalRequests)
			}
		}
	})
}

// generateMockEvent 生成模拟事件
func generateMockEvent(index int) *observatory.Event {
	success := index%10 != 0 // 90% 成功率

	latency := time.Millisecond * time.Duration(100+index%50)
	if !success {
		latency = time.Second * 2 // 失败的请求延迟更高
	}

	chunks := []observatory.ChunkInfo{}
	if success {
		chunks = []observatory.ChunkInfo{
			{
				ChunkID:         "chunk-" + string(rune('0'+index%10)),
				Tier:            "L1",
				SimilarityScore: 0.7 + float64(index%30)/100.0,
			},
		}
	}

	errorMsg := ""
	if !success {
		errorMsg = "timeout"
	}

	return &observatory.Event{
		EventID:   "event-" + string(rune('0'+index%10)) + string(rune('a'+index/10%26)),
		RequestID: "req-" + string(rune('0'+index%10)),
		SessionID: "session-" + string(rune('0'+index%5)),
		Timestamp: time.Now(),
		EventType: observatory.EventTypeRetrieve,
		Query:     "test query " + string(rune('0'+index%10)),
		RetrievedChunks: chunks,
		TotalTokens: 100 + index%50,
		Latency: observatory.LatencyBreakdown{
			Total:    latency,
			L1Lookup: time.Millisecond * 20,
			L2Search: time.Millisecond * 50,
		},
		Error:   errorMsg,
		TopicID: "topic-" + string(rune('0'+index%3)),
	}
}
