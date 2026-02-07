// Package news 财经新闻数据源
package news

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/cloudwego/eino/vdocstool/tools/finance/types"
)

// CLSSource 财联社数据源
type CLSSource struct {
	client *http.Client
}

// NewCLSSource 创建财联社数据源
func NewCLSSource() *CLSSource {
	return &CLSSource{
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Name 数据源名称
func (c *CLSSource) Name() string {
	return "cls"
}

// GetNews 获取财联社快讯
func (c *CLSSource) GetNews(ctx context.Context, opts *NewsOptions) ([]*types.News, error) {
	if opts == nil {
		opts = &NewsOptions{Limit: 50}
	}

	// 财联社电报API
	reqURL := "https://www.cls.cn/nodeapi/updateTelegraph"
	
	// 构建请求参数
	params := fmt.Sprintf("?app=CailianpressWeb&lastTime=%d&os=web&rn=%d&sv=7.7.5",
		time.Now().Unix(), opts.Limit)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL+params, nil)
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Referer", "https://www.cls.cn/telegraph")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response failed: %w", err)
	}

	return c.parseNewsResponse(body)
}

// parseNewsResponse 解析新闻响应
func (c *CLSSource) parseNewsResponse(body []byte) ([]*types.News, error) {
	var result struct {
		Code int    `json:"errno"`
		Msg  string `json:"errmsg"`
		Data struct {
			RollData []struct {
				ID          int64  `json:"id"`
				Title       string `json:"title"`
				Content     string `json:"content"`
				Brief       string `json:"brief"`
				CTime       int64  `json:"ctime"`
				Level       string `json:"level"`       // 重要性级别
				Subjects    []struct {
					SubjectName string `json:"subject_name"`
				} `json:"subjects"`
				Stocks []struct {
					Name   string `json:"name"`
					Symbol string `json:"symbol"`
				} `json:"stocks"`
			} `json:"roll_data"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse json failed: %w", err)
	}

	if result.Code != 0 {
		return nil, fmt.Errorf("API error: %s", result.Msg)
	}

	news := make([]*types.News, 0, len(result.Data.RollData))
	for _, item := range result.Data.RollData {
		n := &types.News{
			ID:          strconv.FormatInt(item.ID, 10),
			Title:       item.Title,
			Content:     item.Content,
			Summary:     item.Brief,
			Source:      c.Name(),
			PublishTime: time.Unix(item.CTime, 0),
			Category:    "快讯",
		}

		// 解析重要性
		switch item.Level {
		case "A":
			n.Importance = 5
		case "B":
			n.Importance = 3
		default:
			n.Importance = 1
		}

		// 关联标签
		for _, subject := range item.Subjects {
			n.Tags = append(n.Tags, subject.SubjectName)
		}

		// 关联股票
		for _, stock := range item.Stocks {
			n.Symbols = append(n.Symbols, stock.Symbol)
		}

		news = append(news, n)
	}

	return news, nil
}

// GetFlash 获取实时快讯
func (c *CLSSource) GetFlash(ctx context.Context, limit int) ([]*types.News, error) {
	return c.GetNews(ctx, &NewsOptions{
		Limit:    limit,
		Category: "flash",
	})
}

// GetImportantNews 获取重要新闻
func (c *CLSSource) GetImportantNews(ctx context.Context, limit int) ([]*types.News, error) {
	allNews, err := c.GetNews(ctx, &NewsOptions{Limit: limit * 3})
	if err != nil {
		return nil, err
	}

	// 过滤重要新闻
	important := make([]*types.News, 0)
	for _, n := range allNews {
		if n.Importance >= 3 {
			important = append(important, n)
			if len(important) >= limit {
				break
			}
		}
	}

	return important, nil
}

// SearchNews 搜索新闻
func (c *CLSSource) SearchNews(ctx context.Context, keyword string, limit int) ([]*types.News, error) {
	// 财联社搜索API
	reqURL := fmt.Sprintf("https://www.cls.cn/api/searchV4?keyword=%s&type=1&page=1&rn=%d",
		keyword, limit)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Referer", "https://www.cls.cn/")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response failed: %w", err)
	}

	var result struct {
		Code int `json:"errno"`
		Data struct {
			Article []struct {
				ID      int64  `json:"id"`
				Title   string `json:"title"`
				Brief   string `json:"brief"`
				CTime   int64  `json:"ctime"`
			} `json:"article"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse json failed: %w", err)
	}

	news := make([]*types.News, 0, len(result.Data.Article))
	for _, item := range result.Data.Article {
		news = append(news, &types.News{
			ID:          strconv.FormatInt(item.ID, 10),
			Title:       item.Title,
			Summary:     item.Brief,
			Source:      c.Name(),
			PublishTime: time.Unix(item.CTime, 0),
			Category:    "文章",
		})
	}

	return news, nil
}

// GetStockNews 获取个股新闻
func (c *CLSSource) GetStockNews(ctx context.Context, symbol string, limit int) ([]*types.News, error) {
	// 先获取所有新闻，然后过滤
	allNews, err := c.GetNews(ctx, &NewsOptions{Limit: 200})
	if err != nil {
		return nil, err
	}

	// 过滤与指定股票相关的新闻
	symbol = strings.ToUpper(symbol)
	related := make([]*types.News, 0)
	for _, n := range allNews {
		for _, s := range n.Symbols {
			if strings.Contains(strings.ToUpper(s), symbol) {
				related = append(related, n)
				break
			}
		}
		if len(related) >= limit {
			break
		}
	}

	return related, nil
}

// HealthCheck 健康检查
func (c *CLSSource) HealthCheck(ctx context.Context) error {
	_, err := c.GetNews(ctx, &NewsOptions{Limit: 1})
	return err
}
