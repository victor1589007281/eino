package router

import (
	"testing"
	"time"
)

func TestNewEngineScorer(t *testing.T) {
	qt := NewQualityTracker()
	qm := NewQuotaManager("")
	classifier := NewQueryClassifier()

	scorer := NewEngineScorer(qt, qm, classifier)
	if scorer == nil {
		t.Fatal("NewEngineScorer returned nil")
	}
}

func TestEngineScorer_Score(t *testing.T) {
	qt := NewQualityTracker()
	qm := NewQuotaManager("")
	classifier := NewQueryClassifier()

	// 添加一些搜索记录以建立质量基线
	for i := 0; i < 10; i++ {
		qt.Record("serper", &SearchRecord{
			Timestamp:   time.Now(),
			QueryType:   QueryTypeTechnical,
			ResultCount: 10,
			Latency:     time.Millisecond * 200,
			Success:     true,
		})
	}

	scorer := NewEngineScorer(qt, qm, classifier)

	score := scorer.Score("serper", "golang tutorial")
	if score < 0 || score > 1 {
		t.Errorf("Score = %v, should be between 0 and 1", score)
	}

	// 得分应该相对较高，因为有良好的质量记录
	if score < 0.3 {
		t.Errorf("Score = %v, expected higher score for engine with good track record", score)
	}
}

func TestEngineScorer_ScoreWithDetails(t *testing.T) {
	qt := NewQualityTracker()
	qm := NewQuotaManager("")
	classifier := NewQueryClassifier()

	// 添加搜索记录
	for i := 0; i < 5; i++ {
		qt.Record("test_engine", &SearchRecord{
			Timestamp:   time.Now(),
			QueryType:   QueryTypeGeneral,
			ResultCount: 8,
			Latency:     time.Millisecond * 300,
			Success:     true,
		})
	}

	scorer := NewEngineScorer(qt, qm, classifier)

	details := scorer.ScoreWithDetails("test_engine", "python programming")
	if details == nil {
		t.Fatal("ScoreWithDetails returned nil")
	}

	// 检查各项得分
	if details.SuccessScore < 0 || details.SuccessScore > 1 {
		t.Errorf("SuccessScore = %v, should be between 0 and 1", details.SuccessScore)
	}

	if details.FinalScore < 0 || details.FinalScore > 1 {
		t.Errorf("FinalScore = %v, should be between 0 and 1", details.FinalScore)
	}
}

func TestEngineScorer_ScoreComparison(t *testing.T) {
	qt := NewQualityTracker()
	qm := NewQuotaManager("")
	classifier := NewQueryClassifier()

	// 好引擎: 高成功率
	for i := 0; i < 20; i++ {
		qt.Record("good_engine", &SearchRecord{
			Timestamp:   time.Now(),
			QueryType:   QueryTypeGeneral,
			ResultCount: 10,
			Latency:     time.Millisecond * 100,
			Success:     true,
		})
	}

	// 差引擎: 低成功率
	for i := 0; i < 20; i++ {
		qt.Record("bad_engine", &SearchRecord{
			Timestamp:   time.Now(),
			QueryType:   QueryTypeGeneral,
			ResultCount: 2,
			Latency:     time.Second,
			Success:     i%5 == 0, // 20% 成功率
		})
	}

	scorer := NewEngineScorer(qt, qm, classifier)

	goodScore := scorer.Score("good_engine", "test query")
	badScore := scorer.Score("bad_engine", "test query")

	if goodScore <= badScore {
		t.Errorf("Good engine score (%v) should be higher than bad engine score (%v)",
			goodScore, badScore)
	}
}

func TestEngineScorer_ConsecutiveFailurePenalty(t *testing.T) {
	qt := NewQualityTracker()
	qm := NewQuotaManager("")
	classifier := NewQueryClassifier()

	// 先记录成功
	for i := 0; i < 10; i++ {
		qt.Record("failing_engine", &SearchRecord{
			Timestamp:   time.Now(),
			QueryType:   QueryTypeGeneral,
			ResultCount: 10,
			Latency:     time.Millisecond * 200,
			Success:     true,
		})
	}

	scorer := NewEngineScorer(qt, qm, classifier)
	scoreBeforeFails := scorer.Score("failing_engine", "test query")

	// 添加连续失败
	for i := 0; i < 5; i++ {
		qt.Record("failing_engine", &SearchRecord{
			Timestamp:   time.Now(),
			QueryType:   QueryTypeGeneral,
			ResultCount: 0,
			Latency:     time.Second * 5,
			Success:     false,
			ErrorType:   "timeout",
		})
	}

	// 清除缓存
	scorer.ClearCache()

	scoreAfterFails := scorer.Score("failing_engine", "test query")

	// 连续失败后得分应该降低
	if scoreAfterFails >= scoreBeforeFails {
		t.Errorf("Score after failures (%v) should be lower than before (%v)",
			scoreAfterFails, scoreBeforeFails)
	}
}

func TestEngineScorer_QueryTypeMatchScore(t *testing.T) {
	tests := []struct {
		engine    string
		queryType QueryType
		wantMin   float64
	}{
		{"serper", QueryTypeTechnical, 0.5},
		{"tavily", QueryTypeTechnical, 0.5},
		{"duckduckgo", QueryTypeGeneral, 0.5},
	}

	for _, tt := range tests {
		t.Run(tt.engine+"_"+string(tt.queryType), func(t *testing.T) {
			score := GetQueryTypeMatchScore(tt.engine, tt.queryType)
			if score < tt.wantMin {
				t.Errorf("GetQueryTypeMatchScore(%s, %s) = %v, want >= %v",
					tt.engine, tt.queryType, score, tt.wantMin)
			}
		})
	}
}

func TestDefaultScorerWeights(t *testing.T) {
	// 验证权重合理
	total := DefaultScorerWeights.Quality +
		DefaultScorerWeights.SuccessRate +
		DefaultScorerWeights.QuotaHealth +
		DefaultScorerWeights.QueryTypeMatch +
		DefaultScorerWeights.Latency +
		DefaultScorerWeights.Cost

	// 权重总和应该接近1.0
	if total < 0.9 || total > 1.1 {
		t.Errorf("Weight sum = %v, expected around 1.0", total)
	}

	// 各权重应该为正
	if DefaultScorerWeights.Quality <= 0 {
		t.Error("Quality weight should be positive")
	}
	if DefaultScorerWeights.SuccessRate <= 0 {
		t.Error("SuccessRate weight should be positive")
	}
}
