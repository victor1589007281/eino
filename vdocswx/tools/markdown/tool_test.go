package markdown

import (
	"context"
	"encoding/json"
	"testing"
)

func TestNewMarkdownParseTool(t *testing.T) {
	tool := NewMarkdownParseTool()
	if tool == nil {
		t.Fatal("Expected non-nil tool")
	}
	
	info, err := tool.Info(context.Background())
	if err != nil {
		t.Fatalf("Info failed: %v", err)
	}
	
	if info.Name != "markdown_parse" {
		t.Errorf("Expected name 'markdown_parse', got '%s'", info.Name)
	}
}

func TestMarkdownParseTool_Parse(t *testing.T) {
	tool := NewMarkdownParseTool()
	
	markdown := `# 测试文章

这是一段引言文字。

## 第一节

第一节的内容。

### 小节1.1

小节内容。

## 第二节

第二节的内容，包含[链接](https://example.com)和图片。

![示例图片](https://example.com/image.png)

` + "```python\n" + `def hello():
    print("Hello")
` + "```\n"

	input, _ := json.Marshal(MarkdownParseInput{Content: markdown})
	
	result, err := tool.InvokableRun(context.Background(), string(input))
	if err != nil {
		t.Fatalf("InvokableRun failed: %v", err)
	}
	
	var output MarkdownParseOutput
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("Failed to unmarshal output: %v", err)
	}
	
	// 验证标题
	if output.Title != "测试文章" {
		t.Errorf("Expected title '测试文章', got '%s'", output.Title)
	}
	
	// 验证标题数量（包括一级和二级标题）
	if len(output.Headings) < 3 {
		t.Errorf("Expected at least 3 headings, got %d", len(output.Headings))
	}
	
	// 验证链接
	if len(output.Links) == 0 {
		t.Error("Expected at least 1 link")
	}
	
	// 验证图片
	if len(output.Images) == 0 {
		t.Error("Expected at least 1 image")
	}
	
	// 验证代码块
	if len(output.CodeBlocks) == 0 {
		t.Error("Expected at least 1 code block")
	} else if output.CodeBlocks[0].Language != "python" {
		t.Errorf("Expected code block language 'python', got '%s'", output.CodeBlocks[0].Language)
	}
}

func TestMarkdownFormatTool_Format(t *testing.T) {
	tool := NewMarkdownFormatTool()
	
	tests := []struct {
		name     string
		input    string
		expected string
		changed  bool
	}{
		{
			name:     "Add space between CN and EN",
			input:    "这是一个Python测试",
			expected: "这是一个 Python 测试",
			changed:  true,
		},
		{
			name:     "Remove extra blank lines",
			input:    "第一行\n\n\n\n第二行",
			expected: "第一行\n\n第二行",
			changed:  true,
		},
		{
			name:     "No changes needed",
			input:    "这是一段正常的文字。",
			expected: "这是一段正常的文字。",
			changed:  false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inputJSON, _ := json.Marshal(MarkdownFormatInput{Content: tt.input})
			
			result, err := tool.InvokableRun(context.Background(), string(inputJSON))
			if err != nil {
				t.Fatalf("InvokableRun failed: %v", err)
			}
			
			var output MarkdownFormatOutput
			if err := json.Unmarshal([]byte(result), &output); err != nil {
				t.Fatalf("Failed to unmarshal output: %v", err)
			}
			
			if output.FormattedContent != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, output.FormattedContent)
			}
			
			hasChanges := len(output.Changes) > 0
			if hasChanges != tt.changed {
				t.Errorf("Expected changes=%v, got %v", tt.changed, hasChanges)
			}
		})
	}
}

func TestMarkdownParseTool_CountWords(t *testing.T) {
	tool := NewMarkdownParseTool()
	
	tests := []struct {
		name     string
		input    string
		minWords int
		maxWords int
	}{
		{
			name:     "Chinese only",
			input:    "这是一个测试",
			minWords: 4,
			maxWords: 6,
		},
		{
			name:     "English only",
			input:    "This is a test",
			minWords: 3,
			maxWords: 5,
		},
		{
			name:     "Mixed",
			input:    "这是一个Python测试",
			minWords: 5,
			maxWords: 8,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count := tool.countWords(tt.input)
			if count < tt.minWords || count > tt.maxWords {
				t.Errorf("countWords(%s) = %d, expected between %d and %d", 
					tt.input, count, tt.minWords, tt.maxWords)
			}
		})
	}
}

func TestMarkdownFormatTool_FixPunctuation(t *testing.T) {
	tool := NewMarkdownFormatTool()
	
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "测试。。",
			expected: "测试。",
		},
		{
			input:    "测试，，",
			expected: "测试，",
		},
		{
			input:    "测试 ，继续",
			expected: "测试，继续",
		},
	}
	
	for _, tt := range tests {
		result := tool.fixPunctuation(tt.input)
		if result != tt.expected {
			t.Errorf("fixPunctuation(%s) = %s, expected %s", tt.input, result, tt.expected)
		}
	}
}
