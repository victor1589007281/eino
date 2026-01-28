package router

import (
	"context"
	"testing"
	"time"
)

func TestRouter_SelectModel(t *testing.T) {
	config := DefaultRouterConfig()
	r := NewRouter(config)

	ctx := context.Background()

	// Test simple task routing
	model, err := r.SelectModel(ctx, TaskTypeSimple)
	if err != nil {
		t.Fatalf("SelectModel failed: %v", err)
	}
	if model == nil {
		t.Fatal("Expected model, got nil")
	}

	// Test reasoning task routing
	model, err = r.SelectModel(ctx, TaskTypeReasoning)
	if err != nil {
		t.Fatalf("SelectModel failed: %v", err)
	}
	if model == nil {
		t.Fatal("Expected model, got nil")
	}
}

func TestRouter_SelectModelByPriority(t *testing.T) {
	config := &RouterConfig{
		Models: map[string]ModelConfig{
			"model-high": {
				Provider: ProviderDeepSeek,
				Model:    "model-high",
				Priority: 1,
				Enabled:  true,
			},
			"model-low": {
				Provider: ProviderQwen,
				Model:    "model-low",
				Priority: 10,
				Enabled:  true,
			},
		},
		CostOptimize: false,
		LoadBalance:  false,
	}

	r := NewRouter(config)
	ctx := context.Background()

	model, err := r.SelectModel(ctx, TaskTypeSimple)
	if err != nil {
		t.Fatalf("SelectModel failed: %v", err)
	}
	if model.Model != "model-high" {
		t.Errorf("Expected model-high (priority 1), got %s", model.Model)
	}
}

func TestRouter_SelectModelByCost(t *testing.T) {
	config := &RouterConfig{
		Models: map[string]ModelConfig{
			"model-cheap": {
				Provider:     ProviderDeepSeek,
				Model:        "model-cheap",
				Priority:     10,
				CostPerToken: 0.001,
				Enabled:      true,
			},
			"model-expensive": {
				Provider:     ProviderQwen,
				Model:        "model-expensive",
				Priority:     1,
				CostPerToken: 0.01,
				Enabled:      true,
			},
		},
		CostOptimize: true,
		LoadBalance:  false,
	}

	r := NewRouter(config)
	ctx := context.Background()

	model, err := r.SelectModel(ctx, TaskTypeSimple)
	if err != nil {
		t.Fatalf("SelectModel failed: %v", err)
	}
	if model.Model != "model-cheap" {
		t.Errorf("Expected model-cheap (lower cost), got %s", model.Model)
	}
}

func TestRouter_SelectModelByLoad(t *testing.T) {
	config := &RouterConfig{
		Models: map[string]ModelConfig{
			"model-a": {
				Provider: ProviderDeepSeek,
				Model:    "model-a",
				Enabled:  true,
			},
			"model-b": {
				Provider: ProviderQwen,
				Model:    "model-b",
				Enabled:  true,
			},
		},
		CostOptimize: false,
		LoadBalance:  true,
	}

	r := NewRouter(config)
	ctx := context.Background()

	// Increment load on model-a
	r.IncrementLoad("model-a")
	r.IncrementLoad("model-a")
	r.IncrementLoad("model-a")

	model, err := r.SelectModel(ctx, TaskTypeSimple)
	if err != nil {
		t.Fatalf("SelectModel failed: %v", err)
	}
	// Should select model-b (lower load)
	if model.Model != "model-b" {
		t.Errorf("Expected model-b (lower load), got %s", model.Model)
	}
}

func TestRouter_GetFallback(t *testing.T) {
	config := &RouterConfig{
		Models: map[string]ModelConfig{
			"model-a": {Provider: ProviderDeepSeek, Model: "model-a", Enabled: true},
			"model-b": {Provider: ProviderQwen, Model: "model-b", Enabled: true},
			"model-c": {Provider: ProviderErnie, Model: "model-c", Enabled: true},
		},
		FallbackOrder: []string{"model-a", "model-b", "model-c"},
	}

	r := NewRouter(config)

	// Test fallback from model-a
	fallback, err := r.GetFallback("model-a")
	if err != nil {
		t.Fatalf("GetFallback failed: %v", err)
	}
	if fallback.Model != "model-b" {
		t.Errorf("Expected model-b, got %s", fallback.Model)
	}

	// Test fallback from model-b
	fallback, err = r.GetFallback("model-b")
	if err != nil {
		t.Fatalf("GetFallback failed: %v", err)
	}
	if fallback.Model != "model-c" {
		t.Errorf("Expected model-c, got %s", fallback.Model)
	}

	// Test no fallback from last model
	_, err = r.GetFallback("model-c")
	if err == nil {
		t.Error("Expected error for last model in fallback chain")
	}
}

func TestRouter_RecordUsage(t *testing.T) {
	config := DefaultRouterConfig()
	r := NewRouter(config)

	modelName := "deepseek-chat"
	
	r.RecordUsage(modelName, true, 100, 50*time.Millisecond)
	r.RecordUsage(modelName, true, 200, 100*time.Millisecond)
	r.RecordUsage(modelName, false, 0, 10*time.Millisecond)

	stats := r.GetModelStats()
	modelStats := stats[modelName]

	if modelStats == nil {
		t.Fatal("Expected model stats")
	}
	if modelStats.RequestCount != 3 {
		t.Errorf("Expected RequestCount 3, got %d", modelStats.RequestCount)
	}
	if modelStats.ErrorCount != 1 {
		t.Errorf("Expected ErrorCount 1, got %d", modelStats.ErrorCount)
	}
	if modelStats.TotalTokens != 300 {
		t.Errorf("Expected TotalTokens 300, got %d", modelStats.TotalTokens)
	}
}

func TestRouter_SetModelAvailability(t *testing.T) {
	config := DefaultRouterConfig()
	r := NewRouter(config)

	modelName := "deepseek-chat"

	// Disable model
	r.SetModelAvailability(modelName, false)

	stats := r.GetModelStats()
	if stats[modelName].Available {
		t.Error("Model should be unavailable")
	}

	// Enable model
	r.SetModelAvailability(modelName, true)

	stats = r.GetModelStats()
	if !stats[modelName].Available {
		t.Error("Model should be available")
	}
}

func TestRouter_LoadManagement(t *testing.T) {
	config := DefaultRouterConfig()
	r := NewRouter(config)

	modelName := "deepseek-chat"

	// Increment load
	r.IncrementLoad(modelName)
	r.IncrementLoad(modelName)

	stats := r.GetModelStats()
	if stats[modelName].CurrentLoad != 2 {
		t.Errorf("Expected CurrentLoad 2, got %d", stats[modelName].CurrentLoad)
	}

	// Decrement load
	r.DecrementLoad(modelName)

	stats = r.GetModelStats()
	if stats[modelName].CurrentLoad != 1 {
		t.Errorf("Expected CurrentLoad 1, got %d", stats[modelName].CurrentLoad)
	}

	// Decrement below 0 should stay at 0
	r.DecrementLoad(modelName)
	r.DecrementLoad(modelName)

	stats = r.GetModelStats()
	if stats[modelName].CurrentLoad != 0 {
		t.Errorf("Expected CurrentLoad 0, got %d", stats[modelName].CurrentLoad)
	}
}

func TestRouter_NoAvailableModel(t *testing.T) {
	config := &RouterConfig{
		Models: map[string]ModelConfig{
			"model-a": {
				Provider:  ProviderDeepSeek,
				Model:     "model-a",
				Enabled:   false, // disabled
				TaskTypes: []TaskType{TaskTypeSimple},
			},
		},
	}

	r := NewRouter(config)
	ctx := context.Background()

	_, err := r.SelectModel(ctx, TaskTypeSimple)
	if err == nil {
		t.Error("Expected error when no model available")
	}
}

func TestRouter_TaskTypeSupport(t *testing.T) {
	config := &RouterConfig{
		Models: map[string]ModelConfig{
			"reasoning-model": {
				Provider:  ProviderDeepSeek,
				Model:     "reasoning-model",
				Enabled:   true,
				TaskTypes: []TaskType{TaskTypeReasoning, TaskTypeAnalysis},
			},
			"simple-model": {
				Provider:  ProviderQwen,
				Model:     "simple-model",
				Enabled:   true,
				TaskTypes: []TaskType{TaskTypeSimple},
			},
		},
	}

	r := NewRouter(config)
	ctx := context.Background()

	// Simple task should use simple-model
	model, err := r.SelectModel(ctx, TaskTypeSimple)
	if err != nil {
		t.Fatalf("SelectModel failed: %v", err)
	}
	if model.Model != "simple-model" {
		t.Errorf("Expected simple-model for simple task, got %s", model.Model)
	}

	// Reasoning task should use reasoning-model
	model, err = r.SelectModel(ctx, TaskTypeReasoning)
	if err != nil {
		t.Fatalf("SelectModel failed: %v", err)
	}
	if model.Model != "reasoning-model" {
		t.Errorf("Expected reasoning-model for reasoning task, got %s", model.Model)
	}
}

func TestRouter_StatsFunc(t *testing.T) {
	config := DefaultRouterConfig()
	r := NewRouter(config)

	called := false
	r.SetStatsFunc(func(provider string, success bool, tokens int64, duration time.Duration) {
		called = true
		if provider != "deepseek-chat" {
			t.Errorf("Expected provider deepseek-chat, got %s", provider)
		}
		if tokens != 100 {
			t.Errorf("Expected tokens 100, got %d", tokens)
		}
	})

	r.RecordUsage("deepseek-chat", true, 100, 50*time.Millisecond)

	if !called {
		t.Error("Stats function was not called")
	}
}

func TestDefaultRouterConfig(t *testing.T) {
	config := DefaultRouterConfig()

	if config.DefaultProvider != ProviderDeepSeek {
		t.Errorf("Expected default provider DeepSeek, got %s", config.DefaultProvider)
	}
	if len(config.Models) == 0 {
		t.Error("Expected some default models")
	}
	if len(config.FallbackOrder) == 0 {
		t.Error("Expected some fallback order")
	}
	if len(config.TaskRouting) == 0 {
		t.Error("Expected some task routing")
	}
}

func BenchmarkRouter_SelectModel(b *testing.B) {
	config := DefaultRouterConfig()
	r := NewRouter(config)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = r.SelectModel(ctx, TaskTypeSimple)
	}
}

func BenchmarkRouter_RecordUsage(b *testing.B) {
	config := DefaultRouterConfig()
	r := NewRouter(config)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.RecordUsage("deepseek-chat", true, 100, 50*time.Millisecond)
	}
}
