// Package observatory 事件收集器
package observatory

import (
	"math/rand"
	"sync"
	"time"
)

// Collector 事件收集器
type Collector struct {
	// 环形缓冲区
	buffer *RingBuffer

	// 采样控制
	sampleRate float64

	// 统计
	totalEvents   int64
	sampledEvents int64

	mu sync.RWMutex
}

// NewCollector 创建收集器
func NewCollector(bufferSize int, sampleRate float64) *Collector {
	return &Collector{
		buffer:     NewRingBuffer(bufferSize),
		sampleRate: sampleRate,
	}
}

// Collect 收集事件
func (c *Collector) Collect(event *Event) {
	c.mu.Lock()
	c.totalEvents++

	// 采样
	if c.sampleRate < 1.0 && rand.Float64() > c.sampleRate {
		c.mu.Unlock()
		return
	}

	c.sampledEvents++
	c.mu.Unlock()

	c.buffer.Add(event)
}

// GetEvents 获取事件
func (c *Collector) GetEvents(limit int) []*Event {
	return c.buffer.GetRecent(limit)
}

// GetEvent 获取单个事件
func (c *Collector) GetEvent(eventID string) *Event {
	return c.buffer.Get(eventID)
}

// GetStats 获取统计
func (c *Collector) GetStats() CollectorStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return CollectorStats{
		TotalEvents:   c.totalEvents,
		SampledEvents: c.sampledEvents,
		BufferSize:    c.buffer.Size(),
		BufferUsed:    c.buffer.Count(),
		SampleRate:    c.sampleRate,
	}
}

// CollectorStats 收集器统计
type CollectorStats struct {
	TotalEvents   int64   `json:"total_events"`
	SampledEvents int64   `json:"sampled_events"`
	BufferSize    int     `json:"buffer_size"`
	BufferUsed    int     `json:"buffer_used"`
	SampleRate    float64 `json:"sample_rate"`
}

// RingBuffer 环形缓冲区
type RingBuffer struct {
	events   []*Event
	index    int
	count    int
	size     int
	eventMap map[string]int // eventID -> index

	mu sync.RWMutex
}

// NewRingBuffer 创建环形缓冲区
func NewRingBuffer(size int) *RingBuffer {
	return &RingBuffer{
		events:   make([]*Event, size),
		size:     size,
		eventMap: make(map[string]int),
	}
}

// Add 添加事件
func (rb *RingBuffer) Add(event *Event) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	// 如果覆盖旧事件，从 map 中删除
	if rb.events[rb.index] != nil {
		delete(rb.eventMap, rb.events[rb.index].EventID)
	}

	rb.events[rb.index] = event
	rb.eventMap[event.EventID] = rb.index

	rb.index = (rb.index + 1) % rb.size
	if rb.count < rb.size {
		rb.count++
	}
}

// Get 获取事件
func (rb *RingBuffer) Get(eventID string) *Event {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	if idx, ok := rb.eventMap[eventID]; ok {
		return rb.events[idx]
	}
	return nil
}

// GetRecent 获取最近的事件
func (rb *RingBuffer) GetRecent(limit int) []*Event {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	if limit > rb.count {
		limit = rb.count
	}

	result := make([]*Event, 0, limit)

	// 从最新的开始
	idx := (rb.index - 1 + rb.size) % rb.size
	for i := 0; i < limit; i++ {
		if rb.events[idx] != nil {
			result = append(result, rb.events[idx])
		}
		idx = (idx - 1 + rb.size) % rb.size
	}

	return result
}

// GetInTimeRange 获取时间范围内的事件
func (rb *RingBuffer) GetInTimeRange(start, end time.Time) []*Event {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	var result []*Event

	for i := 0; i < rb.count; i++ {
		idx := (rb.index - 1 - i + rb.size) % rb.size
		event := rb.events[idx]
		if event == nil {
			continue
		}

		if event.Timestamp.After(start) && event.Timestamp.Before(end) {
			result = append(result, event)
		}

		// 如果事件时间早于开始时间，可以停止
		if event.Timestamp.Before(start) {
			break
		}
	}

	return result
}

// Size 获取缓冲区大小
func (rb *RingBuffer) Size() int {
	return rb.size
}

// Count 获取当前事件数
func (rb *RingBuffer) Count() int {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	return rb.count
}
