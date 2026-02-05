// Package capsule 主题胶囊
package capsule

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/cloudwego/eino/vdocstool/tools/memory/storage"
)

// CapsuleManager 胶囊管理器
type CapsuleManager struct {
	summarizer      *Summarizer
	entityExtractor *EntityExtractor
}

// NewCapsuleManager 创建胶囊管理器
func NewCapsuleManager() *CapsuleManager {
	return &CapsuleManager{
		summarizer:      NewSummarizer(),
		entityExtractor: NewEntityExtractor(),
	}
}

// CreateCapsule 从消息创建胶囊
func (m *CapsuleManager) CreateCapsule(ctx context.Context, sessionID string, topicTitle string, messages []*storage.Message) (*storage.TopicCapsule, error) {
	if len(messages) == 0 {
		return nil, nil
	}

	capsule := &storage.TopicCapsule{
		ID:             uuid.New().String(),
		SessionID:      sessionID,
		TopicTitle:     topicTitle,
		CreatedAt:      messages[0].Timestamp,
		LastAccessedAt: time.Now(),
		MessageCount:   len(messages),
	}

	// 生成摘要
	capsule.Summary, capsule.SummaryVector = m.summarizer.Summarize(ctx, messages)

	// 提取关键片段
	capsule.KeyFragments = m.extractKeyFragments(messages)

	// 提取实体和关系
	capsule.Entities, capsule.Relations = m.entityExtractor.Extract(ctx, messages)

	// 计算 Token 总数
	for _, msg := range messages {
		capsule.TokenCount += msg.TokenCount
	}

	return capsule, nil
}

// extractKeyFragments 提取关键片段
func (m *CapsuleManager) extractKeyFragments(messages []*storage.Message) []*storage.KeyFragment {
	var fragments []*storage.KeyFragment

	for i, msg := range messages {
		importance := m.calculateImportance(msg, i, len(messages))

		// 只保留重要片段
		if importance >= 0.5 {
			fragments = append(fragments, &storage.KeyFragment{
				ID:         msg.ID,
				Content:    msg.Content,
				Role:       msg.Role,
				Timestamp:  msg.Timestamp,
				Importance: importance,
			})
		}
	}

	// 限制数量
	if len(fragments) > 15 {
		// 按重要性排序并保留前15个
		fragments = sortAndTruncate(fragments, 15)
	}

	return fragments
}

// calculateImportance 计算重要性
func (m *CapsuleManager) calculateImportance(msg *storage.Message, index, total int) float64 {
	importance := 0.3 // 基础分

	// 位置因素
	if index == 0 {
		importance += 0.3 // 第一条消息
	}
	if index == total-1 {
		importance += 0.2 // 最后一条消息
	}

	// 角色因素
	if msg.Role == "user" {
		importance += 0.2 // 用户消息更重要
	}

	// 长度因素
	if len(msg.Content) > 200 {
		importance += 0.1
	}
	if len(msg.Content) > 500 {
		importance += 0.1
	}

	// 内容因素
	if containsCodeBlock(msg.Content) {
		importance += 0.2 // 包含代码块
	}
	if containsQuestion(msg.Content) {
		importance += 0.1 // 包含问题
	}

	if importance > 1.0 {
		importance = 1.0
	}

	return importance
}

// UpdateCapsule 更新胶囊
func (m *CapsuleManager) UpdateCapsule(ctx context.Context, capsule *storage.TopicCapsule, newMessages []*storage.Message) error {
	if len(newMessages) == 0 {
		return nil
	}

	// 更新摘要
	allContent := capsule.Summary + "\n"
	for _, msg := range newMessages {
		allContent += msg.Content + "\n"
	}

	// 重新生成摘要（简单追加）
	newSummary, _ := m.summarizer.SummarizeText(ctx, allContent)
	capsule.Summary = newSummary

	// 追加关键片段
	newFragments := m.extractKeyFragments(newMessages)
	capsule.KeyFragments = append(capsule.KeyFragments, newFragments...)

	// 限制片段数量
	if len(capsule.KeyFragments) > 20 {
		capsule.KeyFragments = sortAndTruncate(capsule.KeyFragments, 20)
	}

	// 提取新实体
	newEntities, newRelations := m.entityExtractor.Extract(ctx, newMessages)
	capsule.Entities = mergeEntities(capsule.Entities, newEntities)
	capsule.Relations = mergeRelations(capsule.Relations, newRelations)

	// 更新计数
	capsule.MessageCount += len(newMessages)
	for _, msg := range newMessages {
		capsule.TokenCount += msg.TokenCount
	}

	capsule.LastAccessedAt = time.Now()
	capsule.AccessCount++

	return nil
}

// MergeCapsules 合并胶囊
func (m *CapsuleManager) MergeCapsules(ctx context.Context, capsules []*storage.TopicCapsule) (*storage.TopicCapsule, error) {
	if len(capsules) == 0 {
		return nil, nil
	}
	if len(capsules) == 1 {
		return capsules[0], nil
	}

	merged := &storage.TopicCapsule{
		ID:             uuid.New().String(),
		SessionID:      capsules[0].SessionID,
		CreatedAt:      capsules[0].CreatedAt,
		LastAccessedAt: time.Now(),
	}

	// 合并标题
	titles := make([]string, len(capsules))
	for i, c := range capsules {
		titles[i] = c.TopicTitle
	}
	merged.TopicTitle = joinTitles(titles)

	// 合并摘要
	summaries := make([]string, len(capsules))
	for i, c := range capsules {
		summaries[i] = c.Summary
	}
	merged.Summary, _ = m.summarizer.SummarizeText(ctx, joinText(summaries))

	// 合并片段（按重要性排序）
	var allFragments []*storage.KeyFragment
	for _, c := range capsules {
		allFragments = append(allFragments, c.KeyFragments...)
	}
	merged.KeyFragments = sortAndTruncate(allFragments, 20)

	// 合并实体和关系
	for _, c := range capsules {
		merged.Entities = mergeEntities(merged.Entities, c.Entities)
		merged.Relations = mergeRelations(merged.Relations, c.Relations)
		merged.MessageCount += c.MessageCount
		merged.TokenCount += c.TokenCount
	}

	return merged, nil
}

// 辅助函数
func sortAndTruncate(fragments []*storage.KeyFragment, limit int) []*storage.KeyFragment {
	if len(fragments) <= limit {
		return fragments
	}

	// 简单排序：按重要性降序
	for i := 0; i < len(fragments)-1; i++ {
		for j := i + 1; j < len(fragments); j++ {
			if fragments[j].Importance > fragments[i].Importance {
				fragments[i], fragments[j] = fragments[j], fragments[i]
			}
		}
	}

	return fragments[:limit]
}

func containsCodeBlock(text string) bool {
	return contains(text, "```") || contains(text, "    ") // 4空格缩进
}

func containsQuestion(text string) bool {
	return contains(text, "?") || contains(text, "？") ||
		contains(text, "怎么") || contains(text, "如何") ||
		contains(text, "什么") || contains(text, "为什么")
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && findSubstring(s, substr) >= 0
}

func findSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func mergeEntities(a, b []*storage.Entity) []*storage.Entity {
	seen := make(map[string]bool)
	var result []*storage.Entity

	for _, e := range a {
		if !seen[e.Name] {
			seen[e.Name] = true
			result = append(result, e)
		}
	}
	for _, e := range b {
		if !seen[e.Name] {
			seen[e.Name] = true
			result = append(result, e)
		}
	}

	return result
}

func mergeRelations(a, b []*storage.Relation) []*storage.Relation {
	seen := make(map[string]bool)
	var result []*storage.Relation

	for _, r := range a {
		key := r.FromEntity + "->" + r.ToEntity + ":" + r.RelationType
		if !seen[key] {
			seen[key] = true
			result = append(result, r)
		}
	}
	for _, r := range b {
		key := r.FromEntity + "->" + r.ToEntity + ":" + r.RelationType
		if !seen[key] {
			seen[key] = true
			result = append(result, r)
		}
	}

	return result
}

func joinTitles(titles []string) string {
	if len(titles) == 0 {
		return ""
	}
	result := titles[0]
	for i := 1; i < len(titles); i++ {
		result += " & " + titles[i]
	}
	return result
}

func joinText(texts []string) string {
	result := ""
	for i, t := range texts {
		if i > 0 {
			result += "\n\n"
		}
		result += t
	}
	return result
}
