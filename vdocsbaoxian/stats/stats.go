// Package stats provides statistics and metrics collection.
package stats

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// Stats represents the statistics collector.
type Stats struct {
	token *TokenStats
	cache *CacheStats
	agent *AgentStats
	
	startTime time.Time
	mu        sync.RWMutex
}

// TokenStats represents token usage statistics.
type TokenStats struct {
	TotalInputTokens   int64 `json:"total_input_tokens"`
	TotalOutputTokens  int64 `json:"total_output_tokens"`
	TotalTokens        int64 `json:"total_tokens"`
	SessionInputTokens int64 `json:"session_input_tokens"`
	SessionOutputTokens int64 `json:"session_output_tokens"`
	DailyBudget        int64 `json:"daily_budget"`
	DailyUsed          int64 `json:"daily_used"`
	LastReset          time.Time `json:"last_reset"`
	
	// Per-provider stats
	ProviderStats map[string]*ProviderTokenStats `json:"provider_stats"`
	mu            sync.RWMutex
}

// ProviderTokenStats represents token stats per provider.
type ProviderTokenStats struct {
	InputTokens  int64 `json:"input_tokens"`
	OutputTokens int64 `json:"output_tokens"`
	TotalTokens  int64 `json:"total_tokens"`
	RequestCount int64 `json:"request_count"`
	ErrorCount   int64 `json:"error_count"`
}

// CacheStats represents cache statistics.
type CacheStats struct {
	L1Hits      int64 `json:"l1_hits"`
	L1Misses    int64 `json:"l1_misses"`
	L2Hits      int64 `json:"l2_hits"`
	L2Misses    int64 `json:"l2_misses"`
	L3Hits      int64 `json:"l3_hits"`
	L3Misses    int64 `json:"l3_misses"`
	TotalHits   int64 `json:"total_hits"`
	TotalMisses int64 `json:"total_misses"`
	
	// Per-cache-type stats
	IndexCacheStats  *CacheTypeStats `json:"index_cache_stats"`
	QueryCacheStats  *CacheTypeStats `json:"query_cache_stats"`
	VerifyCacheStats *CacheTypeStats `json:"verify_cache_stats"`
}

// CacheTypeStats represents stats for a specific cache type.
type CacheTypeStats struct {
	Hits       int64 `json:"hits"`
	Misses     int64 `json:"misses"`
	Sets       int64 `json:"sets"`
	Deletes    int64 `json:"deletes"`
	Evictions  int64 `json:"evictions"`
	SizeBytes  int64 `json:"size_bytes"`
	ItemCount  int64 `json:"item_count"`
}

// AgentStats represents agent execution statistics.
type AgentStats struct {
	TotalQueries     int64         `json:"total_queries"`
	SuccessfulQueries int64        `json:"successful_queries"`
	FailedQueries    int64         `json:"failed_queries"`
	TotalDuration    time.Duration `json:"total_duration"`
	AvgDuration      time.Duration `json:"avg_duration"`
	
	// Per-agent stats
	SubAgentStats map[string]*SubAgentStats `json:"sub_agent_stats"`
	mu            sync.RWMutex
}

// SubAgentStats represents stats for a sub-agent.
type SubAgentStats struct {
	Name          string        `json:"name"`
	Invocations   int64         `json:"invocations"`
	Successes     int64         `json:"successes"`
	Failures      int64         `json:"failures"`
	TotalDuration time.Duration `json:"total_duration"`
	AvgDuration   time.Duration `json:"avg_duration"`
}

// NewStats creates a new stats collector.
func NewStats() *Stats {
	return &Stats{
		token: &TokenStats{
			ProviderStats: make(map[string]*ProviderTokenStats),
			LastReset:     time.Now(),
		},
		cache: &CacheStats{
			IndexCacheStats:  &CacheTypeStats{},
			QueryCacheStats:  &CacheTypeStats{},
			VerifyCacheStats: &CacheTypeStats{},
		},
		agent: &AgentStats{
			SubAgentStats: make(map[string]*SubAgentStats),
		},
		startTime: time.Now(),
	}
}

// RecordTokenUsage records token usage.
func (s *Stats) RecordTokenUsage(provider string, inputTokens, outputTokens int64) {
	s.token.mu.Lock()
	defer s.token.mu.Unlock()

	atomic.AddInt64(&s.token.TotalInputTokens, inputTokens)
	atomic.AddInt64(&s.token.TotalOutputTokens, outputTokens)
	atomic.AddInt64(&s.token.TotalTokens, inputTokens+outputTokens)
	atomic.AddInt64(&s.token.SessionInputTokens, inputTokens)
	atomic.AddInt64(&s.token.SessionOutputTokens, outputTokens)
	atomic.AddInt64(&s.token.DailyUsed, inputTokens+outputTokens)

	if _, ok := s.token.ProviderStats[provider]; !ok {
		s.token.ProviderStats[provider] = &ProviderTokenStats{}
	}
	
	ps := s.token.ProviderStats[provider]
	atomic.AddInt64(&ps.InputTokens, inputTokens)
	atomic.AddInt64(&ps.OutputTokens, outputTokens)
	atomic.AddInt64(&ps.TotalTokens, inputTokens+outputTokens)
	atomic.AddInt64(&ps.RequestCount, 1)
}

// RecordProviderError records a provider error.
func (s *Stats) RecordProviderError(provider string) {
	s.token.mu.Lock()
	defer s.token.mu.Unlock()

	if _, ok := s.token.ProviderStats[provider]; !ok {
		s.token.ProviderStats[provider] = &ProviderTokenStats{}
	}
	atomic.AddInt64(&s.token.ProviderStats[provider].ErrorCount, 1)
}

// RecordCacheHit records a cache hit.
func (s *Stats) RecordCacheHit(level int, cacheType string) {
	switch level {
	case 1:
		atomic.AddInt64(&s.cache.L1Hits, 1)
	case 2:
		atomic.AddInt64(&s.cache.L2Hits, 1)
	case 3:
		atomic.AddInt64(&s.cache.L3Hits, 1)
	}
	atomic.AddInt64(&s.cache.TotalHits, 1)

	s.recordCacheTypeHit(cacheType)
}

// RecordCacheMiss records a cache miss.
func (s *Stats) RecordCacheMiss(level int, cacheType string) {
	switch level {
	case 1:
		atomic.AddInt64(&s.cache.L1Misses, 1)
	case 2:
		atomic.AddInt64(&s.cache.L2Misses, 1)
	case 3:
		atomic.AddInt64(&s.cache.L3Misses, 1)
	}
	atomic.AddInt64(&s.cache.TotalMisses, 1)

	s.recordCacheTypeMiss(cacheType)
}

func (s *Stats) recordCacheTypeHit(cacheType string) {
	switch cacheType {
	case "index":
		atomic.AddInt64(&s.cache.IndexCacheStats.Hits, 1)
	case "query":
		atomic.AddInt64(&s.cache.QueryCacheStats.Hits, 1)
	case "verify":
		atomic.AddInt64(&s.cache.VerifyCacheStats.Hits, 1)
	}
}

func (s *Stats) recordCacheTypeMiss(cacheType string) {
	switch cacheType {
	case "index":
		atomic.AddInt64(&s.cache.IndexCacheStats.Misses, 1)
	case "query":
		atomic.AddInt64(&s.cache.QueryCacheStats.Misses, 1)
	case "verify":
		atomic.AddInt64(&s.cache.VerifyCacheStats.Misses, 1)
	}
}

// RecordQuery records a query execution.
func (s *Stats) RecordQuery(success bool, duration time.Duration) {
	s.agent.mu.Lock()
	defer s.agent.mu.Unlock()

	atomic.AddInt64(&s.agent.TotalQueries, 1)
	if success {
		atomic.AddInt64(&s.agent.SuccessfulQueries, 1)
	} else {
		atomic.AddInt64(&s.agent.FailedQueries, 1)
	}
	
	s.agent.TotalDuration += duration
	if s.agent.TotalQueries > 0 {
		s.agent.AvgDuration = s.agent.TotalDuration / time.Duration(s.agent.TotalQueries)
	}
}

// RecordSubAgentExecution records a sub-agent execution.
func (s *Stats) RecordSubAgentExecution(agentName string, success bool, duration time.Duration) {
	s.agent.mu.Lock()
	defer s.agent.mu.Unlock()

	if _, ok := s.agent.SubAgentStats[agentName]; !ok {
		s.agent.SubAgentStats[agentName] = &SubAgentStats{Name: agentName}
	}

	as := s.agent.SubAgentStats[agentName]
	atomic.AddInt64(&as.Invocations, 1)
	if success {
		atomic.AddInt64(&as.Successes, 1)
	} else {
		atomic.AddInt64(&as.Failures, 1)
	}
	
	as.TotalDuration += duration
	if as.Invocations > 0 {
		as.AvgDuration = as.TotalDuration / time.Duration(as.Invocations)
	}
}

// GetTokenStats returns token statistics.
func (s *Stats) GetTokenStats() *TokenStats {
	s.token.mu.RLock()
	defer s.token.mu.RUnlock()
	
	// Create a copy
	stats := &TokenStats{
		TotalInputTokens:    s.token.TotalInputTokens,
		TotalOutputTokens:   s.token.TotalOutputTokens,
		TotalTokens:         s.token.TotalTokens,
		SessionInputTokens:  s.token.SessionInputTokens,
		SessionOutputTokens: s.token.SessionOutputTokens,
		DailyBudget:         s.token.DailyBudget,
		DailyUsed:           s.token.DailyUsed,
		LastReset:           s.token.LastReset,
		ProviderStats:       make(map[string]*ProviderTokenStats),
	}
	
	for k, v := range s.token.ProviderStats {
		stats.ProviderStats[k] = &ProviderTokenStats{
			InputTokens:  v.InputTokens,
			OutputTokens: v.OutputTokens,
			TotalTokens:  v.TotalTokens,
			RequestCount: v.RequestCount,
			ErrorCount:   v.ErrorCount,
		}
	}
	
	return stats
}

// GetCacheStats returns cache statistics.
func (s *Stats) GetCacheStats() *CacheStats {
	return &CacheStats{
		L1Hits:      atomic.LoadInt64(&s.cache.L1Hits),
		L1Misses:    atomic.LoadInt64(&s.cache.L1Misses),
		L2Hits:      atomic.LoadInt64(&s.cache.L2Hits),
		L2Misses:    atomic.LoadInt64(&s.cache.L2Misses),
		L3Hits:      atomic.LoadInt64(&s.cache.L3Hits),
		L3Misses:    atomic.LoadInt64(&s.cache.L3Misses),
		TotalHits:   atomic.LoadInt64(&s.cache.TotalHits),
		TotalMisses: atomic.LoadInt64(&s.cache.TotalMisses),
		IndexCacheStats: &CacheTypeStats{
			Hits:   atomic.LoadInt64(&s.cache.IndexCacheStats.Hits),
			Misses: atomic.LoadInt64(&s.cache.IndexCacheStats.Misses),
		},
		QueryCacheStats: &CacheTypeStats{
			Hits:   atomic.LoadInt64(&s.cache.QueryCacheStats.Hits),
			Misses: atomic.LoadInt64(&s.cache.QueryCacheStats.Misses),
		},
		VerifyCacheStats: &CacheTypeStats{
			Hits:   atomic.LoadInt64(&s.cache.VerifyCacheStats.Hits),
			Misses: atomic.LoadInt64(&s.cache.VerifyCacheStats.Misses),
		},
	}
}

// GetAgentStats returns agent statistics.
func (s *Stats) GetAgentStats() *AgentStats {
	s.agent.mu.RLock()
	defer s.agent.mu.RUnlock()
	
	stats := &AgentStats{
		TotalQueries:      s.agent.TotalQueries,
		SuccessfulQueries: s.agent.SuccessfulQueries,
		FailedQueries:     s.agent.FailedQueries,
		TotalDuration:     s.agent.TotalDuration,
		AvgDuration:       s.agent.AvgDuration,
		SubAgentStats:     make(map[string]*SubAgentStats),
	}
	
	for k, v := range s.agent.SubAgentStats {
		stats.SubAgentStats[k] = &SubAgentStats{
			Name:          v.Name,
			Invocations:   v.Invocations,
			Successes:     v.Successes,
			Failures:      v.Failures,
			TotalDuration: v.TotalDuration,
			AvgDuration:   v.AvgDuration,
		}
	}
	
	return stats
}

// GetCacheHitRate returns the overall cache hit rate.
func (s *Stats) GetCacheHitRate() float64 {
	hits := atomic.LoadInt64(&s.cache.TotalHits)
	misses := atomic.LoadInt64(&s.cache.TotalMisses)
	total := hits + misses
	if total == 0 {
		return 0
	}
	return float64(hits) / float64(total)
}

// GetTokenBudgetUsage returns the token budget usage percentage.
func (s *Stats) GetTokenBudgetUsage() float64 {
	if s.token.DailyBudget == 0 {
		return 0
	}
	return float64(s.token.DailyUsed) / float64(s.token.DailyBudget)
}

// SetDailyBudget sets the daily token budget.
func (s *Stats) SetDailyBudget(budget int64) {
	s.token.mu.Lock()
	defer s.token.mu.Unlock()
	s.token.DailyBudget = budget
}

// ResetDaily resets daily statistics.
func (s *Stats) ResetDaily() {
	s.token.mu.Lock()
	defer s.token.mu.Unlock()
	s.token.DailyUsed = 0
	s.token.LastReset = time.Now()
}

// ResetSession resets session statistics.
func (s *Stats) ResetSession() {
	s.token.mu.Lock()
	defer s.token.mu.Unlock()
	s.token.SessionInputTokens = 0
	s.token.SessionOutputTokens = 0
}

// GetUptime returns the uptime duration.
func (s *Stats) GetUptime() time.Duration {
	return time.Since(s.startTime)
}

// Summary represents a stats summary.
type Summary struct {
	Uptime           time.Duration         `json:"uptime"`
	TokenStats       *TokenStats           `json:"token_stats"`
	CacheStats       *CacheStats           `json:"cache_stats"`
	CacheHitRate     float64               `json:"cache_hit_rate"`
	AgentStats       *AgentStats           `json:"agent_stats"`
	TokenBudgetUsage float64               `json:"token_budget_usage"`
}

// GetSummary returns a summary of all statistics.
func (s *Stats) GetSummary() *Summary {
	return &Summary{
		Uptime:           s.GetUptime(),
		TokenStats:       s.GetTokenStats(),
		CacheStats:       s.GetCacheStats(),
		CacheHitRate:     s.GetCacheHitRate(),
		AgentStats:       s.GetAgentStats(),
		TokenBudgetUsage: s.GetTokenBudgetUsage(),
	}
}

// StartDailyResetWorker starts a worker to reset daily stats.
func (s *Stats) StartDailyResetWorker(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		
		for {
			select {
			case <-ctx.Done():
				return
			case t := <-ticker.C:
				// Reset at midnight
				if t.Hour() == 0 && t.Minute() == 0 {
					s.ResetDaily()
				}
			}
		}
	}()
}
