// Package config provides configuration management for OCR Agent.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the main configuration structure.
type Config struct {
	Server    ServerConfig    `yaml:"server"`
	LLM       LLMConfig       `yaml:"llm"`
	OCR       OCRConfig       `yaml:"ocr"`
	Storage   StorageConfig   `yaml:"storage"`
	Cache     CacheConfig     `yaml:"cache"`
	Output    OutputConfig    `yaml:"output"`
	Logging   LoggingConfig   `yaml:"logging"`
	Telemetry TelemetryConfig `yaml:"telemetry"`
}

// ServerConfig represents server configuration.
type ServerConfig struct {
	Host         string        `yaml:"host"`
	Port         int           `yaml:"port"`
	ReadTimeout  time.Duration `yaml:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout"`
	MaxBodySize  int64         `yaml:"max_body_size"` // in bytes
}

// LLMConfig represents LLM provider configuration.
type LLMConfig struct {
	DefaultProvider string                     `yaml:"default_provider"`
	Providers       map[string]*ProviderConfig `yaml:"providers"`
	Temperature     float64                    `yaml:"temperature"`
	MaxTokens       int                        `yaml:"max_tokens"`
	RetryConfig     RetryConfig                `yaml:"retry"`
}

// ProviderConfig represents a specific LLM provider configuration.
type ProviderConfig struct {
	Type      string  `yaml:"type"`      // openai, anthropic, zhipu, deepseek, ollama, qwen
	Enabled   bool    `yaml:"enabled"`
	APIKey    string  `yaml:"api_key"`
	BaseURL   string  `yaml:"base_url"`
	Model     string  `yaml:"model"`
	MaxTokens int     `yaml:"max_tokens"`
	Timeout   time.Duration `yaml:"timeout"`
	RateLimit int     `yaml:"rate_limit"` // requests per minute
}

// OCRConfig represents OCR engine configuration.
type OCRConfig struct {
	Engine          string                `yaml:"engine"`           // tesseract, paddleocr, api
	Language        string                `yaml:"language"`         // chi_sim, eng, chi_sim+eng
	TesseractPath   string                `yaml:"tesseract_path"`
	PaddleModelPath string                `yaml:"paddle_model_path"`
	APIProviders    map[string]*OCRAPIConfig `yaml:"api_providers"`
	ImageConfig     ImageConfig           `yaml:"image"`
	PDFConfig       PDFConfig             `yaml:"pdf"`
	InvoiceConfig   InvoiceConfig         `yaml:"invoice"`
	MaxConcurrency  int                   `yaml:"max_concurrency"`
	Timeout         time.Duration         `yaml:"timeout"`
}

// OCRAPIConfig represents API-based OCR provider configuration.
type OCRAPIConfig struct {
	Enabled   bool   `yaml:"enabled"`
	Provider  string `yaml:"provider"` // baidu, tencent, aliyun, azure
	APIKey    string `yaml:"api_key"`
	SecretKey string `yaml:"secret_key"`
	Endpoint  string `yaml:"endpoint"`
	Region    string `yaml:"region"`
}

// ImageConfig represents image processing configuration.
type ImageConfig struct {
	MaxWidth       int      `yaml:"max_width"`
	MaxHeight      int      `yaml:"max_height"`
	MaxFileSize    int64    `yaml:"max_file_size"`   // in bytes
	SupportedTypes []string `yaml:"supported_types"` // jpg, png, gif, bmp, tiff
	Preprocessing  bool     `yaml:"preprocessing"`   // enable image preprocessing
	DPI            int      `yaml:"dpi"`
}

// PDFConfig represents PDF processing configuration.
type PDFConfig struct {
	MaxPages       int     `yaml:"max_pages"`
	MaxFileSize    int64   `yaml:"max_file_size"` // in bytes
	DPI            int     `yaml:"dpi"`
	ExtractImages  bool    `yaml:"extract_images"`
	ExtractText    bool    `yaml:"extract_text"`
	OCRScannedPDF  bool    `yaml:"ocr_scanned_pdf"`
	PageRange      string  `yaml:"page_range"` // e.g., "1-5", "all"
}

// InvoiceConfig represents invoice recognition configuration.
type InvoiceConfig struct {
	Types       []string `yaml:"types"`       // vat_invoice, receipt, custom
	AutoClassify bool    `yaml:"auto_classify"`
	ExtractFields []string `yaml:"extract_fields"` // invoice_code, date, amount, etc.
	Validate      bool     `yaml:"validate"`       // validate invoice authenticity
}

// StorageConfig represents storage configuration.
type StorageConfig struct {
	Type       string `yaml:"type"` // sqlite, mysql, postgresql
	DSN        string `yaml:"dsn"`
	DataDir    string `yaml:"data_dir"`
	MaxOpenConns int  `yaml:"max_open_conns"`
	MaxIdleConns int  `yaml:"max_idle_conns"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime"`
}

// CacheConfig represents cache configuration.
type CacheConfig struct {
	Enabled     bool          `yaml:"enabled"`
	Type        string        `yaml:"type"` // memory, redis
	L1Size      int           `yaml:"l1_size"`
	L2Size      int           `yaml:"l2_size"`
	TTL         time.Duration `yaml:"ttl"`
	RedisConfig *RedisConfig  `yaml:"redis"`
}

// RedisConfig represents Redis cache configuration.
type RedisConfig struct {
	Addresses  []string `yaml:"addresses"`
	Password   string   `yaml:"password"`
	DB         int      `yaml:"db"`
	PoolSize   int      `yaml:"pool_size"`
}

// OutputConfig represents output formatting configuration.
type OutputConfig struct {
	Format        string `yaml:"format"`    // json, markdown, html
	IncludeRaw    bool   `yaml:"include_raw"`
	IncludeImages bool   `yaml:"include_images"`
	OutputDir     string `yaml:"output_dir"`
	Template      string `yaml:"template"`
}

// LoggingConfig represents logging configuration.
type LoggingConfig struct {
	Level      string `yaml:"level"` // debug, info, warn, error
	Format     string `yaml:"format"` // json, text
	Output     string `yaml:"output"` // stdout, file
	FilePath   string `yaml:"file_path"`
	MaxSize    int    `yaml:"max_size"`    // MB
	MaxBackups int    `yaml:"max_backups"`
	MaxAge     int    `yaml:"max_age"`     // days
	Compress   bool   `yaml:"compress"`
}

// TelemetryConfig represents telemetry configuration.
type TelemetryConfig struct {
	Enabled     bool   `yaml:"enabled"`
	MetricsPort int    `yaml:"metrics_port"`
	TracingType string `yaml:"tracing_type"` // jaeger, zipkin
	TracingURL  string `yaml:"tracing_url"`
}

// RetryConfig represents retry configuration.
type RetryConfig struct {
	MaxRetries int           `yaml:"max_retries"`
	InitialDelay time.Duration `yaml:"initial_delay"`
	MaxDelay     time.Duration `yaml:"max_delay"`
	Multiplier   float64       `yaml:"multiplier"`
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Host:         "0.0.0.0",
			Port:         8080,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 60 * time.Second,
			MaxBodySize:  100 * 1024 * 1024, // 100MB
		},
		LLM: LLMConfig{
			DefaultProvider: "deepseek",
			Temperature:     0.1,
			MaxTokens:       4096,
			Providers: map[string]*ProviderConfig{
				"deepseek": {
					Type:      "deepseek",
					Enabled:   true,
					Model:     "deepseek-chat",
					MaxTokens: 4096,
					Timeout:   60 * time.Second,
					RateLimit: 100,
				},
				"qwen": {
					Type:      "qwen",
					Enabled:   false,
					Model:     "qwen-max",
					MaxTokens: 8192,
					Timeout:   60 * time.Second,
					RateLimit: 50,
				},
			},
			RetryConfig: RetryConfig{
				MaxRetries:   3,
				InitialDelay: time.Second,
				MaxDelay:     30 * time.Second,
				Multiplier:   2.0,
			},
		},
		OCR: OCRConfig{
			Engine:         "tesseract",
			Language:       "chi_sim+eng",
			TesseractPath:  "/usr/bin/tesseract",
			MaxConcurrency: 4,
			Timeout:        120 * time.Second,
			ImageConfig: ImageConfig{
				MaxWidth:       4096,
				MaxHeight:      4096,
				MaxFileSize:    50 * 1024 * 1024, // 50MB
				SupportedTypes: []string{"jpg", "jpeg", "png", "gif", "bmp", "tiff", "webp"},
				Preprocessing:  true,
				DPI:            300,
			},
			PDFConfig: PDFConfig{
				MaxPages:      100,
				MaxFileSize:   100 * 1024 * 1024, // 100MB
				DPI:           300,
				ExtractImages: true,
				ExtractText:   true,
				OCRScannedPDF: true,
				PageRange:     "all",
			},
			InvoiceConfig: InvoiceConfig{
				Types: []string{
					"vat_invoice",          // 增值税发票
					"vat_special_invoice",  // 增值税专用发票
					"receipt",              // 收据
					"taxi_receipt",         // 出租车发票
					"train_ticket",         // 火车票
					"air_ticket",           // 机票
					"hotel_invoice",        // 酒店发票
					"toll_invoice",         // 过路费发票
				},
				AutoClassify: true,
				ExtractFields: []string{
					"invoice_code",      // 发票代码
					"invoice_number",    // 发票号码
					"invoice_date",      // 开票日期
					"check_code",        // 校验码
					"buyer_name",        // 购买方名称
					"buyer_tax_id",      // 购买方纳税人识别号
					"seller_name",       // 销售方名称
					"seller_tax_id",     // 销售方纳税人识别号
					"amount",            // 金额
					"tax_amount",        // 税额
					"total_amount",      // 价税合计
					"items",             // 商品明细
				},
				Validate: false,
			},
			APIProviders: map[string]*OCRAPIConfig{
				"baidu": {
					Enabled:  false,
					Provider: "baidu",
					Endpoint: "https://aip.baidubce.com",
				},
				"tencent": {
					Enabled:  false,
					Provider: "tencent",
				},
				"aliyun": {
					Enabled:  false,
					Provider: "aliyun",
				},
			},
		},
		Storage: StorageConfig{
			Type:            "sqlite",
			DataDir:         "./data",
			MaxOpenConns:    10,
			MaxIdleConns:    5,
			ConnMaxLifetime: 30 * time.Minute,
		},
		Cache: CacheConfig{
			Enabled: true,
			Type:    "memory",
			L1Size:  100,
			L2Size:  1000,
			TTL:     time.Hour,
		},
		Output: OutputConfig{
			Format:        "json",
			IncludeRaw:    false,
			IncludeImages: false,
			OutputDir:     "./output",
		},
		Logging: LoggingConfig{
			Level:      "info",
			Format:     "json",
			Output:     "stdout",
			MaxSize:    100,
			MaxBackups: 3,
			MaxAge:     7,
			Compress:   true,
		},
		Telemetry: TelemetryConfig{
			Enabled:     false,
			MetricsPort: 9090,
		},
	}
}

// LoadConfig loads configuration from a file.
func LoadConfig(path string) (*Config, error) {
	config := DefaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Load environment variables
	config.loadEnvOverrides()

	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return config, nil
}

// LoadConfigFromEnv loads configuration from environment variables.
func LoadConfigFromEnv() (*Config, error) {
	config := DefaultConfig()
	config.loadEnvOverrides()

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return config, nil
}

// loadEnvOverrides loads configuration overrides from environment variables.
func (c *Config) loadEnvOverrides() {
	// LLM API Keys
	if key := os.Getenv("DEEPSEEK_API_KEY"); key != "" {
		if p, ok := c.LLM.Providers["deepseek"]; ok {
			p.APIKey = key
		}
	}
	if key := os.Getenv("QWEN_API_KEY"); key != "" {
		if p, ok := c.LLM.Providers["qwen"]; ok {
			p.APIKey = key
		}
	}
	if key := os.Getenv("OPENAI_API_KEY"); key != "" {
		if p, ok := c.LLM.Providers["openai"]; ok {
			p.APIKey = key
		}
	}
	if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
		if p, ok := c.LLM.Providers["anthropic"]; ok {
			p.APIKey = key
		}
	}

	// OCR API Keys
	if key := os.Getenv("BAIDU_OCR_API_KEY"); key != "" {
		if p, ok := c.OCR.APIProviders["baidu"]; ok {
			p.APIKey = key
			p.Enabled = true
		}
	}
	if key := os.Getenv("BAIDU_OCR_SECRET_KEY"); key != "" {
		if p, ok := c.OCR.APIProviders["baidu"]; ok {
			p.SecretKey = key
		}
	}
	if key := os.Getenv("TENCENT_OCR_SECRET_ID"); key != "" {
		if p, ok := c.OCR.APIProviders["tencent"]; ok {
			p.APIKey = key
			p.Enabled = true
		}
	}
	if key := os.Getenv("TENCENT_OCR_SECRET_KEY"); key != "" {
		if p, ok := c.OCR.APIProviders["tencent"]; ok {
			p.SecretKey = key
		}
	}

	// Storage
	if dsn := os.Getenv("DATABASE_URL"); dsn != "" {
		c.Storage.DSN = dsn
	}
	if dataDir := os.Getenv("DATA_DIR"); dataDir != "" {
		c.Storage.DataDir = dataDir
	}

	// Cache Redis
	if addr := os.Getenv("REDIS_ADDR"); addr != "" {
		if c.Cache.RedisConfig == nil {
			c.Cache.RedisConfig = &RedisConfig{}
		}
		c.Cache.RedisConfig.Addresses = strings.Split(addr, ",")
		c.Cache.Type = "redis"
	}
	if pass := os.Getenv("REDIS_PASSWORD"); pass != "" {
		if c.Cache.RedisConfig != nil {
			c.Cache.RedisConfig.Password = pass
		}
	}

	// Server
	if port := os.Getenv("PORT"); port != "" {
		var p int
		fmt.Sscanf(port, "%d", &p)
		if p > 0 {
			c.Server.Port = p
		}
	}

	// Logging
	if level := os.Getenv("LOG_LEVEL"); level != "" {
		c.Logging.Level = level
	}

	// Output
	if outputDir := os.Getenv("OUTPUT_DIR"); outputDir != "" {
		c.Output.OutputDir = outputDir
	}
}

// Validate validates the configuration.
func (c *Config) Validate() error {
	// Validate server config
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", c.Server.Port)
	}

	// Validate LLM config
	if c.LLM.DefaultProvider == "" {
		return fmt.Errorf("default LLM provider not specified")
	}
	if _, ok := c.LLM.Providers[c.LLM.DefaultProvider]; !ok {
		return fmt.Errorf("default LLM provider not found: %s", c.LLM.DefaultProvider)
	}

	hasEnabledProvider := false
	for name, provider := range c.LLM.Providers {
		if provider.Enabled {
			hasEnabledProvider = true
			if provider.Model == "" {
				return fmt.Errorf("model not specified for provider: %s", name)
			}
		}
	}
	if !hasEnabledProvider {
		return fmt.Errorf("no LLM provider enabled")
	}

	// Validate OCR config
	if c.OCR.MaxConcurrency <= 0 {
		c.OCR.MaxConcurrency = 4
	}

	// Validate storage config
	if c.Storage.Type == "" {
		c.Storage.Type = "sqlite"
	}
	if c.Storage.DataDir == "" {
		c.Storage.DataDir = "./data"
	}

	// Create directories if needed
	dirs := []string{c.Storage.DataDir, c.Output.OutputDir}
	for _, dir := range dirs {
		if dir != "" && !filepath.IsAbs(dir) {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", dir, err)
			}
		}
	}

	return nil
}

// SaveConfig saves configuration to a file.
func (c *Config) SaveConfig(path string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// GetProviderConfig returns the configuration for a specific LLM provider.
func (c *Config) GetProviderConfig(name string) (*ProviderConfig, error) {
	provider, ok := c.LLM.Providers[name]
	if !ok {
		return nil, fmt.Errorf("provider not found: %s", name)
	}
	return provider, nil
}

// GetOCRAPIConfig returns the configuration for a specific OCR API provider.
func (c *Config) GetOCRAPIConfig(name string) (*OCRAPIConfig, error) {
	provider, ok := c.OCR.APIProviders[name]
	if !ok {
		return nil, fmt.Errorf("OCR API provider not found: %s", name)
	}
	return provider, nil
}
