// Package testutil 存储组件 Mock
package testutil

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
)

// ====================== Elasticsearch Mock ======================

// MockElasticsearch 模拟 Elasticsearch 客户端
type MockElasticsearch struct {
	indices map[string]*MockIndex
	mu      sync.RWMutex
}

// MockIndex 模拟索引
type MockIndex struct {
	Documents map[string]map[string]interface{}
	Settings  map[string]interface{}
}

// NewMockElasticsearch 创建 Mock ES
func NewMockElasticsearch() *MockElasticsearch {
	return &MockElasticsearch{
		indices: make(map[string]*MockIndex),
	}
}

// CreateIndex 创建索引
func (es *MockElasticsearch) CreateIndex(ctx context.Context, indexName string, settings map[string]interface{}) error {
	es.mu.Lock()
	defer es.mu.Unlock()

	es.indices[indexName] = &MockIndex{
		Documents: make(map[string]map[string]interface{}),
		Settings:  settings,
	}
	return nil
}

// Index 索引文档
func (es *MockElasticsearch) Index(ctx context.Context, indexName, docID string, doc map[string]interface{}) error {
	es.mu.Lock()
	defer es.mu.Unlock()

	index, ok := es.indices[indexName]
	if !ok {
		index = &MockIndex{
			Documents: make(map[string]map[string]interface{}),
		}
		es.indices[indexName] = index
	}

	index.Documents[docID] = doc
	return nil
}

// Get 获取文档
func (es *MockElasticsearch) Get(ctx context.Context, indexName, docID string) (map[string]interface{}, error) {
	es.mu.RLock()
	defer es.mu.RUnlock()

	index, ok := es.indices[indexName]
	if !ok {
		return nil, fmt.Errorf("index not found")
	}

	doc, ok := index.Documents[docID]
	if !ok {
		return nil, fmt.Errorf("document not found")
	}

	return doc, nil
}

// Search 搜索文档
func (es *MockElasticsearch) Search(ctx context.Context, indexName string, query map[string]interface{}) ([]map[string]interface{}, error) {
	es.mu.RLock()
	defer es.mu.RUnlock()

	index, ok := es.indices[indexName]
	if !ok {
		return nil, fmt.Errorf("index not found")
	}

	// 简化搜索：返回所有文档
	var results []map[string]interface{}
	for id, doc := range index.Documents {
		result := map[string]interface{}{
			"_id":     id,
			"_source": doc,
			"_score":  1.0,
		}
		results = append(results, result)
	}

	return results, nil
}

// Delete 删除文档
func (es *MockElasticsearch) Delete(ctx context.Context, indexName, docID string) error {
	es.mu.Lock()
	defer es.mu.Unlock()

	index, ok := es.indices[indexName]
	if !ok {
		return nil
	}

	delete(index.Documents, docID)
	return nil
}

// Count 统计文档数量
func (es *MockElasticsearch) Count(ctx context.Context, indexName string) (int64, error) {
	es.mu.RLock()
	defer es.mu.RUnlock()

	index, ok := es.indices[indexName]
	if !ok {
		return 0, nil
	}

	return int64(len(index.Documents)), nil
}

// Close 关闭
func (es *MockElasticsearch) Close() error {
	return nil
}

// ====================== Milvus Mock ======================

// MockMilvus 模拟 Milvus 客户端
type MockMilvus struct {
	collections map[string]*MockCollection
	mu          sync.RWMutex
}

// MockCollection 模拟集合
type MockCollection struct {
	Name      string
	Dimension int
	Vectors   []*MockVector
}

// MockVector 模拟向量
type MockVector struct {
	ID        string
	Vector    []float64
	Metadata  map[string]interface{}
	CreatedAt time.Time
}

// NewMockMilvus 创建 Mock Milvus
func NewMockMilvus() *MockMilvus {
	return &MockMilvus{
		collections: make(map[string]*MockCollection),
	}
}

// CreateCollection 创建集合
func (m *MockMilvus) CreateCollection(ctx context.Context, name string, dimension int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.collections[name] = &MockCollection{
		Name:      name,
		Dimension: dimension,
		Vectors:   make([]*MockVector, 0),
	}
	return nil
}

// Insert 插入向量
func (m *MockMilvus) Insert(ctx context.Context, collectionName string, id string, vector []float64, metadata map[string]interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	collection, ok := m.collections[collectionName]
	if !ok {
		return fmt.Errorf("collection not found")
	}

	collection.Vectors = append(collection.Vectors, &MockVector{
		ID:        id,
		Vector:    vector,
		Metadata:  metadata,
		CreatedAt: time.Now(),
	})
	return nil
}

// Search 向量搜索
func (m *MockMilvus) Search(ctx context.Context, collectionName string, queryVector []float64, topK int) ([]*VectorSearchResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	collection, ok := m.collections[collectionName]
	if !ok {
		return nil, fmt.Errorf("collection not found")
	}

	// 计算相似度并排序
	type scored struct {
		vector *MockVector
		score  float64
	}
	var scored_vectors []scored

	for _, v := range collection.Vectors {
		sim := cosineSimilarity(queryVector, v.Vector)
		scored_vectors = append(scored_vectors, scored{v, sim})
	}

	// 按相似度排序
	sort.Slice(scored_vectors, func(i, j int) bool {
		return scored_vectors[i].score > scored_vectors[j].score
	})

	// 取 TopK
	if len(scored_vectors) > topK {
		scored_vectors = scored_vectors[:topK]
	}

	var results []*VectorSearchResult
	for _, sv := range scored_vectors {
		results = append(results, &VectorSearchResult{
			ID:       sv.vector.ID,
			Score:    sv.score,
			Metadata: sv.vector.Metadata,
		})
	}

	return results, nil
}

// VectorSearchResult 向量搜索结果
type VectorSearchResult struct {
	ID       string
	Score    float64
	Metadata map[string]interface{}
}

// Delete 删除向量
func (m *MockMilvus) Delete(ctx context.Context, collectionName, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	collection, ok := m.collections[collectionName]
	if !ok {
		return nil
	}

	for i, v := range collection.Vectors {
		if v.ID == id {
			collection.Vectors = append(collection.Vectors[:i], collection.Vectors[i+1:]...)
			break
		}
	}
	return nil
}

// Count 统计向量数量
func (m *MockMilvus) Count(ctx context.Context, collectionName string) (int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	collection, ok := m.collections[collectionName]
	if !ok {
		return 0, nil
	}

	return int64(len(collection.Vectors)), nil
}

// Close 关闭
func (m *MockMilvus) Close() error {
	return nil
}

// cosineSimilarity 计算余弦相似度
func cosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}

	var dot, normA, normB float64
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}

// ====================== S3 Mock ======================

// MockS3 模拟 S3 客户端
type MockS3 struct {
	buckets map[string]map[string][]byte
	mu      sync.RWMutex
}

// NewMockS3 创建 Mock S3
func NewMockS3() *MockS3 {
	return &MockS3{
		buckets: make(map[string]map[string][]byte),
	}
}

// CreateBucket 创建桶
func (s *MockS3) CreateBucket(ctx context.Context, bucket string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.buckets[bucket] = make(map[string][]byte)
	return nil
}

// PutObject 上传对象
func (s *MockS3) PutObject(ctx context.Context, bucket, key string, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.buckets[bucket] == nil {
		s.buckets[bucket] = make(map[string][]byte)
	}
	s.buckets[bucket][key] = data
	return nil
}

// PutObjectCompressed 上传压缩对象
func (s *MockS3) PutObjectCompressed(ctx context.Context, bucket, key string, data []byte) error {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	gz.Write(data)
	gz.Close()

	return s.PutObject(ctx, bucket, key, buf.Bytes())
}

// GetObject 获取对象
func (s *MockS3) GetObject(ctx context.Context, bucket, key string) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	b, ok := s.buckets[bucket]
	if !ok {
		return nil, fmt.Errorf("bucket not found")
	}

	data, ok := b[key]
	if !ok {
		return nil, fmt.Errorf("object not found")
	}

	return data, nil
}

// GetObjectDecompressed 获取解压缩对象
func (s *MockS3) GetObjectDecompressed(ctx context.Context, bucket, key string) ([]byte, error) {
	data, err := s.GetObject(ctx, bucket, key)
	if err != nil {
		return nil, err
	}

	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return data, nil // 可能没有压缩
	}
	defer gz.Close()

	return io.ReadAll(gz)
}

// DeleteObject 删除对象
func (s *MockS3) DeleteObject(ctx context.Context, bucket, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if b, ok := s.buckets[bucket]; ok {
		delete(b, key)
	}
	return nil
}

// ListObjects 列出对象
func (s *MockS3) ListObjects(ctx context.Context, bucket, prefix string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	b, ok := s.buckets[bucket]
	if !ok {
		return nil, nil
	}

	var keys []string
	for key := range b {
		if prefix == "" || strings.HasPrefix(key, prefix) {
			keys = append(keys, key)
		}
	}
	return keys, nil
}

// Close 关闭
func (s *MockS3) Close() error {
	return nil
}

// ====================== Neo4j Mock ======================

// MockNeo4j 模拟 Neo4j 客户端
type MockNeo4j struct {
	nodes     map[string]*MockNode
	relations []*MockRelation
	mu        sync.RWMutex
}

// MockNode 模拟节点
type MockNode struct {
	ID         string
	Labels     []string
	Properties map[string]interface{}
}

// MockRelation 模拟关系
type MockRelation struct {
	ID         string
	Type       string
	FromNodeID string
	ToNodeID   string
	Properties map[string]interface{}
}

// NewMockNeo4j 创建 Mock Neo4j
func NewMockNeo4j() *MockNeo4j {
	return &MockNeo4j{
		nodes:     make(map[string]*MockNode),
		relations: make([]*MockRelation, 0),
	}
}

// CreateNode 创建节点
func (n *MockNeo4j) CreateNode(ctx context.Context, labels []string, properties map[string]interface{}) (string, error) {
	n.mu.Lock()
	defer n.mu.Unlock()

	id := fmt.Sprintf("node_%d", len(n.nodes)+1)
	n.nodes[id] = &MockNode{
		ID:         id,
		Labels:     labels,
		Properties: properties,
	}
	return id, nil
}

// GetNode 获取节点
func (n *MockNeo4j) GetNode(ctx context.Context, id string) (*MockNode, error) {
	n.mu.RLock()
	defer n.mu.RUnlock()

	node, ok := n.nodes[id]
	if !ok {
		return nil, fmt.Errorf("node not found")
	}
	return node, nil
}

// FindNodesByLabel 按标签查找节点
func (n *MockNeo4j) FindNodesByLabel(ctx context.Context, label string) ([]*MockNode, error) {
	n.mu.RLock()
	defer n.mu.RUnlock()

	var results []*MockNode
	for _, node := range n.nodes {
		for _, l := range node.Labels {
			if l == label {
				results = append(results, node)
				break
			}
		}
	}
	return results, nil
}

// FindNodesByProperty 按属性查找节点
func (n *MockNeo4j) FindNodesByProperty(ctx context.Context, key string, value interface{}) ([]*MockNode, error) {
	n.mu.RLock()
	defer n.mu.RUnlock()

	var results []*MockNode
	for _, node := range n.nodes {
		if v, ok := node.Properties[key]; ok {
			// 简化比较
			if fmt.Sprintf("%v", v) == fmt.Sprintf("%v", value) {
				results = append(results, node)
			}
		}
	}
	return results, nil
}

// CreateRelation 创建关系
func (n *MockNeo4j) CreateRelation(ctx context.Context, fromID, toID, relType string, properties map[string]interface{}) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.relations = append(n.relations, &MockRelation{
		ID:         fmt.Sprintf("rel_%d", len(n.relations)+1),
		Type:       relType,
		FromNodeID: fromID,
		ToNodeID:   toID,
		Properties: properties,
	})
	return nil
}

// GetRelations 获取节点的关系
func (n *MockNeo4j) GetRelations(ctx context.Context, nodeID string, depth int) ([]*MockRelation, error) {
	n.mu.RLock()
	defer n.mu.RUnlock()

	var results []*MockRelation
	visited := make(map[string]bool)
	n.traverseRelations(nodeID, depth, visited, &results)
	return results, nil
}

// traverseRelations 遍历关系
func (n *MockNeo4j) traverseRelations(nodeID string, depth int, visited map[string]bool, results *[]*MockRelation) {
	if depth <= 0 || visited[nodeID] {
		return
	}
	visited[nodeID] = true

	for _, rel := range n.relations {
		if rel.FromNodeID == nodeID || rel.ToNodeID == nodeID {
			*results = append(*results, rel)
			
			// 继续遍历
			nextID := rel.ToNodeID
			if rel.ToNodeID == nodeID {
				nextID = rel.FromNodeID
			}
			n.traverseRelations(nextID, depth-1, visited, results)
		}
	}
}

// DeleteNode 删除节点
func (n *MockNeo4j) DeleteNode(ctx context.Context, id string) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	delete(n.nodes, id)

	// 删除相关关系
	var newRelations []*MockRelation
	for _, rel := range n.relations {
		if rel.FromNodeID != id && rel.ToNodeID != id {
			newRelations = append(newRelations, rel)
		}
	}
	n.relations = newRelations

	return nil
}

// Count 统计节点和关系数量
func (n *MockNeo4j) Count(ctx context.Context) (nodeCount, relationCount int64, err error) {
	n.mu.RLock()
	defer n.mu.RUnlock()

	return int64(len(n.nodes)), int64(len(n.relations)), nil
}

// Close 关闭
func (n *MockNeo4j) Close() error {
	return nil
}

// ExportToJSON 导出为 JSON（用于调试）
func (n *MockNeo4j) ExportToJSON() (string, error) {
	n.mu.RLock()
	defer n.mu.RUnlock()

	data := map[string]interface{}{
		"nodes":     n.nodes,
		"relations": n.relations,
	}
	b, err := json.MarshalIndent(data, "", "  ")
	return string(b), err
}
