// Package config 配置管理
package config

import (
	"encoding/json"
	"os"
	"sync"
	"time"
)

// Config 全局配置
type Config struct {
	Search    SearchConfig    `json:"search"`
	Email     EmailConfig     `json:"email"`
	Memory    MemoryConfig    `json:"memory"`
	JobSearch JobSearchConfig `json:"jobsearch"`
	MCP       MCPConfig       `json:"mcp"`
	Logging   LoggingConfig   `json:"logging"`
}

// JobSearchConfig 招聘搜索配置
type JobSearchConfig struct {
	// Adzuna API 配置（免费公开 API）
	AdzunaAppID  string `json:"adzuna_app_id"`
	AdzunaAppKey string `json:"adzuna_app_key"`
	// 缓存配置（纳秒）
	CacheTTL int64 `json:"cache_ttl"`
	// 速率限制（每分钟请求数）
	RateLimit int `json:"rate_limit"`
	// 默认国家/地区
	DefaultCountry string `json:"default_country"`
}

// SearchConfig 搜索工具配置
type SearchConfig struct {
	// 默认搜索引擎
	DefaultEngine string `json:"default_engine"`
	// 最大结果数
	MaxResults int `json:"max_results"`
	// 请求超时时间
	Timeout time.Duration `json:"timeout"`
	// 是否启用自动降级
	AutoFallback bool `json:"auto_fallback"`
	// 搜索引擎配置
	Engines map[string]EngineConfig `json:"engines"`
	// 路由配置
	Router RouterConfig `json:"router"`
}

// EngineConfig 搜索引擎配置
type EngineConfig struct {
	Enabled  bool   `json:"enabled"`
	APIKey   string `json:"api_key"`
	Endpoint string `json:"endpoint"`
	Priority int    `json:"priority"` // 优先级，数字越小优先级越高
}

// RouterConfig 路由配置
type RouterConfig struct {
	// 路由策略: health_first, round_robin, weighted
	Strategy string `json:"strategy"`
	// 熔断阈值（连续失败次数）
	CircuitBreakerThreshold int `json:"circuit_breaker_threshold"`
	// 熔断恢复时间
	CircuitBreakerTimeout time.Duration `json:"circuit_breaker_timeout"`
	// 健康检查间隔
	HealthCheckInterval time.Duration `json:"health_check_interval"`
}

// EmailConfig 邮件工具配置
type EmailConfig struct {
	// 默认邮箱提供商
	DefaultProvider string `json:"default_provider"`
	// 附件下载目录
	AttachmentDir string `json:"attachment_dir"`
	// 邮箱提供商配置
	Providers map[string]EmailProviderConfig `json:"providers"`
	// 意图分析配置
	Intent IntentConfig `json:"intent"`
}

// EmailProviderConfig 邮箱提供商配置
type EmailProviderConfig struct {
	IMAPServer string `json:"imap_server"`
	IMAPPort   int    `json:"imap_port"`
	SMTPServer string `json:"smtp_server"`
	SMTPPort   int    `json:"smtp_port"`
	UseSSL     bool   `json:"use_ssl"`
}

// IntentConfig 意图分析配置
type IntentConfig struct {
	// 是否启用意图分析
	Enabled bool `json:"enabled"`
	// 关键词配置文件路径
	KeywordsFile string `json:"keywords_file"`
}

// MemoryConfig 记忆工具配置
type MemoryConfig struct {
	// L1 工作记忆配置
	L1 L1Config `json:"l1"`
	// L2 短期记忆配置
	L2 L2Config `json:"l2"`
	// L3 长期记忆配置
	L3 L3Config `json:"l3"`
	// 检索配置
	Retrieval RetrievalConfig `json:"retrieval"`
}

// L1Config L1工作记忆配置
type L1Config struct {
	// Redis 地址
	RedisAddr string `json:"redis_addr"`
	// Redis 密码
	RedisPassword string `json:"redis_password"`
	// Redis DB
	RedisDB int `json:"redis_db"`
	// 最大消息数
	MaxMessages int `json:"max_messages"`
	// 最大Token数
	MaxTokens int `json:"max_tokens"`
	// 键前缀
	KeyPrefix string `json:"key_prefix"`
}

// L2Config L2短期记忆配置
type L2Config struct {
	// Milvus 地址
	MilvusAddr string `json:"milvus_addr"`
	// 集合名称
	CollectionName string `json:"collection_name"`
	// PostgreSQL 连接字符串
	PostgresDSN string `json:"postgres_dsn"`
	// 数据保留天数
	RetentionDays int `json:"retention_days"`
	// 向量维度
	VectorDimension int `json:"vector_dimension"`
}

// L3Config L3长期记忆配置
type L3Config struct {
	// S3 Endpoint
	S3Endpoint string `json:"s3_endpoint"`
	// S3 Bucket
	S3Bucket string `json:"s3_bucket"`
	// S3 Access Key
	S3AccessKey string `json:"s3_access_key"`
	// S3 Secret Key
	S3SecretKey string `json:"s3_secret_key"`
	// S3 Region
	S3Region string `json:"s3_region"`
	// Neo4j URI
	Neo4jURI string `json:"neo4j_uri"`
	// Neo4j 用户名
	Neo4jUser string `json:"neo4j_user"`
	// Neo4j 密码
	Neo4jPassword string `json:"neo4j_password"`
	// 是否启用压缩
	EnableCompression bool `json:"enable_compression"`
}

// RetrievalConfig 检索配置
type RetrievalConfig struct {
	// 相似度阈值
	SimilarityThreshold float64 `json:"similarity_threshold"`
	// 默认Token预算
	DefaultTokenBudget int `json:"default_token_budget"`
	// 多路召回权重
	ChannelWeights ChannelWeights `json:"channel_weights"`
	// 时间衰减因子
	TimeDecayFactor float64 `json:"time_decay_factor"`
}

// ChannelWeights 多路召回权重
type ChannelWeights struct {
	SemanticSearch float64 `json:"semantic_search"` // 向量语义检索权重
	EntityGraph    float64 `json:"entity_graph"`    // 实体关联图谱权重
	TemporalNear   float64 `json:"temporal_near"`   // 时序邻近主题权重
	UserPinned     float64 `json:"user_pinned"`     // 用户显式锁定权重
}

// MCPConfig MCP服务器配置
type MCPConfig struct {
	// 服务器名称
	ServerName string `json:"server_name"`
	// 服务器版本
	ServerVersion string `json:"server_version"`
	// 监听地址
	ListenAddr string `json:"listen_addr"`
}

// LoggingConfig 日志配置
type LoggingConfig struct {
	Level  string `json:"level"`
	Format string `json:"format"`
	Output string `json:"output"`
}

var (
	globalConfig *Config
	configOnce   sync.Once
)

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		Search: SearchConfig{
			DefaultEngine: "duckduckgo",
			MaxResults:    10,
			Timeout:       30 * time.Second,
			AutoFallback:  true,
			Engines: map[string]EngineConfig{
				"duckduckgo": {Enabled: true, Priority: 1},
				"bing":       {Enabled: true, Priority: 2},
				"baidu":      {Enabled: true, Priority: 3},
				"serper":     {Enabled: false, Priority: 4},
			},
			Router: RouterConfig{
				Strategy:                "health_first",
				CircuitBreakerThreshold: 3,
				CircuitBreakerTimeout:   60 * time.Second,
				HealthCheckInterval:     30 * time.Second,
			},
		},
		Email: EmailConfig{
			DefaultProvider: "qq",
			AttachmentDir:   "/tmp/email_attachments",
			Providers: map[string]EmailProviderConfig{
				"qq": {
					IMAPServer: "imap.qq.com",
					IMAPPort:   993,
					SMTPServer: "smtp.qq.com",
					SMTPPort:   465,
					UseSSL:     true,
				},
				"163": {
					IMAPServer: "imap.163.com",
					IMAPPort:   993,
					SMTPServer: "smtp.163.com",
					SMTPPort:   465,
					UseSSL:     true,
				},
				"126": {
					IMAPServer: "imap.126.com",
					IMAPPort:   993,
					SMTPServer: "smtp.126.com",
					SMTPPort:   465,
					UseSSL:     true,
				},
				"gmail": {
					IMAPServer: "imap.gmail.com",
					IMAPPort:   993,
					SMTPServer: "smtp.gmail.com",
					SMTPPort:   587,
					UseSSL:     true,
				},
			},
			Intent: IntentConfig{
				Enabled:      true,
				KeywordsFile: "",
			},
		},
		Memory: MemoryConfig{
			L1: L1Config{
				RedisAddr:     "localhost:6379",
				RedisPassword: "",
				RedisDB:       0,
				MaxMessages:   20,
				MaxTokens:     4000,
				KeyPrefix:     "memory:l1:",
			},
			L2: L2Config{
				MilvusAddr:      "localhost:19530",
				CollectionName:  "memory_capsules",
				PostgresDSN:     "postgres://localhost:5432/memory?sslmode=disable",
				RetentionDays:   30,
				VectorDimension: 1536,
			},
			L3: L3Config{
				S3Endpoint:        "localhost:9000",
				S3Bucket:          "memory-archive",
				S3Region:          "us-east-1",
				Neo4jURI:          "bolt://localhost:7687",
				Neo4jUser:         "neo4j",
				Neo4jPassword:     "",
				EnableCompression: true,
			},
			Retrieval: RetrievalConfig{
				SimilarityThreshold: 0.8,
				DefaultTokenBudget:  4000,
				ChannelWeights: ChannelWeights{
					SemanticSearch: 0.4,
					EntityGraph:    0.3,
					TemporalNear:   0.2,
					UserPinned:     0.1,
				},
				TimeDecayFactor: 0.95,
			},
		},
		MCP: MCPConfig{
			ServerName:    "agent-tools",
			ServerVersion: "1.0.0",
			ListenAddr:    ":8080",
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
			Output: "stdout",
		},
	}
}

// Load 从文件加载配置
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return nil, err
	}

	// 替换环境变量占位符 ${VAR_NAME}
	data = expandEnvVariables(data)

	cfg := DefaultConfig()
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// expandEnvVariables 替换配置中的环境变量占位符
func expandEnvVariables(data []byte) []byte {
	content := string(data)
	// 匹配 ${VAR_NAME} 格式
	for {
		start := -1
		for i := 0; i < len(content)-2; i++ {
			if content[i] == '$' && content[i+1] == '{' {
				start = i
				break
			}
		}
		if start == -1 {
			break
		}
		end := -1
		for i := start + 2; i < len(content); i++ {
			if content[i] == '}' {
				end = i
				break
			}
		}
		if end == -1 {
			break
		}
		varName := content[start+2 : end]
		varValue := os.Getenv(varName)
		content = content[:start] + varValue + content[end+1:]
	}
	return []byte(content)
}

// Save 保存配置到文件
func (c *Config) Save(path string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// GetConfig 获取全局配置（单例）
func GetConfig() *Config {
	configOnce.Do(func() {
		globalConfig = DefaultConfig()
	})
	return globalConfig
}

// SetConfig 设置全局配置
func SetConfig(cfg *Config) {
	globalConfig = cfg
}

// LoadFromEnv 从环境变量加载配置覆盖
func (c *Config) LoadFromEnv() {
	// Search
	if v := os.Getenv("SEARCH_DEFAULT_ENGINE"); v != "" {
		c.Search.DefaultEngine = v
	}
	if v := os.Getenv("SERPER_API_KEY"); v != "" {
		if eng, ok := c.Search.Engines["serper"]; ok {
			eng.APIKey = v
			eng.Enabled = true
			c.Search.Engines["serper"] = eng
		}
	}

	// Email
	if v := os.Getenv("EMAIL_ATTACHMENT_DIR"); v != "" {
		c.Email.AttachmentDir = v
	}

	// Memory L1
	if v := os.Getenv("REDIS_ADDR"); v != "" {
		c.Memory.L1.RedisAddr = v
	}
	if v := os.Getenv("REDIS_PASSWORD"); v != "" {
		c.Memory.L1.RedisPassword = v
	}

	// Memory L2
	if v := os.Getenv("MILVUS_ADDR"); v != "" {
		c.Memory.L2.MilvusAddr = v
	}
	if v := os.Getenv("POSTGRES_DSN"); v != "" {
		c.Memory.L2.PostgresDSN = v
	}

	// Memory L3
	if v := os.Getenv("S3_ENDPOINT"); v != "" {
		c.Memory.L3.S3Endpoint = v
	}
	if v := os.Getenv("S3_BUCKET"); v != "" {
		c.Memory.L3.S3Bucket = v
	}
	if v := os.Getenv("S3_ACCESS_KEY"); v != "" {
		c.Memory.L3.S3AccessKey = v
	}
	if v := os.Getenv("S3_SECRET_KEY"); v != "" {
		c.Memory.L3.S3SecretKey = v
	}
	if v := os.Getenv("NEO4J_URI"); v != "" {
		c.Memory.L3.Neo4jURI = v
	}
	if v := os.Getenv("NEO4J_USER"); v != "" {
		c.Memory.L3.Neo4jUser = v
	}
	if v := os.Getenv("NEO4J_PASSWORD"); v != "" {
		c.Memory.L3.Neo4jPassword = v
	}

	// MCP
	if v := os.Getenv("MCP_LISTEN_ADDR"); v != "" {
		c.MCP.ListenAddr = v
	}
}
