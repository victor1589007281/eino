// Package storage L2 短期记忆存储 (Milvus + PostgreSQL)
package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/cloudwego/eino/vdocstool/config"
)

// L2ShortTermMemory L2 短期记忆存储
type L2ShortTermMemory struct {
	db             *sql.DB
	vectorDim      int
	retentionDays  int
	collectionName string
	// milvusClient would be added when milvus-sdk-go is available
}

// NewL2ShortTermMemory 创建 L2 存储
func NewL2ShortTermMemory(cfg config.L2Config) (*L2ShortTermMemory, error) {
	// 连接 PostgreSQL
	db, err := sql.Open("pgx", cfg.PostgresDSN)
	if err != nil {
		return nil, fmt.Errorf("postgres connect failed: %w", err)
	}

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("postgres ping failed: %w", err)
	}

	store := &L2ShortTermMemory{
		db:             db,
		vectorDim:      cfg.VectorDimension,
		retentionDays:  cfg.RetentionDays,
		collectionName: cfg.CollectionName,
	}

	// 初始化表结构
	if err := store.initSchema(ctx); err != nil {
		return nil, fmt.Errorf("init schema failed: %w", err)
	}

	return store, nil
}

// initSchema 初始化表结构
func (s *L2ShortTermMemory) initSchema(ctx context.Context) error {
	schema := `
	CREATE TABLE IF NOT EXISTS topic_capsules (
		id VARCHAR(36) PRIMARY KEY,
		session_id VARCHAR(255) NOT NULL,
		topic_title TEXT,
		summary TEXT,
		key_fragments JSONB,
		entities JSONB,
		relations JSONB,
		metadata JSONB,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		last_accessed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		access_count INTEGER DEFAULT 0,
		message_count INTEGER DEFAULT 0,
		token_count INTEGER DEFAULT 0
	);

	CREATE INDEX IF NOT EXISTS idx_capsules_session ON topic_capsules(session_id);
	CREATE INDEX IF NOT EXISTS idx_capsules_accessed ON topic_capsules(last_accessed_at);
	CREATE INDEX IF NOT EXISTS idx_capsules_created ON topic_capsules(created_at);
	`

	_, err := s.db.ExecContext(ctx, schema)
	return err
}

// Tier 返回存储层级
func (s *L2ShortTermMemory) Tier() StorageTier {
	return TierL2
}

// Store 存储数据
func (s *L2ShortTermMemory) Store(ctx context.Context, data interface{}) error {
	capsule, ok := data.(*TopicCapsule)
	if !ok {
		return fmt.Errorf("invalid data type, expected *TopicCapsule")
	}
	return s.StoreCapsule(ctx, capsule)
}

// Get 获取数据
func (s *L2ShortTermMemory) Get(ctx context.Context, id string) (interface{}, error) {
	return s.GetCapsule(ctx, id)
}

// Delete 删除数据
func (s *L2ShortTermMemory) Delete(ctx context.Context, id string) error {
	return s.DeleteCapsule(ctx, id)
}

// Close 关闭存储
func (s *L2ShortTermMemory) Close() error {
	return s.db.Close()
}

// StoreCapsule 存储主题胶囊
func (s *L2ShortTermMemory) StoreCapsule(ctx context.Context, capsule *TopicCapsule) error {
	if capsule.ID == "" {
		capsule.ID = uuid.New().String()
	}
	if capsule.CreatedAt.IsZero() {
		capsule.CreatedAt = time.Now()
	}
	capsule.LastAccessedAt = time.Now()

	// 序列化 JSON 字段
	keyFragmentsJSON, _ := json.Marshal(capsule.KeyFragments)
	entitiesJSON, _ := json.Marshal(capsule.Entities)
	relationsJSON, _ := json.Marshal(capsule.Relations)
	metadataJSON, _ := json.Marshal(capsule.Metadata)

	query := `
	INSERT INTO topic_capsules 
		(id, session_id, topic_title, summary, key_fragments, entities, relations, metadata, 
		 created_at, last_accessed_at, access_count, message_count, token_count)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	ON CONFLICT (id) DO UPDATE SET
		topic_title = EXCLUDED.topic_title,
		summary = EXCLUDED.summary,
		key_fragments = EXCLUDED.key_fragments,
		entities = EXCLUDED.entities,
		relations = EXCLUDED.relations,
		metadata = EXCLUDED.metadata,
		last_accessed_at = EXCLUDED.last_accessed_at,
		access_count = topic_capsules.access_count + 1,
		message_count = EXCLUDED.message_count,
		token_count = EXCLUDED.token_count
	`

	_, err := s.db.ExecContext(ctx, query,
		capsule.ID,
		capsule.SessionID,
		capsule.TopicTitle,
		capsule.Summary,
		keyFragmentsJSON,
		entitiesJSON,
		relationsJSON,
		metadataJSON,
		capsule.CreatedAt,
		capsule.LastAccessedAt,
		capsule.AccessCount,
		capsule.MessageCount,
		capsule.TokenCount,
	)

	if err != nil {
		return fmt.Errorf("insert capsule failed: %w", err)
	}

	// TODO: 存储向量到 Milvus

	return nil
}

// GetCapsule 获取主题胶囊
func (s *L2ShortTermMemory) GetCapsule(ctx context.Context, capsuleID string) (*TopicCapsule, error) {
	query := `
	SELECT id, session_id, topic_title, summary, key_fragments, entities, relations, metadata,
		   created_at, last_accessed_at, access_count, message_count, token_count
	FROM topic_capsules
	WHERE id = $1
	`

	row := s.db.QueryRowContext(ctx, query, capsuleID)

	var capsule TopicCapsule
	var keyFragmentsJSON, entitiesJSON, relationsJSON, metadataJSON []byte

	err := row.Scan(
		&capsule.ID,
		&capsule.SessionID,
		&capsule.TopicTitle,
		&capsule.Summary,
		&keyFragmentsJSON,
		&entitiesJSON,
		&relationsJSON,
		&metadataJSON,
		&capsule.CreatedAt,
		&capsule.LastAccessedAt,
		&capsule.AccessCount,
		&capsule.MessageCount,
		&capsule.TokenCount,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query capsule failed: %w", err)
	}

	// 反序列化 JSON 字段
	json.Unmarshal(keyFragmentsJSON, &capsule.KeyFragments)
	json.Unmarshal(entitiesJSON, &capsule.Entities)
	json.Unmarshal(relationsJSON, &capsule.Relations)
	json.Unmarshal(metadataJSON, &capsule.Metadata)

	return &capsule, nil
}

// SearchCapsules 搜索胶囊
func (s *L2ShortTermMemory) SearchCapsules(ctx context.Context, query *CapsuleQuery) ([]*TopicCapsule, error) {
	sqlQuery := `
	SELECT id, session_id, topic_title, summary, key_fragments, entities, relations, metadata,
		   created_at, last_accessed_at, access_count, message_count, token_count
	FROM topic_capsules
	WHERE 1=1
	`
	args := make([]interface{}, 0)
	argIdx := 1

	if query.SessionID != "" {
		sqlQuery += fmt.Sprintf(" AND session_id = $%d", argIdx)
		args = append(args, query.SessionID)
		argIdx++
	}

	if query.TopicTitle != "" {
		sqlQuery += fmt.Sprintf(" AND topic_title ILIKE $%d", argIdx)
		args = append(args, "%"+query.TopicTitle+"%")
		argIdx++
	}

	if !query.Since.IsZero() {
		sqlQuery += fmt.Sprintf(" AND created_at >= $%d", argIdx)
		args = append(args, query.Since)
		argIdx++
	}

	if !query.Before.IsZero() {
		sqlQuery += fmt.Sprintf(" AND created_at <= $%d", argIdx)
		args = append(args, query.Before)
		argIdx++
	}

	sqlQuery += " ORDER BY last_accessed_at DESC"

	if query.Limit > 0 {
		sqlQuery += fmt.Sprintf(" LIMIT %d", query.Limit)
	}

	rows, err := s.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("query capsules failed: %w", err)
	}
	defer rows.Close()

	var capsules []*TopicCapsule
	for rows.Next() {
		var capsule TopicCapsule
		var keyFragmentsJSON, entitiesJSON, relationsJSON, metadataJSON []byte

		err := rows.Scan(
			&capsule.ID,
			&capsule.SessionID,
			&capsule.TopicTitle,
			&capsule.Summary,
			&keyFragmentsJSON,
			&entitiesJSON,
			&relationsJSON,
			&metadataJSON,
			&capsule.CreatedAt,
			&capsule.LastAccessedAt,
			&capsule.AccessCount,
			&capsule.MessageCount,
			&capsule.TokenCount,
		)
		if err != nil {
			continue
		}

		json.Unmarshal(keyFragmentsJSON, &capsule.KeyFragments)
		json.Unmarshal(entitiesJSON, &capsule.Entities)
		json.Unmarshal(relationsJSON, &capsule.Relations)
		json.Unmarshal(metadataJSON, &capsule.Metadata)

		capsules = append(capsules, &capsule)
	}

	return capsules, nil
}

// SearchByVector 向量搜索
func (s *L2ShortTermMemory) SearchByVector(ctx context.Context, vector []float64, topK int) ([]*TopicCapsule, error) {
	// TODO: 实现 Milvus 向量搜索
	// 暂时返回最近访问的胶囊
	return s.SearchCapsules(ctx, &CapsuleQuery{Limit: topK})
}

// UpdateAccessTime 更新访问时间
func (s *L2ShortTermMemory) UpdateAccessTime(ctx context.Context, capsuleID string) error {
	query := `
	UPDATE topic_capsules 
	SET last_accessed_at = $1, access_count = access_count + 1
	WHERE id = $2
	`
	_, err := s.db.ExecContext(ctx, query, time.Now(), capsuleID)
	return err
}

// GetExpiredCapsules 获取过期胶囊
func (s *L2ShortTermMemory) GetExpiredCapsules(ctx context.Context, threshold time.Duration) ([]*TopicCapsule, error) {
	cutoff := time.Now().Add(-threshold)
	return s.SearchCapsules(ctx, &CapsuleQuery{
		Before: cutoff,
	})
}

// DeleteCapsule 删除胶囊
func (s *L2ShortTermMemory) DeleteCapsule(ctx context.Context, capsuleID string) error {
	query := `DELETE FROM topic_capsules WHERE id = $1`
	_, err := s.db.ExecContext(ctx, query, capsuleID)
	return err
}

// GetStats 获取统计信息
func (s *L2ShortTermMemory) GetStats(ctx context.Context) (*L2Stats, error) {
	var stats L2Stats

	query := `
	SELECT 
		COUNT(*) as capsule_count,
		COALESCE(SUM(message_count), 0) as total_messages,
		COALESCE(SUM(token_count), 0) as total_tokens
	FROM topic_capsules
	`

	row := s.db.QueryRowContext(ctx, query)
	err := row.Scan(&stats.CapsuleCount, &stats.TotalMessages, &stats.TotalTokens)
	if err != nil {
		return nil, err
	}

	return &stats, nil
}

// L2Stats L2 统计信息
type L2Stats struct {
	CapsuleCount  int `json:"capsule_count"`
	TotalMessages int `json:"total_messages"`
	TotalTokens   int `json:"total_tokens"`
}
