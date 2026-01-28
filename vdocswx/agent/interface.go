// Package agent 定义了微信公众号文章润色系统的Agent接口和核心类型
package agent

import (
	"context"

	"github.com/cloudwego/eino/schema"
)

// ArticleInput 定义文章输入结构
type ArticleInput struct {
	Content     string            // 文章内容（Markdown格式）
	Title       string            // 文章标题
	FilePath    string            // 文章文件路径（可选）
	Type        ArticleType       // 文章类型
	Metadata    map[string]string // 额外元数据
	UserContext string            // 用户额外上下文或要求
}

// ArticleType 定义文章类型
type ArticleType string

const (
	ArticleTypeTech      ArticleType = "tech"       // 技术类文章
	ArticleTypeNonTech   ArticleType = "non_tech"   // 非技术类文章
	ArticleTypeTutorial  ArticleType = "tutorial"   // 教程类文章
	ArticleTypeNews      ArticleType = "news"       // 新闻资讯类
	ArticleTypeOpinion   ArticleType = "opinion"    // 观点类文章
	ArticleTypeUnknown   ArticleType = "unknown"    // 未知类型
)

// PolishResult 定义润色结果
type PolishResult struct {
	OriginalContent  string            // 原始内容
	PolishedContent  string            // 润色后内容
	Changes          []Change          // 修改记录
	Suggestions      []Suggestion      // 改进建议
	Statistics       Statistics        // 统计信息
	SubAgentResults  map[string]*SubAgentResult // 各子Agent处理结果
}

// Change 定义单个修改
type Change struct {
	Type        ChangeType // 修改类型
	Original    string     // 原始文本
	Modified    string     // 修改后文本
	Reason      string     // 修改原因
	Position    Position   // 修改位置
	Confidence  float64    // 置信度 (0-1)
}

// ChangeType 定义修改类型
type ChangeType string

const (
	ChangeTypeGrammar    ChangeType = "grammar"    // 语法修正
	ChangeTypeLogic      ChangeType = "logic"      // 逻辑优化
	ChangeTypeStructure  ChangeType = "structure"  // 结构调整
	ChangeTypeStyle      ChangeType = "style"      // 风格统一
	ChangeTypePunctuation ChangeType = "punctuation" // 标点修正
	ChangeTypeSpelling   ChangeType = "spelling"   // 拼写修正
	ChangeTypeExpression ChangeType = "expression" // 表达优化
)

// Position 定义文本位置
type Position struct {
	StartLine   int // 起始行
	EndLine     int // 结束行
	StartColumn int // 起始列
	EndColumn   int // 结束列
}

// Suggestion 定义改进建议
type Suggestion struct {
	Type        SuggestionType // 建议类型
	Content     string         // 建议内容
	Priority    Priority       // 优先级
	Section     string         // 相关章节
	Reason      string         // 建议原因
}

// SuggestionType 定义建议类型
type SuggestionType string

const (
	SuggestionTypeTitle      SuggestionType = "title"      // 标题建议
	SuggestionTypeStructure  SuggestionType = "structure"  // 结构建议
	SuggestionTypeContent    SuggestionType = "content"    // 内容建议
	SuggestionTypeReadability SuggestionType = "readability" // 可读性建议
	SuggestionTypeSEO        SuggestionType = "seo"        // SEO建议
	SuggestionTypeEngagement SuggestionType = "engagement" // 互动性建议
)

// Priority 定义优先级
type Priority int

const (
	PriorityLow    Priority = 1
	PriorityMedium Priority = 2
	PriorityHigh   Priority = 3
)

// Statistics 定义统计信息
type Statistics struct {
	OriginalCharCount  int     // 原始字符数
	PolishedCharCount  int     // 润色后字符数
	OriginalWordCount  int     // 原始词数
	PolishedWordCount  int     // 润色后词数
	TotalChanges       int     // 总修改数
	GrammarChanges     int     // 语法修改数
	StyleChanges       int     // 风格修改数
	StructureChanges   int     // 结构修改数
	ReadabilityScore   float64 // 可读性评分
	TokensUsed         int     // 使用的Token数
	ProcessingTime     int64   // 处理时间(毫秒)
}

// SubAgentResult 定义子Agent处理结果
type SubAgentResult struct {
	AgentName    string   // Agent名称
	Success      bool     // 是否成功
	Output       string   // 输出内容
	Changes      []Change // 修改记录
	Error        string   // 错误信息
	TokensUsed   int      // Token使用量
	Duration     int64    // 处理时间(毫秒)
}

// PolishAgent 定义润色Agent接口
type PolishAgent interface {
	// Polish 对文章进行润色处理
	Polish(ctx context.Context, input *ArticleInput) (*PolishResult, error)
	
	// Stream 流式润色处理
	Stream(ctx context.Context, input *ArticleInput) (<-chan *StreamEvent, error)
	
	// Chat 多轮对话交互
	Chat(ctx context.Context, sessionID string, message string) (*ChatResponse, error)
	
	// GetHistory 获取会话历史
	GetHistory(ctx context.Context, sessionID string) ([]*schema.Message, error)
}

// SubAgent 定义子Agent接口
type SubAgent interface {
	// Name 返回Agent名称
	Name() string
	
	// Process 处理文章内容
	Process(ctx context.Context, content string, articleType ArticleType) (*SubAgentResult, error)
	
	// CanHandle 判断是否能处理该类型文章
	CanHandle(articleType ArticleType) bool
}

// StreamEvent 定义流式事件
type StreamEvent struct {
	Type    StreamEventType // 事件类型
	Content string          // 事件内容
	Agent   string          // 产生事件的Agent
	Done    bool            // 是否完成
	Error   error           // 错误信息
}

// StreamEventType 定义流式事件类型
type StreamEventType string

const (
	StreamEventTypeProgress StreamEventType = "progress"  // 进度更新
	StreamEventTypeChunk    StreamEventType = "chunk"     // 内容块
	StreamEventTypeChange   StreamEventType = "change"    // 修改事件
	StreamEventTypeDone     StreamEventType = "done"      // 完成事件
	StreamEventTypeError    StreamEventType = "error"     // 错误事件
)

// ChatResponse 定义对话响应
type ChatResponse struct {
	Message      string         // 响应消息
	Modifications []Change      // 本轮修改
	Suggestions  []Suggestion   // 建议
	SessionID    string         // 会话ID
}

// IntentType 定义意图类型
type IntentType string

const (
	IntentTypePolish     IntentType = "polish"       // 全文润色
	IntentTypeGrammar    IntentType = "grammar"      // 语法检查
	IntentTypeStyle      IntentType = "style"        // 风格优化
	IntentTypeStructure  IntentType = "structure"    // 结构调整
	IntentTypeQuestion   IntentType = "question"     // 问题咨询
	IntentTypeModify     IntentType = "modify"       // 指定修改
	IntentTypeRollback   IntentType = "rollback"     // 回滚修改
	IntentTypeExport     IntentType = "export"       // 导出结果
)

// Intent 定义识别出的意图
type Intent struct {
	Type       IntentType        // 意图类型
	Confidence float64           // 置信度
	Entities   map[string]string // 提取的实体
	RawQuery   string            // 原始查询
}

// Plan 定义执行计划
type Plan struct {
	Steps     []PlanStep // 执行步骤
	Parallel  bool       // 是否并行执行
	Estimated int64      // 预估时间(毫秒)
}

// PlanStep 定义计划步骤
type PlanStep struct {
	ID          string   // 步骤ID
	AgentName   string   // 执行Agent
	Action      string   // 执行动作
	Input       string   // 输入数据
	DependsOn   []string // 依赖的步骤ID
	Priority    Priority // 优先级
}
