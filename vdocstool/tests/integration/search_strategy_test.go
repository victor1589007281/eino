// Package integration 集成测试
package integration

import (
	"testing"
	"time"

	"github.com/cloudwego/eino/vdocstool/tools/search/router"
)

// TestDynamicSearchStrategy_Integration 动态搜索策略集成测试
func TestDynamicSearchStrategy_Integration(t *testing.T) {
	// 创建所有组件
	classifier := router.NewQueryClassifier()
	quotaManager := router.NewQuotaManager("") // 空路径表示不持久化
	qualityTracker := router.NewQualityTracker()
	scorer := router.NewEngineScorer(qualityTracker, quotaManager, classifier)

	// 测试场景1: 技术查询
	t.Run("TechnicalQuery", func(t *testing.T) {
		query := "golang context package usage example"
		classification := classifier.Classify(query)

		if classification.PrimaryType != router.QueryTypeTechnical {
			t.Errorf("Expected QueryTypeTechnical, got %v", classification.PrimaryType)
		}

		// 获取引擎得分
		engines := []string{"serper", "tavily", "duckduckgo", "bing"}
		t.Logf("Engine scores for technical query:")
		for _, engine := range engines {
			score := scorer.Score(engine, query)
			t.Logf("  %s: %.2f", engine, score)
		}
	})

	// 测试场景2: 新闻查询
	t.Run("NewsQuery", func(t *testing.T) {
		query := "latest AI news today"
		classification := classifier.Classify(query)

		if classification.PrimaryType != router.QueryTypeNews {
			t.Errorf("Expected QueryTypeNews, got %v", classification.PrimaryType)
		}

		engines := []string{"serper", "tavily", "duckduckgo"}
		t.Logf("Engine scores for news query:")
		for _, engine := range engines {
			score := scorer.Score(engine, query)
			t.Logf("  %s: %.2f", engine, score)
		}
	})

	// 测试场景3: 配额管理
	t.Run("QuotaManagement", func(t *testing.T) {
		// 使用预定义的引擎
		quota := quotaManager.GetQuota("serper")
		if quota != nil {
			t.Logf("Serper quota: daily=%d, monthly=%d", quota.DailyLimit, quota.MonthlyLimit)

			// 检查是否可用
			if !quotaManager.IsAvailable("serper") {
				t.Error("Serper should be available initially")
			}

			// 增加使用量
			quotaManager.IncrementUsage("serper")
			t.Logf("After increment: daily_used=%d", quota.DailyUsed)
		}

		// 检查免费引擎
		if !quotaManager.IsAvailable("duckduckgo") {
			t.Error("DuckDuckGo should always be available (free)")
		}
	})

	// 测试场景4: 质量追踪
	t.Run("QualityTracking", func(t *testing.T) {
		// 记录一些搜索结果
		for i := 0; i < 10; i++ {
			qualityTracker.Record("test_engine", &router.SearchRecord{
				Timestamp:   time.Now(),
				QueryType:   router.QueryTypeGeneral,
				ResultCount: 10 - i,
				Latency:     time.Millisecond * time.Duration(100+i*20),
				Success:     i < 8, // 80% 成功率
			})
		}

		metrics := qualityTracker.GetMetrics("test_engine")
		if metrics == nil {
			t.Fatal("GetMetrics returned nil")
		}

		t.Logf("Quality Metrics for test_engine:")
		t.Logf("  Success Rate: %.2f", metrics.SuccessRate)
		t.Logf("  Avg Result Count: %.1f", metrics.AvgResultCount)
		t.Logf("  Latency P50: %v", metrics.LatencyP50)
		t.Logf("  Latency P95: %v", metrics.LatencyP95)

		// 验证成功率大约是80%
		if metrics.SuccessRate < 0.7 || metrics.SuccessRate > 0.9 {
			t.Errorf("Expected ~80%% success rate, got %.2f", metrics.SuccessRate)
		}
	})

	// 测试场景5: 连续失败检测
	t.Run("ConsecutiveFailures", func(t *testing.T) {
		// 记录连续失败
		for i := 0; i < 5; i++ {
			qualityTracker.Record("failing_engine", &router.SearchRecord{
				Timestamp:   time.Now(),
				QueryType:   router.QueryTypeGeneral,
				ResultCount: 0,
				Latency:     time.Second * 5,
				Success:     false,
				ErrorType:   "timeout",
			})
		}

		consecutiveFails := qualityTracker.GetConsecutiveFails("failing_engine")
		if consecutiveFails != 5 {
			t.Errorf("Expected 5 consecutive failures, got %d", consecutiveFails)
		}

		// 连续失败应该影响得分
		score := scorer.Score("failing_engine", "test query")
		t.Logf("Failing engine score: %.2f", score)

		// 清除缓存后再评分
		scorer.ClearCache()
		scoreAfterClear := scorer.Score("failing_engine", "test query")
		t.Logf("Score after cache clear: %.2f", scoreAfterClear)
	})

	// 测试场景6: 引擎对比
	t.Run("EngineComparison", func(t *testing.T) {
		// 为好引擎添加优质记录
		for i := 0; i < 20; i++ {
			qualityTracker.Record("good_engine", &router.SearchRecord{
				Timestamp:   time.Now(),
				QueryType:   router.QueryTypeGeneral,
				ResultCount: 10,
				Latency:     time.Millisecond * 100,
				Success:     true,
			})
		}

		scorer.ClearCache()

		goodScore := scorer.Score("good_engine", "test query")
		badScore := scorer.Score("failing_engine", "test query")

		t.Logf("Good engine score: %.2f", goodScore)
		t.Logf("Bad engine score: %.2f", badScore)

		if goodScore <= badScore {
			t.Error("Good engine should have higher score than bad engine")
		}
	})
}
