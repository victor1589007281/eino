// Package invoice provides invoice recognition and extraction capabilities.
package invoice

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/cloudwego/eino/vdocsOCR/config"
	"github.com/cloudwego/eino/vdocsOCR/tools/ocr"
)

// InvoiceType represents the type of invoice.
type InvoiceType string

const (
	InvoiceTypeVAT          InvoiceType = "vat_invoice"          // 增值税普通发票
	InvoiceTypeVATSpecial   InvoiceType = "vat_special_invoice"  // 增值税专用发票
	InvoiceTypeReceipt      InvoiceType = "receipt"              // 收据
	InvoiceTypeTaxiReceipt  InvoiceType = "taxi_receipt"         // 出租车发票
	InvoiceTypeTrainTicket  InvoiceType = "train_ticket"         // 火车票
	InvoiceTypeAirTicket    InvoiceType = "air_ticket"           // 机票
	InvoiceTypeHotelInvoice InvoiceType = "hotel_invoice"        // 酒店发票
	InvoiceTypeTollInvoice  InvoiceType = "toll_invoice"         // 过路费发票
	InvoiceTypeUnknown      InvoiceType = "unknown"              // 未知类型
)

// InvoiceRecognizer handles invoice recognition and extraction.
type InvoiceRecognizer struct {
	config     *config.InvoiceConfig
	ocrManager *ocr.OCRManager
	patterns   map[string]*regexp.Regexp
}

// NewInvoiceRecognizer creates a new invoice recognizer.
func NewInvoiceRecognizer(cfg *config.InvoiceConfig, ocrMgr *ocr.OCRManager) *InvoiceRecognizer {
	r := &InvoiceRecognizer{
		config:     cfg,
		ocrManager: ocrMgr,
		patterns:   make(map[string]*regexp.Regexp),
	}

	// Compile regex patterns
	r.patterns["invoice_code"] = regexp.MustCompile(`发票代码[：:\s]*(\d{10,12})`)
	r.patterns["invoice_number"] = regexp.MustCompile(`发票号码[：:\s]*(\d{8,20})`)
	r.patterns["invoice_date"] = regexp.MustCompile(`开票日期[：:\s]*([\d年月日\-/\.]+)`)
	r.patterns["check_code"] = regexp.MustCompile(`校验码[：:\s]*([\d\s]{20,24})`)
	r.patterns["buyer_name"] = regexp.MustCompile(`(?:购买方|购货单位|购方|名\s*称)[：:\s]*([^\n\r]+)`)
	r.patterns["buyer_tax_id"] = regexp.MustCompile(`(?:购买方|购方)?(?:纳税人)?识别号[：:\s]*([A-Za-z0-9]{15,20})`)
	r.patterns["seller_name"] = regexp.MustCompile(`(?:销售方|销货单位|销方|名\s*称)[：:\s]*([^\n\r]+)`)
	r.patterns["seller_tax_id"] = regexp.MustCompile(`(?:销售方|销方)?(?:纳税人)?识别号[：:\s]*([A-Za-z0-9]{15,20})`)
	r.patterns["amount"] = regexp.MustCompile(`(?:金额|合计)[（(]?不含税[）)]?[：:\s]*¥?\s*([\d,\.]+)`)
	r.patterns["tax_amount"] = regexp.MustCompile(`税额[：:\s]*¥?\s*([\d,\.]+)`)
	r.patterns["total_amount"] = regexp.MustCompile(`(?:价税合计|合计|总计|总额)[（(]?大写[）)]?[：:\s]*([^\n\r]+)`)
	r.patterns["total_amount_lower"] = regexp.MustCompile(`(?:价税合计|合计|总计|总额)[（(]?小写[）)]?[：:\s]*¥?\s*([\d,\.]+)`)

	// Additional patterns for specific invoice types
	r.patterns["train_from"] = regexp.MustCompile(`(?:出发|始发|从)[：:\s]*([^\n\r]+?)[站]?`)
	r.patterns["train_to"] = regexp.MustCompile(`(?:到达|终点|至)[：:\s]*([^\n\r]+?)[站]?`)
	r.patterns["train_date"] = regexp.MustCompile(`([\d]{4}年[\d]{1,2}月[\d]{1,2}日)`)
	r.patterns["train_seat"] = regexp.MustCompile(`([一二三]等座|硬座|软座|硬卧|软卧|无座|商务座)`)
	r.patterns["train_number"] = regexp.MustCompile(`([GCDZTK]?\d{1,4})次`)
	r.patterns["train_price"] = regexp.MustCompile(`¥?\s*([\d\.]+)元`)

	r.patterns["taxi_date"] = regexp.MustCompile(`日期[：:\s]*([\d年月日\-/\.]+)`)
	r.patterns["taxi_amount"] = regexp.MustCompile(`金额[：:\s]*¥?\s*([\d\.]+)`)
	r.patterns["taxi_time"] = regexp.MustCompile(`(?:上车|乘车)?时间[：:\s]*([\d:]+)`)

	r.patterns["flight_from"] = regexp.MustCompile(`(?:始发地|出发)[：:\s]*([^\n\r]+)`)
	r.patterns["flight_to"] = regexp.MustCompile(`(?:目的地|到达)[：:\s]*([^\n\r]+)`)
	r.patterns["flight_number"] = regexp.MustCompile(`([A-Z]{2}\d{3,4})`)
	r.patterns["flight_date"] = regexp.MustCompile(`([\d]{4}[-/][\d]{2}[-/][\d]{2})`)

	return r
}

// Invoice represents a recognized invoice.
type Invoice struct {
	Type       InvoiceType            `json:"type"`
	Confidence float64                `json:"confidence"`
	Fields     map[string]string      `json:"fields"`
	Items      []InvoiceItem          `json:"items,omitempty"`
	RawText    string                 `json:"raw_text,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// InvoiceItem represents an item on an invoice.
type InvoiceItem struct {
	Index       int     `json:"index"`
	Name        string  `json:"name"`
	Spec        string  `json:"spec,omitempty"`        // 规格型号
	Unit        string  `json:"unit,omitempty"`        // 单位
	Quantity    float64 `json:"quantity,omitempty"`    // 数量
	UnitPrice   float64 `json:"unit_price,omitempty"`  // 单价
	Amount      float64 `json:"amount"`                // 金额
	TaxRate     float64 `json:"tax_rate,omitempty"`    // 税率
	TaxAmount   float64 `json:"tax_amount,omitempty"`  // 税额
}

// RecognizeOptions represents options for invoice recognition.
type RecognizeOptions struct {
	Language     string   `json:"language"`
	ExpectedType InvoiceType `json:"expected_type"`  // 预期的发票类型
	ExtractItems bool     `json:"extract_items"`     // 是否提取商品明细
	Validate     bool     `json:"validate"`          // 是否验证发票真伪
}

// RecognizeResult represents the result of invoice recognition.
type RecognizeResult struct {
	Invoice     *Invoice      `json:"invoice"`
	OCRResult   *ocr.OCRResult `json:"ocr_result,omitempty"`
	Duration    time.Duration `json:"duration"`
	ProcessedAt time.Time     `json:"processed_at"`
}

// Recognize performs invoice recognition on image data.
func (r *InvoiceRecognizer) Recognize(ctx context.Context, imageData []byte, opts *RecognizeOptions) (*RecognizeResult, error) {
	start := time.Now()

	if opts == nil {
		opts = &RecognizeOptions{
			Language:     "chi_sim+eng",
			ExtractItems: true,
		}
	}

	// Perform OCR
	ocrResult, err := r.ocrManager.Recognize(ctx, imageData, &ocr.RecognizeOptions{
		Language: opts.Language,
	})
	if err != nil {
		return nil, fmt.Errorf("OCR failed: %w", err)
	}

	// Classify invoice type
	invoiceType := r.classifyInvoice(ocrResult.Text)

	// Extract fields based on type
	invoice := &Invoice{
		Type:     invoiceType,
		Fields:   make(map[string]string),
		RawText:  ocrResult.Text,
		Metadata: make(map[string]interface{}),
	}

	// Extract common fields
	r.extractCommonFields(ocrResult.Text, invoice)

	// Extract type-specific fields
	switch invoiceType {
	case InvoiceTypeVAT, InvoiceTypeVATSpecial:
		r.extractVATFields(ocrResult.Text, invoice)
		if opts.ExtractItems {
			invoice.Items = r.extractVATItems(ocrResult.Text)
		}
	case InvoiceTypeTrainTicket:
		r.extractTrainTicketFields(ocrResult.Text, invoice)
	case InvoiceTypeTaxiReceipt:
		r.extractTaxiFields(ocrResult.Text, invoice)
	case InvoiceTypeAirTicket:
		r.extractAirTicketFields(ocrResult.Text, invoice)
	}

	// Calculate confidence
	invoice.Confidence = r.calculateConfidence(invoice)

	return &RecognizeResult{
		Invoice:     invoice,
		OCRResult:   ocrResult,
		Duration:    time.Since(start),
		ProcessedAt: time.Now(),
	}, nil
}

// classifyInvoice classifies the invoice type based on OCR text.
func (r *InvoiceRecognizer) classifyInvoice(text string) InvoiceType {
	textLower := strings.ToLower(text)

	// Check for VAT invoice indicators
	if strings.Contains(text, "增值税专用发票") {
		return InvoiceTypeVATSpecial
	}
	if strings.Contains(text, "增值税普通发票") || strings.Contains(text, "增值税电子普通发票") {
		return InvoiceTypeVAT
	}

	// Check for train ticket indicators
	if strings.Contains(text, "火车票") || strings.Contains(text, "铁路客票") ||
		(r.patterns["train_number"].MatchString(text) && (strings.Contains(text, "座") || strings.Contains(text, "卧"))) {
		return InvoiceTypeTrainTicket
	}

	// Check for taxi receipt indicators
	if strings.Contains(text, "出租") || strings.Contains(textLower, "taxi") || strings.Contains(text, "打车") {
		return InvoiceTypeTaxiReceipt
	}

	// Check for air ticket indicators
	if strings.Contains(text, "机票") || strings.Contains(text, "航空") ||
		strings.Contains(textLower, "flight") || strings.Contains(text, "登机") {
		return InvoiceTypeAirTicket
	}

	// Check for hotel invoice
	if strings.Contains(text, "住宿") || strings.Contains(text, "酒店") || strings.Contains(text, "宾馆") {
		return InvoiceTypeHotelInvoice
	}

	// Check for toll invoice
	if strings.Contains(text, "过路") || strings.Contains(text, "通行费") || strings.Contains(text, "高速公路") {
		return InvoiceTypeTollInvoice
	}

	// Check for general receipt
	if strings.Contains(text, "收据") {
		return InvoiceTypeReceipt
	}

	// Default: if has invoice code/number, treat as VAT
	if r.patterns["invoice_code"].MatchString(text) || r.patterns["invoice_number"].MatchString(text) {
		return InvoiceTypeVAT
	}

	return InvoiceTypeUnknown
}

// extractCommonFields extracts common fields from invoice text.
func (r *InvoiceRecognizer) extractCommonFields(text string, invoice *Invoice) {
	// Invoice code
	if matches := r.patterns["invoice_code"].FindStringSubmatch(text); len(matches) > 1 {
		invoice.Fields["invoice_code"] = strings.TrimSpace(matches[1])
	}

	// Invoice number
	if matches := r.patterns["invoice_number"].FindStringSubmatch(text); len(matches) > 1 {
		invoice.Fields["invoice_number"] = strings.TrimSpace(matches[1])
	}

	// Invoice date
	if matches := r.patterns["invoice_date"].FindStringSubmatch(text); len(matches) > 1 {
		invoice.Fields["invoice_date"] = r.normalizeDate(matches[1])
	}

	// Check code
	if matches := r.patterns["check_code"].FindStringSubmatch(text); len(matches) > 1 {
		invoice.Fields["check_code"] = strings.ReplaceAll(strings.TrimSpace(matches[1]), " ", "")
	}
}

// extractVATFields extracts VAT invoice specific fields.
func (r *InvoiceRecognizer) extractVATFields(text string, invoice *Invoice) {
	// Buyer info
	if matches := r.patterns["buyer_name"].FindStringSubmatch(text); len(matches) > 1 {
		invoice.Fields["buyer_name"] = strings.TrimSpace(matches[1])
	}
	if matches := r.patterns["buyer_tax_id"].FindStringSubmatch(text); len(matches) > 1 {
		invoice.Fields["buyer_tax_id"] = strings.TrimSpace(matches[1])
	}

	// Seller info
	if matches := r.patterns["seller_name"].FindStringSubmatch(text); len(matches) > 1 {
		invoice.Fields["seller_name"] = strings.TrimSpace(matches[1])
	}
	if matches := r.patterns["seller_tax_id"].FindStringSubmatch(text); len(matches) > 1 {
		invoice.Fields["seller_tax_id"] = strings.TrimSpace(matches[1])
	}

	// Amounts
	if matches := r.patterns["amount"].FindStringSubmatch(text); len(matches) > 1 {
		invoice.Fields["amount"] = r.normalizeAmount(matches[1])
	}
	if matches := r.patterns["tax_amount"].FindStringSubmatch(text); len(matches) > 1 {
		invoice.Fields["tax_amount"] = r.normalizeAmount(matches[1])
	}
	if matches := r.patterns["total_amount_lower"].FindStringSubmatch(text); len(matches) > 1 {
		invoice.Fields["total_amount"] = r.normalizeAmount(matches[1])
	} else if matches := r.patterns["total_amount"].FindStringSubmatch(text); len(matches) > 1 {
		invoice.Fields["total_amount_chinese"] = strings.TrimSpace(matches[1])
	}
}

// extractVATItems extracts item details from VAT invoice.
func (r *InvoiceRecognizer) extractVATItems(text string) []InvoiceItem {
	// This is a simplified implementation
	// In production, you would use more sophisticated parsing
	var items []InvoiceItem

	// Pattern for item lines
	// Format: 商品名称 规格 单位 数量 单价 金额 税率 税额
	itemPattern := regexp.MustCompile(`([^\d\s]+)\s+([^\d\s]*)\s+([^\d\s]*)\s+([\d\.]+)\s+([\d\.]+)\s+([\d\.]+)\s*([\d\.]*%?)\s*([\d\.]*)`)

	matches := itemPattern.FindAllStringSubmatch(text, -1)
	for i, match := range matches {
		if len(match) >= 7 {
			item := InvoiceItem{
				Index: i + 1,
				Name:  strings.TrimSpace(match[1]),
			}

			if len(match) > 2 {
				item.Spec = strings.TrimSpace(match[2])
			}
			if len(match) > 3 {
				item.Unit = strings.TrimSpace(match[3])
			}
			if len(match) > 4 {
				fmt.Sscanf(match[4], "%f", &item.Quantity)
			}
			if len(match) > 5 {
				fmt.Sscanf(match[5], "%f", &item.UnitPrice)
			}
			if len(match) > 6 {
				fmt.Sscanf(match[6], "%f", &item.Amount)
			}
			if len(match) > 7 {
				rate := strings.TrimSuffix(match[7], "%")
				fmt.Sscanf(rate, "%f", &item.TaxRate)
			}
			if len(match) > 8 {
				fmt.Sscanf(match[8], "%f", &item.TaxAmount)
			}

			items = append(items, item)
		}
	}

	return items
}

// extractTrainTicketFields extracts train ticket specific fields.
func (r *InvoiceRecognizer) extractTrainTicketFields(text string, invoice *Invoice) {
	if matches := r.patterns["train_from"].FindStringSubmatch(text); len(matches) > 1 {
		invoice.Fields["from_station"] = strings.TrimSpace(matches[1])
	}
	if matches := r.patterns["train_to"].FindStringSubmatch(text); len(matches) > 1 {
		invoice.Fields["to_station"] = strings.TrimSpace(matches[1])
	}
	if matches := r.patterns["train_date"].FindStringSubmatch(text); len(matches) > 1 {
		invoice.Fields["travel_date"] = r.normalizeDate(matches[1])
	}
	if matches := r.patterns["train_seat"].FindStringSubmatch(text); len(matches) > 1 {
		invoice.Fields["seat_class"] = strings.TrimSpace(matches[1])
	}
	if matches := r.patterns["train_number"].FindStringSubmatch(text); len(matches) > 1 {
		invoice.Fields["train_number"] = strings.TrimSpace(matches[1])
	}
	if matches := r.patterns["train_price"].FindStringSubmatch(text); len(matches) > 1 {
		invoice.Fields["amount"] = r.normalizeAmount(matches[1])
	}
}

// extractTaxiFields extracts taxi receipt specific fields.
func (r *InvoiceRecognizer) extractTaxiFields(text string, invoice *Invoice) {
	if matches := r.patterns["taxi_date"].FindStringSubmatch(text); len(matches) > 1 {
		invoice.Fields["date"] = r.normalizeDate(matches[1])
	}
	if matches := r.patterns["taxi_amount"].FindStringSubmatch(text); len(matches) > 1 {
		invoice.Fields["amount"] = r.normalizeAmount(matches[1])
	}
	if matches := r.patterns["taxi_time"].FindStringSubmatch(text); len(matches) > 1 {
		invoice.Fields["time"] = strings.TrimSpace(matches[1])
	}
}

// extractAirTicketFields extracts air ticket specific fields.
func (r *InvoiceRecognizer) extractAirTicketFields(text string, invoice *Invoice) {
	if matches := r.patterns["flight_from"].FindStringSubmatch(text); len(matches) > 1 {
		invoice.Fields["departure"] = strings.TrimSpace(matches[1])
	}
	if matches := r.patterns["flight_to"].FindStringSubmatch(text); len(matches) > 1 {
		invoice.Fields["destination"] = strings.TrimSpace(matches[1])
	}
	if matches := r.patterns["flight_number"].FindStringSubmatch(text); len(matches) > 1 {
		invoice.Fields["flight_number"] = strings.TrimSpace(matches[1])
	}
	if matches := r.patterns["flight_date"].FindStringSubmatch(text); len(matches) > 1 {
		invoice.Fields["flight_date"] = r.normalizeDate(matches[1])
	}
}

// normalizeDate normalizes date string to a standard format.
func (r *InvoiceRecognizer) normalizeDate(dateStr string) string {
	// Remove spaces
	dateStr = strings.ReplaceAll(dateStr, " ", "")

	// Try to convert to YYYY-MM-DD format
	patterns := []struct {
		pattern  *regexp.Regexp
		template string
	}{
		{regexp.MustCompile(`(\d{4})年(\d{1,2})月(\d{1,2})日`), "%s-%s-%s"},
		{regexp.MustCompile(`(\d{4})[/\-\.](\d{1,2})[/\-\.](\d{1,2})`), "%s-%s-%s"},
	}

	for _, p := range patterns {
		if matches := p.pattern.FindStringSubmatch(dateStr); len(matches) > 3 {
			month := matches[2]
			if len(month) == 1 {
				month = "0" + month
			}
			day := matches[3]
			if len(day) == 1 {
				day = "0" + day
			}
			return fmt.Sprintf(p.template, matches[1], month, day)
		}
	}

	return dateStr
}

// normalizeAmount normalizes amount string.
func (r *InvoiceRecognizer) normalizeAmount(amountStr string) string {
	// Remove commas and spaces
	amountStr = strings.ReplaceAll(amountStr, ",", "")
	amountStr = strings.ReplaceAll(amountStr, " ", "")
	amountStr = strings.ReplaceAll(amountStr, "¥", "")
	return strings.TrimSpace(amountStr)
}

// calculateConfidence calculates the confidence score for the invoice.
func (r *InvoiceRecognizer) calculateConfidence(invoice *Invoice) float64 {
	var score float64
	var total float64

	// Required fields based on invoice type
	requiredFields := map[InvoiceType][]string{
		InvoiceTypeVAT:         {"invoice_code", "invoice_number", "invoice_date", "total_amount"},
		InvoiceTypeVATSpecial:  {"invoice_code", "invoice_number", "invoice_date", "buyer_tax_id", "seller_tax_id", "total_amount"},
		InvoiceTypeTrainTicket: {"from_station", "to_station", "travel_date", "amount"},
		InvoiceTypeTaxiReceipt: {"date", "amount"},
		InvoiceTypeAirTicket:   {"departure", "destination", "flight_number", "flight_date"},
	}

	fields, ok := requiredFields[invoice.Type]
	if !ok {
		return 0.5 // Unknown type
	}

	for _, field := range fields {
		total++
		if _, exists := invoice.Fields[field]; exists {
			score++
		}
	}

	if total == 0 {
		return 0.5
	}

	return score / total
}

// InvoiceToJSON converts invoice to JSON.
func InvoiceToJSON(invoice *Invoice) (string, error) {
	data, err := json.MarshalIndent(invoice, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ResultToJSON converts recognition result to JSON.
func ResultToJSON(result *RecognizeResult) (string, error) {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}
