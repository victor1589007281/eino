// Package ocr 发票模板系统
// 提供标准化的发票字段提取和结构化输出
package ocr

import (
	"regexp"
	"strings"
)

// InvoiceType 发票类型
type InvoiceType string

const (
	InvoiceTypeAuto             InvoiceType = "auto"              // 自动识别
	InvoiceTypeMedical          InvoiceType = "medical"           // 医疗发票
	InvoiceTypeVAT              InvoiceType = "vat"               // 增值税发票
	InvoiceTypeVATSpecial       InvoiceType = "vat_special"       // 增值税专用发票
	InvoiceTypeVATNormal        InvoiceType = "vat_normal"        // 增值税普通发票
	InvoiceTypeVATElectronic    InvoiceType = "vat_electronic"    // 增值税电子发票
	InvoiceTypeTaxi             InvoiceType = "taxi"              // 出租车发票
	InvoiceTypeTrain            InvoiceType = "train"             // 火车票
	InvoiceTypeFlight           InvoiceType = "flight"            // 机票行程单
	InvoiceTypeQuota            InvoiceType = "quota"             // 定额发票
	InvoiceTypeRollTicket       InvoiceType = "roll_ticket"       // 过路费发票
	InvoiceTypeReceipt          InvoiceType = "receipt"           // 收据
	InvoiceTypeGeneral          InvoiceType = "general"           // 普通发票
)

// MedicalInvoiceResult 医疗发票识别结果
type MedicalInvoiceResult struct {
	InvoiceResult
	
	// 医疗发票专用字段
	HospitalName          string `json:"hospital_name,omitempty"`           // 医院名称
	PatientName           string `json:"patient_name,omitempty"`            // 患者姓名
	PatientType           string `json:"patient_type,omitempty"`            // 患者类型 (门诊/住院)
	InsuranceType         string `json:"insurance_type,omitempty"`          // 医保类型 (城镇职工/城乡居民)
	SocialSecurityNo      string `json:"social_security_no,omitempty"`      // 社保卡号
	OutpatientNo          string `json:"outpatient_no,omitempty"`           // 门诊号
	AdmissionNo           string `json:"admission_no,omitempty"`            // 住院号
	
	// 费用明细
	TotalAmount           string `json:"total_amount,omitempty"`            // 总金额
	InsurancePay          string `json:"insurance_pay,omitempty"`           // 医保统筹支付
	PersonalAccountPay    string `json:"personal_account_pay,omitempty"`    // 个人账户支付
	PersonalCashPay       string `json:"personal_cash_pay,omitempty"`       // 个人现金支付
	PersonalSelfPay       string `json:"personal_self_pay,omitempty"`       // 个人自付
	OtherPay              string `json:"other_pay,omitempty"`               // 其他支付
	
	// 费用项目分类
	ExaminationFee        string `json:"examination_fee,omitempty"`         // 检查费
	TreatmentFee          string `json:"treatment_fee,omitempty"`           // 治疗费
	MedicineFee           string `json:"medicine_fee,omitempty"`            // 药品费
	MaterialFee           string `json:"material_fee,omitempty"`            // 材料费
	RegistrationFee       string `json:"registration_fee,omitempty"`        // 挂号费
	BedFee                string `json:"bed_fee,omitempty"`                 // 床位费
	NursingFee            string `json:"nursing_fee,omitempty"`             // 护理费
	SurgeryFee            string `json:"surgery_fee,omitempty"`             // 手术费
}

// VATInvoiceResult 增值税发票识别结果
type VATInvoiceResult struct {
	InvoiceResult
	
	// 发票基本信息
	InvoiceTitle          string `json:"invoice_title,omitempty"`           // 发票抬头
	MachineNo             string `json:"machine_no,omitempty"`              // 机器编号
	CheckCode             string `json:"check_code,omitempty"`              // 校验码
	
	// 销售方信息
	SellerName            string `json:"seller_name,omitempty"`             // 销售方名称
	SellerTaxNo           string `json:"seller_tax_no,omitempty"`           // 销售方纳税人识别号
	SellerAddress         string `json:"seller_address,omitempty"`          // 销售方地址电话
	SellerBank            string `json:"seller_bank,omitempty"`             // 销售方开户行及账号
	
	// 购买方信息
	BuyerName             string `json:"buyer_name,omitempty"`              // 购买方名称
	BuyerTaxNo            string `json:"buyer_tax_no,omitempty"`            // 购买方纳税人识别号
	BuyerAddress          string `json:"buyer_address,omitempty"`           // 购买方地址电话
	BuyerBank             string `json:"buyer_bank,omitempty"`              // 购买方开户行及账号
	
	// 金额信息
	AmountWithoutTax      string `json:"amount_without_tax,omitempty"`      // 不含税金额
	TaxAmount             string `json:"tax_amount,omitempty"`              // 税额
	AmountInWords         string `json:"amount_in_words,omitempty"`         // 价税合计(大写)
	AmountInFigures       string `json:"amount_in_figures,omitempty"`       // 价税合计(小写)
	
	// 其他信息
	TaxRate               string `json:"tax_rate,omitempty"`                // 税率
	Drawer                string `json:"drawer,omitempty"`                  // 开票人
	Reviewer              string `json:"reviewer,omitempty"`                // 复核人
	Payee                 string `json:"payee,omitempty"`                   // 收款人
	Remarks               string `json:"remarks,omitempty"`                 // 备注
}

// InvoiceTemplate 发票模板
type InvoiceTemplate struct {
	Type          InvoiceType
	Name          string
	Keywords      []string // 用于自动识别的关键词
	FieldPatterns map[string]*regexp.Regexp
}

// InvoiceTemplateManager 发票模板管理器
type InvoiceTemplateManager struct {
	templates map[InvoiceType]*InvoiceTemplate
}

// NewInvoiceTemplateManager 创建发票模板管理器
func NewInvoiceTemplateManager() *InvoiceTemplateManager {
	m := &InvoiceTemplateManager{
		templates: make(map[InvoiceType]*InvoiceTemplate),
	}
	m.initTemplates()
	return m
}

// initTemplates 初始化发票模板
func (m *InvoiceTemplateManager) initTemplates() {
	// 医疗发票模板
	m.templates[InvoiceTypeMedical] = &InvoiceTemplate{
		Type:     InvoiceTypeMedical,
		Name:     "医疗发票",
		Keywords: []string{"医疗", "门诊", "住院", "挂号", "医保", "统筹基金", "个人账户支付", "诊察", "医院", "患者"},
		FieldPatterns: map[string]*regexp.Regexp{
			"hospital_name":        regexp.MustCompile(`(?:医院名称|医疗机构)[：:]\s*(.+?)(?:\s|$)`),
			"patient_name":         regexp.MustCompile(`(?:患者姓名|姓名)[：:]\s*(.+?)(?:\s|$)`),
			"date":                 regexp.MustCompile(`(?:开票日期|日期|H\s*&\s*OP)[：:]?\s*(\d{4}[-/年]?\d{1,2}[-/月]?\d{1,2}日?)`),
			"total_amount":         regexp.MustCompile(`(?:合计|总[金额计]|V8\)?)[：:\s]*(\d+\.?\d*)`),
			"insurance_pay":        regexp.MustCompile(`(?:医保)?统筹[基金]*支付[：:t]?\s*(\d+\.?\d*)`),
			"personal_account":     regexp.MustCompile(`个人账户支付[：:;]?\s*(\d+\.?\d*)`),
			"personal_cash":        regexp.MustCompile(`(?:个人)?现金支付[：:]?\s*(\d+\.?\d*)`),
			"personal_self_pay":    regexp.MustCompile(`个人自[付竺][：:]?\s*(\d+\.?\d*)`),
			"other_pay":            regexp.MustCompile(`其他支付[：:]?\s*(\d+\.?\d*)`),
			"outpatient_no":        regexp.MustCompile(`门诊号[：:,]?\s*(\d+)`),
			"admission_no":         regexp.MustCompile(`住院号[：:]?\s*(\d+)`),
			"social_security_no":   regexp.MustCompile(`(?:社保卡号|医保卡号|PR|社会保障号)[（(（]?[^)）]*[)）]?[：:]\s*(\d+)`),
			"insurance_type":       regexp.MustCompile(`医保类型[：:]?\s*(.+?)(?:\s|$)`),
			"invoice_no":           regexp.MustCompile(`(?:发票号码|票据号码)[：:]?\s*(\d+)`),
			"invoice_code":         regexp.MustCompile(`(?:发票代码|票据代码)[：:]?\s*(\d+)`),
			"examination_fee":      regexp.MustCompile(`检查费[：:]?\s*(\d+\.?\d*)`),
			"treatment_fee":        regexp.MustCompile(`治疗费[：:]?\s*(\d+\.?\d*)`),
			"medicine_fee":         regexp.MustCompile(`(?:药品费|西药费|中药费)[：:]?\s*(\d+\.?\d*)`),
			"registration_fee":     regexp.MustCompile(`(?:挂号费|诊察费)[：:]?\s*(\d+\.?\d*)`),
		},
	}

	// 增值税发票模板
	m.templates[InvoiceTypeVAT] = &InvoiceTemplate{
		Type:     InvoiceTypeVAT,
		Name:     "增值税发票",
		Keywords: []string{"增值税", "专用发票", "普通发票", "电子发票", "纳税人识别号", "价税合计", "税率", "税额"},
		FieldPatterns: map[string]*regexp.Regexp{
			"invoice_code":       regexp.MustCompile(`(?:发票代码|标号码)[：:\-]?\s*(\d+)`),
			"invoice_no":         regexp.MustCompile(`(?:发票号码|标号码)[：:\-]?\s*[—\-]?(\d{15,24})`),
			"date":               regexp.MustCompile(`开票日期[：:"]?\s*(\d{4}年\d{1,2}月\d{1,2}日|\d{4}[-/]\d{2}[-/]\d{2})`),
			"check_code":         regexp.MustCompile(`校验码[：:]?\s*(\d+)`),
			"machine_no":         regexp.MustCompile(`机器编号[：:]?\s*(\d+)`),
			"buyer_name":         regexp.MustCompile(`(?:购[买方]|名\s*称)[|｜]?[：:]?\s*(.+?)(?:\s|统一|纳税|$)`),
			"buyer_tax_no":       regexp.MustCompile(`(?:购[买方])?(?:统一社会信用代码[/／])?纳税人识别号[：:]?\s*([A-Z0-9]{15,20})`),
			"seller_name":        regexp.MustCompile(`(?:销[售方]|名\s*称)[|｜]?[：:]?\s*(.+?)(?:\s|统一|纳税|$)`),
			"seller_tax_no":      regexp.MustCompile(`(?:销[售方])?(?:统一社会信用代码[/／])?纳税人识别号[：:]?\s*([A-Z0-9]{15,20})`),
			"amount_without_tax": regexp.MustCompile(`合\s*计\s*[¥￥]?\s*(\d+\.?\d*)`),
			"tax_amount":         regexp.MustCompile(`(?:税\s*[额领]|¥)\s*(\d+\.?\d*)\s*[}）\]]?$`),
			"total_amount":       regexp.MustCompile(`(?:价税合计|小写)[（(]?[小大写]*[)）]?\s*[¥￥]?\s*(\d+\.?\d*)`),
			"amount_in_words":    regexp.MustCompile(`(?:价税合计|大写)[（(]?大写[)）]?\s*(.+?)(?:小写|$|\s)`),
			"tax_rate":           regexp.MustCompile(`(\d+%)\s*(?:[¥￥]|\d)`),
			"drawer":             regexp.MustCompile(`(?:开票人|A)[：:]?\s*(.+?)(?:\s|复核|收款|$)`),
			"reviewer":           regexp.MustCompile(`复核[人]?[：:]?\s*(.+?)(?:\s|开票|收款|$)`),
			"payee":              regexp.MustCompile(`收款人[：:]?\s*(.+?)(?:\s|开票|复核|$)`),
			"remarks":            regexp.MustCompile(`备[注]?\s*[：:]?\s*(.+?)(?:$)`),
		},
	}

	// 普通发票模板
	m.templates[InvoiceTypeGeneral] = &InvoiceTemplate{
		Type:     InvoiceTypeGeneral,
		Name:     "普通发票",
		Keywords: []string{"发票", "收据", "金额", "合计"},
		FieldPatterns: map[string]*regexp.Regexp{
			"invoice_code":  regexp.MustCompile(`发票代码[：:]?\s*(\d+)`),
			"invoice_no":    regexp.MustCompile(`(?:发票号码|No|NO)[.：:]?\s*(\d+)`),
			"date":          regexp.MustCompile(`(?:日期|开票日期)[：:]?\s*(\d{4}[-/年]?\d{1,2}[-/月]?\d{1,2}日?)`),
			"total_amount":  regexp.MustCompile(`(?:合计|总计|金额|总金额)[：:]?\s*[¥￥]?(\d+\.?\d*)`),
			"seller_name":   regexp.MustCompile(`(?:销售方|收款单位|开票单位)[：:]?\s*(.+?)(?:\s|$)`),
			"buyer_name":    regexp.MustCompile(`(?:购买方|付款单位)[：:]?\s*(.+?)(?:\s|$)`),
		},
	}

	// 出租车发票模板
	m.templates[InvoiceTypeTaxi] = &InvoiceTemplate{
		Type:     InvoiceTypeTaxi,
		Name:     "出租车发票",
		Keywords: []string{"出租车", "出租汽车", "里程", "等候", "车牌"},
		FieldPatterns: map[string]*regexp.Regexp{
			"invoice_no":   regexp.MustCompile(`发票号码[：:]?\s*(\d+)`),
			"date":         regexp.MustCompile(`(?:日期|上车时间)[：:]?\s*(\d{4}[-/年]?\d{1,2}[-/月]?\d{1,2})`),
			"time":         regexp.MustCompile(`(?:上车时间|时间)[：:]?\s*(\d{1,2}:\d{2})`),
			"total_amount": regexp.MustCompile(`(?:金额|合计)[：:]?\s*[¥￥]?(\d+\.?\d*)`),
			"mileage":      regexp.MustCompile(`(?:里程|公里)[：:]?\s*(\d+\.?\d*)`),
			"wait_time":    regexp.MustCompile(`(?:等候时间|等候)[：:]?\s*(\d+)`),
			"plate_no":     regexp.MustCompile(`(?:车牌号|车号)[：:]?\s*([A-Z京津沪渝冀豫云辽黑湘皖鲁新苏浙赣鄂桂甘晋蒙陕吉闽贵粤青藏川宁琼][A-Z0-9]{5,6})`),
		},
	}

	// 火车票模板
	m.templates[InvoiceTypeTrain] = &InvoiceTemplate{
		Type:     InvoiceTypeTrain,
		Name:     "火车票",
		Keywords: []string{"火车", "铁路", "车次", "座位号", "检票口"},
		FieldPatterns: map[string]*regexp.Regexp{
			"date":          regexp.MustCompile(`(\d{4}年\d{1,2}月\d{1,2}日)`),
			"train_no":      regexp.MustCompile(`(?:车次)[：:]?\s*([A-Z]?\d+)`),
			"departure":     regexp.MustCompile(`(.+?)(?:站|→)`),
			"arrival":       regexp.MustCompile(`→\s*(.+?)(?:站|\s)`),
			"seat_no":       regexp.MustCompile(`(\d+车\d+[A-F]?号)`),
			"seat_type":     regexp.MustCompile(`(一等座|二等座|硬座|软座|硬卧|软卧|商务座)`),
			"total_amount":  regexp.MustCompile(`[¥￥](\d+\.?\d*)`),
			"passenger":     regexp.MustCompile(`(.{2,4})\s+\d{17}[0-9Xx]`),
			"id_number":     regexp.MustCompile(`(\d{17}[0-9Xx])`),
		},
	}

	// 机票行程单模板
	m.templates[InvoiceTypeFlight] = &InvoiceTemplate{
		Type:     InvoiceTypeFlight,
		Name:     "机票行程单",
		Keywords: []string{"航空", "航班", "机票", "行程单", "登机", "舱位"},
		FieldPatterns: map[string]*regexp.Regexp{
			"date":           regexp.MustCompile(`(\d{4}[-/年]\d{1,2}[-/月]\d{1,2}日?)`),
			"flight_no":      regexp.MustCompile(`(?:航班号?)[：:]?\s*([A-Z]{2}\d{3,4})`),
			"departure":      regexp.MustCompile(`(?:出发|始发)[：:]?\s*(.+?)(?:\s|→)`),
			"arrival":        regexp.MustCompile(`(?:到达|目的)[：:]?\s*(.+?)(?:\s|$)`),
			"departure_time": regexp.MustCompile(`(?:起飞|出发时间)[：:]?\s*(\d{2}:\d{2})`),
			"arrival_time":   regexp.MustCompile(`(?:到达时间)[：:]?\s*(\d{2}:\d{2})`),
			"seat_class":     regexp.MustCompile(`(经济舱|公务舱|头等舱)`),
			"total_amount":   regexp.MustCompile(`(?:票价|金额|合计)[：:]?\s*[¥￥CNY]?(\d+\.?\d*)`),
			"passenger":      regexp.MustCompile(`(?:旅客姓名|姓名)[：:]?\s*(.+?)(?:\s|$)`),
		},
	}
}

// DetectInvoiceType 自动检测发票类型
func (m *InvoiceTemplateManager) DetectInvoiceType(text string) InvoiceType {
	text = strings.ToLower(text)
	
	// 统计每种类型的关键词匹配数
	maxScore := 0
	bestType := InvoiceTypeGeneral
	
	for invoiceType, template := range m.templates {
		score := 0
		for _, keyword := range template.Keywords {
			if strings.Contains(text, strings.ToLower(keyword)) {
				score++
			}
		}
		if score > maxScore {
			maxScore = score
			bestType = invoiceType
		}
	}
	
	return bestType
}

// ExtractFields 从文本中提取字段
func (m *InvoiceTemplateManager) ExtractFields(text string, invoiceType InvoiceType) map[string]string {
	fields := make(map[string]string)
	
	template, ok := m.templates[invoiceType]
	if !ok {
		template = m.templates[InvoiceTypeGeneral]
	}
	
	for fieldName, pattern := range template.FieldPatterns {
		matches := pattern.FindStringSubmatch(text)
		if len(matches) > 1 {
			value := strings.TrimSpace(matches[1])
			if value != "" {
				fields[fieldName] = value
			}
		}
	}
	
	return fields
}

// ParseMedicalInvoice 解析医疗发票
func (m *InvoiceTemplateManager) ParseMedicalInvoice(ocrResult *OCRResult) *MedicalInvoiceResult {
	result := &MedicalInvoiceResult{
		InvoiceResult: InvoiceResult{
			OCRResult:   *ocrResult,
			InvoiceType: string(InvoiceTypeMedical),
		},
	}
	
	text := ocrResult.FullText
	fields := m.ExtractFields(text, InvoiceTypeMedical)
	
	// 填充字段
	result.HospitalName = fields["hospital_name"]
	result.PatientName = fields["patient_name"]
	result.Date = fields["date"]
	result.TotalAmount = normalizeAmount(fields["total_amount"])
	result.InsurancePay = normalizeAmount(fields["insurance_pay"])
	result.PersonalAccountPay = normalizeAmount(fields["personal_account"])
	result.PersonalCashPay = normalizeAmount(fields["personal_cash"])
	result.PersonalSelfPay = normalizeAmount(fields["personal_self_pay"])
	result.OtherPay = normalizeAmount(fields["other_pay"])
	result.OutpatientNo = fields["outpatient_no"]
	result.AdmissionNo = fields["admission_no"]
	result.SocialSecurityNo = fields["social_security_no"]
	result.InvoiceNo = fields["invoice_no"]
	result.InvoiceCode = fields["invoice_code"]
	result.ExaminationFee = normalizeAmount(fields["examination_fee"])
	result.TreatmentFee = normalizeAmount(fields["treatment_fee"])
	result.MedicineFee = normalizeAmount(fields["medicine_fee"])
	result.RegistrationFee = normalizeAmount(fields["registration_fee"])
	
	// 检测医保类型
	if strings.Contains(text, "城镇职工") {
		result.InsuranceType = "城镇职工医保"
	} else if strings.Contains(text, "城乡居民") {
		result.InsuranceType = "城乡居民医保"
	} else if strings.Contains(text, "新农合") {
		result.InsuranceType = "新农合"
	}
	
	// 检测患者类型
	if strings.Contains(text, "门诊") {
		result.PatientType = "门诊"
	} else if strings.Contains(text, "住院") {
		result.PatientType = "住院"
	}
	
	return result
}

// ParseVATInvoice 解析增值税发票
func (m *InvoiceTemplateManager) ParseVATInvoice(ocrResult *OCRResult) *VATInvoiceResult {
	result := &VATInvoiceResult{
		InvoiceResult: InvoiceResult{
			OCRResult: *ocrResult,
		},
	}
	
	text := ocrResult.FullText
	fields := m.ExtractFields(text, InvoiceTypeVAT)
	
	// 检测发票类型
	if strings.Contains(text, "专用发票") {
		result.InvoiceType = string(InvoiceTypeVATSpecial)
	} else if strings.Contains(text, "电子") || strings.Contains(text, "标号码") {
		result.InvoiceType = string(InvoiceTypeVATElectronic)
	} else {
		result.InvoiceType = string(InvoiceTypeVATNormal)
	}
	
	// 填充字段
	result.InvoiceCode = fields["invoice_code"]
	result.InvoiceNo = fields["invoice_no"]
	result.Date = fields["date"]
	result.CheckCode = fields["check_code"]
	result.MachineNo = fields["machine_no"]
	result.BuyerName = cleanFieldValue(fields["buyer_name"])
	result.BuyerTaxNo = fields["buyer_tax_no"]
	result.SellerName = cleanFieldValue(fields["seller_name"])
	result.SellerTaxNo = fields["seller_tax_no"]
	result.AmountWithoutTax = normalizeAmount(fields["amount_without_tax"])
	result.TaxAmount = normalizeAmount(fields["tax_amount"])
	result.TotalAmount = normalizeAmount(fields["total_amount"])
	result.AmountInWords = fields["amount_in_words"]
	result.AmountInFigures = normalizeAmount(fields["total_amount"])
	result.TaxRate = fields["tax_rate"]
	result.Drawer = cleanFieldValue(fields["drawer"])
	result.Reviewer = cleanFieldValue(fields["reviewer"])
	result.Payee = cleanFieldValue(fields["payee"])
	result.Remarks = fields["remarks"]
	
	// 从文本中尝试提取更多信息 (使用备用正则)
	if result.BuyerName == "" {
		buyerNameRe := regexp.MustCompile(`购[买方]?\s*[|｜]?\s*名称[：:]?\s*(.+?)(?:\s{2,}|$)`)
		if matches := buyerNameRe.FindStringSubmatch(text); len(matches) > 1 {
			result.BuyerName = cleanFieldValue(matches[1])
		}
	}
	
	if result.SellerName == "" {
		sellerNameRe := regexp.MustCompile(`销[售方]?\s*[|｜]?\s*名称[：:]?\s*(.+?)(?:\s{2,}|$)`)
		if matches := sellerNameRe.FindStringSubmatch(text); len(matches) > 1 {
			result.SellerName = cleanFieldValue(matches[1])
		}
	}
	
	return result
}

// cleanFieldValue 清理字段值
func cleanFieldValue(value string) string {
	if value == "" {
		return ""
	}
	// 移除多余的空格
	value = strings.TrimSpace(value)
	// 移除控制字符
	value = strings.Map(func(r rune) rune {
		if r < 32 {
			return -1
		}
		return r
	}, value)
	return value
}

// ParseGeneralInvoice 解析普通发票
func (m *InvoiceTemplateManager) ParseGeneralInvoice(ocrResult *OCRResult) *InvoiceResult {
	result := &InvoiceResult{
		OCRResult:   *ocrResult,
		InvoiceType: string(InvoiceTypeGeneral),
	}
	
	text := ocrResult.FullText
	fields := m.ExtractFields(text, InvoiceTypeGeneral)
	
	result.InvoiceCode = fields["invoice_code"]
	result.InvoiceNo = fields["invoice_no"]
	result.Date = fields["date"]
	result.TotalAmount = normalizeAmount(fields["total_amount"])
	result.SellerName = fields["seller_name"]
	result.BuyerName = fields["buyer_name"]
	
	return result
}

// ParseInvoice 自动识别并解析发票
func (m *InvoiceTemplateManager) ParseInvoice(ocrResult *OCRResult, specifiedType InvoiceType) interface{} {
	invoiceType := specifiedType
	if invoiceType == InvoiceTypeAuto || invoiceType == "" {
		invoiceType = m.DetectInvoiceType(ocrResult.FullText)
	}
	
	switch invoiceType {
	case InvoiceTypeMedical:
		return m.ParseMedicalInvoice(ocrResult)
	case InvoiceTypeVAT, InvoiceTypeVATSpecial, InvoiceTypeVATNormal, InvoiceTypeVATElectronic:
		return m.ParseVATInvoice(ocrResult)
	default:
		return m.ParseGeneralInvoice(ocrResult)
	}
}

// GetTemplate 获取模板
func (m *InvoiceTemplateManager) GetTemplate(invoiceType InvoiceType) *InvoiceTemplate {
	return m.templates[invoiceType]
}

// GetAllTemplates 获取所有模板
func (m *InvoiceTemplateManager) GetAllTemplates() map[InvoiceType]*InvoiceTemplate {
	return m.templates
}

// normalizeAmount 标准化金额格式
func normalizeAmount(amount string) string {
	if amount == "" {
		return ""
	}
	
	// 移除货币符号
	amount = strings.TrimLeft(amount, "¥￥$CNY ")
	
	// 移除空格
	amount = strings.ReplaceAll(amount, " ", "")
	
	// 确保小数点格式正确
	if !strings.Contains(amount, ".") && len(amount) > 2 {
		// 如果没有小数点且长度大于2，假设最后两位是小数
		// 这只是一个启发式处理，实际情况可能需要更复杂的逻辑
	}
	
	return amount
}

// GetInvoiceTypeDescription 获取发票类型描述
func GetInvoiceTypeDescription(t InvoiceType) string {
	descriptions := map[InvoiceType]string{
		InvoiceTypeAuto:          "自动识别",
		InvoiceTypeMedical:       "医疗发票",
		InvoiceTypeVAT:           "增值税发票",
		InvoiceTypeVATSpecial:    "增值税专用发票",
		InvoiceTypeVATNormal:     "增值税普通发票",
		InvoiceTypeVATElectronic: "增值税电子发票",
		InvoiceTypeTaxi:          "出租车发票",
		InvoiceTypeTrain:         "火车票",
		InvoiceTypeFlight:        "机票行程单",
		InvoiceTypeQuota:         "定额发票",
		InvoiceTypeRollTicket:    "过路费发票",
		InvoiceTypeReceipt:       "收据",
		InvoiceTypeGeneral:       "普通发票",
	}
	
	if desc, ok := descriptions[t]; ok {
		return desc
	}
	return string(t)
}
