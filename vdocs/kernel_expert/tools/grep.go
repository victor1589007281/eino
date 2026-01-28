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

// Package tools provides tool implementations for the Linux kernel expert agent.
package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// GrepTool provides text search using ripgrep.
type GrepTool struct {
	rgPath     string
	sourcePath string
	maxResults int
	timeout    time.Duration
}

// GrepInput represents input for grep search.
type GrepInput struct {
	Pattern       string   `json:"pattern"`        // Search pattern (regex supported)
	FileTypes     []string `json:"file_types"`     // File types to search (c, h, S)
	Directories   []string `json:"directories"`    // Directories to search in
	CaseSensitive bool     `json:"case_sensitive"` // Case sensitive search
	Context       int      `json:"context"`        // Context lines before/after match
	MaxResults    int      `json:"max_results"`    // Maximum number of results
	WholeWord     bool     `json:"whole_word"`     // Match whole words only
}

// GrepMatch represents a single match.
type GrepMatch struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Content string `json:"content"`
	Context string `json:"context"`
}

// GrepResult represents grep search results.
type GrepResult struct {
	Matches   []GrepMatch `json:"matches"`
	Total     int         `json:"total"`
	Truncated bool        `json:"truncated"`
}

// NewGrepTool creates a new grep tool.
func NewGrepTool(sourcePath string) *GrepTool {
	return &GrepTool{
		rgPath:     "rg", // ripgrep
		sourcePath: sourcePath,
		maxResults: 50,
		timeout:    30 * time.Second,
	}
}

// Info returns tool information.
func (t *GrepTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "grep_code",
		Desc: `Search Linux kernel source code using pattern matching.
Use this tool to find code by exact text patterns, function names, macro definitions, etc.
Supports regex patterns. Returns matching lines with file path and line number.

Example patterns:
- Function definition: "^static.*int\s+process_one_work"
- Macro definition: "#define\s+TASK_RUNNING"
- Structure: "struct\s+task_struct\s*\{"
- Function call: "schedule\s*\("`,
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"pattern": {
				Type:     schema.String,
				Desc:     "Search pattern (supports regex). E.g., 'fork', '^void\\s+do_fork'",
				Required: true,
			},
			"file_types": {
				Type:     schema.Array,
				Desc:     "File types to search: c, h, S. Default: [c, h]",
				Required: false,
			},
			"directories": {
				Type:     schema.Array,
				Desc:     "Subdirectories to search in, e.g., ['kernel', 'mm', 'fs']",
				Required: false,
			},
			"case_sensitive": {
				Type:     schema.Boolean,
				Desc:     "Case sensitive search. Default: false",
				Required: false,
			},
			"context": {
				Type:     schema.Integer,
				Desc:     "Number of context lines. Default: 3",
				Required: false,
			},
			"max_results": {
				Type:     schema.Integer,
				Desc:     "Maximum results to return. Default: 30",
				Required: false,
			},
			"whole_word": {
				Type:     schema.Boolean,
				Desc:     "Match whole words only. Default: false",
				Required: false,
			},
		}),
	}, nil
}

// InvokableRun executes the grep search.
func (t *GrepTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	var input GrepInput
	if err := json.Unmarshal([]byte(argumentsInJSON), &input); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}

	result, err := t.Search(ctx, &input)
	if err != nil {
		return "", err
	}

	output, err := json.Marshal(result)
	if err != nil {
		return "", err
	}

	return string(output), nil
}

// Search performs the grep search.
func (t *GrepTool) Search(ctx context.Context, input *GrepInput) (*GrepResult, error) {
	// Set defaults
	if input.MaxResults <= 0 {
		input.MaxResults = 30
	}
	if input.MaxResults > t.maxResults {
		input.MaxResults = t.maxResults
	}
	if input.Context <= 0 {
		input.Context = 3
	}
	if len(input.FileTypes) == 0 {
		input.FileTypes = []string{"c", "h"}
	}

	// Build ripgrep command
	args := t.buildArgs(input)

	// Determine search paths
	searchPaths := []string{t.sourcePath}
	if len(input.Directories) > 0 {
		searchPaths = make([]string, len(input.Directories))
		for i, dir := range input.Directories {
			searchPaths[i] = fmt.Sprintf("%s/%s", t.sourcePath, dir)
		}
	}
	args = append(args, searchPaths...)

	// Create command with timeout
	ctx, cancel := context.WithTimeout(ctx, t.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, t.rgPath, args...)
	output, err := cmd.Output()
	if err != nil {
		// ripgrep returns exit code 1 when no matches found
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return &GrepResult{Matches: []GrepMatch{}, Total: 0}, nil
		}
		return nil, fmt.Errorf("grep error: %w", err)
	}

	return t.parseOutput(string(output), input.MaxResults), nil
}

// buildArgs builds ripgrep arguments.
func (t *GrepTool) buildArgs(input *GrepInput) []string {
	args := []string{
		"-n",                                              // Line numbers
		"--max-count", strconv.Itoa(input.MaxResults * 2), // Get extra for dedup
		"-C", strconv.Itoa(input.Context), // Context lines
		"--color", "never", // No color codes
	}

	// Case sensitivity
	if !input.CaseSensitive {
		args = append(args, "-i")
	}

	// Whole word
	if input.WholeWord {
		args = append(args, "-w")
	}

	// File types
	for _, ft := range input.FileTypes {
		switch ft {
		case "c":
			args = append(args, "-t", "c")
		case "h":
			args = append(args, "--glob", "*.h")
		case "S":
			args = append(args, "--glob", "*.S")
		}
	}

	// Pattern
	args = append(args, input.Pattern)

	return args
}

// parseOutput parses ripgrep output.
func (t *GrepTool) parseOutput(output string, maxResults int) *GrepResult {
	matches := make([]GrepMatch, 0)
	scanner := bufio.NewScanner(strings.NewReader(output))

	var currentMatch *GrepMatch
	var contextLines []string

	for scanner.Scan() {
		line := scanner.Text()

		// Parse line: file:line:content or file-line-content (context)
		if strings.Contains(line, ":") || strings.Contains(line, "-") {
			parts := splitGrepLine(line)
			if len(parts) >= 3 {
				lineNum, err := strconv.Atoi(parts[1])
				if err != nil {
					continue
				}

				isMatch := strings.Contains(line, ":")

				if isMatch {
					// Save previous match
					if currentMatch != nil {
						currentMatch.Context = strings.Join(contextLines, "\n")
						matches = append(matches, *currentMatch)
						if len(matches) >= maxResults {
							break
						}
					}

					// Start new match
					currentMatch = &GrepMatch{
						File:    parts[0],
						Line:    lineNum,
						Content: parts[2],
					}
					contextLines = []string{}
				} else if currentMatch != nil {
					contextLines = append(contextLines, parts[2])
				}
			}
		}
	}

	// Don't forget the last match
	if currentMatch != nil && len(matches) < maxResults {
		currentMatch.Context = strings.Join(contextLines, "\n")
		matches = append(matches, *currentMatch)
	}

	return &GrepResult{
		Matches:   matches,
		Total:     len(matches),
		Truncated: len(matches) >= maxResults,
	}
}

// splitGrepLine splits a grep output line.
func splitGrepLine(line string) []string {
	// Format: file:line:content or file-line-content
	var parts []string
	var current strings.Builder
	colonCount := 0

	for _, c := range line {
		if (c == ':' || c == '-') && colonCount < 2 {
			parts = append(parts, current.String())
			current.Reset()
			colonCount++
		} else {
			current.WriteRune(c)
		}
	}
	parts = append(parts, current.String())

	return parts
}

// Compile-time check
var _ tool.InvokableTool = (*GrepTool)(nil)
