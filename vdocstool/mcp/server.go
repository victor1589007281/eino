// Package mcp MCP Server 封装
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
)

// Tool MCP工具定义
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

// Resource MCP资源定义
type Resource struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
}

// Prompt MCP提示词定义
type Prompt struct {
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

// ToolHandler 工具处理器
type ToolHandler func(ctx context.Context, args map[string]interface{}) (interface{}, error)

// ResourceHandler 资源处理器
type ResourceHandler func(ctx context.Context, uri string) ([]byte, string, error)

// Server MCP服务器
type Server struct {
	name    string
	version string

	tools        map[string]*Tool
	toolHandlers map[string]ToolHandler

	resources        map[string]*Resource
	resourceHandlers map[string]ResourceHandler

	prompts map[string]*Prompt

	mu sync.RWMutex
}

// NewServer 创建MCP服务器
func NewServer(name, version string) *Server {
	return &Server{
		name:             name,
		version:          version,
		tools:            make(map[string]*Tool),
		toolHandlers:     make(map[string]ToolHandler),
		resources:        make(map[string]*Resource),
		resourceHandlers: make(map[string]ResourceHandler),
		prompts:          make(map[string]*Prompt),
	}
}

// RegisterTool 注册工具
func (s *Server) RegisterTool(tool *Tool, handler ToolHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tools[tool.Name] = tool
	s.toolHandlers[tool.Name] = handler
}

// RegisterResource 注册资源
func (s *Server) RegisterResource(resource *Resource, handler ResourceHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.resources[resource.URI] = resource
	s.resourceHandlers[resource.URI] = handler
}

// RegisterPrompt 注册提示词
func (s *Server) RegisterPrompt(prompt *Prompt) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prompts[prompt.Name] = prompt
}

// GetTools 获取工具列表
func (s *Server) GetTools() []*Tool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*Tool, 0, len(s.tools))
	for _, tool := range s.tools {
		result = append(result, tool)
	}
	return result
}

// GetResources 获取资源列表
func (s *Server) GetResources() []*Resource {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*Resource, 0, len(s.resources))
	for _, resource := range s.resources {
		result = append(result, resource)
	}
	return result
}

// GetPrompts 获取提示词列表
func (s *Server) GetPrompts() []*Prompt {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*Prompt, 0, len(s.prompts))
	for _, prompt := range s.prompts {
		result = append(result, prompt)
	}
	return result
}

// CallTool 调用工具
func (s *Server) CallTool(ctx context.Context, name string, args map[string]interface{}) (interface{}, error) {
	s.mu.RLock()
	handler, ok := s.toolHandlers[name]
	s.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("unknown tool: %s", name)
	}

	return handler(ctx, args)
}

// ReadResource 读取资源
func (s *Server) ReadResource(ctx context.Context, uri string) ([]byte, string, error) {
	s.mu.RLock()
	handler, ok := s.resourceHandlers[uri]
	s.mu.RUnlock()

	if !ok {
		return nil, "", fmt.Errorf("unknown resource: %s", uri)
	}

	return handler(ctx, uri)
}

// Request MCP请求
type Request struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// Response MCP响应
type Response struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *Error      `json:"error,omitempty"`
}

// Error MCP错误
type Error struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// ServeHTTP HTTP处理器
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, nil, -32700, "Parse error")
		return
	}

	resp := s.handleRequest(r.Context(), &req)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleRequest(ctx context.Context, req *Request) *Response {
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
		return &Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &Error{
				Code:    -32601,
				Message: "Method not found",
			},
		}
	}
}

func (s *Server) handleInitialize(req *Request) *Response {
	return &Response{
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

func (s *Server) handleListTools(req *Request) *Response {
	return &Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]interface{}{
			"tools": s.GetTools(),
		},
	}
}

func (s *Server) handleCallTool(ctx context.Context, req *Request) *Response {
	params, ok := req.Params.(map[string]interface{})
	if !ok {
		return &Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &Error{
				Code:    -32602,
				Message: "Invalid params",
			},
		}
	}

	name, _ := params["name"].(string)
	args, _ := params["arguments"].(map[string]interface{})

	result, err := s.CallTool(ctx, name, args)
	if err != nil {
		return &Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &Error{
				Code:    -32000,
				Message: err.Error(),
			},
		}
	}

	// 格式化结果
	var content interface{}
	switch v := result.(type) {
	case string:
		content = []map[string]interface{}{
			{"type": "text", "text": v},
		}
	case []byte:
		content = []map[string]interface{}{
			{"type": "text", "text": string(v)},
		}
	default:
		jsonData, _ := json.Marshal(v)
		content = []map[string]interface{}{
			{"type": "text", "text": string(jsonData)},
		}
	}

	return &Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]interface{}{
			"content": content,
		},
	}
}

func (s *Server) handleListResources(req *Request) *Response {
	return &Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]interface{}{
			"resources": s.GetResources(),
		},
	}
}

func (s *Server) handleReadResource(ctx context.Context, req *Request) *Response {
	params, ok := req.Params.(map[string]interface{})
	if !ok {
		return &Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &Error{
				Code:    -32602,
				Message: "Invalid params",
			},
		}
	}

	uri, _ := params["uri"].(string)

	data, mimeType, err := s.ReadResource(ctx, uri)
	if err != nil {
		return &Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &Error{
				Code:    -32000,
				Message: err.Error(),
			},
		}
	}

	return &Response{
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

func (s *Server) handleListPrompts(req *Request) *Response {
	return &Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]interface{}{
			"prompts": s.GetPrompts(),
		},
	}
}

func (s *Server) writeError(w http.ResponseWriter, id interface{}, code int, message string) {
	resp := &Response{
		JSONRPC: "2.0",
		ID:      id,
		Error: &Error{
			Code:    code,
			Message: message,
		},
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// ToolBuilder 工具构建器辅助
type ToolBuilder struct {
	tool *Tool
}

// NewToolBuilder 创建工具构建器
func NewToolBuilder(name, description string) *ToolBuilder {
	return &ToolBuilder{
		tool: &Tool{
			Name:        name,
			Description: description,
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
				"required":   []string{},
			},
		},
	}
}

// AddProperty 添加属性
func (b *ToolBuilder) AddProperty(name, propType, description string, required bool) *ToolBuilder {
	props := b.tool.InputSchema["properties"].(map[string]interface{})
	props[name] = map[string]interface{}{
		"type":        propType,
		"description": description,
	}

	if required {
		req := b.tool.InputSchema["required"].([]string)
		b.tool.InputSchema["required"] = append(req, name)
	}

	return b
}

// AddEnumProperty 添加枚举属性
func (b *ToolBuilder) AddEnumProperty(name, description string, enum []string, required bool) *ToolBuilder {
	props := b.tool.InputSchema["properties"].(map[string]interface{})
	props[name] = map[string]interface{}{
		"type":        "string",
		"description": description,
		"enum":        enum,
	}

	if required {
		req := b.tool.InputSchema["required"].([]string)
		b.tool.InputSchema["required"] = append(req, name)
	}

	return b
}

// Build 构建工具
func (b *ToolBuilder) Build() *Tool {
	return b.tool
}
