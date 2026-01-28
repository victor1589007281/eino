package interaction

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestManager_RegisterAgent(t *testing.T) {
	config := DefaultConfig()
	m := NewManager(config)

	agent := &AgentInfo{
		ID:          "test-agent",
		Name:        "Test Agent",
		Description: "A test agent",
		Endpoint:    "http://localhost:8080",
		Protocol:    ProtocolHTTP,
		Enabled:     true,
	}

	m.RegisterAgent(agent)

	// Verify registration
	info, err := m.GetAgent("test-agent")
	if err != nil {
		t.Fatalf("GetAgent failed: %v", err)
	}
	if info.Name != "Test Agent" {
		t.Errorf("Expected name 'Test Agent', got '%s'", info.Name)
	}
}

func TestManager_UnregisterAgent(t *testing.T) {
	config := DefaultConfig()
	m := NewManager(config)

	agent := &AgentInfo{
		ID:       "test-agent",
		Endpoint: "http://localhost:8080",
		Protocol: ProtocolHTTP,
		Enabled:  true,
	}

	m.RegisterAgent(agent)
	m.UnregisterAgent("test-agent")

	_, err := m.GetAgent("test-agent")
	if err == nil {
		t.Error("Expected error after unregistration")
	}
}

func TestManager_ListAgents(t *testing.T) {
	config := DefaultConfig()
	m := NewManager(config)

	m.RegisterAgent(&AgentInfo{ID: "agent1", Endpoint: "http://localhost:8081"})
	m.RegisterAgent(&AgentInfo{ID: "agent2", Endpoint: "http://localhost:8082"})
	m.RegisterAgent(&AgentInfo{ID: "agent3", Endpoint: "http://localhost:8083"})

	agents := m.ListAgents()
	if len(agents) != 3 {
		t.Errorf("Expected 3 agents, got %d", len(agents))
	}
}

func TestManager_RegisterHandler(t *testing.T) {
	config := DefaultConfig()
	m := NewManager(config)

	called := false
	handler := func(ctx context.Context, msg *Message) (*Message, error) {
		called = true
		return &Message{
			ID:      msg.ID,
			Type:    "response",
			Content: "handled",
		}, nil
	}

	m.RegisterHandler("test", handler)

	msg := &Message{
		ID:   "1",
		Type: "test",
	}

	ctx := context.Background()
	_, err := m.HandleMessage(ctx, msg)
	if err != nil {
		t.Fatalf("HandleMessage failed: %v", err)
	}
	if !called {
		t.Error("Handler was not called")
	}
}

func TestManager_HandleMessage_NoHandler(t *testing.T) {
	config := DefaultConfig()
	m := NewManager(config)

	msg := &Message{
		ID:   "1",
		Type: "unknown",
	}

	ctx := context.Background()
	_, err := m.HandleMessage(ctx, msg)
	if err == nil {
		t.Error("Expected error for unknown message type")
	}
}

func TestManager_SendMessage_HTTP(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var msg Message
		if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		response := Message{
			ID:      msg.ID,
			From:    "test-agent",
			To:      msg.From,
			Type:    "response",
			Content: "received",
			ReplyTo: msg.ID,
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	config := DefaultConfig()
	m := NewManager(config)

	m.RegisterAgent(&AgentInfo{
		ID:       "test-agent",
		Endpoint: server.URL,
		Protocol: ProtocolHTTP,
		Enabled:  true,
		Timeout:  5 * time.Second,
	})

	msg := &Message{
		ID:      "1",
		To:      "test-agent",
		Type:    "test",
		Content: "hello",
	}

	ctx := context.Background()
	resp, err := m.SendMessage(ctx, msg)
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}
	if resp.Content != "received" {
		t.Errorf("Expected content 'received', got '%v'", resp.Content)
	}
}

func TestManager_SendMessage_DisabledAgent(t *testing.T) {
	config := DefaultConfig()
	m := NewManager(config)

	m.RegisterAgent(&AgentInfo{
		ID:       "disabled-agent",
		Endpoint: "http://localhost:8080",
		Protocol: ProtocolHTTP,
		Enabled:  false,
	})

	msg := &Message{
		ID:   "1",
		To:   "disabled-agent",
		Type: "test",
	}

	ctx := context.Background()
	_, err := m.SendMessage(ctx, msg)
	if err == nil {
		t.Error("Expected error for disabled agent")
	}
}

func TestManager_SendMessage_A2A(t *testing.T) {
	// Create A2A test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req A2ARequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		response := A2AResponse{
			JSONRPC: "2.0",
			Result:  "success",
			ID:      req.ID,
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	config := DefaultConfig()
	m := NewManager(config)

	m.RegisterAgent(&AgentInfo{
		ID:       "a2a-agent",
		Endpoint: server.URL,
		Protocol: ProtocolA2A,
		Enabled:  true,
		Timeout:  5 * time.Second,
	})

	msg := &Message{
		ID:      "1",
		To:      "a2a-agent",
		Type:    "test",
		Content: "hello",
	}

	ctx := context.Background()
	resp, err := m.SendMessage(ctx, msg)
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}
	if resp.Content != "success" {
		t.Errorf("Expected content 'success', got '%v'", resp.Content)
	}
}

func TestManager_Query(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := Message{
			ID:      "1",
			Type:    "response",
			Content: "answer",
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	config := DefaultConfig()
	m := NewManager(config)

	m.RegisterAgent(&AgentInfo{
		ID:       "query-agent",
		Endpoint: server.URL,
		Protocol: ProtocolHTTP,
		Enabled:  true,
	})

	ctx := context.Background()
	result, err := m.Query(ctx, "query-agent", "what is insurance")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if result != "answer" {
		t.Errorf("Expected result 'answer', got '%v'", result)
	}
}

func TestManager_Delegate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := Message{
			ID:      "1",
			Type:    "response",
			Content: map[string]interface{}{"status": "completed"},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	config := DefaultConfig()
	m := NewManager(config)

	m.RegisterAgent(&AgentInfo{
		ID:       "worker-agent",
		Endpoint: server.URL,
		Protocol: ProtocolHTTP,
		Enabled:  true,
	})

	task := map[string]interface{}{
		"type": "analyze",
		"data": "test data",
	}

	ctx := context.Background()
	result, err := m.Delegate(ctx, "worker-agent", task)
	if err != nil {
		t.Fatalf("Delegate failed: %v", err)
	}

	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("Expected map result")
	}
	if resultMap["status"] != "completed" {
		t.Errorf("Expected status 'completed', got '%v'", resultMap["status"])
	}
}

func TestManager_Broadcast(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		response := Message{
			ID:      "1",
			Type:    "response",
			Content: "ok",
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	config := DefaultConfig()
	m := NewManager(config)

	m.RegisterAgent(&AgentInfo{ID: "agent1", Endpoint: server.URL, Protocol: ProtocolHTTP, Enabled: true})
	m.RegisterAgent(&AgentInfo{ID: "agent2", Endpoint: server.URL, Protocol: ProtocolHTTP, Enabled: true})
	m.RegisterAgent(&AgentInfo{ID: "agent3", Endpoint: server.URL, Protocol: ProtocolHTTP, Enabled: true})

	msg := &Message{
		ID:      "broadcast-1",
		Type:    "notify",
		Content: "hello all",
	}

	ctx := context.Background()
	responses, errs := m.Broadcast(ctx, []string{"agent1", "agent2", "agent3"}, msg)

	// Check all succeeded
	for i, err := range errs {
		if err != nil {
			t.Errorf("Agent %d error: %v", i, err)
		}
	}

	if len(responses) != 3 {
		t.Errorf("Expected 3 responses, got %d", len(responses))
	}

	if callCount != 3 {
		t.Errorf("Expected 3 calls, got %d", callCount)
	}
}

func TestManager_HealthCheck(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := Message{
			ID:   "1",
			Type: "health_check_response",
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	config := DefaultConfig()
	m := NewManager(config)

	m.RegisterAgent(&AgentInfo{
		ID:       "health-agent",
		Endpoint: server.URL,
		Protocol: ProtocolHTTP,
		Enabled:  true,
	})

	ctx := context.Background()
	err := m.HealthCheck(ctx, "health-agent")
	if err != nil {
		t.Fatalf("HealthCheck failed: %v", err)
	}
}

func TestValidateMessage(t *testing.T) {
	tests := []struct {
		name    string
		msg     *Message
		wantErr bool
	}{
		{
			name:    "nil message",
			msg:     nil,
			wantErr: true,
		},
		{
			name:    "missing ID",
			msg:     &Message{To: "agent", Type: "test"},
			wantErr: true,
		},
		{
			name:    "missing To",
			msg:     &Message{ID: "1", Type: "test"},
			wantErr: true,
		},
		{
			name:    "missing Type",
			msg:     &Message{ID: "1", To: "agent"},
			wantErr: true,
		},
		{
			name:    "valid message",
			msg:     &Message{ID: "1", To: "agent", Type: "test"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateMessage(tt.msg)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateMessage() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.LocalAgentID == "" {
		t.Error("Expected LocalAgentID")
	}
	if config.LocalAgentName == "" {
		t.Error("Expected LocalAgentName")
	}
	if config.DefaultTimeout == 0 {
		t.Error("Expected DefaultTimeout")
	}
	if config.RetryCount == 0 {
		t.Error("Expected RetryCount")
	}
}

func BenchmarkManager_SendMessage(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := Message{ID: "1", Type: "response", Content: "ok"}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	config := DefaultConfig()
	m := NewManager(config)

	m.RegisterAgent(&AgentInfo{
		ID:       "bench-agent",
		Endpoint: server.URL,
		Protocol: ProtocolHTTP,
		Enabled:  true,
	})

	msg := &Message{
		ID:      "1",
		To:      "bench-agent",
		Type:    "test",
		Content: "hello",
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = m.SendMessage(ctx, msg)
	}
}
