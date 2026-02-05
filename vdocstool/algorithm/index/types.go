// Package index 索引引擎类型定义
package index

import (
	"context"
	"time"
)

// Document 索引文档
type Document struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"` // capsule, email, message
	Title       string                 `json:"title"`
	Summary     string                 `json:"summary"`
	Content     string                 `json:"content"`
	Keywords    []string               `json:"keywords,omitempty"`
	Vector      []float64              `json:"vector,omitempty"`
	Entities    []*Entity              `json:"entities,omitempty"`
	Relations   []*Relation            `json:"relations,omitempty"`
	SessionID   string                 `json:"session_id,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

// Entity 实体
type Entity struct {
	ID         string                 `json:"id"`
	Name       string                 `json:"name"`
	Type       string                 `json:"type"`
	Attributes map[string]interface{} `json:"attributes,omitempty"`
}

// Relation 关系
type Relation struct {
	FromEntity   string  `json:"from_entity"`
	ToEntity     string  `json:"to_entity"`
	RelationType string  `json:"relation_type"`
	Weight       float64 `json:"weight,omitempty"`
}

// SearchQuery 搜索查询
type SearchQuery struct {
	// Text 文本查询
	Text string `json:"text,omitempty"`
	// Vector 向量查询
	Vector []float64 `json:"vector,omitempty"`
	// Filters 过滤条件
	Filters *Filters `json:"filters,omitempty"`
	// TopK 返回数量
	TopK int `json:"top_k"`
	// UseKeyword 是否使用关键词搜索
	UseKeyword bool `json:"use_keyword"`
	// UseVector 是否使用向量搜索
	UseVector bool `json:"use_vector"`
	// UseGraph 是否使用图搜索
	UseGraph bool `json:"use_graph"`
	// Highlight 是否高亮
	Highlight bool `json:"highlight"`
}

// Filters 过滤条件
type Filters struct {
	SessionID string     `json:"session_id,omitempty"`
	Type      string     `json:"type,omitempty"`
	Types     []string   `json:"types,omitempty"`
	Since     *time.Time `json:"since,omitempty"`
	Before    *time.Time `json:"before,omitempty"`
}

// SearchResult 搜索结果
type SearchResult struct {
	Hits       []*SearchHit `json:"hits"`
	Total      int64        `json:"total"`
	TookMs     int64        `json:"took_ms"`
	MaxScore   float64      `json:"max_score"`
}

// SearchHit 搜索命中
type SearchHit struct {
	ID             string              `json:"id"`
	Score          float64             `json:"score"`
	VectorDistance float64             `json:"vector_distance,omitempty"`
	FusedScore     float64             `json:"fused_score,omitempty"`
	Source         *Document           `json:"source,omitempty"`
	Highlights     map[string][]string `json:"highlights,omitempty"`
	Sources        []string            `json:"sources,omitempty"` // keyword, vector, graph
}

// Index 索引接口
type Index interface {
	// Index 索引文档
	Index(ctx context.Context, doc *Document) error

	// IndexBatch 批量索引
	IndexBatch(ctx context.Context, docs []*Document) error

	// Delete 删除文档
	Delete(ctx context.Context, id string) error

	// Search 搜索
	Search(ctx context.Context, query *SearchQuery) (*SearchResult, error)

	// HealthCheck 健康检查
	HealthCheck(ctx context.Context) error

	// Close 关闭
	Close() error
}

// InvertedIndex 倒排索引接口
type InvertedIndex interface {
	Index

	// SearchKeyword 关键词搜索
	SearchKeyword(ctx context.Context, keyword string, topK int) ([]*SearchHit, error)
}

// VectorIndex 向量索引接口
type VectorIndex interface {
	Index

	// SearchVector 向量搜索
	SearchVector(ctx context.Context, vector []float64, topK int, filter string) ([]*SearchHit, error)

	// UpsertVector 更新或插入向量
	UpsertVector(ctx context.Context, id string, vector []float64, metadata map[string]interface{}) error
}

// GraphIndex 图索引接口
type GraphIndex interface {
	// IndexEntity 索引实体
	IndexEntity(ctx context.Context, entity *Entity) error

	// IndexRelation 索引关系
	IndexRelation(ctx context.Context, relation *Relation) error

	// QueryEntities 查询实体
	QueryEntities(ctx context.Context, query string, limit int) ([]*Entity, error)

	// QueryRelations 查询关系
	QueryRelations(ctx context.Context, entityName string, depth int) ([]*Relation, error)

	// HealthCheck 健康检查
	HealthCheck(ctx context.Context) error

	// Close 关闭
	Close() error
}
