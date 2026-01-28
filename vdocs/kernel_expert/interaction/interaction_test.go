package interaction

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestA2AServer_RegisterHandler(t *testing.T) {
	card := &AgentCard{
		ID:   "test-agent",
		Name: "Test Agent",
	}

	server := NewA2AServer(card)

	server.RegisterHandler("test_capability", func(ctx context.Context, input map[string]interface{}) (*A2AResponse, error) {
		return &A2AResponse{Success: true}, nil
	})

	// 验证handler已注册
	if len(server.handlers) != 1 {
		t.Errorf("Expected 1 handler, got %d", len(server.handlers))
	}
}

func TestA2AServer_HandleRequest(t *testing.T) {
	card := &AgentCard{
		ID:   "test-agent",
		Name: "Test Agent",
	}

	server := NewA2AServer(card)

	// 注册echo处理器
	server.RegisterHandler("echo", func(ctx context.Context, input map[string]interface{}) (*A2AResponse, error) {
		return &A2AResponse{
			Success: true,
			Output: map[string]interface{}{
				"echo": input["message"],
			},
		}, nil
	})

	ctx := context.Background()

	// 创建请求
	msg := &A2AMessage{
		ID:        "msg-001",
		Type:      A2AMessageRequest,
		From:      "client",
		To:        "test-agent",
		Timestamp: time.Now(),
		Payload: &A2ARequest{
			Capability: "echo",
			Input: map[string]interface{}{
				"message": "Hello",
			},
		},
	}

	resp, err := server.HandleRequest(ctx, msg)
	if err != nil {
		t.Fatalf("HandleRequest failed: %v", err)
	}

	if resp.Type != A2AMessageResponse {
		t.Errorf("Expected response type, got %s", resp.Type)
	}

	if resp.From != "test-agent" {
		t.Errorf("Expected from test-agent, got %s", resp.From)
	}
}

func TestA2AServer_HandleRequest_NotFound(t *testing.T) {
	card := &AgentCard{ID: "test-agent"}
	server := NewA2AServer(card)
	ctx := context.Background()

	msg := &A2AMessage{
		ID:   "msg-001",
		Type: A2AMessageRequest,
		From: "client",
		Payload: &A2ARequest{
			Capability: "nonexistent",
			Input:      map[string]interface{}{},
		},
	}

	resp, err := server.HandleRequest(ctx, msg)
	if err != nil {
		t.Fatalf("HandleRequest failed: %v", err)
	}

	if resp.Type != A2AMessageError {
		t.Errorf("Expected error type, got %s", resp.Type)
	}
}

func TestA2AServer_ServeHTTP(t *testing.T) {
	card := &AgentCard{
		ID:   "test-agent",
		Name: "Test Agent",
	}

	server := NewA2AServer(card)
	server.RegisterHandler("ping", func(ctx context.Context, input map[string]interface{}) (*A2AResponse, error) {
		return &A2AResponse{
			Success: true,
			Output:  map[string]interface{}{"pong": true},
		}, nil
	})

	// 创建HTTP测试
	ts := httptest.NewServer(server)
	defer ts.Close()

	// 构造请求
	msg := &A2AMessage{
		ID:   "msg-001",
		Type: A2AMessageRequest,
		From: "client",
		Payload: map[string]interface{}{
			"capability": "ping",
			"input":      map[string]interface{}{},
		},
	}

	body, _ := json.Marshal(msg)
	resp, err := http.Post(ts.URL, "application/json", strings.NewReader(string(body)))
	if err != nil {
		t.Fatalf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp.StatusCode)
	}
}

func TestA2AClient_RegisterAgent(t *testing.T) {
	client := NewA2AClient()

	card := &AgentCard{
		ID:   "external-agent",
		Name: "External Agent",
		Endpoints: []Endpoint{
			{Protocol: "a2a", URL: "http://localhost:8080/a2a", Priority: 1},
		},
		Capabilities: []Capability{
			{Name: "analyze"},
		},
	}

	client.RegisterAgent(card)

	agents := client.DiscoverAgents("analyze")
	if len(agents) != 1 {
		t.Errorf("Expected 1 agent, got %d", len(agents))
	}
}

func TestA2AClient_DiscoverAgents(t *testing.T) {
	client := NewA2AClient()

	client.RegisterAgent(&AgentCard{
		ID:           "agent1",
		Capabilities: []Capability{{Name: "search"}},
	})
	client.RegisterAgent(&AgentCard{
		ID:           "agent2",
		Capabilities: []Capability{{Name: "analyze"}, {Name: "search"}},
	})
	client.RegisterAgent(&AgentCard{
		ID:           "agent3",
		Capabilities: []Capability{{Name: "generate"}},
	})

	searchAgents := client.DiscoverAgents("search")
	if len(searchAgents) != 2 {
		t.Errorf("Expected 2 agents with search capability, got %d", len(searchAgents))
	}

	generateAgents := client.DiscoverAgents("generate")
	if len(generateAgents) != 1 {
		t.Errorf("Expected 1 agent with generate capability, got %d", len(generateAgents))
	}
}

func TestAgentCard_Validation(t *testing.T) {
	card := &AgentCard{
		ID:          "kernel-expert",
		Name:        "Linux Kernel Expert",
		Description: "Expert agent for Linux kernel analysis",
		Version:     "1.0.0",
		Capabilities: []Capability{
			{
				Name:        "analyze_function",
				Description: "Analyze kernel function",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"function_name": map[string]interface{}{"type": "string"},
					},
				},
			},
		},
		Endpoints: []Endpoint{
			{Protocol: "a2a", URL: "https://api.example.com/a2a", Priority: 1},
			{Protocol: "mcp", URL: "https://api.example.com/mcp", Priority: 2},
		},
	}

	if card.ID == "" {
		t.Error("ID should not be empty")
	}
	if len(card.Capabilities) != 1 {
		t.Error("Should have 1 capability")
	}
	if len(card.Endpoints) != 2 {
		t.Error("Should have 2 endpoints")
	}
}

func TestMCPServer_RegisterTool(t *testing.T) {
	server := NewMCPServer("test-server", "1.0.0")

	tool := &MCPTool{
		Name:        "search_code",
		Description: "Search kernel source code",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]interface{}{"type": "string"},
			},
		},
	}

	server.RegisterTool(tool, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		return map[string]interface{}{"results": []string{}}, nil
	})

	tools := server.GetTools()
	if len(tools) != 1 {
		t.Errorf("Expected 1 tool, got %d", len(tools))
	}
	if tools[0].Name != "search_code" {
		t.Errorf("Expected tool name 'search_code', got '%s'", tools[0].Name)
	}
}

func TestMCPServer_RegisterResource(t *testing.T) {
	server := NewMCPServer("test-server", "1.0.0")

	resource := &MCPResource{
		URI:         "kernel://source/fork.c",
		Name:        "fork.c",
		Description: "Linux kernel fork implementation",
		MimeType:    "text/plain",
	}

	server.RegisterResource(resource, func(ctx context.Context, uri string) ([]byte, string, error) {
		return []byte("source code"), "text/plain", nil
	})

	resources := server.GetResources()
	if len(resources) != 1 {
		t.Errorf("Expected 1 resource, got %d", len(resources))
	}
}

func TestMCPServer_RegisterPrompt(t *testing.T) {
	server := NewMCPServer("test-server", "1.0.0")

	prompt := &MCPPrompt{
		Name:        "analyze",
		Description: "Analyze kernel code",
		Arguments: []Argument{
			{Name: "code", Description: "Code to analyze", Required: true},
		},
	}

	server.RegisterPrompt(prompt)

	prompts := server.GetPrompts()
	if len(prompts) != 1 {
		t.Errorf("Expected 1 prompt, got %d", len(prompts))
	}
}

func TestMCPServer_CallTool(t *testing.T) {
	server := NewMCPServer("test-server", "1.0.0")

	server.RegisterTool(&MCPTool{Name: "echo"}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		return args["message"], nil
	})

	ctx := context.Background()
	result, err := server.CallTool(ctx, "echo", map[string]interface{}{"message": "hello"})
	if err != nil {
		t.Fatalf("CallTool failed: %v", err)
	}

	if result != "hello" {
		t.Errorf("Expected 'hello', got '%v'", result)
	}
}

func TestMCPServer_CallTool_NotFound(t *testing.T) {
	server := NewMCPServer("test-server", "1.0.0")
	ctx := context.Background()

	_, err := server.CallTool(ctx, "nonexistent", nil)
	if err == nil {
		t.Error("Expected error for nonexistent tool")
	}
}

func TestMCPServer_ReadResource(t *testing.T) {
	server := NewMCPServer("test-server", "1.0.0")

	server.RegisterResource(&MCPResource{URI: "test://file"}, func(ctx context.Context, uri string) ([]byte, string, error) {
		return []byte("content"), "text/plain", nil
	})

	ctx := context.Background()
	data, mimeType, err := server.ReadResource(ctx, "test://file")
	if err != nil {
		t.Fatalf("ReadResource failed: %v", err)
	}

	if string(data) != "content" {
		t.Errorf("Expected 'content', got '%s'", string(data))
	}
	if mimeType != "text/plain" {
		t.Errorf("Expected 'text/plain', got '%s'", mimeType)
	}
}

func TestMCPServer_ServeHTTP_Initialize(t *testing.T) {
	server := NewMCPServer("kernel-expert", "1.0.0")

	ts := httptest.NewServer(server)
	defer ts.Close()

	req := &MCPRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
	}

	body, _ := json.Marshal(req)
	resp, err := http.Post(ts.URL, "application/json", strings.NewReader(string(body)))
	if err != nil {
		t.Fatalf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp.StatusCode)
	}

	var mcpResp MCPResponse
	json.NewDecoder(resp.Body).Decode(&mcpResp)

	if mcpResp.Error != nil {
		t.Errorf("Expected no error, got %s", mcpResp.Error.Message)
	}
}

func TestMCPServer_ServeHTTP_ListTools(t *testing.T) {
	server := NewMCPServer("kernel-expert", "1.0.0")
	server.RegisterTool(&MCPTool{Name: "search"}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		return nil, nil
	})

	ts := httptest.NewServer(server)
	defer ts.Close()

	req := &MCPRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/list",
	}

	body, _ := json.Marshal(req)
	resp, err := http.Post(ts.URL, "application/json", strings.NewReader(string(body)))
	if err != nil {
		t.Fatalf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	var mcpResp MCPResponse
	json.NewDecoder(resp.Body).Decode(&mcpResp)

	result, ok := mcpResp.Result.(map[string]interface{})
	if !ok {
		t.Fatal("Expected result to be a map")
	}

	tools, ok := result["tools"]
	if !ok {
		t.Error("Expected tools in result")
	}

	toolsList, ok := tools.([]*MCPTool)
	if ok && len(toolsList) != 1 {
		t.Errorf("Expected 1 tool, got %d", len(toolsList))
	}
}

func TestA2AMessage_Types(t *testing.T) {
	types := []A2AMessageType{
		A2AMessageRequest,
		A2AMessageResponse,
		A2AMessageStream,
		A2AMessageError,
		A2AMessageCancel,
		A2AMessageHeartbeat,
	}

	for _, msgType := range types {
		if msgType == "" {
			t.Error("Message type should not be empty")
		}
	}
}

func TestA2AContext(t *testing.T) {
	ctx := &A2AContext{
		ConversationID: "conv-001",
		ParentID:       "msg-000",
		Metadata:       map[string]string{"key": "value"},
		MaxTokens:      4096,
		Timeout:        30,
	}

	if ctx.ConversationID == "" {
		t.Error("ConversationID should not be empty")
	}
	if ctx.Timeout != 30 {
		t.Errorf("Expected timeout 30, got %d", ctx.Timeout)
	}
}

func TestA2AError(t *testing.T) {
	err := &A2AError{
		Code:    "capability_not_found",
		Message: "Capability not found: analyze",
		Details: map[string]interface{}{"requested": "analyze"},
	}

	if err.Code == "" {
		t.Error("Error code should not be empty")
	}
	if err.Message == "" {
		t.Error("Error message should not be empty")
	}
}

func TestUsage(t *testing.T) {
	usage := &Usage{
		InputTokens:  1000,
		OutputTokens: 500,
		TotalTokens:  1500,
	}

	if usage.TotalTokens != usage.InputTokens+usage.OutputTokens {
		t.Error("Total tokens should equal input + output")
	}
}

func BenchmarkA2AServer_HandleRequest(b *testing.B) {
	card := &AgentCard{ID: "test-agent"}
	server := NewA2AServer(card)
	server.RegisterHandler("echo", func(ctx context.Context, input map[string]interface{}) (*A2AResponse, error) {
		return &A2AResponse{Success: true}, nil
	})

	ctx := context.Background()
	msg := &A2AMessage{
		ID:   "msg-001",
		Type: A2AMessageRequest,
		From: "client",
		Payload: &A2ARequest{
			Capability: "echo",
			Input:      map[string]interface{}{},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		server.HandleRequest(ctx, msg)
	}
}

func BenchmarkMCPServer_CallTool(b *testing.B) {
	server := NewMCPServer("test", "1.0.0")
	server.RegisterTool(&MCPTool{Name: "test"}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		return nil, nil
	})

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		server.CallTool(ctx, "test", nil)
	}
}
