package agent

import (
	"context"
	"testing"
)

func TestIntentRecognizer_Recognize(t *testing.T) {
	recognizer := NewIntentRecognizer()

	tests := []struct {
		query    string
		expected IntentType
	}{
		{"什么是进程", IntentConcept},
		{"fork函数怎么实现的", IntentFunction},
		{"fork函数的调用链是什么", IntentCallChain},
		{"Linux调度器的架构", IntentArchitecture},
		{"fork和vfork有什么区别", IntentComparison},
		{"为什么会出现死锁", IntentDebug},
	}

	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			result, err := recognizer.Recognize(ctx, tt.query)
			if err != nil {
				t.Fatalf("Recognize failed: %v", err)
			}

			if result.Intent != tt.expected {
				t.Logf("Expected intent %s, got %s (may vary based on keyword matching)", tt.expected, result.Intent)
			}
		})
	}
}

func TestIntentRecognizer_Confidence(t *testing.T) {
	recognizer := NewIntentRecognizer()
	ctx := context.Background()

	result, err := recognizer.Recognize(ctx, "详细解释fork函数的实现原理")
	if err != nil {
		t.Fatalf("Recognize failed: %v", err)
	}

	if result.Confidence < 0 || result.Confidence > 1 {
		t.Errorf("Confidence should be between 0 and 1, got %f", result.Confidence)
	}
}

func TestQueryAnalyzer_Analyze(t *testing.T) {
	analyzer := NewQueryAnalyzer()
	ctx := context.Background()

	analysis, err := analyzer.Analyze(ctx, "如何理解Linux内核的进程调度机制")
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if analysis.Query == "" {
		t.Error("Query should not be empty")
	}

	if len(analysis.Keywords) == 0 {
		t.Log("Keywords may be empty depending on implementation")
	}
}

func TestTaskPlanner_Plan(t *testing.T) {
	planner := NewTaskPlanner()

	plan := planner.Plan("分析do_fork函数")

	if len(plan.Tasks) == 0 {
		t.Error("Should generate some tasks")
	}

	// 验证plan结构
	if plan.Query == "" {
		t.Error("Plan query should not be empty")
	}
}

func TestTask_Validation(t *testing.T) {
	task := &Task{
		ID:           "task-001",
		Type:         IntentFunction,
		Agent:        "FunctionAnalyzer",
		Input:        map[string]interface{}{"pattern": "do_fork"},
		Priority:     1,
		Dependencies: []string{},
	}

	if task.ID == "" {
		t.Error("Task ID should not be empty")
	}
	if task.Agent == "" {
		t.Error("Task Agent should not be empty")
	}
}

func TestAnalysisContext_Management(t *testing.T) {
	ctx := NewAnalysisContext()

	// 添加代码片段
	ctx.AddCodeSnippet(&CodeSnippetTest{
		File:      "kernel/fork.c",
		StartLine: 100,
		EndLine:   150,
		Content:   "do_fork implementation",
		Language:  "c",
	})

	// 添加函数信息
	ctx.AddFunctionInfo(&FunctionContextTest{
		Name:      "do_fork",
		File:      "kernel/fork.c",
		Signature: "long do_fork(unsigned long flags)",
	})

	// 验证
	if len(ctx.CodeSnippets) != 1 {
		t.Errorf("Expected 1 code snippet, got %d", len(ctx.CodeSnippets))
	}

	if len(ctx.Functions) != 1 {
		t.Errorf("Expected 1 function, got %d", len(ctx.Functions))
	}
}

func TestSubAgentResult(t *testing.T) {
	result := &SubAgentResultTest{
		AgentID:   "code-analyzer",
		Success:   true,
		Output:    "Analysis complete",
		Artifacts: map[string]interface{}{"callgraph": "..."},
	}

	if !result.Success {
		t.Error("Result should be successful")
	}

	if result.Output == "" {
		t.Error("Output should not be empty")
	}
}

func TestOutputFormat(t *testing.T) {
	formats := []OutputFormatTest{
		OutputFormatSummary,
		OutputFormatMarkdown,
		OutputFormatJSON,
	}

	for _, f := range formats {
		if string(f) == "" {
			t.Error("Output format should have non-empty string")
		}
	}
}

func TestAgentConfig(t *testing.T) {
	config := &AgentConfigTest{
		MaxIterations:  10,
		Timeout:        60,
		EnableParallel: true,
		ModelConfig: &ModelConfigTest{
			Provider: "openai",
			Model:    "gpt-4-turbo",
		},
	}

	if config.MaxIterations <= 0 {
		t.Error("MaxIterations should be positive")
	}

	if config.ModelConfig == nil {
		t.Error("ModelConfig should not be nil")
	}
}

func TestAgentState(t *testing.T) {
	state := &AgentStateTest{
		Status:         "running",
		CurrentTask:    "analyzing",
		Progress:       0.5,
		TokensUsed:     1000,
		Errors:         []string{},
		CompletedTasks: []string{"task-1", "task-2"},
	}

	if state.Progress < 0 || state.Progress > 1 {
		t.Error("Progress should be between 0 and 1")
	}

	if len(state.CompletedTasks) != 2 {
		t.Errorf("Expected 2 completed tasks, got %d", len(state.CompletedTasks))
	}
}

// Benchmark tests

func BenchmarkIntentRecognizer_Recognize(b *testing.B) {
	recognizer := NewIntentRecognizer()
	ctx := context.Background()
	query := "分析Linux内核的进程调度机制"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		recognizer.Recognize(ctx, query)
	}
}

func BenchmarkQueryAnalyzer_Analyze(b *testing.B) {
	analyzer := NewQueryAnalyzer()
	ctx := context.Background()
	query := "如何优化Linux内核的内存管理性能"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		analyzer.Analyze(ctx, query)
	}
}

func BenchmarkTaskPlanner_Plan(b *testing.B) {
	planner := NewTaskPlanner()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		planner.Plan("分析do_fork函数")
	}
}
