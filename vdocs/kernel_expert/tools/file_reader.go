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
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"

	"kernel_expert/indexer"
)

// FileReaderTool provides source file reading capability.
type FileReaderTool struct {
	indexManager *indexer.SimpleIndexManager
	maxLines     int
}

// FileReaderInput represents input for file reading.
type FileReaderInput struct {
	FilePath     string `json:"file_path"`     // File path (relative to source root)
	StartLine    int    `json:"start_line"`    // Start line number (1-based)
	EndLine      int    `json:"end_line"`      // End line number
	ContextLines int    `json:"context_lines"` // Extra context lines around range
}

// FileReaderOutput represents file reading output.
type FileReaderOutput struct {
	FilePath  string `json:"file_path"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
	Content   string `json:"content"`
	LineCount int    `json:"line_count"`
	Truncated bool   `json:"truncated"`
}

// NewFileReaderTool creates a new file reader tool.
func NewFileReaderTool(manager *indexer.SimpleIndexManager) *FileReaderTool {
	return &FileReaderTool{
		indexManager: manager,
		maxLines:     200,
	}
}

// Info returns tool information.
func (t *FileReaderTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "read_source",
		Desc: `Read Linux kernel source file content.
Returns source code lines from the specified file and line range.
Use this to examine implementation details after locating code via grep or call_graph.

Tips:
- Specify start_line and end_line to read specific sections
- Use context_lines to include surrounding code
- If only start_line is given, reads to end of file or max limit`,
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"file_path": {
				Type:     schema.String,
				Desc:     "Source file path relative to kernel root, e.g., 'kernel/fork.c', 'include/linux/sched.h'",
				Required: true,
			},
			"start_line": {
				Type:     schema.Integer,
				Desc:     "Starting line number (1-based). Default: 1",
				Required: false,
			},
			"end_line": {
				Type:     schema.Integer,
				Desc:     "Ending line number. Default: start_line + 100",
				Required: false,
			},
			"context_lines": {
				Type:     schema.Integer,
				Desc:     "Extra lines before start and after end. Default: 0",
				Required: false,
			},
		}),
	}, nil
}

// InvokableRun executes the file reading.
func (t *FileReaderTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	var input FileReaderInput
	if err := json.Unmarshal([]byte(argumentsInJSON), &input); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}

	if input.FilePath == "" {
		return "", fmt.Errorf("file_path is required")
	}

	// Set defaults
	if input.StartLine <= 0 {
		input.StartLine = 1
	}
	if input.EndLine <= 0 {
		input.EndLine = input.StartLine + 100
	}

	// Apply context
	actualStart := input.StartLine - input.ContextLines
	if actualStart < 1 {
		actualStart = 1
	}
	actualEnd := input.EndLine + input.ContextLines

	// Limit lines
	if actualEnd-actualStart > t.maxLines {
		actualEnd = actualStart + t.maxLines
	}

	lines, err := t.indexManager.ReadFile(input.FilePath, actualStart, actualEnd)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	truncated := false
	if len(lines) >= t.maxLines {
		truncated = true
	}

	// Format output with line numbers
	var sb strings.Builder
	for i, line := range lines {
		lineNum := actualStart + i
		sb.WriteString(fmt.Sprintf("%6d | %s\n", lineNum, line))
	}

	output := &FileReaderOutput{
		FilePath:  input.FilePath,
		StartLine: actualStart,
		EndLine:   actualStart + len(lines) - 1,
		Content:   sb.String(),
		LineCount: len(lines),
		Truncated: truncated,
	}

	result, err := json.Marshal(output)
	if err != nil {
		return "", err
	}

	return string(result), nil
}

// Compile-time check
var _ tool.InvokableTool = (*FileReaderTool)(nil)
