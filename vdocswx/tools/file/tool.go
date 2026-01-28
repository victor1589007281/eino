// Package file 提供文件操作工具
package file

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// FileReadTool 文件读取工具
type FileReadTool struct {
	name        string
	description string
	basePath    string
}

// FileWriteTool 文件写入工具
type FileWriteTool struct {
	name        string
	description string
	basePath    string
}

// FileReadInput 文件读取输入
type FileReadInput struct {
	Path string `json:"path"`
}

// FileReadOutput 文件读取输出
type FileReadOutput struct {
	Content  string `json:"content"`
	Size     int64  `json:"size"`
	FileType string `json:"file_type"`
}

// FileWriteInput 文件写入输入
type FileWriteInput struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Append  bool   `json:"append,omitempty"`
}

// FileWriteOutput 文件写入输出
type FileWriteOutput struct {
	Success bool   `json:"success"`
	Path    string `json:"path"`
	Size    int64  `json:"size"`
}

// NewFileReadTool 创建文件读取工具
func NewFileReadTool(basePath string) *FileReadTool {
	return &FileReadTool{
		name:        "file_read",
		description: "读取指定路径的文件内容，支持Markdown、文本等格式",
		basePath:    basePath,
	}
}

// NewFileWriteTool 创建文件写入工具
func NewFileWriteTool(basePath string) *FileWriteTool {
	return &FileWriteTool{
		name:        "file_write",
		description: "将内容写入指定路径的文件",
		basePath:    basePath,
	}
}

// Info 返回工具信息
func (t *FileReadTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: t.name,
		Desc: t.description,
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"path": {
				Type:     schema.String,
				Desc:     "文件路径（相对于工作目录或绝对路径）",
				Required: true,
			},
		}),
	}, nil
}

// InvokableRun 执行文件读取
func (t *FileReadTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	var input FileReadInput
	if err := json.Unmarshal([]byte(argumentsInJSON), &input); err != nil {
		return "", err
	}
	
	// 处理路径
	fullPath := t.resolvePath(input.Path)
	
	// 安全检查
	if !t.isPathSafe(fullPath) {
		return "", fmt.Errorf("access denied: path is outside allowed directory")
	}
	
	// 读取文件
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}
	
	// 获取文件信息
	info, err := os.Stat(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to get file info: %w", err)
	}
	
	output := FileReadOutput{
		Content:  string(content),
		Size:     info.Size(),
		FileType: filepath.Ext(fullPath),
	}
	
	result, err := json.Marshal(output)
	if err != nil {
		return "", err
	}
	
	return string(result), nil
}

// resolvePath 解析路径
func (t *FileReadTool) resolvePath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(t.basePath, path)
}

// isPathSafe 检查路径安全性
func (t *FileReadTool) isPathSafe(path string) bool {
	// 获取绝对路径
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	
	// 获取基础路径的绝对路径
	absBasePath, err := filepath.Abs(t.basePath)
	if err != nil {
		return false
	}
	
	// 检查是否在允许的目录内
	return strings.HasPrefix(absPath, absBasePath)
}

// Info 返回工具信息
func (t *FileWriteTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: t.name,
		Desc: t.description,
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"path": {
				Type:     schema.String,
				Desc:     "文件路径（相对于工作目录或绝对路径）",
				Required: true,
			},
			"content": {
				Type:     schema.String,
				Desc:     "要写入的内容",
				Required: true,
			},
			"append": {
				Type:     schema.Boolean,
				Desc:     "是否追加模式（默认为覆盖）",
				Required: false,
			},
		}),
	}, nil
}

// InvokableRun 执行文件写入
func (t *FileWriteTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	var input FileWriteInput
	if err := json.Unmarshal([]byte(argumentsInJSON), &input); err != nil {
		return "", err
	}
	
	// 处理路径
	fullPath := t.resolvePath(input.Path)
	
	// 安全检查
	if !t.isPathSafe(fullPath) {
		return "", fmt.Errorf("access denied: path is outside allowed directory")
	}
	
	// 确保目录存在
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}
	
	// 确定写入模式
	var flag int
	if input.Append {
		flag = os.O_WRONLY | os.O_CREATE | os.O_APPEND
	} else {
		flag = os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	}
	
	// 写入文件
	file, err := os.OpenFile(fullPath, flag, 0644)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()
	
	n, err := file.WriteString(input.Content)
	if err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}
	
	output := FileWriteOutput{
		Success: true,
		Path:    fullPath,
		Size:    int64(n),
	}
	
	result, err := json.Marshal(output)
	if err != nil {
		return "", err
	}
	
	return string(result), nil
}

// resolvePath 解析路径
func (t *FileWriteTool) resolvePath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(t.basePath, path)
}

// isPathSafe 检查路径安全性
func (t *FileWriteTool) isPathSafe(path string) bool {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	
	absBasePath, err := filepath.Abs(t.basePath)
	if err != nil {
		return false
	}
	
	return strings.HasPrefix(absPath, absBasePath)
}
