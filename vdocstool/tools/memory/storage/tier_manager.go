// Package storage 层级迁移管理器
package storage

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// TierManager 层级迁移管理器
type TierManager struct {
	l1 *L1WorkingMemory
	l2 *L2ShortTermMemory
	l3 *L3LongTermMemory

	// 配置
	l2RetentionDays int
	archiveInterval time.Duration

	// 控制
	stopCh chan struct{}
	wg     sync.WaitGroup
}

// NewTierManager 创建层级迁移管理器
func NewTierManager(l1 *L1WorkingMemory, l2 *L2ShortTermMemory, l3 *L3LongTermMemory) *TierManager {
	return &TierManager{
		l1:              l1,
		l2:              l2,
		l3:              l3,
		l2RetentionDays: 30,
		archiveInterval: 1 * time.Hour,
		stopCh:          make(chan struct{}),
	}
}

// Start 启动后台任务
func (m *TierManager) Start() {
	m.wg.Add(1)
	go m.archiveLoop()
}

// Stop 停止后台任务
func (m *TierManager) Stop() {
	close(m.stopCh)
	m.wg.Wait()
}

// archiveLoop 归档循环
func (m *TierManager) archiveLoop() {
	defer m.wg.Done()

	ticker := time.NewTicker(m.archiveInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ctx := context.Background()
			m.processArchiving(ctx)
		case <-m.stopCh:
			return
		}
	}
}

// processArchiving 处理归档
func (m *TierManager) processArchiving(ctx context.Context) {
	// L2 -> L3: 归档过期胶囊
	threshold := time.Duration(m.l2RetentionDays) * 24 * time.Hour
	expiredCapsules, err := m.l2.GetExpiredCapsules(ctx, threshold)
	if err != nil {
		log.Printf("获取过期胶囊失败: %v", err)
		return
	}

	for _, capsule := range expiredCapsules {
		// 归档到 L3
		archiveID, err := m.l3.ArchiveCapsule(ctx, capsule)
		if err != nil {
			log.Printf("归档胶囊失败 %s: %v", capsule.ID, err)
			continue
		}

		// 从 L2 删除
		if err := m.l2.DeleteCapsule(ctx, capsule.ID); err != nil {
			log.Printf("删除 L2 胶囊失败 %s: %v", capsule.ID, err)
		}

		log.Printf("已归档胶囊 %s -> %s", capsule.ID, archiveID)
	}
}

// MigrateL1ToL2 L1 -> L2 迁移
func (m *TierManager) MigrateL1ToL2(ctx context.Context, sessionID string, topicTitle string) (*TopicCapsule, error) {
	// 获取 L1 中的所有消息
	messages, err := m.l1.GetAllMessages(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("get L1 messages failed: %w", err)
	}

	if len(messages) == 0 {
		return nil, fmt.Errorf("no messages to migrate")
	}

	// 创建主题胶囊
	capsule := &TopicCapsule{
		SessionID:      sessionID,
		TopicTitle:     topicTitle,
		CreatedAt:      messages[0].Timestamp,
		LastAccessedAt: time.Now(),
		MessageCount:   len(messages),
	}

	// 生成摘要（简单实现：取前后几条消息）
	capsule.Summary = m.generateSummary(messages)

	// 提取关键片段
	capsule.KeyFragments = m.extractKeyFragments(messages)

	// 计算 Token 总数
	for _, msg := range messages {
		capsule.TokenCount += msg.TokenCount
	}

	// 存储到 L2
	if err := m.l2.StoreCapsule(ctx, capsule); err != nil {
		return nil, fmt.Errorf("store capsule to L2 failed: %w", err)
	}

	// 清除 L1
	if err := m.l1.ClearSession(ctx, sessionID); err != nil {
		log.Printf("clear L1 session failed: %v", err)
	}

	return capsule, nil
}

// MigrateL2ToL3 L2 -> L3 迁移
func (m *TierManager) MigrateL2ToL3(ctx context.Context, capsuleID string) (string, error) {
	// 获取 L2 中的胶囊
	capsule, err := m.l2.GetCapsule(ctx, capsuleID)
	if err != nil {
		return "", fmt.Errorf("get L2 capsule failed: %w", err)
	}
	if capsule == nil {
		return "", fmt.Errorf("capsule not found")
	}

	// 归档到 L3
	archiveID, err := m.l3.ArchiveCapsule(ctx, capsule)
	if err != nil {
		return "", fmt.Errorf("archive to L3 failed: %w", err)
	}

	// 从 L2 删除
	if err := m.l2.DeleteCapsule(ctx, capsuleID); err != nil {
		log.Printf("delete L2 capsule failed: %v", err)
	}

	return archiveID, nil
}

// RecallL3ToL2 L3 -> L2 召回
func (m *TierManager) RecallL3ToL2(ctx context.Context, archiveID string) (*TopicCapsule, error) {
	// 从 L3 加载
	capsule, err := m.l3.LoadCapsule(ctx, archiveID)
	if err != nil {
		return nil, fmt.Errorf("load from L3 failed: %w", err)
	}

	// 更新访问时间
	capsule.LastAccessedAt = time.Now()
	capsule.AccessCount++

	// 存储到 L2
	if err := m.l2.StoreCapsule(ctx, capsule); err != nil {
		return nil, fmt.Errorf("store to L2 failed: %w", err)
	}

	return capsule, nil
}

// RecallL2ToL1 L2 -> L1 召回
func (m *TierManager) RecallL2ToL1(ctx context.Context, capsuleID string, sessionID string) error {
	// 获取胶囊
	capsule, err := m.l2.GetCapsule(ctx, capsuleID)
	if err != nil {
		return fmt.Errorf("get capsule failed: %w", err)
	}
	if capsule == nil {
		return fmt.Errorf("capsule not found")
	}

	// 将关键片段作为消息添加到 L1
	for _, fragment := range capsule.KeyFragments {
		msg := &Message{
			ID:        fragment.ID,
			SessionID: sessionID,
			TopicID:   capsuleID,
			Role:      fragment.Role,
			Content:   fragment.Content,
			Timestamp: fragment.Timestamp,
		}
		if err := m.l1.AppendMessage(ctx, sessionID, msg); err != nil {
			return fmt.Errorf("append message failed: %w", err)
		}
	}

	// 更新 L2 访问时间
	m.l2.UpdateAccessTime(ctx, capsuleID)

	return nil
}

// generateSummary 生成摘要
func (m *TierManager) generateSummary(messages []*Message) string {
	if len(messages) == 0 {
		return ""
	}

	// 简单实现：取第一条用户消息和最后一条助手回复
	var firstUser, lastAssistant string

	for _, msg := range messages {
		if msg.Role == "user" && firstUser == "" {
			firstUser = truncateText(msg.Content, 200)
		}
		if msg.Role == "assistant" {
			lastAssistant = truncateText(msg.Content, 200)
		}
	}

	summary := ""
	if firstUser != "" {
		summary += "用户: " + firstUser
	}
	if lastAssistant != "" {
		if summary != "" {
			summary += "\n"
		}
		summary += "助手: " + lastAssistant
	}

	return summary
}

// extractKeyFragments 提取关键片段
func (m *TierManager) extractKeyFragments(messages []*Message) []*KeyFragment {
	var fragments []*KeyFragment

	// 简单策略：保留所有用户消息和重要的助手回复
	for i, msg := range messages {
		importance := 0.5

		// 第一条和最后一条消息重要性更高
		if i == 0 || i == len(messages)-1 {
			importance = 1.0
		}

		// 用户消息重要性更高
		if msg.Role == "user" {
			importance = 0.8
		}

		// 长消息可能更重要
		if len(msg.Content) > 500 {
			importance += 0.2
		}

		if importance >= 0.7 || msg.Role == "user" {
			fragments = append(fragments, &KeyFragment{
				ID:         msg.ID,
				Content:    msg.Content,
				Role:       msg.Role,
				Timestamp:  msg.Timestamp,
				Importance: importance,
			})
		}
	}

	// 限制片段数量
	if len(fragments) > 10 {
		fragments = fragments[:10]
	}

	return fragments
}

// GetStats 获取统计信息
func (m *TierManager) GetStats(ctx context.Context) (*StorageStats, error) {
	stats := &StorageStats{}

	// L1 统计
	if l1Stats, err := m.l1.GetStats(ctx); err == nil {
		stats.L1MessageCount = l1Stats.TotalMessages
		stats.L1TokenCount = l1Stats.TotalTokens
		stats.L1SessionCount = l1Stats.SessionCount
	}

	// L2 统计
	if l2Stats, err := m.l2.GetStats(ctx); err == nil {
		stats.L2CapsuleCount = l2Stats.CapsuleCount
	}

	// L3 统计
	if l3Stats, err := m.l3.GetStats(ctx); err == nil {
		stats.L3ArchiveCount = l3Stats.ArchiveCount
		stats.L3TotalSize = l3Stats.TotalSize
		stats.L3EntityCount = l3Stats.EntityCount
		stats.L3RelationCount = l3Stats.RelationCount
	}

	return stats, nil
}

// truncateText 截断文本
func truncateText(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	return text[:maxLen] + "..."
}
