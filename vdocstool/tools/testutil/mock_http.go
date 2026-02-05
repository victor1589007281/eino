// Package testutil 测试工具包
package testutil

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"time"
)

// MockHTTPServer 模拟 HTTP 服务器，用于 Mock 各搜索引擎
type MockHTTPServer struct {
	Server *httptest.Server
	mu     sync.RWMutex

	// 各引擎的响应配置
	responses map[string]*MockResponse

	// 请求记录
	requests []*RecordedRequest

	// 延迟配置
	latencies map[string]time.Duration

	// 错误注入
	failEngines map[string]error
}

// MockResponse 模拟响应
type MockResponse struct {
	StatusCode int
	Body       interface{}
	Headers    map[string]string
}

// RecordedRequest 记录的请求
type RecordedRequest struct {
	Method    string
	Path      string
	Query     string
	Body      string
	Timestamp time.Time
}

// NewMockHTTPServer 创建 Mock HTTP 服务器
func NewMockHTTPServer() *MockHTTPServer {
	m := &MockHTTPServer{
		responses:   make(map[string]*MockResponse),
		latencies:   make(map[string]time.Duration),
		failEngines: make(map[string]error),
	}

	m.Server = httptest.NewServer(http.HandlerFunc(m.handleRequest))
	return m
}

// handleRequest 处理请求
func (m *MockHTTPServer) handleRequest(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	// 记录请求
	m.requests = append(m.requests, &RecordedRequest{
		Method:    r.Method,
		Path:      r.URL.Path,
		Query:     r.URL.RawQuery,
		Timestamp: time.Now(),
	})
	m.mu.Unlock()

	// 根据路径确定引擎
	engine := m.detectEngine(r.URL.Path, r.URL.RawQuery)

	m.mu.RLock()
	// 检查延迟
	if latency, ok := m.latencies[engine]; ok {
		time.Sleep(latency)
	}

	// 检查错误注入
	if err, ok := m.failEngines[engine]; ok && err != nil {
		m.mu.RUnlock()
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	// 获取响应
	resp, ok := m.responses[engine]
	m.mu.RUnlock()

	if !ok {
		// 返回默认响应
		resp = m.getDefaultResponse(engine)
	}

	// 设置 Headers
	for k, v := range resp.Headers {
		w.Header().Set(k, v)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)

	if resp.Body != nil {
		json.NewEncoder(w).Encode(resp.Body)
	}
}

// detectEngine 检测引擎类型
func (m *MockHTTPServer) detectEngine(path, query string) string {
	switch {
	case strings.Contains(path, "duckduckgo") || strings.Contains(path, "ddg"):
		return "duckduckgo"
	case strings.Contains(path, "bing") || strings.Contains(query, "bing"):
		return "bing"
	case strings.Contains(path, "baidu") || strings.Contains(query, "baidu"):
		return "baidu"
	case strings.Contains(path, "serper") || strings.Contains(path, "google"):
		return "serper"
	default:
		return "default"
	}
}

// getDefaultResponse 获取默认响应
func (m *MockHTTPServer) getDefaultResponse(engine string) *MockResponse {
	return &MockResponse{
		StatusCode: http.StatusOK,
		Body:       m.generateSearchResults(engine),
	}
}

// generateSearchResults 生成搜索结果
func (m *MockHTTPServer) generateSearchResults(engine string) interface{} {
	switch engine {
	case "duckduckgo":
		return map[string]interface{}{
			"RelatedTopics": []map[string]interface{}{
				{
					"FirstURL": "https://example.com/1",
					"Text":     "DuckDuckGo Result 1 - Example description",
				},
				{
					"FirstURL": "https://example.com/2",
					"Text":     "DuckDuckGo Result 2 - Another description",
				},
			},
		}
	case "bing":
		return map[string]interface{}{
			"webPages": map[string]interface{}{
				"value": []map[string]interface{}{
					{
						"name":    "Bing Result 1",
						"url":     "https://example.com/bing/1",
						"snippet": "Bing search result description 1",
					},
					{
						"name":    "Bing Result 2",
						"url":     "https://example.com/bing/2",
						"snippet": "Bing search result description 2",
					},
				},
			},
		}
	case "baidu":
		return map[string]interface{}{
			"results": []map[string]interface{}{
				{
					"title":   "百度结果 1",
					"url":     "https://example.com/baidu/1",
					"snippet": "百度搜索结果描述 1",
				},
				{
					"title":   "百度结果 2",
					"url":     "https://example.com/baidu/2",
					"snippet": "百度搜索结果描述 2",
				},
			},
		}
	case "serper":
		return map[string]interface{}{
			"organic": []map[string]interface{}{
				{
					"title":   "Serper/Google Result 1",
					"link":    "https://example.com/serper/1",
					"snippet": "Google search result via Serper API 1",
				},
				{
					"title":   "Serper/Google Result 2",
					"link":    "https://example.com/serper/2",
					"snippet": "Google search result via Serper API 2",
				},
			},
		}
	default:
		return map[string]interface{}{
			"results": []map[string]interface{}{},
		}
	}
}

// SetResponse 设置引擎响应
func (m *MockHTTPServer) SetResponse(engine string, resp *MockResponse) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.responses[engine] = resp
}

// SetLatency 设置引擎延迟
func (m *MockHTTPServer) SetLatency(engine string, latency time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.latencies[engine] = latency
}

// SetFail 设置引擎失败
func (m *MockHTTPServer) SetFail(engine string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failEngines[engine] = err
}

// ClearFail 清除引擎失败
func (m *MockHTTPServer) ClearFail(engine string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.failEngines, engine)
}

// GetRequests 获取记录的请求
func (m *MockHTTPServer) GetRequests() []*RecordedRequest {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]*RecordedRequest{}, m.requests...)
}

// ClearRequests 清除请求记录
func (m *MockHTTPServer) ClearRequests() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.requests = nil
}

// Close 关闭服务器
func (m *MockHTTPServer) Close() {
	m.Server.Close()
}

// URL 返回服务器 URL
func (m *MockHTTPServer) URL() string {
	return m.Server.URL
}
