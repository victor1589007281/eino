// Package storage L3 长期记忆存储 (S3 + Neo4j)
package storage

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"

	"github.com/cloudwego/eino/vdocstool/config"
)

// L3LongTermMemory L3 长期记忆存储
type L3LongTermMemory struct {
	cfg            config.L3Config
	enableCompress bool
	// s3Client would be added when aws-sdk-go is available
	// neo4jDriver would be added when neo4j-go-driver is available

	// 内存模拟存储（用于无外部依赖时）
	archives  map[string]*ArchivedCapsule
	entities  map[string]*Entity
	relations map[string]*Relation
}

// ArchivedCapsule 归档的胶囊
type ArchivedCapsule struct {
	Info       *ArchiveInfo   `json:"info"`
	Capsule    *TopicCapsule  `json:"capsule"`
	Compressed bool           `json:"compressed"`
	Data       []byte         `json:"data,omitempty"`
}

// NewL3LongTermMemory 创建 L3 存储
func NewL3LongTermMemory(cfg config.L3Config) (*L3LongTermMemory, error) {
	store := &L3LongTermMemory{
		cfg:            cfg,
		enableCompress: cfg.EnableCompression,
		archives:       make(map[string]*ArchivedCapsule),
		entities:       make(map[string]*Entity),
		relations:      make(map[string]*Relation),
	}

	// TODO: 初始化 S3 客户端
	// TODO: 初始化 Neo4j 驱动

	return store, nil
}

// Tier 返回存储层级
func (s *L3LongTermMemory) Tier() StorageTier {
	return TierL3
}

// Store 存储数据
func (s *L3LongTermMemory) Store(ctx context.Context, data interface{}) error {
	switch v := data.(type) {
	case *TopicCapsule:
		_, err := s.ArchiveCapsule(ctx, v)
		return err
	case *Entity:
		return s.StoreEntity(ctx, v)
	case *Relation:
		return s.StoreRelation(ctx, v)
	default:
		return fmt.Errorf("unsupported data type")
	}
}

// Get 获取数据
func (s *L3LongTermMemory) Get(ctx context.Context, id string) (interface{}, error) {
	return s.LoadCapsule(ctx, id)
}

// Delete 删除数据
func (s *L3LongTermMemory) Delete(ctx context.Context, id string) error {
	delete(s.archives, id)
	return nil
}

// Close 关闭存储
func (s *L3LongTermMemory) Close() error {
	// TODO: 关闭连接
	return nil
}

// ArchiveCapsule 归档胶囊
func (s *L3LongTermMemory) ArchiveCapsule(ctx context.Context, capsule *TopicCapsule) (string, error) {
	archiveID := uuid.New().String()

	// 序列化胶囊
	data, err := json.Marshal(capsule)
	if err != nil {
		return "", fmt.Errorf("marshal capsule failed: %w", err)
	}

	// 压缩（如果启用）
	var finalData []byte
	compressed := false
	if s.enableCompress {
		var buf bytes.Buffer
		gzWriter := gzip.NewWriter(&buf)
		if _, err := gzWriter.Write(data); err != nil {
			return "", fmt.Errorf("compress failed: %w", err)
		}
		gzWriter.Close()
		finalData = buf.Bytes()
		compressed = true
	} else {
		finalData = data
	}

	// 创建归档信息
	info := &ArchiveInfo{
		ArchiveID:  archiveID,
		CapsuleID:  capsule.ID,
		SessionID:  capsule.SessionID,
		TopicTitle: capsule.TopicTitle,
		Size:       int64(len(finalData)),
		Compressed: compressed,
		ArchivedAt: time.Now(),
	}

	// 存储到内存（实际应该存储到 S3）
	s.archives[archiveID] = &ArchivedCapsule{
		Info:       info,
		Capsule:    capsule,
		Compressed: compressed,
		Data:       finalData,
	}

	// TODO: 上传到 S3
	// s.uploadToS3(ctx, archiveID, finalData)

	// 存储实体和关系到图数据库
	for _, entity := range capsule.Entities {
		s.StoreEntity(ctx, entity)
	}
	for _, relation := range capsule.Relations {
		s.StoreRelation(ctx, relation)
	}

	return archiveID, nil
}

// LoadCapsule 加载胶囊
func (s *L3LongTermMemory) LoadCapsule(ctx context.Context, archiveID string) (*TopicCapsule, error) {
	// 从内存获取（实际应该从 S3 下载）
	archived, ok := s.archives[archiveID]
	if !ok {
		return nil, fmt.Errorf("archive not found: %s", archiveID)
	}

	// 如果已经有解析好的胶囊，直接返回
	if archived.Capsule != nil {
		return archived.Capsule, nil
	}

	// 解压并反序列化
	data := archived.Data
	if archived.Compressed {
		reader, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("decompress failed: %w", err)
		}
		defer reader.Close()

		data, err = io.ReadAll(reader)
		if err != nil {
			return nil, fmt.Errorf("read decompressed data failed: %w", err)
		}
	}

	var capsule TopicCapsule
	if err := json.Unmarshal(data, &capsule); err != nil {
		return nil, fmt.Errorf("unmarshal capsule failed: %w", err)
	}

	return &capsule, nil
}

// StoreEntity 存储实体
func (s *L3LongTermMemory) StoreEntity(ctx context.Context, entity *Entity) error {
	if entity.ID == "" {
		entity.ID = uuid.New().String()
	}
	if entity.CreatedAt.IsZero() {
		entity.CreatedAt = time.Now()
	}

	// 存储到内存（实际应该存储到 Neo4j）
	s.entities[entity.ID] = entity

	// TODO: 存储到 Neo4j
	// CREATE (e:Entity {id: $id, name: $name, type: $type, ...})

	return nil
}

// StoreRelation 存储关系
func (s *L3LongTermMemory) StoreRelation(ctx context.Context, relation *Relation) error {
	if relation.ID == "" {
		relation.ID = uuid.New().String()
	}
	if relation.CreatedAt.IsZero() {
		relation.CreatedAt = time.Now()
	}

	// 存储到内存（实际应该存储到 Neo4j）
	s.relations[relation.ID] = relation

	// TODO: 存储到 Neo4j
	// MATCH (a:Entity {name: $from}), (b:Entity {name: $to})
	// CREATE (a)-[r:RELATES {type: $type}]->(b)

	return nil
}

// QueryEntities 查询实体
func (s *L3LongTermMemory) QueryEntities(ctx context.Context, query string, limit int) ([]*Entity, error) {
	var result []*Entity

	for _, entity := range s.entities {
		if contains(entity.Name, query) || contains(entity.Type, query) {
			result = append(result, entity)
			if limit > 0 && len(result) >= limit {
				break
			}
		}
	}

	// TODO: 从 Neo4j 查询
	// MATCH (e:Entity) WHERE e.name CONTAINS $query RETURN e LIMIT $limit

	return result, nil
}

// QueryRelations 查询关系
func (s *L3LongTermMemory) QueryRelations(ctx context.Context, entityName string, depth int) ([]*Relation, error) {
	var result []*Relation

	for _, relation := range s.relations {
		if relation.FromEntity == entityName || relation.ToEntity == entityName {
			result = append(result, relation)
		}
	}

	// TODO: 从 Neo4j 查询
	// MATCH (e:Entity {name: $name})-[r*1..depth]-(related) RETURN r

	return result, nil
}

// GetArchiveList 获取归档列表
func (s *L3LongTermMemory) GetArchiveList(ctx context.Context, sessionID string) ([]*ArchiveInfo, error) {
	var result []*ArchiveInfo

	for _, archived := range s.archives {
		if sessionID == "" || archived.Info.SessionID == sessionID {
			result = append(result, archived.Info)
		}
	}

	return result, nil
}

// GetStats 获取统计信息
func (s *L3LongTermMemory) GetStats(ctx context.Context) (*L3Stats, error) {
	var totalSize int64
	for _, archived := range s.archives {
		totalSize += archived.Info.Size
	}

	return &L3Stats{
		ArchiveCount:  len(s.archives),
		TotalSize:     totalSize,
		EntityCount:   len(s.entities),
		RelationCount: len(s.relations),
	}, nil
}

// L3Stats L3 统计信息
type L3Stats struct {
	ArchiveCount  int   `json:"archive_count"`
	TotalSize     int64 `json:"total_size"`
	EntityCount   int   `json:"entity_count"`
	RelationCount int   `json:"relation_count"`
}

// contains 检查字符串是否包含子串（不区分大小写）
func contains(s, substr string) bool {
	if substr == "" {
		return true
	}
	// 简单实现
	return len(s) >= len(substr) && (s == substr || containsIgnoreCase(s, substr))
}

func containsIgnoreCase(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if equalIgnoreCase(s[i:i+len(substr)], substr) {
			return true
		}
	}
	return false
}

func equalIgnoreCase(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}
