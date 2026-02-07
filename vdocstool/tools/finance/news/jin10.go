// Package news 金十数据新闻源
package news

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/cloudwego/eino/vdocstool/tools/finance/types"
)

// Jin10Source 金十数据源
type Jin10Source struct {
	client *http.Client
}

// NewJin10Source 创建金十数据源
func NewJin10Source() *Jin10Source {
	return &Jin10Source{
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Name 数据源名称
func (j *Jin10Source) Name() string {
	return "jin10"
}

// GetNews 获取金十快讯
func (j *Jin10Source) GetNews(ctx context.Context, opts *NewsOptions) ([]*types.News, error) {
	if opts == nil {
		opts = &NewsOptions{Limit: 50}
	}

	// 金十快讯API
	reqURL := fmt.Sprintf("https://www.jin10.com/flash_newest.js?t=%d", time.Now().UnixMilli())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Referer", "https://www.jin10.com/")
	req.Header.Set("Accept", "*/*")

	resp, err := j.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response failed: %w", err)
	}

	return j.parseFlashResponse(body, opts.Limit)
}

// parseFlashResponse 解析快讯响应
func (j *Jin10Source) parseFlashResponse(body []byte, limit int) ([]*types.News, error) {
	// 金十返回 JSONP 格式: callback({...})
	// 需要提取JSON部分
	data := string(body)
	
	// 尝试直接解析JSON数组
	var items []struct {
		ID       string `json:"id"`
		Time     string `json:"time"`
		Type     int    `json:"type"`
		Data     struct {
			Content string `json:"content"`
			Title   string `json:"title"`
			Pic     string `json:"pic"`
		} `json:"data"`
		Important int    `json:"important"`
		Tags      []string `json:"tags"`
		Channel   []string `json:"channel"`
	}

	if err := json.Unmarshal([]byte(data), &items); err != nil {
		// 尝试备用API
		return j.getFlashFromBackup(context.Background(), limit)
	}

	news := make([]*types.News, 0, len(items))
	for i, item := range items {
		if i >= limit {
			break
		}

		n := &types.News{
			ID:         item.ID,
			Title:      item.Data.Title,
			Content:    item.Data.Content,
			Source:     j.Name(),
			Importance: item.Important,
			Tags:       item.Tags,
		}

		// 解析时间
		if t, err := time.Parse("2006-01-02 15:04:05", item.Time); err == nil {
			n.PublishTime = t
		} else {
			n.PublishTime = time.Now()
		}

		// 设置分类
		if len(item.Channel) > 0 {
			n.Category = item.Channel[0]
		} else {
			n.Category = "快讯"
		}

		news = append(news, n)
	}

	return news, nil
}

// getFlashFromBackup 备用API获取快讯
func (j *Jin10Source) getFlashFromBackup(ctx context.Context, limit int) ([]*types.News, error) {
	// 使用金十的另一个API
	reqURL := fmt.Sprintf("https://flash-api.jin10.com/get_flash_list?channel=-8200&vip=1&max_time=&t=%d",
		time.Now().UnixMilli())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Referer", "https://www.jin10.com/")
	req.Header.Set("x-app-id", "bVBF4FyRTn5NJF5n")
	req.Header.Set("x-version", "1.0.0")

	resp, err := j.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response failed: %w", err)
	}

	var result struct {
		Code int `json:"ret"`
		Data []struct {
			ID      int64  `json:"id"`
			Time    string `json:"time"`
			Type    int    `json:"type"`
			Content string `json:"data"`
			Important int  `json:"important"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse json failed: %w", err)
	}

	news := make([]*types.News, 0, len(result.Data))
	for i, item := range result.Data {
		if i >= limit {
			break
		}

		n := &types.News{
			ID:         strconv.FormatInt(item.ID, 10),
			Content:    item.Content,
			Source:     j.Name(),
			Importance: item.Important,
			Category:   "快讯",
		}

		// 解析时间
		if t, err := time.Parse("2006-01-02 15:04:05", item.Time); err == nil {
			n.PublishTime = t
		} else {
			n.PublishTime = time.Now()
		}

		// 从内容提取标题（取前50个字符）
		if len(item.Content) > 50 {
			n.Title = item.Content[:50] + "..."
		} else {
			n.Title = item.Content
		}

		news = append(news, n)
	}

	return news, nil
}

// GetCalendar 获取财经日历
func (j *Jin10Source) GetCalendar(ctx context.Context, date time.Time) ([]*types.EconomicEvent, error) {
	dateStr := date.Format("2006-01-02")
	reqURL := fmt.Sprintf("https://cdn-rili.jin10.com/data/%s/economics.json?t=%d",
		dateStr, time.Now().UnixMilli())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Referer", "https://www.jin10.com/")

	resp, err := j.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response failed: %w", err)
	}

	var items []struct {
		ID         string `json:"id"`
		PubTime    string `json:"pub_time"`
		Title      string `json:"name"`
		Country    string `json:"country"`
		Importance int    `json:"star"`
		Actual     string `json:"actual"`
		Previous   string `json:"previous"`
		Consensus  string `json:"consensus"`
		Unit       string `json:"unit"`
	}

	if err := json.Unmarshal(body, &items); err != nil {
		return nil, fmt.Errorf("parse json failed: %w", err)
	}

	events := make([]*types.EconomicEvent, 0, len(items))
	for _, item := range items {
		event := &types.EconomicEvent{
			ID:         item.ID,
			Title:      item.Title,
			Country:    item.Country,
			Importance: item.Importance,
			Previous:   item.Previous,
			Consensus:  item.Consensus,
			Actual:     item.Actual,
			Unit:       item.Unit,
		}

		// 解析时间
		if t, err := time.Parse("2006-01-02 15:04:05", item.PubTime); err == nil {
			event.Time = t
		}

		events = append(events, event)
	}

	return events, nil
}

// GetImportantNews 获取重要财经资讯
func (j *Jin10Source) GetImportantNews(ctx context.Context, limit int) ([]*types.News, error) {
	allNews, err := j.GetNews(ctx, &NewsOptions{Limit: limit * 3})
	if err != nil {
		return nil, err
	}

	// 过滤重要新闻(important >= 1)
	important := make([]*types.News, 0)
	for _, n := range allNews {
		if n.Importance >= 1 {
			important = append(important, n)
			if len(important) >= limit {
				break
			}
		}
	}

	return important, nil
}

// HealthCheck 健康检查
func (j *Jin10Source) HealthCheck(ctx context.Context) error {
	_, err := j.GetNews(ctx, &NewsOptions{Limit: 1})
	return err
}
