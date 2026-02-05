// Package nlp NLP管道类型定义
package nlp

import (
	"context"
)

// Token 分词结果
type Token struct {
	Text   string  `json:"text"`
	Start  int     `json:"start"`
	End    int     `json:"end"`
	POS    string  `json:"pos,omitempty"`     // 词性
	Weight float64 `json:"weight,omitempty"` // 权重
}

// Entity 命名实体
type Entity struct {
	Text       string  `json:"text"`
	Type       string  `json:"type"`
	Start      int     `json:"start"`
	End        int     `json:"end"`
	Confidence float64 `json:"confidence"`
	Normalized string  `json:"normalized,omitempty"`
}

// Relation 关系
type Relation struct {
	Subject    *Entity `json:"subject"`
	Predicate  string  `json:"predicate"`
	Object     *Entity `json:"object"`
	Confidence float64 `json:"confidence"`
}

// ProcessOptions 处理选项
type ProcessOptions struct {
	Tokenize         bool `json:"tokenize"`
	ExtractEntities  bool `json:"extract_entities"`
	ExtractRelations bool `json:"extract_relations"`
	Summarize        bool `json:"summarize"`
	SummaryMaxLength int  `json:"summary_max_length"`
	Language         string `json:"language"`
}

// ProcessResult 处理结果
type ProcessResult struct {
	Tokens    []*Token    `json:"tokens,omitempty"`
	Entities  []*Entity   `json:"entities,omitempty"`
	Relations []*Relation `json:"relations,omitempty"`
	Summary   string      `json:"summary,omitempty"`
	Keywords  []string    `json:"keywords,omitempty"`
}

// Tokenizer 分词器接口
type Tokenizer interface {
	// Tokenize 分词
	Tokenize(ctx context.Context, text string) ([]*Token, error)
	// Close 关闭
	Close() error
}

// NERExtractor NER提取器接口
type NERExtractor interface {
	// Extract 提取实体
	Extract(ctx context.Context, text string) ([]*Entity, error)
	// Close 关闭
	Close() error
}

// RelationExtractor 关系提取器接口
type RelationExtractor interface {
	// Extract 提取关系
	Extract(ctx context.Context, text string, entities []*Entity) ([]*Relation, error)
	// Close 关闭
	Close() error
}

// Summarizer 摘要生成器接口
type Summarizer interface {
	// Summarize 生成摘要
	Summarize(ctx context.Context, text string, maxLength int) (string, error)
	// Close 关闭
	Close() error
}

// 常用实体类型
const (
	EntityPerson       = "PERSON"
	EntityOrganization = "ORGANIZATION"
	EntityLocation     = "LOCATION"
	EntityTime         = "TIME"
	EntityDate         = "DATE"
	EntityMoney        = "MONEY"
	EntityPercent      = "PERCENT"
	EntityTechnology   = "TECHNOLOGY"
	EntityProduct      = "PRODUCT"
	EntityEvent        = "EVENT"
)
