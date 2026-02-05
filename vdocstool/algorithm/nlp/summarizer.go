// Package nlp 摘要生成器实现
package nlp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/vdocstool/algorithm/intent/llm"
	"github.com/cloudwego/eino/vdocstool/algorithm/types"
)

// NewSummarizer 创建摘要生成器
func NewSummarizer(cfg *types.SummarizerConfig) (Summarizer, error) {
	switch cfg.Provider {
	case "llm":
		return NewLLMSummarizer(cfg)
	case "extractive":
		return NewExtractiveSummarizer(cfg), nil
	default:
		return NewExtractiveSummarizer(cfg), nil
	}
}

// LLMSummarizer LLM摘要生成器
type LLMSummarizer struct {
	config *types.SummarizerConfig
	client llm.Client
}

// NewLLMSummarizer 创建LLM摘要生成器
func NewLLMSummarizer(cfg *types.SummarizerConfig) (*LLMSummarizer, error) {
	llmCfg := &types.LLMEngineConfig{
		Provider: "openai",
		Model:    "gpt-4o-mini",
	}
	
	client, err := llm.NewOpenAIClient(llmCfg)
	if err != nil {
		return nil, err
	}

	return &LLMSummarizer{
		config: cfg,
		client: client,
	}, nil
}

// Summarize 生成摘要
func (s *LLMSummarizer) Summarize(ctx context.Context, text string, maxLength int) (string, error) {
	if maxLength == 0 {
		maxLength = s.config.MaxLength
	}
	if maxLength == 0 {
		maxLength = 200
	}

	prompt := fmt.Sprintf(`请为以下文本生成一个简洁的摘要，不超过%d个字。

文本：
%s

摘要：`, maxLength, text)

	resp, err := s.client.Complete(ctx, &llm.CompletionRequest{
		Messages: []llm.ChatMessage{
			{Role: "user", Content: prompt},
		},
		MaxTokens:   maxLength * 2, // 预留空间
		Temperature: 0.3,
	})
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(resp.Text), nil
}

// Close 关闭
func (s *LLMSummarizer) Close() error {
	if s.client != nil {
		return s.client.Close()
	}
	return nil
}

// ExtractiveSummarizer 抽取式摘要生成器
type ExtractiveSummarizer struct {
	config *types.SummarizerConfig
}

// NewExtractiveSummarizer 创建抽取式摘要生成器
func NewExtractiveSummarizer(cfg *types.SummarizerConfig) *ExtractiveSummarizer {
	return &ExtractiveSummarizer{
		config: cfg,
	}
}

// Summarize 生成摘要（简单抽取）
func (s *ExtractiveSummarizer) Summarize(ctx context.Context, text string, maxLength int) (string, error) {
	if maxLength == 0 {
		maxLength = s.config.MaxLength
	}
	if maxLength == 0 {
		maxLength = 200
	}

	// 简单实现：按句子分割，取前N个句子
	sentences := splitSentences(text)
	
	var summary strings.Builder
	for _, sentence := range sentences {
		if summary.Len()+len(sentence) > maxLength {
			break
		}
		if summary.Len() > 0 {
			summary.WriteString(" ")
		}
		summary.WriteString(sentence)
	}

	return summary.String(), nil
}

// Close 关闭
func (s *ExtractiveSummarizer) Close() error {
	return nil
}

// splitSentences 分句
func splitSentences(text string) []string {
	// 简单按标点分句
	delimiters := []string{"。", "！", "？", ".", "!", "?", "\n"}
	
	sentences := []string{text}
	for _, delim := range delimiters {
		var newSentences []string
		for _, s := range sentences {
			parts := strings.Split(s, delim)
			for _, part := range parts {
				part = strings.TrimSpace(part)
				if part != "" {
					newSentences = append(newSentences, part)
				}
			}
		}
		sentences = newSentences
	}
	
	return sentences
}

// NewRelationExtractor 创建关系提取器
func NewRelationExtractor(cfg *types.RelationConfig) (RelationExtractor, error) {
	return NewLLMRelationExtractor(cfg)
}

// LLMRelationExtractor LLM关系提取器
type LLMRelationExtractor struct {
	config *types.RelationConfig
	client llm.Client
}

// NewLLMRelationExtractor 创建LLM关系提取器
func NewLLMRelationExtractor(cfg *types.RelationConfig) (*LLMRelationExtractor, error) {
	llmCfg := &types.LLMEngineConfig{
		Provider: "openai",
		Model:    "gpt-4o-mini",
	}
	
	client, err := llm.NewOpenAIClient(llmCfg)
	if err != nil {
		return nil, err
	}

	return &LLMRelationExtractor{
		config: cfg,
		client: client,
	}, nil
}

// Extract 提取关系
func (r *LLMRelationExtractor) Extract(ctx context.Context, text string, entities []*Entity) ([]*Relation, error) {
	if len(entities) < 2 {
		return nil, nil
	}

	// 构建实体列表
	var entityList strings.Builder
	for _, e := range entities {
		entityList.WriteString(fmt.Sprintf("- %s (%s)\n", e.Text, e.Type))
	}

	prompt := fmt.Sprintf(`请从以下文本中提取实体之间的关系。

文本：%s

已识别的实体：
%s

请以JSON数组格式返回关系，每个关系包含：
- subject: 主体实体文本
- predicate: 关系类型（如：工作于、位于、属于、使用、开发等）
- object: 客体实体文本

只返回JSON数组，不要其他文字。`, text, entityList.String())

	resp, err := r.client.Complete(ctx, &llm.CompletionRequest{
		Messages: []llm.ChatMessage{
			{Role: "user", Content: prompt},
		},
		MaxTokens:   500,
		Temperature: 0.1,
	})
	if err != nil {
		return nil, err
	}

	// 解析响应
	var rawRelations []struct {
		Subject   string `json:"subject"`
		Predicate string `json:"predicate"`
		Object    string `json:"object"`
	}

	jsonText := extractJSONArray(resp.Text)
	if jsonText != "" {
		json.Unmarshal([]byte(jsonText), &rawRelations)
	}

	// 转换为 Relation
	var relations []*Relation
	for _, raw := range rawRelations {
		relations = append(relations, &Relation{
			Subject:    &Entity{Text: raw.Subject},
			Predicate:  raw.Predicate,
			Object:     &Entity{Text: raw.Object},
			Confidence: 0.8,
		})
	}

	return relations, nil
}

// Close 关闭
func (r *LLMRelationExtractor) Close() error {
	if r.client != nil {
		return r.client.Close()
	}
	return nil
}
