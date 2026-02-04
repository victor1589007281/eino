// Package agent provides the MySQL Expert Agent implementation.
package agent

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/middlewares/skill"
	"github.com/cloudwego/eino/adk/prebuilt/supervisor"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"

	"github.com/cloudwego/eino/vdocsMysql/config"
	"github.com/cloudwego/eino/vdocsMysql/tools"
)

// MasterAgent is the main coordinator agent for MySQL kernel analysis.
type MasterAgent struct {
	config       *config.Config
	chatModel    model.ToolCallingChatModel
	agent        adk.ResumableAgent
	skillBackend skill.Backend
}

// MasterAgentConfig contains configuration for creating MasterAgent.
type MasterAgentConfig struct {
	Config    *config.Config
	ChatModel model.ToolCallingChatModel
}

// NewMasterAgent creates a new MasterAgent instance.
func NewMasterAgent(ctx context.Context, cfg *MasterAgentConfig) (*MasterAgent, error) {
	ma := &MasterAgent{
		config:    cfg.Config,
		chatModel: cfg.ChatModel,
	}

	// Create skill backend
	skillBackend, err := NewMySQLSkillBackend(cfg.Config.Source.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to create skill backend: %w", err)
	}
	ma.skillBackend = skillBackend

	// Create tools
	grepTool := tools.NewGrepTool(&tools.GrepToolConfig{
		SourcePath: cfg.Config.Source.Path,
		MaxResults: 50,
	})

	symbolTool := tools.NewSymbolLookupTool(&tools.SymbolLookupConfig{
		SourcePath: cfg.Config.Source.Path,
		TagsPath:   filepath.Join(cfg.Config.Index.Path, "tags"),
	})

	// Create skill middleware
	skillMiddleware, err := skill.New(ctx, &skill.Config{
		Backend:    skillBackend,
		UseChinese: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create skill middleware: %w", err)
	}

	// Create main agent
	mainAgent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "mysql_expert",
		Description: "MySQL内核专家Agent，专门从源码角度解答MySQL内核相关问题",
		Instruction: masterAgentInstruction,
		Model:       cfg.ChatModel,
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				Tools: []tool.BaseTool{grepTool, symbolTool},
			},
		},
		MaxIterations: cfg.Config.Agent.MaxIterations,
		Middlewares:   []adk.AgentMiddleware{skillMiddleware},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create main agent: %w", err)
	}

	// Create sub-agents
	codeSearchAgent, err := ma.createCodeSearchAgent(ctx, grepTool, symbolTool)
	if err != nil {
		return nil, fmt.Errorf("failed to create code search agent: %w", err)
	}

	functionAnalyzerAgent, err := ma.createFunctionAnalyzerAgent(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create function analyzer agent: %w", err)
	}

	// Setup supervisor pattern
	supervisorAgent, err := supervisor.New(ctx, &supervisor.Config{
		Supervisor: mainAgent,
		SubAgents:  []adk.Agent{codeSearchAgent, functionAnalyzerAgent},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create supervisor: %w", err)
	}

	ma.agent = supervisorAgent
	return ma, nil
}

// createCodeSearchAgent creates the code search sub-agent.
func (ma *MasterAgent) createCodeSearchAgent(ctx context.Context, grepTool, symbolTool tool.BaseTool) (adk.Agent, error) {
	return adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "code_search",
		Description: "代码搜索Agent，专门负责在MySQL源码中搜索代码",
		Instruction: codeSearchAgentInstruction,
		Model:       ma.chatModel,
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				Tools: []tool.BaseTool{grepTool, symbolTool},
			},
		},
		MaxIterations: 10,
	})
}

// createFunctionAnalyzerAgent creates the function analyzer sub-agent.
func (ma *MasterAgent) createFunctionAnalyzerAgent(ctx context.Context) (adk.Agent, error) {
	grepTool := tools.NewGrepTool(&tools.GrepToolConfig{
		SourcePath: ma.config.Source.Path,
		MaxResults: 100,
	})

	return adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "function_analyzer",
		Description: "函数分析Agent，专门负责分析函数调用链和代码逻辑",
		Instruction: functionAnalyzerInstruction,
		Model:       ma.chatModel,
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				Tools: []tool.BaseTool{grepTool},
			},
		},
		MaxIterations: 15,
	})
}

// Run executes the agent with the given input.
func (ma *MasterAgent) Run(ctx context.Context, input *adk.AgentInput, opts ...adk.AgentRunOption) *adk.AsyncIterator[*adk.AgentEvent] {
	return ma.agent.Run(ctx, input, opts...)
}

// IntentType represents the type of user intent.
type IntentType string

const (
	IntentCodeSearch       IntentType = "code_search"
	IntentExplainMechanism IntentType = "explain_mechanism"
	IntentCallChain        IntentType = "call_chain"
	IntentPerformance      IntentType = "performance"
	IntentArchitecture     IntentType = "architecture"
	IntentSimulation       IntentType = "simulation"
)

// ClassifyIntent classifies the user's intent from the query.
func ClassifyIntent(query string) IntentType {
	query = strings.ToLower(query)

	// Simple rule-based classification
	if strings.Contains(query, "调用链") || strings.Contains(query, "调用关系") {
		return IntentCallChain
	}
	if strings.Contains(query, "性能") || strings.Contains(query, "瓶颈") || strings.Contains(query, "优化") {
		return IntentPerformance
	}
	if strings.Contains(query, "架构") || strings.Contains(query, "模块") || strings.Contains(query, "设计") {
		return IntentArchitecture
	}
	if strings.Contains(query, "模拟") || strings.Contains(query, "负载") {
		return IntentSimulation
	}
	if strings.Contains(query, "原理") || strings.Contains(query, "如何") || strings.Contains(query, "机制") {
		return IntentExplainMechanism
	}

	return IntentCodeSearch
}

// Agent instructions
const masterAgentInstruction = `你是一个MySQL内核专家Agent，专门从Percona Server源码角度解答MySQL内核相关问题。

## 核心能力
1. **代码搜索**: 使用grep_code工具在源码中搜索关键代码
2. **符号查找**: 使用symbol_lookup工具定位函数/变量/类型定义
3. **技能调用**: 使用skill工具获取特定领域的搜索技巧

## 工作流程
1. 分析用户问题，识别关键实体（函数名、模块名、概念等）
2. 根据问题类型选择合适的搜索策略
3. 使用工具搜索源码，获取相关代码片段
4. 分析代码逻辑，构建调用链
5. 生成带有源码引用的详细回答

## 输出要求
1. **总结模式**: 简短回答，包含关键结论和代码位置
2. **文档模式**: 详细分析，包含Mermaid图表、代码片段、调用链

## 代码引用格式
引用代码时，使用以下格式：
- 文件位置: file_path:line_number
- 函数调用链使用树状结构展示

## 注意事项
- 所有结论必须有源码依据
- 代码搜索要准确定位，避免模糊匹配
- 复杂问题要分步骤分析
- 涉及性能问题要关注热点路径
- **性能优化**: 使用 grep_code 时，尽量指定 file_types (如 ["cc", "h"]) 或 directories (如 ["sql/"]) 以减少搜索范围，提高响应速度`

const codeSearchAgentInstruction = `你是代码搜索专家，负责在MySQL源码中精确定位代码。

## 搜索策略
1. **函数定义**: 使用 "^\\s*返回类型\\s+函数名\\s*\\(" 模式
2. **函数调用**: 使用 "函数名\\s*\\(" 模式
3. **结构体定义**: 使用 "struct\\s+结构体名" 模式
4. **宏定义**: 使用 "#define\\s+宏名" 模式

## 注意事项
- ripgrep正则语法中，字面量花括号必须转义，例如 "function.*\\{" 而不是 "function.*{"
- 尽量指定 file_types (如 ["cc", "h"]) 或 directories (如 ["sql/"]) 以减少搜索范围

## 目录知识
- sql/: SQL层代码
- storage/innobase/: InnoDB存储引擎
- storage/innobase/trx/: 事务管理
- storage/innobase/lock/: 锁管理
- storage/innobase/buf/: Buffer Pool
- storage/innobase/log/: Redo Log

## 输出格式
返回搜索结果时，包含：
1. 文件路径和行号
2. 代码片段
3. 简要说明代码功能`

const functionAnalyzerInstruction = `你是函数分析专家，负责分析MySQL内核函数的调用链和逻辑。

## 分析步骤
1. 定位目标函数定义
2. 分析函数参数和返回值
3. 追踪函数调用关系
4. 理解函数内部逻辑

## 调用链格式
使用树状结构展示调用链：
函数A - file:line
├── 函数B - file:line
│   └── 函数C - file:line
└── 函数D - file:line

## 关键信息
- 函数签名
- 关键参数说明
- 返回值含义
- 调用时机
- 相关数据结构`
