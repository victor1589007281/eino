// Package index 索引管理器
package index

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/cloudwego/eino/vdocstool/algorithm/types"
)

// Manager 索引管理器
type Manager struct {
	config *types.IndexConfig

	// 各索引实例
	esIndex     InvertedIndex
	milvusIndex VectorIndex
	graphIndex  GraphIndex

	// 混合搜索器
	hybrid *HybridSearcher

	mu sync.RWMutex
}

// NewManager 创建索引管理器
func NewManager(cfg *types.IndexConfig) (*Manager, error) {
	m := &Manager{
		config: cfg,
	}

	var err error

	// 初始化 Elasticsearch
	m.esIndex, err = NewESIndex(&cfg.Elasticsearch)
	if err != nil {
		return nil, fmt.Errorf("init es index: %w", err)
	}

	// 初始化 Milvus
	m.milvusIndex, err = NewMilvusIndex(&cfg.Milvus)
	if err != nil {
		// Milvus 失败不阻止启动
		fmt.Printf("warning: init milvus index failed: %v\n", err)
	}

	// 初始化图索引（如果配置了 Neo4j）
	if cfg.Neo4j.URI != "" {
		m.graphIndex, err = NewNeo4jIndex(&cfg.Neo4j)
		if err != nil {
			fmt.Printf("warning: init neo4j index failed: %v\n", err)
		}
	}

	// 创建混合搜索器
	m.hybrid = NewHybridSearcher(m.esIndex, m.milvusIndex, m.graphIndex)

	return m, nil
}

// Index 索引文档
func (m *Manager) Index(ctx context.Context, doc *Document) error {
	var errs []error
	var wg sync.WaitGroup

	// 并行索引到各存储
	// 1. ES 倒排索引
	if m.esIndex != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := m.esIndex.Index(ctx, doc); err != nil {
				errs = append(errs, fmt.Errorf("es: %w", err))
			}
		}()
	}

	// 2. Milvus 向量索引
	if m.milvusIndex != nil && len(doc.Vector) > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := m.milvusIndex.UpsertVector(ctx, doc.ID, doc.Vector, map[string]interface{}{
				"session_id": doc.SessionID,
				"type":       doc.Type,
				"created_at": doc.CreatedAt.Unix(),
			}); err != nil {
				errs = append(errs, fmt.Errorf("milvus: %w", err))
			}
		}()
	}

	// 3. 图索引
	if m.graphIndex != nil && len(doc.Entities) > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for _, entity := range doc.Entities {
				if err := m.graphIndex.IndexEntity(ctx, entity); err != nil {
					errs = append(errs, fmt.Errorf("neo4j entity: %w", err))
				}
			}
			for _, relation := range doc.Relations {
				if err := m.graphIndex.IndexRelation(ctx, relation); err != nil {
					errs = append(errs, fmt.Errorf("neo4j relation: %w", err))
				}
			}
		}()
	}

	wg.Wait()

	if len(errs) > 0 {
		return fmt.Errorf("index errors: %v", errs)
	}
	return nil
}

// IndexBatch 批量索引
func (m *Manager) IndexBatch(ctx context.Context, docs []*Document) error {
	// ES 支持批量
	if m.esIndex != nil {
		if err := m.esIndex.IndexBatch(ctx, docs); err != nil {
			return fmt.Errorf("es batch: %w", err)
		}
	}

	// Milvus 和图索引逐个处理
	for _, doc := range docs {
		if m.milvusIndex != nil && len(doc.Vector) > 0 {
			if err := m.milvusIndex.UpsertVector(ctx, doc.ID, doc.Vector, map[string]interface{}{
				"session_id": doc.SessionID,
				"type":       doc.Type,
			}); err != nil {
				return fmt.Errorf("milvus: %w", err)
			}
		}
	}

	return nil
}

// Search 混合搜索
func (m *Manager) Search(ctx context.Context, query *SearchQuery) (*SearchResult, error) {
	return m.hybrid.Search(ctx, query)
}

// SearchKeyword 关键词搜索
func (m *Manager) SearchKeyword(ctx context.Context, text string, topK int) (*SearchResult, error) {
	if m.esIndex == nil {
		return nil, fmt.Errorf("es index not available")
	}

	hits, err := m.esIndex.SearchKeyword(ctx, text, topK)
	if err != nil {
		return nil, err
	}

	return &SearchResult{
		Hits:  hits,
		Total: int64(len(hits)),
	}, nil
}

// SearchVector 向量搜索
func (m *Manager) SearchVector(ctx context.Context, vector []float64, topK int) (*SearchResult, error) {
	if m.milvusIndex == nil {
		return nil, fmt.Errorf("milvus index not available")
	}

	hits, err := m.milvusIndex.SearchVector(ctx, vector, topK, "")
	if err != nil {
		return nil, err
	}

	return &SearchResult{
		Hits:  hits,
		Total: int64(len(hits)),
	}, nil
}

// Delete 删除文档
func (m *Manager) Delete(ctx context.Context, id string) error {
	var errs []error

	if m.esIndex != nil {
		if err := m.esIndex.Delete(ctx, id); err != nil {
			errs = append(errs, err)
		}
	}

	if m.milvusIndex != nil {
		if err := m.milvusIndex.Delete(ctx, id); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("delete errors: %v", errs)
	}
	return nil
}

// HealthCheck 健康检查
func (m *Manager) HealthCheck(ctx context.Context) error {
	// 至少 ES 可用即为健康
	if m.esIndex != nil {
		if err := m.esIndex.HealthCheck(ctx); err != nil {
			return fmt.Errorf("es: %w", err)
		}
	}
	return nil
}

// Close 关闭
func (m *Manager) Close() error {
	var errs []error

	if m.esIndex != nil {
		if err := m.esIndex.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if m.milvusIndex != nil {
		if err := m.milvusIndex.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if m.graphIndex != nil {
		if err := m.graphIndex.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("close errors: %v", errs)
	}
	return nil
}

// GetStats 获取索引统计
func (m *Manager) GetStats(ctx context.Context) *IndexStats {
	stats := &IndexStats{
		Timestamp: time.Now(),
	}

	var wg sync.WaitGroup

	// 获取 ES 统计
	if m.esIndex != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if esIdx, ok := m.esIndex.(*ESIndex); ok {
				count, err := esIdx.GetDocCount(ctx)
				if err == nil {
					stats.ESDocCount = count
				}
			}
		}()
	}

	// 获取 Milvus 统计
	if m.milvusIndex != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if milvusIdx, ok := m.milvusIndex.(*MilvusIndex); ok {
				count, err := milvusIdx.GetVectorCount(ctx)
				if err == nil {
					stats.MilvusVecCount = count
				}
			}
		}()
	}

	// 获取 Neo4j 统计
	if m.graphIndex != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if neo4jIdx, ok := m.graphIndex.(*Neo4jIndex); ok {
				nodeCount, edgeCount, err := neo4jIdx.GetCounts(ctx)
				if err == nil {
					stats.Neo4jNodeCount = nodeCount
					stats.Neo4jEdgeCount = edgeCount
				}
			}
		}()
	}

	wg.Wait()
	return stats
}

// IndexStats 索引统计
type IndexStats struct {
	ESDocCount     int64     `json:"es_doc_count"`
	ESIndexSize    string    `json:"es_index_size,omitempty"`
	MilvusVecCount int64     `json:"milvus_vec_count"`
	MilvusDimension int      `json:"milvus_dimension,omitempty"`
	Neo4jNodeCount int64     `json:"neo4j_node_count"`
	Neo4jEdgeCount int64     `json:"neo4j_edge_count"`
	Timestamp      time.Time `json:"timestamp"`
}
