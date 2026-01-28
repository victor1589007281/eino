// Package interaction 提供Agent交互协议
package interaction

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
)

// MCPTool MCP工具定义
type MCPTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

// MCPResource MCP资源定义
type MCPResource struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
}

// MCPPrompt MCP提示词定义
type MCPPrompt struct {
	Name        string     `json:"name"`
	Description string     `json:"description,omitempty"`
	Arguments   []Argument `json:"arguments,omitempty"`
}

// Argument 参数定义
type Argument struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

// MCPToolHandler 工具处理器
type MCPToolHandler func(ctx context.Context, args map[string]interface{}) (interface{}, error)

// MCPResourceHandler 资源处理器
type MCPResourceHandler func(ctx context.Context, uri string) ([]byte, string, error)

// MCPServer MCP服务器
type MCPServer struct {
	name    string
	version string

	tools        map[string]*MCPTool
	toolHandlers map[string]MCPToolHandler

	resources        map[string]*MCPResource
	resourceHandlers map[string]MCPResourceHandler

	prompts map[string]*MCPPrompt

	mu sync.RWMutex
}

// NewMCPServer 创建MCP服务器
func NewMCPServer(name, version string) *MCPServer {
	return &MCPServer{
		name:             name,
		version:          version,
		tools:            make(map[string]*MCPTool),
		toolHandlers:     make(map[string]MCPToolHandler),
		resources:        make(map[string]*MCPResource),
		resourceHandlers: make(map[string]MCPResourceHandler),
		prompts:          make(map[string]*MCPPrompt),
	}
}

// RegisterTool 注册工具
func (s *MCPServer) RegisterTool(tool *MCPTool, handler MCPToolHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tools[tool.Name] = tool
	s.toolHandlers[tool.Name] = handler
}

// RegisterResource 注册资源
func (s *MCPServer) RegisterResource(resource *MCPResource, handler MCPResourceHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.resources[resource.URI] = resource
	s.resourceHandlers[resource.URI] = handler
}

// RegisterPrompt 注册提示词
func (s *MCPServer) RegisterPrompt(prompt *MCPPrompt) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prompts[prompt.Name] = prompt
}

// GetTools 获取工具列表
func (s *MCPServer) GetTools() []*MCPTool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*MCPTool, 0, len(s.tools))
	for _, tool := range s.tools {
		result = append(result, tool)
	}
	return result
}

// GetResources 获取资源列表
func (s *MCPServer) GetResources() []*MCPResource {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*MCPResource, 0, len(s.resources))
	for _, resource := range s.resources {
		result = append(result, resource)
	}
	return result
}

// GetPrompts 获取提示词列表
func (s *MCPServer) GetPrompts() []*MCPPrompt {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*MCPPrompt, 0, len(s.prompts))
	for _, prompt := range s.prompts {
		result = append(result, prompt)
	}
	return result
}

// CallTool 调用工具
func (s *MCPServer) CallTool(ctx context.Context, name string, args map[string]interface{}) (interface{}, error) {
	s.mu.RLock()
	handler, ok := s.toolHandlers[name]
	s.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("unknown tool: %s", name)
	}

	return handler(ctx, args)
}

// ReadResource 读取资源
func (s *MCPServer) ReadResource(ctx context.Context, uri string) ([]byte, string, error) {
	s.mu.RLock()
	handler, ok := s.resourceHandlers[uri]
	s.mu.RUnlock()

	if !ok {
		return nil, "", fmt.Errorf("unknown resource: %s", uri)
	}

	return handler(ctx, uri)
}

// MCPRequest MCP请求
type MCPRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// MCPResponse MCP响应
type MCPResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *MCPError   `json:"error,omitempty"`
}

// MCPError MCP错误
type MCPError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// ServeHTTP HTTP处理器
func (s *MCPServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req MCPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, nil, -32700, "Parse error")
		return
	}

	resp := s.handleRequest(r.Context(), &req)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *MCPServer) handleRequest(ctx context.Context, req *MCPRequest) *MCPResponse {
	switch req.Method {
	case "initialize":
		return s.handleInitialize(req)
	case "tools/list":
		return s.handleListTools(req)
	case "tools/call":
		return s.handleCallTool(ctx, req)
	case "resources/list":
		return s.handleListResources(req)
	case "resources/read":
		return s.handleReadResource(ctx, req)
	case "prompts/list":
		return s.handleListPrompts(req)
	default:
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &MCPError{
				Code:    -32601,
				Message: "Method not found",
			},
		}
	}
}

func (s *MCPServer) handleInitialize(req *MCPRequest) *MCPResponse {
	return &MCPResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"serverInfo": map[string]string{
				"name":    s.name,
				"version": s.version,
			},
			"capabilities": map[string]interface{}{
				"tools":     map[string]interface{}{},
				"resources": map[string]interface{}{},
				"prompts":   map[string]interface{}{},
			},
		},
	}
}

func (s *MCPServer) handleListTools(req *MCPRequest) *MCPResponse {
	return &MCPResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]interface{}{
			"tools": s.GetTools(),
		},
	}
}

func (s *MCPServer) handleCallTool(ctx context.Context, req *MCPRequest) *MCPResponse {
	params, ok := req.Params.(map[string]interface{})
	if !ok {
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &MCPError{
				Code:    -32602,
				Message: "Invalid params",
			},
		}
	}

	name, _ := params["name"].(string)
	args, _ := params["arguments"].(map[string]interface{})

	result, err := s.CallTool(ctx, name, args)
	if err != nil {
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &MCPError{
				Code:    -32000,
				Message: err.Error(),
			},
		}
	}

	return &MCPResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]interface{}{
			"content": []map[string]interface{}{
				{
					"type": "text",
					"text": result,
				},
			},
		},
	}
}

func (s *MCPServer) handleListResources(req *MCPRequest) *MCPResponse {
	return &MCPResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]interface{}{
			"resources": s.GetResources(),
		},
	}
}

func (s *MCPServer) handleReadResource(ctx context.Context, req *MCPRequest) *MCPResponse {
	params, ok := req.Params.(map[string]interface{})
	if !ok {
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &MCPError{
				Code:    -32602,
				Message: "Invalid params",
			},
		}
	}

	uri, _ := params["uri"].(string)

	data, mimeType, err := s.ReadResource(ctx, uri)
	if err != nil {
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &MCPError{
				Code:    -32000,
				Message: err.Error(),
			},
		}
	}

	return &MCPResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]interface{}{
			"contents": []map[string]interface{}{
				{
					"uri":      uri,
					"mimeType": mimeType,
					"text":     string(data),
				},
			},
		},
	}
}

func (s *MCPServer) handleListPrompts(req *MCPRequest) *MCPResponse {
	return &MCPResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]interface{}{
			"prompts": s.GetPrompts(),
		},
	}
}

func (s *MCPServer) writeError(w http.ResponseWriter, id interface{}, code int, message string) {
	resp := &MCPResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &MCPError{
			Code:    code,
			Message: message,
		},
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
