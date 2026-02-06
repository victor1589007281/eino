package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// MockMemoryService 模拟Memory服务
type MockMemoryService struct{}

func (m *MockMemoryService) Store(ctx context.Context, req *StoreRequest) (*StoreResponse, error) {
	return &StoreResponse{
		MessageID:  "msg-123",
		Tier:       "L1",
		TokenCount: 10,
	}, nil
}

func (m *MockMemoryService) Retrieve(ctx context.Context, req *RetrieveRequest) (*RetrieveResponse, error) {
	return &RetrieveResponse{
		Context: []*ContextItem{
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

func (m *MockMemoryService) BatchStore(ctx context.Context, req *BatchStoreRequest) (*BatchStoreResponse, error) {
	return &BatchStoreResponse{
		SuccessCount: len(req.Messages),
		TotalTokens:  len(req.Messages) * 10,
	}, nil
}

func (m *MockMemoryService) BatchRetrieve(ctx context.Context, req *BatchRetrieveRequest) (*BatchRetrieveResponse, error) {
	results := make([]*RetrieveResponse, len(req.Requests))
	for i := range req.Requests {
		results[i] = &RetrieveResponse{TotalTokens: 5}
	}
	return &BatchRetrieveResponse{Results: results}, nil
}

func (m *MockMemoryService) GetSession(ctx context.Context, sessionID string) (*SessionInfo, error) {
	return &SessionInfo{
		SessionID:    sessionID,
		MessageCount: 10,
		TokenCount:   100,
		CreatedAt:    time.Now(),
		LastActiveAt: time.Now(),
	}, nil
}

func (m *MockMemoryService) DeleteSession(ctx context.Context, sessionID string) error {
	return nil
}

func (m *MockMemoryService) SwitchTopic(ctx context.Context, req *SwitchTopicRequest) (*SwitchTopicResponse, error) {
	return &SwitchTopicResponse{
		CapsuleID:  "capsule-123",
		TopicTitle: req.NewTopic,
	}, nil
}

func (m *MockMemoryService) RecallTopic(ctx context.Context, req *RecallTopicRequest) (*RecallTopicResponse, error) {
	return &RecallTopicResponse{
		CapsuleID:  "capsule-123",
		TopicTitle: "Recalled Topic",
	}, nil
}

func (m *MockMemoryService) ListTopics(ctx context.Context, sessionID string) ([]*TopicInfo, error) {
	return []*TopicInfo{
		{TopicID: "topic-1", Title: "Topic 1"},
		{TopicID: "topic-2", Title: "Topic 2"},
	}, nil
}

func (m *MockMemoryService) GetEntityRelations(ctx context.Context, entityName string, depth int) ([]*Relation, error) {
	return []*Relation{}, nil
}

func (m *MockMemoryService) Summarize(ctx context.Context, sessionID string) (*SummaryResponse, error) {
	return &SummaryResponse{
		Summary:      "This is a summary",
		MessageCount: 10,
		TokenCount:   50,
	}, nil
}

func (m *MockMemoryService) Archive(ctx context.Context, req *ArchiveRequest) (*ArchiveResponse, error) {
	return &ArchiveResponse{
		CapsuleID: "capsule-123",
		ArchiveID: "archive-123",
		Archived:  true,
	}, nil
}

func (m *MockMemoryService) GetStats(ctx context.Context) (*Stats, error) {
	return &Stats{
		L1Stats: &TierStats{SessionCount: 1, MessageCount: 10},
		L2Stats: &TierStats{SessionCount: 2, MessageCount: 50},
		L3Stats: &TierStats{SessionCount: 5, MessageCount: 200},
	}, nil
}

func setupTestHandler() *http.ServeMux {
	mockService := &MockMemoryService{}
	handler := NewHandler(mockService)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	return mux
}

func TestHandler_Store(t *testing.T) {
	mux := setupTestHandler()

	reqBody := StoreRequest{
		SessionID: "session-123",
		Message: MessageInput{
			Role:    "user",
			Content: "Hello, world!",
		},
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/memory/store", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Store returned status %d, want %d, body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var resp APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if !resp.Success {
		t.Error("Response should be successful")
	}
}

func TestHandler_Retrieve(t *testing.T) {
	mux := setupTestHandler()

	reqBody := RetrieveRequest{
		SessionID: "session-123",
		Query:     "What did I say?",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/memory/retrieve", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Retrieve returned status %d, want %d, body: %s", w.Code, http.StatusOK, w.Body.String())
	}
}

func TestHandler_GetSession(t *testing.T) {
	mux := setupTestHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/memory/session/session-123", nil)

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GetSession returned status %d, want %d, body: %s", w.Code, http.StatusOK, w.Body.String())
	}
}

func TestHandler_DeleteSession(t *testing.T) {
	mux := setupTestHandler()

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/memory/session/session-123", nil)

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("DeleteSession returned status %d, want %d, body: %s", w.Code, http.StatusOK, w.Body.String())
	}
}

func TestHandler_BatchStore(t *testing.T) {
	mux := setupTestHandler()

	reqBody := BatchStoreRequest{
		SessionID: "session-123",
		Messages: []MessageInput{
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: "Hi there!"},
		},
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/memory/batch/store", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("BatchStore returned status %d, want %d, body: %s", w.Code, http.StatusOK, w.Body.String())
	}
}

func TestHandler_GetStats(t *testing.T) {
	mux := setupTestHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/memory/stats", nil)

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GetStats returned status %d, want %d, body: %s", w.Code, http.StatusOK, w.Body.String())
	}
}

func TestHandler_Health(t *testing.T) {
	mux := setupTestHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Health returned status %d, want %d", w.Code, http.StatusOK)
	}
}

func TestHandler_ListTopics(t *testing.T) {
	mux := setupTestHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/memory/topics/session-123", nil)

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("ListTopics returned status %d, want %d, body: %s", w.Code, http.StatusOK, w.Body.String())
	}
}

func TestHandler_SwitchTopic(t *testing.T) {
	mux := setupTestHandler()

	reqBody := SwitchTopicRequest{
		SessionID: "session-123",
		NewTopic:  "New Topic",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/memory/topic/switch", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("SwitchTopic returned status %d, want %d, body: %s", w.Code, http.StatusOK, w.Body.String())
	}
}

func TestHandler_Summarize(t *testing.T) {
	mux := setupTestHandler()

	reqBody := map[string]string{"session_id": "session-123"}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/memory/summarize", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Summarize returned status %d, want %d, body: %s", w.Code, http.StatusOK, w.Body.String())
	}
}

func TestHandler_Archive(t *testing.T) {
	mux := setupTestHandler()

	reqBody := ArchiveRequest{
		SessionID:  "session-123",
		TopicTitle: "Archive Title",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/memory/archive", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Archive returned status %d, want %d, body: %s", w.Code, http.StatusOK, w.Body.String())
	}
}
