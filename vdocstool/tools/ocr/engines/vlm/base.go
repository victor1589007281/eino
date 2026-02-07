// Package vlm 视觉语言模型 OCR 引擎
package vlm

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/vdocstool/tools/ocr"
)

// PromptTemplate OCR 提示词模板
type PromptTemplate struct {
	Name        string
	Description string
	System      string
	User        string
}

// 预定义的提示词模板
var (
	// GeneralOCRPrompt 通用 OCR 提示词
	GeneralOCRPrompt = PromptTemplate{
		Name:        "general",
		Description: "通用文字识别",
		System:      "你是一个专业的 OCR 文字识别助手。请仔细识别图片中的所有文字，保持原有的排版格式。",
		User:        "请识别这张图片中的所有文字，按照原有布局输出：",
	}

	// DocumentOCRPrompt 文档 OCR 提示词
	DocumentOCRPrompt = PromptTemplate{
		Name:        "document",
		Description: "文档识别",
		System:      "你是一个专业的文档 OCR 助手。请识别文档中的文字，保持段落结构，正确识别标题、正文、列表等。",
		User:        "请识别这份文档图片中的文字内容，保持原有的段落和层次结构：",
	}

	// TableOCRPrompt 表格 OCR 提示词
	TableOCRPrompt = PromptTemplate{
		Name:        "table",
		Description: "表格识别",
		System: `你是一个专业的表格 OCR 助手。请识别图片中的表格，并以结构化的方式输出。
输出格式要求：
1. 使用 | 分隔列
2. 每行一条记录
3. 第一行是表头`,
		User: "请识别这张图片中的表格内容，以 Markdown 表格格式输出：",
	}

	// InvoiceOCRPrompt 票据 OCR 提示词
	InvoiceOCRPrompt = PromptTemplate{
		Name:        "invoice",
		Description: "票据识别",
		System: `你是一个专业的票据 OCR 助手。请识别发票/收据中的关键信息。
需要提取的字段：
- 发票代码
- 发票号码
- 开票日期
- 金额（大写/小写）
- 税额
- 销售方/购买方信息`,
		User: "请识别这张发票/票据图片，提取关键信息并以 JSON 格式输出：",
	}

	// IDCardOCRPrompt 证件 OCR 提示词
	IDCardOCRPrompt = PromptTemplate{
		Name:        "id_card",
		Description: "证件识别",
		System: `你是一个专业的证件 OCR 助手。请识别身份证/证件中的信息。
注意：出于隐私保护，身份证号码中间部分请用 * 号替代。`,
		User: "请识别这张证件图片中的信息，以 JSON 格式输出（注意隐私保护）：",
	}

	// HandwritingOCRPrompt 手写体 OCR 提示词
	HandwritingOCRPrompt = PromptTemplate{
		Name:        "handwriting",
		Description: "手写体识别",
		System:      "你是一个专业的手写体识别助手。请仔细识别图片中的手写文字，即使字迹潦草也尽量准确识别。如有不确定的字，请用 [?] 标注。",
		User:        "请识别这张图片中的手写文字：",
	}
)

// GetPromptTemplate 根据场景获取提示词模板
func GetPromptTemplate(scene ocr.SceneType) PromptTemplate {
	switch scene {
	case ocr.SceneDocument:
		return DocumentOCRPrompt
	case ocr.SceneTable:
		return TableOCRPrompt
	case ocr.SceneInvoice:
		return InvoiceOCRPrompt
	case ocr.SceneIDCard:
		return IDCardOCRPrompt
	case ocr.SceneHandwriting:
		return HandwritingOCRPrompt
	default:
		return GeneralOCRPrompt
	}
}

// VLMEngine VLM 引擎基础接口
type VLMEngine interface {
	ocr.Engine

	// Chat 发送图文对话
	Chat(ctx context.Context, imageBase64 string, prompt string) (string, error)

	// ChatWithSystem 带系统提示的图文对话
	ChatWithSystem(ctx context.Context, imageBase64, systemPrompt, userPrompt string) (string, error)
}

// BaseVLMEngine VLM 引擎基类
type BaseVLMEngine struct {
	loaded bool
}

// ParseOCRResponse 解析 VLM 返回的 OCR 结果
func ParseOCRResponse(text string) *ocr.OCRResult {
	result := &ocr.OCRResult{
		Success:  true,
		FullText: text,
	}

	// 按行分割作为文本块
	lines := splitLines(text)
	for _, line := range lines {
		if line != "" {
			result.Blocks = append(result.Blocks, ocr.TextBlock{
				Text:      line,
				BlockType: "text",
			})
		}
	}

	return result
}

// ParseTableResponse 解析 VLM 返回的表格结果
func ParseTableResponse(text string) *ocr.TableResult {
	result := &ocr.TableResult{
		OCRResult: ocr.OCRResult{
			Success:  true,
			FullText: text,
		},
	}

	// 尝试解析 Markdown 表格
	lines := splitLines(text)
	var table ocr.Table
	var inTable bool

	for _, line := range lines {
		line = trimSpace(line)
		if line == "" {
			continue
		}

		// 检测表格行
		if len(line) > 0 && line[0] == '|' {
			inTable = true
			// 解析表格行
			cells := parseTableRow(line)
			if len(cells) > 0 {
				// 跳过分隔行 (|---|---|)
				if !isSeparatorRow(cells) {
					table.Rows = append(table.Rows, cells)
				}
			}
		} else if inTable {
			// 表格结束
			break
		}
	}

	// 设置表头
	if len(table.Rows) > 0 {
		table.Headers = table.Rows[0]
		if len(table.Rows) > 1 {
			table.Rows = table.Rows[1:]
		} else {
			table.Rows = nil
		}
	}

	if len(table.Headers) > 0 || len(table.Rows) > 0 {
		result.Tables = append(result.Tables, table)
	}

	return result
}

// 辅助函数

func splitLines(s string) []string {
	var lines []string
	var current string
	for _, c := range s {
		if c == '\n' {
			lines = append(lines, current)
			current = ""
		} else if c != '\r' {
			current += string(c)
		}
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}

func trimSpace(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}

func parseTableRow(line string) []string {
	var cells []string
	var current string
	for i, c := range line {
		if c == '|' {
			if i > 0 { // 跳过开头的 |
				cells = append(cells, trimSpace(current))
			}
			current = ""
		} else {
			current += string(c)
		}
	}
	return cells
}

func isSeparatorRow(cells []string) bool {
	for _, cell := range cells {
		for _, c := range cell {
			if c != '-' && c != ':' && c != ' ' {
				return false
			}
		}
	}
	return true
}

// FormatError 格式化错误
func FormatError(engine ocr.EngineType, err error) error {
	return fmt.Errorf("%s engine error: %w", engine, err)
}
