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

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/compose"

	"kernel_expert/tools"
)

// CodeSearchAgent searches and locates code.
const codeSearchInstruction = `你是一个代码搜索专家，负责在Linux内核源码中精确定位代码。

## 职责
1. 使用grep_code进行精确的模式匹配搜索
2. 使用index_search进行关键词搜索
3. 返回最相关的代码位置

## 搜索策略
1. 函数定义：使用模式 "^(static\s+)?[\w\s\*]+\s+函数名\s*\("
2. 宏定义：使用模式 "#define\s+宏名"
3. 结构体：使用模式 "struct\s+结构体名\s*\{"
4. 通用搜索：使用index_search进行关键词组合搜索

## 输出要求
- 返回文件路径和行号
- 包含匹配的代码片段
- 按相关性排序结果`

// NewCodeSearchAgent creates a code search agent.
func NewCodeSearchAgent(ctx context.Context, m model.ToolCallingChatModel,
	registry *tools.ToolRegistry) (adk.ResumableAgent, error) {

	return adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "CodeSearchAgent",
		Description: "代码搜索Agent，负责在Linux内核源码中定位代码",
		Instruction: codeSearchInstruction,
		Model:       m,
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				Tools: getToolsForAgent(registry, []string{"grep_code", "index_search"}),
			},
		},
		MaxIterations: 10,
	})
}

// FunctionAnalyzerAgent analyzes function implementations.
const functionAnalyzerInstruction = `你是一个函数分析专家，负责深入分析Linux内核函数的实现。

## 职责
1. 获取函数的基本信息（签名、参数、返回值）
2. 分析函数的核心实现逻辑
3. 识别关键的数据结构和算法
4. 评估函数的复杂度

## 分析步骤
1. 使用function_summary获取函数基本信息
2. 使用read_source读取完整实现
3. 分析参数的用途和校验逻辑
4. 识别关键的控制流和数据流
5. 标注重要的代码段

## 输出要求
- 函数签名和位置
- 参数说明（名称、类型、用途）
- 核心逻辑解释
- 关键代码段引用`

// NewFunctionAnalyzerAgent creates a function analyzer agent.
func NewFunctionAnalyzerAgent(ctx context.Context, m model.ToolCallingChatModel,
	registry *tools.ToolRegistry) (adk.ResumableAgent, error) {

	return adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "FunctionAnalyzerAgent",
		Description: "函数分析Agent，负责深入分析Linux内核函数实现",
		Instruction: functionAnalyzerInstruction,
		Model:       m,
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				Tools: getToolsForAgent(registry, []string{"function_summary", "read_source", "grep_code"}),
			},
		},
		MaxIterations: 15,
	})
}

// CallChainAnalyzerAgent analyzes call relationships.
const callChainAnalyzerInstruction = `你是一个调用链分析专家，负责追踪Linux内核函数的调用关系。

## 职责
1. 追踪函数的调用者（谁调用了这个函数）
2. 追踪函数的被调用者（这个函数调用了谁）
3. 构建完整的调用链
4. 识别关键的调用路径

## 分析步骤
1. 使用call_graph获取调用关系
2. 使用function_summary获取关键函数信息
3. 使用read_source验证调用关系
4. 构建从入口到目标的完整路径

## 输出要求
- 调用链的树状结构
- 每个节点的函数名、文件、行号
- 关键函数的简要说明
- 调用关系的时序描述`

// NewCallChainAnalyzerAgent creates a call chain analyzer agent.
func NewCallChainAnalyzerAgent(ctx context.Context, m model.ToolCallingChatModel,
	registry *tools.ToolRegistry) (adk.ResumableAgent, error) {

	return adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "CallChainAnalyzerAgent",
		Description: "调用链分析Agent，负责追踪Linux内核函数的调用关系",
		Instruction: callChainAnalyzerInstruction,
		Model:       m,
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				Tools: getToolsForAgent(registry, []string{"call_graph", "function_summary", "read_source"}),
			},
		},
		MaxIterations: 15,
	})
}

// ArchitectureAnalyzerAgent analyzes system architecture.
const architectureAnalyzerInstruction = `你是一个架构分析专家，负责分析Linux内核子系统的架构。

## 职责
1. 识别子系统的主要组件
2. 分析组件之间的关系
3. 理解数据流向
4. 绘制架构图

## 分析步骤
1. 使用index_search找到相关文件
2. 使用grep_code搜索关键接口
3. 使用read_source分析核心实现
4. 归纳出整体架构

## 输出要求
- 子系统概述
- 主要组件列表及其职责
- 组件间的依赖关系
- 数据流描述
- 关键接口和数据结构`

// NewArchitectureAnalyzerAgent creates an architecture analyzer agent.
func NewArchitectureAnalyzerAgent(ctx context.Context, m model.ToolCallingChatModel,
	registry *tools.ToolRegistry) (adk.ResumableAgent, error) {

	return adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "ArchitectureAnalyzerAgent",
		Description: "架构分析Agent，负责分析Linux内核子系统架构",
		Instruction: architectureAnalyzerInstruction,
		Model:       m,
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				Tools: getToolsForAgent(registry, []string{"index_search", "grep_code", "read_source"}),
			},
		},
		MaxIterations: 20,
	})
}
