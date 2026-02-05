// Package intent 意图识别路由器
package intent

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/cloudwego/eino/vdocstool/algorithm/types"
)

// Router 意图识别路由器
type Router struct {
	config     *types.IntentConfig
	ruleEngine Recognizer
	mlEngine   Recognizer
	llmEngine  Recognizer
}

// NewRouter 创建路由器
func NewRouter(cfg *types.IntentConfig, rule, ml, llm Recognizer) *Router {
	return &Router{
		config:     cfg,
		ruleEngine: rule,
		mlEngine:   ml,
		llmEngine:  llm,
	}
}

// Route 路由并执行意图识别
func (r *Router) Route(ctx context.Context, input *Input) (*Result, error) {
	switch r.config.Strategy {
	case "cascade":
		return r.cascadeRecognize(ctx, input)
	case "voting":
		return r.votingRecognize(ctx, input)
	case "adaptive":
		return r.adaptiveRecognize(ctx, input)
	default:
		return r.cascadeRecognize(ctx, input)
	}
}

// cascadeRecognize 级联识别：快速 → 精准
func (r *Router) cascadeRecognize(ctx context.Context, input *Input) (*Result, error) {
	threshold := r.config.Fusion.ConfidenceThreshold

	// 1. 先用规则引擎（最快）
	if r.ruleEngine != nil {
		result, err := r.ruleEngine.Recognize(ctx, input)
		if err == nil && result.Confidence >= 0.9 {
			return result, nil
		}
		// 规则引擎高置信度直接返回
		if result != nil && result.Confidence >= threshold {
			return result, nil
		}
	}

	// 2. 规则置信度不够，用 ML
	if r.mlEngine != nil {
		result, err := r.mlEngine.Recognize(ctx, input)
		if err == nil && result.Confidence >= threshold {
			return result, nil
		}
	}

	// 3. ML 置信度不够，用 LLM
	if r.llmEngine != nil {
		result, err := r.llmEngine.Recognize(ctx, input)
		if err == nil {
			return result, nil
		}
	}

	// 4. 全部失败，降级返回
	return r.fallbackResult(input), nil
}

// votingRecognize 投票识别：多引擎并行，投票决策
func (r *Router) votingRecognize(ctx context.Context, input *Input) (*Result, error) {
	var wg sync.WaitGroup
	results := make([]*Result, 3)
	errors := make([]error, 3)

	// 并行调用三个引擎
	if r.ruleEngine != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[0], errors[0] = r.ruleEngine.Recognize(ctx, input)
		}()
	}

	if r.mlEngine != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[1], errors[1] = r.mlEngine.Recognize(ctx, input)
		}()
	}

	if r.llmEngine != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[2], errors[2] = r.llmEngine.Recognize(ctx, input)
		}()
	}

	wg.Wait()

	// 加权投票
	return r.weightedVote(results), nil
}

// weightedVote 加权投票
func (r *Router) weightedVote(results []*Result) *Result {
	weights := r.config.Fusion.Weights
	if weights == nil {
		weights = map[string]float64{"rule": 0.2, "ml": 0.3, "llm": 0.5}
	}

	// 统计每个意图的加权分数
	intentScores := make(map[string]float64)
	intentDetails := make(map[string]*Intent)

	engineNames := []string{"rule", "ml", "llm"}
	for i, result := range results {
		if result == nil || result.Intent == nil {
			continue
		}
		intent := result.Intent.Name
		weight := weights[engineNames[i]]
		intentScores[intent] += result.Confidence * weight
		
		// 保存详情
		if _, ok := intentDetails[intent]; !ok {
			intentDetails[intent] = result.Intent
		}
	}

	// 找出最高分意图
	var bestIntent string
	var bestScore float64
	for intent, score := range intentScores {
		if score > bestScore {
			bestScore = score
			bestIntent = intent
		}
	}

	if bestIntent == "" {
		return &Result{
			Intent:     &Intent{Name: IntentUnknown, Confidence: 0},
			Confidence: 0,
			Source:     "voting",
			Fallback:   true,
		}
	}

	// 收集备选意图
	var alternatives []*Intent
	type intentScore struct {
		name  string
		score float64
	}
	var scores []intentScore
	for intent, score := range intentScores {
		if intent != bestIntent {
			scores = append(scores, intentScore{intent, score})
		}
	}
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].score > scores[j].score
	})
	for _, s := range scores {
		if detail, ok := intentDetails[s.name]; ok {
			alternatives = append(alternatives, detail)
		}
	}

	return &Result{
		Intent:       intentDetails[bestIntent],
		Alternatives: alternatives,
		Confidence:   bestScore,
		Source:       "voting",
	}
}

// adaptiveRecognize 自适应识别：根据输入特征选择引擎
func (r *Router) adaptiveRecognize(ctx context.Context, input *Input) (*Result, error) {
	features := r.extractFeatures(input)

	// 简单明确的查询 → 规则引擎
	if features.Clarity > 0.9 && features.HasPattern && r.ruleEngine != nil {
		result, err := r.ruleEngine.Recognize(ctx, input)
		if err == nil && result.Confidence > 0.8 {
			return result, nil
		}
	}

	// 有上下文依赖 → LLM
	if (features.ContextDependent || features.Ambiguous) && r.llmEngine != nil {
		result, err := r.llmEngine.Recognize(ctx, input)
		if err == nil {
			return result, nil
		}
	}

	// 默认 → ML 或级联
	if r.mlEngine != nil {
		result, err := r.mlEngine.Recognize(ctx, input)
		if err == nil && result.Confidence > 0.7 {
			return result, nil
		}
	}

	// 降级到级联
	return r.cascadeRecognize(ctx, input)
}

// InputFeatures 输入特征
type InputFeatures struct {
	Clarity          float64 // 清晰度
	HasPattern       bool    // 是否匹配已知模式
	ContextDependent bool    // 是否依赖上下文
	Ambiguous        bool    // 是否模糊
	Length           int     // 文本长度
}

// extractFeatures 提取输入特征
func (r *Router) extractFeatures(input *Input) *InputFeatures {
	features := &InputFeatures{
		Length: len(input.Text),
	}

	// 简单特征提取
	text := input.Text

	// 清晰度：短文本且无代词通常更清晰
	if len(text) < 50 {
		features.Clarity = 0.8
	} else if len(text) < 100 {
		features.Clarity = 0.6
	} else {
		features.Clarity = 0.4
	}

	// 上下文依赖：检测代词和引用词
	contextWords := []string{"它", "这个", "那个", "之前", "刚才", "上次", "this", "that", "it", "earlier"}
	for _, word := range contextWords {
		if contains(text, word) {
			features.ContextDependent = true
			features.Clarity *= 0.7
			break
		}
	}

	// 模糊性：检测疑问和不确定词
	ambiguousWords := []string{"可能", "大概", "或者", "也许", "maybe", "probably", "perhaps"}
	for _, word := range ambiguousWords {
		if contains(text, word) {
			features.Ambiguous = true
			features.Clarity *= 0.8
			break
		}
	}

	// 有上下文时更可能需要LLM
	if len(input.Context) > 0 {
		features.ContextDependent = true
	}

	return features
}

// fallbackResult 降级结果
func (r *Router) fallbackResult(input *Input) *Result {
	return &Result{
		Intent: &Intent{
			Name:       IntentUnknown,
			Confidence: 0,
		},
		Confidence: 0,
		Source:     "fallback",
		Fallback:   true,
		Reasoning:  fmt.Sprintf("all engines failed for input: %s", truncate(input.Text, 50)),
	}
}

// contains 检查字符串是否包含子串
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsRune(s, substr))
}

func containsRune(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// truncate 截断字符串
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
