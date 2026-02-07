package vlm

import (
	"testing"

	"github.com/cloudwego/eino/vdocstool/tools/ocr"
)

func TestGetPromptTemplate(t *testing.T) {
	tests := []struct {
		scene    ocr.SceneType
		expected string
	}{
		{ocr.SceneGeneral, "general"},
		{ocr.SceneDocument, "document"},
		{ocr.SceneTable, "table"},
		{ocr.SceneInvoice, "invoice"},
		{ocr.SceneIDCard, "id_card"},
		{ocr.SceneHandwriting, "handwriting"},
		{"unknown", "general"}, // 未知场景返回通用
	}

	for _, tc := range tests {
		template := GetPromptTemplate(tc.scene)
		if template.Name != tc.expected {
			t.Errorf("GetPromptTemplate(%s) = %s, expected %s", tc.scene, template.Name, tc.expected)
		}
		if template.System == "" {
			t.Errorf("GetPromptTemplate(%s) should have system prompt", tc.scene)
		}
		if template.User == "" {
			t.Errorf("GetPromptTemplate(%s) should have user prompt", tc.scene)
		}
	}
}

func TestParseOCRResponse(t *testing.T) {
	text := "Line 1\nLine 2\nLine 3"
	result := ParseOCRResponse(text)

	if !result.Success {
		t.Error("expected success")
	}
	if result.FullText != text {
		t.Errorf("expected %q, got %q", text, result.FullText)
	}
	if len(result.Blocks) != 3 {
		t.Errorf("expected 3 blocks, got %d", len(result.Blocks))
	}
}

func TestParseOCRResponseEmpty(t *testing.T) {
	result := ParseOCRResponse("")
	if !result.Success {
		t.Error("expected success")
	}
	if len(result.Blocks) != 0 {
		t.Errorf("expected 0 blocks for empty text, got %d", len(result.Blocks))
	}
}

func TestParseTableResponse(t *testing.T) {
	text := `
| Header1 | Header2 | Header3 |
|---------|---------|---------|
| Cell1   | Cell2   | Cell3   |
| Cell4   | Cell5   | Cell6   |
`
	result := ParseTableResponse(text)

	if !result.Success {
		t.Error("expected success")
	}
	if len(result.Tables) != 1 {
		t.Fatalf("expected 1 table, got %d", len(result.Tables))
	}

	table := result.Tables[0]
	if len(table.Headers) != 3 {
		t.Errorf("expected 3 headers, got %d", len(table.Headers))
	}
	if len(table.Rows) != 2 {
		t.Errorf("expected 2 rows, got %d", len(table.Rows))
	}
}

func TestParseTableResponseNoTable(t *testing.T) {
	text := "This is just regular text without any table"
	result := ParseTableResponse(text)

	if !result.Success {
		t.Error("expected success")
	}
	if len(result.Tables) != 0 {
		t.Errorf("expected 0 tables, got %d", len(result.Tables))
	}
}

func TestSplitLines(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"a\nb\nc", 3},
		{"single", 1},
		{"", 0},
		{"line1\nline2", 2},
	}

	for _, tc := range tests {
		lines := splitLines(tc.input)
		if len(lines) != tc.expected {
			t.Errorf("splitLines(%q) got %d lines, expected %d", tc.input, len(lines), tc.expected)
		}
	}
}

func TestTrimSpaceVLM(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"  hello  ", "hello"},
		{"\tworld\t", "world"},
		{"no trim", "no trim"},
		{"", ""},
	}

	for _, tc := range tests {
		result := trimSpace(tc.input)
		if result != tc.expected {
			t.Errorf("trimSpace(%q) = %q, expected %q", tc.input, result, tc.expected)
		}
	}
}

func TestParseTableRow(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"| a | b | c |", 3},
		{"| single |", 1},
		{"|col1|col2|", 2},
		{"no pipes", 0},
	}

	for _, tc := range tests {
		cells := parseTableRow(tc.input)
		if len(cells) != tc.expected {
			t.Errorf("parseTableRow(%q) got %d cells, expected %d: %v", tc.input, len(cells), tc.expected, cells)
		}
	}
}

func TestIsSeparatorRow(t *testing.T) {
	tests := []struct {
		cells    []string
		expected bool
	}{
		{[]string{"---", "---", "---"}, true},
		{[]string{":---:", "---:", ":---"}, true},
		{[]string{"data", "data"}, false},
		{[]string{"---", "text"}, false},
	}

	for _, tc := range tests {
		result := isSeparatorRow(tc.cells)
		if result != tc.expected {
			t.Errorf("isSeparatorRow(%v) = %v, expected %v", tc.cells, result, tc.expected)
		}
	}
}

func TestFormatError(t *testing.T) {
	err := FormatError(ocr.EngineQwenVL, nil)
	if err == nil {
		t.Error("expected error")
	}
}

func TestPromptTemplates(t *testing.T) {
	// 测试所有预定义模板
	templates := []PromptTemplate{
		GeneralOCRPrompt,
		DocumentOCRPrompt,
		TableOCRPrompt,
		InvoiceOCRPrompt,
		IDCardOCRPrompt,
		HandwritingOCRPrompt,
	}

	for _, template := range templates {
		if template.Name == "" {
			t.Error("template name should not be empty")
		}
		if template.Description == "" {
			t.Error("template description should not be empty")
		}
		if template.System == "" {
			t.Error("template system prompt should not be empty")
		}
		if template.User == "" {
			t.Error("template user prompt should not be empty")
		}
	}
}
