// Package stats provides statistics collection and reporting for the MySQL Expert Agent.
package stats

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// Collector collects and reports statistics.
type Collector struct {
	tokenStats     *TokenStatsCollector
	cacheStats     *CacheStatsCollector
	requestStats   *RequestStatsCollector
	
	mu             sync.RWMutex
	enabled        bool
	reportInterval time.Duration
	stopCh         chan struct{}
	exporters      []Exporter
}

// CollectorConfig contains configuration for the stats collector.
type CollectorConfig struct {
	Enabled        bool
	ReportInterval time.Duration
	Exporters      []Exporter
}

// NewCollector creates a new statistics collector.
func NewCollector(config *CollectorConfig) *Collector {
	return &Collector{
		tokenStats:     NewTokenStatsCollector(),
		cacheStats:     NewCacheStatsCollector(),
		requestStats:   NewRequestStatsCollector(),
		enabled:        config.Enabled,
		reportInterval: config.ReportInterval,
		stopCh:         make(chan struct{}),
		exporters:      config.Exporters,
	}
}

// Start starts the statistics collector.
func (c *Collector) Start(ctx context.Context) {
	if !c.enabled {
		return
	}

	go c.reportLoop(ctx)
}

// Stop stops the statistics collector.
func (c *Collector) Stop() {
	close(c.stopCh)
}

func (c *Collector) reportLoop(ctx context.Context) {
	ticker := time.NewTicker(c.reportInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.stopCh:
			return
		case <-ticker.C:
			c.export()
		}
	}
}

func (c *Collector) export() {
	summary := c.Summary()
	for _, exporter := range c.exporters {
		exporter.Export(summary)
	}
}

// TokenStats returns the token statistics collector.
func (c *Collector) TokenStats() *TokenStatsCollector {
	return c.tokenStats
}

// CacheStats returns the cache statistics collector.
func (c *Collector) CacheStats() *CacheStatsCollector {
	return c.cacheStats
}

// RequestStats returns the request statistics collector.
func (c *Collector) RequestStats() *RequestStatsCollector {
	return c.requestStats
}

// Summary returns a summary of all statistics.
func (c *Collector) Summary() *StatsSummary {
	return &StatsSummary{
		Timestamp:    time.Now(),
		Token:        c.tokenStats.Summary(),
		Cache:        c.cacheStats.Summary(),
		Request:      c.requestStats.Summary(),
	}
}

// StatsSummary contains a summary of all statistics.
type StatsSummary struct {
	Timestamp time.Time            `json:"timestamp"`
	Token     *TokenStatsSummary   `json:"token"`
	Cache     *CacheStatsSummary   `json:"cache"`
	Request   *RequestStatsSummary `json:"request"`
}

// TokenStatsCollector collects token usage statistics.
type TokenStatsCollector struct {
	mu sync.RWMutex
	
	totalPromptTokens     int64
	totalCompletionTokens int64
	totalCost             float64
	
	byModel    map[string]*ModelTokenStats
	byAgent    map[string]*AgentTokenStats
	
	dailyBudget    int64
	dailyUsed      int64
	budgetResetAt  time.Time
}

// ModelTokenStats contains token stats for a specific model.
type ModelTokenStats struct {
	Model            string  `json:"model"`
	PromptTokens     int64   `json:"prompt_tokens"`
	CompletionTokens int64   `json:"completion_tokens"`
	TotalTokens      int64   `json:"total_tokens"`
	Cost             float64 `json:"cost"`
	RequestCount     int64   `json:"request_count"`
}

// AgentTokenStats contains token stats for a specific agent.
type AgentTokenStats struct {
	Agent            string  `json:"agent"`
	PromptTokens     int64   `json:"prompt_tokens"`
	CompletionTokens int64   `json:"completion_tokens"`
	TotalTokens      int64   `json:"total_tokens"`
	Cost             float64 `json:"cost"`
	RequestCount     int64   `json:"request_count"`
}

// NewTokenStatsCollector creates a new token stats collector.
func NewTokenStatsCollector() *TokenStatsCollector {
	return &TokenStatsCollector{
		byModel:       make(map[string]*ModelTokenStats),
		byAgent:       make(map[string]*AgentTokenStats),
		budgetResetAt: nextMidnight(),
	}
}

// RecordUsage records token usage.
func (c *TokenStatsCollector) RecordUsage(model, agent string, promptTokens, completionTokens int, cost float64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if we need to reset daily budget
	if time.Now().After(c.budgetResetAt) {
		c.dailyUsed = 0
		c.budgetResetAt = nextMidnight()
	}

	atomic.AddInt64(&c.totalPromptTokens, int64(promptTokens))
	atomic.AddInt64(&c.totalCompletionTokens, int64(completionTokens))
	c.totalCost += cost
	c.dailyUsed += int64(promptTokens + completionTokens)

	// Update model stats
	if _, ok := c.byModel[model]; !ok {
		c.byModel[model] = &ModelTokenStats{Model: model}
	}
	c.byModel[model].PromptTokens += int64(promptTokens)
	c.byModel[model].CompletionTokens += int64(completionTokens)
	c.byModel[model].TotalTokens += int64(promptTokens + completionTokens)
	c.byModel[model].Cost += cost
	c.byModel[model].RequestCount++

	// Update agent stats
	if _, ok := c.byAgent[agent]; !ok {
		c.byAgent[agent] = &AgentTokenStats{Agent: agent}
	}
	c.byAgent[agent].PromptTokens += int64(promptTokens)
	c.byAgent[agent].CompletionTokens += int64(completionTokens)
	c.byAgent[agent].TotalTokens += int64(promptTokens + completionTokens)
	c.byAgent[agent].Cost += cost
	c.byAgent[agent].RequestCount++
}

// SetDailyBudget sets the daily token budget.
func (c *TokenStatsCollector) SetDailyBudget(budget int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.dailyBudget = budget
}

// RemainingBudget returns the remaining daily budget.
func (c *TokenStatsCollector) RemainingBudget() int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.dailyBudget - c.dailyUsed
}

// TokenStatsSummary contains a summary of token statistics.
type TokenStatsSummary struct {
	TotalPromptTokens     int64              `json:"total_prompt_tokens"`
	TotalCompletionTokens int64              `json:"total_completion_tokens"`
	TotalTokens           int64              `json:"total_tokens"`
	TotalCost             float64            `json:"total_cost"`
	DailyBudget           int64              `json:"daily_budget"`
	DailyUsed             int64              `json:"daily_used"`
	DailyRemaining        int64              `json:"daily_remaining"`
	ByModel               []*ModelTokenStats `json:"by_model"`
	ByAgent               []*AgentTokenStats `json:"by_agent"`
}

// Summary returns a summary of token statistics.
func (c *TokenStatsCollector) Summary() *TokenStatsSummary {
	c.mu.RLock()
	defer c.mu.RUnlock()

	summary := &TokenStatsSummary{
		TotalPromptTokens:     c.totalPromptTokens,
		TotalCompletionTokens: c.totalCompletionTokens,
		TotalTokens:           c.totalPromptTokens + c.totalCompletionTokens,
		TotalCost:             c.totalCost,
		DailyBudget:           c.dailyBudget,
		DailyUsed:             c.dailyUsed,
		DailyRemaining:        c.dailyBudget - c.dailyUsed,
		ByModel:               make([]*ModelTokenStats, 0, len(c.byModel)),
		ByAgent:               make([]*AgentTokenStats, 0, len(c.byAgent)),
	}

	for _, stats := range c.byModel {
		summary.ByModel = append(summary.ByModel, stats)
	}
	for _, stats := range c.byAgent {
		summary.ByAgent = append(summary.ByAgent, stats)
	}

	return summary
}

// CacheStatsCollector collects cache statistics.
type CacheStatsCollector struct {
	mu     sync.RWMutex
	caches map[string]*CacheMetrics
}

// CacheMetrics contains metrics for a specific cache.
type CacheMetrics struct {
	Name       string    `json:"name"`
	Hits       int64     `json:"hits"`
	Misses     int64     `json:"misses"`
	Size       int64     `json:"size"`
	Evictions  int64     `json:"evictions"`
	HitRate    float64   `json:"hit_rate"`
	LastAccess time.Time `json:"last_access"`
}

// NewCacheStatsCollector creates a new cache stats collector.
func NewCacheStatsCollector() *CacheStatsCollector {
	return &CacheStatsCollector{
		caches: make(map[string]*CacheMetrics),
	}
}

// RecordHit records a cache hit.
func (c *CacheStatsCollector) RecordHit(cacheName string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, ok := c.caches[cacheName]; !ok {
		c.caches[cacheName] = &CacheMetrics{Name: cacheName}
	}
	c.caches[cacheName].Hits++
	c.caches[cacheName].LastAccess = time.Now()
	c.updateHitRate(cacheName)
}

// RecordMiss records a cache miss.
func (c *CacheStatsCollector) RecordMiss(cacheName string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, ok := c.caches[cacheName]; !ok {
		c.caches[cacheName] = &CacheMetrics{Name: cacheName}
	}
	c.caches[cacheName].Misses++
	c.caches[cacheName].LastAccess = time.Now()
	c.updateHitRate(cacheName)
}

// RecordEviction records a cache eviction.
func (c *CacheStatsCollector) RecordEviction(cacheName string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, ok := c.caches[cacheName]; !ok {
		c.caches[cacheName] = &CacheMetrics{Name: cacheName}
	}
	c.caches[cacheName].Evictions++
}

// UpdateSize updates the cache size.
func (c *CacheStatsCollector) UpdateSize(cacheName string, size int64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, ok := c.caches[cacheName]; !ok {
		c.caches[cacheName] = &CacheMetrics{Name: cacheName}
	}
	c.caches[cacheName].Size = size
}

func (c *CacheStatsCollector) updateHitRate(cacheName string) {
	cache := c.caches[cacheName]
	total := cache.Hits + cache.Misses
	if total > 0 {
		cache.HitRate = float64(cache.Hits) / float64(total)
	}
}

// GetHitRate returns the hit rate for a cache.
func (c *CacheStatsCollector) GetHitRate(cacheName string) float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if cache, ok := c.caches[cacheName]; ok {
		return cache.HitRate
	}
	return 0
}

// CacheStatsSummary contains a summary of cache statistics.
type CacheStatsSummary struct {
	TotalHits      int64           `json:"total_hits"`
	TotalMisses    int64           `json:"total_misses"`
	OverallHitRate float64         `json:"overall_hit_rate"`
	Caches         []*CacheMetrics `json:"caches"`
}

// Summary returns a summary of cache statistics.
func (c *CacheStatsCollector) Summary() *CacheStatsSummary {
	c.mu.RLock()
	defer c.mu.RUnlock()

	summary := &CacheStatsSummary{
		Caches: make([]*CacheMetrics, 0, len(c.caches)),
	}

	for _, metrics := range c.caches {
		summary.TotalHits += metrics.Hits
		summary.TotalMisses += metrics.Misses
		// Make a copy
		m := *metrics
		summary.Caches = append(summary.Caches, &m)
	}

	total := summary.TotalHits + summary.TotalMisses
	if total > 0 {
		summary.OverallHitRate = float64(summary.TotalHits) / float64(total)
	}

	return summary
}

// RequestStatsCollector collects request statistics.
type RequestStatsCollector struct {
	mu sync.RWMutex

	totalRequests   int64
	successRequests int64
	failedRequests  int64

	totalLatency time.Duration
	maxLatency   time.Duration
	minLatency   time.Duration

	byEndpoint map[string]*EndpointStats
}

// EndpointStats contains stats for a specific endpoint.
type EndpointStats struct {
	Endpoint       string        `json:"endpoint"`
	Requests       int64         `json:"requests"`
	Successes      int64         `json:"successes"`
	Failures       int64         `json:"failures"`
	TotalLatency   time.Duration `json:"total_latency"`
	AverageLatency time.Duration `json:"average_latency"`
	MaxLatency     time.Duration `json:"max_latency"`
}

// NewRequestStatsCollector creates a new request stats collector.
func NewRequestStatsCollector() *RequestStatsCollector {
	return &RequestStatsCollector{
		byEndpoint: make(map[string]*EndpointStats),
		minLatency: time.Hour, // Start with a large value
	}
}

// RecordRequest records a request.
func (c *RequestStatsCollector) RecordRequest(endpoint string, latency time.Duration, success bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.totalRequests++
	c.totalLatency += latency

	if latency > c.maxLatency {
		c.maxLatency = latency
	}
	if latency < c.minLatency {
		c.minLatency = latency
	}

	if success {
		c.successRequests++
	} else {
		c.failedRequests++
	}

	// Update endpoint stats
	if _, ok := c.byEndpoint[endpoint]; !ok {
		c.byEndpoint[endpoint] = &EndpointStats{Endpoint: endpoint}
	}
	stats := c.byEndpoint[endpoint]
	stats.Requests++
	stats.TotalLatency += latency
	if success {
		stats.Successes++
	} else {
		stats.Failures++
	}
	if latency > stats.MaxLatency {
		stats.MaxLatency = latency
	}
	if stats.Requests > 0 {
		stats.AverageLatency = stats.TotalLatency / time.Duration(stats.Requests)
	}
}

// RequestStatsSummary contains a summary of request statistics.
type RequestStatsSummary struct {
	TotalRequests   int64            `json:"total_requests"`
	SuccessRequests int64            `json:"success_requests"`
	FailedRequests  int64            `json:"failed_requests"`
	SuccessRate     float64          `json:"success_rate"`
	AverageLatency  time.Duration    `json:"average_latency"`
	MaxLatency      time.Duration    `json:"max_latency"`
	MinLatency      time.Duration    `json:"min_latency"`
	ByEndpoint      []*EndpointStats `json:"by_endpoint"`
}

// Summary returns a summary of request statistics.
func (c *RequestStatsCollector) Summary() *RequestStatsSummary {
	c.mu.RLock()
	defer c.mu.RUnlock()

	summary := &RequestStatsSummary{
		TotalRequests:   c.totalRequests,
		SuccessRequests: c.successRequests,
		FailedRequests:  c.failedRequests,
		MaxLatency:      c.maxLatency,
		MinLatency:      c.minLatency,
		ByEndpoint:      make([]*EndpointStats, 0, len(c.byEndpoint)),
	}

	if c.totalRequests > 0 {
		summary.SuccessRate = float64(c.successRequests) / float64(c.totalRequests)
		summary.AverageLatency = c.totalLatency / time.Duration(c.totalRequests)
	}

	for _, stats := range c.byEndpoint {
		s := *stats
		summary.ByEndpoint = append(summary.ByEndpoint, &s)
	}

	return summary
}

// Exporter defines the interface for statistics exporters.
type Exporter interface {
	Export(summary *StatsSummary)
}

// LogExporter exports statistics to logs.
type LogExporter struct {
	logger func(format string, args ...interface{})
}

// NewLogExporter creates a new log exporter.
func NewLogExporter(logger func(format string, args ...interface{})) *LogExporter {
	return &LogExporter{logger: logger}
}

// Export exports statistics to logs.
func (e *LogExporter) Export(summary *StatsSummary) {
	e.logger("[Stats] Tokens: %d (prompt: %d, completion: %d), Cost: $%.4f",
		summary.Token.TotalTokens, summary.Token.TotalPromptTokens, summary.Token.TotalCompletionTokens, summary.Token.TotalCost)
	e.logger("[Stats] Cache: %.2f%% hit rate (hits: %d, misses: %d)",
		summary.Cache.OverallHitRate*100, summary.Cache.TotalHits, summary.Cache.TotalMisses)
	e.logger("[Stats] Requests: %d (success: %.2f%%, avg latency: %v)",
		summary.Request.TotalRequests, summary.Request.SuccessRate*100, summary.Request.AverageLatency)
}

// JSONExporter exports statistics as JSON.
type JSONExporter struct {
	writer func(data []byte)
}

// NewJSONExporter creates a new JSON exporter.
func NewJSONExporter(writer func(data []byte)) *JSONExporter {
	return &JSONExporter{writer: writer}
}

// Export exports statistics as JSON.
func (e *JSONExporter) Export(summary *StatsSummary) {
	data, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return
	}
	e.writer(data)
}

// PrometheusExporter exports statistics in Prometheus format.
type PrometheusExporter struct {
	mu       sync.RWMutex
	summary  *StatsSummary
	port     int
	server   *http.Server
}

// NewPrometheusExporter creates a new Prometheus exporter.
func NewPrometheusExporter(port int) *PrometheusExporter {
	return &PrometheusExporter{
		port: port,
	}
}

// Export updates the current statistics.
func (e *PrometheusExporter) Export(summary *StatsSummary) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.summary = summary
}

// Start starts the Prometheus HTTP server.
func (e *PrometheusExporter) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/metrics", e.handleMetrics)

	e.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", e.port),
		Handler: mux,
	}

	go e.server.ListenAndServe()
	return nil
}

// Stop stops the Prometheus HTTP server.
func (e *PrometheusExporter) Stop(ctx context.Context) error {
	if e.server != nil {
		return e.server.Shutdown(ctx)
	}
	return nil
}

func (e *PrometheusExporter) handleMetrics(w http.ResponseWriter, r *http.Request) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.summary == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	w.Header().Set("Content-Type", "text/plain; version=0.0.4")

	// Token metrics
	fmt.Fprintf(w, "# HELP mysql_expert_tokens_total Total number of tokens used\n")
	fmt.Fprintf(w, "# TYPE mysql_expert_tokens_total counter\n")
	fmt.Fprintf(w, "mysql_expert_tokens_total{type=\"prompt\"} %d\n", e.summary.Token.TotalPromptTokens)
	fmt.Fprintf(w, "mysql_expert_tokens_total{type=\"completion\"} %d\n", e.summary.Token.TotalCompletionTokens)

	fmt.Fprintf(w, "# HELP mysql_expert_cost_total Total cost in USD\n")
	fmt.Fprintf(w, "# TYPE mysql_expert_cost_total counter\n")
	fmt.Fprintf(w, "mysql_expert_cost_total %.6f\n", e.summary.Token.TotalCost)

	// Cache metrics
	fmt.Fprintf(w, "# HELP mysql_expert_cache_hits_total Total cache hits\n")
	fmt.Fprintf(w, "# TYPE mysql_expert_cache_hits_total counter\n")
	fmt.Fprintf(w, "mysql_expert_cache_hits_total %d\n", e.summary.Cache.TotalHits)

	fmt.Fprintf(w, "# HELP mysql_expert_cache_misses_total Total cache misses\n")
	fmt.Fprintf(w, "# TYPE mysql_expert_cache_misses_total counter\n")
	fmt.Fprintf(w, "mysql_expert_cache_misses_total %d\n", e.summary.Cache.TotalMisses)

	fmt.Fprintf(w, "# HELP mysql_expert_cache_hit_rate Cache hit rate\n")
	fmt.Fprintf(w, "# TYPE mysql_expert_cache_hit_rate gauge\n")
	fmt.Fprintf(w, "mysql_expert_cache_hit_rate %.4f\n", e.summary.Cache.OverallHitRate)

	// Request metrics
	fmt.Fprintf(w, "# HELP mysql_expert_requests_total Total number of requests\n")
	fmt.Fprintf(w, "# TYPE mysql_expert_requests_total counter\n")
	fmt.Fprintf(w, "mysql_expert_requests_total{status=\"success\"} %d\n", e.summary.Request.SuccessRequests)
	fmt.Fprintf(w, "mysql_expert_requests_total{status=\"failed\"} %d\n", e.summary.Request.FailedRequests)

	fmt.Fprintf(w, "# HELP mysql_expert_request_latency_seconds Request latency in seconds\n")
	fmt.Fprintf(w, "# TYPE mysql_expert_request_latency_seconds gauge\n")
	fmt.Fprintf(w, "mysql_expert_request_latency_seconds{type=\"avg\"} %.6f\n", e.summary.Request.AverageLatency.Seconds())
	fmt.Fprintf(w, "mysql_expert_request_latency_seconds{type=\"max\"} %.6f\n", e.summary.Request.MaxLatency.Seconds())
}

// Helper functions

func nextMidnight() time.Time {
	now := time.Now()
	return time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
}
