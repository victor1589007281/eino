package observatory

import (
	"testing"
	"time"
)

func TestNewAnalyzer(t *testing.T) {
	analyzer := NewAnalyzer([]string{"1m", "5m", "1h"})
	if analyzer == nil {
		t.Fatal("NewAnalyzer returned nil")
	}
}

func TestAnalyzer_ProcessEvent(t *testing.T) {
	analyzer := NewAnalyzer([]string{"1m"})

	event := &Event{
		EventID:   "event1",
		RequestID: "req1",
		SessionID: "session1",
		Timestamp: time.Now(),
		EventType: EventTypeStore,
		Latency: LatencyBreakdown{
			Total:    time.Millisecond * 50,
			L1Lookup: time.Millisecond * 10,
			L2Search: time.Millisecond * 20,
		},
		RetrievedChunks: []ChunkInfo{
			{ChunkID: "chunk1", Tier: "L1", SimilarityScore: 0.95},
		},
		TotalTokens: 100,
	}

	// 处理事件
	analyzer.ProcessEvent(event)

	// 获取窗口统计
	stats := analyzer.GetWindowStats("1m")
	if stats == nil {
		t.Fatal("GetWindowStats returned nil")
	}
	if stats.TotalRequests == 0 {
		t.Error("TotalRequests should be > 0 after ProcessEvent")
	}
}

func TestAnalyzer_GetRealtimeMetrics(t *testing.T) {
	analyzer := NewAnalyzer([]string{"1m"})

	// 添加一些事件
	for i := 0; i < 5; i++ {
		event := &Event{
			EventID:   "event" + string(rune('0'+i)),
			RequestID: "req" + string(rune('0'+i)),
			SessionID: "session1",
			Timestamp: time.Now(),
			EventType: EventTypeRetrieve,
			Latency: LatencyBreakdown{
				Total: time.Millisecond * time.Duration(100+i*20),
			},
			RetrievedChunks: []ChunkInfo{
				{ChunkID: "chunk1", Tier: "L1", SimilarityScore: 0.9},
			},
		}
		analyzer.ProcessEvent(event)
	}

	metrics := analyzer.GetRealtimeMetrics()
	if metrics == nil {
		t.Fatal("GetRealtimeMetrics returned nil")
	}
}

func TestAnalyzer_GetWindowStats(t *testing.T) {
	analyzer := NewAnalyzer([]string{"1m", "5m"})

	// 添加一些事件
	for i := 0; i < 10; i++ {
		event := &Event{
			EventID:   "event" + string(rune('0'+i)),
			RequestID: "req" + string(rune('0'+i)),
			SessionID: "session1",
			Timestamp: time.Now(),
			EventType: EventTypeStore,
			Latency: LatencyBreakdown{
				Total: time.Millisecond * time.Duration(50+i*10),
			},
		}
		analyzer.ProcessEvent(event)
	}

	// 检查窗口统计
	stats := analyzer.GetWindowStats("1m")
	if stats == nil {
		t.Fatal("GetWindowStats returned nil")
	}

	if stats.TotalRequests == 0 {
		t.Error("TotalRequests should be > 0")
	}
}

func TestAnalyzer_GetWindowStats_NonExistent(t *testing.T) {
	analyzer := NewAnalyzer([]string{"1m"})

	// 获取不存在的窗口
	stats := analyzer.GetWindowStats("nonexistent")
	if stats != nil {
		t.Error("GetWindowStats should return nil for nonexistent window")
	}
}

func TestAnalyzer_GetTrends(t *testing.T) {
	analyzer := NewAnalyzer([]string{"1m", "1h"})

	// 添加一些事件
	for i := 0; i < 5; i++ {
		event := &Event{
			EventID:   "event" + string(rune('0'+i)),
			RequestID: "req" + string(rune('0'+i)),
			SessionID: "session1",
			Timestamp: time.Now(),
			EventType: EventTypeRetrieve,
			Latency: LatencyBreakdown{
				Total: time.Millisecond * 100,
			},
		}
		analyzer.ProcessEvent(event)
	}

	trends := analyzer.GetTrends(time.Hour)
	if trends == nil {
		t.Fatal("GetTrends returned nil")
	}
}

func TestAnalyzer_Lifecycle(t *testing.T) {
	analyzer := NewAnalyzer([]string{"1m"})

	// 启动后台任务（在goroutine中）
	go analyzer.StartBackgroundTasks()

	// 等待后台任务启动
	time.Sleep(time.Millisecond * 50)

	// 添加事件
	event := &Event{
		EventID:   "event1",
		RequestID: "req1",
		SessionID: "session1",
		Timestamp: time.Now(),
		EventType: EventTypeStore,
	}
	analyzer.ProcessEvent(event)

	// 停止
	analyzer.Stop()

	// 等待后台任务退出
	time.Sleep(time.Millisecond * 50)
}
