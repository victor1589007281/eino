// Package agent provides the core agent implementations for OCR Agent.
package agent

import (
	"context"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
	
	"github.com/cloudwego/eino/vdocsOCR/tools/invoice"
	"github.com/cloudwego/eino/vdocsOCR/tools/ocr"
	"github.com/cloudwego/eino/vdocsOCR/tools/pdf"
)

// TaskType represents the type of OCR task.
type TaskType string

const (
	// TaskTypeImageOCR represents image OCR task.
	TaskTypeImageOCR TaskType = "image_ocr"
	// TaskTypePDFOCR represents PDF OCR task.
	TaskTypePDFOCR TaskType = "pdf_ocr"
	// TaskTypeInvoiceRecognition represents invoice recognition task.
	TaskTypeInvoiceRecognition TaskType = "invoice_recognition"
	// TaskTypeStructuredExtraction represents structured data extraction task.
	TaskTypeStructuredExtraction TaskType = "structured_extraction"
	// TaskTypeBatchOCR represents batch OCR task.
	TaskTypeBatchOCR TaskType = "batch_ocr"
	// TaskTypeUnknown represents unknown task type.
	TaskTypeUnknown TaskType = "unknown"
)

// String returns the string representation of TaskType.
func (t TaskType) String() string {
	return string(t)
}

// DocumentType represents the type of document.
type DocumentType string

const (
	DocumentTypeImage   DocumentType = "image"
	DocumentTypePDF     DocumentType = "pdf"
	DocumentTypeInvoice DocumentType = "invoice"
	DocumentTypeUnknown DocumentType = "unknown"
)

// OCRAgent defines the interface for OCR agents.
type OCRAgent interface {
	adk.Agent

	// ProcessImage processes an image and returns OCR result.
	ProcessImage(ctx context.Context, imageData []byte, opts *ProcessOptions) (*ProcessResult, error)

	// ProcessPDF processes a PDF and returns OCR result.
	ProcessPDF(ctx context.Context, pdfData []byte, opts *ProcessOptions) (*ProcessResult, error)

	// ProcessInvoice processes an invoice image and returns structured data.
	ProcessInvoice(ctx context.Context, imageData []byte, opts *ProcessOptions) (*ProcessResult, error)

	// ProcessFile processes a file and automatically detects the type.
	ProcessFile(ctx context.Context, filePath string, opts *ProcessOptions) (*ProcessResult, error)

	// GetCapabilities returns the capabilities of the agent.
	GetCapabilities() []string
}

// ProcessOptions represents options for document processing.
type ProcessOptions struct {
	// Task type
	TaskType TaskType `json:"task_type"`
	
	// Document type hint
	DocumentType DocumentType `json:"document_type"`
	
	// OCR options
	Language     string `json:"language"`
	DPI          int    `json:"dpi"`
	Preprocess   bool   `json:"preprocess"`
	
	// PDF options
	PageRange    string `json:"page_range"`
	ExtractText  bool   `json:"extract_text"`
	ExtractImages bool  `json:"extract_images"`
	
	// Invoice options
	InvoiceType   invoice.InvoiceType `json:"invoice_type"`
	ExtractItems  bool                `json:"extract_items"`
	ValidateInvoice bool              `json:"validate_invoice"`
	
	// Output options
	OutputFormat  string `json:"output_format"`  // json, markdown, text
	IncludeRaw    bool   `json:"include_raw"`
	IncludeImages bool   `json:"include_images"`
	
	// Extraction schema (for structured extraction)
	Schema        *ExtractionSchema `json:"schema,omitempty"`
	
	// Timeout
	Timeout time.Duration `json:"timeout"`
}

// DefaultProcessOptions returns default processing options.
func DefaultProcessOptions() *ProcessOptions {
	return &ProcessOptions{
		TaskType:      TaskTypeImageOCR,
		Language:      "chi_sim+eng",
		DPI:           300,
		Preprocess:    true,
		PageRange:     "all",
		ExtractText:   true,
		ExtractImages: true,
		ExtractItems:  true,
		OutputFormat:  "json",
		IncludeRaw:    false,
		Timeout:       120 * time.Second,
	}
}

// ExtractionSchema defines the schema for structured data extraction.
type ExtractionSchema struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Fields      []ExtractionField      `json:"fields"`
	Tables      []TableSchema          `json:"tables,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// ExtractionField defines a field to extract.
type ExtractionField struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"`        // string, number, date, boolean, array
	Description string   `json:"description"`
	Required    bool     `json:"required"`
	Pattern     string   `json:"pattern,omitempty"`   // regex pattern
	Examples    []string `json:"examples,omitempty"`
	Aliases     []string `json:"aliases,omitempty"`   // alternative names
}

// TableSchema defines a table to extract.
type TableSchema struct {
	Name    string            `json:"name"`
	Columns []ExtractionField `json:"columns"`
}

// ProcessResult represents the result of document processing.
type ProcessResult struct {
	// Task info
	TaskType     TaskType     `json:"task_type"`
	DocumentType DocumentType `json:"document_type"`
	
	// OCR result
	OCRResult    *ocr.OCRResult        `json:"ocr_result,omitempty"`
	
	// PDF result
	PDFResult    *pdf.PDFResult        `json:"pdf_result,omitempty"`
	
	// Invoice result
	InvoiceResult *invoice.RecognizeResult `json:"invoice_result,omitempty"`
	
	// Structured data
	StructuredData *StructuredData `json:"structured_data,omitempty"`
	
	// Full text content
	Text         string `json:"text"`
	
	// Summary generated by LLM
	Summary      string `json:"summary,omitempty"`
	
	// Errors and warnings
	Errors       []string `json:"errors,omitempty"`
	Warnings     []string `json:"warnings,omitempty"`
	
	// Processing metadata
	Duration     time.Duration          `json:"duration"`
	ProcessedAt  time.Time              `json:"processed_at"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// StructuredData represents extracted structured data.
type StructuredData struct {
	SchemaName  string                 `json:"schema_name"`
	Fields      map[string]interface{} `json:"fields"`
	Tables      []ExtractedTable       `json:"tables,omitempty"`
	Confidence  float64                `json:"confidence"`
}

// ExtractedTable represents an extracted table.
type ExtractedTable struct {
	Name    string                   `json:"name"`
	Headers []string                 `json:"headers"`
	Rows    []map[string]interface{} `json:"rows"`
}

// AgentMessage represents a message in agent communication.
type AgentMessage struct {
	Role    string                 `json:"role"`
	Content string                 `json:"content"`
	Tools   []*schema.ToolInfo     `json:"tools,omitempty"`
	Extra   map[string]interface{} `json:"extra,omitempty"`
}

// AgentResponse represents a response from an agent.
type AgentResponse struct {
	Content       string                 `json:"content"`
	ProcessResult *ProcessResult         `json:"process_result,omitempty"`
	ToolCalls     []ToolCall             `json:"tool_calls,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// ToolCall represents a tool call.
type ToolCall struct {
	ID       string                 `json:"id"`
	Name     string                 `json:"name"`
	Args     map[string]interface{} `json:"args"`
	Result   string                 `json:"result,omitempty"`
}

// Session represents an agent session.
type Session struct {
	ID          string        `json:"id"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
	Messages    []AgentMessage `json:"messages"`
	Results     []ProcessResult `json:"results"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// NewSession creates a new session.
func NewSession() *Session {
	return &Session{
		ID:        generateSessionID(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Messages:  make([]AgentMessage, 0),
		Results:   make([]ProcessResult, 0),
		Metadata:  make(map[string]interface{}),
	}
}

// AddMessage adds a message to the session.
func (s *Session) AddMessage(msg AgentMessage) {
	s.Messages = append(s.Messages, msg)
	s.UpdatedAt = time.Now()
}

// AddResult adds a result to the session.
func (s *Session) AddResult(result ProcessResult) {
	s.Results = append(s.Results, result)
	s.UpdatedAt = time.Now()
}

// generateSessionID generates a unique session ID.
func generateSessionID() string {
	return time.Now().Format("20060102150405") + "-" + randomString(8)
}

// randomString generates a random string of given length.
func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
		time.Sleep(time.Nanosecond)
	}
	return string(b)
}

// Instruction templates for different tasks
var (
	// ImageOCRInstruction is the instruction for image OCR task.
	ImageOCRInstruction = `你是一个专业的OCR识别助手。请仔细分析提供的图片，提取其中的所有文字内容。

要求：
1. 保持原文格式和结构
2. 准确识别中英文混合文本
3. 正确识别表格、列表等结构化内容
4. 如果有识别不确定的内容，用[?]标注
5. 输出结构化的JSON格式结果`

	// PDFOCRInstruction is the instruction for PDF OCR task.
	PDFOCRInstruction = `你是一个专业的PDF文档处理助手。请仔细分析提供的PDF文档，提取其中的所有内容。

要求：
1. 逐页处理并保持页面结构
2. 提取文字、表格、图片中的文字
3. 保持文档的逻辑结构
4. 对于扫描件PDF，进行OCR识别
5. 输出结构化的JSON格式结果`

	// InvoiceRecognitionInstruction is the instruction for invoice recognition task.
	InvoiceRecognitionInstruction = `你是一个专业的发票识别助手。请仔细分析提供的发票图片，提取关键信息。

要求：
1. 自动识别发票类型（增值税发票、火车票、出租车发票等）
2. 提取发票代码、发票号码、开票日期、金额等关键字段
3. 对于增值税发票，提取购买方、销售方信息和商品明细
4. 验证数据的完整性和一致性
5. 输出结构化的JSON格式结果`

	// StructuredExtractionInstruction is the instruction for structured extraction task.
	StructuredExtractionInstruction = `你是一个专业的信息提取助手。请根据提供的模式定义，从文档中提取结构化数据。

要求：
1. 严格按照提供的字段定义提取数据
2. 处理字段的多种可能表述（别名）
3. 验证数据类型和格式
4. 对于必填字段，如未找到则标注为空
5. 输出符合模式定义的JSON格式结果`
)

// GetInstructionForTask returns the instruction for a specific task type.
func GetInstructionForTask(taskType TaskType) string {
	switch taskType {
	case TaskTypeImageOCR:
		return ImageOCRInstruction
	case TaskTypePDFOCR:
		return PDFOCRInstruction
	case TaskTypeInvoiceRecognition:
		return InvoiceRecognitionInstruction
	case TaskTypeStructuredExtraction:
		return StructuredExtractionInstruction
	default:
		return ImageOCRInstruction
	}
}
