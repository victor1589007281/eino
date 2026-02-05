// Package capsule 摘要生成器
package capsule

import (
	"context"
	"strings"

	"github.com/cloudwego/eino/vdocstool/tools/memory/storage"
)

// Summarizer 摘要生成器
type Summarizer struct {
	maxSummaryLength int
	vectorDim        int
}

// NewSummarizer 创建摘要生成器
func NewSummarizer() *Summarizer {
	return &Summarizer{
		maxSummaryLength: 500,
		vectorDim:        1536,
	}
}

// Summarize 生成摘要和向量
func (s *Summarizer) Summarize(ctx context.Context, messages []*storage.Message) (string, []float64) {
	if len(messages) == 0 {
		return "", nil
	}

	// 构建摘要内容
	var userQueries []string
	var assistantResponses []string

	for _, msg := range messages {
		switch msg.Role {
		case "user":
			userQueries = append(userQueries, msg.Content)
		case "assistant":
			assistantResponses = append(assistantResponses, msg.Content)
		}
	}

	// 生成摘要
	summary := s.generateSummary(userQueries, assistantResponses)

	// TODO: 调用 Embedding API 生成向量
	// vector := s.generateVector(ctx, summary)

	return summary, nil
}

// SummarizeText 摘要文本
func (s *Summarizer) SummarizeText(ctx context.Context, text string) (string, []float64) {
	summary := s.truncateToSentence(text, s.maxSummaryLength)
	return summary, nil
}

// generateSummary 生成摘要
func (s *Summarizer) generateSummary(userQueries, assistantResponses []string) string {
	var parts []string

	// 主题：基于用户第一个问题
	if len(userQueries) > 0 {
		topic := s.extractTopic(userQueries[0])
		if topic != "" {
			parts = append(parts, "主题: "+topic)
		}
	}

	// 关键问题
	if len(userQueries) > 0 {
		questions := s.extractKeyQuestions(userQueries)
		if len(questions) > 0 {
			parts = append(parts, "问题: "+strings.Join(questions, "; "))
		}
	}

	// 关键结论
	if len(assistantResponses) > 0 {
		conclusion := s.extractConclusion(assistantResponses)
		if conclusion != "" {
			parts = append(parts, "结论: "+conclusion)
		}
	}

	summary := strings.Join(parts, "\n")
	return s.truncateToSentence(summary, s.maxSummaryLength)
}

// extractTopic 提取主题
func (s *Summarizer) extractTopic(firstQuery string) string {
	// 取第一句作为主题
	topic := s.getFirstSentence(firstQuery)
	if len(topic) > 100 {
		topic = topic[:100] + "..."
	}
	return topic
}

// extractKeyQuestions 提取关键问题
func (s *Summarizer) extractKeyQuestions(queries []string) []string {
	var questions []string
	seen := make(map[string]bool)

	for _, q := range queries {
		// 简化问题
		simplified := s.simplifyQuestion(q)
		if simplified != "" && !seen[simplified] {
			seen[simplified] = true
			questions = append(questions, simplified)
		}

		// 最多保留3个关键问题
		if len(questions) >= 3 {
			break
		}
	}

	return questions
}

// simplifyQuestion 简化问题
func (s *Summarizer) simplifyQuestion(question string) string {
	// 取第一句
	first := s.getFirstSentence(question)
	if len(first) > 80 {
		first = first[:80] + "..."
	}
	return first
}

// extractConclusion 提取结论
func (s *Summarizer) extractConclusion(responses []string) string {
	if len(responses) == 0 {
		return ""
	}

	// 取最后一个回复的第一段作为结论
	lastResponse := responses[len(responses)-1]
	conclusion := s.getFirstParagraph(lastResponse)

	if len(conclusion) > 200 {
		conclusion = conclusion[:200] + "..."
	}

	return conclusion
}

// getFirstSentence 获取第一句
func (s *Summarizer) getFirstSentence(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}

	// 查找句子结束标记
	endMarks := []string{"。", "？", "！", ".", "?", "!"}
	minPos := len(text)

	for _, mark := range endMarks {
		pos := strings.Index(text, mark)
		if pos > 0 && pos < minPos {
			minPos = pos
		}
	}

	if minPos < len(text) {
		return text[:minPos+1]
	}

	// 没找到句子结束，查找换行
	newline := strings.Index(text, "\n")
	if newline > 0 {
		return text[:newline]
	}

	return text
}

// getFirstParagraph 获取第一段
func (s *Summarizer) getFirstParagraph(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}

	// 查找段落结束（双换行）
	doubleNewline := strings.Index(text, "\n\n")
	if doubleNewline > 0 {
		return text[:doubleNewline]
	}

	// 查找单换行
	newline := strings.Index(text, "\n")
	if newline > 0 && newline < 500 {
		return text[:newline]
	}

	// 取前500字符
	if len(text) > 500 {
		return text[:500] + "..."
	}

	return text
}

// truncateToSentence 截断到句子边界
func (s *Summarizer) truncateToSentence(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}

	truncated := text[:maxLen]

	// 尝试在句子边界截断
	endMarks := []string{"。", "？", "！", ".", "?", "!"}
	lastPos := -1

	for _, mark := range endMarks {
		pos := strings.LastIndex(truncated, mark)
		if pos > lastPos && pos > maxLen/2 {
			lastPos = pos
		}
	}

	if lastPos > 0 {
		return truncated[:lastPos+1]
	}

	// 尝试在词边界截断
	lastSpace := strings.LastIndex(truncated, " ")
	if lastSpace > maxLen/2 {
		return truncated[:lastSpace] + "..."
	}

	return truncated + "..."
}

// ExtractKeywords 提取关键词
func (s *Summarizer) ExtractKeywords(text string) []string {
	// 简单实现：提取出现频率高的词
	words := strings.Fields(text)
	wordCount := make(map[string]int)

	for _, word := range words {
		word = strings.ToLower(word)
		// 过滤太短的词
		if len(word) < 2 {
			continue
		}
		// 过滤停用词
		if isStopWord(word) {
			continue
		}
		wordCount[word]++
	}

	// 按频率排序
	type wordFreq struct {
		word  string
		count int
	}
	var freqs []wordFreq
	for w, c := range wordCount {
		freqs = append(freqs, wordFreq{w, c})
	}

	// 简单排序
	for i := 0; i < len(freqs)-1; i++ {
		for j := i + 1; j < len(freqs); j++ {
			if freqs[j].count > freqs[i].count {
				freqs[i], freqs[j] = freqs[j], freqs[i]
			}
		}
	}

	// 取前10个
	var keywords []string
	for i := 0; i < len(freqs) && i < 10; i++ {
		keywords = append(keywords, freqs[i].word)
	}

	return keywords
}

// isStopWord 检查是否为停用词
func isStopWord(word string) bool {
	stopWords := map[string]bool{
		"的": true, "了": true, "是": true, "在": true, "我": true,
		"有": true, "和": true, "就": true, "不": true, "人": true,
		"都": true, "一": true, "个": true, "上": true, "也": true,
		"the": true, "a": true, "an": true, "is": true, "are": true,
		"to": true, "of": true, "and": true, "in": true, "that": true,
		"for": true, "it": true, "with": true, "as": true, "on": true,
		"be": true, "this": true, "have": true, "from": true, "or": true,
	}
	return stopWords[word]
}
