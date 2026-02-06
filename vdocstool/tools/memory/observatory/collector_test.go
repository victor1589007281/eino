package observatory

import (
	"testing"
	"time"
)

func TestNewCollector(t *testing.T) {
	collector := NewCollector(100, 1.0)
	if collector == nil {
		t.Fatal("NewCollector returned nil")
	}
}

func TestCollector_Collect(t *testing.T) {
	collector := NewCollector(100, 1.0)

	event := &Event{
		EventID:   "event1",
		RequestID: "req1",
		SessionID: "session1",
		Timestamp: time.Now(),
		EventType: EventTypeStore,
	}

	collector.Collect(event)

	stats := collector.GetStats()
	if stats.TotalEvents != 1 {
		t.Errorf("TotalEvents = %d, want 1", stats.TotalEvents)
	}
	if stats.SampledEvents != 1 {
		t.Errorf("SampledEvents = %d, want 1", stats.SampledEvents)
	}
}

func TestCollector_Sampling(t *testing.T) {
	// 50% 采样率
	collector := NewCollector(100, 0.5)

	// 收集很多事件
	for i := 0; i < 100; i++ {
		event := &Event{
			EventID:   "event" + string(rune(i)),
			RequestID: "req" + string(rune(i)),
			SessionID: "session1",
			Timestamp: time.Now(),
			EventType: EventTypeStore,
		}
		collector.Collect(event)
	}

	stats := collector.GetStats()
	if stats.TotalEvents != 100 {
		t.Errorf("TotalEvents = %d, want 100", stats.TotalEvents)
	}

	// 采样事件应该大约是50%，但由于随机性，允许较大范围
	if stats.SampledEvents < 20 || stats.SampledEvents > 80 {
		t.Errorf("SampledEvents = %d, expected between 20 and 80 (50%% sampling)", stats.SampledEvents)
	}
}

func TestCollector_GetEvents(t *testing.T) {
	collector := NewCollector(100, 1.0)

	for i := 0; i < 10; i++ {
		event := &Event{
			EventID:   "event" + string(rune('0'+i)),
			RequestID: "req" + string(rune('0'+i)),
			SessionID: "session1",
			Timestamp: time.Now(),
			EventType: EventTypeStore,
		}
		collector.Collect(event)
	}

	events := collector.GetEvents(5)
	if len(events) != 5 {
		t.Errorf("GetEvents(5) returned %d events, want 5", len(events))
	}
}

func TestCollector_BufferOverflow(t *testing.T) {
	// 小缓冲区
	collector := NewCollector(5, 1.0)

	// 添加超过缓冲区大小的事件
	for i := 0; i < 10; i++ {
		event := &Event{
			EventID:   "event" + string(rune('0'+i)),
			RequestID: "req" + string(rune('0'+i)),
			SessionID: "session1",
			Timestamp: time.Now(),
			EventType: EventTypeStore,
		}
		collector.Collect(event)
	}

	stats := collector.GetStats()
	if stats.BufferUsed != 5 {
		t.Errorf("BufferUsed = %d, want 5 (buffer size)", stats.BufferUsed)
	}
}

func TestRingBuffer(t *testing.T) {
	rb := NewRingBuffer(5)

	// 添加事件
	for i := 0; i < 3; i++ {
		event := &Event{
			EventID:   "event" + string(rune('0'+i)),
			RequestID: "req" + string(rune('0'+i)),
		}
		rb.Add(event)
	}

	if rb.Count() != 3 {
		t.Errorf("Count = %d, want 3", rb.Count())
	}

	// 获取最近的事件
	events := rb.GetRecent(2)
	if len(events) != 2 {
		t.Errorf("GetRecent(2) returned %d events, want 2", len(events))
	}
}

func TestRingBuffer_Overflow(t *testing.T) {
	rb := NewRingBuffer(3)

	// 添加超过容量的事件
	for i := 0; i < 5; i++ {
		event := &Event{
			EventID:   "event" + string(rune('0'+i)),
			RequestID: "req" + string(rune('0'+i)),
		}
		rb.Add(event)
	}

	if rb.Count() != 3 {
		t.Errorf("Count = %d, want 3 (buffer size)", rb.Count())
	}
}

func TestRingBuffer_Get(t *testing.T) {
	rb := NewRingBuffer(10)

	event := &Event{
		EventID:   "target-event",
		RequestID: "req1",
	}
	rb.Add(event)

	retrieved := rb.Get("target-event")
	if retrieved == nil {
		t.Fatal("Get returned nil for existing event")
	}
	if retrieved.EventID != "target-event" {
		t.Errorf("Got wrong event: %s", retrieved.EventID)
	}

	// 获取不存在的事件
	notFound := rb.Get("nonexistent")
	if notFound != nil {
		t.Error("Get should return nil for nonexistent event")
	}
}
