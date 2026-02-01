package agent

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"

	"github.com/cloudwego/eino/vdocsOCR/config"
	"github.com/cloudwego/eino/vdocsOCR/tools/invoice"
	"github.com/cloudwego/eino/vdocsOCR/tools/ocr"
	"github.com/cloudwego/eino/vdocsOCR/tools/pdf"
)

// AgentFactory creates and manages OCR agents.
type AgentFactory struct {
	config       *config.Config
	modelPool    map[string]model.ToolCallingChatModel
	ocrManager   *ocr.OCRManager
	pdfProcessor *pdf.PDFProcessor
	invoiceRecognizer *invoice.InvoiceRecognizer
}

// NewAgentFactory creates a new agent factory.
func NewAgentFactory(cfg *config.Config) (*AgentFactory, error) {
	factory := &AgentFactory{
		config:    cfg,
		modelPool: make(map[string]model.ToolCallingChatModel),
	}

	// Initialize OCR manager
	ocrMgr, err := ocr.NewOCRManager(&cfg.OCR)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize OCR manager: %w", err)
	}
	factory.ocrManager = ocrMgr

	// Initialize PDF processor
	pdfProc, err := pdf.NewPDFProcessor(&cfg.OCR.PDFConfig, ocrMgr)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize PDF processor: %w", err)
	}
	factory.pdfProcessor = pdfProc

	// Initialize invoice recognizer
	factory.invoiceRecognizer = invoice.NewInvoiceRecognizer(&cfg.OCR.InvoiceConfig, ocrMgr)

	// Initialize model pool
	if err := factory.initModelPool(); err != nil {
		return nil, fmt.Errorf("failed to initialize model pool: %w", err)
	}

	return factory, nil
}

// initModelPool initializes the model pool based on configuration.
func (f *AgentFactory) initModelPool() error {
	for name, providerCfg := range f.config.LLM.Providers {
		if !providerCfg.Enabled {
			continue
		}

		// Create model based on provider type
		chatModel, err := f.createModel(name, providerCfg)
		if err != nil {
			// Log warning but continue with other providers
			fmt.Printf("Warning: failed to create model for provider %s: %v\n", name, err)
			continue
		}

		f.modelPool[name] = chatModel
	}

	if len(f.modelPool) == 0 {
		return fmt.Errorf("no LLM providers available")
	}

	return nil
}

// createModel creates a model based on provider configuration.
func (f *AgentFactory) createModel(name string, cfg *config.ProviderConfig) (model.ToolCallingChatModel, error) {
	// This is a placeholder - actual implementation would use eino-ext providers
	// For example: arkmodel, openai, anthropic, etc.
	
	switch cfg.Type {
	case "openai":
		return f.createOpenAIModel(cfg)
	case "anthropic":
		return f.createAnthropicModel(cfg)
	case "deepseek":
		return f.createDeepSeekModel(cfg)
	case "qwen":
		return f.createQwenModel(cfg)
	case "ollama":
		return f.createOllamaModel(cfg)
	default:
		return nil, fmt.Errorf("unsupported provider type: %s", cfg.Type)
	}
}

// Placeholder model creation functions
// In production, these would use eino-ext providers

func (f *AgentFactory) createOpenAIModel(cfg *config.ProviderConfig) (model.ToolCallingChatModel, error) {
	// TODO: Implement using eino-ext/components/model/openai
	// Example:
	// return openai.NewChatModel(ctx, &openai.ChatModelConfig{
	//     APIKey: cfg.APIKey,
	//     Model:  cfg.Model,
	//     BaseURL: cfg.BaseURL,
	// })
	return nil, fmt.Errorf("openai model not implemented yet")
}

func (f *AgentFactory) createAnthropicModel(cfg *config.ProviderConfig) (model.ToolCallingChatModel, error) {
	// TODO: Implement using eino-ext/components/model/anthropic
	return nil, fmt.Errorf("anthropic model not implemented yet")
}

func (f *AgentFactory) createDeepSeekModel(cfg *config.ProviderConfig) (model.ToolCallingChatModel, error) {
	// TODO: Implement using DeepSeek API (compatible with OpenAI)
	// DeepSeek uses OpenAI-compatible API
	return nil, fmt.Errorf("deepseek model not implemented yet")
}

func (f *AgentFactory) createQwenModel(cfg *config.ProviderConfig) (model.ToolCallingChatModel, error) {
	// TODO: Implement using Qwen API
	return nil, fmt.Errorf("qwen model not implemented yet")
}

func (f *AgentFactory) createOllamaModel(cfg *config.ProviderConfig) (model.ToolCallingChatModel, error) {
	// TODO: Implement using eino-ext/components/model/ollama
	return nil, fmt.Errorf("ollama model not implemented yet")
}

// GetModel returns a model by provider name.
func (f *AgentFactory) GetModel(provider string) (model.ToolCallingChatModel, error) {
	m, ok := f.modelPool[provider]
	if !ok {
		return nil, fmt.Errorf("model provider not found: %s", provider)
	}
	return m, nil
}

// GetDefaultModel returns the default model.
func (f *AgentFactory) GetDefaultModel() (model.ToolCallingChatModel, error) {
	return f.GetModel(f.config.LLM.DefaultProvider)
}

// CreateOCRAgent creates the main OCR agent.
func (f *AgentFactory) CreateOCRAgent(ctx context.Context) (*OCRAgentImpl, error) {
	chatModel, err := f.GetDefaultModel()
	if err != nil {
		// For development, allow nil model with OCR-only mode
		fmt.Printf("Warning: LLM not available, running in OCR-only mode\n")
	}

	// Create tools
	tools := f.createTools()

	return &OCRAgentImpl{
		config:       f.config,
		chatModel:    chatModel,
		ocrManager:   f.ocrManager,
		pdfProcessor: f.pdfProcessor,
		invoiceRecognizer: f.invoiceRecognizer,
		tools:        tools,
	}, nil
}

// createTools creates the tool set for the OCR agent.
func (f *AgentFactory) createTools() []tool.BaseTool {
	var tools []tool.BaseTool

	// OCR Tool
	tools = append(tools, NewOCRTool(f.ocrManager))

	// PDF Tool
	tools = append(tools, NewPDFTool(f.pdfProcessor))

	// Invoice Tool
	tools = append(tools, NewInvoiceTool(f.invoiceRecognizer))

	// Extraction Tool
	tools = append(tools, NewExtractionTool())

	return tools
}

// CreateReActAgent creates a ReAct-style agent.
func (f *AgentFactory) CreateReActAgent(ctx context.Context) (adk.Agent, error) {
	chatModel, err := f.GetDefaultModel()
	if err != nil {
		return nil, err
	}

	tools := f.createTools()

	// Convert tools to adk tools
	adkTools := make([]adk.Tool, 0, len(tools))
	for _, t := range tools {
		adkTool, ok := t.(adk.Tool)
		if ok {
			adkTools = append(adkTools, adkTool)
		}
	}

	// Create ReAct agent using eino adk
	agent, err := adk.NewReActAgent(ctx, &adk.ReActAgentConfig{
		Model:       chatModel,
		Tools:       adkTools,
		MaxSteps:    10,
		Instruction: ImageOCRInstruction,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create ReAct agent: %w", err)
	}

	return agent, nil
}

// GetOCRManager returns the OCR manager.
func (f *AgentFactory) GetOCRManager() *ocr.OCRManager {
	return f.ocrManager
}

// GetPDFProcessor returns the PDF processor.
func (f *AgentFactory) GetPDFProcessor() *pdf.PDFProcessor {
	return f.pdfProcessor
}

// GetInvoiceRecognizer returns the invoice recognizer.
func (f *AgentFactory) GetInvoiceRecognizer() *invoice.InvoiceRecognizer {
	return f.invoiceRecognizer
}

// Close closes all resources.
func (f *AgentFactory) Close() error {
	var errs []error

	if f.ocrManager != nil {
		if err := f.ocrManager.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if f.pdfProcessor != nil {
		if err := f.pdfProcessor.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors closing factory: %v", errs)
	}
	return nil
}
