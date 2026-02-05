// Package retrieval 检索管道
package retrieval

import (
	"context"

	"github.com/cloudwego/eino/vdocstool/tools/memory/storage"
)

// Pipeline 检索管道
type Pipeline struct {
	intentParser   *IntentParser
	l1Matcher      *L1Matcher
	multiChannel   *MultiChannelRecall
	reranker       *Reranker
	assembler      *ContextAssembler
}

// NewPipeline 创建检索管道
func NewPipeline(l1 *storage.L1WorkingMemory, l2 *storage.L2ShortTermMemory, l3 *storage.L3LongTermMemory, cfg RetrievalConfig) *Pipeline {
	return &Pipeline{
		intentParser: NewIntentParser(),
		l1Matcher:    NewL1Matcher(l1),
		multiChannel: NewMultiChannelRecall(l2, l3, cfg.ChannelWeights),
		reranker:     NewReranker(cfg.SimilarityThreshold, cfg.TimeDecayFactor),
		assembler:    NewContextAssembler(cfg.DefaultTokenBudget),
	}
}

// RetrievalConfig 检索配置
type RetrievalConfig struct {
	SimilarityThreshold float64
	DefaultTokenBudget  int
	ChannelWeights      ChannelWeights
	TimeDecayFactor     float64
}

// ChannelWeights 通道权重
type ChannelWeights struct {
	SemanticSearch float64
	EntityGraph    float64
	TemporalNear   float64
	UserPinned     float64
}

// RetrieveRequest 检索请求
type RetrieveRequest struct {
	Query       string `json:"query"`
	SessionID   string `json:"session_id"`
	TokenBudget int    `json:"token_budget,omitempty"`
	TopicID     string `json:"topic_id,omitempty"` // 锁定的主题
}

// RetrieveResponse 检索响应
type RetrieveResponse struct {
	Context       []*ContextItem          `json:"context"`
	Sources       []*ContextSource        `json:"sources"`
	IntentInfo    *IntentInfo             `json:"intent_info"`
	TotalTokens   int                     `json:"total_tokens"`
	RecallStats   *RecallStats            `json:"recall_stats"`
}

// ContextItem 上下文项
type ContextItem struct {
	Content     string  `json:"content"`
	Role        string  `json:"role"`
	Source      string  `json:"source"` // l1, l2, l3
	Relevance   float64 `json:"relevance"`
	TokenCount  int     `json:"token_count"`
}

// ContextSource 上下文来源
type ContextSource struct {
	ID       string `json:"id"`
	Type     string `json:"type"` // message, capsule, archive
	Title    string `json:"title,omitempty"`
	Tier     string `json:"tier"`
}

// Retrieve 执行检索
func (p *Pipeline) Retrieve(ctx context.Context, req *RetrieveRequest) (*RetrieveResponse, error) {
	response := &RetrieveResponse{
		RecallStats: &RecallStats{},
	}

	// 第1层：意图解析
	intent := p.intentParser.Parse(req.Query)
	response.IntentInfo = intent

	// 如果不需要历史上下文，直接返回
	if !intent.NeedsHistory {
		return response, nil
	}

	// 第2层：L1 工作记忆匹配
	l1Results, l1Hit, err := p.l1Matcher.Match(ctx, req.SessionID, req.Query, req.TopicID)
	if err != nil {
		return nil, err
	}

	response.RecallStats.L1Checked = true
	response.RecallStats.L1HitCount = len(l1Results)

	// 如果 L1 命中且相似度高，直接使用
	if l1Hit && intent.SimilarityThreshold <= 0.8 {
		response.Context = l1Results
		response.Sources = p.extractSources(l1Results, "L1")
		response.TotalTokens = p.calculateTotalTokens(l1Results)
		return response, nil
	}

	// 第3层：多路召回
	multiResults, err := p.multiChannel.Recall(ctx, req.SessionID, req.Query, intent)
	if err != nil {
		return nil, err
	}

	response.RecallStats.MultiChannelResults = len(multiResults)

	// 合并 L1 和多路召回结果
	allResults := append(l1Results, multiResults...)

	// 第4层：重排序
	rerankedResults := p.reranker.Rerank(ctx, allResults, req.Query)

	// 第5层：上下文组装
	tokenBudget := req.TokenBudget
	if tokenBudget <= 0 {
		tokenBudget = p.assembler.defaultBudget
	}

	assembled := p.assembler.Assemble(rerankedResults, tokenBudget)

	response.Context = assembled
	response.Sources = p.extractSources(assembled, "")
	response.TotalTokens = p.calculateTotalTokens(assembled)

	return response, nil
}

// extractSources 提取来源
func (p *Pipeline) extractSources(items []*ContextItem, tier string) []*ContextSource {
	seen := make(map[string]bool)
	var sources []*ContextSource

	for _, item := range items {
		sourceID := item.Source
		if seen[sourceID] {
			continue
		}
		seen[sourceID] = true

		source := &ContextSource{
			ID:   sourceID,
			Type: "message",
			Tier: tier,
		}
		if tier == "" {
			source.Tier = item.Source
		}

		sources = append(sources, source)
	}

	return sources
}

// calculateTotalTokens 计算总Token数
func (p *Pipeline) calculateTotalTokens(items []*ContextItem) int {
	total := 0
	for _, item := range items {
		total += item.TokenCount
	}
	return total
}

// RecallStats 召回统计
type RecallStats struct {
	L1Checked           bool `json:"l1_checked"`
	L1HitCount          int  `json:"l1_hit_count"`
	MultiChannelResults int  `json:"multi_channel_results"`
	AfterRerank         int  `json:"after_rerank"`
	AfterAssemble       int  `json:"after_assemble"`
}
