// Package interaction provides agent interaction capabilities.
package interaction

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// Protocol represents the interaction protocol.
type Protocol string

const (
	ProtocolHTTP      Protocol = "http"
	ProtocolGRPC      Protocol = "grpc"
	ProtocolWebSocket Protocol = "websocket"
	ProtocolA2A       Protocol = "a2a" // Agent-to-Agent protocol
)

// Message represents an interaction message.
type Message struct {
	ID        string                 `json:"id"`
	From      string                 `json:"from"`
	To        string                 `json:"to"`
	Type      string                 `json:"type"`
	Content   interface{}            `json:"content"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
	ReplyTo   string                 `json:"reply_to,omitempty"`
}

// AgentInfo represents external agent information.
type AgentInfo struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Endpoint    string            `json:"endpoint"`
	Protocol    Protocol          `json:"protocol"`
	Capabilities []string         `json:"capabilities"`
	APIKey      string            `json:"api_key,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
	Timeout     time.Duration     `json:"timeout"`
	Enabled     bool              `json:"enabled"`
}

// InteractionConfig represents interaction configuration.
type InteractionConfig struct {
	LocalAgentID   string                `json:"local_agent_id"`
	LocalAgentName string                `json:"local_agent_name"`
	Agents         map[string]*AgentInfo `json:"agents"`
	DefaultTimeout time.Duration         `json:"default_timeout"`
	RetryCount     int                   `json:"retry_count"`
	RetryDelay     time.Duration         `json:"retry_delay"`
}

// Manager manages agent interactions.
type Manager struct {
	config    *InteractionConfig
	client    *http.Client
	agents    map[string]*AgentInfo
	handlers  map[string]MessageHandler
	mu        sync.RWMutex
}

// MessageHandler handles incoming messages.
type MessageHandler func(ctx context.Context, msg *Message) (*Message, error)

// NewManager creates a new interaction manager.
func NewManager(config *InteractionConfig) *Manager {
	if config.DefaultTimeout == 0 {
		config.DefaultTimeout = 30 * time.Second
	}
	if config.RetryCount == 0 {
		config.RetryCount = 3
	}
	if config.RetryDelay == 0 {
		config.RetryDelay = time.Second
	}
	
	return &Manager{
		config: config,
		client: &http.Client{
			Timeout: config.DefaultTimeout,
		},
		agents:   config.Agents,
		handlers: make(map[string]MessageHandler),
	}
}

// RegisterAgent registers an external agent.
func (m *Manager) RegisterAgent(info *AgentInfo) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.agents[info.ID] = info
}

// UnregisterAgent unregisters an external agent.
func (m *Manager) UnregisterAgent(agentID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.agents, agentID)
}

// RegisterHandler registers a message handler.
func (m *Manager) RegisterHandler(messageType string, handler MessageHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.handlers[messageType] = handler
}

// GetAgent returns agent info by ID.
func (m *Manager) GetAgent(agentID string) (*AgentInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	if agent, exists := m.agents[agentID]; exists {
		return agent, nil
	}
	return nil, fmt.Errorf("agent not found: %s", agentID)
}

// ListAgents returns all registered agents.
func (m *Manager) ListAgents() []*AgentInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	var agents []*AgentInfo
	for _, agent := range m.agents {
		agents = append(agents, agent)
	}
	return agents
}

// SendMessage sends a message to an external agent.
func (m *Manager) SendMessage(ctx context.Context, msg *Message) (*Message, error) {
	agent, err := m.GetAgent(msg.To)
	if err != nil {
		return nil, err
	}
	
	if !agent.Enabled {
		return nil, fmt.Errorf("agent %s is disabled", msg.To)
	}
	
	msg.From = m.config.LocalAgentID
	msg.Timestamp = time.Now()
	
	switch agent.Protocol {
	case ProtocolHTTP:
		return m.sendHTTP(ctx, agent, msg)
	case ProtocolA2A:
		return m.sendA2A(ctx, agent, msg)
	default:
		return nil, fmt.Errorf("unsupported protocol: %s", agent.Protocol)
	}
}

func (m *Manager) sendHTTP(ctx context.Context, agent *AgentInfo, msg *Message) (*Message, error) {
	data, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}
	
	var lastErr error
	for i := 0; i <= m.config.RetryCount; i++ {
		if i > 0 {
			time.Sleep(m.config.RetryDelay)
		}
		
		req, err := http.NewRequestWithContext(ctx, "POST", agent.Endpoint, bytes.NewReader(data))
		if err != nil {
			lastErr = err
			continue
		}
		
		req.Header.Set("Content-Type", "application/json")
		if agent.APIKey != "" {
			req.Header.Set("Authorization", "Bearer "+agent.APIKey)
		}
		for k, v := range agent.Headers {
			req.Header.Set(k, v)
		}
		
		resp, err := m.client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		defer resp.Body.Close()
		
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			lastErr = fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
			continue
		}
		
		var reply Message
		if err := json.NewDecoder(resp.Body).Decode(&reply); err != nil {
			lastErr = err
			continue
		}
		
		return &reply, nil
	}
	
	return nil, fmt.Errorf("all retries failed: %w", lastErr)
}

// A2A Protocol (Agent-to-Agent) implementation
// Reference: Google's A2A protocol concept

// A2ARequest represents an A2A protocol request.
type A2ARequest struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
	ID      string      `json:"id"`
}

// A2AResponse represents an A2A protocol response.
type A2AResponse struct {
	JSONRPC string       `json:"jsonrpc"`
	Result  interface{}  `json:"result,omitempty"`
	Error   *A2AError    `json:"error,omitempty"`
	ID      string       `json:"id"`
}

// A2AError represents an A2A protocol error.
type A2AError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func (m *Manager) sendA2A(ctx context.Context, agent *AgentInfo, msg *Message) (*Message, error) {
	a2aReq := &A2ARequest{
		JSONRPC: "2.0",
		Method:  msg.Type,
		Params:  msg.Content,
		ID:      msg.ID,
	}
	
	data, err := json.Marshal(a2aReq)
	if err != nil {
		return nil, err
	}
	
	var lastErr error
	for i := 0; i <= m.config.RetryCount; i++ {
		if i > 0 {
			time.Sleep(m.config.RetryDelay)
		}
		
		req, err := http.NewRequestWithContext(ctx, "POST", agent.Endpoint, bytes.NewReader(data))
		if err != nil {
			lastErr = err
			continue
		}
		
		req.Header.Set("Content-Type", "application/json")
		if agent.APIKey != "" {
			req.Header.Set("Authorization", "Bearer "+agent.APIKey)
		}
		
		resp, err := m.client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		defer resp.Body.Close()
		
		var a2aResp A2AResponse
		if err := json.NewDecoder(resp.Body).Decode(&a2aResp); err != nil {
			lastErr = err
			continue
		}
		
		if a2aResp.Error != nil {
			return nil, fmt.Errorf("A2A error %d: %s", a2aResp.Error.Code, a2aResp.Error.Message)
		}
		
		return &Message{
			ID:        a2aResp.ID,
			From:      agent.ID,
			To:        m.config.LocalAgentID,
			Type:      "response",
			Content:   a2aResp.Result,
			Timestamp: time.Now(),
			ReplyTo:   msg.ID,
		}, nil
	}
	
	return nil, fmt.Errorf("all retries failed: %w", lastErr)
}

// HandleMessage handles an incoming message.
func (m *Manager) HandleMessage(ctx context.Context, msg *Message) (*Message, error) {
	m.mu.RLock()
	handler, exists := m.handlers[msg.Type]
	m.mu.RUnlock()
	
	if !exists {
		return nil, fmt.Errorf("no handler for message type: %s", msg.Type)
	}
	
	return handler(ctx, msg)
}

// Query sends a query to an external agent and waits for response.
func (m *Manager) Query(ctx context.Context, agentID string, query string) (interface{}, error) {
	msg := &Message{
		ID:      generateID(),
		To:      agentID,
		Type:    "query",
		Content: query,
	}
	
	resp, err := m.SendMessage(ctx, msg)
	if err != nil {
		return nil, err
	}
	
	return resp.Content, nil
}

// Delegate delegates a task to an external agent.
func (m *Manager) Delegate(ctx context.Context, agentID string, task interface{}) (interface{}, error) {
	msg := &Message{
		ID:      generateID(),
		To:      agentID,
		Type:    "delegate",
		Content: task,
	}
	
	resp, err := m.SendMessage(ctx, msg)
	if err != nil {
		return nil, err
	}
	
	return resp.Content, nil
}

// Broadcast sends a message to multiple agents.
func (m *Manager) Broadcast(ctx context.Context, agentIDs []string, msg *Message) ([]*Message, []error) {
	var wg sync.WaitGroup
	responses := make([]*Message, len(agentIDs))
	errs := make([]error, len(agentIDs))
	
	for i, agentID := range agentIDs {
		wg.Add(1)
		go func(idx int, id string) {
			defer wg.Done()
			
			msgCopy := *msg
			msgCopy.To = id
			
			resp, err := m.SendMessage(ctx, &msgCopy)
			responses[idx] = resp
			errs[idx] = err
		}(i, agentID)
	}
	
	wg.Wait()
	return responses, errs
}

// DiscoverAgents discovers agents from a registry endpoint.
func (m *Manager) DiscoverAgents(ctx context.Context, registryURL string) ([]*AgentInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", registryURL+"/agents", nil)
	if err != nil {
		return nil, err
	}
	
	resp, err := m.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("discovery failed: HTTP %d", resp.StatusCode)
	}
	
	var agents []*AgentInfo
	if err := json.NewDecoder(resp.Body).Decode(&agents); err != nil {
		return nil, err
	}
	
	return agents, nil
}

// HealthCheck checks if an agent is healthy.
func (m *Manager) HealthCheck(ctx context.Context, agentID string) error {
	agent, err := m.GetAgent(agentID)
	if err != nil {
		return err
	}
	
	msg := &Message{
		ID:      generateID(),
		To:      agentID,
		Type:    "health_check",
		Content: nil,
	}
	
	_, err = m.sendHTTP(ctx, agent, msg)
	return err
}

func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// DefaultConfig returns a default interaction configuration.
func DefaultConfig() *InteractionConfig {
	return &InteractionConfig{
		LocalAgentID:   "insurance-expert",
		LocalAgentName: "保险专家Agent",
		Agents:         make(map[string]*AgentInfo),
		DefaultTimeout: 30 * time.Second,
		RetryCount:     3,
		RetryDelay:     time.Second,
	}
}

// ValidateMessage validates a message.
func ValidateMessage(msg *Message) error {
	if msg == nil {
		return errors.New("message is nil")
	}
	if msg.ID == "" {
		return errors.New("message ID is required")
	}
	if msg.To == "" {
		return errors.New("message recipient is required")
	}
	if msg.Type == "" {
		return errors.New("message type is required")
	}
	return nil
}
