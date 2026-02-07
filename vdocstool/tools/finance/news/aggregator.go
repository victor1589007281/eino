// Package news 新闻聚合服务
package news

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/cloudwego/eino/vdocstool/tools/finance/types"
)

// NewsOptions 新闻查询选项
type NewsOptions struct {
	Limit    int
	Category string
	Symbol   string
	Since    time.Time
}

// NewsSource 新闻源接口
type NewsSource interface {
	Name() string
	GetNews(ctx context.Context, opts *NewsOptions) ([]*types.News, error)
	HealthCheck(ctx context.Context) error
}

// Aggregator 新闻聚合器
type Aggregator struct {
	sources     []NewsSource
	sourceMap   map[string]NewsSource
	mu          sync.RWMutex
	dedupeCache map[string]bool  // 用于去重
	cacheMu     sync.Mutex
}

// NewAggregator 创建新闻聚合器
func NewAggregator() *Aggregator {
	agg := &Aggregator{
		sources:     make([]NewsSource, 0),
		sourceMap:   make(map[string]NewsSource),
		dedupeCache: make(map[string]bool),
	}

	// 注册默认数据源
	agg.RegisterSource(NewCLSSource())
	agg.RegisterSource(NewJin10Source())

	return agg
}

// RegisterSource 注册新闻源
func (a *Aggregator) RegisterSource(source NewsSource) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.sources = append(a.sources, source)
	a.sourceMap[source.Name()] = source
}

// GetNews 获取聚合新闻
func (a *Aggregator) GetNews(ctx context.Context, opts *NewsOptions) ([]*types.News, error) {
	if opts == nil {
		opts = &NewsOptions{Limit: 50}
	}

	a.mu.RLock()
	sources := make([]NewsSource, len(a.sources))
	copy(sources, a.sources)
	a.mu.RUnlock()

	// 并行获取各数据源新闻
	var wg sync.WaitGroup
	resultCh := make(chan []*types.News, len(sources))
	errCh := make(chan error, len(sources))

	for _, source := range sources {
		wg.Add(1)
		go func(s NewsSource) {
			defer wg.Done()

			news, err := s.GetNews(ctx, opts)
			if err != nil {
				errCh <- err
				return
			}
			resultCh <- news
		}(source)
	}

	// 等待所有goroutine完成
	go func() {
		wg.Wait()
		close(resultCh)
		close(errCh)
	}()

	// 收集结果
	allNews := make([]*types.News, 0)
	for news := range resultCh {
		allNews = append(allNews, news...)
	}

	// 去重
	allNews = a.deduplicate(allNews)

	// 按时间排序
	sort.Slice(allNews, func(i, j int) bool {
		return allNews[i].PublishTime.After(allNews[j].PublishTime)
	})

	// 限制返回数量
	if len(allNews) > opts.Limit {
		allNews = allNews[:opts.Limit]
	}

	return allNews, nil
}

// GetNewsBySource 从指定数据源获取新闻
func (a *Aggregator) GetNewsBySource(ctx context.Context, sourceName string, opts *NewsOptions) ([]*types.News, error) {
	a.mu.RLock()
	source, ok := a.sourceMap[sourceName]
	a.mu.RUnlock()

	if !ok {
		return nil, nil
	}

	return source.GetNews(ctx, opts)
}

// GetImportantNews 获取重要新闻
func (a *Aggregator) GetImportantNews(ctx context.Context, limit int) ([]*types.News, error) {
	allNews, err := a.GetNews(ctx, &NewsOptions{Limit: limit * 5})
	if err != nil {
		return nil, err
	}

	// 按重要性和时间排序
	sort.Slice(allNews, func(i, j int) bool {
		if allNews[i].Importance != allNews[j].Importance {
			return allNews[i].Importance > allNews[j].Importance
		}
		return allNews[i].PublishTime.After(allNews[j].PublishTime)
	})

	if len(allNews) > limit {
		allNews = allNews[:limit]
	}

	return allNews, nil
}

// GetFlashNews 获取实时快讯
func (a *Aggregator) GetFlashNews(ctx context.Context, limit int) ([]*types.News, error) {
	return a.GetNews(ctx, &NewsOptions{
		Limit:    limit,
		Category: "快讯",
	})
}

// GetStockNews 获取个股相关新闻
func (a *Aggregator) GetStockNews(ctx context.Context, symbol string, limit int) ([]*types.News, error) {
	// 从财联社获取个股新闻
	a.mu.RLock()
	clsSource, ok := a.sourceMap["cls"]
	a.mu.RUnlock()

	if !ok {
		return nil, nil
	}

	if cls, ok := clsSource.(*CLSSource); ok {
		return cls.GetStockNews(ctx, symbol, limit)
	}

	return nil, nil
}

// GetCalendar 获取财经日历
func (a *Aggregator) GetCalendar(ctx context.Context, date time.Time) ([]*types.EconomicEvent, error) {
	a.mu.RLock()
	jin10Source, ok := a.sourceMap["jin10"]
	a.mu.RUnlock()

	if !ok {
		return nil, nil
	}

	if jin10, ok := jin10Source.(*Jin10Source); ok {
		return jin10.GetCalendar(ctx, date)
	}

	return nil, nil
}

// SearchNews 搜索新闻
func (a *Aggregator) SearchNews(ctx context.Context, keyword string, limit int) ([]*types.News, error) {
	a.mu.RLock()
	clsSource, ok := a.sourceMap["cls"]
	a.mu.RUnlock()

	if !ok {
		return nil, nil
	}

	if cls, ok := clsSource.(*CLSSource); ok {
		return cls.SearchNews(ctx, keyword, limit)
	}

	return nil, nil
}

// deduplicate 新闻去重
func (a *Aggregator) deduplicate(news []*types.News) []*types.News {
	a.cacheMu.Lock()
	defer a.cacheMu.Unlock()

	// 清理过期缓存(保留最近1000条)
	if len(a.dedupeCache) > 1000 {
		a.dedupeCache = make(map[string]bool)
	}

	result := make([]*types.News, 0, len(news))
	seen := make(map[string]bool)

	for _, n := range news {
		// 使用ID和标题组合作为去重key
		key := n.Source + "_" + n.ID
		if n.ID == "" {
			key = n.Source + "_" + n.Title
		}

		if seen[key] || a.dedupeCache[key] {
			continue
		}

		seen[key] = true
		a.dedupeCache[key] = true
		result = append(result, n)
	}

	return result
}

// HealthCheck 检查所有数据源健康状态
func (a *Aggregator) HealthCheck(ctx context.Context) map[string]error {
	a.mu.RLock()
	sources := make([]NewsSource, len(a.sources))
	copy(sources, a.sources)
	a.mu.RUnlock()

	results := make(map[string]error)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, source := range sources {
		wg.Add(1)
		go func(s NewsSource) {
			defer wg.Done()
			err := s.HealthCheck(ctx)
			mu.Lock()
			results[s.Name()] = err
			mu.Unlock()
		}(source)
	}

	wg.Wait()
	return results
}

// GetSourceNames 获取所有数据源名称
func (a *Aggregator) GetSourceNames() []string {
	a.mu.RLock()
	defer a.mu.RUnlock()

	names := make([]string, 0, len(a.sources))
	for _, s := range a.sources {
		names = append(names, s.Name())
	}
	return names
}
