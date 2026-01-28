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
	"context"
	"encoding/json"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"

	"kernel_expert/indexer"
)

// CallGraphTool provides function call relationship analysis.
type CallGraphTool struct {
	indexManager *indexer.SimpleIndexManager
}

// CallGraphInput represents input for call graph analysis.
type CallGraphInput struct {
	Function  string `json:"function"`  // Function to analyze
	Direction string `json:"direction"` // callers, callees, or both
	MaxDepth  int    `json:"max_depth"` // Maximum depth to traverse
}

// CallGraphOutput represents call graph output.
type CallGraphOutput struct {
	Function   string                 `json:"function"`
	Direction  string                 `json:"direction"`
	CallChain  *indexer.CallChainNode `json:"call_chain"`
	TotalNodes int                    `json:"total_nodes"`
}

// NewCallGraphTool creates a new call graph tool.
func NewCallGraphTool(manager *indexer.SimpleIndexManager) *CallGraphTool {
	return &CallGraphTool{
		indexManager: manager,
	}
}

// Info returns tool information.
func (t *CallGraphTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "call_graph",
		Desc: `Analyze function call relationships in Linux kernel.
Returns call chain showing which functions call or are called by the target function.
Essential for understanding code flow and dependencies.

Directions:
- callers: functions that call this function (backward trace)
- callees: functions called by this function (forward trace)
- both: both directions

Returns a tree structure with function name, file, line number at each node.`,
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"function": {
				Type:     schema.String,
				Desc:     "Function name to analyze, e.g., 'do_fork', 'schedule'",
				Required: true,
			},
			"direction": {
				Type:     schema.String,
				Desc:     "Direction: 'callers' (who calls this), 'callees' (what this calls), or 'both'. Default: callees",
				Required: false,
			},
			"max_depth": {
				Type:     schema.Integer,
				Desc:     "Maximum depth to traverse. Default: 5, Max: 10",
				Required: false,
			},
		}),
	}, nil
}

// InvokableRun executes the call graph analysis.
func (t *CallGraphTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	var input CallGraphInput
	if err := json.Unmarshal([]byte(argumentsInJSON), &input); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}

	if input.Function == "" {
		return "", fmt.Errorf("function is required")
	}

	// Set defaults
	if input.Direction == "" {
		input.Direction = "callees"
	}
	if input.MaxDepth <= 0 {
		input.MaxDepth = 5
	}
	if input.MaxDepth > 10 {
		input.MaxDepth = 10
	}

	chain := t.indexManager.GetCallChain(input.Function, input.Direction, input.MaxDepth)

	output := &CallGraphOutput{
		Function:   input.Function,
		Direction:  input.Direction,
		CallChain:  chain,
		TotalNodes: countNodes(chain),
	}

	result, err := json.Marshal(output)
	if err != nil {
		return "", err
	}

	return string(result), nil
}

// countNodes counts the total number of nodes in a call chain.
func countNodes(node *indexer.CallChainNode) int {
	if node == nil {
		return 0
	}
	count := 1
	for _, child := range node.Children {
		count += countNodes(child)
	}
	return count
}

// Compile-time check
var _ tool.InvokableTool = (*CallGraphTool)(nil)
