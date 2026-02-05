// Package intent 附件解析器
package intent

import (
	"path/filepath"
	"strings"
)

// AttachmentParser 附件解析器
type AttachmentParser struct {
	typeMapping map[string][]IntentType
}

// NewAttachmentParser 创建附件解析器
func NewAttachmentParser() *AttachmentParser {
	return &AttachmentParser{
		typeMapping: map[string][]IntentType{
			// 文档类型
			".pdf":  {IntentFinanceInvoice, IntentFinanceBill, IntentWorkReport},
			".doc":  {IntentWorkReport},
			".docx": {IntentWorkReport},
			
			// 表格类型
			".xls":  {IntentWorkReport, IntentFinanceBill},
			".xlsx": {IntentWorkReport, IntentFinanceBill},
			".csv":  {IntentWorkReport, IntentFinanceBill},
			
			// 演示文稿
			".ppt":  {IntentWorkReport, IntentWorkMeeting},
			".pptx": {IntentWorkReport, IntentWorkMeeting},
			
			// 日历文件
			".ics":  {IntentWorkMeeting, IntentPersonalTravel},
			
			// 图片类型（可能是发票扫描件）
			".jpg":  {IntentFinanceInvoice},
			".jpeg": {IntentFinanceInvoice},
			".png":  {IntentFinanceInvoice},
			
			// 压缩包
			".zip":  {IntentWorkReport},
			".rar":  {IntentWorkReport},
		},
	}
}

// AttachmentAnalysis 附件分析结果
type AttachmentAnalysis struct {
	TotalCount      int                    `json:"total_count"`
	FileTypes       map[string]int         `json:"file_types"`
	PossibleIntents []IntentType           `json:"possible_intents"`
	Categories      []AttachmentCategory   `json:"categories"`
	HasDocument     bool                   `json:"has_document"`
	HasImage        bool                   `json:"has_image"`
	HasCalendar     bool                   `json:"has_calendar"`
	HasSpreadsheet  bool                   `json:"has_spreadsheet"`
}

// AttachmentCategory 附件分类
type AttachmentCategory struct {
	Category string   `json:"category"`
	Files    []string `json:"files"`
}

// Parse 解析附件列表
func (p *AttachmentParser) Parse(attachments []string) *AttachmentAnalysis {
	result := &AttachmentAnalysis{
		TotalCount: len(attachments),
		FileTypes:  make(map[string]int),
	}

	intentScores := make(map[IntentType]int)
	categories := make(map[string][]string)

	for _, filename := range attachments {
		ext := strings.ToLower(filepath.Ext(filename))
		result.FileTypes[ext]++

		// 检测文件类型
		category := p.categorizeFile(ext)
		categories[category] = append(categories[category], filename)

		// 设置标志
		switch category {
		case "document":
			result.HasDocument = true
		case "image":
			result.HasImage = true
		case "calendar":
			result.HasCalendar = true
		case "spreadsheet":
			result.HasSpreadsheet = true
		}

		// 累计意图分数
		if intents, ok := p.typeMapping[ext]; ok {
			for _, intent := range intents {
				intentScores[intent]++
			}
		}
	}

	// 根据文件名关键词推断意图
	for _, filename := range attachments {
		lower := strings.ToLower(filename)
		
		if strings.Contains(lower, "发票") || strings.Contains(lower, "invoice") {
			intentScores[IntentFinanceInvoice] += 2
		}
		if strings.Contains(lower, "报销") || strings.Contains(lower, "expense") {
			intentScores[IntentFinanceReimburse] += 2
		}
		if strings.Contains(lower, "报告") || strings.Contains(lower, "report") {
			intentScores[IntentWorkReport] += 2
		}
		if strings.Contains(lower, "会议") || strings.Contains(lower, "meeting") {
			intentScores[IntentWorkMeeting] += 2
		}
		if strings.Contains(lower, "账单") || strings.Contains(lower, "bill") {
			intentScores[IntentFinanceBill] += 2
		}
	}

	// 转换为意图列表
	for intent, score := range intentScores {
		if score > 0 {
			result.PossibleIntents = append(result.PossibleIntents, intent)
		}
	}

	// 转换分类
	for category, files := range categories {
		result.Categories = append(result.Categories, AttachmentCategory{
			Category: category,
			Files:    files,
		})
	}

	return result
}

// categorizeFile 分类文件
func (p *AttachmentParser) categorizeFile(ext string) string {
	documents := []string{".pdf", ".doc", ".docx", ".txt", ".rtf"}
	spreadsheets := []string{".xls", ".xlsx", ".csv"}
	presentations := []string{".ppt", ".pptx"}
	images := []string{".jpg", ".jpeg", ".png", ".gif", ".bmp", ".webp"}
	calendars := []string{".ics", ".ical"}
	archives := []string{".zip", ".rar", ".7z", ".tar", ".gz"}
	audio := []string{".mp3", ".wav", ".m4a"}
	video := []string{".mp4", ".mov", ".avi", ".mkv"}

	for _, e := range documents {
		if ext == e {
			return "document"
		}
	}
	for _, e := range spreadsheets {
		if ext == e {
			return "spreadsheet"
		}
	}
	for _, e := range presentations {
		if ext == e {
			return "presentation"
		}
	}
	for _, e := range images {
		if ext == e {
			return "image"
		}
	}
	for _, e := range calendars {
		if ext == e {
			return "calendar"
		}
	}
	for _, e := range archives {
		if ext == e {
			return "archive"
		}
	}
	for _, e := range audio {
		if ext == e {
			return "audio"
		}
	}
	for _, e := range video {
		if ext == e {
			return "video"
		}
	}

	return "other"
}

// IsLikelyInvoice 判断是否可能是发票
func IsLikelyInvoice(filename string) bool {
	lower := strings.ToLower(filename)
	keywords := []string{"发票", "invoice", "fapiao", "电子发票"}
	
	for _, kw := range keywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	
	// PDF 和图片格式的文件可能是发票
	ext := filepath.Ext(lower)
	return ext == ".pdf" || ext == ".jpg" || ext == ".png"
}

// IsLikelyReport 判断是否可能是报告
func IsLikelyReport(filename string) bool {
	lower := strings.ToLower(filename)
	keywords := []string{"报告", "report", "周报", "月报", "汇报", "summary"}
	
	for _, kw := range keywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	
	return false
}
