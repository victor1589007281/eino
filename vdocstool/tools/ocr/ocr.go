// Package ocr OCR 工具 - MCP 工具入口
package ocr

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/cloudwego/eino/vdocstool/mcp"
)

// Tool OCR 工具
type Tool struct {
	config          *Config
	engineManager   *EngineManager
	preprocessor    *Preprocessor
	invoiceManager  *InvoiceTemplateManager
	mu              sync.RWMutex
}

// NewTool 创建 OCR 工具
func NewTool(config *Config) (*Tool, error) {
	if config == nil {
		config = DefaultConfig()
	}

	preprocessConfig := &PreprocessConfig{
		EnableGrayscale:   false,
		EnableBinarize:    false,
		EnableDenoise:     false,
		EnableDeskew:      false,
		BinarizeThreshold: 128,
		MaxWidth:          4096,
		MaxHeight:         4096,
		JPEGQuality:       90,
	}

	tool := &Tool{
		config:         config,
		engineManager:  NewEngineManager(config),
		preprocessor:   NewPreprocessor(preprocessConfig),
		invoiceManager: NewInvoiceTemplateManager(),
	}

	return tool, nil
}

// Start 启动工具
func (t *Tool) Start(ctx context.Context) error {
	return t.engineManager.Start(ctx)
}

// Close 关闭工具
func (t *Tool) Close() error {
	return t.engineManager.Stop()
}

// RegisterEngines 注册引擎
func (t *Tool) RegisterEngines(engines ...Engine) {
	for _, engine := range engines {
		t.engineManager.RegisterEngine(engine)
	}
}

// RegisterTools 注册 MCP 工具
func (t *Tool) RegisterTools(server *mcp.Server) {
	// ocr_image - 通用图像 OCR
	ocrImageBuilder := mcp.NewToolBuilder("ocr_image", "识别图片中的文字，支持本地路径、URL或Base64")
	ocrImageBuilder.AddProperty("image_path", "string", "本地图片路径", false)
	ocrImageBuilder.AddProperty("image_url", "string", "图片URL", false)
	ocrImageBuilder.AddProperty("image_base64", "string", "图片Base64编码", false)
	ocrImageBuilder.AddEnumProperty("scene", "识别场景", []string{"general", "document", "print", "handwriting"}, false)
	ocrImageBuilder.AddProperty("languages", "string", "识别语言，用逗号分隔，如zh,en", false)
	ocrImageBuilder.AddEnumProperty("strategy", "路由策略", []string{"quality_first", "cost_first", "speed_first", "smart"}, false)
	ocrImageBuilder.AddProperty("return_position", "boolean", "是否返回文字位置", false)
	ocrImageBuilder.AddProperty("return_confidence", "boolean", "是否返回置信度", false)
	server.RegisterTool(ocrImageBuilder.Build(), t.handleOCRImage)

	// ocr_document - 文档 OCR
	ocrDocBuilder := mcp.NewToolBuilder("ocr_document", "识别文档图片，支持版面分析")
	ocrDocBuilder.AddProperty("image_path", "string", "本地图片路径", false)
	ocrDocBuilder.AddProperty("image_url", "string", "图片URL", false)
	ocrDocBuilder.AddProperty("image_base64", "string", "图片Base64编码", false)
	ocrDocBuilder.AddProperty("analyze_layout", "boolean", "是否分析版面", false)
	server.RegisterTool(ocrDocBuilder.Build(), t.handleOCRDocument)

	// ocr_table - 表格 OCR
	ocrTableBuilder := mcp.NewToolBuilder("ocr_table", "识别图片中的表格，返回结构化数据")
	ocrTableBuilder.AddProperty("image_path", "string", "本地图片路径", false)
	ocrTableBuilder.AddProperty("image_url", "string", "图片URL", false)
	ocrTableBuilder.AddProperty("image_base64", "string", "图片Base64编码", false)
	ocrTableBuilder.AddEnumProperty("output_format", "输出格式", []string{"json", "html", "csv"}, false)
	server.RegisterTool(ocrTableBuilder.Build(), t.handleOCRTable)

	// ocr_invoice - 票据 OCR
	ocrInvoiceBuilder := mcp.NewToolBuilder("ocr_invoice", "识别发票、收据等票据，支持医疗发票、增值税发票、普通发票等多种类型")
	ocrInvoiceBuilder.AddProperty("image_path", "string", "本地图片路径", false)
	ocrInvoiceBuilder.AddProperty("image_url", "string", "图片URL", false)
	ocrInvoiceBuilder.AddProperty("image_base64", "string", "图片Base64编码", false)
	ocrInvoiceBuilder.AddEnumProperty("invoice_type", "票据类型", []string{
		"auto",           // 自动识别
		"medical",        // 医疗发票
		"vat",            // 增值税发票
		"vat_special",    // 增值税专用发票
		"vat_normal",     // 增值税普通发票
		"vat_electronic", // 增值税电子发票
		"taxi",           // 出租车发票
		"train",          // 火车票
		"flight",         // 机票行程单
		"general",        // 普通发票
	}, false)
	ocrInvoiceBuilder.AddProperty("return_confidence", "boolean", "是否返回置信度", false)
	server.RegisterTool(ocrInvoiceBuilder.Build(), t.handleOCRInvoice)

	// ocr_card - 证件 OCR
	ocrCardBuilder := mcp.NewToolBuilder("ocr_card", "识别身份证、护照等证件（注意隐私保护）")
	ocrCardBuilder.AddProperty("image_path", "string", "本地图片路径", false)
	ocrCardBuilder.AddProperty("image_url", "string", "图片URL", false)
	ocrCardBuilder.AddProperty("image_base64", "string", "图片Base64编码", false)
	ocrCardBuilder.AddEnumProperty("card_type", "证件类型", []string{"auto", "id_card", "passport", "driver_license", "business_license"}, false)
	server.RegisterTool(ocrCardBuilder.Build(), t.handleOCRCard)

	// ocr_set_strategy - 设置路由策略
	ocrStrategyBuilder := mcp.NewToolBuilder("ocr_set_strategy", "设置OCR路由策略")
	ocrStrategyBuilder.AddEnumProperty("strategy", "路由策略", []string{"quality_first", "cost_first", "speed_first", "smart"}, true)
	server.RegisterTool(ocrStrategyBuilder.Build(), t.handleSetStrategy)

	// ocr_status - 获取引擎状态
	ocrStatusBuilder := mcp.NewToolBuilder("ocr_status", "获取OCR引擎状态和统计信息")
	server.RegisterTool(ocrStatusBuilder.Build(), t.handleGetStatus)

	// ocr_quality - 评估图片质量
	ocrQualityBuilder := mcp.NewToolBuilder("ocr_quality", "评估图片质量，判断是否适合OCR识别")
	ocrQualityBuilder.AddProperty("image_path", "string", "本地图片路径", false)
	ocrQualityBuilder.AddProperty("image_url", "string", "图片URL", false)
	ocrQualityBuilder.AddProperty("image_base64", "string", "图片Base64编码", false)
	server.RegisterTool(ocrQualityBuilder.Build(), t.handleAssessQuality)
}

// handleOCRImage 处理通用 OCR 请求
func (t *Tool) handleOCRImage(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	req, err := t.parseOCRRequest(params)
	if err != nil {
		return nil, err
	}

	// 设置默认场景
	if req.Scene == "" {
		req.Scene = SceneGeneral
	}

	// 设置默认策略
	if req.Strategy == "" {
		req.Strategy = t.config.DefaultStrategy
	}

	return t.doOCR(ctx, req)
}

// handleOCRDocument 处理文档 OCR 请求
func (t *Tool) handleOCRDocument(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	req, err := t.parseOCRRequest(params)
	if err != nil {
		return nil, err
	}

	req.Scene = SceneDocument
	if req.Strategy == "" {
		req.Strategy = t.config.DefaultStrategy
	}

	ocrResult, err := t.doOCR(ctx, req)
	if err != nil {
		return nil, err
	}

	docResult := &DocumentResult{
		OCRResult: *ocrResult,
	}

	// 分析版面 (如果启用)
	if analyzeLayout, ok := params["analyze_layout"].(bool); ok && analyzeLayout {
		docResult.Layout = t.analyzeLayout(ocrResult)
	}

	// 提取段落
	docResult.Paragraphs = t.extractParagraphs(ocrResult)

	return docResult, nil
}

// handleOCRTable 处理表格 OCR 请求
func (t *Tool) handleOCRTable(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	req, err := t.parseOCRRequest(params)
	if err != nil {
		return nil, err
	}

	req.Scene = SceneTable
	if req.Strategy == "" {
		req.Strategy = StrategyQualityFirst // 表格识别优先质量
	}

	// 获取引擎
	engine, err := t.engineManager.SelectEngine(ctx, SceneTable, req.Strategy)
	if err != nil {
		return nil, err
	}

	// 执行表格识别
	startTime := time.Now()
	result, err := engine.RecognizeTable(ctx, req)
	latency := time.Since(startTime).Milliseconds()

	t.engineManager.UpdateStats(engine.Name(), latency, err != nil)

	if err != nil {
		return t.tryFallback(ctx, req, err)
	}

	// 格式化输出
	outputFormat := "json"
	if format, ok := params["output_format"].(string); ok {
		outputFormat = format
	}

	return t.formatTableResult(result, outputFormat), nil
}

// handleOCRInvoice 处理票据 OCR 请求
func (t *Tool) handleOCRInvoice(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	req, err := t.parseOCRRequest(params)
	if err != nil {
		return nil, err
	}

	req.Scene = SceneInvoice
	if req.Strategy == "" {
		req.Strategy = StrategyQualityFirst
	}

	// 获取指定的发票类型
	invoiceType := InvoiceTypeAuto
	if typeStr, ok := params["invoice_type"].(string); ok && typeStr != "" {
		invoiceType = InvoiceType(typeStr)
	}

	// 首先执行通用 OCR 获取文本
	ocrResult, err := t.doOCR(ctx, req)
	if err != nil {
		return nil, err
	}

	// 使用发票模板管理器解析发票
	result := t.invoiceManager.ParseInvoice(ocrResult, invoiceType)

	return result, nil
}

// handleOCRCard 处理证件 OCR 请求
func (t *Tool) handleOCRCard(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	req, err := t.parseOCRRequest(params)
	if err != nil {
		return nil, err
	}

	req.Scene = SceneIDCard
	if req.Strategy == "" {
		req.Strategy = StrategyQualityFirst
	}

	// 获取引擎
	engine, err := t.engineManager.SelectEngine(ctx, SceneIDCard, req.Strategy)
	if err != nil {
		return nil, err
	}

	// 执行证件识别
	startTime := time.Now()
	result, err := engine.RecognizeIDCard(ctx, req)
	latency := time.Since(startTime).Milliseconds()

	t.engineManager.UpdateStats(engine.Name(), latency, err != nil)

	if err != nil {
		return t.tryFallback(ctx, req, err)
	}

	// 隐私保护：部分隐藏敏感信息
	t.maskSensitiveInfo(result)

	return result, nil
}

// handleSetStrategy 设置路由策略
func (t *Tool) handleSetStrategy(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	strategy, ok := params["strategy"].(string)
	if !ok {
		return nil, fmt.Errorf("strategy is required")
	}

	t.mu.Lock()
	t.config.DefaultStrategy = RoutingStrategy(strategy)
	t.mu.Unlock()

	return map[string]interface{}{
		"success":  true,
		"strategy": strategy,
		"message":  fmt.Sprintf("routing strategy set to %s", strategy),
	}, nil
}

// handleGetStatus 获取引擎状态
func (t *Tool) handleGetStatus(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	status := t.engineManager.GetStatus()
	health := t.engineManager.HealthCheck(ctx)

	result := map[string]interface{}{
		"engines":          status,
		"default_strategy": t.config.DefaultStrategy,
		"max_memory_mb":    t.config.MaxMemoryMB,
		"health":           make(map[string]string),
	}

	healthMap := result["health"].(map[string]string)
	for engine, err := range health {
		if err != nil {
			healthMap[string(engine)] = fmt.Sprintf("unhealthy: %v", err)
		} else {
			healthMap[string(engine)] = "healthy"
		}
	}

	return result, nil
}

// handleAssessQuality 评估图片质量
func (t *Tool) handleAssessQuality(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	req, err := t.parseOCRRequest(params)
	if err != nil {
		return nil, err
	}

	// 加载图片
	imgData, err := t.preprocessor.LoadImage(ctx, req)
	if err != nil {
		return nil, err
	}

	// 评估质量
	quality := t.preprocessor.AssessQuality(imgData)

	// 给出建议
	var suggestions []string
	if quality.Score < 0.5 {
		suggestions = append(suggestions, "图片质量较低，建议重新拍摄或使用更高质量的图片")
	}
	if quality.Clarity < 0.5 {
		suggestions = append(suggestions, "图片模糊，建议使用更清晰的图片")
	}
	if quality.Brightness < 0.3 {
		suggestions = append(suggestions, "图片过暗，建议增加光照")
	} else if quality.Brightness > 0.8 {
		suggestions = append(suggestions, "图片过亮，建议减少光照或避免过曝")
	}
	if quality.IsSkewed {
		suggestions = append(suggestions, "图片可能有倾斜，建议校正")
	}

	return map[string]interface{}{
		"quality":     quality,
		"suitable":    quality.Score >= 0.5,
		"suggestions": suggestions,
	}, nil
}

// parseOCRRequest 解析 OCR 请求
func (t *Tool) parseOCRRequest(params map[string]interface{}) (*OCRRequest, error) {
	req := &OCRRequest{}

	if imagePath, ok := params["image_path"].(string); ok {
		req.ImagePath = imagePath
	}
	if imageURL, ok := params["image_url"].(string); ok {
		req.ImageURL = imageURL
	}
	if imageBase64, ok := params["image_base64"].(string); ok {
		req.ImageBase64 = imageBase64
	}

	// 至少需要一个图像源
	if req.ImagePath == "" && req.ImageURL == "" && req.ImageBase64 == "" {
		return nil, fmt.Errorf("at least one of image_path, image_url, or image_base64 is required")
	}

	if scene, ok := params["scene"].(string); ok {
		req.Scene = SceneType(scene)
	}
	if strategy, ok := params["strategy"].(string); ok {
		req.Strategy = RoutingStrategy(strategy)
	}
	if languages, ok := params["languages"].(string); ok {
		req.Languages = splitLanguages(languages)
	}
	if returnPosition, ok := params["return_position"].(bool); ok {
		req.ReturnPosition = returnPosition
	}
	if returnConfidence, ok := params["return_confidence"].(bool); ok {
		req.ReturnConfidence = returnConfidence
	}
	if specifiedEngine, ok := params["specified_engine"].(string); ok {
		req.SpecifiedEngine = EngineType(specifiedEngine)
	}
	if timeout, ok := params["timeout"].(float64); ok {
		req.Timeout = int(timeout)
	}

	return req, nil
}

// doOCR 执行 OCR 识别
func (t *Tool) doOCR(ctx context.Context, req *OCRRequest) (*OCRResult, error) {
	var engine Engine
	var err error

	// 指定引擎
	if req.SpecifiedEngine != "" {
		engine, err = t.engineManager.GetEngine(ctx, req.SpecifiedEngine)
	} else {
		engine, err = t.engineManager.SelectEngine(ctx, req.Scene, req.Strategy)
	}

	if err != nil {
		return nil, err
	}

	// 执行识别
	startTime := time.Now()
	result, err := engine.Recognize(ctx, req)
	latency := time.Since(startTime).Milliseconds()

	t.engineManager.UpdateStats(engine.Name(), latency, err != nil)

	if err != nil {
		return t.tryFallbackOCR(ctx, req, err)
	}

	return result, nil
}

// tryFallback 尝试降级
func (t *Tool) tryFallback(ctx context.Context, req *OCRRequest, originalErr error) (interface{}, error) {
	fallbackChain := t.engineManager.GetFallbackChain()
	
	for _, engineType := range fallbackChain {
		engine, err := t.engineManager.GetEngine(ctx, engineType)
		if err != nil {
			continue
		}

		result, err := engine.Recognize(ctx, req)
		if err == nil {
			result.Metadata.ProcessTime = time.Now()
			return result, nil
		}
	}

	return nil, fmt.Errorf("all fallback engines failed, original error: %v", originalErr)
}

// tryFallbackOCR 尝试降级 (返回 OCRResult)
func (t *Tool) tryFallbackOCR(ctx context.Context, req *OCRRequest, originalErr error) (*OCRResult, error) {
	result, err := t.tryFallback(ctx, req, originalErr)
	if err != nil {
		return nil, err
	}
	if ocrResult, ok := result.(*OCRResult); ok {
		return ocrResult, nil
	}
	return nil, fmt.Errorf("unexpected result type")
}

// analyzeLayout 分析版面
func (t *Tool) analyzeLayout(result *OCRResult) *LayoutInfo {
	// 简化的版面分析
	layout := &LayoutInfo{
		Columns: 1,
	}

	if len(result.Blocks) > 0 {
		layout.Regions = make([]LayoutRegion, 0, len(result.Blocks))
		for _, block := range result.Blocks {
			region := LayoutRegion{
				Type: block.BlockType,
				Text: block.Text,
			}
			if block.Box != nil {
				region.Box = block.Box
			}
			layout.Regions = append(layout.Regions, region)
		}
	}

	return layout
}

// extractParagraphs 提取段落
func (t *Tool) extractParagraphs(result *OCRResult) []string {
	if result.FullText == "" {
		return nil
	}

	// 按空行分段
	paragraphs := make([]string, 0)
	current := ""

	for _, block := range result.Blocks {
		if block.Text == "" {
			if current != "" {
				paragraphs = append(paragraphs, current)
				current = ""
			}
		} else {
			if current != "" {
				current += " "
			}
			current += block.Text
		}
	}

	if current != "" {
		paragraphs = append(paragraphs, current)
	}

	return paragraphs
}

// formatTableResult 格式化表格结果
func (t *Tool) formatTableResult(result *TableResult, format string) interface{} {
	if len(result.Tables) == 0 {
		return result
	}

	switch format {
	case "html":
		for i := range result.Tables {
			result.Tables[i].HTML = t.tableToHTML(&result.Tables[i])
		}
	case "csv":
		for i := range result.Tables {
			result.Tables[i].CSV = t.tableToCSV(&result.Tables[i])
		}
	}

	return result
}

// tableToHTML 表格转 HTML
func (t *Tool) tableToHTML(table *Table) string {
	html := "<table border='1'>\n"
	
	if len(table.Headers) > 0 {
		html += "  <thead><tr>\n"
		for _, h := range table.Headers {
			html += fmt.Sprintf("    <th>%s</th>\n", h)
		}
		html += "  </tr></thead>\n"
	}
	
	html += "  <tbody>\n"
	for _, row := range table.Rows {
		html += "    <tr>\n"
		for _, cell := range row {
			html += fmt.Sprintf("      <td>%s</td>\n", cell)
		}
		html += "    </tr>\n"
	}
	html += "  </tbody>\n</table>"
	
	return html
}

// tableToCSV 表格转 CSV
func (t *Tool) tableToCSV(table *Table) string {
	var csv string
	
	if len(table.Headers) > 0 {
		csv = joinCSV(table.Headers) + "\n"
	}
	
	for _, row := range table.Rows {
		csv += joinCSV(row) + "\n"
	}
	
	return csv
}

// maskSensitiveInfo 隐藏敏感信息
func (t *Tool) maskSensitiveInfo(result *IDCardResult) {
	if result.IDNumber != "" && len(result.IDNumber) > 8 {
		// 只显示前4位和后4位
		masked := result.IDNumber[:4] + "**********" + result.IDNumber[len(result.IDNumber)-4:]
		result.IDNumber = masked
	}
}

// splitLanguages 分割语言列表
func splitLanguages(s string) []string {
	if s == "" {
		return nil
	}
	
	// 尝试不同的分隔符
	for _, sep := range []string{",", ";", " "} {
		parts := splitBy(s, sep)
		if len(parts) > 1 {
			return parts
		}
	}
	
	// 没有找到分隔符，返回原字符串
	s = stringsTrimSpace(s)
	if s == "" {
		return nil
	}
	return []string{s}
}

func splitBy(s, sep string) []string {
	result := make([]string, 0)
	for _, p := range stringsSplit(s, sep) {
		p = stringsTrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

func stringsSplit(s, sep string) []string {
	var result []string
	start := 0
	for i := 0; i <= len(s)-len(sep); i++ {
		if s[i:i+len(sep)] == sep {
			result = append(result, s[start:i])
			start = i + len(sep)
		}
	}
	result = append(result, s[start:])
	return result
}

func stringsTrimSpace(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}

func joinCSV(row []string) string {
	result := ""
	for i, cell := range row {
		if i > 0 {
			result += ","
		}
		// 转义逗号和引号
		needQuote := false
		for _, c := range cell {
			if c == ',' || c == '"' || c == '\n' {
				needQuote = true
				break
			}
		}
		if needQuote {
			escaped := ""
			for _, c := range cell {
				if c == '"' {
					escaped += "\"\""
				} else {
					escaped += string(c)
				}
			}
			result += "\"" + escaped + "\""
		} else {
			result += cell
		}
	}
	return result
}

// ToJSON 转换为 JSON 字符串
func ToJSON(v interface{}) string {
	data, _ := json.Marshal(v)
	return string(data)
}
