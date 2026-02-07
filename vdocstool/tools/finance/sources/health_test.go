package sources

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cloudwego/eino/vdocstool/tools/finance/types"
)

// MockDataSource 模拟数据源
type MockDataSource struct {
	name            string
	priority        int
	healthCheckErr  error
	healthCheckTime time.Duration
}

func (m *MockDataSource) Name() string        { return m.name }
func (m *MockDataSource) Priority() int       { return m.priority }
func (m *MockDataSource) SupportedAssets() []types.AssetType { return nil }
func (m *MockDataSource) SupportedMarkets() []types.Market   { return nil }
func (m *MockDataSource) GetQuote(ctx context.Context, symbol string) (*types.Quote, error) {
	return nil, nil
}
func (m *MockDataSource) GetQuotes(ctx context.Context, symbols []string) ([]*types.Quote, error) {
	return nil, nil
}
func (m *MockDataSource) GetKLine(ctx context.Context, symbol string, period string, count int) ([]*types.KLine, error) {
	return nil, nil
}
func (m *MockDataSource) Search(ctx context.Context, keyword string) ([]*types.SearchResult, error) {
	return nil, nil
}
func (m *MockDataSource) HealthCheck(ctx context.Context) error {
	if m.healthCheckTime > 0 {
		time.Sleep(m.healthCheckTime)
	}
	return m.healthCheckErr
}

func TestCircuitBreaker_CanExecute(t *testing.T) {
	config := DefaultHealthConfig()
	config.FailureThreshold = 3
	config.RecoveryTimeout = 100 * time.Millisecond

	cb := NewCircuitBreaker(config)

	// 初始状态应该是关闭（可执行）
	if !cb.CanExecute() {
		t.Error("Expected circuit breaker to allow execution in closed state")
	}

	// 记录失败，不足以触发熔断
	cb.RecordFailure()
	cb.RecordFailure()
	if !cb.CanExecute() {
		t.Error("Expected circuit breaker to still allow execution after 2 failures")
	}

	// 再记录一次失败，触发熔断
	cb.RecordFailure()
	if cb.CanExecute() {
		t.Error("Expected circuit breaker to block execution after threshold failures")
	}

	// 等待恢复
	time.Sleep(150 * time.Millisecond)
	if !cb.CanExecute() {
		t.Error("Expected circuit breaker to enter half-open state after recovery timeout")
	}

	// 在半开状态下成功
	cb.RecordSuccess()
	cb.RecordSuccess()
	if !cb.CanExecute() {
		t.Error("Expected circuit breaker to return to closed state after successes")
	}
}

func TestCircuitBreaker_HalfOpenFailure(t *testing.T) {
	config := DefaultHealthConfig()
	config.FailureThreshold = 2
	config.SuccessThreshold = 2
	config.RecoveryTimeout = 50 * time.Millisecond

	cb := NewCircuitBreaker(config)

	// 触发熔断
	cb.RecordFailure()
	cb.RecordFailure()

	// 等待进入半开状态
	time.Sleep(60 * time.Millisecond)
	cb.CanExecute() // 触发状态转换

	// 在半开状态下失败，应该立即熔断
	cb.RecordFailure()
	if cb.CanExecute() {
		t.Error("Expected circuit breaker to open after failure in half-open state")
	}
}

func TestCircuitBreaker_Reset(t *testing.T) {
	config := DefaultHealthConfig()
	config.FailureThreshold = 2

	cb := NewCircuitBreaker(config)

	// 触发熔断
	cb.RecordFailure()
	cb.RecordFailure()

	if cb.CanExecute() {
		t.Error("Expected circuit breaker to be open")
	}

	// 重置
	cb.Reset()

	if !cb.CanExecute() {
		t.Error("Expected circuit breaker to allow execution after reset")
	}
}

func TestHealthChecker_RegisterSource(t *testing.T) {
	hc := NewHealthChecker(nil)

	source := &MockDataSource{name: "test", priority: 1}
	hc.RegisterSource(source)

	health := hc.GetHealth("test")
	if health == nil {
		t.Error("Expected health info to be available after registration")
	}

	if health.Name != "test" {
		t.Errorf("Expected health name to be 'test', got '%s'", health.Name)
	}

	if health.Status != StatusHealthy {
		t.Errorf("Expected initial status to be healthy, got '%s'", health.Status)
	}
}

func TestHealthChecker_IsHealthy(t *testing.T) {
	hc := NewHealthChecker(nil)

	source := &MockDataSource{name: "test", priority: 1}
	hc.RegisterSource(source)

	if !hc.IsHealthy("test") {
		t.Error("Expected newly registered source to be healthy")
	}

	if hc.IsHealthy("unknown") {
		t.Error("Expected unknown source to be unhealthy")
	}
}

func TestHealthChecker_CanUse(t *testing.T) {
	hc := NewHealthChecker(nil)

	source := &MockDataSource{name: "test", priority: 1}
	hc.RegisterSource(source)

	if !hc.CanUse("test") {
		t.Error("Expected newly registered source to be usable")
	}

	if hc.CanUse("unknown") {
		t.Error("Expected unknown source to be unusable")
	}
}

func TestHealthChecker_RecordSuccessFailure(t *testing.T) {
	config := DefaultHealthConfig()
	config.FailureThreshold = 2
	hc := NewHealthChecker(config)

	source := &MockDataSource{name: "test", priority: 1}
	hc.RegisterSource(source)

	// 记录成功
	hc.RecordSuccess("test")
	if !hc.CanUse("test") {
		t.Error("Expected source to be usable after success")
	}

	// 记录失败直到熔断
	hc.RecordFailure("test")
	hc.RecordFailure("test")

	if hc.CanUse("test") {
		t.Error("Expected source to be unusable after threshold failures")
	}
}

func TestHealthChecker_GetHealthySourcesSorted(t *testing.T) {
	hc := NewHealthChecker(nil)

	source1 := &MockDataSource{name: "high", priority: 1}
	source2 := &MockDataSource{name: "low", priority: 10}
	source3 := &MockDataSource{name: "mid", priority: 5}

	hc.RegisterSource(source1)
	hc.RegisterSource(source2)
	hc.RegisterSource(source3)

	sources := hc.GetHealthySourcesSorted()

	if len(sources) != 3 {
		t.Errorf("Expected 3 healthy sources, got %d", len(sources))
	}

	// 验证排序
	if sources[0].Name() != "high" {
		t.Errorf("Expected first source to be 'high', got '%s'", sources[0].Name())
	}
	if sources[1].Name() != "mid" {
		t.Errorf("Expected second source to be 'mid', got '%s'", sources[1].Name())
	}
	if sources[2].Name() != "low" {
		t.Errorf("Expected third source to be 'low', got '%s'", sources[2].Name())
	}
}

func TestHealthChecker_GetAllHealth(t *testing.T) {
	hc := NewHealthChecker(nil)

	hc.RegisterSource(&MockDataSource{name: "source1", priority: 1})
	hc.RegisterSource(&MockDataSource{name: "source2", priority: 2})

	health := hc.GetAllHealth()

	if len(health) != 2 {
		t.Errorf("Expected 2 health entries, got %d", len(health))
	}

	if _, ok := health["source1"]; !ok {
		t.Error("Expected health entry for 'source1'")
	}
	if _, ok := health["source2"]; !ok {
		t.Error("Expected health entry for 'source2'")
	}
}

func TestHealthChecker_CheckSource(t *testing.T) {
	config := DefaultHealthConfig()
	config.CheckInterval = 10 * time.Millisecond
	hc := NewHealthChecker(config)

	// 成功的数据源
	successSource := &MockDataSource{
		name:           "success",
		priority:       1,
		healthCheckErr: nil,
	}

	// 失败的数据源
	failSource := &MockDataSource{
		name:           "fail",
		priority:       2,
		healthCheckErr: errors.New("health check failed"),
	}

	hc.RegisterSource(successSource)
	hc.RegisterSource(failSource)

	// 手动触发检查
	hc.checkSource(successSource)
	hc.checkSource(failSource)

	successHealth := hc.GetHealth("success")
	failHealth := hc.GetHealth("fail")

	if successHealth.ConsecutiveFails != 0 {
		t.Errorf("Expected success source to have 0 consecutive fails, got %d", successHealth.ConsecutiveFails)
	}

	if failHealth.ConsecutiveFails != 1 {
		t.Errorf("Expected fail source to have 1 consecutive fail, got %d", failHealth.ConsecutiveFails)
	}
}
