// Package rest Memory REST API 类型定义
package rest

import "time"

// StoreRequest 存储请求
type StoreRequest struct {
	SessionID string            `json:"session_id"`
	Source    string            `json:"source,omitempty"`
	Message   MessageInput      `json:"message"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	Options   *StoreOptions     `json:"options,omitempty"`
}

// MessageInput 消息输入
type MessageInput struct {
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	TopicID   string    `json:"topic_id,omitempty"`
	Timestamp time.Time `json:"timestamp,omitempty"`
}

// StoreOptions 存储选项
type StoreOptions struct {
	ExtractEntities   bool    `json:"extract_entities,omitempty"`
	GenerateEmbedding bool    `json:"generate_embedding,omitempty"`
	Importance        float64 `json:"importance,omitempty"`
}

// StoreResponse 存储响应
type StoreResponse struct {
	MessageID         string   `json:"message_id"`
	Tier              string   `json:"tier"`
	TokenCount        int      `json:"token_count"`
	EntitiesExtracted []string `json:"entities_extracted,omitempty"`
	ArchiveTriggered  bool     `json:"archive_triggered"`
}

// RetrieveRequest 检索请求
type RetrieveRequest struct {
	SessionID string           `json:"session_id"`
	Query     string           `json:"query"`
	Options   *RetrieveOptions `json:"options,omitempty"`
}

// RetrieveOptions 检索选项
type RetrieveOptions struct {
	TokenBudget    int               `json:"token_budget,omitempty"`
	TopicID        string            `json:"topic_id,omitempty"`
	TimeRange      *TimeRange        `json:"time_range,omitempty"`
	Filters        map[string]string `json:"filters,omitempty"`
	IncludeSummary bool              `json:"include_summary,omitempty"`
	IncludeEntities bool             `json:"include_entities,omitempty"`
}

// TimeRange 时间范围
type TimeRange struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// RetrieveResponse 检索响应
type RetrieveResponse struct {
	Context     []*ContextItem `json:"context"`
	Summary     string         `json:"summary,omitempty"`
	Entities    []*EntityInfo  `json:"entities,omitempty"`
	TotalTokens int            `json:"total_tokens"`
	SearchStats *SearchStats   `json:"search_stats,omitempty"`
}

// ContextItem 上下文项
type ContextItem struct {
	MessageID      string    `json:"message_id"`
	Role           string    `json:"role"`
	Content        string    `json:"content"`
	Timestamp      time.Time `json:"timestamp"`
	RelevanceScore float64   `json:"relevance_score"`
	Tier           string    `json:"tier"`
}

// EntityInfo 实体信息
type EntityInfo struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Mentions int    `json:"mentions"`
}

// SearchStats 搜索统计
type SearchStats struct {
	L1Hits          int   `json:"l1_hits"`
	L2Hits          int   `json:"l2_hits"`
	L3Hits          int   `json:"l3_hits"`
	SearchLatencyMs int64 `json:"search_latency_ms"`
}

// BatchStoreRequest 批量存储请求
type BatchStoreRequest struct {
	SessionID string          `json:"session_id"`
	Messages  []MessageInput  `json:"messages"`
	Options   *BatchStoreOptions `json:"options,omitempty"`
}

// BatchStoreOptions 批量存储选项
type BatchStoreOptions struct {
	PreserveOrder      bool `json:"preserve_order,omitempty"`
	ParallelProcessing bool `json:"parallel_processing,omitempty"`
}

// BatchStoreResponse 批量存储响应
type BatchStoreResponse struct {
	Results      []*StoreResponse `json:"results"`
	SuccessCount int              `json:"success_count"`
	FailedCount  int              `json:"failed_count"`
	TotalTokens  int              `json:"total_tokens"`
}

// BatchRetrieveRequest 批量检索请求
type BatchRetrieveRequest struct {
	Requests []SingleRetrieveRequest `json:"requests"`
	Options  *BatchRetrieveOptions   `json:"options,omitempty"`
}

// SingleRetrieveRequest 单个检索请求
type SingleRetrieveRequest struct {
	SessionID string `json:"session_id"`
	Query     string `json:"query"`
}

// BatchRetrieveOptions 批量检索选项
type BatchRetrieveOptions struct {
	Parallel  bool `json:"parallel,omitempty"`
	TimeoutMs int  `json:"timeout_ms,omitempty"`
}

// BatchRetrieveResponse 批量检索响应
type BatchRetrieveResponse struct {
	Results []*RetrieveResponse `json:"results"`
}

// SessionInfo 会话信息
type SessionInfo struct {
	SessionID       string    `json:"session_id"`
	MessageCount    int       `json:"message_count"`
	TokenCount      int       `json:"token_count"`
	TopicCount      int       `json:"topic_count"`
	CurrentTopicID  string    `json:"current_topic_id,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	LastActiveAt    time.Time `json:"last_active_at"`
}

// SwitchTopicRequest 切换主题请求
type SwitchTopicRequest struct {
	SessionID string `json:"session_id"`
	NewTopic  string `json:"new_topic,omitempty"`
}

// SwitchTopicResponse 切换主题响应
type SwitchTopicResponse struct {
	CapsuleID    string `json:"capsule_id"`
	TopicTitle   string `json:"topic_title"`
	MessageCount int    `json:"message_count"`
	TokenCount   int    `json:"token_count"`
}

// RecallTopicRequest 召回主题请求
type RecallTopicRequest struct {
	SessionID  string `json:"session_id"`
	TopicQuery string `json:"topic_query"`
}

// RecallTopicResponse 召回主题响应
type RecallTopicResponse struct {
	CapsuleID    string `json:"capsule_id"`
	TopicTitle   string `json:"topic_title"`
	LoadedFromL3 bool   `json:"loaded_from_l3"`
	Summary      string `json:"summary"`
}

// TopicInfo 主题信息
type TopicInfo struct {
	TopicID      string    `json:"topic_id"`
	Title        string    `json:"title"`
	MessageCount int       `json:"message_count"`
	TokenCount   int       `json:"token_count"`
	CreatedAt    time.Time `json:"created_at"`
	LastActiveAt time.Time `json:"last_active_at"`
	Tier         string    `json:"tier"` // L2 or L3
}

// Relation 实体关系
type Relation struct {
	Source   string `json:"source"`
	Target   string `json:"target"`
	Type     string `json:"type"`
	Weight   float64 `json:"weight"`
}

// EntityRelationsResponse 实体关系响应
type EntityRelationsResponse struct {
	EntityName string      `json:"entity_name"`
	Relations  []*Relation `json:"relations"`
	Depth      int         `json:"depth"`
}

// SummaryResponse 摘要响应
type SummaryResponse struct {
	Summary      string   `json:"summary"`
	MessageCount int      `json:"message_count"`
	TokenCount   int      `json:"token_count"`
	KeyEntities  []string `json:"key_entities,omitempty"`
}

// ArchiveRequest 归档请求
type ArchiveRequest struct {
	SessionID  string `json:"session_id"`
	TopicTitle string `json:"topic_title,omitempty"`
}

// ArchiveResponse 归档响应
type ArchiveResponse struct {
	CapsuleID string `json:"capsule_id,omitempty"`
	ArchiveID string `json:"archive_id,omitempty"`
	Archived  bool   `json:"archived"`
}

// Stats 统计信息
type Stats struct {
	L1Stats *TierStats `json:"l1"`
	L2Stats *TierStats `json:"l2"`
	L3Stats *TierStats `json:"l3"`
	Total   *TotalStats `json:"total"`
}

// TierStats 层级统计
type TierStats struct {
	SessionCount int   `json:"session_count"`
	MessageCount int   `json:"message_count"`
	TokenCount   int64 `json:"token_count"`
	SizeBytes    int64 `json:"size_bytes"`
}

// TotalStats 总计统计
type TotalStats struct {
	TotalSessions int   `json:"total_sessions"`
	TotalMessages int   `json:"total_messages"`
	TotalTokens   int64 `json:"total_tokens"`
	TotalSizeBytes int64 `json:"total_size_bytes"`
}
