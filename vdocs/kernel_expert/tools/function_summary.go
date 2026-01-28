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

// FunctionSummaryTool provides function information lookup.
type FunctionSummaryTool struct {
	indexManager *indexer.SimpleIndexManager
}

// FunctionSummaryInput represents input for function lookup.
type FunctionSummaryInput struct {
	FunctionName string `json:"function_name"` // Function name
	FileName     string `json:"file_name"`     // Optional file name filter
}

// FunctionSummaryOutput represents function summary output.
type FunctionSummaryOutput struct {
	Found     bool                    `json:"found"`
	Functions []*indexer.FunctionInfo `json:"functions"`
	Count     int                     `json:"count"`
}

// NewFunctionSummaryTool creates a new function summary tool.
func NewFunctionSummaryTool(manager *indexer.SimpleIndexManager) *FunctionSummaryTool {
	return &FunctionSummaryTool{
		indexManager: manager,
	}
}

// Info returns tool information.
func (t *FunctionSummaryTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "function_summary",
		Desc: `Get summary information about a Linux kernel function.
Returns function signature, file location, line numbers, and basic metadata.
Use this for quick lookup of function definitions without reading full source.

Returns:
- Function name, file path, start/end lines
- Function signature
- Parameter information
- Whether static/inline`,
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"function_name": {
				Type:     schema.String,
				Desc:     "Name of the function to look up, e.g., 'do_fork', 'schedule'",
				Required: true,
			},
			"file_name": {
				Type:     schema.String,
				Desc:     "Optional file name filter to disambiguate functions with same name",
				Required: false,
			},
		}),
	}, nil
}

// InvokableRun executes the function lookup.
func (t *FunctionSummaryTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	var input FunctionSummaryInput
	if err := json.Unmarshal([]byte(argumentsInJSON), &input); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}

	if input.FunctionName == "" {
		return "", fmt.Errorf("function_name is required")
	}

	functions := t.indexManager.GetFunction(input.FunctionName)

	// Filter by file if specified
	if input.FileName != "" {
		filtered := make([]*indexer.FunctionInfo, 0)
		for _, f := range functions {
			if f.File == input.FileName || containsPath(f.File, input.FileName) {
				filtered = append(filtered, f)
			}
		}
		functions = filtered
	}

	output := &FunctionSummaryOutput{
		Found:     len(functions) > 0,
		Functions: functions,
		Count:     len(functions),
	}

	result, err := json.Marshal(output)
	if err != nil {
		return "", err
	}

	return string(result), nil
}

// containsPath checks if path contains the given substring.
func containsPath(path, sub string) bool {
	return len(path) >= len(sub) && (path == sub ||
		path[len(path)-len(sub):] == sub ||
		contains(path, "/"+sub) ||
		contains(path, sub+"/"))
}

func contains(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// Compile-time check
var _ tool.InvokableTool = (*FunctionSummaryTool)(nil)
