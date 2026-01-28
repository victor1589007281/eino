package models

import (
	"context"
	"testing"
)

func TestComplexityRouter_Route(t *testing.T) {
	registry := NewModelRegistry()
	registry.InitDefaultModels()

	router := NewComplexityRouter(registry)
	ctx := context.Background()

	tests := []struct {
		name       string
		complexity TaskComplexity
		wantModels []string // 期望的候选模型
	}{
		{
			name:       "Simple task",
			complexity: ComplexitySimple,
			wantModels: []string{"gpt-4o-mini", "qwen-turbo", "claude-3-haiku", "deepseek-chat"},
		},
		{
			name:       "Moderate task",
			complexity: ComplexityModerate,
			wantModels: []string{"gpt-4o", "qwen-plus", "claude-3-sonnet", "glm-4"},
		},
		{
			name:       "Complex task",
			complexity: ComplexityComplex,
			wantModels: []string{"gpt-4-turbo", "qwen-max", "claude-3-sonnet", "deepseek-coder"},
		},
		{
			name:       "Expert task",
			complexity: ComplexityExpert,
			wantModels: []string{"gpt-4-turbo", "claude-3-opus", "qwen-max"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &Task{
				Complexity: tt.complexity,
			}

			model, err := router.Route(ctx, task)
			if err != nil {
				t.Fatalf("Route failed: %v", err)
			}

			found := false
			for _, m := range tt.wantModels {
				if model.Name == m {
					found = true
					break
				}
			}

			if !found {
				t.Errorf("Expected one of %v, got %s", tt.wantModels, model.Name)
			}
		})
	}
}

func TestComplexityRouter_WithConstraints(t *testing.T) {
	registry := NewModelRegistry()
	registry.InitDefaultModels()

	router := NewComplexityRouter(registry)
	ctx := context.Background()

	// 测试指定模型
	task := &Task{
		Complexity: ComplexityModerate,
		Constraints: &Constraints{
			Models: []string{"qwen-plus"},
		},
	}

	model, err := router.Route(ctx, task)
	if err != nil {
		t.Fatalf("Route failed: %v", err)
	}

	if model.Name != "qwen-plus" {
		t.Errorf("Expected qwen-plus, got %s", model.Name)
	}
}

func TestCostRouter_Route(t *testing.T) {
	registry := NewModelRegistry()
	registry.InitDefaultModels()

	router := NewCostRouter(registry)
	ctx := context.Background()

	task := &Task{
		InputTokens: 1000,
	}

	model, err := router.Route(ctx, task)
	if err != nil {
		t.Fatalf("Route failed: %v", err)
	}

	// 应该选择最便宜的模型（本地模型或低价模型）
	cheapModels := []string{"llama3", "codellama", "qwen2", "gpt-4o-mini", "claude-3-haiku", "deepseek-chat"}
	found := false
	for _, m := range cheapModels {
		if model.Name == m {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("Expected a cheap model, got %s (price: input=%f, output=%f)",
			model.Name, model.InputPrice, model.OutputPrice)
	}
}

func TestComplexityEvaluator_Evaluate(t *testing.T) {
	evaluator := NewComplexityEvaluator()

	tests := []struct {
		name     string
		query    string
		intent   string
		expected TaskComplexity
	}{
		{
			name:     "Simple concept query",
			query:    "什么是fork",
			intent:   "concept",
			expected: ComplexitySimple,
		},
		{
			name:     "Moderate function query",
			query:    "fork函数是如何实现的",
			intent:   "function",
			expected: ComplexityModerate,
		},
		{
			name:     "Complex architecture query with keywords",
			query:    "详细解释Linux调度器的架构设计，包括所有的调度类",
			intent:   "architecture",
			expected: ComplexityComplex,
		},
		{
			name:     "Expert debug query",
			query:    "为什么在高并发场景下会出现这个性能问题，请深入分析完整的调用链",
			intent:   "debug",
			expected: ComplexityExpert,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := evaluator.Evaluate(tt.query, tt.intent)

			// 允许一级误差
			diff := int(got) - int(tt.expected)
			if diff < -1 || diff > 1 {
				t.Errorf("Evaluate(%q, %q) = %d, want ~%d", tt.query, tt.intent, got, tt.expected)
			}
		})
	}
}

func TestModelRegistry_ListModels(t *testing.T) {
	registry := NewModelRegistry()
	registry.InitDefaultModels()

	models := registry.ListModels()
	if len(models) == 0 {
		t.Error("Expected some models")
	}

	// 验证有各种提供商的模型
	providers := make(map[string]bool)
	for _, m := range models {
		providers[m.Provider] = true
	}

	expectedProviders := []string{"openai", "anthropic", "qwen", "zhipu", "moonshot", "deepseek"}
	for _, p := range expectedProviders {
		if !providers[p] {
			t.Errorf("Expected provider %s", p)
		}
	}
}

func TestModelRegistry_GetModelInfo(t *testing.T) {
	registry := NewModelRegistry()
	registry.InitDefaultModels()

	info, ok := registry.GetModelInfo("gpt-4-turbo")
	if !ok {
		t.Fatal("Expected gpt-4-turbo to exist")
	}

	if info.Provider != "openai" {
		t.Errorf("Expected provider openai, got %s", info.Provider)
	}

	if info.MaxContext != 128000 {
		t.Errorf("Expected max context 128000, got %d", info.MaxContext)
	}

	if !info.SupportsTools {
		t.Error("Expected gpt-4-turbo to support tools")
	}
}

func TestLoadBalanceRouter_Route(t *testing.T) {
	registry := NewModelRegistry()
	registry.InitDefaultModels()

	weights := map[string]int{
		"gpt-4o":    50,
		"qwen-plus": 30,
		"glm-4":     20,
	}

	router := NewLoadBalanceRouter(registry, weights)
	ctx := context.Background()

	// 统计路由分布
	counts := make(map[string]int)
	iterations := 100

	for i := 0; i < iterations; i++ {
		task := &Task{}
		model, err := router.Route(ctx, task)
		if err != nil {
			t.Fatalf("Route failed: %v", err)
		}
		counts[model.Name]++
	}

	// 验证分布大致符合权重
	// gpt-4o 应该有大约50%
	gptRatio := float64(counts["gpt-4o"]) / float64(iterations)
	if gptRatio < 0.3 || gptRatio > 0.7 {
		t.Errorf("gpt-4o ratio %f not in expected range", gptRatio)
	}
}

func BenchmarkComplexityRouter_Route(b *testing.B) {
	registry := NewModelRegistry()
	registry.InitDefaultModels()

	router := NewComplexityRouter(registry)
	ctx := context.Background()
	task := &Task{Complexity: ComplexityModerate}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		router.Route(ctx, task)
	}
}

func BenchmarkComplexityEvaluator_Evaluate(b *testing.B) {
	evaluator := NewComplexityEvaluator()
	query := "详细解释Linux调度器的架构设计"
	intent := "architecture"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		evaluator.Evaluate(query, intent)
	}
}
