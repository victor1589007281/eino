// Package rest Memory REST API 处理器
package rest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Handler REST API 处理器
type Handler struct {
	memoryService MemoryService
	authService   *AuthService
	rateLimiter   *RateLimiter
}

// MemoryService 内存服务接口
type MemoryService interface {
	Store(ctx context.Context, req *StoreRequest) (*StoreResponse, error)
	Retrieve(ctx context.Context, req *RetrieveRequest) (*RetrieveResponse, error)
	BatchStore(ctx context.Context, req *BatchStoreRequest) (*BatchStoreResponse, error)
	BatchRetrieve(ctx context.Context, req *BatchRetrieveRequest) (*BatchRetrieveResponse, error)
	GetSession(ctx context.Context, sessionID string) (*SessionInfo, error)
	DeleteSession(ctx context.Context, sessionID string) error
	SwitchTopic(ctx context.Context, req *SwitchTopicRequest) (*SwitchTopicResponse, error)
	RecallTopic(ctx context.Context, req *RecallTopicRequest) (*RecallTopicResponse, error)
	ListTopics(ctx context.Context, sessionID string) ([]*TopicInfo, error)
	GetEntityRelations(ctx context.Context, entityName string, depth int) ([]*Relation, error)
	Summarize(ctx context.Context, sessionID string) (*SummaryResponse, error)
	Archive(ctx context.Context, req *ArchiveRequest) (*ArchiveResponse, error)
	GetStats(ctx context.Context) (*Stats, error)
}

// NewHandler 创建 REST 处理器
func NewHandler(service MemoryService) *Handler {
	return &Handler{
		memoryService: service,
		authService:   NewAuthService(),
		rateLimiter:   NewRateLimiter(100, 20), // 100 req/min, burst 20
	}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	// Memory 路由
	mux.HandleFunc("/api/v1/memory/store", h.middleware(h.handleStore))
	mux.HandleFunc("/api/v1/memory/retrieve", h.middleware(h.handleRetrieve))
	mux.HandleFunc("/api/v1/memory/batch/store", h.middleware(h.handleBatchStore))
	mux.HandleFunc("/api/v1/memory/batch/retrieve", h.middleware(h.handleBatchRetrieve))
	mux.HandleFunc("/api/v1/memory/session/", h.middleware(h.handleSession))
	mux.HandleFunc("/api/v1/memory/topic/switch", h.middleware(h.handleSwitchTopic))
	mux.HandleFunc("/api/v1/memory/topic/recall", h.middleware(h.handleRecallTopic))
	mux.HandleFunc("/api/v1/memory/topics/", h.middleware(h.handleListTopics))
	mux.HandleFunc("/api/v1/memory/entities/", h.middleware(h.handleGetEntities))
	mux.HandleFunc("/api/v1/memory/summarize", h.middleware(h.handleSummarize))
	mux.HandleFunc("/api/v1/memory/archive", h.middleware(h.handleArchive))
	mux.HandleFunc("/api/v1/memory/stats", h.middleware(h.handleGetStats))
	mux.HandleFunc("/api/v1/health", h.handleHealth)
}

// middleware 中间件链
func (h *Handler) middleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 设置通用响应头
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Request-ID", uuid.New().String())

		// CORS
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// 认证
		if !h.authService.Authenticate(r) {
			h.writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid or missing API key")
			return
		}

		// 限流
		if !h.rateLimiter.Allow(r.RemoteAddr) {
			h.writeError(w, http.StatusTooManyRequests, "RATE_LIMITED", "Too many requests")
			return
		}

		next(w, r)
	}
}

// handleStore 处理存储请求
func (h *Handler) handleStore(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		h.writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only POST is allowed")
		return
	}

	var req StoreRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid JSON: "+err.Error())
		return
	}

	if req.SessionID == "" || req.Message.Content == "" {
		h.writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "session_id and message.content are required")
		return
	}

	resp, err := h.memoryService.Store(r.Context(), &req)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	h.writeSuccess(w, resp)
}

// handleRetrieve 处理检索请求
func (h *Handler) handleRetrieve(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		h.writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only POST is allowed")
		return
	}

	var req RetrieveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid JSON: "+err.Error())
		return
	}

	if req.SessionID == "" || req.Query == "" {
		h.writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "session_id and query are required")
		return
	}

	resp, err := h.memoryService.Retrieve(r.Context(), &req)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	h.writeSuccess(w, resp)
}

// handleBatchStore 处理批量存储请求
func (h *Handler) handleBatchStore(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		h.writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only POST is allowed")
		return
	}

	var req BatchStoreRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid JSON: "+err.Error())
		return
	}

	resp, err := h.memoryService.BatchStore(r.Context(), &req)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	h.writeSuccess(w, resp)
}

// handleBatchRetrieve 处理批量检索请求
func (h *Handler) handleBatchRetrieve(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		h.writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only POST is allowed")
		return
	}

	var req BatchRetrieveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid JSON: "+err.Error())
		return
	}

	resp, err := h.memoryService.BatchRetrieve(r.Context(), &req)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	h.writeSuccess(w, resp)
}

// handleSession 处理会话请求 (GET/DELETE)
func (h *Handler) handleSession(w http.ResponseWriter, r *http.Request) {
	sessionID := strings.TrimPrefix(r.URL.Path, "/api/v1/memory/session/")
	if sessionID == "" {
		h.writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "session_id is required")
		return
	}

	switch r.Method {
	case "GET":
		info, err := h.memoryService.GetSession(r.Context(), sessionID)
		if err != nil {
			h.writeError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
			return
		}
		h.writeSuccess(w, info)

	case "DELETE":
		if err := h.memoryService.DeleteSession(r.Context(), sessionID); err != nil {
			h.writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
			return
		}
		h.writeSuccess(w, map[string]bool{"deleted": true})

	default:
		h.writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET and DELETE are allowed")
	}
}

// handleSwitchTopic 处理切换主题请求
func (h *Handler) handleSwitchTopic(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		h.writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only POST is allowed")
		return
	}

	var req SwitchTopicRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid JSON: "+err.Error())
		return
	}

	resp, err := h.memoryService.SwitchTopic(r.Context(), &req)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	h.writeSuccess(w, resp)
}

// handleRecallTopic 处理召回主题请求
func (h *Handler) handleRecallTopic(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		h.writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only POST is allowed")
		return
	}

	var req RecallTopicRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid JSON: "+err.Error())
		return
	}

	resp, err := h.memoryService.RecallTopic(r.Context(), &req)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	h.writeSuccess(w, resp)
}

// handleListTopics 处理列出主题请求
func (h *Handler) handleListTopics(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		h.writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET is allowed")
		return
	}

	sessionID := strings.TrimPrefix(r.URL.Path, "/api/v1/memory/topics/")
	if sessionID == "" {
		h.writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "session_id is required")
		return
	}

	topics, err := h.memoryService.ListTopics(r.Context(), sessionID)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	h.writeSuccess(w, map[string]interface{}{
		"session_id": sessionID,
		"topics":     topics,
	})
}

// handleGetEntities 处理获取实体关系请求
func (h *Handler) handleGetEntities(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		h.writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET is allowed")
		return
	}

	entityName := strings.TrimPrefix(r.URL.Path, "/api/v1/memory/entities/")
	if entityName == "" {
		h.writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "entity_name is required")
		return
	}

	depth := 2
	if d := r.URL.Query().Get("depth"); d != "" {
		fmt.Sscanf(d, "%d", &depth)
	}

	relations, err := h.memoryService.GetEntityRelations(r.Context(), entityName, depth)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	h.writeSuccess(w, map[string]interface{}{
		"entity_name": entityName,
		"depth":       depth,
		"relations":   relations,
	})
}

// handleSummarize 处理生成摘要请求
func (h *Handler) handleSummarize(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		h.writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only POST is allowed")
		return
	}

	var req struct {
		SessionID string `json:"session_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid JSON: "+err.Error())
		return
	}

	resp, err := h.memoryService.Summarize(r.Context(), req.SessionID)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	h.writeSuccess(w, resp)
}

// handleArchive 处理归档请求
func (h *Handler) handleArchive(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		h.writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only POST is allowed")
		return
	}

	var req ArchiveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid JSON: "+err.Error())
		return
	}

	resp, err := h.memoryService.Archive(r.Context(), &req)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	h.writeSuccess(w, resp)
}

// handleGetStats 处理获取统计请求
func (h *Handler) handleGetStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		h.writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET is allowed")
		return
	}

	stats, err := h.memoryService.GetStats(r.Context())
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	h.writeSuccess(w, stats)
}

// handleHealth 健康检查
func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "healthy",
		"time":   time.Now().Format(time.RFC3339),
	})
}

// writeSuccess 写入成功响应
func (h *Handler) writeSuccess(w http.ResponseWriter, data interface{}) {
	resp := APIResponse{
		Success: true,
		Data:    data,
		Meta: ResponseMeta{
			RequestID: w.Header().Get("X-Request-ID"),
			Timestamp: time.Now().Format(time.RFC3339),
		},
	}
	json.NewEncoder(w).Encode(resp)
}

// writeError 写入错误响应
func (h *Handler) writeError(w http.ResponseWriter, status int, code, message string) {
	w.WriteHeader(status)
	resp := APIResponse{
		Success: false,
		Error: &APIError{
			Code:    code,
			Message: message,
		},
		Meta: ResponseMeta{
			RequestID: w.Header().Get("X-Request-ID"),
			Timestamp: time.Now().Format(time.RFC3339),
		},
	}
	json.NewEncoder(w).Encode(resp)
}

// APIResponse API 响应结构
type APIResponse struct {
	Success bool         `json:"success"`
	Data    interface{}  `json:"data,omitempty"`
	Error   *APIError    `json:"error,omitempty"`
	Meta    ResponseMeta `json:"meta"`
}

// APIError API 错误
type APIError struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
}

// ResponseMeta 响应元数据
type ResponseMeta struct {
	RequestID string `json:"request_id"`
	Timestamp string `json:"timestamp"`
	LatencyMs int64  `json:"latency_ms,omitempty"`
}

// AuthService 认证服务
type AuthService struct {
	apiKeys map[string]string // key -> scope
	mu      sync.RWMutex
}

// NewAuthService 创建认证服务
func NewAuthService() *AuthService {
	return &AuthService{
		apiKeys: map[string]string{
			"mem_dev_test123": "admin", // 开发测试 key
		},
	}
}

// Authenticate 认证请求
func (s *AuthService) Authenticate(r *http.Request) bool {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		// 开发模式允许无认证
		return true
	}

	// Bearer token 格式
	if strings.HasPrefix(auth, "Bearer ") {
		token := strings.TrimPrefix(auth, "Bearer ")
		s.mu.RLock()
		_, ok := s.apiKeys[token]
		s.mu.RUnlock()
		return ok
	}

	return false
}

// AddAPIKey 添加 API Key
func (s *AuthService) AddAPIKey(key, scope string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.apiKeys[key] = scope
}

// RateLimiter 限流器
type RateLimiter struct {
	requests map[string]*rateLimitEntry
	limit    int
	burst    int
	mu       sync.RWMutex
}

type rateLimitEntry struct {
	count     int
	resetTime time.Time
}

// NewRateLimiter 创建限流器
func NewRateLimiter(limit, burst int) *RateLimiter {
	rl := &RateLimiter{
		requests: make(map[string]*rateLimitEntry),
		limit:    limit,
		burst:    burst,
	}
	go rl.cleanup()
	return rl
}

// Allow 检查是否允许请求
func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	entry, ok := rl.requests[key]

	if !ok || now.After(entry.resetTime) {
		rl.requests[key] = &rateLimitEntry{
			count:     1,
			resetTime: now.Add(time.Minute),
		}
		return true
	}

	if entry.count >= rl.limit+rl.burst {
		return false
	}

	entry.count++
	return true
}

// cleanup 清理过期条目
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for key, entry := range rl.requests {
			if now.After(entry.resetTime) {
				delete(rl.requests, key)
			}
		}
		rl.mu.Unlock()
	}
}
