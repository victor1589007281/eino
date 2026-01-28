// Package agent 测试辅助函数
package agent

import "context"

// IntentRecognizer 意图识别器 (for tests)
type IntentRecognizer struct {
	classifier *IntentClassifier
}

// NewIntentRecognizer 创建意图识别器
func NewIntentRecognizer() *IntentRecognizer {
	return &IntentRecognizer{
		classifier: NewIntentClassifier(),
	}
}

// IntentResultTest 意图识别结果 (for tests)
type IntentResultTest struct {
	Intent     IntentType
	Confidence float64
	Entities   []string
}

// Recognize 识别意图 (wrapper for tests)
func (r *IntentRecognizer) Recognize(ctx context.Context, query string) (*IntentResultTest, error) {
	result := r.classifier.Classify(query)
	return &IntentResultTest{
		Intent:     IntentType(result.Primary),
		Confidence: result.Confidence,
		Entities:   result.Entities,
	}, nil
}

// QueryAnalyzer 查询分析器 (for tests)
type QueryAnalyzer struct{}

// NewQueryAnalyzer 创建查询分析器
func NewQueryAnalyzer() *QueryAnalyzer {
	return &QueryAnalyzer{}
}

// QueryAnalysis 查询分析结果
type QueryAnalysis struct {
	Query    string
	Keywords []string
}

// Analyze 分析查询
func (a *QueryAnalyzer) Analyze(ctx context.Context, query string) (*QueryAnalysis, error) {
	// 简单的关键词提取
	keywords := extractKeywords(query)
	return &QueryAnalysis{
		Query:    query,
		Keywords: keywords,
	}, nil
}

func extractKeywords(query string) []string {
	// 简单实现：按空格分词
	keywords := make([]string, 0)
	word := ""
	for _, c := range query {
		if c == ' ' || c == '，' || c == '。' || c == '的' {
			if len(word) > 1 {
				keywords = append(keywords, word)
			}
			word = ""
		} else {
			word += string(c)
		}
	}
	if len(word) > 1 {
		keywords = append(keywords, word)
	}
	return keywords
}

// TaskPlannerTest 任务规划器 (for tests)
type TaskPlannerTest struct {
	planner *TaskPlanner
}

// NewTaskPlannerForTest 创建测试用任务规划器
func NewTaskPlannerForTest() *TaskPlannerTest {
	return &TaskPlannerTest{
		planner: NewTaskPlanner(),
	}
}

// Plan 规划任务 (wrapper for tests)
func (p *TaskPlannerTest) Plan(ctx context.Context, query string, intent *IntentResultTest) ([]*TaskTest, error) {
	plan := p.planner.Plan(query)

	tasks := make([]*TaskTest, len(plan.Tasks))
	for i, t := range plan.Tasks {
		tasks[i] = &TaskTest{
			ID:           t.ID,
			Type:         string(t.Type),
			Tool:         t.Agent,
			Description:  "",
			Args:         t.Input,
			Priority:     1,
			Dependencies: t.Dependencies,
		}
	}
	return tasks, nil
}

// TaskTest 测试用任务类型
type TaskTest struct {
	ID           string
	Type         string
	Tool         string
	Description  string
	Args         map[string]interface{}
	Priority     int
	Dependencies []string
}

// AnalysisContextTest 分析上下文
type AnalysisContextTest struct {
	CodeSnippets []*CodeSnippetTest
	Functions    []*FunctionContextTest
}

// NewAnalysisContext 创建分析上下文
func NewAnalysisContext() *AnalysisContextTest {
	return &AnalysisContextTest{
		CodeSnippets: make([]*CodeSnippetTest, 0),
		Functions:    make([]*FunctionContextTest, 0),
	}
}

// AddCodeSnippet 添加代码片段
func (c *AnalysisContextTest) AddCodeSnippet(snippet *CodeSnippetTest) {
	c.CodeSnippets = append(c.CodeSnippets, snippet)
}

// AddFunctionInfo 添加函数信息
func (c *AnalysisContextTest) AddFunctionInfo(info *FunctionContextTest) {
	c.Functions = append(c.Functions, info)
}

// CodeSnippetTest 代码片段
type CodeSnippetTest struct {
	File      string
	StartLine int
	EndLine   int
	Content   string
	Language  string
}

// FunctionContextTest 函数上下文
type FunctionContextTest struct {
	Name      string
	File      string
	Signature string
}

// SubAgentResultTest 子Agent结果
type SubAgentResultTest struct {
	AgentID   string
	Success   bool
	Output    string
	Artifacts map[string]interface{}
}

// OutputFormatTest 输出格式
type OutputFormatTest string

const (
	OutputFormatSummary  OutputFormatTest = "summary"
	OutputFormatMarkdown OutputFormatTest = "markdown"
	OutputFormatJSON     OutputFormatTest = "json"
)

// AgentConfigTest Agent配置
type AgentConfigTest struct {
	MaxIterations  int
	Timeout        int
	EnableParallel bool
	ModelConfig    *ModelConfigTest
}

// ModelConfigTest 模型配置
type ModelConfigTest struct {
	Provider string
	Model    string
}

// AgentStateTest Agent状态
type AgentStateTest struct {
	Status         string
	CurrentTask    string
	Progress       float64
	TokensUsed     int
	Errors         []string
	CompletedTasks []string
}
