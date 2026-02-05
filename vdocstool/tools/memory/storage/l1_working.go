// Package storage L1 工作记忆存储 (Redis)
package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"

	"github.com/cloudwego/eino/vdocstool/config"
)

// L1WorkingMemory L1 工作记忆存储
type L1WorkingMemory struct {
	client     *redis.Client
	keyPrefix  string
	maxMessages int
	maxTokens   int
}

// NewL1WorkingMemory 创建 L1 存储
func NewL1WorkingMemory(cfg config.L1Config) (*L1WorkingMemory, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis connect failed: %w", err)
	}

	return &L1WorkingMemory{
		client:      client,
		keyPrefix:   cfg.KeyPrefix,
		maxMessages: cfg.MaxMessages,
		maxTokens:   cfg.MaxTokens,
	}, nil
}

// Tier 返回存储层级
func (s *L1WorkingMemory) Tier() StorageTier {
	return TierL1
}

// Store 存储数据
func (s *L1WorkingMemory) Store(ctx context.Context, data interface{}) error {
	msg, ok := data.(*Message)
	if !ok {
		return fmt.Errorf("invalid data type, expected *Message")
	}
	return s.AppendMessage(ctx, msg.SessionID, msg)
}

// Get 获取数据
func (s *L1WorkingMemory) Get(ctx context.Context, id string) (interface{}, error) {
	// L1 不支持按ID获取单条消息
	return nil, fmt.Errorf("not supported")
}

// Delete 删除数据
func (s *L1WorkingMemory) Delete(ctx context.Context, id string) error {
	// L1 不支持删除单条消息
	return fmt.Errorf("not supported")
}

// Close 关闭存储
func (s *L1WorkingMemory) Close() error {
	return s.client.Close()
}

// AppendMessage 追加消息
func (s *L1WorkingMemory) AppendMessage(ctx context.Context, sessionID string, msg *Message) error {
	key := s.messageListKey(sessionID)

	// 序列化消息
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal message failed: %w", err)
	}

	// 追加到列表
	if err := s.client.RPush(ctx, key, data).Err(); err != nil {
		return fmt.Errorf("rpush failed: %w", err)
	}

	// 更新 Token 计数
	tokenKey := s.tokenCountKey(sessionID)
	s.client.IncrBy(ctx, tokenKey, int64(msg.TokenCount))

	// 更新会话活跃列表
	s.client.SAdd(ctx, s.activeSessionsKey(), sessionID)

	// 设置过期时间（24小时无活动后过期）
	s.client.Expire(ctx, key, 24*time.Hour)
	s.client.Expire(ctx, tokenKey, 24*time.Hour)

	return nil
}

// GetRecentMessages 获取最近的消息
func (s *L1WorkingMemory) GetRecentMessages(ctx context.Context, sessionID string, limit int) ([]*Message, error) {
	key := s.messageListKey(sessionID)

	// 获取最近的消息
	data, err := s.client.LRange(ctx, key, int64(-limit), -1).Result()
	if err != nil {
		return nil, fmt.Errorf("lrange failed: %w", err)
	}

	messages := make([]*Message, 0, len(data))
	for _, item := range data {
		var msg Message
		if err := json.Unmarshal([]byte(item), &msg); err != nil {
			continue
		}
		messages = append(messages, &msg)
	}

	return messages, nil
}

// GetAllMessages 获取所有消息
func (s *L1WorkingMemory) GetAllMessages(ctx context.Context, sessionID string) ([]*Message, error) {
	key := s.messageListKey(sessionID)

	data, err := s.client.LRange(ctx, key, 0, -1).Result()
	if err != nil {
		return nil, fmt.Errorf("lrange failed: %w", err)
	}

	messages := make([]*Message, 0, len(data))
	for _, item := range data {
		var msg Message
		if err := json.Unmarshal([]byte(item), &msg); err != nil {
			continue
		}
		messages = append(messages, &msg)
	}

	return messages, nil
}

// GetMessageCount 获取消息数量
func (s *L1WorkingMemory) GetMessageCount(ctx context.Context, sessionID string) (int, error) {
	key := s.messageListKey(sessionID)
	count, err := s.client.LLen(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("llen failed: %w", err)
	}
	return int(count), nil
}

// GetTokenCount 获取 Token 总数
func (s *L1WorkingMemory) GetTokenCount(ctx context.Context, sessionID string) (int, error) {
	key := s.tokenCountKey(sessionID)
	count, err := s.client.Get(ctx, key).Int()
	if err == redis.Nil {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("get token count failed: %w", err)
	}
	return count, nil
}

// ClearSession 清除会话
func (s *L1WorkingMemory) ClearSession(ctx context.Context, sessionID string) error {
	keys := []string{
		s.messageListKey(sessionID),
		s.tokenCountKey(sessionID),
	}

	if err := s.client.Del(ctx, keys...).Err(); err != nil {
		return fmt.Errorf("del failed: %w", err)
	}

	// 从活跃列表移除
	s.client.SRem(ctx, s.activeSessionsKey(), sessionID)

	return nil
}

// GetActiveSessions 获取活跃会话列表
func (s *L1WorkingMemory) GetActiveSessions(ctx context.Context) ([]string, error) {
	sessions, err := s.client.SMembers(ctx, s.activeSessionsKey()).Result()
	if err != nil {
		return nil, fmt.Errorf("smembers failed: %w", err)
	}
	return sessions, nil
}

// ShouldArchive 检查是否需要归档
func (s *L1WorkingMemory) ShouldArchive(ctx context.Context, sessionID string) (bool, string, error) {
	// 检查消息数量
	count, err := s.GetMessageCount(ctx, sessionID)
	if err != nil {
		return false, "", err
	}
	if count >= s.maxMessages {
		return true, fmt.Sprintf("message count %d >= %d", count, s.maxMessages), nil
	}

	// 检查 Token 数量
	tokens, err := s.GetTokenCount(ctx, sessionID)
	if err != nil {
		return false, "", err
	}
	if tokens >= s.maxTokens {
		return true, fmt.Sprintf("token count %d >= %d", tokens, s.maxTokens), nil
	}

	return false, "", nil
}

// TrimToLimit 裁剪到限制
func (s *L1WorkingMemory) TrimToLimit(ctx context.Context, sessionID string, keepCount int) ([]*Message, error) {
	key := s.messageListKey(sessionID)

	// 获取所有消息
	allMessages, err := s.GetAllMessages(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	if len(allMessages) <= keepCount {
		return nil, nil // 无需裁剪
	}

	// 计算要移除的消息
	toRemove := allMessages[:len(allMessages)-keepCount]

	// 裁剪列表
	if err := s.client.LTrim(ctx, key, int64(-keepCount), -1).Err(); err != nil {
		return nil, fmt.Errorf("ltrim failed: %w", err)
	}

	// 重新计算 Token 数
	remainingMessages, _ := s.GetAllMessages(ctx, sessionID)
	totalTokens := 0
	for _, msg := range remainingMessages {
		totalTokens += msg.TokenCount
	}
	s.client.Set(ctx, s.tokenCountKey(sessionID), totalTokens, 24*time.Hour)

	return toRemove, nil
}

// key helpers
func (s *L1WorkingMemory) messageListKey(sessionID string) string {
	return fmt.Sprintf("%smessages:%s", s.keyPrefix, sessionID)
}

func (s *L1WorkingMemory) tokenCountKey(sessionID string) string {
	return fmt.Sprintf("%stokens:%s", s.keyPrefix, sessionID)
}

func (s *L1WorkingMemory) activeSessionsKey() string {
	return fmt.Sprintf("%sactive_sessions", s.keyPrefix)
}

// GetStats 获取统计信息
func (s *L1WorkingMemory) GetStats(ctx context.Context) (*L1Stats, error) {
	sessions, err := s.GetActiveSessions(ctx)
	if err != nil {
		return nil, err
	}

	stats := &L1Stats{
		SessionCount: len(sessions),
	}

	for _, sessionID := range sessions {
		count, _ := s.GetMessageCount(ctx, sessionID)
		tokens, _ := s.GetTokenCount(ctx, sessionID)
		stats.TotalMessages += count
		stats.TotalTokens += tokens
	}

	return stats, nil
}

// L1Stats L1 统计信息
type L1Stats struct {
	SessionCount  int `json:"session_count"`
	TotalMessages int `json:"total_messages"`
	TotalTokens   int `json:"total_tokens"`
}
