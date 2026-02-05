package retrieval

import (
	"testing"
)

func TestIntentParser_Parse(t *testing.T) {
	parser := NewIntentParser()

	tests := []struct {
		name              string
		query             string
		wantNeedsHistory  bool
		wantExplicitSwitch bool
		wantHasReference  bool
		wantQueryType     string
	}{
		{
			name:              "显式主题切换",
			query:             "回到刚才的话题",
			wantNeedsHistory:  true,
			wantExplicitSwitch: true,
			wantHasReference:  false,
			wantQueryType:     "statement",
		},
		{
			name:              "引用之前内容",
			query:             "之前说的Redis方案是什么？",
			wantNeedsHistory:  true,
			wantExplicitSwitch: false,
			wantHasReference:  true,
			wantQueryType:     "question",
		},
		{
			name:              "独立问题",
			query:             "Python怎么读取文件？",
			wantNeedsHistory:  true, // 问题类型默认需要历史
			wantExplicitSwitch: false,
			wantHasReference:  false,
			wantQueryType:     "question",
		},
		{
			name:              "新话题指示",
			query:             "换个话题，我想问关于Go的问题",
			wantNeedsHistory:  false,
			wantExplicitSwitch: false,
			wantHasReference:  false,
			wantQueryType:     "statement",
		},
		{
			name:              "命令类型",
			query:             "请帮我创建一个Python脚本",
			wantNeedsHistory:  false,
			wantExplicitSwitch: false,
			wantHasReference:  false,
			wantQueryType:     "command",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.Parse(tt.query)

			if result.NeedsHistory != tt.wantNeedsHistory {
				t.Errorf("Parse() NeedsHistory = %v, want %v", result.NeedsHistory, tt.wantNeedsHistory)
			}

			if result.ExplicitTopicSwitch != tt.wantExplicitSwitch {
				t.Errorf("Parse() ExplicitTopicSwitch = %v, want %v", result.ExplicitTopicSwitch, tt.wantExplicitSwitch)
			}

			if result.HasReference != tt.wantHasReference {
				t.Errorf("Parse() HasReference = %v, want %v", result.HasReference, tt.wantHasReference)
			}

			if result.QueryType != tt.wantQueryType {
				t.Errorf("Parse() QueryType = %v, want %v", result.QueryType, tt.wantQueryType)
			}
		})
	}
}

func TestIntentParser_ExtractTopicKeywords(t *testing.T) {
	parser := NewIntentParser()

	tests := []struct {
		name         string
		query        string
		wantKeywords []string
	}{
		{
			name:         "包含技术关键词",
			query:        "如何使用Redis缓存数据？",
			wantKeywords: []string{"redis", "缓存"},
		},
		{
			name:         "包含数据库关键词",
			query:        "MySQL和MongoDB这两个数据库的区别是什么？",
			wantKeywords: []string{"mysql", "mongodb", "数据库"},
		},
		{
			name:         "无技术关键词",
			query:        "今天天气怎么样？",
			wantKeywords: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.Parse(tt.query)

			for _, wantKw := range tt.wantKeywords {
				found := false
				for _, gotKw := range result.TopicKeywords {
					if gotKw == wantKw {
						found = true
						break
					}
				}
				if !found && len(tt.wantKeywords) > 0 {
					t.Errorf("Parse() missing keyword %s in %v", wantKw, result.TopicKeywords)
				}
			}
		})
	}
}

func TestReranker_Rerank(t *testing.T) {
	reranker := NewReranker(0.3, 0.95)

	items := []*ContextItem{
		{Content: "Python异步编程", Role: "assistant", Source: "L1", Relevance: 0.6},
		{Content: "Redis缓存策略", Role: "assistant", Source: "L2", Relevance: 0.8},
		{Content: "无关内容", Role: "user", Source: "L1", Relevance: 0.1},
		{Content: "Python asyncio教程", Role: "assistant", Source: "L2", Relevance: 0.7},
	}

	query := "Python异步"
	result := reranker.Rerank(nil, items, query)

	// 验证排序（高相关度在前）
	if len(result) == 0 {
		t.Fatal("Rerank returned empty result")
	}

	// 验证过滤（低相关度应该被过滤）
	for _, item := range result {
		if item.Relevance < 0.3 {
			t.Errorf("Item with relevance %f should have been filtered", item.Relevance)
		}
	}

	// 验证顺序
	for i := 0; i < len(result)-1; i++ {
		if result[i].Relevance < result[i+1].Relevance {
			t.Error("Results should be sorted by relevance descending")
		}
	}
}

func TestContextAssembler_Assemble(t *testing.T) {
	assembler := NewContextAssembler(1000)

	items := []*ContextItem{
		{Content: "高相关度内容1", Role: "assistant", Source: "L1", Relevance: 0.9, TokenCount: 100},
		{Content: "高相关度内容2", Role: "assistant", Source: "L1", Relevance: 0.85, TokenCount: 100},
		{Content: "中等相关度内容", Role: "assistant", Source: "L2", Relevance: 0.5, TokenCount: 200},
		{Content: "低相关度内容", Role: "assistant", Source: "L3", Relevance: 0.2, TokenCount: 300},
	}

	result := assembler.Assemble(items, 500)

	// 验证总Token不超预算
	totalTokens := 0
	for _, item := range result {
		totalTokens += item.TokenCount
	}

	if totalTokens > 500 {
		t.Errorf("Total tokens %d exceeds budget 500", totalTokens)
	}

	// 验证高相关度内容被优先包含
	if len(result) == 0 {
		t.Fatal("Assemble returned empty result")
	}

	if result[0].Relevance < 0.7 {
		t.Error("High relevance content should be included first")
	}
}

func TestDiversityReranker_Rerank(t *testing.T) {
	reranker := NewDiversityReranker(2)

	items := []*ContextItem{
		{Content: "L1内容1", Source: "L1", Relevance: 0.9},
		{Content: "L1内容2", Source: "L1", Relevance: 0.85},
		{Content: "L1内容3", Source: "L1", Relevance: 0.8},
		{Content: "L2内容1", Source: "L2", Relevance: 0.7},
		{Content: "L2内容2", Source: "L2", Relevance: 0.6},
	}

	result := reranker.Rerank(items)

	// 统计每个来源的数量
	sourceCount := make(map[string]int)
	for _, item := range result {
		sourceCount[item.Source]++
	}

	// 验证每个来源不超过2个
	for source, count := range sourceCount {
		if count > 2 {
			t.Errorf("Source %s has %d items, should be at most 2", source, count)
		}
	}
}

func TestCalculateTextSimilarity(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		text     string
		minSim   float64
	}{
		{
			name:   "完全匹配",
			query:  "Python异步编程",
			text:   "Python异步编程教程",
			minSim: 0.5,
		},
		{
			name:   "部分匹配",
			query:  "Redis缓存",
			text:   "使用Redis实现数据缓存",
			minSim: 0.3,
		},
		{
			name:   "无匹配",
			query:  "Python编程",
			text:   "Java开发指南",
			minSim: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sim := calculateTextSimilarity(tt.query, tt.text)
			if sim < tt.minSim {
				t.Errorf("calculateTextSimilarity() = %v, want >= %v", sim, tt.minSim)
			}
		})
	}
}

func TestEstimateTokenCount(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		minCount int
		maxCount int
	}{
		{
			name:     "纯中文",
			text:     "这是一段中文文本测试",
			minCount: 10,
			maxCount: 20,
		},
		{
			name:     "纯英文",
			text:     "This is an English text for testing",
			minCount: 5,
			maxCount: 10,
		},
		{
			name:     "中英混合",
			text:     "这是一段mixed中英文text测试",
			minCount: 8,
			maxCount: 18,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count := estimateTokenCount(tt.text)
			if count < tt.minCount || count > tt.maxCount {
				t.Errorf("estimateTokenCount() = %d, want between %d and %d", count, tt.minCount, tt.maxCount)
			}
		})
	}
}
