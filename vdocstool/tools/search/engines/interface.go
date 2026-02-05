// Package engines 搜索引擎适配器
package engines

import (
	"context"
)

// SearchResult 搜索结果
type SearchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
	Content string `json:"content,omitempty"`
}

// SearchRequest 搜索请求
type SearchRequest struct {
	Query      string `json:"query"`
	MaxResults int    `json:"max_results"`
	Language   string `json:"language,omitempty"`
	Region     string `json:"region,omitempty"`
}

// Engine 搜索引擎接口
type Engine interface {
	// Name 返回引擎名称
	Name() string

	// Search 执行搜索
	Search(ctx context.Context, req *SearchRequest) ([]*SearchResult, error)

	// HealthCheck 健康检查
	HealthCheck(ctx context.Context) error

	// Priority 返回优先级（数字越小优先级越高）
	Priority() int
}

// EngineStatus 引擎状态
type EngineStatus int

const (
	StatusHealthy  EngineStatus = iota // 健康
	StatusDegraded                     // 降级（部分可用）
	StatusUnhealthy                    // 不健康
	StatusCircuitOpen                  // 熔断
)

func (s EngineStatus) String() string {
	switch s {
	case StatusHealthy:
		return "healthy"
	case StatusDegraded:
		return "degraded"
	case StatusUnhealthy:
		return "unhealthy"
	case StatusCircuitOpen:
		return "circuit_open"
	default:
		return "unknown"
	}
}

// EngineMetrics 引擎指标
type EngineMetrics struct {
	TotalRequests   int64   `json:"total_requests"`
	SuccessRequests int64   `json:"success_requests"`
	FailedRequests  int64   `json:"failed_requests"`
	AvgLatencyMs    float64 `json:"avg_latency_ms"`
	LastLatencyMs   float64 `json:"last_latency_ms"`
	ErrorRate       float64 `json:"error_rate"`
}

// EngineInfo 引擎信息
type EngineInfo struct {
	Name     string        `json:"name"`
	Status   EngineStatus  `json:"status"`
	Priority int           `json:"priority"`
	Metrics  EngineMetrics `json:"metrics"`
}
