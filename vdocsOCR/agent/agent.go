package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"

	"github.com/cloudwego/eino/vdocsOCR/config"
	"github.com/cloudwego/eino/vdocsOCR/tools/invoice"
	"github.com/cloudwego/eino/vdocsOCR/tools/ocr"
	"github.com/cloudwego/eino/vdocsOCR/tools/pdf"
)

// OCRAgentImpl implements the OCRAgent interface.
type OCRAgentImpl struct {
	config            *config.Config
	chatModel         model.ToolCallingChatModel
	ocrManager        *ocr.OCRManager
	pdfProcessor      *pdf.PDFProcessor
	invoiceRecognizer *invoice.InvoiceRecognizer
	tools             []tool.BaseTool
	session           *Session
}

// Compile-time check that OCRAgentImpl implements OCRAgent.
var _ OCRAgent = (*OCRAgentImpl)(nil)

// Name returns the agent name.
func (a *OCRAgentImpl) Name() string {
	return "ocr_agent"
}

// Description returns the agent description.
func (a *OCRAgentImpl) Description() string {
	return "OCR Agent for recognizing and extracting information from images, PDFs, and invoices"
}

// Generate generates a response based on the input messages.
func (a *OCRAgentImpl) Generate(ctx context.Context, messages []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	if a.chatModel == nil {
		return nil, fmt.Errorf("LLM not configured")
	}

	// Build system message
	systemMsg := schema.SystemMessage(a.buildSystemPrompt())
	allMessages := append([]*schema.Message{systemMsg}, messages...)

	// Call LLM
	response, err := a.chatModel.Generate(ctx, allMessages, opts...)
	if err != nil {
		return nil, fmt.Errorf("LLM generation failed: %w", err)
	}

	return response, nil
}

// Stream streams a response based on the input messages.
func (a *OCRAgentImpl) Stream(ctx context.Context, messages []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	if a.chatModel == nil {
		return nil, fmt.Errorf("LLM not configured")
	}

	// Build system message
	systemMsg := schema.SystemMessage(a.buildSystemPrompt())
	allMessages := append([]*schema.Message{systemMsg}, messages...)

	// Get streaming interface
	streamModel, ok := a.chatModel.(model.ChatModel)
	if !ok {
		return nil, fmt.Errorf("model does not support streaming")
	}

	return streamModel.Stream(ctx, allMessages, opts...)
}

// buildSystemPrompt builds the system prompt for the agent.
func (a *OCRAgentImpl) buildSystemPrompt() string {
	return `你是一个专业的OCR识别和信息提取助手。

## 能力
1. **图片OCR**: 识别图片中的文字，支持中英文混合
2. **PDF处理**: 提取PDF文档中的文字和图片，支持扫描件OCR
3. **发票识别**: 自动识别各类发票并提取关键信息
4. **结构化提取**: 根据用户定义的模式提取结构化数据

## 工具
- ocr_image: 对图片进行OCR识别
- ocr_pdf: 处理PDF文档
- recognize_invoice: 识别和提取发票信息
- extract_structured: 按模式提取结构化数据

## 输出要求
1. 保持原文格式和结构
2. 准确识别中英文混合文本
3. 正确识别表格、列表等结构化内容
4. 对于识别不确定的内容，用[?]标注
5. 输出结构化的JSON格式结果

## 注意事项
1. 如果图片质量较差，告知用户可能影响识别准确度
2. 对于大型PDF文档，按页分批处理
3. 发票识别时自动检测发票类型
4. 始终验证提取数据的完整性`
}

// ProcessImage processes an image and returns OCR result.
func (a *OCRAgentImpl) ProcessImage(ctx context.Context, imageData []byte, opts *ProcessOptions) (*ProcessResult, error) {
	start := time.Now()

	if opts == nil {
		opts = DefaultProcessOptions()
	}

	result := &ProcessResult{
		TaskType:     TaskTypeImageOCR,
		DocumentType: DocumentTypeImage,
		ProcessedAt:  time.Now(),
		Metadata:     make(map[string]interface{}),
	}

	// Perform OCR
	ocrOpts := &ocr.RecognizeOptions{
		Language:   opts.Language,
		DPI:        opts.DPI,
		Preprocess: opts.Preprocess,
	}

	ocrResult, err := a.ocrManager.Recognize(ctx, imageData, ocrOpts)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("OCR failed: %v", err))
		return result, err
	}

	result.OCRResult = ocrResult
	result.Text = ocrResult.Text

	// If LLM is available, generate summary
	if a.chatModel != nil && opts.OutputFormat != "raw" {
		summary, err := a.generateSummary(ctx, ocrResult.Text, opts)
		if err == nil {
			result.Summary = summary
		}
	}

	result.Duration = time.Since(start)
	result.Metadata["image_size"] = len(imageData)
	result.Metadata["ocr_confidence"] = ocrResult.Confidence

	return result, nil
}

// ProcessPDF processes a PDF and returns OCR result.
func (a *OCRAgentImpl) ProcessPDF(ctx context.Context, pdfData []byte, opts *ProcessOptions) (*ProcessResult, error) {
	start := time.Now()

	if opts == nil {
		opts = DefaultProcessOptions()
	}

	result := &ProcessResult{
		TaskType:     TaskTypePDFOCR,
		DocumentType: DocumentTypePDF,
		ProcessedAt:  time.Now(),
		Metadata:     make(map[string]interface{}),
	}

	// Write PDF to temp file
	tempFile, err := os.CreateTemp("", "ocr-*.pdf")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tempFile.Name())

	if _, err := tempFile.Write(pdfData); err != nil {
		return nil, fmt.Errorf("failed to write temp file: %w", err)
	}
	tempFile.Close()

	// Process PDF
	pdfOpts := &pdf.ProcessOptions{
		PageRange:     opts.PageRange,
		ExtractText:   opts.ExtractText,
		ExtractImages: opts.ExtractImages,
		OCRScannedPDF: true,
		DPI:           opts.DPI,
		Language:      opts.Language,
	}

	pdfResult, err := a.pdfProcessor.ProcessFile(ctx, tempFile.Name(), pdfOpts)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("PDF processing failed: %v", err))
		return result, err
	}

	result.PDFResult = pdfResult
	result.Text = pdfResult.FullText

	// If LLM is available, generate summary
	if a.chatModel != nil && opts.OutputFormat != "raw" {
		summary, err := a.generateSummary(ctx, pdfResult.FullText, opts)
		if err == nil {
			result.Summary = summary
		}
	}

	result.Duration = time.Since(start)
	result.Metadata["pdf_size"] = len(pdfData)
	result.Metadata["page_count"] = pdfResult.PageCount

	return result, nil
}

// ProcessInvoice processes an invoice image and returns structured data.
func (a *OCRAgentImpl) ProcessInvoice(ctx context.Context, imageData []byte, opts *ProcessOptions) (*ProcessResult, error) {
	start := time.Now()

	if opts == nil {
		opts = DefaultProcessOptions()
	}

	result := &ProcessResult{
		TaskType:     TaskTypeInvoiceRecognition,
		DocumentType: DocumentTypeInvoice,
		ProcessedAt:  time.Now(),
		Metadata:     make(map[string]interface{}),
	}

	// Recognize invoice
	invoiceOpts := &invoice.RecognizeOptions{
		Language:     opts.Language,
		ExpectedType: opts.InvoiceType,
		ExtractItems: opts.ExtractItems,
		Validate:     opts.ValidateInvoice,
	}

	invoiceResult, err := a.invoiceRecognizer.Recognize(ctx, imageData, invoiceOpts)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("Invoice recognition failed: %v", err))
		return result, err
	}

	result.InvoiceResult = invoiceResult
	result.Text = invoiceResult.OCRResult.Text

	// Convert invoice to structured data
	result.StructuredData = a.invoiceToStructuredData(invoiceResult.Invoice)

	result.Duration = time.Since(start)
	result.Metadata["invoice_type"] = string(invoiceResult.Invoice.Type)
	result.Metadata["confidence"] = invoiceResult.Invoice.Confidence

	return result, nil
}

// ProcessFile processes a file and automatically detects the type.
func (a *OCRAgentImpl) ProcessFile(ctx context.Context, filePath string, opts *ProcessOptions) (*ProcessResult, error) {
	// Read file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Detect file type
	ext := strings.ToLower(filepath.Ext(filePath))

	switch ext {
	case ".pdf":
		return a.ProcessPDF(ctx, data, opts)
	case ".jpg", ".jpeg", ".png", ".gif", ".bmp", ".tiff", ".webp":
		// Auto-detect if it's an invoice
		if opts != nil && opts.TaskType == TaskTypeInvoiceRecognition {
			return a.ProcessInvoice(ctx, data, opts)
		}
		return a.ProcessImage(ctx, data, opts)
	default:
		return nil, fmt.Errorf("unsupported file type: %s", ext)
	}
}

// GetCapabilities returns the capabilities of the agent.
func (a *OCRAgentImpl) GetCapabilities() []string {
	return []string{
		"image_ocr",
		"pdf_ocr",
		"invoice_recognition",
		"structured_extraction",
		"multi_language_support",
		"table_extraction",
	}
}

// generateSummary generates a summary using LLM.
func (a *OCRAgentImpl) generateSummary(ctx context.Context, text string, opts *ProcessOptions) (string, error) {
	if a.chatModel == nil {
		return "", fmt.Errorf("LLM not available")
	}

	prompt := fmt.Sprintf(`请对以下OCR识别结果进行整理和总结：

%s

要求：
1. 修正明显的识别错误
2. 整理格式使其更易读
3. 提取关键信息
4. 如有表格，保持表格结构`, text)

	messages := []*schema.Message{
		schema.UserMessage(prompt),
	}

	response, err := a.chatModel.Generate(ctx, messages)
	if err != nil {
		return "", err
	}

	return response.Content, nil
}

// invoiceToStructuredData converts invoice to structured data format.
func (a *OCRAgentImpl) invoiceToStructuredData(inv *invoice.Invoice) *StructuredData {
	fields := make(map[string]interface{})
	
	// Copy all fields
	for k, v := range inv.Fields {
		fields[k] = v
	}
	
	// Add type and confidence
	fields["invoice_type"] = string(inv.Type)
	fields["confidence"] = inv.Confidence

	// Convert items to table
	var tables []ExtractedTable
	if len(inv.Items) > 0 {
		headers := []string{"序号", "名称", "规格", "单位", "数量", "单价", "金额", "税率", "税额"}
		rows := make([]map[string]interface{}, 0, len(inv.Items))

		for _, item := range inv.Items {
			rows = append(rows, map[string]interface{}{
				"序号": item.Index,
				"名称": item.Name,
				"规格": item.Spec,
				"单位": item.Unit,
				"数量": item.Quantity,
				"单价": item.UnitPrice,
				"金额": item.Amount,
				"税率": item.TaxRate,
				"税额": item.TaxAmount,
			})
		}

		tables = append(tables, ExtractedTable{
			Name:    "商品明细",
			Headers: headers,
			Rows:    rows,
		})
	}

	return &StructuredData{
		SchemaName: string(inv.Type),
		Fields:     fields,
		Tables:     tables,
		Confidence: inv.Confidence,
	}
}

// ResultToJSON converts the process result to JSON.
func (r *ProcessResult) ToJSON() (string, error) {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ResultToMarkdown converts the process result to Markdown.
func (r *ProcessResult) ToMarkdown() string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# OCR 识别结果\n\n"))
	sb.WriteString(fmt.Sprintf("- **任务类型**: %s\n", r.TaskType))
	sb.WriteString(fmt.Sprintf("- **文档类型**: %s\n", r.DocumentType))
	sb.WriteString(fmt.Sprintf("- **处理时间**: %s\n", r.Duration))
	sb.WriteString(fmt.Sprintf("- **处理时间点**: %s\n\n", r.ProcessedAt.Format(time.RFC3339)))

	// Summary
	if r.Summary != "" {
		sb.WriteString("## 摘要\n\n")
		sb.WriteString(r.Summary)
		sb.WriteString("\n\n")
	}

	// Text content
	if r.Text != "" {
		sb.WriteString("## 识别文本\n\n")
		sb.WriteString("```\n")
		sb.WriteString(r.Text)
		sb.WriteString("\n```\n\n")
	}

	// Structured data
	if r.StructuredData != nil {
		sb.WriteString("## 结构化数据\n\n")
		
		// Fields
		sb.WriteString("### 字段\n\n")
		sb.WriteString("| 字段 | 值 |\n")
		sb.WriteString("|------|----|\n")
		for k, v := range r.StructuredData.Fields {
			sb.WriteString(fmt.Sprintf("| **%s** | %v |\n", k, v))
		}
		sb.WriteString("\n")

		// Tables
		for _, table := range r.StructuredData.Tables {
			sb.WriteString(fmt.Sprintf("### %s\n\n", table.Name))
			
			// Headers
			sb.WriteString("|")
			for _, h := range table.Headers {
				sb.WriteString(fmt.Sprintf(" %s |", h))
			}
			sb.WriteString("\n|")
			for range table.Headers {
				sb.WriteString("------|")
			}
			sb.WriteString("\n")

			// Rows
			for _, row := range table.Rows {
				sb.WriteString("|")
				for _, h := range table.Headers {
					sb.WriteString(fmt.Sprintf(" %v |", row[h]))
				}
				sb.WriteString("\n")
			}
			sb.WriteString("\n")
		}
	}

	// Errors
	if len(r.Errors) > 0 {
		sb.WriteString("## 错误\n\n")
		for _, err := range r.Errors {
			sb.WriteString(fmt.Sprintf("- %s\n", err))
		}
		sb.WriteString("\n")
	}

	// Warnings
	if len(r.Warnings) > 0 {
		sb.WriteString("## 警告\n\n")
		for _, warn := range r.Warnings {
			sb.WriteString(fmt.Sprintf("- %s\n", warn))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}
