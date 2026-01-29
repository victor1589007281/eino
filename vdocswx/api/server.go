// Package api 提供HTTP API服务
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/cloudwego/eino/vdocswx/agent"
	"github.com/cloudwego/eino/vdocswx/agent/master"
	"github.com/cloudwego/eino/vdocswx/config"
	"github.com/cloudwego/eino/vdocswx/stats"
	"github.com/cloudwego/eino/vdocswx/storage"
)

// Server HTTP服务器
type Server struct {
	cfg         *config.Config
	masterAgent *master.MasterAgent
	storage     *storage.SQLiteStorage
	stats       *stats.Collector
	httpServer  *http.Server
}

// NewServer 创建服务器
func NewServer(cfg *config.Config, masterAgent *master.MasterAgent, storage *storage.SQLiteStorage, statsCollector *stats.Collector) *Server {
	return &Server{
		cfg:         cfg,
		masterAgent: masterAgent,
		storage:     storage,
		stats:       statsCollector,
	}
}

// Start 启动服务器
func (s *Server) Start(addr string) error {
	mux := http.NewServeMux()

	// 健康检查
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/ready", s.handleReady)

	// API路由
	mux.HandleFunc("/api/v1/polish", s.handlePolish)
	mux.HandleFunc("/api/v1/chat", s.handleChat)
	mux.HandleFunc("/api/v1/upload", s.handleUpload)
	mux.HandleFunc("/api/v1/stats", s.handleStats)
	mux.HandleFunc("/api/v1/history", s.handleHistory)

	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:      s.withMiddleware(mux),
		ReadTimeout:  s.cfg.Server.ReadTimeout,
		WriteTimeout: s.cfg.Server.WriteTimeout,
		IdleTimeout:  s.cfg.Server.IdleTimeout,
	}

	return s.httpServer.ListenAndServe()
}

// Shutdown 关闭服务器
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

// withMiddleware 添加中间件
func (s *Server) withMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// CORS
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// 记录请求
		start := time.Now()
		next.ServeHTTP(w, r)
		duration := time.Since(start)

		// 日志
		fmt.Printf("[%s] %s %s %v\n", time.Now().Format(time.RFC3339), r.Method, r.URL.Path, duration)
	})
}

// handleHealth 健康检查
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.jsonResponse(w, http.StatusOK, map[string]string{"status": "healthy"})
}

// handleReady 就绪检查
func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	s.jsonResponse(w, http.StatusOK, map[string]string{"status": "ready"})
}

// PolishRequest 润色请求
type PolishRequest struct {
	Content     string `json:"content"`
	Title       string `json:"title,omitempty"`
	Type        string `json:"type,omitempty"`
	UserContext string `json:"user_context,omitempty"`
}

// PolishResponse 润色响应
type PolishResponse struct {
	PolishedContent string             `json:"polished_content"`
	Changes         []agent.Change     `json:"changes"`
	Suggestions     []agent.Suggestion `json:"suggestions"`
	Statistics      agent.Statistics   `json:"statistics"`
}

// handlePolish 处理润色请求
func (s *Server) handlePolish(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.errorResponse(w, http.StatusMethodNotAllowed, "只支持POST方法")
		return
	}

	var req PolishRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.errorResponse(w, http.StatusBadRequest, "无效的请求体")
		return
	}

	if req.Content == "" {
		s.errorResponse(w, http.StatusBadRequest, "内容不能为空")
		return
	}

	// 创建输入
	input := &agent.ArticleInput{
		Content:     req.Content,
		Title:       req.Title,
		Type:        agent.ArticleType(req.Type),
		UserContext: req.UserContext,
	}

	if input.Type == "" {
		input.Type = agent.ArticleTypeUnknown
	}

	// 执行润色
	ctx := r.Context()
	result, err := s.masterAgent.Polish(ctx, input)
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, fmt.Sprintf("润色失败: %v", err))
		return
	}

	// 返回结果
	response := PolishResponse{
		PolishedContent: result.PolishedContent,
		Changes:         result.Changes,
		Suggestions:     result.Suggestions,
		Statistics:      result.Statistics,
	}

	s.jsonResponse(w, http.StatusOK, response)
}

// ChatRequest 对话请求
type ChatRequest struct {
	SessionID string `json:"session_id"`
	Message   string `json:"message"`
	Draft     string `json:"draft,omitempty"` // 当前草稿（首次对话时提供）
}

// ChatResponse 对话响应
type ChatResponse struct {
	Message       string             `json:"message"`
	SessionID     string             `json:"session_id"`
	Modifications []agent.Change     `json:"modifications,omitempty"`
	Suggestions   []agent.Suggestion `json:"suggestions,omitempty"`
}

// handleChat 处理对话请求
func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.errorResponse(w, http.StatusMethodNotAllowed, "只支持POST方法")
		return
	}

	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.errorResponse(w, http.StatusBadRequest, "无效的请求体")
		return
	}

	if req.Message == "" {
		s.errorResponse(w, http.StatusBadRequest, "消息不能为空")
		return
	}

	// 生成会话ID
	if req.SessionID == "" {
		req.SessionID = fmt.Sprintf("session_%d", time.Now().UnixNano())
	}

	// 如果提供了草稿，设置到会话
	if req.Draft != "" {
		s.masterAgent.SetSessionDraft(req.SessionID, req.Draft)
	}

	// 执行对话
	ctx := r.Context()
	chatResp, err := s.masterAgent.Chat(ctx, req.SessionID, req.Message)
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, fmt.Sprintf("对话失败: %v", err))
		return
	}

	response := ChatResponse{
		Message:       chatResp.Message,
		SessionID:     chatResp.SessionID,
		Modifications: chatResp.Modifications,
		Suggestions:   chatResp.Suggestions,
	}

	s.jsonResponse(w, http.StatusOK, response)
}

// handleUpload 处理文件上传
func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.errorResponse(w, http.StatusMethodNotAllowed, "只支持POST方法")
		return
	}

	// 限制文件大小
	r.Body = http.MaxBytesReader(w, r.Body, int64(s.cfg.Input.MaxFileSizeMB)*1024*1024)

	// 解析multipart表单
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		s.errorResponse(w, http.StatusBadRequest, "文件过大或格式错误")
		return
	}

	// 获取上传的文件
	file, header, err := r.FormFile("file")
	if err != nil {
		s.errorResponse(w, http.StatusBadRequest, "获取文件失败")
		return
	}
	defer file.Close()

	// 读取文件内容
	content, err := io.ReadAll(file)
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, "读取文件失败")
		return
	}

	// 自动润色（如果请求中指定）
	autoPolish := r.FormValue("auto_polish") == "true"

	response := map[string]interface{}{
		"filename": header.Filename,
		"size":     len(content),
		"content":  string(content),
	}

	if autoPolish {
		input := &agent.ArticleInput{
			Content:  string(content),
			FilePath: header.Filename,
			Type:     agent.ArticleTypeUnknown,
		}

		result, err := s.masterAgent.Polish(r.Context(), input)
		if err != nil {
			response["polish_error"] = err.Error()
		} else {
			response["polished_content"] = result.PolishedContent
			response["changes"] = result.Changes
			response["statistics"] = result.Statistics
		}
	}

	s.jsonResponse(w, http.StatusOK, response)
}

// handleStats 处理统计请求
func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.errorResponse(w, http.StatusMethodNotAllowed, "只支持GET方法")
		return
	}

	totalTokens, llmTokens := s.stats.GetTokenStats()
	hits, misses, evictions := s.stats.GetCacheStats()

	// 计算缓存命中率
	cacheHitRates := make(map[string]float64)
	for name, h := range hits {
		m := misses[name]
		total := h + m
		if total > 0 {
			cacheHitRates[name] = float64(h) / float64(total) * 100
		}
	}

	response := map[string]interface{}{
		"tokens": map[string]interface{}{
			"total":    totalTokens,
			"by_model": llmTokens,
		},
		"cache": map[string]interface{}{
			"hits":      hits,
			"misses":    misses,
			"evictions": evictions,
			"hit_rates": cacheHitRates,
		},
	}

	s.jsonResponse(w, http.StatusOK, response)
}

// handleHistory 处理历史记录请求
func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.errorResponse(w, http.StatusMethodNotAllowed, "只支持GET方法")
		return
	}

	sessionID := r.URL.Query().Get("session_id")
	if sessionID == "" {
		s.errorResponse(w, http.StatusBadRequest, "需要session_id参数")
		return
	}

	history, err := s.masterAgent.GetHistory(r.Context(), sessionID)
	if err != nil {
		s.errorResponse(w, http.StatusNotFound, fmt.Sprintf("会话不存在: %v", err))
		return
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"session_id": sessionID,
		"history":    history,
	})
}

// jsonResponse 返回JSON响应
func (s *Server) jsonResponse(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// errorResponse 返回错误响应
func (s *Server) errorResponse(w http.ResponseWriter, status int, message string) {
	s.jsonResponse(w, status, map[string]string{"error": message})
}
