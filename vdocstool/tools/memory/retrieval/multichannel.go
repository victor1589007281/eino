// Package retrieval 多路召回
package retrieval

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino/vdocstool/tools/memory/storage"
)

// MultiChannelRecall 多路召回器
type MultiChannelRecall struct {
	l2      *storage.L2ShortTermMemory
	l3      *storage.L3LongTermMemory
	weights ChannelWeights
}

// NewMultiChannelRecall 创建多路召回器
func NewMultiChannelRecall(l2 *storage.L2ShortTermMemory, l3 *storage.L3LongTermMemory, weights ChannelWeights) *MultiChannelRecall {
	return &MultiChannelRecall{
		l2:      l2,
		l3:      l3,
		weights: weights,
	}
}

// ChannelResult 通道结果
type ChannelResult struct {
	Channel string
	Items   []*ContextItem
	Weight  float64
}

// Recall 执行多路召回
func (m *MultiChannelRecall) Recall(ctx context.Context, sessionID, query string, intent *IntentInfo) ([]*ContextItem, error) {
	var wg sync.WaitGroup
	results := make(chan *ChannelResult, 4)
	errors := make(chan error, 4)

	// 通道A：向量语义检索
	wg.Add(1)
	go func() {
		defer wg.Done()
		items, err := m.semanticSearch(ctx, sessionID, query)
		if err != nil {
			errors <- err
			return
		}
		results <- &ChannelResult{
			Channel: "semantic",
			Items:   items,
			Weight:  m.weights.SemanticSearch,
		}
	}()

	// 通道B：实体关联图谱
	if len(intent.TopicKeywords) > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			items, err := m.entityGraphSearch(ctx, sessionID, intent.TopicKeywords)
			if err != nil {
				errors <- err
				return
			}
			results <- &ChannelResult{
				Channel: "entity_graph",
				Items:   items,
				Weight:  m.weights.EntityGraph,
			}
		}()
	}

	// 通道C：时序邻近主题
	wg.Add(1)
	go func() {
		defer wg.Done()
		items, err := m.temporalNearSearch(ctx, sessionID)
		if err != nil {
			errors <- err
			return
		}
		results <- &ChannelResult{
			Channel: "temporal",
			Items:   items,
			Weight:  m.weights.TemporalNear,
		}
	}()

	// 通道D：用户显式锁定（如果有）
	if intent.ExplicitTopicSwitch && len(intent.ReferenceHints) > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			items, err := m.pinnedTopicSearch(ctx, sessionID, intent.ReferenceHints)
			if err != nil {
				errors <- err
				return
			}
			results <- &ChannelResult{
				Channel: "pinned",
				Items:   items,
				Weight:  m.weights.UserPinned,
			}
		}()
	}

	// 等待所有通道完成
	go func() {
		wg.Wait()
		close(results)
		close(errors)
	}()

	// 收集结果
	var allResults []*ContextItem
	for result := range results {
		// 应用权重
		for _, item := range result.Items {
			item.Relevance *= result.Weight
		}
		allResults = append(allResults, result.Items...)
	}

	// 检查错误（可选择性忽略部分通道错误）
	for err := range errors {
		if err != nil {
			// 记录但不中断
			_ = err
		}
	}

	return allResults, nil
}

// semanticSearch 向量语义检索
func (m *MultiChannelRecall) semanticSearch(ctx context.Context, sessionID, query string) ([]*ContextItem, error) {
	// TODO: 将 query 转换为向量
	// vector := embedQuery(query)

	// 从 L2 搜索
	capsules, err := m.l2.SearchCapsules(ctx, &storage.CapsuleQuery{
		SessionID: sessionID,
		Limit:     10,
	})
	if err != nil {
		return nil, err
	}

	var items []*ContextItem
	for _, capsule := range capsules {
		// 计算相似度（简化：基于关键词）
		relevance := calculateTextSimilarity(query, capsule.Summary)

		items = append(items, &ContextItem{
			Content:    capsule.Summary,
			Role:       "context",
			Source:     "L2:" + capsule.ID,
			Relevance:  relevance,
			TokenCount: estimateTokenCount(capsule.Summary),
		})

		// 添加关键片段
		for _, fragment := range capsule.KeyFragments {
			fragRelevance := calculateTextSimilarity(query, fragment.Content)
			items = append(items, &ContextItem{
				Content:    fragment.Content,
				Role:       fragment.Role,
				Source:     "L2:" + capsule.ID,
				Relevance:  fragRelevance * fragment.Importance,
				TokenCount: estimateTokenCount(fragment.Content),
			})
		}
	}

	return items, nil
}

// entityGraphSearch 实体图谱搜索
func (m *MultiChannelRecall) entityGraphSearch(ctx context.Context, sessionID string, keywords []string) ([]*ContextItem, error) {
	var items []*ContextItem

	// 从 L3 查询相关实体
	for _, keyword := range keywords {
		entities, err := m.l3.QueryEntities(ctx, keyword, 5)
		if err != nil {
			continue
		}

		for _, entity := range entities {
			// 查询实体关系
			relations, _ := m.l3.QueryRelations(ctx, entity.Name, 2)

			// 构建上下文描述
			content := "相关实体: " + entity.Name + " (" + entity.Type + ")"
			if len(relations) > 0 {
				content += "\n关联: "
				for i, rel := range relations {
					if i > 0 {
						content += ", "
					}
					content += rel.FromEntity + " -> " + rel.ToEntity
				}
			}

			items = append(items, &ContextItem{
				Content:    content,
				Role:       "context",
				Source:     "L3:entity:" + entity.ID,
				Relevance:  0.6,
				TokenCount: estimateTokenCount(content),
			})
		}
	}

	return items, nil
}

// temporalNearSearch 时序邻近搜索
func (m *MultiChannelRecall) temporalNearSearch(ctx context.Context, sessionID string) ([]*ContextItem, error) {
	// 获取最近访问的胶囊
	capsules, err := m.l2.SearchCapsules(ctx, &storage.CapsuleQuery{
		SessionID: sessionID,
		Since:     time.Now().Add(-7 * 24 * time.Hour), // 最近7天
		Limit:     5,
	})
	if err != nil {
		return nil, err
	}

	var items []*ContextItem
	for i, capsule := range capsules {
		// 时间越近权重越高
		recency := 1.0 - float64(i)*0.1

		items = append(items, &ContextItem{
			Content:    "最近话题: " + capsule.TopicTitle + "\n" + capsule.Summary,
			Role:       "context",
			Source:     "L2:" + capsule.ID,
			Relevance:  recency * 0.5,
			TokenCount: estimateTokenCount(capsule.Summary),
		})
	}

	return items, nil
}

// pinnedTopicSearch 用户锁定主题搜索
func (m *MultiChannelRecall) pinnedTopicSearch(ctx context.Context, sessionID string, hints []string) ([]*ContextItem, error) {
	var items []*ContextItem

	for _, hint := range hints {
		// 搜索匹配的胶囊
		capsules, err := m.l2.SearchCapsules(ctx, &storage.CapsuleQuery{
			SessionID:  sessionID,
			TopicTitle: hint,
			Limit:      3,
		})
		if err != nil {
			continue
		}

		for _, capsule := range capsules {
			items = append(items, &ContextItem{
				Content:    capsule.Summary,
				Role:       "context",
				Source:     "L2:" + capsule.ID,
				Relevance:  0.9, // 显式引用权重高
				TokenCount: estimateTokenCount(capsule.Summary),
			})

			// 添加关键片段
			for _, fragment := range capsule.KeyFragments {
				items = append(items, &ContextItem{
					Content:    fragment.Content,
					Role:       fragment.Role,
					Source:     "L2:" + capsule.ID,
					Relevance:  0.8 * fragment.Importance,
					TokenCount: estimateTokenCount(fragment.Content),
				})
			}
		}
	}

	return items, nil
}

// calculateTextSimilarity 计算文本相似度
func calculateTextSimilarity(query, text string) float64 {
	// 方法1：基于 token 匹配
	queryWords := tokenize(query)
	textWords := tokenize(text)

	if len(queryWords) == 0 || len(textWords) == 0 {
		// 回退到子串匹配
		return substringSimiliarity(query, text)
	}

	querySet := make(map[string]bool)
	for _, w := range queryWords {
		querySet[w] = true
	}

	// 精确匹配
	exactMatchCount := 0
	for _, w := range textWords {
		if querySet[w] {
			exactMatchCount++
		}
	}

	// 子串匹配（用于中文）
	substringMatchCount := 0
	for qw := range querySet {
		for _, tw := range textWords {
			if len(qw) >= 2 && len(tw) >= 2 {
				if strings.Contains(tw, qw) || strings.Contains(qw, tw) {
					substringMatchCount++
					break
				}
			}
		}
	}

	// 组合得分
	exactScore := float64(exactMatchCount) / float64(len(queryWords))
	substringScore := float64(substringMatchCount) / float64(len(queryWords)) * 0.5

	// 如果 token 级别匹配效果不好，尝试原始字符串级别的匹配
	if exactScore+substringScore < 0.3 {
		// 尝试直接的子串匹配
		rawSim := rawTextSimilarity(query, text)
		if rawSim > exactScore+substringScore {
			return rawSim
		}
	}

	return exactScore + substringScore
}

// rawTextSimilarity 基于原始文本的相似度计算（适用于中英混合文本）
func rawTextSimilarity(query, text string) float64 {
	// 如果 query 是 text 的子串
	if strings.Contains(text, query) {
		return 1.0
	}
	
	// 分解 query 为更小的片段进行匹配
	qRunes := []rune(query)
	matchedRunes := 0
	
	// 滑动窗口匹配
	for windowSize := len(qRunes); windowSize >= 2; windowSize-- {
		for i := 0; i <= len(qRunes)-windowSize; i++ {
			window := string(qRunes[i : i+windowSize])
			if strings.Contains(text, window) {
				// 计算这个窗口覆盖的字符数
				matchedRunes += windowSize
				break
			}
		}
		if matchedRunes > 0 {
			break
		}
	}
	
	if matchedRunes == 0 {
		// 尝试单个字符匹配（中文字符）
		for _, r := range qRunes {
			if r > 127 { // 中文字符
				if strings.ContainsRune(text, r) {
					matchedRunes++
				}
			}
		}
	}
	
	if len(qRunes) == 0 {
		return 0
	}
	
	return float64(matchedRunes) / float64(len(qRunes))
}

// substringSimiliarity 子串相似度
func substringSimiliarity(query, text string) float64 {
	if len(query) == 0 || len(text) == 0 {
		return 0
	}
	
	// 简单的子串包含检查
	if strings.Contains(text, query) {
		return 1.0
	}
	if strings.Contains(query, text) {
		return float64(len(text)) / float64(len(query))
	}
	
	// 计算公共前缀长度
	commonLen := 0
	minLen := len(query)
	if len(text) < minLen {
		minLen = len(text)
	}
	
	qRunes := []rune(query)
	tRunes := []rune(text)
	for i := 0; i < minLen && i < len(qRunes) && i < len(tRunes); i++ {
		if qRunes[i] == tRunes[i] {
			commonLen++
		} else {
			break
		}
	}
	
	return float64(commonLen) / float64(len(qRunes))
}

// estimateTokenCount 估算Token数量
func estimateTokenCount(text string) int {
	// 简单估算：中文每个字约1.5个token，英文每个词约1个token
	chineseCount := 0
	englishWords := 0
	inWord := false

	for _, r := range text {
		if r > 127 {
			chineseCount++
			inWord = false
		} else if isAlphaNumeric(r) {
			if !inWord {
				englishWords++
				inWord = true
			}
		} else {
			inWord = false
		}
	}

	return int(float64(chineseCount)*1.5) + englishWords
}
