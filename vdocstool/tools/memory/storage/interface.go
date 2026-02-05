// Package storage 存储接口定义
package storage

import (
	"context"
	"time"
)

// Message 消息
type Message struct {
	ID        string                 `json:"id"`
	SessionID string                 `json:"session_id"`
	TopicID   string                 `json:"topic_id,omitempty"`
	Role      string                 `json:"role"` // user, assistant, system, tool
	Content   string                 `json:"content"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
	TokenCount int                   `json:"token_count,omitempty"`
}

// StorageTier 存储层级
type StorageTier int

const (
	TierL1 StorageTier = iota + 1 // L1 工作记忆
	TierL2                        // L2 短期记忆
	TierL3                        // L3 长期记忆
)

func (t StorageTier) String() string {
	switch t {
	case TierL1:
		return "L1"
	case TierL2:
		return "L2"
	case TierL3:
		return "L3"
	default:
		return "Unknown"
	}
}

// Storage 存储接口
type Storage interface {
	// Tier 返回存储层级
	Tier() StorageTier

	// Store 存储数据
	Store(ctx context.Context, data interface{}) error

	// Get 获取数据
	Get(ctx context.Context, id string) (interface{}, error)

	// Delete 删除数据
	Delete(ctx context.Context, id string) error

	// Close 关闭存储
	Close() error
}

// L1Storage L1 存储接口
type L1Storage interface {
	Storage

	// AppendMessage 追加消息
	AppendMessage(ctx context.Context, sessionID string, msg *Message) error

	// GetRecentMessages 获取最近的消息
	GetRecentMessages(ctx context.Context, sessionID string, limit int) ([]*Message, error)

	// GetAllMessages 获取所有消息
	GetAllMessages(ctx context.Context, sessionID string) ([]*Message, error)

	// GetMessageCount 获取消息数量
	GetMessageCount(ctx context.Context, sessionID string) (int, error)

	// GetTokenCount 获取Token总数
	GetTokenCount(ctx context.Context, sessionID string) (int, error)

	// ClearSession 清除会话
	ClearSession(ctx context.Context, sessionID string) error

	// GetActiveSessions 获取活跃会话列表
	GetActiveSessions(ctx context.Context) ([]string, error)
}

// L2Storage L2 存储接口
type L2Storage interface {
	Storage

	// StoreCapsule 存储主题胶囊
	StoreCapsule(ctx context.Context, capsule *TopicCapsule) error

	// GetCapsule 获取主题胶囊
	GetCapsule(ctx context.Context, capsuleID string) (*TopicCapsule, error)

	// SearchCapsules 搜索胶囊
	SearchCapsules(ctx context.Context, query *CapsuleQuery) ([]*TopicCapsule, error)

	// SearchByVector 向量搜索
	SearchByVector(ctx context.Context, vector []float64, topK int) ([]*TopicCapsule, error)

	// UpdateAccessTime 更新访问时间
	UpdateAccessTime(ctx context.Context, capsuleID string) error

	// GetExpiredCapsules 获取过期胶囊
	GetExpiredCapsules(ctx context.Context, threshold time.Duration) ([]*TopicCapsule, error)

	// DeleteCapsule 删除胶囊
	DeleteCapsule(ctx context.Context, capsuleID string) error
}

// L3Storage L3 存储接口
type L3Storage interface {
	Storage

	// ArchiveCapsule 归档胶囊
	ArchiveCapsule(ctx context.Context, capsule *TopicCapsule) (string, error)

	// LoadCapsule 加载胶囊
	LoadCapsule(ctx context.Context, archiveID string) (*TopicCapsule, error)

	// StoreEntity 存储实体
	StoreEntity(ctx context.Context, entity *Entity) error

	// StoreRelation 存储关系
	StoreRelation(ctx context.Context, relation *Relation) error

	// QueryEntities 查询实体
	QueryEntities(ctx context.Context, query string, limit int) ([]*Entity, error)

	// QueryRelations 查询关系
	QueryRelations(ctx context.Context, entityName string, depth int) ([]*Relation, error)

	// GetArchiveList 获取归档列表
	GetArchiveList(ctx context.Context, sessionID string) ([]*ArchiveInfo, error)
}

// TopicCapsule 主题胶囊
type TopicCapsule struct {
	ID             string                 `json:"id"`
	SessionID      string                 `json:"session_id"`
	TopicTitle     string                 `json:"topic_title"`
	Summary        string                 `json:"summary"`
	SummaryVector  []float64              `json:"summary_vector,omitempty"`
	KeyFragments   []*KeyFragment         `json:"key_fragments"`
	Entities       []*Entity              `json:"entities,omitempty"`
	Relations      []*Relation            `json:"relations,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt      time.Time              `json:"created_at"`
	LastAccessedAt time.Time              `json:"last_accessed_at"`
	AccessCount    int                    `json:"access_count"`
	MessageCount   int                    `json:"message_count"`
	TokenCount     int                    `json:"token_count"`
}

// KeyFragment 关键片段
type KeyFragment struct {
	ID         string    `json:"id"`
	Content    string    `json:"content"`
	Vector     []float64 `json:"vector,omitempty"`
	Role       string    `json:"role"`
	Timestamp  time.Time `json:"timestamp"`
	Importance float64   `json:"importance"`
}

// Entity 实体
type Entity struct {
	ID         string                 `json:"id"`
	Name       string                 `json:"name"`
	Type       string                 `json:"type"` // person, project, technology, etc.
	Attributes map[string]interface{} `json:"attributes,omitempty"`
	SessionID  string                 `json:"session_id,omitempty"`
	CreatedAt  time.Time              `json:"created_at"`
}

// Relation 关系
type Relation struct {
	ID           string    `json:"id"`
	FromEntity   string    `json:"from_entity"`
	ToEntity     string    `json:"to_entity"`
	RelationType string    `json:"relation_type"`
	Weight       float64   `json:"weight,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// CapsuleQuery 胶囊查询
type CapsuleQuery struct {
	SessionID  string    `json:"session_id,omitempty"`
	Keywords   []string  `json:"keywords,omitempty"`
	Since      time.Time `json:"since,omitempty"`
	Before     time.Time `json:"before,omitempty"`
	TopicTitle string    `json:"topic_title,omitempty"`
	Limit      int       `json:"limit,omitempty"`
}

// ArchiveInfo 归档信息
type ArchiveInfo struct {
	ArchiveID   string    `json:"archive_id"`
	CapsuleID   string    `json:"capsule_id"`
	SessionID   string    `json:"session_id"`
	TopicTitle  string    `json:"topic_title"`
	Size        int64     `json:"size"`
	Compressed  bool      `json:"compressed"`
	ArchivedAt  time.Time `json:"archived_at"`
}

// StorageStats 存储统计
type StorageStats struct {
	L1MessageCount   int     `json:"l1_message_count"`
	L1TokenCount     int     `json:"l1_token_count"`
	L1SessionCount   int     `json:"l1_session_count"`
	L2CapsuleCount   int     `json:"l2_capsule_count"`
	L2TotalSize      int64   `json:"l2_total_size"`
	L3ArchiveCount   int     `json:"l3_archive_count"`
	L3TotalSize      int64   `json:"l3_total_size"`
	L3EntityCount    int     `json:"l3_entity_count"`
	L3RelationCount  int     `json:"l3_relation_count"`
}
