package tools

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// SymbolLookupTool provides symbol lookup functionality using ctags.
type SymbolLookupTool struct {
	sourcePath string
	tagsPath   string
	ctagsPath  string
}

// SymbolLookupConfig configures the SymbolLookupTool.
type SymbolLookupConfig struct {
	SourcePath string
	TagsPath   string
	CtagsPath  string
}

// NewSymbolLookupTool creates a new SymbolLookupTool instance.
func NewSymbolLookupTool(config *SymbolLookupConfig) *SymbolLookupTool {
	ctagsPath := config.CtagsPath
	if ctagsPath == "" {
		ctagsPath = "ctags"
	}
	return &SymbolLookupTool{
		sourcePath: config.SourcePath,
		tagsPath:   config.TagsPath,
		ctagsPath:  ctagsPath,
	}
}

// Info returns the tool information.
func (t *SymbolLookupTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "symbol_lookup",
		Desc: `精确查找符号（函数、变量、类型）的定义位置。

返回信息：
- 符号类型（function/variable/type/macro）
- 定义位置（文件:行号）
- 签名/声明
- 所属模块

适用场景：
- 查找函数定义
- 查找类型声明
- 定位特定符号`,
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"symbol": {
				Type:     schema.String,
				Required: true,
				Desc:     "符号名称",
			},
			"type": {
				Type: schema.String,
				Desc: "符号类型：function | variable | type | macro | any",
			},
			"fuzzy": {
				Type: schema.Boolean,
				Desc: "是否模糊匹配，默认false",
			},
		}),
	}, nil
}

// SymbolInput represents the input parameters for symbol lookup.
type SymbolInput struct {
	Symbol string `json:"symbol"`
	Type   string `json:"type"`
	Fuzzy  bool   `json:"fuzzy"`
}

// SymbolResult represents a symbol lookup result.
type SymbolResult struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	FilePath   string `json:"file_path"`
	Line       int    `json:"line"`
	Signature  string `json:"signature"`
	Module     string `json:"module"`
	Scope      string `json:"scope,omitempty"`
	AccessType string `json:"access_type,omitempty"`
}

// InvokableRun executes the symbol lookup.
func (t *SymbolLookupTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	var input SymbolInput
	if err := json.Unmarshal([]byte(argumentsInJSON), &input); err != nil {
		return "", fmt.Errorf("failed to parse input: %w", err)
	}

	if input.Symbol == "" {
		return "", fmt.Errorf("symbol is required")
	}

	// Use ctags to find symbol
	results, err := t.lookupSymbol(ctx, input)
	if err != nil {
		return "", err
	}

	return t.formatResults(results), nil
}

// lookupSymbol searches for a symbol using ctags.
func (t *SymbolLookupTool) lookupSymbol(ctx context.Context, input SymbolInput) ([]SymbolResult, error) {
	// Build ctags command for on-demand lookup
	args := []string{
		"-R",
		"--output-format=json",
		"--fields=+nKS",
		"--languages=C,C++",
	}

	// Add pattern based on fuzzy mode
	if input.Fuzzy {
		args = append(args, fmt.Sprintf("--name-pattern=.*%s.*", input.Symbol))
	} else {
		args = append(args, fmt.Sprintf("--name-pattern=^%s$", input.Symbol))
	}

	// Filter by type if specified
	if input.Type != "" && input.Type != "any" {
		kindMap := map[string]string{
			"function": "f",
			"variable": "v",
			"type":     "t,s,c,g,u",
			"macro":    "d",
		}
		if kinds, ok := kindMap[input.Type]; ok {
			args = append(args, fmt.Sprintf("--kinds-c++=%s", kinds))
		}
	}

	args = append(args, t.sourcePath)

	cmd := exec.CommandContext(ctx, t.ctagsPath, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// Try alternative approach using readtags if ctags fails
		return t.lookupFromTagsFile(ctx, input)
	}

	return t.parseCtagsOutput(stdout.String()), nil
}

// lookupFromTagsFile searches in existing tags file.
func (t *SymbolLookupTool) lookupFromTagsFile(ctx context.Context, input SymbolInput) ([]SymbolResult, error) {
	if t.tagsPath == "" {
		return nil, fmt.Errorf("no tags file configured")
	}

	// Use readtags utility
	args := []string{"-t", t.tagsPath}
	if input.Fuzzy {
		args = append(args, "-p", "-i", input.Symbol)
	} else {
		args = append(args, "-", input.Symbol)
	}

	cmd := exec.CommandContext(ctx, "readtags", args...)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		return nil, err
	}

	return t.parseReadtagsOutput(stdout.String()), nil
}

// parseCtagsOutput parses JSON output from ctags.
func (t *SymbolLookupTool) parseCtagsOutput(output string) []SymbolResult {
	var results []SymbolResult

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var entry map[string]interface{}
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}

		result := SymbolResult{}

		if name, ok := entry["name"].(string); ok {
			result.Name = name
		}
		if kind, ok := entry["kind"].(string); ok {
			result.Type = t.normalizeKind(kind)
		}
		if path, ok := entry["path"].(string); ok {
			result.FilePath = strings.TrimPrefix(path, t.sourcePath+"/")
			result.Module = t.detectModule(path)
		}
		if line, ok := entry["line"].(float64); ok {
			result.Line = int(line)
		}
		if signature, ok := entry["signature"].(string); ok {
			result.Signature = signature
		}
		if scope, ok := entry["scope"].(string); ok {
			result.Scope = scope
		}

		results = append(results, result)
	}

	return results
}

// parseReadtagsOutput parses output from readtags.
func (t *SymbolLookupTool) parseReadtagsOutput(output string) []SymbolResult {
	var results []SymbolResult

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, "!_TAG") {
			continue
		}

		// Parse tab-separated format: name\tfile\tpattern\tfields
		parts := strings.Split(line, "\t")
		if len(parts) < 3 {
			continue
		}

		result := SymbolResult{
			Name:     parts[0],
			FilePath: strings.TrimPrefix(parts[1], t.sourcePath+"/"),
			Module:   t.detectModule(parts[1]),
		}

		// Parse additional fields
		for _, field := range parts[3:] {
			if strings.HasPrefix(field, "line:") {
				fmt.Sscanf(field, "line:%d", &result.Line)
			} else if strings.HasPrefix(field, "kind:") {
				result.Type = t.normalizeKind(strings.TrimPrefix(field, "kind:"))
			} else if strings.HasPrefix(field, "signature:") {
				result.Signature = strings.TrimPrefix(field, "signature:")
			}
		}

		results = append(results, result)
	}

	return results
}

// normalizeKind converts ctags kind to human-readable type.
func (t *SymbolLookupTool) normalizeKind(kind string) string {
	kindMap := map[string]string{
		"f": "function",
		"v": "variable",
		"t": "typedef",
		"s": "struct",
		"c": "class",
		"g": "enum",
		"u": "union",
		"d": "macro",
		"m": "member",
		"p": "prototype",
	}
	if normalized, ok := kindMap[kind]; ok {
		return normalized
	}
	return kind
}

// detectModule determines the module based on file path.
func (t *SymbolLookupTool) detectModule(path string) string {
	if strings.Contains(path, "storage/innobase") {
		return "InnoDB"
	}
	if strings.Contains(path, "sql/") {
		if strings.Contains(path, "sql_parse") {
			return "Parser"
		}
		if strings.Contains(path, "sql_optimizer") || strings.Contains(path, "opt_") {
			return "Optimizer"
		}
		if strings.Contains(path, "sql_executor") {
			return "Executor"
		}
		return "SQL Layer"
	}
	if strings.Contains(path, "plugin/") {
		return "Plugin"
	}
	return "Core"
}

// formatResults formats the lookup results for display.
func (t *SymbolLookupTool) formatResults(results []SymbolResult) string {
	if len(results) == 0 {
		return "未找到符号定义"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("找到 %d 个符号定义:\n\n", len(results)))

	for _, r := range results {
		sb.WriteString(fmt.Sprintf("### %s\n", r.Name))
		sb.WriteString(fmt.Sprintf("- **类型**: %s\n", r.Type))
		sb.WriteString(fmt.Sprintf("- **位置**: `%s:%d`\n", r.FilePath, r.Line))
		sb.WriteString(fmt.Sprintf("- **模块**: %s\n", r.Module))
		if r.Signature != "" {
			sb.WriteString(fmt.Sprintf("- **签名**: `%s`\n", r.Signature))
		}
		if r.Scope != "" {
			sb.WriteString(fmt.Sprintf("- **作用域**: %s\n", r.Scope))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}
