package grpc

import (
	"context"
	"testing"
	"time"

	"github.com/cloudwego/eino/vdocstool/tools/memory/rest"
)

// MockMemoryService 模拟Memory服务
type MockMemoryService struct {
	StoreFunc            func(ctx context.Context, req *rest.StoreRequest) (*rest.StoreResponse, error)
	RetrieveFunc         func(ctx context.Context, req *rest.RetrieveRequest) (*rest.RetrieveResponse, error)
	BatchStoreFunc       func(ctx context.Context, req *rest.BatchStoreRequest) (*rest.BatchStoreResponse, error)
	BatchRetrieveFunc    func(ctx context.Context, req *rest.BatchRetrieveRequest) (*rest.BatchRetrieveResponse, error)
	GetSessionFunc       func(ctx context.Context, sessionID string) (*rest.SessionInfo, error)
	DeleteSessionFunc    func(ctx context.Context, sessionID string) error
	SwitchTopicFunc      func(ctx context.Context, req *rest.SwitchTopicRequest) (*rest.SwitchTopicResponse, error)
	RecallTopicFunc      func(ctx context.Context, req *rest.RecallTopicRequest) (*rest.RecallTopicResponse, error)
	ListTopicsFunc       func(ctx context.Context, sessionID string) ([]*rest.TopicInfo, error)
	GetEntityRelationsFunc func(ctx context.Context, entityName string, depth int) ([]*rest.Relation, error)
	SummarizeFunc        func(ctx context.Context, sessionID string) (*rest.SummaryResponse, error)
	ArchiveFunc          func(ctx context.Context, req *rest.ArchiveRequest) (*rest.ArchiveResponse, error)
	GetStatsFunc         func(ctx context.Context) (*rest.Stats, error)
}

func (m *MockMemoryService) Store(ctx context.Context, req *rest.StoreRequest) (*rest.StoreResponse, error) {
	if m.StoreFunc != nil {
		return m.StoreFunc(ctx, req)
	}
	return &rest.StoreResponse{
		MessageID:  "msg-123",
		Tier:       "L1",
		TokenCount: 10,
	}, nil
}

func (m *MockMemoryService) Retrieve(ctx context.Context, req *rest.RetrieveRequest) (*rest.RetrieveResponse, error) {
	if m.RetrieveFunc != nil {
		return m.RetrieveFunc(ctx, req)
	}
	return &rest.RetrieveResponse{
		Context: []*rest.ContextItem{
			{
				MessageID: "msg-123",
				Role:      "user",
				Content:   "Hello",
				Timestamp: time.Now(),
			},
		},
		TotalTokens: 5,
	}, nil
}

func (m *MockMemoryService) BatchStore(ctx context.Context, req *rest.BatchStoreRequest) (*rest.BatchStoreResponse, error) {
	if m.BatchStoreFunc != nil {
		return m.BatchStoreFunc(ctx, req)
	}
	return &rest.BatchStoreResponse{
		SuccessCount: len(req.Messages),
		TotalTokens:  len(req.Messages) * 10,
	}, nil
}

func (m *MockMemoryService) BatchRetrieve(ctx context.Context, req *rest.BatchRetrieveRequest) (*rest.BatchRetrieveResponse, error) {
	if m.BatchRetrieveFunc != nil {
		return m.BatchRetrieveFunc(ctx, req)
	}
	results := make([]*rest.RetrieveResponse, len(req.Requests))
	for i := range req.Requests {
		results[i] = &rest.RetrieveResponse{TotalTokens: 5}
	}
	return &rest.BatchRetrieveResponse{Results: results}, nil
}

func (m *MockMemoryService) GetSession(ctx context.Context, sessionID string) (*rest.SessionInfo, error) {
	if m.GetSessionFunc != nil {
		return m.GetSessionFunc(ctx, sessionID)
	}
	return &rest.SessionInfo{
		SessionID:    sessionID,
		MessageCount: 10,
		TokenCount:   100,
		CreatedAt:    time.Now(),
		LastActiveAt: time.Now(),
	}, nil
}

func (m *MockMemoryService) DeleteSession(ctx context.Context, sessionID string) error {
	if m.DeleteSessionFunc != nil {
		return m.DeleteSessionFunc(ctx, sessionID)
	}
	return nil
}

func (m *MockMemoryService) SwitchTopic(ctx context.Context, req *rest.SwitchTopicRequest) (*rest.SwitchTopicResponse, error) {
	if m.SwitchTopicFunc != nil {
		return m.SwitchTopicFunc(ctx, req)
	}
	return &rest.SwitchTopicResponse{
		CapsuleID:  "capsule-123",
		TopicTitle: req.NewTopic,
	}, nil
}

func (m *MockMemoryService) RecallTopic(ctx context.Context, req *rest.RecallTopicRequest) (*rest.RecallTopicResponse, error) {
	if m.RecallTopicFunc != nil {
		return m.RecallTopicFunc(ctx, req)
	}
	return &rest.RecallTopicResponse{
		CapsuleID:  "capsule-123",
		TopicTitle: "Recalled Topic",
	}, nil
}

func (m *MockMemoryService) ListTopics(ctx context.Context, sessionID string) ([]*rest.TopicInfo, error) {
	if m.ListTopicsFunc != nil {
		return m.ListTopicsFunc(ctx, sessionID)
	}
	return []*rest.TopicInfo{
		{TopicID: "topic-1", Title: "Topic 1"},
	}, nil
}

func (m *MockMemoryService) GetEntityRelations(ctx context.Context, entityName string, depth int) ([]*rest.Relation, error) {
	if m.GetEntityRelationsFunc != nil {
		return m.GetEntityRelationsFunc(ctx, entityName, depth)
	}
	return []*rest.Relation{}, nil
}

func (m *MockMemoryService) Summarize(ctx context.Context, sessionID string) (*rest.SummaryResponse, error) {
	if m.SummarizeFunc != nil {
		return m.SummarizeFunc(ctx, sessionID)
	}
	return &rest.SummaryResponse{
		Summary:      "Summary",
		MessageCount: 10,
	}, nil
}

func (m *MockMemoryService) Archive(ctx context.Context, req *rest.ArchiveRequest) (*rest.ArchiveResponse, error) {
	if m.ArchiveFunc != nil {
		return m.ArchiveFunc(ctx, req)
	}
	return &rest.ArchiveResponse{
		CapsuleID: "capsule-123",
		Archived:  true,
	}, nil
}

func (m *MockMemoryService) GetStats(ctx context.Context) (*rest.Stats, error) {
	if m.GetStatsFunc != nil {
		return m.GetStatsFunc(ctx)
	}
	return &rest.Stats{
		L1Stats: &rest.TierStats{SessionCount: 1},
		L2Stats: &rest.TierStats{SessionCount: 2},
		L3Stats: &rest.TierStats{SessionCount: 5},
	}, nil
}

func TestNewServer(t *testing.T) {
	mockService := &MockMemoryService{}
	config := DefaultServerConfig()
	
	server := NewServer(config, mockService)
	if server == nil {
		t.Fatal("NewServer returned nil")
	}
}

func TestRateLimiter(t *testing.T) {
	limiter := NewRateLimiter(5, time.Minute)

	// 前5次应该允许
	for i := 0; i < 5; i++ {
		if !limiter.Allow("test-key") {
			t.Errorf("Request %d should be allowed", i+1)
		}
	}

	// 第6次应该被限制
	if limiter.Allow("test-key") {
		t.Error("Request 6 should be rate limited")
	}

	// 不同的key应该允许
	if !limiter.Allow("other-key") {
		t.Error("Request with different key should be allowed")
	}
}

func TestRateLimiter_Reset(t *testing.T) {
	limiter := NewRateLimiter(2, time.Millisecond*100)

	// 使用完配额
	limiter.Allow("test")
	limiter.Allow("test")
	
	if limiter.Allow("test") {
		t.Error("Should be rate limited")
	}

	// 等待窗口重置
	time.Sleep(time.Millisecond * 150)

	// 应该再次允许
	if !limiter.Allow("test") {
		t.Error("Should be allowed after window reset")
	}
}

func TestServerConfig(t *testing.T) {
	config := DefaultServerConfig()

	if config.Address != ":50051" {
		t.Errorf("Default address = %s, want :50051", config.Address)
	}

	if config.MaxMsgSize != 16*1024*1024 {
		t.Errorf("Default max msg size = %d, want 16MB", config.MaxMsgSize)
	}

	if config.RateLimit != 100 {
		t.Errorf("Default rate limit = %d, want 100", config.RateLimit)
	}
}
