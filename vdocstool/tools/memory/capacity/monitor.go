// Package capacity 容量监控
package capacity

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/cloudwego/eino/vdocstool/tools/memory/storage"
)

// Monitor 容量监控器
type Monitor struct {
	l1          *storage.L1WorkingMemory
	l2          *storage.L2ShortTermMemory
	l3          *storage.L3LongTermMemory
	tierManager *storage.TierManager

	// 配置
	l1MessageThreshold int
	l1TokenThreshold   int
	l2RetentionDays    int
	checkInterval      time.Duration

	// 控制
	stopCh chan struct{}
	wg     sync.WaitGroup

	// 回调
	onArchiveNeeded func(sessionID string, reason string)
}

// NewMonitor 创建监控器
func NewMonitor(l1 *storage.L1WorkingMemory, l2 *storage.L2ShortTermMemory, l3 *storage.L3LongTermMemory, tierManager *storage.TierManager) *Monitor {
	return &Monitor{
		l1:                 l1,
		l2:                 l2,
		l3:                 l3,
		tierManager:        tierManager,
		l1MessageThreshold: 20,
		l1TokenThreshold:   4000,
		l2RetentionDays:    30,
		checkInterval:      5 * time.Minute,
		stopCh:             make(chan struct{}),
	}
}

// SetThresholds 设置阈值
func (m *Monitor) SetThresholds(messages, tokens int) {
	m.l1MessageThreshold = messages
	m.l1TokenThreshold = tokens
}

// SetArchiveCallback 设置归档回调
func (m *Monitor) SetArchiveCallback(callback func(sessionID string, reason string)) {
	m.onArchiveNeeded = callback
}

// Start 启动监控
func (m *Monitor) Start() {
	m.wg.Add(1)
	go m.monitorLoop()
}

// Stop 停止监控
func (m *Monitor) Stop() {
	close(m.stopCh)
	m.wg.Wait()
}

// monitorLoop 监控循环
func (m *Monitor) monitorLoop() {
	defer m.wg.Done()

	ticker := time.NewTicker(m.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.checkAll()
		case <-m.stopCh:
			return
		}
	}
}

// checkAll 检查所有会话
func (m *Monitor) checkAll() {
	ctx := context.Background()

	// 检查 L1 容量
	sessions, err := m.l1.GetActiveSessions(ctx)
	if err != nil {
		log.Printf("获取活跃会话失败: %v", err)
		return
	}

	for _, sessionID := range sessions {
		m.checkSession(ctx, sessionID)
	}

	// 检查 L2 过期胶囊
	m.checkL2Expiration(ctx)
}

// checkSession 检查单个会话
func (m *Monitor) checkSession(ctx context.Context, sessionID string) {
	// 检查消息数量
	messageCount, err := m.l1.GetMessageCount(ctx, sessionID)
	if err != nil {
		return
	}

	// 检查 Token 数量
	tokenCount, err := m.l1.GetTokenCount(ctx, sessionID)
	if err != nil {
		return
	}

	// 判断是否需要归档
	needArchive := false
	reason := ""

	if messageCount >= m.l1MessageThreshold {
		needArchive = true
		reason = "message_count_exceeded"
	} else if tokenCount >= m.l1TokenThreshold {
		needArchive = true
		reason = "token_count_exceeded"
	}

	if needArchive {
		if m.onArchiveNeeded != nil {
			m.onArchiveNeeded(sessionID, reason)
		} else {
			// 默认处理：自动归档
			m.autoArchive(ctx, sessionID, reason)
		}
	}
}

// autoArchive 自动归档
func (m *Monitor) autoArchive(ctx context.Context, sessionID, reason string) {
	log.Printf("自动归档会话 %s，原因: %s", sessionID, reason)

	// 生成主题标题
	topicTitle := "自动归档: " + time.Now().Format("2006-01-02 15:04:05")

	// 执行迁移
	capsule, err := m.tierManager.MigrateL1ToL2(ctx, sessionID, topicTitle)
	if err != nil {
		log.Printf("归档失败: %v", err)
		return
	}

	log.Printf("归档成功，胶囊ID: %s", capsule.ID)
}

// checkL2Expiration 检查 L2 过期
func (m *Monitor) checkL2Expiration(ctx context.Context) {
	threshold := time.Duration(m.l2RetentionDays) * 24 * time.Hour

	capsules, err := m.l2.GetExpiredCapsules(ctx, threshold)
	if err != nil {
		log.Printf("获取过期胶囊失败: %v", err)
		return
	}

	for _, capsule := range capsules {
		// 归档到 L3
		archiveID, err := m.l3.ArchiveCapsule(ctx, capsule)
		if err != nil {
			log.Printf("L2->L3 归档失败 %s: %v", capsule.ID, err)
			continue
		}

		// 从 L2 删除
		if err := m.l2.DeleteCapsule(ctx, capsule.ID); err != nil {
			log.Printf("删除 L2 胶囊失败 %s: %v", capsule.ID, err)
		}

		log.Printf("L2 胶囊 %s 已归档到 L3: %s", capsule.ID, archiveID)
	}
}

// GetCapacityStats 获取容量统计
func (m *Monitor) GetCapacityStats(ctx context.Context) (*CapacityStats, error) {
	stats := &CapacityStats{}

	// L1 统计
	if l1Stats, err := m.l1.GetStats(ctx); err == nil {
		stats.L1 = L1CapacityStats{
			SessionCount:     l1Stats.SessionCount,
			TotalMessages:    l1Stats.TotalMessages,
			TotalTokens:      l1Stats.TotalTokens,
			MessageThreshold: m.l1MessageThreshold,
			TokenThreshold:   m.l1TokenThreshold,
		}
	}

	// L2 统计
	if l2Stats, err := m.l2.GetStats(ctx); err == nil {
		stats.L2 = L2CapacityStats{
			CapsuleCount:  l2Stats.CapsuleCount,
			TotalMessages: l2Stats.TotalMessages,
			TotalTokens:   l2Stats.TotalTokens,
			RetentionDays: m.l2RetentionDays,
		}
	}

	// L3 统计
	if l3Stats, err := m.l3.GetStats(ctx); err == nil {
		stats.L3 = L3CapacityStats{
			ArchiveCount:  l3Stats.ArchiveCount,
			TotalSize:     l3Stats.TotalSize,
			EntityCount:   l3Stats.EntityCount,
			RelationCount: l3Stats.RelationCount,
		}
	}

	return stats, nil
}

// CapacityStats 容量统计
type CapacityStats struct {
	L1 L1CapacityStats `json:"l1"`
	L2 L2CapacityStats `json:"l2"`
	L3 L3CapacityStats `json:"l3"`
}

// L1CapacityStats L1 容量统计
type L1CapacityStats struct {
	SessionCount     int `json:"session_count"`
	TotalMessages    int `json:"total_messages"`
	TotalTokens      int `json:"total_tokens"`
	MessageThreshold int `json:"message_threshold"`
	TokenThreshold   int `json:"token_threshold"`
}

// L2CapacityStats L2 容量统计
type L2CapacityStats struct {
	CapsuleCount  int `json:"capsule_count"`
	TotalMessages int `json:"total_messages"`
	TotalTokens   int `json:"total_tokens"`
	RetentionDays int `json:"retention_days"`
}

// L3CapacityStats L3 容量统计
type L3CapacityStats struct {
	ArchiveCount  int   `json:"archive_count"`
	TotalSize     int64 `json:"total_size"`
	EntityCount   int   `json:"entity_count"`
	RelationCount int   `json:"relation_count"`
}
