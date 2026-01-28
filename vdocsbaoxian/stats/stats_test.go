package stats

import (
	"testing"
	"time"
)

func TestStats_TokenUsage(t *testing.T) {
	s := NewStats()

	// Record some usage
	s.RecordTokenUsage("deepseek", 100, 50)
	s.RecordTokenUsage("deepseek", 200, 100)
	s.RecordTokenUsage("qwen", 150, 75)

	stats := s.GetTokenStats()

	if stats.TotalInputTokens != 450 {
		t.Errorf("Expected TotalInputTokens 450, got %d", stats.TotalInputTokens)
	}
	if stats.TotalOutputTokens != 225 {
		t.Errorf("Expected TotalOutputTokens 225, got %d", stats.TotalOutputTokens)
	}
	if stats.TotalTokens != 675 {
		t.Errorf("Expected TotalTokens 675, got %d", stats.TotalTokens)
	}

	// Check provider stats
	deepseekStats := stats.ProviderStats["deepseek"]
	if deepseekStats == nil {
		t.Fatal("Expected deepseek stats")
	}
	if deepseekStats.InputTokens != 300 {
		t.Errorf("Expected deepseek input 300, got %d", deepseekStats.InputTokens)
	}
	if deepseekStats.RequestCount != 2 {
		t.Errorf("Expected deepseek requests 2, got %d", deepseekStats.RequestCount)
	}
}

func TestStats_CacheHitRate(t *testing.T) {
	s := NewStats()

	// Record cache hits and misses
	s.RecordCacheHit(1, "index")
	s.RecordCacheHit(1, "index")
	s.RecordCacheHit(2, "query")
	s.RecordCacheMiss(1, "index")

	stats := s.GetCacheStats()

	if stats.L1Hits != 2 {
		t.Errorf("Expected L1Hits 2, got %d", stats.L1Hits)
	}
	if stats.L2Hits != 1 {
		t.Errorf("Expected L2Hits 1, got %d", stats.L2Hits)
	}
	if stats.L1Misses != 1 {
		t.Errorf("Expected L1Misses 1, got %d", stats.L1Misses)
	}
	if stats.TotalHits != 3 {
		t.Errorf("Expected TotalHits 3, got %d", stats.TotalHits)
	}
	if stats.TotalMisses != 1 {
		t.Errorf("Expected TotalMisses 1, got %d", stats.TotalMisses)
	}

	// Test hit rate
	hitRate := s.GetCacheHitRate()
	expectedRate := 0.75 // 3/(3+1)
	if hitRate != expectedRate {
		t.Errorf("Expected hit rate %.2f, got %.2f", expectedRate, hitRate)
	}
}

func TestStats_QueryRecording(t *testing.T) {
	s := NewStats()

	// Record queries
	s.RecordQuery(true, 100*time.Millisecond)
	s.RecordQuery(true, 200*time.Millisecond)
	s.RecordQuery(false, 50*time.Millisecond)

	stats := s.GetAgentStats()

	if stats.TotalQueries != 3 {
		t.Errorf("Expected TotalQueries 3, got %d", stats.TotalQueries)
	}
	if stats.SuccessfulQueries != 2 {
		t.Errorf("Expected SuccessfulQueries 2, got %d", stats.SuccessfulQueries)
	}
	if stats.FailedQueries != 1 {
		t.Errorf("Expected FailedQueries 1, got %d", stats.FailedQueries)
	}
}

func TestStats_SubAgentExecution(t *testing.T) {
	s := NewStats()

	// Record sub-agent executions
	s.RecordSubAgentExecution("LegalAgent", true, 100*time.Millisecond)
	s.RecordSubAgentExecution("LegalAgent", true, 150*time.Millisecond)
	s.RecordSubAgentExecution("LegalAgent", false, 50*time.Millisecond)
	s.RecordSubAgentExecution("ProductAgent", true, 200*time.Millisecond)

	stats := s.GetAgentStats()

	legalStats := stats.SubAgentStats["LegalAgent"]
	if legalStats == nil {
		t.Fatal("Expected LegalAgent stats")
	}
	if legalStats.Invocations != 3 {
		t.Errorf("Expected LegalAgent invocations 3, got %d", legalStats.Invocations)
	}
	if legalStats.Successes != 2 {
		t.Errorf("Expected LegalAgent successes 2, got %d", legalStats.Successes)
	}
	if legalStats.Failures != 1 {
		t.Errorf("Expected LegalAgent failures 1, got %d", legalStats.Failures)
	}

	productStats := stats.SubAgentStats["ProductAgent"]
	if productStats == nil {
		t.Fatal("Expected ProductAgent stats")
	}
	if productStats.Invocations != 1 {
		t.Errorf("Expected ProductAgent invocations 1, got %d", productStats.Invocations)
	}
}

func TestStats_ProviderError(t *testing.T) {
	s := NewStats()

	s.RecordTokenUsage("deepseek", 100, 50)
	s.RecordProviderError("deepseek")
	s.RecordProviderError("deepseek")

	stats := s.GetTokenStats()
	deepseekStats := stats.ProviderStats["deepseek"]
	
	if deepseekStats.ErrorCount != 2 {
		t.Errorf("Expected error count 2, got %d", deepseekStats.ErrorCount)
	}
}

func TestStats_DailyBudget(t *testing.T) {
	s := NewStats()

	s.SetDailyBudget(10000)
	s.RecordTokenUsage("deepseek", 2000, 1000)

	usage := s.GetTokenBudgetUsage()
	expectedUsage := 0.3 // 3000/10000
	if usage != expectedUsage {
		t.Errorf("Expected usage %.2f, got %.2f", expectedUsage, usage)
	}

	// Test reset
	s.ResetDaily()
	usage = s.GetTokenBudgetUsage()
	if usage != 0 {
		t.Errorf("Expected usage 0 after reset, got %.2f", usage)
	}
}

func TestStats_SessionReset(t *testing.T) {
	s := NewStats()

	s.RecordTokenUsage("deepseek", 100, 50)
	
	stats := s.GetTokenStats()
	if stats.SessionInputTokens != 100 {
		t.Errorf("Expected SessionInputTokens 100, got %d", stats.SessionInputTokens)
	}

	s.ResetSession()

	stats = s.GetTokenStats()
	if stats.SessionInputTokens != 0 {
		t.Errorf("Expected SessionInputTokens 0 after reset, got %d", stats.SessionInputTokens)
	}

	// Total should remain
	if stats.TotalInputTokens != 100 {
		t.Errorf("Expected TotalInputTokens 100, got %d", stats.TotalInputTokens)
	}
}

func TestStats_Summary(t *testing.T) {
	s := NewStats()

	s.RecordTokenUsage("deepseek", 100, 50)
	s.RecordCacheHit(1, "index")
	s.RecordCacheMiss(1, "index")
	s.RecordQuery(true, 100*time.Millisecond)
	s.SetDailyBudget(1000)

	summary := s.GetSummary()

	if summary.TokenStats == nil {
		t.Error("Expected TokenStats in summary")
	}
	if summary.CacheStats == nil {
		t.Error("Expected CacheStats in summary")
	}
	if summary.AgentStats == nil {
		t.Error("Expected AgentStats in summary")
	}
	if summary.CacheHitRate != 0.5 {
		t.Errorf("Expected CacheHitRate 0.5, got %.2f", summary.CacheHitRate)
	}
	if summary.TokenBudgetUsage != 0.15 { // 150/1000
		t.Errorf("Expected TokenBudgetUsage 0.15, got %.2f", summary.TokenBudgetUsage)
	}
}

func TestStats_Uptime(t *testing.T) {
	s := NewStats()
	
	time.Sleep(10 * time.Millisecond)
	
	uptime := s.GetUptime()
	if uptime < 10*time.Millisecond {
		t.Errorf("Expected uptime >= 10ms, got %v", uptime)
	}
}

func TestStats_ZeroBudget(t *testing.T) {
	s := NewStats()

	// Budget not set, should return 0
	usage := s.GetTokenBudgetUsage()
	if usage != 0 {
		t.Errorf("Expected usage 0 with no budget, got %.2f", usage)
	}
}

func TestStats_ZeroCacheOperations(t *testing.T) {
	s := NewStats()

	// No cache operations, hit rate should be 0
	hitRate := s.GetCacheHitRate()
	if hitRate != 0 {
		t.Errorf("Expected hit rate 0 with no operations, got %.2f", hitRate)
	}
}

func BenchmarkStats_RecordTokenUsage(b *testing.B) {
	s := NewStats()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.RecordTokenUsage("deepseek", 100, 50)
	}
}

func BenchmarkStats_RecordCacheHit(b *testing.B) {
	s := NewStats()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.RecordCacheHit(1, "index")
	}
}

func BenchmarkStats_GetSummary(b *testing.B) {
	s := NewStats()
	
	// Add some data
	for i := 0; i < 100; i++ {
		s.RecordTokenUsage("deepseek", 100, 50)
		s.RecordCacheHit(1, "index")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = s.GetSummary()
	}
}
