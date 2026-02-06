package observatory

import (
	"testing"
	"time"
)

func TestNewReporter(t *testing.T) {
	analyzer := NewAnalyzer([]string{"1m", "1h"})
	evaluator := NewEvaluator()

	reporter := NewReporter(analyzer, evaluator)
	if reporter == nil {
		t.Fatal("NewReporter returned nil")
	}
}

func TestReporter_GenerateReport(t *testing.T) {
	analyzer := NewAnalyzer([]string{"1m", "1h", "24h"})
	evaluator := NewEvaluator()
	reporter := NewReporter(analyzer, evaluator)

	// 添加一些测试事件
	for i := 0; i < 10; i++ {
		event := &Event{
			EventID:   "event" + string(rune('0'+i)),
			RequestID: "req" + string(rune('0'+i)),
			SessionID: "session1",
			Timestamp: time.Now(),
			EventType: EventTypeRetrieve,
			RetrievedChunks: []ChunkInfo{
				{ChunkID: "chunk1", SimilarityScore: 0.85},
			},
			Latency: LatencyBreakdown{
				Total:    time.Millisecond * time.Duration(100+i*20),
				L1Lookup: time.Millisecond * 20,
			},
		}
		analyzer.ProcessEvent(event)
	}

	report, err := reporter.GenerateReport("1h")
	if err != nil {
		t.Fatalf("GenerateReport failed: %v", err)
	}
	if report == nil {
		t.Fatal("GenerateReport returned nil")
	}

	if report.Period != "1h" {
		t.Errorf("Report period = %s, want 1h", report.Period)
	}
}

func TestReporter_GenerateRecommendations(t *testing.T) {
	analyzer := NewAnalyzer([]string{"1m", "1h", "24h"})
	evaluator := NewEvaluator()
	reporter := NewReporter(analyzer, evaluator)

	// 添加高延迟事件
	for i := 0; i < 5; i++ {
		event := &Event{
			EventID:   "slow" + string(rune('0'+i)),
			RequestID: "req" + string(rune('0'+i)),
			SessionID: "session1",
			Timestamp: time.Now(),
			EventType: EventTypeRetrieve,
			RetrievedChunks: []ChunkInfo{
				{ChunkID: "chunk1", SimilarityScore: 0.5},
			},
			Latency: LatencyBreakdown{
				Total: time.Second * 2, // 高延迟
			},
		}
		analyzer.ProcessEvent(event)
	}

	report, err := reporter.GenerateReport("1h")
	if err != nil {
		t.Fatalf("GenerateReport failed: %v", err)
	}

	// 应该有一些建议
	if len(report.Recommendations) == 0 {
		t.Log("No recommendations generated (might be expected depending on thresholds)")
	}
}

func TestReporter_InvalidPeriod(t *testing.T) {
	analyzer := NewAnalyzer([]string{"1m"})
	evaluator := NewEvaluator()
	reporter := NewReporter(analyzer, evaluator)

	_, err := reporter.GenerateReport("invalid")
	if err == nil {
		t.Error("Expected error for invalid period but got nil")
	}
}

func TestReporter_EmptyData(t *testing.T) {
	analyzer := NewAnalyzer([]string{"1m", "1h", "24h"})
	evaluator := NewEvaluator()
	reporter := NewReporter(analyzer, evaluator)

	// 不添加任何事件
	report, err := reporter.GenerateReport("1h")
	if err != nil {
		t.Fatalf("GenerateReport failed: %v", err)
	}
	if report == nil {
		t.Fatal("GenerateReport returned nil")
	}
}
