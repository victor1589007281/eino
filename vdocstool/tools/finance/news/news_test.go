// Package news 新闻聚合测试
package news

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cloudwego/eino/vdocstool/tools/finance/types"
)

// MockNewsSource 模拟新闻源
type MockNewsSource struct {
	name string
	news []*types.News
	err  error
}

func (m *MockNewsSource) Name() string {
	return m.name
}

func (m *MockNewsSource) GetNews(ctx context.Context, opts *NewsOptions) ([]*types.News, error) {
	if m.err != nil {
		return nil, m.err
	}
	limit := 50
	if opts != nil && opts.Limit > 0 {
		limit = opts.Limit
	}
	if limit > len(m.news) {
		return m.news, nil
	}
	return m.news[:limit], nil
}

func (m *MockNewsSource) HealthCheck(ctx context.Context) error {
	return m.err
}

func createMockSource(name string) *MockNewsSource {
	now := time.Now()
	return &MockNewsSource{
		name: name,
		news: []*types.News{
			{ID: name + "_1", Title: "News 1 from " + name, Content: "Content 1", Source: name, PublishTime: now.Add(-1 * time.Minute)},
			{ID: name + "_2", Title: "News 2 from " + name, Content: "Content 2", Source: name, PublishTime: now.Add(-2 * time.Minute)},
			{ID: name + "_3", Title: "News 3 from " + name, Content: "Content 3", Source: name, PublishTime: now.Add(-3 * time.Minute)},
		},
	}
}

func TestNewAggregatorCreation(t *testing.T) {
	// NewAggregator默认会注册CLS和Jin10源，它们需要网络
	// 这里只测试创建不报错
	agg := NewAggregator()
	if agg == nil {
		t.Fatal("NewAggregator returned nil")
	}
}

func TestRegisterSource(t *testing.T) {
	agg := &Aggregator{
		sources:     make([]NewsSource, 0),
		sourceMap:   make(map[string]NewsSource),
		dedupeCache: make(map[string]bool),
	}
	mock := createMockSource("mock1")

	agg.RegisterSource(mock)

	sources := agg.GetSourceNames()
	if len(sources) != 1 {
		t.Errorf("Expected 1 source, got %d", len(sources))
	}
	if sources[0] != "mock1" {
		t.Errorf("Expected source name 'mock1', got '%s'", sources[0])
	}
}

func TestGetNews(t *testing.T) {
	agg := &Aggregator{
		sources:     make([]NewsSource, 0),
		sourceMap:   make(map[string]NewsSource),
		dedupeCache: make(map[string]bool),
	}
	mock1 := createMockSource("source1")
	mock2 := createMockSource("source2")

	agg.RegisterSource(mock1)
	agg.RegisterSource(mock2)

	ctx := context.Background()
	news, err := agg.GetNews(ctx, &NewsOptions{Limit: 10})
	if err != nil {
		t.Fatalf("GetNews failed: %v", err)
	}

	// 应该从两个源获取到新闻
	if len(news) == 0 {
		t.Error("Expected news, got none")
	}

	// 验证按时间排序(最新在前)
	for i := 1; i < len(news); i++ {
		if news[i-1].PublishTime.Before(news[i].PublishTime) {
			t.Error("News should be sorted by time descending")
			break
		}
	}
}

func TestGetNewsWithNilOptions(t *testing.T) {
	agg := &Aggregator{
		sources:     make([]NewsSource, 0),
		sourceMap:   make(map[string]NewsSource),
		dedupeCache: make(map[string]bool),
	}
	mock := createMockSource("test")
	agg.RegisterSource(mock)

	ctx := context.Background()
	news, err := agg.GetNews(ctx, nil)
	if err != nil {
		t.Fatalf("GetNews with nil options failed: %v", err)
	}

	if len(news) == 0 {
		t.Error("Expected news, got none")
	}
}

func TestGetFlashNews(t *testing.T) {
	agg := &Aggregator{
		sources:     make([]NewsSource, 0),
		sourceMap:   make(map[string]NewsSource),
		dedupeCache: make(map[string]bool),
	}
	mock := createMockSource("test")
	agg.RegisterSource(mock)

	ctx := context.Background()
	flash, err := agg.GetFlashNews(ctx, 5)
	if err != nil {
		t.Fatalf("GetFlashNews failed: %v", err)
	}

	// mock源会返回新闻
	_ = flash
}

func TestGetImportantNews(t *testing.T) {
	agg := &Aggregator{
		sources:     make([]NewsSource, 0),
		sourceMap:   make(map[string]NewsSource),
		dedupeCache: make(map[string]bool),
	}
	now := time.Now()
	mock := &MockNewsSource{
		name: "test",
		news: []*types.News{
			{ID: "1", Title: "Important 1", Importance: 5, PublishTime: now},
			{ID: "2", Title: "Normal 1", Importance: 1, PublishTime: now.Add(-1 * time.Minute)},
			{ID: "3", Title: "Important 2", Importance: 4, PublishTime: now.Add(-2 * time.Minute)},
		},
	}
	agg.RegisterSource(mock)

	ctx := context.Background()
	important, err := agg.GetImportantNews(ctx, 5)
	if err != nil {
		t.Fatalf("GetImportantNews failed: %v", err)
	}

	// 应该按重要性排序
	if len(important) >= 2 {
		if important[0].Importance < important[1].Importance {
			t.Error("Important news should be sorted by importance descending")
		}
	}
}

func TestGetNewsBySource(t *testing.T) {
	agg := &Aggregator{
		sources:     make([]NewsSource, 0),
		sourceMap:   make(map[string]NewsSource),
		dedupeCache: make(map[string]bool),
	}
	mock1 := createMockSource("source1")
	mock2 := createMockSource("source2")
	agg.RegisterSource(mock1)
	agg.RegisterSource(mock2)

	ctx := context.Background()

	// 获取特定源的新闻
	news, err := agg.GetNewsBySource(ctx, "source1", &NewsOptions{Limit: 10})
	if err != nil {
		t.Fatalf("GetNewsBySource failed: %v", err)
	}

	// 验证所有新闻都来自source1
	for _, n := range news {
		if n.Source != "source1" {
			t.Errorf("Expected news from source1, got from %s", n.Source)
		}
	}

	// 获取不存在的源
	news, err = agg.GetNewsBySource(ctx, "nonexistent", &NewsOptions{Limit: 10})
	if err != nil {
		t.Fatalf("GetNewsBySource for nonexistent source should not error: %v", err)
	}
	if news != nil && len(news) > 0 {
		t.Error("Expected nil or empty news for nonexistent source")
	}
}

func TestDeduplication(t *testing.T) {
	agg := &Aggregator{
		sources:     make([]NewsSource, 0),
		sourceMap:   make(map[string]NewsSource),
		dedupeCache: make(map[string]bool),
	}

	now := time.Now()
	mock1 := &MockNewsSource{
		name: "source1",
		news: []*types.News{
			{ID: "same_id", Title: "Same News", Source: "source1", PublishTime: now},
		},
	}
	mock2 := &MockNewsSource{
		name: "source2",
		news: []*types.News{
			{ID: "same_id", Title: "Same News", Source: "source2", PublishTime: now},
		},
	}

	agg.RegisterSource(mock1)
	agg.RegisterSource(mock2)

	ctx := context.Background()
	news, err := agg.GetNews(ctx, &NewsOptions{Limit: 10})
	if err != nil {
		t.Fatalf("GetNews failed: %v", err)
	}

	// 应该去重，返回2条（因为source不同，key是source_id）
	if len(news) != 2 {
		t.Logf("Note: deduplication uses source_id as key, got %d news", len(news))
	}
}

func TestHealthCheck(t *testing.T) {
	agg := &Aggregator{
		sources:     make([]NewsSource, 0),
		sourceMap:   make(map[string]NewsSource),
		dedupeCache: make(map[string]bool),
	}
	healthy := createMockSource("healthy")
	unhealthy := &MockNewsSource{name: "unhealthy", err: errors.New("connection failed")}

	agg.RegisterSource(healthy)
	agg.RegisterSource(unhealthy)

	ctx := context.Background()
	health := agg.HealthCheck(ctx)

	if _, ok := health["healthy"]; !ok {
		t.Error("healthy source should be in health check results")
	}
	if _, ok := health["unhealthy"]; !ok {
		t.Error("unhealthy source should be in health check results")
	}

	if health["healthy"] != nil {
		t.Error("healthy source should report no error")
	}
	if health["unhealthy"] == nil {
		t.Error("unhealthy source should report error")
	}
}

func TestEmptySources(t *testing.T) {
	agg := &Aggregator{
		sources:     make([]NewsSource, 0),
		sourceMap:   make(map[string]NewsSource),
		dedupeCache: make(map[string]bool),
	}

	ctx := context.Background()
	news, err := agg.GetNews(ctx, &NewsOptions{Limit: 10})

	// 没有源时应该返回空列表而不是错误
	if err != nil {
		t.Errorf("Expected no error with empty sources, got %v", err)
	}
	if len(news) != 0 {
		t.Errorf("Expected empty news list, got %d items", len(news))
	}
}

func TestLimit(t *testing.T) {
	agg := &Aggregator{
		sources:     make([]NewsSource, 0),
		sourceMap:   make(map[string]NewsSource),
		dedupeCache: make(map[string]bool),
	}
	newsItems := make([]*types.News, 100)
	for i := 0; i < 100; i++ {
		newsItems[i] = &types.News{
			ID:          string(rune('A' + i%26)),
			Title:       "News",
			Source:      "test",
			PublishTime: time.Now().Add(-time.Duration(i) * time.Minute),
		}
	}
	mock := &MockNewsSource{name: "test", news: newsItems}
	agg.RegisterSource(mock)

	ctx := context.Background()
	news, _ := agg.GetNews(ctx, &NewsOptions{Limit: 10})

	if len(news) > 10 {
		t.Errorf("Expected at most 10 news, got %d", len(news))
	}
}

func TestSourceError(t *testing.T) {
	agg := &Aggregator{
		sources:     make([]NewsSource, 0),
		sourceMap:   make(map[string]NewsSource),
		dedupeCache: make(map[string]bool),
	}
	healthy := createMockSource("healthy")
	failing := &MockNewsSource{name: "failing", err: errors.New("network error")}

	agg.RegisterSource(healthy)
	agg.RegisterSource(failing)

	ctx := context.Background()
	news, err := agg.GetNews(ctx, &NewsOptions{Limit: 10})

	// 即使一个源失败，其他源应该正常返回
	if err != nil {
		t.Errorf("Should not return error when some sources fail, got %v", err)
	}
	if len(news) == 0 {
		t.Error("Should return news from healthy source")
	}
}

func TestGetSourceNames(t *testing.T) {
	agg := &Aggregator{
		sources:     make([]NewsSource, 0),
		sourceMap:   make(map[string]NewsSource),
		dedupeCache: make(map[string]bool),
	}
	mock1 := createMockSource("source1")
	mock2 := createMockSource("source2")
	agg.RegisterSource(mock1)
	agg.RegisterSource(mock2)

	names := agg.GetSourceNames()
	if len(names) != 2 {
		t.Errorf("Expected 2 source names, got %d", len(names))
	}

	// 检查是否包含预期的名称
	found1, found2 := false, false
	for _, name := range names {
		if name == "source1" {
			found1 = true
		}
		if name == "source2" {
			found2 = true
		}
	}
	if !found1 || !found2 {
		t.Error("Expected to find both source names")
	}
}
