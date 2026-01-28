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

package tools

import (
	"github.com/cloudwego/eino/components/tool"

	"kernel_expert/indexer"
)

// ToolRegistry manages all available tools.
type ToolRegistry struct {
	tools map[string]tool.BaseTool
}

// NewToolRegistry creates a new tool registry with all tools.
func NewToolRegistry(sourcePath string, indexManager *indexer.SimpleIndexManager) *ToolRegistry {
	registry := &ToolRegistry{
		tools: make(map[string]tool.BaseTool),
	}

	// Register grep tool
	registry.Register("grep_code", NewGrepTool(sourcePath))

	// Register index-based tools
	if indexManager != nil {
		registry.Register("index_search", NewIndexSearchTool(indexManager))
		registry.Register("function_summary", NewFunctionSummaryTool(indexManager))
		registry.Register("call_graph", NewCallGraphTool(indexManager))
		registry.Register("read_source", NewFileReaderTool(indexManager))
	}

	return registry
}

// Register registers a tool.
func (r *ToolRegistry) Register(name string, t tool.BaseTool) {
	r.tools[name] = t
}

// Get returns a tool by name.
func (r *ToolRegistry) Get(name string) tool.BaseTool {
	return r.tools[name]
}

// GetAll returns all registered tools.
func (r *ToolRegistry) GetAll() []tool.BaseTool {
	tools := make([]tool.BaseTool, 0, len(r.tools))
	for _, t := range r.tools {
		tools = append(tools, t)
	}
	return tools
}

// GetNames returns all tool names.
func (r *ToolRegistry) GetNames() []string {
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	return names
}
