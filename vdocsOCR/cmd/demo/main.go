// Package main provides a demo for OCR Agent.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/cloudwego/eino/vdocsOCR/agent"
	"github.com/cloudwego/eino/vdocsOCR/config"
	"github.com/cloudwego/eino/vdocsOCR/output"
)

func main() {
	fmt.Println("========================================")
	fmt.Println("       OCR Agent Demo")
	fmt.Println("========================================")
	fmt.Println()

	// Load configuration
	cfg := config.DefaultConfig()
	cfg.Storage.DataDir = "./demo-data"
	cfg.Output.OutputDir = "./demo-output"

	// Create output directory
	os.MkdirAll(cfg.Output.OutputDir, 0755)

	// Create agent factory
	factory, err := agent.NewAgentFactory(cfg)
	if err != nil {
		log.Printf("Warning: Agent factory initialization failed: %v", err)
		log.Println("Running in limited mode (OCR tools may not be available)")
	}
	if factory != nil {
		defer factory.Close()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Demo 1: Show capabilities
	fmt.Println("📋 Demo 1: Agent Capabilities")
	fmt.Println("-----------------------------")
	if factory != nil {
		ocrAgent, err := factory.CreateOCRAgent(ctx)
		if err != nil {
			log.Printf("Failed to create OCR agent: %v", err)
		} else {
			capabilities := ocrAgent.GetCapabilities()
			fmt.Println("Available capabilities:")
			for i, cap := range capabilities {
				fmt.Printf("  %d. %s\n", i+1, cap)
			}
		}
	}
	fmt.Println()

	// Demo 2: Process options
	fmt.Println("⚙️  Demo 2: Default Process Options")
	fmt.Println("------------------------------------")
	opts := agent.DefaultProcessOptions()
	fmt.Printf("  Language:      %s\n", opts.Language)
	fmt.Printf("  DPI:           %d\n", opts.DPI)
	fmt.Printf("  Output Format: %s\n", opts.OutputFormat)
	fmt.Printf("  Timeout:       %v\n", opts.Timeout)
	fmt.Println()

	// Demo 3: Output formatting
	fmt.Println("📄 Demo 3: Output Formatting")
	fmt.Println("----------------------------")
	
	// Create a sample result
	sampleResult := &agent.ProcessResult{
		TaskType:     agent.TaskTypeImageOCR,
		DocumentType: agent.DocumentTypeImage,
		Text:         "这是一段测试文本\nThis is test text\n\n发票代码：123456789012\n发票号码：12345678",
		Summary:      "这是一张包含中英文的测试图片，识别出发票相关信息。",
		Duration:     150 * time.Millisecond,
		ProcessedAt:  time.Now(),
		StructuredData: &agent.StructuredData{
			SchemaName: "invoice_preview",
			Fields: map[string]interface{}{
				"发票代码": "123456789012",
				"发票号码": "12345678",
				"类型":     "增值税普通发票",
			},
			Confidence: 0.92,
		},
	}

	// Create formatter
	formatter, err := output.NewFormatter(cfg.Output.OutputDir, false, false)
	if err != nil {
		log.Printf("Failed to create formatter: %v", err)
	} else {
		// Demo different formats
		formats := []output.OutputFormat{
			output.FormatJSON,
			output.FormatMarkdown,
			output.FormatText,
		}

		for _, format := range formats {
			fmt.Printf("\n  Output in %s format:\n", format)
			content, err := formatter.Format(sampleResult, format)
			if err != nil {
				fmt.Printf("    Error: %v\n", err)
				continue
			}

			// Show first 500 characters
			if len(content) > 500 {
				content = content[:500] + "...(truncated)"
			}
			fmt.Println("  ---")
			fmt.Println(content)
			fmt.Println("  ---")

			// Save to file
			filename := fmt.Sprintf("demo_output.%s", format)
			if err := formatter.FormatAndSave(sampleResult, format, "demo_output"); err != nil {
				fmt.Printf("    Failed to save: %v\n", err)
			} else {
				fmt.Printf("    Saved to: %s\n", filepath.Join(cfg.Output.OutputDir, filename))
			}
		}
	}
	fmt.Println()

	// Demo 4: Session management
	fmt.Println("💬 Demo 4: Session Management")
	fmt.Println("-----------------------------")
	session := agent.NewSession()
	fmt.Printf("  Session ID: %s\n", session.ID)
	fmt.Printf("  Created At: %s\n", session.CreatedAt.Format(time.RFC3339))

	// Add messages
	session.AddMessage(agent.AgentMessage{
		Role:    "user",
		Content: "请识别这张发票图片",
	})
	session.AddMessage(agent.AgentMessage{
		Role:    "assistant",
		Content: "好的，我将对图片进行OCR识别并提取发票信息。",
	})

	fmt.Printf("  Messages: %d\n", len(session.Messages))
	for i, msg := range session.Messages {
		fmt.Printf("    %d. [%s]: %s\n", i+1, msg.Role, msg.Content[:min(50, len(msg.Content))])
	}
	fmt.Println()

	// Demo 5: Extraction schema
	fmt.Println("📊 Demo 5: Extraction Schema")
	fmt.Println("----------------------------")
	schema := &agent.ExtractionSchema{
		Name:        "invoice",
		Description: "发票信息提取模式",
		Fields: []agent.ExtractionField{
			{Name: "invoice_code", Type: "string", Description: "发票代码", Required: true},
			{Name: "invoice_number", Type: "string", Description: "发票号码", Required: true},
			{Name: "amount", Type: "number", Description: "金额", Required: true},
			{Name: "date", Type: "date", Description: "日期", Required: false},
		},
	}

	fmt.Printf("  Schema: %s\n", schema.Name)
	fmt.Printf("  Description: %s\n", schema.Description)
	fmt.Printf("  Fields:\n")
	for _, field := range schema.Fields {
		required := ""
		if field.Required {
			required = " (required)"
		}
		fmt.Printf("    - %s [%s]: %s%s\n", field.Name, field.Type, field.Description, required)
	}
	fmt.Println()

	// Demo 6: Task instructions
	fmt.Println("📝 Demo 6: Task Instructions")
	fmt.Println("----------------------------")
	tasks := []agent.TaskType{
		agent.TaskTypeImageOCR,
		agent.TaskTypeInvoiceRecognition,
		agent.TaskTypePDFOCR,
	}

	for _, task := range tasks {
		instruction := agent.GetInstructionForTask(task)
		preview := instruction
		if len(preview) > 100 {
			preview = preview[:100] + "..."
		}
		fmt.Printf("  %s:\n    %s\n\n", task, preview)
	}

	fmt.Println("========================================")
	fmt.Println("       Demo Complete!")
	fmt.Println("========================================")
	fmt.Printf("Output files saved to: %s\n", cfg.Output.OutputDir)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
