package router

import (
	"testing"
	"time"
)

func TestNewQualityTracker(t *testing.T) {
	tracker := NewQualityTracker()
	if tracker == nil {
		t.Fatal("NewQualityTracker returned nil")
	}
}

func TestQualityTracker_Record(t *testing.T) {
	tracker := NewQualityTracker()

	// 记录一些成功的搜索
	for i := 0; i < 5; i++ {
		tracker.Record("serper", &SearchRecord{
			Timestamp:   time.Now(),
			QueryType:   QueryTypeTechnical,
			ResultCount: 10,
			Latency:     time.Millisecond * 200,
			Success:     true,
		})
	}

	// 记录一个失败的搜索
	tracker.Record("serper", &SearchRecord{
		Timestamp:   time.Now(),
		QueryType:   QueryTypeTechnical,
		ResultCount: 0,
		Latency:     time.Second * 5,
		Success:     false,
		ErrorType:   "timeout",
	})

	metrics := tracker.GetMetrics("serper")
	if metrics == nil {
		t.Fatal("GetMetrics returned nil")
	}

	// 检查成功率
	expectedSuccessRate := 5.0 / 6.0
	if metrics.SuccessRate < expectedSuccessRate-0.01 || metrics.SuccessRate > expectedSuccessRate+0.01 {
		t.Errorf("SuccessRate = %v, want approximately %v", metrics.SuccessRate, expectedSuccessRate)
	}

	// 检查样本数
	if metrics.TotalRecords != 6 {
		t.Errorf("TotalRecords = %d, want 6", metrics.TotalRecords)
	}
}

func TestQualityTracker_GetMetrics_Empty(t *testing.T) {
	tracker := NewQualityTracker()

	metrics := tracker.GetMetrics("nonexistent")
	if metrics == nil {
		t.Fatal("GetMetrics should return default metrics for nonexistent engine")
	}

	// 默认指标应该是乐观估计值 (0.8)
	if metrics.SuccessRate != 0.8 {
		t.Errorf("Default SuccessRate = %v, want 0.8", metrics.SuccessRate)
	}
}

func TestQualityTracker_LatencyPercentiles(t *testing.T) {
	tracker := NewQualityTracker()

	// 记录具有不同延迟的搜索
	latencies := []time.Duration{
		time.Millisecond * 100,
		time.Millisecond * 200,
		time.Millisecond * 300,
		time.Millisecond * 400,
		time.Millisecond * 500,
		time.Millisecond * 600,
		time.Millisecond * 700,
		time.Millisecond * 800,
		time.Millisecond * 900,
		time.Millisecond * 1000,
	}

	for _, lat := range latencies {
		tracker.Record("test", &SearchRecord{
			Timestamp:   time.Now(),
			QueryType:   QueryTypeGeneral,
			ResultCount: 10,
			Latency:     lat,
			Success:     true,
		})
	}

	metrics := tracker.GetMetrics("test")

	// P50 应该大约在 500-600ms 之间
	if metrics.LatencyP50 < time.Millisecond*400 || metrics.LatencyP50 > time.Millisecond*700 {
		t.Errorf("LatencyP50 = %v, expected around 500-600ms", metrics.LatencyP50)
	}

	// P95 应该在 900-1000ms 之间
	if metrics.LatencyP95 < time.Millisecond*800 || metrics.LatencyP95 > time.Millisecond*1100 {
		t.Errorf("LatencyP95 = %v, expected around 900-1000ms", metrics.LatencyP95)
	}
}

func TestQualityTracker_ConsecutiveFailures(t *testing.T) {
	tracker := NewQualityTracker()

	// 记录连续失败
	for i := 0; i < 5; i++ {
		tracker.Record("failing", &SearchRecord{
			Timestamp:   time.Now(),
			QueryType:   QueryTypeGeneral,
			ResultCount: 0,
			Latency:     time.Second,
			Success:     false,
			ErrorType:   "error",
		})
	}

	// 使用 GetConsecutiveFails 方法检查当前连续失败数
	consecutiveFails := tracker.GetConsecutiveFails("failing")
	if consecutiveFails != 5 {
		t.Errorf("ConsecutiveFails = %d, want 5", consecutiveFails)
	}

	// 记录一个成功的搜索
	tracker.Record("failing", &SearchRecord{
		Timestamp:   time.Now(),
		QueryType:   QueryTypeGeneral,
		ResultCount: 10,
		Latency:     time.Millisecond * 200,
		Success:     true,
	})

	// 成功后当前连续失败数应为0
	consecutiveFails = tracker.GetConsecutiveFails("failing")
	if consecutiveFails != 0 {
		t.Errorf("ConsecutiveFails after success = %d, want 0", consecutiveFails)
	}

	// 但 metrics.ConsecutiveFails 记录的是历史最大连续失败数
	metrics := tracker.GetMetrics("failing")
	if metrics.ConsecutiveFails != 5 {
		t.Errorf("Max ConsecutiveFails in history = %d, want 5", metrics.ConsecutiveFails)
	}
}

func TestSlidingWindow(t *testing.T) {
	window := NewSlidingWindow(5)

	// 添加记录
	for i := 0; i < 10; i++ {
		window.Add(&SearchRecord{
			Timestamp:   time.Now(),
			ResultCount: i,
			Success:     true,
		})
	}

	// 窗口应该只保留最近5条记录，验证通过计算指标
	metrics := window.CalculateMetrics()
	if metrics.TotalRecords != 5 {
		t.Errorf("TotalRecords = %d, want 5", metrics.TotalRecords)
	}
}

func TestSlidingWindow_CalculateMetrics(t *testing.T) {
	window := NewSlidingWindow(5)

	for i := 0; i < 3; i++ {
		window.Add(&SearchRecord{
			Timestamp:   time.Now(),
			ResultCount: 10,
			Latency:     time.Millisecond * 200,
			Success:     true,
		})
	}

	metrics := window.CalculateMetrics()
	if metrics.TotalRecords != 3 {
		t.Errorf("TotalRecords = %d, want 3", metrics.TotalRecords)
	}
	if metrics.SuccessRate != 1.0 {
		t.Errorf("SuccessRate = %v, want 1.0", metrics.SuccessRate)
	}
}
