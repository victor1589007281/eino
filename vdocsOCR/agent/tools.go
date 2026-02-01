package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"

	"github.com/cloudwego/eino/vdocsOCR/tools/invoice"
	"github.com/cloudwego/eino/vdocsOCR/tools/ocr"
	"github.com/cloudwego/eino/vdocsOCR/tools/pdf"
)

// OCRTool implements the OCR tool.
type OCRTool struct {
	ocrManager *ocr.OCRManager
}

// NewOCRTool creates a new OCR tool.
func NewOCRTool(manager *ocr.OCRManager) *OCRTool {
	return &OCRTool{ocrManager: manager}
}

// Info returns the tool information.
func (t *OCRTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name:        "ocr_image",
		Description: "对图片进行OCR识别，提取其中的文字内容。支持中英文混合识别。",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"image_base64": map[string]interface{}{
					"type":        "string",
					"description": "Base64编码的图片数据",
				},
				"image_path": map[string]interface{}{
					"type":        "string",
					"description": "图片文件路径（与image_base64二选一）",
				},
				"language": map[string]interface{}{
					"type":        "string",
					"description": "OCR语言，默认为chi_sim+eng（简体中文+英文）",
					"default":     "chi_sim+eng",
				},
			},
			"required": []string{},
		},
	}, nil
}

// Run executes the OCR tool.
func (t *OCRTool) Run(ctx context.Context, input string) (string, error) {
	var params struct {
		ImageBase64 string `json:"image_base64"`
		ImagePath   string `json:"image_path"`
		Language    string `json:"language"`
	}

	if err := json.Unmarshal([]byte(input), &params); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}

	var imageData []byte
	var err error

	if params.ImageBase64 != "" {
		imageData, err = ocr.ReadImageFromBase64(params.ImageBase64)
		if err != nil {
			return "", fmt.Errorf("invalid base64 data: %w", err)
		}
	} else if params.ImagePath != "" {
		result, err := t.ocrManager.RecognizeFile(ctx, params.ImagePath, &ocr.RecognizeOptions{
			Language: params.Language,
		})
		if err != nil {
			return "", fmt.Errorf("OCR failed: %w", err)
		}
		return ocr.OCRResultToJSON(result)
	} else {
		return "", fmt.Errorf("either image_base64 or image_path is required")
	}

	result, err := t.ocrManager.Recognize(ctx, imageData, &ocr.RecognizeOptions{
		Language: params.Language,
	})
	if err != nil {
		return "", fmt.Errorf("OCR failed: %w", err)
	}

	return ocr.OCRResultToJSON(result)
}

// Compile-time check
var _ tool.BaseTool = (*OCRTool)(nil)

// PDFTool implements the PDF processing tool.
type PDFTool struct {
	pdfProcessor *pdf.PDFProcessor
}

// NewPDFTool creates a new PDF tool.
func NewPDFTool(processor *pdf.PDFProcessor) *PDFTool {
	return &PDFTool{pdfProcessor: processor}
}

// Info returns the tool information.
func (t *PDFTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name:        "ocr_pdf",
		Description: "处理PDF文档，提取文字和图片内容。支持扫描件PDF的OCR识别。",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"pdf_path": map[string]interface{}{
					"type":        "string",
					"description": "PDF文件路径",
				},
				"page_range": map[string]interface{}{
					"type":        "string",
					"description": "页面范围，如'1-5'、'1,3,5'或'all'",
					"default":     "all",
				},
				"extract_images": map[string]interface{}{
					"type":        "boolean",
					"description": "是否提取图片",
					"default":     true,
				},
				"ocr_scanned": map[string]interface{}{
					"type":        "boolean",
					"description": "是否对扫描件进行OCR",
					"default":     true,
				},
				"language": map[string]interface{}{
					"type":        "string",
					"description": "OCR语言",
					"default":     "chi_sim+eng",
				},
			},
			"required": []string{"pdf_path"},
		},
	}, nil
}

// Run executes the PDF tool.
func (t *PDFTool) Run(ctx context.Context, input string) (string, error) {
	var params struct {
		PDFPath       string `json:"pdf_path"`
		PageRange     string `json:"page_range"`
		ExtractImages bool   `json:"extract_images"`
		OCRScanned    bool   `json:"ocr_scanned"`
		Language      string `json:"language"`
	}

	// Set defaults
	params.PageRange = "all"
	params.ExtractImages = true
	params.OCRScanned = true
	params.Language = "chi_sim+eng"

	if err := json.Unmarshal([]byte(input), &params); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}

	if params.PDFPath == "" {
		return "", fmt.Errorf("pdf_path is required")
	}

	opts := &pdf.ProcessOptions{
		PageRange:     params.PageRange,
		ExtractText:   true,
		ExtractImages: params.ExtractImages,
		OCRScannedPDF: params.OCRScanned,
		Language:      params.Language,
	}

	result, err := t.pdfProcessor.ProcessFile(ctx, params.PDFPath, opts)
	if err != nil {
		return "", fmt.Errorf("PDF processing failed: %w", err)
	}

	return pdf.PDFResultToJSON(result)
}

// Compile-time check
var _ tool.BaseTool = (*PDFTool)(nil)

// InvoiceTool implements the invoice recognition tool.
type InvoiceTool struct {
	recognizer *invoice.InvoiceRecognizer
}

// NewInvoiceTool creates a new invoice tool.
func NewInvoiceTool(recognizer *invoice.InvoiceRecognizer) *InvoiceTool {
	return &InvoiceTool{recognizer: recognizer}
}

// Info returns the tool information.
func (t *InvoiceTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name:        "recognize_invoice",
		Description: "识别发票图片并提取关键信息。支持增值税发票、火车票、出租车票、机票等多种类型。",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"image_base64": map[string]interface{}{
					"type":        "string",
					"description": "Base64编码的发票图片",
				},
				"image_path": map[string]interface{}{
					"type":        "string",
					"description": "发票图片路径（与image_base64二选一）",
				},
				"invoice_type": map[string]interface{}{
					"type":        "string",
					"description": "预期的发票类型，如不指定则自动检测",
					"enum": []string{
						"vat_invoice",
						"vat_special_invoice",
						"receipt",
						"taxi_receipt",
						"train_ticket",
						"air_ticket",
						"hotel_invoice",
						"toll_invoice",
					},
				},
				"extract_items": map[string]interface{}{
					"type":        "boolean",
					"description": "是否提取商品明细",
					"default":     true,
				},
				"language": map[string]interface{}{
					"type":        "string",
					"description": "OCR语言",
					"default":     "chi_sim+eng",
				},
			},
			"required": []string{},
		},
	}, nil
}

// Run executes the invoice tool.
func (t *InvoiceTool) Run(ctx context.Context, input string) (string, error) {
	var params struct {
		ImageBase64  string `json:"image_base64"`
		ImagePath    string `json:"image_path"`
		InvoiceType  string `json:"invoice_type"`
		ExtractItems bool   `json:"extract_items"`
		Language     string `json:"language"`
	}

	// Set defaults
	params.ExtractItems = true
	params.Language = "chi_sim+eng"

	if err := json.Unmarshal([]byte(input), &params); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}

	var imageData []byte
	var err error

	if params.ImageBase64 != "" {
		imageData, err = ocr.ReadImageFromBase64(params.ImageBase64)
		if err != nil {
			return "", fmt.Errorf("invalid base64 data: %w", err)
		}
	} else if params.ImagePath != "" {
		imageData, err = readFileData(params.ImagePath)
		if err != nil {
			return "", fmt.Errorf("failed to read image: %w", err)
		}
	} else {
		return "", fmt.Errorf("either image_base64 or image_path is required")
	}

	opts := &invoice.RecognizeOptions{
		Language:     params.Language,
		ExpectedType: invoice.InvoiceType(params.InvoiceType),
		ExtractItems: params.ExtractItems,
	}

	result, err := t.recognizer.Recognize(ctx, imageData, opts)
	if err != nil {
		return "", fmt.Errorf("invoice recognition failed: %w", err)
	}

	return invoice.ResultToJSON(result)
}

// Compile-time check
var _ tool.BaseTool = (*InvoiceTool)(nil)

// ExtractionTool implements the structured extraction tool.
type ExtractionTool struct{}

// NewExtractionTool creates a new extraction tool.
func NewExtractionTool() *ExtractionTool {
	return &ExtractionTool{}
}

// Info returns the tool information.
func (t *ExtractionTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name:        "extract_structured",
		Description: "从OCR文本中按照指定的模式提取结构化数据。",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"text": map[string]interface{}{
					"type":        "string",
					"description": "要提取的文本内容",
				},
				"schema": map[string]interface{}{
					"type":        "object",
					"description": "提取模式定义",
					"properties": map[string]interface{}{
						"name": map[string]interface{}{
							"type":        "string",
							"description": "模式名称",
						},
						"fields": map[string]interface{}{
							"type":        "array",
							"description": "要提取的字段列表",
							"items": map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"name": map[string]interface{}{
										"type": "string",
									},
									"type": map[string]interface{}{
										"type": "string",
									},
									"description": map[string]interface{}{
										"type": "string",
									},
									"required": map[string]interface{}{
										"type": "boolean",
									},
								},
							},
						},
					},
				},
			},
			"required": []string{"text", "schema"},
		},
	}, nil
}

// Run executes the extraction tool.
func (t *ExtractionTool) Run(ctx context.Context, input string) (string, error) {
	var params struct {
		Text   string            `json:"text"`
		Schema *ExtractionSchema `json:"schema"`
	}

	if err := json.Unmarshal([]byte(input), &params); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}

	if params.Text == "" {
		return "", fmt.Errorf("text is required")
	}
	if params.Schema == nil {
		return "", fmt.Errorf("schema is required")
	}

	// This is a basic implementation
	// In production, this would use LLM for intelligent extraction
	result := &StructuredData{
		SchemaName: params.Schema.Name,
		Fields:     make(map[string]interface{}),
		Confidence: 0.5, // Placeholder
	}

	// Initialize fields
	for _, field := range params.Schema.Fields {
		result.Fields[field.Name] = nil
	}

	output, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", err
	}

	return string(output), nil
}

// Compile-time check
var _ tool.BaseTool = (*ExtractionTool)(nil)

// Helper function to read file data
func readFileData(path string) ([]byte, error) {
	return ocr.ReadImageFromReader(nil) // Placeholder, actual implementation would read file
}
