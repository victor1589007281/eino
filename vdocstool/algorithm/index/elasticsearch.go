// Package index Elasticsearch 倒排索引实现
package index

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/cloudwego/eino/vdocstool/algorithm/types"
)

// ESIndex Elasticsearch 索引
type ESIndex struct {
	config     *types.ESConfig
	httpClient *http.Client
	baseURL    string
	indexName  string
}

// NewESIndex 创建 Elasticsearch 索引
func NewESIndex(cfg *types.ESConfig) (*ESIndex, error) {
	if len(cfg.Addresses) == 0 {
		return nil, fmt.Errorf("no elasticsearch addresses configured")
	}

	idx := &ESIndex{
		config: cfg,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseURL:   cfg.Addresses[0],
		indexName: cfg.IndexName,
	}

	// 确保索引存在
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := idx.ensureIndex(ctx); err != nil {
		return nil, fmt.Errorf("ensure index: %w", err)
	}

	return idx, nil
}

// ensureIndex 确保索引存在
func (e *ESIndex) ensureIndex(ctx context.Context) error {
	// 检查索引是否存在
	req, _ := http.NewRequestWithContext(ctx, "HEAD", e.baseURL+"/"+e.indexName, nil)
	e.setAuth(req)

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()

	if resp.StatusCode == 200 {
		return nil // 索引已存在
	}

	// 创建索引，配置 IK 中文分词
	mapping := map[string]interface{}{
		"settings": map[string]interface{}{
			"number_of_shards":   e.config.Shards,
			"number_of_replicas": e.config.Replicas,
			"analysis": map[string]interface{}{
				"analyzer": map[string]interface{}{
					"ik_smart_analyzer": map[string]interface{}{
						"type":      "custom",
						"tokenizer": "ik_smart",
					},
					"ik_max_analyzer": map[string]interface{}{
						"type":      "custom",
						"tokenizer": "ik_max_word",
					},
				},
			},
		},
		"mappings": map[string]interface{}{
			"properties": map[string]interface{}{
				"id":         map[string]interface{}{"type": "keyword"},
				"type":       map[string]interface{}{"type": "keyword"},
				"session_id": map[string]interface{}{"type": "keyword"},
				"title": map[string]interface{}{
					"type":            "text",
					"analyzer":        "ik_max_word",
					"search_analyzer": "ik_smart",
				},
				"summary": map[string]interface{}{
					"type":            "text",
					"analyzer":        "ik_max_word",
					"search_analyzer": "ik_smart",
				},
				"content": map[string]interface{}{
					"type":            "text",
					"analyzer":        "ik_max_word",
					"search_analyzer": "ik_smart",
				},
				"keywords":   map[string]interface{}{"type": "keyword"},
				"created_at": map[string]interface{}{"type": "date"},
				"updated_at": map[string]interface{}{"type": "date"},
			},
		},
	}

	body, _ := json.Marshal(mapping)
	req, _ = http.NewRequestWithContext(ctx, "PUT", e.baseURL+"/"+e.indexName, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	e.setAuth(req)

	resp, err = e.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("create index failed: %s", string(respBody))
	}

	return nil
}

// setAuth 设置认证
func (e *ESIndex) setAuth(req *http.Request) {
	if e.config.Username != "" && e.config.Password != "" {
		req.SetBasicAuth(e.config.Username, e.config.Password)
	}
}

// Index 索引文档
func (e *ESIndex) Index(ctx context.Context, doc *Document) error {
	esDoc := map[string]interface{}{
		"id":         doc.ID,
		"type":       doc.Type,
		"title":      doc.Title,
		"summary":    doc.Summary,
		"content":    doc.Content,
		"keywords":   doc.Keywords,
		"session_id": doc.SessionID,
		"created_at": doc.CreatedAt,
		"updated_at": doc.UpdatedAt,
	}

	body, _ := json.Marshal(esDoc)
	url := fmt.Sprintf("%s/%s/_doc/%s", e.baseURL, e.indexName, doc.ID)
	req, _ := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	e.setAuth(req)

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("index failed: %s", string(respBody))
	}

	return nil
}

// IndexBatch 批量索引
func (e *ESIndex) IndexBatch(ctx context.Context, docs []*Document) error {
	if len(docs) == 0 {
		return nil
	}

	// 构建 bulk 请求
	var buf bytes.Buffer
	for _, doc := range docs {
		// action 行
		action := map[string]interface{}{
			"index": map[string]interface{}{
				"_index": e.indexName,
				"_id":    doc.ID,
			},
		}
		actionBytes, _ := json.Marshal(action)
		buf.Write(actionBytes)
		buf.WriteByte('\n')

		// document 行
		esDoc := map[string]interface{}{
			"id":         doc.ID,
			"type":       doc.Type,
			"title":      doc.Title,
			"summary":    doc.Summary,
			"content":    doc.Content,
			"keywords":   doc.Keywords,
			"session_id": doc.SessionID,
			"created_at": doc.CreatedAt,
			"updated_at": doc.UpdatedAt,
		}
		docBytes, _ := json.Marshal(esDoc)
		buf.Write(docBytes)
		buf.WriteByte('\n')
	}

	req, _ := http.NewRequestWithContext(ctx, "POST", e.baseURL+"/_bulk", &buf)
	req.Header.Set("Content-Type", "application/x-ndjson")
	e.setAuth(req)

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("bulk index failed: %s", string(respBody))
	}

	return nil
}

// Delete 删除文档
func (e *ESIndex) Delete(ctx context.Context, id string) error {
	url := fmt.Sprintf("%s/%s/_doc/%s", e.baseURL, e.indexName, id)
	req, _ := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	e.setAuth(req)

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()

	return nil
}

// Search 搜索
func (e *ESIndex) Search(ctx context.Context, query *SearchQuery) (*SearchResult, error) {
	return e.search(ctx, query.Text, query.Filters, query.TopK, query.Highlight)
}

// SearchKeyword 关键词搜索
func (e *ESIndex) SearchKeyword(ctx context.Context, keyword string, topK int) ([]*SearchHit, error) {
	result, err := e.search(ctx, keyword, nil, topK, true)
	if err != nil {
		return nil, err
	}
	return result.Hits, nil
}

// search 内部搜索实现
func (e *ESIndex) search(ctx context.Context, text string, filters *Filters, topK int, highlight bool) (*SearchResult, error) {
	// 构建查询
	esQuery := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": []interface{}{
					map[string]interface{}{
						"multi_match": map[string]interface{}{
							"query":  text,
							"fields": []string{"title^3", "summary^2", "content", "keywords^2"},
							"type":   "best_fields",
						},
					},
				},
			},
		},
		"size": topK,
	}

	// 添加过滤条件
	if filters != nil {
		filterClauses := make([]interface{}, 0)

		if filters.SessionID != "" {
			filterClauses = append(filterClauses, map[string]interface{}{
				"term": map[string]interface{}{"session_id": filters.SessionID},
			})
		}

		if filters.Type != "" {
			filterClauses = append(filterClauses, map[string]interface{}{
				"term": map[string]interface{}{"type": filters.Type},
			})
		}

		if filters.Since != nil {
			filterClauses = append(filterClauses, map[string]interface{}{
				"range": map[string]interface{}{
					"created_at": map[string]interface{}{"gte": filters.Since.Format(time.RFC3339)},
				},
			})
		}

		if len(filterClauses) > 0 {
			esQuery["query"].(map[string]interface{})["bool"].(map[string]interface{})["filter"] = filterClauses
		}
	}

	// 添加高亮
	if highlight {
		esQuery["highlight"] = map[string]interface{}{
			"fields": map[string]interface{}{
				"title":   map[string]interface{}{},
				"summary": map[string]interface{}{},
				"content": map[string]interface{}{},
			},
			"pre_tags":  []string{"<em>"},
			"post_tags": []string{"</em>"},
		}
	}

	body, _ := json.Marshal(esQuery)
	url := fmt.Sprintf("%s/%s/_search", e.baseURL, e.indexName)
	req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	e.setAuth(req)

	start := time.Now()
	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	took := time.Since(start).Milliseconds()

	// 解析响应
	var esResp struct {
		Hits struct {
			Total struct {
				Value int64 `json:"value"`
			} `json:"total"`
			MaxScore float64 `json:"max_score"`
			Hits     []struct {
				ID        string              `json:"_id"`
				Score     float64             `json:"_score"`
				Source    json.RawMessage     `json:"_source"`
				Highlight map[string][]string `json:"highlight"`
			} `json:"hits"`
		} `json:"hits"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&esResp); err != nil {
		return nil, err
	}

	// 转换结果
	hits := make([]*SearchHit, 0, len(esResp.Hits.Hits))
	for _, h := range esResp.Hits.Hits {
		var doc Document
		json.Unmarshal(h.Source, &doc)

		hits = append(hits, &SearchHit{
			ID:         h.ID,
			Score:      h.Score,
			Source:     &doc,
			Highlights: h.Highlight,
			Sources:    []string{"keyword"},
		})
	}

	return &SearchResult{
		Hits:     hits,
		Total:    esResp.Hits.Total.Value,
		TookMs:   took,
		MaxScore: esResp.Hits.MaxScore,
	}, nil
}

// HealthCheck 健康检查
func (e *ESIndex) HealthCheck(ctx context.Context) error {
	req, _ := http.NewRequestWithContext(ctx, "GET", e.baseURL+"/_cluster/health", nil)
	e.setAuth(req)

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("es unhealthy: status %d", resp.StatusCode)
	}

	return nil
}

// Close 关闭
func (e *ESIndex) Close() error {
	e.httpClient.CloseIdleConnections()
	return nil
}

// Refresh 刷新索引
func (e *ESIndex) Refresh(ctx context.Context) error {
	url := fmt.Sprintf("%s/%s/_refresh", e.baseURL, e.indexName)
	req, _ := http.NewRequestWithContext(ctx, "POST", url, nil)
	e.setAuth(req)

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()

	return nil
}

// GetDocCount 获取文档数量
func (e *ESIndex) GetDocCount(ctx context.Context) (int64, error) {
	url := fmt.Sprintf("%s/%s/_count", e.baseURL, e.indexName)
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	e.setAuth(req)

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var result struct {
		Count int64 `json:"count"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}

	return result.Count, nil
}

// GetIndexStats 获取索引统计
func (e *ESIndex) GetIndexStats(ctx context.Context) (*ESIndexStats, error) {
	url := fmt.Sprintf("%s/%s/_stats", e.baseURL, e.indexName)
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	e.setAuth(req)

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		All struct {
			Primaries struct {
				Docs struct {
					Count   int64 `json:"count"`
					Deleted int64 `json:"deleted"`
				} `json:"docs"`
				Store struct {
					SizeInBytes int64 `json:"size_in_bytes"`
				} `json:"store"`
			} `json:"primaries"`
		} `json:"_all"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &ESIndexStats{
		DocCount:    result.All.Primaries.Docs.Count,
		DeletedDocs: result.All.Primaries.Docs.Deleted,
		SizeBytes:   result.All.Primaries.Store.SizeInBytes,
	}, nil
}

// ESIndexStats ES索引统计
type ESIndexStats struct {
	DocCount    int64 `json:"doc_count"`
	DeletedDocs int64 `json:"deleted_docs"`
	SizeBytes   int64 `json:"size_bytes"`
}

// 确保 ESIndex 实现了 InvertedIndex 接口
var _ InvertedIndex = (*ESIndex)(nil)
