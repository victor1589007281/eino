// Package tools provides various tools for code analysis.
package tools

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// GrepTool provides code search functionality using ripgrep.
type GrepTool struct {
	sourcePath  string
	ripgrepPath string
	maxResults  int
}

// GrepToolConfig configures the GrepTool.
type GrepToolConfig struct {
	SourcePath  string
	RipgrepPath string
	MaxResults  int
}

// NewGrepTool creates a new GrepTool instance.
func NewGrepTool(config *GrepToolConfig) *GrepTool {
	ripgrepPath := config.RipgrepPath
	if ripgrepPath == "" {
		ripgrepPath = "rg"
	}
	maxResults := config.MaxResults
	if maxResults == 0 {
		maxResults = 50
	}
	return &GrepTool{
		sourcePath:  config.SourcePath,
		ripgrepPath: ripgrepPath,
		maxResults:  maxResults,
	}
}

// Info returns the tool information.
func (t *GrepTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "grep_code",
		Desc: `在MySQL源码中搜索代码。支持正则表达式。
适用场景：
- 搜索函数定义/调用
- 搜索特定代码模式
- 搜索关键字/变量名

注意：
- ripgrep正则语法中，字面量花括号需要转义，例如 interface\{ \}。
- C++代码中的模板语法 <T> 等通常不需要转义。

参数说明：
- pattern: 搜索模式，支持正则表达式
- file_types: 文件类型过滤，如 ["cc", "h"]
- directories: 目录过滤，如 ["sql/", "storage/innobase/"]
- context_lines: 上下文行数，默认3
- case_sensitive: 是否大小写敏感，默认true`,
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"pattern": {
				Type:     schema.String,
				Required: true,
				Desc:     "搜索模式，支持正则表达式",
			},
			"file_types": {
				Type: schema.Array,
				Desc: "文件类型过滤，如 [\"cc\", \"h\"]",
			},
			"directories": {
				Type: schema.Array,
				Desc: "目录过滤，如 [\"sql/\", \"storage/innobase/\"]",
			},
			"context_lines": {
				Type: schema.Integer,
				Desc: "上下文行数，默认3",
			},
			"case_sensitive": {
				Type: schema.Boolean,
				Desc: "是否大小写敏感，默认true",
			},
			"multiline": {
				Type: schema.Boolean,
				Desc: "是否启用多行匹配模式（允许匹配换行符），默认false",
			},
		}),
	}, nil
}

// GrepInput represents the input parameters for grep.
type GrepInput struct {
	Pattern       string   `json:"pattern"`
	FileTypes     []string `json:"file_types"`
	Directories   []string `json:"directories"`
	ContextLines  int      `json:"context_lines"`
	CaseSensitive *bool    `json:"case_sensitive"`
	Multiline     bool     `json:"multiline"`
}

// GrepResult represents a single grep result.
type GrepResult struct {
	FilePath    string   `json:"file_path"`
	Line        int      `json:"line"`
	Column      int      `json:"column"`
	Content     string   `json:"content"`
	Context     []string `json:"context,omitempty"`
	MatchLength int      `json:"match_length"`
}

// InvokableRun executes the grep search.
func (t *GrepTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	var input GrepInput
	if err := json.Unmarshal([]byte(argumentsInJSON), &input); err != nil {
		return "", fmt.Errorf("failed to parse input: %w", err)
	}

	if input.Pattern == "" {
		return "", fmt.Errorf("pattern is required")
	}

	// Build ripgrep command
	args := []string{
		"--json",
		"--line-number",
		"--column",
	}

	// Context lines
	contextLines := input.ContextLines
	if contextLines == 0 {
		contextLines = 3
	}
	args = append(args, "-C", strconv.Itoa(contextLines))

	// Case sensitivity
	if input.CaseSensitive != nil && !*input.CaseSensitive {
		args = append(args, "-i")
	}

	// Multiline mode
	if input.Multiline {
		args = append(args, "--multiline", "--multiline-dotall")
	}

	// File types
	for _, ft := range input.FileTypes {
		args = append(args, "--type-add", fmt.Sprintf("custom:*.%s", ft))
		args = append(args, "-t", "custom")
	}

	// Max results
	args = append(args, "-m", strconv.Itoa(t.maxResults))

	// Pattern
	args = append(args, input.Pattern)

	// Search paths
	if len(input.Directories) > 0 {
		for _, dir := range input.Directories {
			args = append(args, fmt.Sprintf("%s/%s", t.sourcePath, dir))
		}
	} else {
		args = append(args, t.sourcePath)
	}

	// Execute ripgrep
	cmd := exec.CommandContext(ctx, t.ripgrepPath, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		// ripgrep returns exit code 1 when no matches found
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return "未找到匹配结果", nil
		}
		return "", fmt.Errorf("ripgrep error: %s", stderr.String())
	}

	// Parse JSON output
	results := t.parseRipgrepOutput(stdout.String())

	// Format output
	return t.formatResults(results), nil
}

// parseRipgrepOutput parses the JSON lines output from ripgrep.
func (t *GrepTool) parseRipgrepOutput(output string) []GrepResult {
	var results []GrepResult

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var msg map[string]interface{}
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			continue
		}

		msgType, ok := msg["type"].(string)
		if !ok || msgType != "match" {
			continue
		}

		data, ok := msg["data"].(map[string]interface{})
		if !ok {
			continue
		}

		result := GrepResult{}

		if path, ok := data["path"].(map[string]interface{}); ok {
			if text, ok := path["text"].(string); ok {
				result.FilePath = strings.TrimPrefix(text, t.sourcePath+"/")
			}
		}

		if lineNum, ok := data["line_number"].(float64); ok {
			result.Line = int(lineNum)
		}

		if lines, ok := data["lines"].(map[string]interface{}); ok {
			if text, ok := lines["text"].(string); ok {
				result.Content = strings.TrimSpace(text)
			}
		}

		if submatches, ok := data["submatches"].([]interface{}); ok && len(submatches) > 0 {
			if submatch, ok := submatches[0].(map[string]interface{}); ok {
				if start, ok := submatch["start"].(float64); ok {
					result.Column = int(start)
				}
				if end, ok := submatch["end"].(float64); ok {
					result.MatchLength = int(end) - result.Column
				}
			}
		}

		results = append(results, result)
	}

	return results
}

// formatResults formats the search results for display.
func (t *GrepTool) formatResults(results []GrepResult) string {
	if len(results) == 0 {
		return "未找到匹配结果"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("找到 %d 个匹配结果:\n\n", len(results)))

	for i, r := range results {
		if i >= t.maxResults {
			sb.WriteString(fmt.Sprintf("\n... 还有更多结果，已截断显示前 %d 条", t.maxResults))
			break
		}

		sb.WriteString(fmt.Sprintf("### %s:%d\n", r.FilePath, r.Line))
		sb.WriteString("```cpp\n")
		sb.WriteString(r.Content)
		sb.WriteString("\n```\n\n")
	}

	return sb.String()
}
