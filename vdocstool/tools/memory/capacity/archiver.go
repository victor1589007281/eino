// Package capacity 归档策略
package capacity

import (
	"context"
	"log"
	"time"

	"github.com/cloudwego/eino/vdocstool/tools/memory/capsule"
	"github.com/cloudwego/eino/vdocstool/tools/memory/storage"
)

// Archiver 归档器
type Archiver struct {
	l1             *storage.L1WorkingMemory
	l2             *storage.L2ShortTermMemory
	l3             *storage.L3LongTermMemory
	capsuleManager *capsule.CapsuleManager
}

// NewArchiver 创建归档器
func NewArchiver(l1 *storage.L1WorkingMemory, l2 *storage.L2ShortTermMemory, l3 *storage.L3LongTermMemory) *Archiver {
	return &Archiver{
		l1:             l1,
		l2:             l2,
		l3:             l3,
		capsuleManager: capsule.NewCapsuleManager(),
	}
}

// ArchiveStrategy 归档策略
type ArchiveStrategy string

const (
	StrategyAuto       ArchiveStrategy = "auto"       // 自动选择
	StrategyFull       ArchiveStrategy = "full"       // 完整归档
	StrategyIncremental ArchiveStrategy = "incremental" // 增量归档
	StrategySummary    ArchiveStrategy = "summary"    // 仅摘要
)

// ArchiveRequest 归档请求
type ArchiveRequest struct {
	SessionID   string          `json:"session_id"`
	TopicTitle  string          `json:"topic_title,omitempty"`
	Strategy    ArchiveStrategy `json:"strategy,omitempty"`
	KeepRecent  int             `json:"keep_recent,omitempty"` // 保留最近N条在L1
}

// ArchiveResult 归档结果
type ArchiveResult struct {
	CapsuleID     string    `json:"capsule_id,omitempty"`
	ArchiveID     string    `json:"archive_id,omitempty"`
	MessageCount  int       `json:"message_count"`
	TokenCount    int       `json:"token_count"`
	Strategy      string    `json:"strategy"`
	ArchivedAt    time.Time `json:"archived_at"`
	KeptInL1      int       `json:"kept_in_l1,omitempty"`
}

// ArchiveL1ToL2 L1 -> L2 归档
func (a *Archiver) ArchiveL1ToL2(ctx context.Context, req *ArchiveRequest) (*ArchiveResult, error) {
	result := &ArchiveResult{
		Strategy:   string(req.Strategy),
		ArchivedAt: time.Now(),
	}

	// 获取所有 L1 消息
	messages, err := a.l1.GetAllMessages(ctx, req.SessionID)
	if err != nil {
		return nil, err
	}

	if len(messages) == 0 {
		return result, nil
	}

	result.MessageCount = len(messages)

	// 选择策略
	strategy := req.Strategy
	if strategy == "" || strategy == StrategyAuto {
		strategy = a.selectStrategy(messages)
	}

	// 计算要归档的消息
	var toArchive []*storage.Message
	keepRecent := req.KeepRecent
	if keepRecent > 0 && keepRecent < len(messages) {
		toArchive = messages[:len(messages)-keepRecent]
		result.KeptInL1 = keepRecent
	} else {
		toArchive = messages
	}

	// 创建胶囊
	topicTitle := req.TopicTitle
	if topicTitle == "" {
		topicTitle = a.generateTopicTitle(toArchive)
	}

	capsuleData, err := a.capsuleManager.CreateCapsule(ctx, req.SessionID, topicTitle, toArchive)
	if err != nil {
		return nil, err
	}

	result.CapsuleID = capsuleData.ID
	result.TokenCount = capsuleData.TokenCount

	// 根据策略处理
	switch strategy {
	case StrategySummary:
		// 仅保存摘要，丢弃原始消息
		capsuleData.KeyFragments = nil
	case StrategyIncremental:
		// 保留部分关键片段
		if len(capsuleData.KeyFragments) > 10 {
			capsuleData.KeyFragments = capsuleData.KeyFragments[:10]
		}
	}

	// 存储到 L2
	if err := a.l2.StoreCapsule(ctx, capsuleData); err != nil {
		return nil, err
	}

	// 清理 L1
	if req.KeepRecent > 0 {
		// 裁剪 L1，只保留最近的消息
		_, err = a.l1.TrimToLimit(ctx, req.SessionID, req.KeepRecent)
	} else {
		// 清除所有
		err = a.l1.ClearSession(ctx, req.SessionID)
	}

	if err != nil {
		log.Printf("清理 L1 失败: %v", err)
	}

	return result, nil
}

// ArchiveL2ToL3 L2 -> L3 归档
func (a *Archiver) ArchiveL2ToL3(ctx context.Context, capsuleID string) (*ArchiveResult, error) {
	result := &ArchiveResult{
		Strategy:   "l2_to_l3",
		ArchivedAt: time.Now(),
	}

	// 获取胶囊
	capsuleData, err := a.l2.GetCapsule(ctx, capsuleID)
	if err != nil {
		return nil, err
	}
	if capsuleData == nil {
		return result, nil
	}

	result.CapsuleID = capsuleID
	result.MessageCount = capsuleData.MessageCount
	result.TokenCount = capsuleData.TokenCount

	// 归档到 L3
	archiveID, err := a.l3.ArchiveCapsule(ctx, capsuleData)
	if err != nil {
		return nil, err
	}

	result.ArchiveID = archiveID

	// 从 L2 删除
	if err := a.l2.DeleteCapsule(ctx, capsuleID); err != nil {
		log.Printf("删除 L2 胶囊失败: %v", err)
	}

	return result, nil
}

// selectStrategy 选择归档策略
func (a *Archiver) selectStrategy(messages []*storage.Message) ArchiveStrategy {
	// 计算总 Token 数
	totalTokens := 0
	for _, msg := range messages {
		totalTokens += msg.TokenCount
	}

	// 根据大小选择策略
	if totalTokens > 10000 {
		return StrategySummary // 太大，只保存摘要
	} else if totalTokens > 5000 {
		return StrategyIncremental // 中等大小，增量归档
	}

	return StrategyFull // 完整归档
}

// generateTopicTitle 生成主题标题
func (a *Archiver) generateTopicTitle(messages []*storage.Message) string {
	if len(messages) == 0 {
		return "Unknown Topic"
	}

	// 从第一条用户消息提取主题
	for _, msg := range messages {
		if msg.Role == "user" {
			content := msg.Content
			// 取前50字符作为标题
			if len(content) > 50 {
				content = content[:50] + "..."
			}
			return content
		}
	}

	return "Session " + messages[0].SessionID[:8]
}

// BatchArchive 批量归档
func (a *Archiver) BatchArchive(ctx context.Context, sessionIDs []string) ([]*ArchiveResult, error) {
	var results []*ArchiveResult

	for _, sessionID := range sessionIDs {
		result, err := a.ArchiveL1ToL2(ctx, &ArchiveRequest{
			SessionID: sessionID,
			Strategy:  StrategyAuto,
		})
		if err != nil {
			log.Printf("归档会话 %s 失败: %v", sessionID, err)
			continue
		}
		results = append(results, result)
	}

	return results, nil
}

// CleanupOldArchives 清理旧归档
func (a *Archiver) CleanupOldArchives(ctx context.Context, olderThan time.Duration) (int, error) {
	// 获取过期胶囊
	capsules, err := a.l2.GetExpiredCapsules(ctx, olderThan)
	if err != nil {
		return 0, err
	}

	cleaned := 0
	for _, capsuleData := range capsules {
		// 归档到 L3
		_, err := a.l3.ArchiveCapsule(ctx, capsuleData)
		if err != nil {
			continue
		}

		// 从 L2 删除
		if err := a.l2.DeleteCapsule(ctx, capsuleData.ID); err == nil {
			cleaned++
		}
	}

	return cleaned, nil
}
