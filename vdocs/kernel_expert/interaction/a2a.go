// Package interaction 提供Agent交互协议
package interaction

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
)

// A2AMessageType A2A消息类型
type A2AMessageType string

const (
	A2AMessageRequest   A2AMessageType = "request"
	A2AMessageResponse  A2AMessageType = "response"
	A2AMessageStream    A2AMessageType = "stream"
	A2AMessageError     A2AMessageType = "error"
	A2AMessageCancel    A2AMessageType = "cancel"
	A2AMessageHeartbeat A2AMessageType = "heartbeat"
)

// A2AMessage A2A消息
type A2AMessage struct {
	ID        string         `json:"id"`
	Type      A2AMessageType `json:"type"`
	From      string         `json:"from"`
	To        string         `json:"to"`
	Timestamp time.Time      `json:"timestamp"`
	Payload   interface{}    `json:"payload"`
	Context   *A2AContext    `json:"context,omitempty"`
}

// A2AContext A2A上下文
type A2AContext struct {
	ConversationID string            `json:"conversation_id"`
	ParentID       string            `json:"parent_id,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty"`
	MaxTokens      int               `json:"max_tokens,omitempty"`
	Timeout        int               `json:"timeout_seconds,omitempty"`
}

// A2ARequest A2A请求
type A2ARequest struct {
	Capability string                 `json:"capability"`
	Input      map[string]interface{} `json:"input"`
	Streaming  bool                   `json:"streaming"`
}

// A2AResponse A2A响应
type A2AResponse struct {
	Success bool                   `json:"success"`
	Output  map[string]interface{} `json:"output,omitempty"`
	Error   *A2AError              `json:"error,omitempty"`
	Usage   *Usage                 `json:"usage,omitempty"`
}

// A2AError A2A错误
type A2AError struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
}

// Usage 使用量统计
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

// AgentCard Agent身份卡片
type AgentCard struct {
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	Description  string       `json:"description"`
	Version      string       `json:"version"`
	Capabilities []Capability `json:"capabilities"`
	Endpoints    []Endpoint   `json:"endpoints"`
	Auth         *AuthConfig  `json:"auth,omitempty"`
}

// Capability Agent能力
type Capability struct {
	Name         string                 `json:"name"`
	Description  string                 `json:"description"`
	InputSchema  map[string]interface{} `json:"input_schema"`
	OutputSchema map[string]interface{} `json:"output_schema,omitempty"`
}

// Endpoint 通信端点
type Endpoint struct {
	Protocol string `json:"protocol"` // a2a, mcp, rest, grpc
	URL      string `json:"url"`
	Priority int    `json:"priority"`
}

// AuthConfig 认证配置
type AuthConfig struct {
	Type      string `json:"type"` // none, api_key, jwt, oauth
	APIKey    string `json:"api_key,omitempty"`
	JWTSecret string `json:"jwt_secret,omitempty"`
}

// CapabilityHandler 能力处理器
type CapabilityHandler func(ctx context.Context, input map[string]interface{}) (*A2AResponse, error)

// A2AServer A2A服务器
type A2AServer struct {
	card     *AgentCard
	handlers map[string]CapabilityHandler
	mu       sync.RWMutex
}

// NewA2AServer 创建A2A服务器
func NewA2AServer(card *AgentCard) *A2AServer {
	return &A2AServer{
		card:     card,
		handlers: make(map[string]CapabilityHandler),
	}
}

// RegisterHandler 注册能力处理器
func (s *A2AServer) RegisterHandler(capability string, handler CapabilityHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handlers[capability] = handler
}

// HandleRequest 处理请求
func (s *A2AServer) HandleRequest(ctx context.Context, msg *A2AMessage) (*A2AMessage, error) {
	// 解析请求
	reqData, err := json.Marshal(msg.Payload)
	if err != nil {
		return s.errorResponse(msg, "invalid_request", "Invalid request payload")
	}

	var req A2ARequest
	if err := json.Unmarshal(reqData, &req); err != nil {
		return s.errorResponse(msg, "invalid_request", "Invalid request format")
	}

	// 获取处理器
	s.mu.RLock()
	handler, ok := s.handlers[req.Capability]
	s.mu.RUnlock()

	if !ok {
		return s.errorResponse(msg, "capability_not_found", "Capability not found: "+req.Capability)
	}

	// 设置超时
	if msg.Context != nil && msg.Context.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(msg.Context.Timeout)*time.Second)
		defer cancel()
	}

	// 执行处理
	resp, err := handler(ctx, req.Input)
	if err != nil {
		return s.errorResponse(msg, "handler_error", err.Error())
	}

	return &A2AMessage{
		ID:        uuid.New().String(),
		Type:      A2AMessageResponse,
		From:      s.card.ID,
		To:        msg.From,
		Timestamp: time.Now(),
		Payload:   resp,
		Context:   msg.Context,
	}, nil
}

func (s *A2AServer) errorResponse(msg *A2AMessage, code, message string) (*A2AMessage, error) {
	return &A2AMessage{
		ID:        uuid.New().String(),
		Type:      A2AMessageError,
		From:      s.card.ID,
		To:        msg.From,
		Timestamp: time.Now(),
		Payload: &A2AResponse{
			Success: false,
			Error: &A2AError{
				Code:    code,
				Message: message,
			},
		},
		Context: msg.Context,
	}, nil
}

// GetCard 获取Agent卡片
func (s *A2AServer) GetCard() *AgentCard {
	return s.card
}

// ServeHTTP HTTP处理器
func (s *A2AServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var msg A2AMessage
	if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	resp, err := s.HandleRequest(r.Context(), &msg)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// A2AClient A2A客户端
type A2AClient struct {
	httpClient *http.Client
	cards      map[string]*AgentCard
	mu         sync.RWMutex
}

// NewA2AClient 创建A2A客户端
func NewA2AClient() *A2AClient {
	return &A2AClient{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		cards:      make(map[string]*AgentCard),
	}
}

// RegisterAgent 注册外部Agent
func (c *A2AClient) RegisterAgent(card *AgentCard) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cards[card.ID] = card
}

// Call 调用外部Agent
func (c *A2AClient) Call(ctx context.Context, agentID, capability string, input map[string]interface{}) (*A2AResponse, error) {
	c.mu.RLock()
	card, ok := c.cards[agentID]
	c.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("unknown agent: %s", agentID)
	}

	// 选择A2A端点
	var endpoint *Endpoint
	for i := range card.Endpoints {
		if card.Endpoints[i].Protocol == "a2a" {
			endpoint = &card.Endpoints[i]
			break
		}
	}

	if endpoint == nil {
		return nil, fmt.Errorf("no a2a endpoint for agent: %s", agentID)
	}

	// 构造请求
	msg := &A2AMessage{
		ID:        uuid.New().String(),
		Type:      A2AMessageRequest,
		From:      "kernel-expert", // 本地Agent ID
		To:        agentID,
		Timestamp: time.Now(),
		Payload: &A2ARequest{
			Capability: capability,
			Input:      input,
		},
		Context: &A2AContext{
			ConversationID: uuid.New().String(),
		},
	}

	// 发送请求
	return c.sendRequest(ctx, endpoint.URL, msg)
}

func (c *A2AClient) sendRequest(ctx context.Context, url string, msg *A2AMessage) (*A2AResponse, error) {
	data, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Body = newReadCloser(data)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var respMsg A2AMessage
	if err := json.NewDecoder(resp.Body).Decode(&respMsg); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	// 解析响应payload
	payloadData, err := json.Marshal(respMsg.Payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}

	var a2aResp A2AResponse
	if err := json.Unmarshal(payloadData, &a2aResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return &a2aResp, nil
}

// DiscoverAgents 发现支持指定能力的Agent
func (c *A2AClient) DiscoverAgents(capability string) []*AgentCard {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var result []*AgentCard
	for _, card := range c.cards {
		for _, cap := range card.Capabilities {
			if cap.Name == capability {
				result = append(result, card)
				break
			}
		}
	}
	return result
}

type readCloser struct {
	data []byte
	pos  int
}

func newReadCloser(data []byte) *readCloser {
	return &readCloser{data: data}
}

func (r *readCloser) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, nil
	}
	n = copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

func (r *readCloser) Close() error {
	return nil
}
