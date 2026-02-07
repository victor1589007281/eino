// Package ocr OCR 工具类型定义
package ocr

import (
	"context"
	"time"
)

// SceneType 场景类型
type SceneType string

const (
	SceneGeneral     SceneType = "general"     // 通用场景
	SceneDocument    SceneType = "document"    // 文档
	SceneTable       SceneType = "table"       // 表格
	SceneInvoice     SceneType = "invoice"     // 票据
	SceneIDCard      SceneType = "id_card"     // 证件
	SceneHandwriting SceneType = "handwriting" // 手写
	ScenePrint       SceneType = "print"       // 印刷
)

// EngineType 引擎类型
type EngineType string

const (
	EngineRapidOCR   EngineType = "rapidocr"   // RapidOCR (CPU优化)
	EnginePaddleOCR  EngineType = "paddleocr"  // PaddleOCR
	EngineTesseract  EngineType = "tesseract"  // Tesseract
	EngineBaidu      EngineType = "baidu"      // 百度云 OCR
	EngineTencent    EngineType = "tencent"    // 腾讯云 OCR
	EngineQwenVL     EngineType = "qwen_vl"    // 通义千问 VL
	EngineGPT4V      EngineType = "gpt4v"      // OpenAI GPT-4V
	EngineClaude     EngineType = "claude"     // Claude Vision
)

// EngineLayer 引擎层级
type EngineLayer int

const (
	LayerTraditional EngineLayer = 1 // L1 传统OCR (Tesseract)
	LayerSmallModel  EngineLayer = 2 // L2 小模型 (RapidOCR, PaddleOCR)
	LayerVLM         EngineLayer = 3 // L3 视觉大模型 (GPT-4V, Claude)
)

// RoutingStrategy 路由策略
type RoutingStrategy string

const (
	StrategyQualityFirst RoutingStrategy = "quality_first" // 质量优先
	StrategyCostFirst    RoutingStrategy = "cost_first"    // 成本优先
	StrategySpeedFirst   RoutingStrategy = "speed_first"   // 速度优先
	StrategySmart        RoutingStrategy = "smart"         // 智能路由

	// 简写别名
	StrategyQuality RoutingStrategy = "quality" // 质量优先
	StrategyCost    RoutingStrategy = "cost"    // 成本优先
	StrategySpeed   RoutingStrategy = "speed"   // 速度优先
)

// OCRRequest OCR 请求
type OCRRequest struct {
	// 输入源 (三选一)
	ImagePath   string `json:"image_path,omitempty"`   // 本地图片路径
	ImageURL    string `json:"image_url,omitempty"`    // 图片URL
	ImageBase64 string `json:"image_base64,omitempty"` // Base64编码

	// 识别参数
	Scene     SceneType       `json:"scene,omitempty"`    // 场景类型
	Languages []string        `json:"languages,omitempty"` // 识别语言列表 ["zh", "en"]
	Strategy  RoutingStrategy `json:"strategy,omitempty"` // 路由策略

	// 可选参数
	DetectAngle      bool `json:"detect_angle,omitempty"`      // 是否检测旋转角度
	AutoPreprocess   bool `json:"auto_preprocess,omitempty"`   // 是否自动预处理
	ReturnConfidence bool `json:"return_confidence,omitempty"` // 是否返回置信度
	ReturnPosition   bool `json:"return_position,omitempty"`   // 是否返回位置信息

	// 高级参数
	SpecifiedEngine EngineType `json:"specified_engine,omitempty"` // 指定引擎 (跳过路由)
	Timeout         int        `json:"timeout,omitempty"`          // 超时时间(秒)
}

// OCRResult OCR 识别结果
type OCRResult struct {
	Success    bool        `json:"success"`
	FullText   string      `json:"full_text"`              // 完整识别文本
	Blocks     []TextBlock `json:"blocks,omitempty"`       // 文本块列表
	Engine     EngineType  `json:"engine"`                 // 使用的引擎
	Latency    int64       `json:"latency_ms"`             // 处理耗时(ms)
	Confidence float64     `json:"confidence,omitempty"`   // 整体置信度
	Error      string      `json:"error,omitempty"`        // 错误信息
	Metadata   *OCRMeta    `json:"metadata,omitempty"`     // 元数据
}

// TextBlock 文本块
type TextBlock struct {
	Text       string     `json:"text"`                 // 文本内容
	Confidence float64    `json:"confidence,omitempty"` // 置信度 0-1
	Box        *BoundingBox `json:"box,omitempty"`       // 边界框
	BlockType  string     `json:"block_type,omitempty"` // 类型: text, title, table, figure
}

// BoundingBox 边界框
type BoundingBox struct {
	X      int `json:"x"`      // 左上角X
	Y      int `json:"y"`      // 左上角Y
	Width  int `json:"width"`  // 宽度
	Height int `json:"height"` // 高度
}

// OCRMeta 元数据
type OCRMeta struct {
	ImageWidth  int       `json:"image_width,omitempty"`  // 图片宽度
	ImageHeight int       `json:"image_height,omitempty"` // 图片高度
	Angle       float64   `json:"angle,omitempty"`        // 旋转角度
	PageCount   int       `json:"page_count,omitempty"`   // 页数 (PDF)
	ProcessTime time.Time `json:"process_time"`           // 处理时间
}

// TableResult 表格识别结果
type TableResult struct {
	OCRResult
	Tables []Table `json:"tables,omitempty"` // 表格列表
}

// Table 表格
type Table struct {
	Rows    [][]string `json:"rows"`               // 行数据
	Headers []string   `json:"headers,omitempty"`  // 表头
	HTML    string     `json:"html,omitempty"`     // HTML格式
	CSV     string     `json:"csv,omitempty"`      // CSV格式
}

// InvoiceResult 票据识别结果
type InvoiceResult struct {
	OCRResult
	InvoiceType   string            `json:"invoice_type,omitempty"`   // 票据类型
	InvoiceNo     string            `json:"invoice_no,omitempty"`     // 发票号码
	InvoiceCode   string            `json:"invoice_code,omitempty"`   // 发票代码
	Date          string            `json:"date,omitempty"`           // 开票日期
	TotalAmount   string            `json:"total_amount,omitempty"`   // 总金额
	TaxAmount     string            `json:"tax_amount,omitempty"`     // 税额
	SellerName    string            `json:"seller_name,omitempty"`    // 销售方名称
	BuyerName     string            `json:"buyer_name,omitempty"`     // 购买方名称
	Items         []InvoiceItem     `json:"items,omitempty"`          // 商品明细
	ExtraFields   map[string]string `json:"extra_fields,omitempty"`   // 额外字段
}

// InvoiceItem 发票项目
type InvoiceItem struct {
	Name     string `json:"name"`               // 商品名称
	Quantity string `json:"quantity,omitempty"` // 数量
	Unit     string `json:"unit,omitempty"`     // 单位
	Price    string `json:"price,omitempty"`    // 单价
	Amount   string `json:"amount,omitempty"`   // 金额
}

// IDCardResult 证件识别结果
type IDCardResult struct {
	OCRResult
	CardType    string            `json:"card_type,omitempty"`    // 证件类型
	Name        string            `json:"name,omitempty"`         // 姓名
	IDNumber    string            `json:"id_number,omitempty"`    // 证件号码
	Gender      string            `json:"gender,omitempty"`       // 性别
	Ethnicity   string            `json:"ethnicity,omitempty"`    // 民族
	Birthday    string            `json:"birthday,omitempty"`     // 出生日期
	Address     string            `json:"address,omitempty"`      // 地址
	IssueDate   string            `json:"issue_date,omitempty"`   // 签发日期
	ExpiryDate  string            `json:"expiry_date,omitempty"`  // 有效期至
	Authority   string            `json:"authority,omitempty"`    // 签发机关
	ExtraFields map[string]string `json:"extra_fields,omitempty"` // 额外字段
}

// DocumentResult 文档识别结果
type DocumentResult struct {
	OCRResult
	Title      string       `json:"title,omitempty"`      // 文档标题
	Paragraphs []string     `json:"paragraphs,omitempty"` // 段落列表
	Layout     *LayoutInfo  `json:"layout,omitempty"`     // 版面信息
}

// LayoutInfo 版面信息
type LayoutInfo struct {
	Columns   int          `json:"columns,omitempty"`   // 栏数
	HasHeader bool         `json:"has_header,omitempty"`// 是否有页眉
	HasFooter bool         `json:"has_footer,omitempty"`// 是否有页脚
	Regions   []LayoutRegion `json:"regions,omitempty"` // 区域列表
}

// LayoutRegion 版面区域
type LayoutRegion struct {
	Type string      `json:"type"` // text, title, table, figure, list
	Box  *BoundingBox `json:"box"`
	Text string      `json:"text,omitempty"`
}

// Engine OCR 引擎接口
type Engine interface {
	// Name 返回引擎名称
	Name() EngineType

	// Layer 返回引擎层级
	Layer() EngineLayer

	// SupportedScenes 支持的场景
	SupportedScenes() []SceneType

	// Recognize 执行 OCR 识别
	Recognize(ctx context.Context, req *OCRRequest) (*OCRResult, error)

	// RecognizeTable 表格识别 (可选实现)
	RecognizeTable(ctx context.Context, req *OCRRequest) (*TableResult, error)

	// RecognizeInvoice 票据识别 (可选实现)
	RecognizeInvoice(ctx context.Context, req *OCRRequest) (*InvoiceResult, error)

	// RecognizeIDCard 证件识别 (可选实现)
	RecognizeIDCard(ctx context.Context, req *OCRRequest) (*IDCardResult, error)

	// MemoryUsage 返回当前内存占用 (MB)
	MemoryUsage() int

	// IsLoaded 是否已加载
	IsLoaded() bool

	// Load 加载引擎
	Load(ctx context.Context) error

	// Unload 卸载引擎
	Unload() error

	// Health 健康检查
	Health(ctx context.Context) error

	// Stats 获取统计信息
	Stats() EngineStatus
}

// ImageQuality 图片质量信息
type ImageQuality struct {
	Score      float64 `json:"score"`       // 综合评分 0-1
	Clarity    float64 `json:"clarity"`     // 清晰度 0-1
	Contrast   float64 `json:"contrast"`    // 对比度 0-1
	Brightness float64 `json:"brightness"`  // 亮度 0-1
	Sharpness  float64 `json:"sharpness"`   // 清晰度 0-1 (与 Clarity 相同)
	NoiseLevel float64 `json:"noise_level"` // 噪声级别 0-1
	HasShadow  bool    `json:"has_shadow"`  // 是否有阴影
	IsSkewed   bool    `json:"is_skewed"`   // 是否倾斜
	SkewAngle  float64 `json:"skew_angle"`  // 倾斜角度
}

// EngineStatus 引擎状态
type EngineStatus struct {
	Engine      EngineType `json:"engine"`
	Available   bool       `json:"available"`
	Loaded      bool       `json:"loaded"`
	MemoryMB    int        `json:"memory_mb"`
	LastUsed    time.Time  `json:"last_used,omitempty"`
	TotalCalls  int64      `json:"total_calls"`
	FailedCalls int64      `json:"failed_calls"`
	AvgLatency  float64    `json:"avg_latency_ms"`
}

// Config OCR 工具配置
type Config struct {
	// 内存管理
	MaxMemoryMB       int           `json:"max_memory_mb"`       // 最大内存使用 (MB)
	IdleTimeout       time.Duration `json:"idle_timeout"`        // 空闲超时后卸载
	ResidentEngines   []EngineType  `json:"resident_engines"`    // 常驻引擎
	
	// CPU 优化
	NumThreads        int  `json:"num_threads"`         // 线程数
	EnableMKLDNN      bool `json:"enable_mkl_dnn"`      // MKL-DNN 加速
	UseMemoryPool     bool `json:"use_memory_pool"`     // 使用内存池
	
	// 引擎配置
	TesseractPath     string `json:"tesseract_path,omitempty"`
	TesseractLang     string `json:"tesseract_lang,omitempty"`
	RapidOCREndpoint  string `json:"rapidocr_endpoint,omitempty"`
	PaddleOCREndpoint string `json:"paddleocr_endpoint,omitempty"`
	
	// 云服务配置
	BaiduAPIKey       string `json:"baidu_api_key,omitempty"`
	BaiduSecretKey    string `json:"baidu_secret_key,omitempty"`
	TencentSecretID   string `json:"tencent_secret_id,omitempty"`
	TencentSecretKey  string `json:"tencent_secret_key,omitempty"`
	
	// VLM 配置
	QwenAPIKey        string `json:"qwen_api_key,omitempty"`
	OpenAIAPIKey      string `json:"openai_api_key,omitempty"`
	ClaudeAPIKey      string `json:"claude_api_key,omitempty"`
	
	// 路由配置
	DefaultStrategy   RoutingStrategy `json:"default_strategy"`
	FallbackChain     []EngineType    `json:"fallback_chain"`
	
	// 配额配置
	DailyQuota        map[EngineType]int `json:"daily_quota,omitempty"`
	MonthlyQuota      map[EngineType]int `json:"monthly_quota,omitempty"`
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		MaxMemoryMB:     1500,
		IdleTimeout:     5 * time.Minute,
		ResidentEngines: []EngineType{EngineRapidOCR},
		NumThreads:      4,
		EnableMKLDNN:    false,
		UseMemoryPool:   true,
		TesseractPath:   "tesseract",
		TesseractLang:   "chi_sim+eng",
		DefaultStrategy: StrategySmart,
		FallbackChain:   []EngineType{EngineRapidOCR, EngineTesseract, EngineBaidu},
	}
}
