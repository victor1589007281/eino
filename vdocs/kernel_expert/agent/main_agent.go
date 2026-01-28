/*
 * Copyright 2025 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package agent

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"

	"kernel_expert/indexer"
	"kernel_expert/output"
	"kernel_expert/tools"
)

const linuxKernelExpertInstruction = `你是一位资深的Linux内核专家，拥有对Linux内核源码的深入理解。

## 你的职责
1. 基于Linux内核源代码解答用户的问题
2. 所有回答必须有源码依据，引用具体的文件和行号
3. 分析要深入到实现细节，不只是概念层面

## 工作流程
1. 理解问题：首先理解用户想知道什么
2. 搜索定位：使用grep_code或index_search工具定位相关代码
3. 深入分析：使用read_source读取源码，使用call_graph分析调用关系
4. 验证确认：确保分析结论有代码支撑
5. 组织输出：清晰地组织回答，包含代码引用

## 工具使用指南
- grep_code: 精确搜索代码，适合查找函数定义、宏定义等
- index_search: 关键词搜索，适合探索性搜索
- function_summary: 快速获取函数信息
- call_graph: 分析函数调用关系
- read_source: 读取源码文件

## 回答要求
1. 准确性：引用的代码必须真实存在
2. 完整性：分析要覆盖关键点
3. 深度：要解释为什么这样实现，不只是是什么
4. 格式：使用清晰的结构，包含代码块

## 输出格式
根据问题类型选择输出格式：
- 简单问题：直接回答，附代码引用
- 复杂问题：分章节详细分析
- 调用链问题：包含调用关系图
- 架构问题：包含架构图

记住：你的每一个结论都必须能在源码中找到依据！`

// LinuxKernelExpertConfig contains configuration for the expert agent.
type LinuxKernelExpertConfig struct {
	Model      model.ToolCallingChatModel
	SourcePath string
	IndexPath  string
	MaxWorkers int
}

// LinuxKernelExpert is the main agent for Linux kernel analysis.
type LinuxKernelExpert struct {
	agent        adk.ResumableAgent
	planner      *TaskPlanner
	coordinator  *SubAgentCoordinator
	outputFmt    *output.Formatter
	indexManager *indexer.SimpleIndexManager
	tools        *tools.ToolRegistry
}

// NewLinuxKernelExpert creates a new Linux kernel expert agent.
func NewLinuxKernelExpert(ctx context.Context, cfg *LinuxKernelExpertConfig) (*LinuxKernelExpert, error) {
	// Initialize index manager
	indexManager := indexer.NewSimpleIndexManager(cfg.SourcePath, cfg.IndexPath)
	if err := indexManager.Initialize(ctx); err != nil {
		fmt.Printf("Warning: failed to initialize index: %v\n", err)
	}

	// Create tool registry
	toolRegistry := tools.NewToolRegistry(cfg.SourcePath, indexManager)

	// Create sub-agents
	subAgents, err := createSubAgents(ctx, cfg.Model, indexManager, toolRegistry)
	if err != nil {
		return nil, fmt.Errorf("failed to create sub-agents: %w", err)
	}

	// Create main agent
	mainAgent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "LinuxKernelExpert",
		Description: "Linux内核专家，基于源码解答Linux内核相关问题",
		Instruction: linuxKernelExpertInstruction,
		Model:       cfg.Model,
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig:    createToolsNodeConfig(toolRegistry),
			EmitInternalEvents: true,
		},
		MaxIterations: 30,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create main agent: %w", err)
	}

	// Set sub-agents
	resumableAgent, err := adk.SetSubAgents(ctx, mainAgent, subAgents)
	if err != nil {
		return nil, fmt.Errorf("failed to set sub-agents: %w", err)
	}

	return &LinuxKernelExpert{
		agent:        resumableAgent,
		planner:      NewTaskPlanner(),
		coordinator:  NewSubAgentCoordinator(cfg.MaxWorkers),
		outputFmt:    output.NewFormatter(),
		indexManager: indexManager,
		tools:        toolRegistry,
	}, nil
}

// createSubAgents creates all sub-agents.
func createSubAgents(ctx context.Context, chatModel model.ToolCallingChatModel,
	indexManager *indexer.SimpleIndexManager, toolRegistry *tools.ToolRegistry) ([]adk.Agent, error) {

	// Code Search Agent
	codeSearchAgent, err := NewCodeSearchAgent(ctx, chatModel, toolRegistry)
	if err != nil {
		return nil, fmt.Errorf("failed to create CodeSearchAgent: %w", err)
	}

	// Function Analyzer Agent
	funcAnalyzerAgent, err := NewFunctionAnalyzerAgent(ctx, chatModel, toolRegistry)
	if err != nil {
		return nil, fmt.Errorf("failed to create FunctionAnalyzerAgent: %w", err)
	}

	// Call Chain Analyzer Agent
	callChainAgent, err := NewCallChainAnalyzerAgent(ctx, chatModel, toolRegistry)
	if err != nil {
		return nil, fmt.Errorf("failed to create CallChainAnalyzerAgent: %w", err)
	}

	// Architecture Analyzer Agent
	archAgent, err := NewArchitectureAnalyzerAgent(ctx, chatModel, toolRegistry)
	if err != nil {
		return nil, fmt.Errorf("failed to create ArchitectureAnalyzerAgent: %w", err)
	}

	return []adk.Agent{
		codeSearchAgent,
		funcAnalyzerAgent,
		callChainAgent,
		archAgent,
	}, nil
}

// createToolsNodeConfig creates tools node configuration.
func createToolsNodeConfig(registry *tools.ToolRegistry) compose.ToolsNodeConfig {
	return compose.ToolsNodeConfig{
		Tools: registry.GetAll(),
	}
}

// Run runs the agent with the given query.
func (e *LinuxKernelExpert) Run(ctx context.Context, query string, outputType OutputType) (*output.FormattedOutput, error) {
	// Plan tasks
	plan := e.planner.Plan(query)

	// Run agent
	input := &adk.AgentInput{
		Messages: []*schema.Message{
			schema.UserMessage(query),
		},
		EnableStreaming: false,
	}

	iter := e.agent.Run(ctx, input)

	// Collect results
	var lastContent string
	for {
		event, ok := iter.Next()
		if !ok {
			break
		}

		if event.Err != nil {
			return nil, event.Err
		}

		if event.Output != nil && event.Output.MessageOutput != nil {
			msg, err := event.Output.MessageOutput.GetMessage()
			if err != nil {
				continue
			}
			if msg != nil && msg.Content != "" {
				lastContent = msg.Content
			}
		}
	}

	// Format output
	formatted, err := e.outputFmt.Format(lastContent, plan.Intent, outputType)
	if err != nil {
		return nil, fmt.Errorf("failed to format output: %w", err)
	}

	return formatted, nil
}

// RunStream runs the agent with streaming output.
func (e *LinuxKernelExpert) RunStream(ctx context.Context, query string) *adk.AsyncIterator[*adk.AgentEvent] {
	input := &adk.AgentInput{
		Messages: []*schema.Message{
			schema.UserMessage(query),
		},
		EnableStreaming: true,
	}

	return e.agent.Run(ctx, input)
}

// SubAgentCoordinator coordinates sub-agent execution.
type SubAgentCoordinator struct {
	maxWorkers int
	semaphore  chan struct{}
}

// NewSubAgentCoordinator creates a new coordinator.
func NewSubAgentCoordinator(maxWorkers int) *SubAgentCoordinator {
	if maxWorkers <= 0 {
		maxWorkers = 4
	}
	return &SubAgentCoordinator{
		maxWorkers: maxWorkers,
		semaphore:  make(chan struct{}, maxWorkers),
	}
}

// Message type aliases for convenience
type Message = schema.Message

// UserMessage creates a user message (wrapper for schema.UserMessage)
func UserMessage(content string) *Message { return schema.UserMessage(content) }

// Helper function to get tools for sub-agents
func getToolsForAgent(registry *tools.ToolRegistry, toolNames []string) []tool.BaseTool {
	result := make([]tool.BaseTool, 0, len(toolNames))
	for _, name := range toolNames {
		if t := registry.Get(name); t != nil {
			result = append(result, t)
		}
	}
	return result
}
