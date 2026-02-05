// Package nlp NLP处理管道
package nlp

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/vdocstool/algorithm/types"
)

// Pipeline NLP处理管道
type Pipeline struct {
	config     *types.NLPConfig
	tokenizer  Tokenizer
	ner        NERExtractor
	relation   RelationExtractor
	summarizer Summarizer
}

// NewPipeline 创建NLP管道
func NewPipeline(cfg *types.NLPConfig) (*Pipeline, error) {
	p := &Pipeline{
		config: cfg,
	}

	var err error

	// 初始化分词器
	p.tokenizer, err = NewTokenizer(&cfg.Tokenizer)
	if err != nil {
		return nil, fmt.Errorf("init tokenizer: %w", err)
	}

	// 初始化NER
	p.ner, err = NewNERExtractor(&cfg.NER)
	if err != nil {
		// NER 失败不阻止启动
		fmt.Printf("warning: init ner failed: %v\n", err)
	}

	// 初始化关系抽取
	if cfg.Relation.Enabled {
		p.relation, err = NewRelationExtractor(&cfg.Relation)
		if err != nil {
			fmt.Printf("warning: init relation extractor failed: %v\n", err)
		}
	}

	// 初始化摘要生成器
	p.summarizer, err = NewSummarizer(&cfg.Summarizer)
	if err != nil {
		fmt.Printf("warning: init summarizer failed: %v\n", err)
	}

	return p, nil
}

// Process 处理文本
func (p *Pipeline) Process(ctx context.Context, text string, opts *ProcessOptions) (*ProcessResult, error) {
	if opts == nil {
		opts = &ProcessOptions{
			Tokenize:        true,
			ExtractEntities: true,
		}
	}

	result := &ProcessResult{}

	// 1. 分词
	if opts.Tokenize && p.tokenizer != nil {
		tokens, err := p.tokenizer.Tokenize(ctx, text)
		if err != nil {
			return nil, fmt.Errorf("tokenize: %w", err)
		}
		result.Tokens = tokens
		result.Keywords = extractKeywords(tokens, 10)
	}

	// 2. NER
	if opts.ExtractEntities && p.ner != nil {
		entities, err := p.ner.Extract(ctx, text)
		if err != nil {
			// NER 失败不影响主流程
			fmt.Printf("warning: ner failed: %v\n", err)
		} else {
			result.Entities = entities
		}
	}

	// 3. 关系抽取
	if opts.ExtractRelations && p.relation != nil && len(result.Entities) > 1 {
		relations, err := p.relation.Extract(ctx, text, result.Entities)
		if err != nil {
			fmt.Printf("warning: relation extraction failed: %v\n", err)
		} else {
			result.Relations = relations
		}
	}

	// 4. 摘要
	if opts.Summarize && p.summarizer != nil {
		maxLen := opts.SummaryMaxLength
		if maxLen == 0 {
			maxLen = 200
		}
		summary, err := p.summarizer.Summarize(ctx, text, maxLen)
		if err != nil {
			fmt.Printf("warning: summarize failed: %v\n", err)
		} else {
			result.Summary = summary
		}
	}

	return result, nil
}

// Tokenize 分词
func (p *Pipeline) Tokenize(ctx context.Context, text string) ([]*Token, error) {
	if p.tokenizer == nil {
		return nil, fmt.Errorf("tokenizer not available")
	}
	return p.tokenizer.Tokenize(ctx, text)
}

// ExtractEntities 提取实体
func (p *Pipeline) ExtractEntities(ctx context.Context, text string) ([]*Entity, error) {
	if p.ner == nil {
		return nil, fmt.Errorf("ner not available")
	}
	return p.ner.Extract(ctx, text)
}

// Summarize 生成摘要
func (p *Pipeline) Summarize(ctx context.Context, text string, maxLength int) (string, error) {
	if p.summarizer == nil {
		return "", fmt.Errorf("summarizer not available")
	}
	return p.summarizer.Summarize(ctx, text, maxLength)
}

// HealthCheck 健康检查
func (p *Pipeline) HealthCheck(ctx context.Context) error {
	// 至少分词器可用即为健康
	if p.tokenizer == nil {
		return fmt.Errorf("tokenizer not available")
	}
	return nil
}

// Close 关闭管道
func (p *Pipeline) Close() error {
	var errs []error

	if p.tokenizer != nil {
		if err := p.tokenizer.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if p.ner != nil {
		if err := p.ner.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if p.relation != nil {
		if err := p.relation.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if p.summarizer != nil {
		if err := p.summarizer.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("close errors: %v", errs)
	}
	return nil
}

// extractKeywords 从分词结果中提取关键词
func extractKeywords(tokens []*Token, limit int) []string {
	// 按权重排序，取前N个
	if len(tokens) == 0 {
		return nil
	}

	// 简单去重和过滤
	seen := make(map[string]bool)
	var keywords []string

	for _, token := range tokens {
		if token.Weight > 0.5 && !seen[token.Text] && len(token.Text) >= 2 {
			seen[token.Text] = true
			keywords = append(keywords, token.Text)
			if len(keywords) >= limit {
				break
			}
		}
	}

	return keywords
}
