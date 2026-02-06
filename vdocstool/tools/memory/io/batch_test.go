package io

import (
	"context"
	"testing"
	"time"
)

func TestNewBatchReader(t *testing.T) {
	reader := NewBatchReader(100, time.Second*5)
	if reader == nil {
		t.Fatal("NewBatchReader returned nil")
	}
}

func TestBatchReader_Config(t *testing.T) {
	maxBatchSize := 50
	timeout := time.Second * 10
	reader := NewBatchReader(maxBatchSize, timeout)

	if reader.maxBatchSize != maxBatchSize {
		t.Errorf("maxBatchSize = %d, want %d", reader.maxBatchSize, maxBatchSize)
	}
	if reader.timeout != timeout {
		t.Errorf("timeout = %v, want %v", reader.timeout, timeout)
	}
}

func TestBatchReadRequest(t *testing.T) {
	req := &BatchReadRequest{
		L1Keys: []L1ReadKey{
			{SessionID: "session1", MessageID: "msg1"},
			{SessionID: "session1", MessageID: "msg2"},
		},
		L2Keys: []L2ReadKey{
			{CapsuleID: "cap1", Fields: []string{"summary", "title"}},
		},
		L3Keys: []L3ReadKey{
			{ArchiveID: "archive1", ChunkIDs: []string{"chunk1", "chunk2"}},
		},
	}

	if len(req.L1Keys) != 2 {
		t.Errorf("L1Keys count = %d, want 2", len(req.L1Keys))
	}
	if len(req.L2Keys) != 1 {
		t.Errorf("L2Keys count = %d, want 1", len(req.L2Keys))
	}
	if len(req.L3Keys) != 1 {
		t.Errorf("L3Keys count = %d, want 1", len(req.L3Keys))
	}
}

func TestBatchReadResult(t *testing.T) {
	result := &BatchReadResult{
		L1Results: map[string]*Message{
			"msg1": {ID: "msg1", Content: "Hello", Role: "user"},
		},
		L2Results: map[string]*Capsule{
			"cap1": {ID: "cap1", Title: "Test Capsule"},
		},
		L3Results: map[string]*ArchiveChunk{
			"chunk1": {ID: "chunk1", Content: "Archive content"},
		},
		Errors: nil,
	}

	if len(result.L1Results) != 1 {
		t.Errorf("L1Results count = %d, want 1", len(result.L1Results))
	}
	if len(result.L2Results) != 1 {
		t.Errorf("L2Results count = %d, want 1", len(result.L2Results))
	}
	if len(result.L3Results) != 1 {
		t.Errorf("L3Results count = %d, want 1", len(result.L3Results))
	}
}

func TestMessage(t *testing.T) {
	msg := &Message{
		ID:         "msg1",
		SessionID:  "session1",
		Content:    "Test content",
		Role:       "user",
		Timestamp:  time.Now(),
		TokenCount: 10,
	}

	if msg.ID != "msg1" {
		t.Errorf("ID = %s, want msg1", msg.ID)
	}
	if msg.TokenCount != 10 {
		t.Errorf("TokenCount = %d, want 10", msg.TokenCount)
	}
}

func TestCapsule(t *testing.T) {
	capsule := &Capsule{
		ID:        "cap1",
		SessionID: "session1",
		Title:     "Test Capsule",
		Summary:   "A test summary",
		Messages: []*Message{
			{ID: "msg1", Content: "Hello"},
			{ID: "msg2", Content: "World"},
		},
	}

	if len(capsule.Messages) != 2 {
		t.Errorf("Messages count = %d, want 2", len(capsule.Messages))
	}
}

// MockL1Reader 模拟L1读取器
type MockL1Reader struct {
	data map[string][]byte
}

func (m *MockL1Reader) MGet(ctx context.Context, keys []string) (map[string][]byte, error) {
	result := make(map[string][]byte)
	for _, key := range keys {
		if val, ok := m.data[key]; ok {
			result[key] = val
		}
	}
	return result, nil
}

// MockL2Reader 模拟L2读取器
type MockL2Reader struct {
	data map[string]*Capsule
}

func (m *MockL2Reader) BatchGet(ctx context.Context, ids []string) ([]*Capsule, error) {
	var result []*Capsule
	for _, id := range ids {
		if cap, ok := m.data[id]; ok {
			result = append(result, cap)
		}
	}
	return result, nil
}

// MockL3Reader 模拟L3读取器
type MockL3Reader struct {
	data map[string][]*ArchiveChunk
}

func (m *MockL3Reader) BatchFetch(ctx context.Context, archiveID string, chunkIDs []string) ([]*ArchiveChunk, error) {
	chunks, ok := m.data[archiveID]
	if !ok {
		return nil, nil
	}

	var result []*ArchiveChunk
	chunkIDSet := make(map[string]bool)
	for _, id := range chunkIDs {
		chunkIDSet[id] = true
	}

	for _, chunk := range chunks {
		if chunkIDSet[chunk.ID] {
			result = append(result, chunk)
		}
	}
	return result, nil
}
