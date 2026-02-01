// +build integration

package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cloudwego/eino/vdocsOCR/agent"
	"github.com/cloudwego/eino/vdocsOCR/config"
	"github.com/cloudwego/eino/vdocsOCR/output"
	"github.com/cloudwego/eino/vdocsOCR/tools/invoice"
	"github.com/cloudwego/eino/vdocsOCR/tools/ocr"
	"github.com/cloudwego/eino/vdocsOCR/tools/pdf"
)

// TestIntegration_FullPipeline tests the full OCR pipeline.
func TestIntegration_FullPipeline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Load config
	cfg := config.DefaultConfig()
	cfg.Storage.DataDir = t.TempDir()
	cfg.Output.OutputDir = t.TempDir()

	// Skip if tesseract not available
	ocrMgr, err := ocr.NewOCRManager(&cfg.OCR)
	if err != nil {
		t.Skipf("OCR manager not available: %v", err)
	}
	defer ocrMgr.Close()

	// Create PDF processor
	pdfProc, err := pdf.NewPDFProcessor(&cfg.OCR.PDFConfig, ocrMgr)
	if err != nil {
		t.Skipf("PDF processor not available: %v", err)
	}
	defer pdfProc.Close()

	// Create invoice recognizer
	invoiceRec := invoice.NewInvoiceRecognizer(&cfg.OCR.InvoiceConfig, ocrMgr)

	// Test with a simple text image if available
	testImagePath := os.Getenv("TEST_IMAGE_PATH")
	if testImagePath == "" {
		t.Skip("TEST_IMAGE_PATH not set, skipping integration test")
	}

	// Test OCR
	result, err := ocrMgr.RecognizeFile(ctx, testImagePath, &ocr.RecognizeOptions{
		Language: "chi_sim+eng",
	})
	if err != nil {
		t.Fatalf("OCR failed: %v", err)
	}

	if result.Text == "" {
		t.Error("OCR result should not be empty")
	}

	t.Logf("OCR Result: %s", result.Text[:min(100, len(result.Text))])

	// Test invoice recognition
	imageData, err := os.ReadFile(testImagePath)
	if err != nil {
		t.Fatalf("Failed to read test image: %v", err)
	}

	invoiceResult, err := invoiceRec.Recognize(ctx, imageData, &invoice.RecognizeOptions{
		Language:     "chi_sim+eng",
		ExtractItems: true,
	})
	if err != nil {
		t.Logf("Invoice recognition failed (may be expected): %v", err)
	} else {
		t.Logf("Invoice Type: %s, Confidence: %.2f", invoiceResult.Invoice.Type, invoiceResult.Invoice.Confidence)
	}
}

// TestIntegration_AgentFactory tests the agent factory.
func TestIntegration_AgentFactory(t *testing.T) {
	ctx := context.Background()

	cfg := config.DefaultConfig()
	cfg.Storage.DataDir = t.TempDir()
	cfg.Output.OutputDir = t.TempDir()

	// Note: LLM won't be available in tests, but OCR-only mode should work
	factory, err := agent.NewAgentFactory(cfg)
	if err != nil {
		t.Skipf("Agent factory not available (OCR tools may not be installed): %v", err)
	}
	defer factory.Close()

	// Create OCR agent
	ocrAgent, err := factory.CreateOCRAgent(ctx)
	if err != nil {
		t.Fatalf("Failed to create OCR agent: %v", err)
	}

	// Verify agent capabilities
	capabilities := ocrAgent.GetCapabilities()
	if len(capabilities) == 0 {
		t.Error("Agent should have capabilities")
	}

	expected := []string{"image_ocr", "pdf_ocr", "invoice_recognition"}
	for _, cap := range expected {
		found := false
		for _, c := range capabilities {
			if c == cap {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Missing capability: %s", cap)
		}
	}
}

// TestIntegration_OutputFormatting tests output formatting.
func TestIntegration_OutputFormatting(t *testing.T) {
	outputDir := t.TempDir()

	formatter, err := output.NewFormatter(outputDir, true, false)
	if err != nil {
		t.Fatalf("Failed to create formatter: %v", err)
	}

	// Create a sample result
	result := &agent.ProcessResult{
		TaskType:     agent.TaskTypeImageOCR,
		DocumentType: agent.DocumentTypeImage,
		Text:         "这是一段测试文本\nThis is test text",
		Summary:      "测试摘要",
		Duration:     100 * time.Millisecond,
		ProcessedAt:  time.Now(),
		StructuredData: &agent.StructuredData{
			SchemaName: "test",
			Fields: map[string]interface{}{
				"field1": "value1",
				"field2": 123,
			},
			Confidence: 0.95,
		},
	}

	// Test all formats
	formats := []output.OutputFormat{
		output.FormatJSON,
		output.FormatMarkdown,
		output.FormatHTML,
		output.FormatText,
		output.FormatCSV,
	}

	for _, format := range formats {
		t.Run(string(format), func(t *testing.T) {
			content, err := formatter.Format(result, format)
			if err != nil {
				t.Fatalf("Format %s failed: %v", format, err)
			}

			if content == "" {
				t.Errorf("Format %s produced empty output", format)
			}

			// Save to file
			err = formatter.FormatAndSave(result, format, "test_output")
			if err != nil {
				t.Errorf("FormatAndSave %s failed: %v", format, err)
			}

			// Verify file exists
			ext := "." + string(format)
			filePath := filepath.Join(outputDir, "test_output"+ext)
			if _, err := os.Stat(filePath); os.IsNotExist(err) {
				t.Errorf("Output file not created: %s", filePath)
			}
		})
	}
}

// TestIntegration_ConfigLoading tests configuration loading.
func TestIntegration_ConfigLoading(t *testing.T) {
	// Create temp config file
	configContent := `
server:
  host: "127.0.0.1"
  port: 9090

llm:
  default_provider: "deepseek"
  providers:
    deepseek:
      type: "deepseek"
      enabled: true
      model: "deepseek-chat"

ocr:
  engine: "tesseract"
  language: "chi_sim+eng"
  max_concurrency: 2

storage:
  type: "sqlite"
  data_dir: "./test-data"

cache:
  enabled: true
  type: "memory"
`

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify loaded values
	if cfg.Server.Port != 9090 {
		t.Errorf("Expected port 9090, got %d", cfg.Server.Port)
	}
	if cfg.OCR.Language != "chi_sim+eng" {
		t.Errorf("Expected language chi_sim+eng, got %s", cfg.OCR.Language)
	}
	if cfg.OCR.MaxConcurrency != 2 {
		t.Errorf("Expected max concurrency 2, got %d", cfg.OCR.MaxConcurrency)
	}
}

// TestIntegration_EnvironmentOverrides tests environment variable overrides.
func TestIntegration_EnvironmentOverrides(t *testing.T) {
	// Set environment variables
	os.Setenv("DEEPSEEK_API_KEY", "test-key-12345")
	os.Setenv("PORT", "8888")
	os.Setenv("LOG_LEVEL", "debug")
	os.Setenv("OUTPUT_DIR", t.TempDir())
	defer func() {
		os.Unsetenv("DEEPSEEK_API_KEY")
		os.Unsetenv("PORT")
		os.Unsetenv("LOG_LEVEL")
		os.Unsetenv("OUTPUT_DIR")
	}()

	cfg, err := config.LoadConfigFromEnv()
	if err != nil {
		t.Fatalf("Failed to load config from env: %v", err)
	}

	if cfg.Server.Port != 8888 {
		t.Errorf("Expected port 8888, got %d", cfg.Server.Port)
	}
	if cfg.Logging.Level != "debug" {
		t.Errorf("Expected log level debug, got %s", cfg.Logging.Level)
	}
	if cfg.LLM.Providers["deepseek"].APIKey != "test-key-12345" {
		t.Error("API key should be set from environment")
	}
}

// TestIntegration_Session tests session management.
func TestIntegration_Session(t *testing.T) {
	session := agent.NewSession()

	if session.ID == "" {
		t.Error("Session should have an ID")
	}
	if session.CreatedAt.IsZero() {
		t.Error("Session should have creation time")
	}
	if len(session.Messages) != 0 {
		t.Error("New session should have no messages")
	}

	// Add message
	session.AddMessage(agent.AgentMessage{
		Role:    "user",
		Content: "Test message",
	})

	if len(session.Messages) != 1 {
		t.Error("Session should have one message")
	}
	if session.Messages[0].Content != "Test message" {
		t.Error("Message content mismatch")
	}

	// Add result
	session.AddResult(agent.ProcessResult{
		TaskType: agent.TaskTypeImageOCR,
		Text:     "Test result",
	})

	if len(session.Results) != 1 {
		t.Error("Session should have one result")
	}
}

// TestIntegration_ProcessOptions tests process options.
func TestIntegration_ProcessOptions(t *testing.T) {
	opts := agent.DefaultProcessOptions()

	if opts.Language != "chi_sim+eng" {
		t.Errorf("Expected default language chi_sim+eng, got %s", opts.Language)
	}
	if opts.DPI != 300 {
		t.Errorf("Expected default DPI 300, got %d", opts.DPI)
	}
	if !opts.ExtractText {
		t.Error("ExtractText should be true by default")
	}
	if !opts.ExtractItems {
		t.Error("ExtractItems should be true by default")
	}
	if opts.OutputFormat != "json" {
		t.Errorf("Expected default output format json, got %s", opts.OutputFormat)
	}
	if opts.Timeout != 120*time.Second {
		t.Errorf("Expected default timeout 120s, got %v", opts.Timeout)
	}
}

// TestIntegration_ExtractionSchema tests extraction schema.
func TestIntegration_ExtractionSchema(t *testing.T) {
	schema := &agent.ExtractionSchema{
		Name:        "invoice",
		Description: "Invoice extraction schema",
		Fields: []agent.ExtractionField{
			{
				Name:        "invoice_number",
				Type:        "string",
				Description: "发票号码",
				Required:    true,
				Pattern:     `\d{8,20}`,
			},
			{
				Name:        "amount",
				Type:        "number",
				Description: "金额",
				Required:    true,
			},
			{
				Name:        "date",
				Type:        "date",
				Description: "日期",
				Required:    false,
				Aliases:     []string{"开票日期", "日期"},
			},
		},
		Tables: []agent.TableSchema{
			{
				Name: "items",
				Columns: []agent.ExtractionField{
					{Name: "name", Type: "string"},
					{Name: "quantity", Type: "number"},
					{Name: "price", Type: "number"},
				},
			},
		},
	}

	if schema.Name != "invoice" {
		t.Error("Schema name mismatch")
	}
	if len(schema.Fields) != 3 {
		t.Errorf("Expected 3 fields, got %d", len(schema.Fields))
	}
	if len(schema.Tables) != 1 {
		t.Error("Expected 1 table")
	}
	if schema.Fields[0].Required != true {
		t.Error("First field should be required")
	}
}

// TestIntegration_TaskInstructions tests task instructions.
func TestIntegration_TaskInstructions(t *testing.T) {
	tasks := []agent.TaskType{
		agent.TaskTypeImageOCR,
		agent.TaskTypePDFOCR,
		agent.TaskTypeInvoiceRecognition,
		agent.TaskTypeStructuredExtraction,
	}

	for _, task := range tasks {
		instruction := agent.GetInstructionForTask(task)
		if instruction == "" {
			t.Errorf("Instruction for %s should not be empty", task)
		}
	}

	// Test unknown task
	instruction := agent.GetInstructionForTask(agent.TaskTypeUnknown)
	if instruction == "" {
		t.Error("Unknown task should return default instruction")
	}
}

// Helper function
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
