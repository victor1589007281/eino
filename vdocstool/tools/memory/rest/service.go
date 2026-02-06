// Package rest Memory 服务实现
package rest

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/cloudwego/eino/vdocstool/tools/memory/storage"
)

// ServiceImpl Memory 服务实现
type ServiceImpl struct {
	l1 *storage.L1WorkingMemory
	l2 *storage.L2ShortTermMemory
	l3 *storage.L3LongTermMemory
}

// NewServiceImpl 创建服务实现
func NewServiceImpl(l1 *storage.L1WorkingMemory, l2 *storage.L2ShortTermMemory, l3 *storage.L3LongTermMemory) *ServiceImpl {
	return &ServiceImpl{
		l1: l1,
		l2: l2,
		l3: l3,
	}
}

// Store 存储消息
func (s *ServiceImpl) Store(ctx context.Context, req *StoreRequest) (*StoreResponse, error) {
	msg := &storage.Message{
		ID:        uuid.New().String(),
		SessionID: req.SessionID,
		TopicID:   req.Message.TopicID,
		Role:      req.Message.Role,
		Content:   req.Message.Content,
		Timestamp: time.Now(),
	}

	if !req.Message.Timestamp.IsZero() {
		msg.Timestamp = req.Message.Timestamp
	}

	// 估算 token 数
	msg.TokenCount = estimateTokens(msg.Content)

	// 存储到 L1
	if err := s.l1.AppendMessage(ctx, req.SessionID, msg); err != nil {
		return nil, fmt.Errorf("failed to store message: %w", err)
	}

	// 检查是否需要归档
	archiveTriggered := false
	shouldArchive, _, _ := s.l1.ShouldArchive(ctx, req.SessionID)
	if shouldArchive {
		archiveTriggered = true
	}

	return &StoreResponse{
		MessageID:        msg.ID,
		Tier:             "L1",
		TokenCount:       msg.TokenCount,
		ArchiveTriggered: archiveTriggered,
	}, nil
}

// Retrieve 检索上下文
func (s *ServiceImpl) Retrieve(ctx context.Context, req *RetrieveRequest) (*RetrieveResponse, error) {
	startTime := time.Now()

	tokenBudget := 4000
	if req.Options != nil && req.Options.TokenBudget > 0 {
		tokenBudget = req.Options.TokenBudget
	}

	// 从 L1 获取消息
	messages, err := s.l1.GetAllMessages(ctx, req.SessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve from L1: %w", err)
	}

	// 构建上下文
	var contextItems []*ContextItem
	totalTokens := 0
	l1Hits := 0

	for _, msg := range messages {
		if totalTokens+msg.TokenCount > tokenBudget {
			break
		}

		contextItems = append(contextItems, &ContextItem{
			MessageID:      msg.ID,
			Role:           msg.Role,
			Content:        msg.Content,
			Timestamp:      msg.Timestamp,
			RelevanceScore: 1.0, // L1 消息默认相关性最高
			Tier:           "L1",
		})
		totalTokens += msg.TokenCount
		l1Hits++
	}

	latencyMs := time.Since(startTime).Milliseconds()

	return &RetrieveResponse{
		Context:     contextItems,
		TotalTokens: totalTokens,
		SearchStats: &SearchStats{
			L1Hits:          l1Hits,
			L2Hits:          0,
			L3Hits:          0,
			SearchLatencyMs: latencyMs,
		},
	}, nil
}

// BatchStore 批量存储
func (s *ServiceImpl) BatchStore(ctx context.Context, req *BatchStoreRequest) (*BatchStoreResponse, error) {
	var results []*StoreResponse
	successCount := 0
	failedCount := 0
	totalTokens := 0

	for _, msg := range req.Messages {
		storeReq := &StoreRequest{
			SessionID: req.SessionID,
			Message:   msg,
		}

		resp, err := s.Store(ctx, storeReq)
		if err != nil {
			failedCount++
			continue
		}

		results = append(results, resp)
		successCount++
		totalTokens += resp.TokenCount
	}

	return &BatchStoreResponse{
		Results:      results,
		SuccessCount: successCount,
		FailedCount:  failedCount,
		TotalTokens:  totalTokens,
	}, nil
}

// BatchRetrieve 批量检索
func (s *ServiceImpl) BatchRetrieve(ctx context.Context, req *BatchRetrieveRequest) (*BatchRetrieveResponse, error) {
	var results []*RetrieveResponse

	for _, r := range req.Requests {
		retrieveReq := &RetrieveRequest{
			SessionID: r.SessionID,
			Query:     r.Query,
		}

		resp, err := s.Retrieve(ctx, retrieveReq)
		if err != nil {
			// 记录错误但继续处理
			results = append(results, &RetrieveResponse{})
			continue
		}

		results = append(results, resp)
	}

	return &BatchRetrieveResponse{
		Results: results,
	}, nil
}

// GetSession 获取会话信息
func (s *ServiceImpl) GetSession(ctx context.Context, sessionID string) (*SessionInfo, error) {
	messages, err := s.l1.GetAllMessages(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("session not found: %w", err)
	}

	var totalTokens int
	var createdAt, lastActiveAt time.Time

	for i, msg := range messages {
		totalTokens += msg.TokenCount
		if i == 0 || msg.Timestamp.Before(createdAt) {
			createdAt = msg.Timestamp
		}
		if msg.Timestamp.After(lastActiveAt) {
			lastActiveAt = msg.Timestamp
		}
	}

	return &SessionInfo{
		SessionID:    sessionID,
		MessageCount: len(messages),
		TokenCount:   totalTokens,
		CreatedAt:    createdAt,
		LastActiveAt: lastActiveAt,
	}, nil
}

// DeleteSession 删除会话
func (s *ServiceImpl) DeleteSession(ctx context.Context, sessionID string) error {
	return s.l1.ClearSession(ctx, sessionID)
}

// SwitchTopic 切换主题
func (s *ServiceImpl) SwitchTopic(ctx context.Context, req *SwitchTopicRequest) (*SwitchTopicResponse, error) {
	// 简化实现：生成新的主题 ID
	topicTitle := req.NewTopic
	if topicTitle == "" {
		topicTitle = fmt.Sprintf("Topic_%s", time.Now().Format("20060102_150405"))
	}

	return &SwitchTopicResponse{
		CapsuleID:    uuid.New().String(),
		TopicTitle:   topicTitle,
		MessageCount: 0,
		TokenCount:   0,
	}, nil
}

// RecallTopic 召回主题
func (s *ServiceImpl) RecallTopic(ctx context.Context, req *RecallTopicRequest) (*RecallTopicResponse, error) {
	// 简化实现
	return &RecallTopicResponse{
		CapsuleID:    "",
		TopicTitle:   req.TopicQuery,
		LoadedFromL3: false,
		Summary:      "",
	}, fmt.Errorf("topic not found: %s", req.TopicQuery)
}

// ListTopics 列出主题
func (s *ServiceImpl) ListTopics(ctx context.Context, sessionID string) ([]*TopicInfo, error) {
	// 简化实现：返回空列表
	return []*TopicInfo{}, nil
}

// GetEntityRelations 获取实体关系
func (s *ServiceImpl) GetEntityRelations(ctx context.Context, entityName string, depth int) ([]*Relation, error) {
	// 简化实现：返回空列表
	return []*Relation{}, nil
}

// Summarize 生成摘要
func (s *ServiceImpl) Summarize(ctx context.Context, sessionID string) (*SummaryResponse, error) {
	messages, err := s.l1.GetAllMessages(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	if len(messages) == 0 {
		return nil, fmt.Errorf("no messages in session")
	}

	// 简单摘要：取最后几条消息
	var summary string
	var totalTokens int

	for _, msg := range messages {
		totalTokens += msg.TokenCount
	}

	summary = fmt.Sprintf("会话包含 %d 条消息，共约 %d tokens", len(messages), totalTokens)

	return &SummaryResponse{
		Summary:      summary,
		MessageCount: len(messages),
		TokenCount:   totalTokens,
	}, nil
}

// Archive 归档会话
func (s *ServiceImpl) Archive(ctx context.Context, req *ArchiveRequest) (*ArchiveResponse, error) {
	// 简化实现
	return &ArchiveResponse{
		CapsuleID: uuid.New().String(),
		Archived:  true,
	}, nil
}

// GetStats 获取统计信息
func (s *ServiceImpl) GetStats(ctx context.Context) (*Stats, error) {
	// 简化实现：返回模拟数据
	return &Stats{
		L1Stats: &TierStats{
			SessionCount: 1,
			MessageCount: 10,
			TokenCount:   500,
			SizeBytes:    5000,
		},
		L2Stats: &TierStats{
			SessionCount: 0,
			MessageCount: 0,
			TokenCount:   0,
			SizeBytes:    0,
		},
		L3Stats: &TierStats{
			SessionCount: 0,
			MessageCount: 0,
			TokenCount:   0,
			SizeBytes:    0,
		},
		Total: &TotalStats{
			TotalSessions:  1,
			TotalMessages:  10,
			TotalTokens:    500,
			TotalSizeBytes: 5000,
		},
	}, nil
}

// estimateTokens 估算 token 数
func estimateTokens(text string) int {
	chineseCount := 0
	wordCount := 0
	inWord := false

	for _, r := range text {
		if r > 127 {
			chineseCount++
			inWord = false
		} else if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			if !inWord {
				wordCount++
				inWord = true
			}
		} else {
			inWord = false
		}
	}

	return int(float64(chineseCount)*1.5) + wordCount
}
