package observatory

import (
	"testing"
	"time"
)

func TestNewEvaluator(t *testing.T) {
	evaluator := NewEvaluator()
	if evaluator == nil {
		t.Fatal("NewEvaluator returned nil")
	}
}

func TestEvaluator_Evaluate_HighLatency(t *testing.T) {
	evaluator := NewEvaluator()

	event := &Event{
		EventID:   "event1",
		RequestID: "req1",
		SessionID: "session1",
		Timestamp: time.Now(),
		EventType: EventTypeRetrieve,
		Latency: LatencyBreakdown{
			Total: time.Second * 3, // 超过2秒触发critical
		},
	}

	result := evaluator.Evaluate(event)
	if result == nil {
		t.Fatal("Evaluate returned nil")
	}

	if len(result.Issues) == 0 {
		t.Error("Expected high latency issues but got none")
	}

	hasHighLatencyIssue := false
	for _, issue := range result.Issues {
		if issue.Type == IssueTypeHighLatency {
			hasHighLatencyIssue = true
			break
		}
	}
	if !hasHighLatencyIssue {
		t.Error("Expected IssueTypeHighLatency but not found")
	}
}

func TestEvaluator_Evaluate_EmptyResult(t *testing.T) {
	evaluator := NewEvaluator()

	event := &Event{
		EventID:         "event1",
		RequestID:       "req1",
		SessionID:       "session1",
		Timestamp:       time.Now(),
		EventType:       EventTypeRetrieve,
		RetrievedChunks: []ChunkInfo{}, // 空结果
		Latency: LatencyBreakdown{
			Total: time.Millisecond * 100,
		},
	}

	result := evaluator.Evaluate(event)
	if result == nil {
		t.Fatal("Evaluate returned nil")
	}

	hasEmptyResultIssue := false
	for _, issue := range result.Issues {
		if issue.Type == IssueTypeEmptyResult {
			hasEmptyResultIssue = true
			break
		}
	}
	if !hasEmptyResultIssue {
		t.Error("Expected IssueTypeEmptyResult but not found")
	}
}

func TestEvaluator_Evaluate_LowRelevance(t *testing.T) {
	evaluator := NewEvaluator()

	event := &Event{
		EventID:   "event1",
		RequestID: "req1",
		SessionID: "session1",
		Timestamp: time.Now(),
		EventType: EventTypeRetrieve,
		RetrievedChunks: []ChunkInfo{
			{ChunkID: "chunk1", SimilarityScore: 0.3}, // 低相似度
			{ChunkID: "chunk2", SimilarityScore: 0.2},
		},
		Latency: LatencyBreakdown{
			Total: time.Millisecond * 100,
		},
	}

	result := evaluator.Evaluate(event)
	if result == nil {
		t.Fatal("Evaluate returned nil")
	}

	hasLowRelevanceIssue := false
	for _, issue := range result.Issues {
		if issue.Type == IssueTypeLowRelevance {
			hasLowRelevanceIssue = true
			break
		}
	}
	if !hasLowRelevanceIssue {
		t.Error("Expected IssueTypeLowRelevance but not found")
	}
}

func TestEvaluator_Evaluate_NoIssues(t *testing.T) {
	evaluator := NewEvaluator()

	event := &Event{
		EventID:   "event1",
		RequestID: "req1",
		SessionID: "session1",
		Timestamp: time.Now(),
		EventType: EventTypeRetrieve,
		RetrievedChunks: []ChunkInfo{
			{ChunkID: "chunk1", Tier: "L1", SimilarityScore: 0.9}, // 从L1缓存命中
			{ChunkID: "chunk2", Tier: "L1", SimilarityScore: 0.85},
		},
		Latency: LatencyBreakdown{
			Total:    time.Millisecond * 100,
			L1Lookup: time.Millisecond * 50, // 有L1查询
		},
	}

	result := evaluator.Evaluate(event)
	if result == nil {
		t.Fatal("Evaluate returned nil")
	}

	// 检查是否有严重问题（允许一些轻微的警告）
	hasCriticalIssues := false
	for _, issue := range result.Issues {
		if issue.Severity == "critical" || issue.Severity == "error" {
			hasCriticalIssues = true
			t.Logf("Critical Issue: %s - %s", issue.Type, issue.Message)
		}
	}
	if hasCriticalIssues {
		t.Error("Expected no critical issues")
	}
}

func TestEvaluator_Evaluate_RelevanceScore(t *testing.T) {
	evaluator := NewEvaluator()

	// 高质量事件
	goodEvent := &Event{
		EventID:   "good",
		RequestID: "req1",
		Timestamp: time.Now(),
		EventType: EventTypeRetrieve,
		RetrievedChunks: []ChunkInfo{
			{ChunkID: "chunk1", SimilarityScore: 0.95},
		},
		Latency: LatencyBreakdown{
			Total: time.Millisecond * 50,
		},
	}

	// 低质量事件
	badEvent := &Event{
		EventID:   "bad",
		RequestID: "req2",
		Timestamp: time.Now(),
		EventType: EventTypeRetrieve,
		RetrievedChunks: []ChunkInfo{
			{ChunkID: "chunk1", SimilarityScore: 0.3},
		},
		Latency: LatencyBreakdown{
			Total: time.Second * 3,
		},
	}

	goodResult := evaluator.Evaluate(goodEvent)
	badResult := evaluator.Evaluate(badEvent)

	if goodResult.RelevanceScore <= badResult.RelevanceScore {
		t.Errorf("Good event score (%v) should be higher than bad event score (%v)",
			goodResult.RelevanceScore, badResult.RelevanceScore)
	}
}

func TestEvaluator_EvaluateBatch(t *testing.T) {
	evaluator := NewEvaluator()

	events := []*Event{
		{
			EventID:   "event1",
			RequestID: "req1",
			Timestamp: time.Now(),
			EventType: EventTypeStore,
			Latency:   LatencyBreakdown{Total: time.Millisecond * 100},
		},
		{
			EventID:   "event2",
			RequestID: "req2",
			Timestamp: time.Now(),
			EventType: EventTypeRetrieve,
			Latency:   LatencyBreakdown{Total: time.Second * 3}, // 高延迟
		},
	}

	results := evaluator.EvaluateBatch(events)
	if len(results) != 2 {
		t.Errorf("EvaluateBatch returned %d results, want 2", len(results))
	}
}
