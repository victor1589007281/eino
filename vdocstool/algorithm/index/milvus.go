// Package index Milvus 向量索引实现
package index

import (
	"context"
	"fmt"
	"time"

	"github.com/cloudwego/eino/vdocstool/algorithm/types"
	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
)

// MilvusIndex Milvus 向量索引
type MilvusIndex struct {
	config         *types.MilvusConfig
	client         client.Client
	collectionName string
	dimension      int
}

// NewMilvusIndex 创建 Milvus 索引
func NewMilvusIndex(cfg *types.MilvusConfig) (*MilvusIndex, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 连接 Milvus
	c, err := client.NewClient(ctx, client.Config{
		Address: cfg.Address,
	})
	if err != nil {
		return nil, fmt.Errorf("connect milvus: %w", err)
	}

	idx := &MilvusIndex{
		config:         cfg,
		client:         c,
		collectionName: cfg.CollectionName,
		dimension:      cfg.Dimension,
	}

	// 确保 collection 存在
	if err := idx.ensureCollection(ctx); err != nil {
		c.Close()
		return nil, fmt.Errorf("ensure collection: %w", err)
	}

	return idx, nil
}

// ensureCollection 确保 collection 存在
func (m *MilvusIndex) ensureCollection(ctx context.Context) error {
	// 检查 collection 是否存在
	exists, err := m.client.HasCollection(ctx, m.collectionName)
	if err != nil {
		return err
	}

	if exists {
		// 加载 collection
		return m.client.LoadCollection(ctx, m.collectionName, false)
	}

	// 创建 collection
	schema := &entity.Schema{
		CollectionName: m.collectionName,
		Description:    "VDocs vector storage",
		Fields: []*entity.Field{
			{
				Name:       "id",
				DataType:   entity.FieldTypeVarChar,
				PrimaryKey: true,
				AutoID:     false,
				TypeParams: map[string]string{
					"max_length": "64",
				},
			},
			{
				Name:     "vector",
				DataType: entity.FieldTypeFloatVector,
				TypeParams: map[string]string{
					"dim": fmt.Sprintf("%d", m.dimension),
				},
			},
			{
				Name:     "session_id",
				DataType: entity.FieldTypeVarChar,
				TypeParams: map[string]string{
					"max_length": "64",
				},
			},
			{
				Name:     "doc_type",
				DataType: entity.FieldTypeVarChar,
				TypeParams: map[string]string{
					"max_length": "32",
				},
			},
			{
				Name:     "created_at",
				DataType: entity.FieldTypeInt64,
			},
		},
	}

	if err := m.client.CreateCollection(ctx, schema, 2); err != nil {
		return err
	}

	// 创建索引
	indexType := m.config.IndexType
	if indexType == "" {
		indexType = "HNSW"
	}

	metricType := entity.L2
	if m.config.MetricType == "IP" {
		metricType = entity.IP
	}

	var idx entity.Index
	switch indexType {
	case "HNSW":
		idx, err = entity.NewIndexHNSW(metricType, 16, 256)
	case "IVF_FLAT":
		idx, err = entity.NewIndexIvfFlat(metricType, 128)
	default:
		idx, err = entity.NewIndexHNSW(metricType, 16, 256)
	}

	if err != nil {
		return err
	}

	if err := m.client.CreateIndex(ctx, m.collectionName, "vector", idx, false); err != nil {
		return err
	}

	// 加载 collection
	return m.client.LoadCollection(ctx, m.collectionName, false)
}

// Index 索引文档（向量）
func (m *MilvusIndex) Index(ctx context.Context, doc *Document) error {
	if len(doc.Vector) == 0 {
		return nil
	}
	return m.UpsertVector(ctx, doc.ID, doc.Vector, map[string]interface{}{
		"session_id": doc.SessionID,
		"type":       doc.Type,
		"created_at": doc.CreatedAt.Unix(),
	})
}

// IndexBatch 批量索引
func (m *MilvusIndex) IndexBatch(ctx context.Context, docs []*Document) error {
	for _, doc := range docs {
		if err := m.Index(ctx, doc); err != nil {
			return err
		}
	}
	return nil
}

// UpsertVector 更新或插入向量
func (m *MilvusIndex) UpsertVector(ctx context.Context, id string, vector []float64, metadata map[string]interface{}) error {
	// 先尝试删除旧的
	_ = m.Delete(ctx, id)

	// 准备数据
	ids := []string{id}
	vectors := [][]float32{toFloat32(vector)}

	sessionID := ""
	if v, ok := metadata["session_id"].(string); ok {
		sessionID = v
	}
	sessionIDs := []string{sessionID}

	docType := ""
	if v, ok := metadata["type"].(string); ok {
		docType = v
	}
	docTypes := []string{docType}

	var createdAt int64
	if v, ok := metadata["created_at"].(int64); ok {
		createdAt = v
	}
	createdAts := []int64{createdAt}

	// 插入
	idColumn := entity.NewColumnVarChar("id", ids)
	vectorColumn := entity.NewColumnFloatVector("vector", m.dimension, vectors)
	sessionColumn := entity.NewColumnVarChar("session_id", sessionIDs)
	typeColumn := entity.NewColumnVarChar("doc_type", docTypes)
	createdColumn := entity.NewColumnInt64("created_at", createdAts)

	_, err := m.client.Insert(ctx, m.collectionName, "",
		idColumn, vectorColumn, sessionColumn, typeColumn, createdColumn)

	return err
}

// Delete 删除向量
func (m *MilvusIndex) Delete(ctx context.Context, id string) error {
	expr := fmt.Sprintf(`id == "%s"`, id)
	return m.client.Delete(ctx, m.collectionName, "", expr)
}

// Search 搜索
func (m *MilvusIndex) Search(ctx context.Context, query *SearchQuery) (*SearchResult, error) {
	if len(query.Vector) == 0 {
		return nil, fmt.Errorf("vector required for milvus search")
	}

	filter := ""
	if query.Filters != nil && query.Filters.SessionID != "" {
		filter = fmt.Sprintf(`session_id == "%s"`, query.Filters.SessionID)
	}

	hits, err := m.SearchVector(ctx, query.Vector, query.TopK, filter)
	if err != nil {
		return nil, err
	}

	return &SearchResult{
		Hits:  hits,
		Total: int64(len(hits)),
	}, nil
}

// SearchVector 向量搜索
func (m *MilvusIndex) SearchVector(ctx context.Context, vector []float64, topK int, filter string) ([]*SearchHit, error) {
	// 搜索参数
	sp, _ := entity.NewIndexHNSWSearchParam(64)

	// 执行搜索
	vectors := []entity.Vector{entity.FloatVector(toFloat32(vector))}

	results, err := m.client.Search(
		ctx,
		m.collectionName,
		nil,
		filter,
		[]string{"id", "session_id", "doc_type", "created_at"},
		vectors,
		"vector",
		entity.L2,
		topK,
		sp,
	)
	if err != nil {
		return nil, err
	}

	if len(results) == 0 {
		return nil, nil
	}

	// 转换结果
	hits := make([]*SearchHit, 0)
	for i := 0; i < results[0].ResultCount; i++ {
		var id string
		if idCol, ok := results[0].Fields.GetColumn("id").(*entity.ColumnVarChar); ok {
			id, _ = idCol.ValueByIdx(i)
		}

		hits = append(hits, &SearchHit{
			ID:             id,
			VectorDistance: float64(results[0].Scores[i]),
			Score:          1.0 / (1.0 + float64(results[0].Scores[i])), // 距离转相似度
			Sources:        []string{"vector"},
		})
	}

	return hits, nil
}

// HealthCheck 健康检查
func (m *MilvusIndex) HealthCheck(ctx context.Context) error {
	_, err := m.client.HasCollection(ctx, m.collectionName)
	return err
}

// Close 关闭
func (m *MilvusIndex) Close() error {
	return m.client.Close()
}

// GetVectorCount 获取向量数量
func (m *MilvusIndex) GetVectorCount(ctx context.Context) (int64, error) {
	// 获取 collection 统计信息
	stats, err := m.client.GetCollectionStatistics(ctx, m.collectionName)
	if err != nil {
		return 0, err
	}

	// 解析行数 - stats 是 map[string]string
	if rowCountStr, ok := stats["row_count"]; ok {
		var count int64
		fmt.Sscanf(rowCountStr, "%d", &count)
		return count, nil
	}

	return 0, nil
}

// GetCollectionInfo 获取集合信息
func (m *MilvusIndex) GetCollectionInfo(ctx context.Context) (*MilvusCollectionInfo, error) {
	// 获取 collection 描述
	collection, err := m.client.DescribeCollection(ctx, m.collectionName)
	if err != nil {
		return nil, err
	}

	// 获取统计信息
	count, _ := m.GetVectorCount(ctx)

	return &MilvusCollectionInfo{
		Name:        m.collectionName,
		Dimension:   m.dimension,
		VectorCount: count,
		Schema:      collection.Schema.CollectionName,
	}, nil
}

// MilvusCollectionInfo Milvus集合信息
type MilvusCollectionInfo struct {
	Name        string `json:"name"`
	Dimension   int    `json:"dimension"`
	VectorCount int64  `json:"vector_count"`
	Schema      string `json:"schema"`
}

// toFloat32 转换为 float32
func toFloat32(v []float64) []float32 {
	result := make([]float32, len(v))
	for i, val := range v {
		result[i] = float32(val)
	}
	return result
}

// 确保 MilvusIndex 实现了 VectorIndex 接口
var _ VectorIndex = (*MilvusIndex)(nil)
