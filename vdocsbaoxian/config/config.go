// Package config provides configuration management for Insurance Expert Agent.
package config

import (
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the main configuration structure.
type Config struct {
	Log         LogConfig         `yaml:"log"`
	DataSources DataSourcesConfig `yaml:"data_sources"`
	Index       IndexConfig       `yaml:"index"`
	Cache       CacheConfig       `yaml:"cache"`
	Storage     StorageConfig     `yaml:"storage"`
	Stats       StatsConfig       `yaml:"stats"`
	Agent       AgentConfig       `yaml:"agent"`
	Output      OutputConfig      `yaml:"output"`
	LLM         LLMConfig         `yaml:"llm"`
	Interaction InteractionConfig `yaml:"interaction"`
	Verify      VerifyConfig      `yaml:"verify"`
}

// LogConfig defines logging configuration.
type LogConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
	Output string `yaml:"output"`
}

// DataSourcesConfig defines data sources configuration.
type DataSourcesConfig struct {
	LegalDB   string `yaml:"legal_db"`
	ProductDB string `yaml:"product_db"`
	ClaimDB   string `yaml:"claim_db"`
	HealthDB  string `yaml:"health_db"`
	WebCache  string `yaml:"web_cache"`
}

// IndexConfig defines indexing configuration.
type IndexConfig struct {
	Path            string            `yaml:"path"`
	RebuildOnStart  bool              `yaml:"rebuild_on_start"`
	ParallelWorkers int               `yaml:"parallel_workers"`
	BatchSize       int               `yaml:"batch_size"`
	UpdateInterval  time.Duration     `yaml:"update_interval"`
	ChineseDict     string            `yaml:"chinese_dict"`
	Persistence     PersistenceConfig `yaml:"persistence"`
	Vector          VectorConfig      `yaml:"vector"`
}

// PersistenceConfig defines index persistence settings.
type PersistenceConfig struct {
	PersistOnBuild    bool          `yaml:"persist_on_build"`
	PersistInterval   time.Duration `yaml:"persist_interval"`
	PersistOnShutdown bool          `yaml:"persist_on_shutdown"`
	LocalPath         string        `yaml:"local_path"`
	RecoveryStrategy  string        `yaml:"recovery_strategy"`
}

// VectorConfig defines vector index configuration.
type VectorConfig struct {
	Enabled     bool   `yaml:"enabled"`
	Provider    string `yaml:"provider"` // openai, ollama, local
	Model       string `yaml:"model"`
	Dimension   int    `yaml:"dimension"`
	IndexType   string `yaml:"index_type"` // flat, hnsw, ivf
	StoragePath string `yaml:"storage_path"`
}

// CacheConfig defines multi-layer cache configuration.
type CacheConfig struct {
	L1 L1CacheConfig `yaml:"l1"`
	L2 L2CacheConfig `yaml:"l2"`
	L3 L3CacheConfig `yaml:"l3"`
}

// L1CacheConfig defines L1 (memory) cache configuration.
type L1CacheConfig struct {
	Enabled    bool          `yaml:"enabled"`
	MaxSize    int           `yaml:"max_size"`
	MaxMemory  int64         `yaml:"max_memory"`
	TTL        time.Duration `yaml:"ttl"`
	ShardCount int           `yaml:"shard_count"`
}

// L2CacheConfig defines L2 (Redis/local) cache configuration.
type L2CacheConfig struct {
	Enabled   bool          `yaml:"enabled"`
	Backend   string        `yaml:"backend"` // redis, sqlite
	TTL       time.Duration `yaml:"ttl"`
	Redis     *RedisConfig  `yaml:"redis,omitempty"`
	LocalPath string        `yaml:"local_path,omitempty"`
}

// L3CacheConfig defines L3 (persistent) cache configuration.
type L3CacheConfig struct {
	Enabled     bool          `yaml:"enabled"`
	DBPath      string        `yaml:"db_path"`
	TTL         time.Duration `yaml:"ttl"`
	MaxSize     int64         `yaml:"max_size_bytes"`
	Compression bool          `yaml:"compression"`
}

// RedisConfig defines Redis connection configuration.
type RedisConfig struct {
	Mode         string     `yaml:"mode"` // standalone, sentinel, cluster
	Addresses    []string   `yaml:"addresses"`
	Password     string     `yaml:"password"`
	PasswordEnv  string     `yaml:"password_env"`
	Database     int        `yaml:"database"`
	MaxRetries   int        `yaml:"max_retries"`
	PoolSize     int        `yaml:"pool_size"`
	MinIdleConns int        `yaml:"min_idle_conns"`
	MasterName   string     `yaml:"master_name,omitempty"`
	TLS          *TLSConfig `yaml:"tls,omitempty"`
}

// TLSConfig defines TLS configuration.
type TLSConfig struct {
	Enabled    bool   `yaml:"enabled"`
	CertFile   string `yaml:"cert_file"`
	KeyFile    string `yaml:"key_file"`
	CAFile     string `yaml:"ca_file"`
	SkipVerify bool   `yaml:"skip_verify"`
}

// StorageConfig defines storage backend configuration.
type StorageConfig struct {
	Primary  string        `yaml:"primary"` // sqlite, mysql
	SQLite   *SQLiteConfig `yaml:"sqlite,omitempty"`
	MySQL    *MySQLConfig  `yaml:"mysql,omitempty"`
	BoltDB   *BoltDBConfig `yaml:"boltdb,omitempty"`
}

// SQLiteConfig defines SQLite configuration.
type SQLiteConfig struct {
	Path            string        `yaml:"path"`
	MaxOpenConns    int           `yaml:"max_open_conns"`
	MaxIdleConns    int           `yaml:"max_idle_conns"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime"`
	JournalMode     string        `yaml:"journal_mode"`
	SynchronousMode string        `yaml:"synchronous"`
}

// MySQLConfig defines MySQL configuration.
type MySQLConfig struct {
	Host            string        `yaml:"host"`
	HostEnv         string        `yaml:"host_env"`
	Port            int           `yaml:"port"`
	User            string        `yaml:"user"`
	Password        string        `yaml:"password"`
	PasswordEnv     string        `yaml:"password_env"`
	Database        string        `yaml:"database"`
	Charset         string        `yaml:"charset"`
	MaxOpenConns    int           `yaml:"max_open_conns"`
	MaxIdleConns    int           `yaml:"max_idle_conns"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime"`
	TLS             *TLSConfig    `yaml:"tls,omitempty"`
}

// BoltDBConfig defines BoltDB configuration.
type BoltDBConfig struct {
	Path string `yaml:"path"`
}

// StatsConfig defines statistics configuration.
type StatsConfig struct {
	Token      TokenStatsConfig `yaml:"token"`
	Cache      CacheStatsConfig `yaml:"cache"`
	Prometheus PrometheusConfig `yaml:"prometheus"`
}

// TokenStatsConfig defines token statistics configuration.
type TokenStatsConfig struct {
	Enabled         bool          `yaml:"enabled"`
	CountMethod     string        `yaml:"count_method"` // estimate, tiktoken, model
	BudgetEnabled   bool          `yaml:"budget_enabled"`
	DailyBudget     int64         `yaml:"daily_budget"`
	SessionBudget   int64         `yaml:"session_budget"`
	AlertThreshold  float64       `yaml:"alert_threshold"`
	PersistEnabled  bool          `yaml:"persist_enabled"`
	PersistInterval time.Duration `yaml:"persist_interval"`
	RetentionDays   int           `yaml:"retention_days"`
}

// CacheStatsConfig defines cache statistics configuration.
type CacheStatsConfig struct {
	Enabled        bool          `yaml:"enabled"`
	DetailLevel    string        `yaml:"detail_level"` // basic, standard, detailed
	ReportInterval time.Duration `yaml:"report_interval"`
	ReportEnabled  bool          `yaml:"report_enabled"`
}

// PrometheusConfig defines Prometheus metrics configuration.
type PrometheusConfig struct {
	Enabled   bool   `yaml:"enabled"`
	Port      int    `yaml:"port"`
	Path      string `yaml:"path"`
	Namespace string `yaml:"namespace"`
}

// AgentConfig defines agent behavior configuration.
type AgentConfig struct {
	MaxIterations       int           `yaml:"max_iterations"`
	MaxConcurrentAgents int           `yaml:"max_concurrent_agents"`
	Timeout             time.Duration `yaml:"timeout"`
	EnableStreaming     bool          `yaml:"enable_streaming"`
	VerifyMode          bool          `yaml:"verify_mode"`
}

// OutputConfig defines output configuration.
type OutputConfig struct {
	Mode              string `yaml:"mode"` // summary, document
	IncludeDiagrams   bool   `yaml:"include_diagrams"`
	IncludeSources    bool   `yaml:"include_sources"`
	VerifyAnnotations bool   `yaml:"verify_annotations"`
	OutputDir         string `yaml:"output_dir"`
}

// VerifyConfig defines verification configuration.
type VerifyConfig struct {
	Enabled           bool          `yaml:"enabled"`
	LegalVerify       bool          `yaml:"legal_verify"`
	SourceVerify      bool          `yaml:"source_verify"`
	TimelinessCheck   bool          `yaml:"timeliness_check"`
	MinConfidence     float64       `yaml:"min_confidence"`
	CacheVerifyResult bool          `yaml:"cache_verify_result"`
	CacheTTL          time.Duration `yaml:"cache_ttl"`
}

// LLMConfig defines LLM provider configuration.
type LLMConfig struct {
	DefaultProvider string                     `yaml:"default_provider"`
	DefaultModel    string                     `yaml:"default_model"`
	Providers       map[string]*ProviderConfig `yaml:"providers"`
	Routing         *RoutingConfig             `yaml:"routing,omitempty"`
	Fallback        *FallbackConfig            `yaml:"fallback,omitempty"`
	RateLimiting    *RateLimitConfig           `yaml:"rate_limiting,omitempty"`
}

// ProviderConfig defines LLM provider configuration.
type ProviderConfig struct {
	Name         string                  `yaml:"name"`
	Type         string                  `yaml:"type"` // openai, anthropic, zhipu, deepseek, qwen, ollama
	Enabled      bool                    `yaml:"enabled"`
	Priority     int                     `yaml:"priority"`
	BaseURL      string                  `yaml:"base_url"`
	APIKey       string                  `yaml:"api_key"`
	APIKeyEnv    string                  `yaml:"api_key_env"`
	APIVersion   string                  `yaml:"api_version,omitempty"`
	Organization string                  `yaml:"organization,omitempty"`
	Models       map[string]*ModelConfig `yaml:"models,omitempty"`
	Timeout      time.Duration           `yaml:"timeout"`
	MaxRetries   int                     `yaml:"max_retries"`
	RetryDelay   time.Duration           `yaml:"retry_delay"`
	HTTPProxy    string                  `yaml:"http_proxy,omitempty"`
	HTTPProxyEnv string                  `yaml:"http_proxy_env,omitempty"`
	TLS          *TLSConfig              `yaml:"tls,omitempty"`
	Headers      map[string]string       `yaml:"headers,omitempty"`
}

// ModelConfig defines individual model configuration.
type ModelConfig struct {
	Name             string   `yaml:"name"`
	DisplayName      string   `yaml:"display_name"`
	Description      string   `yaml:"description,omitempty"`
	Enabled          bool     `yaml:"enabled"`
	Capabilities     []string `yaml:"capabilities,omitempty"`
	MaxContextLength int      `yaml:"max_context_length"`
	MaxOutputLength  int      `yaml:"max_output_length"`
	Temperature      float64  `yaml:"temperature"`
	TopP             float64  `yaml:"top_p,omitempty"`
	InputCost        float64  `yaml:"input_cost_per_1k"`
	OutputCost       float64  `yaml:"output_cost_per_1k"`
	UseCases         []string `yaml:"use_cases,omitempty"`
}

// RoutingConfig defines model routing configuration.
type RoutingConfig struct {
	Enabled              bool                        `yaml:"enabled"`
	Strategy             string                      `yaml:"strategy"` // simple, cost_optimized, quality_first, smart
	SceneMapping         map[string]*SceneModels     `yaml:"scene_mapping,omitempty"`
	ContextLengthRouting []*ContextLengthRule        `yaml:"context_length_routing,omitempty"`
}

// SceneModels defines models for a specific scene.
type SceneModels struct {
	Preferred []string `yaml:"preferred"`
	Fallback  []string `yaml:"fallback"`
}

// ContextLengthRule defines routing based on context length.
type ContextLengthRule struct {
	MaxLength int      `yaml:"max_length"`
	Models    []string `yaml:"models"`
}

// FallbackConfig defines fallback configuration.
type FallbackConfig struct {
	Enabled              bool          `yaml:"enabled"`
	MaxAttempts          int           `yaml:"max_attempts"`
	DelayBetweenAttempts time.Duration `yaml:"delay_between_attempts"`
	FallbackChain        []string      `yaml:"fallback_chain"`
}

// RateLimitConfig defines rate limiting configuration.
type RateLimitConfig struct {
	Enabled     bool                      `yaml:"enabled"`
	Global      *RateLimitRule            `yaml:"global,omitempty"`
	PerProvider map[string]*RateLimitRule `yaml:"per_provider,omitempty"`
}

// RateLimitRule defines a rate limit rule.
type RateLimitRule struct {
	RequestsPerMinute int `yaml:"requests_per_minute"`
	TokensPerMinute   int `yaml:"tokens_per_minute"`
}

// InteractionConfig defines agent interaction configuration.
type InteractionConfig struct {
	MCPServer       *MCPServerConfig       `yaml:"mcp_server,omitempty"`
	MCPClient       *MCPClientConfig       `yaml:"mcp_client,omitempty"`
	A2AServer       *A2AServerConfig       `yaml:"a2a_server,omitempty"`
	A2AClient       *A2AClientConfig       `yaml:"a2a_client,omitempty"`
	RESTServer      *RESTServerConfig      `yaml:"rest_server,omitempty"`
	WebSocketServer *WebSocketServerConfig `yaml:"websocket_server,omitempty"`
}

// MCPServerConfig defines MCP server configuration.
type MCPServerConfig struct {
	Enabled     bool   `yaml:"enabled"`
	Name        string `yaml:"name"`
	Version     string `yaml:"version"`
	Description string `yaml:"description"`
	Transport   string `yaml:"transport"` // stdio, sse, websocket
	Address     string `yaml:"address"`
	Path        string `yaml:"path,omitempty"`
}

// MCPClientConfig defines MCP client configuration.
type MCPClientConfig struct {
	Servers map[string]*MCPServerEndpoint `yaml:"servers,omitempty"`
}

// MCPServerEndpoint defines an MCP server endpoint.
type MCPServerEndpoint struct {
	Name      string            `yaml:"name"`
	Transport string            `yaml:"transport"`
	Command   string            `yaml:"command,omitempty"`
	Args      []string          `yaml:"args,omitempty"`
	URL       string            `yaml:"url,omitempty"`
	Headers   map[string]string `yaml:"headers,omitempty"`
	Timeout   time.Duration     `yaml:"timeout"`
}

// A2AServerConfig defines A2A server configuration.
type A2AServerConfig struct {
	Enabled bool       `yaml:"enabled"`
	Address string     `yaml:"address"`
	TLS     *TLSConfig `yaml:"tls,omitempty"`
}

// A2AClientConfig defines A2A client configuration.
type A2AClientConfig struct {
	Agents map[string]*A2AAgentEndpoint `yaml:"agents,omitempty"`
}

// A2AAgentEndpoint defines an A2A agent endpoint.
type A2AAgentEndpoint struct {
	URL       string            `yaml:"url"`
	APIKey    string            `yaml:"api_key,omitempty"`
	APIKeyEnv string            `yaml:"api_key_env,omitempty"`
	Timeout   time.Duration     `yaml:"timeout"`
	Headers   map[string]string `yaml:"headers,omitempty"`
}

// RESTServerConfig defines REST server configuration.
type RESTServerConfig struct {
	Enabled   bool             `yaml:"enabled"`
	Address   string           `yaml:"address"`
	BasePath  string           `yaml:"base_path"`
	TLS       *TLSConfig       `yaml:"tls,omitempty"`
	CORS      *CORSConfig      `yaml:"cors,omitempty"`
	Auth      *AuthConfig      `yaml:"auth,omitempty"`
	RateLimit *RateLimitConfig `yaml:"rate_limit,omitempty"`
}

// CORSConfig defines CORS configuration.
type CORSConfig struct {
	Enabled          bool     `yaml:"enabled"`
	AllowOrigins     []string `yaml:"allow_origins"`
	AllowMethods     []string `yaml:"allow_methods"`
	AllowHeaders     []string `yaml:"allow_headers"`
	ExposeHeaders    []string `yaml:"expose_headers,omitempty"`
	AllowCredentials bool     `yaml:"allow_credentials"`
	MaxAge           int      `yaml:"max_age"`
}

// AuthConfig defines authentication configuration.
type AuthConfig struct {
	Enabled   bool     `yaml:"enabled"`
	Type      string   `yaml:"type"` // api_key, jwt, basic
	APIKeys   []string `yaml:"api_keys,omitempty"`
	JWTSecret string   `yaml:"jwt_secret,omitempty"`
}

// WebSocketServerConfig defines WebSocket server configuration.
type WebSocketServerConfig struct {
	Enabled         bool          `yaml:"enabled"`
	Address         string        `yaml:"address"`
	Path            string        `yaml:"path"`
	PingInterval    time.Duration `yaml:"ping_interval"`
	PongWait        time.Duration `yaml:"pong_wait"`
	MaxMessageSize  int64         `yaml:"max_message_size"`
	WriteBufferSize int           `yaml:"write_buffer_size"`
	ReadBufferSize  int           `yaml:"read_buffer_size"`
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	return &Config{
		Log: LogConfig{
			Level:  "info",
			Format: "json",
			Output: "stdout",
		},
		DataSources: DataSourcesConfig{
			LegalDB:   "/data/legal",
			ProductDB: "/data/products",
			ClaimDB:   "/data/claims",
			HealthDB:  "/data/health",
			WebCache:  "/data/webcache",
		},
		Index: IndexConfig{
			Path:            "/data/index",
			RebuildOnStart:  false,
			ParallelWorkers: 8,
			BatchSize:       1000,
			UpdateInterval:  time.Hour,
			ChineseDict:     "/data/dict/insurance.txt",
			Persistence: PersistenceConfig{
				PersistOnBuild:    true,
				PersistInterval:   30 * time.Minute,
				PersistOnShutdown: true,
				LocalPath:         "/data/index",
				RecoveryStrategy:  "auto",
			},
			Vector: VectorConfig{
				Enabled:     true,
				Provider:    "openai",
				Model:       "text-embedding-3-small",
				Dimension:   1536,
				IndexType:   "flat",
				StoragePath: "/data/index/vectors",
			},
		},
		Cache: CacheConfig{
			L1: L1CacheConfig{
				Enabled:    true,
				MaxSize:    10000,
				TTL:        5 * time.Minute,
				ShardCount: 16,
			},
			L2: L2CacheConfig{
				Enabled:   true,
				Backend:   "sqlite",
				TTL:       30 * time.Minute,
				LocalPath: "/data/cache/l2.db",
			},
			L3: L3CacheConfig{
				Enabled:     true,
				DBPath:      "/data/cache/l3.db",
				TTL:         24 * time.Hour,
				Compression: true,
			},
		},
		Storage: StorageConfig{
			Primary: "sqlite",
			SQLite: &SQLiteConfig{
				Path:            "/data/insurance_expert.db",
				MaxOpenConns:    10,
				MaxIdleConns:    5,
				ConnMaxLifetime: time.Hour,
				JournalMode:     "WAL",
				SynchronousMode: "NORMAL",
			},
			BoltDB: &BoltDBConfig{
				Path: "/data/knowledge_graph.db",
			},
		},
		Stats: StatsConfig{
			Token: TokenStatsConfig{
				Enabled:         true,
				CountMethod:     "estimate",
				BudgetEnabled:   true,
				DailyBudget:     1000000,
				AlertThreshold:  0.8,
				PersistEnabled:  true,
				PersistInterval: 5 * time.Minute,
				RetentionDays:   30,
			},
			Cache: CacheStatsConfig{
				Enabled:        true,
				DetailLevel:    "standard",
				ReportInterval: 5 * time.Minute,
				ReportEnabled:  true,
			},
			Prometheus: PrometheusConfig{
				Enabled:   true,
				Port:      9090,
				Path:      "/metrics",
				Namespace: "insurance_expert",
			},
		},
		Agent: AgentConfig{
			MaxIterations:       20,
			MaxConcurrentAgents: 10,
			Timeout:             5 * time.Minute,
			EnableStreaming:     true,
			VerifyMode:          true,
		},
		Output: OutputConfig{
			Mode:              "document",
			IncludeDiagrams:   true,
			IncludeSources:    true,
			VerifyAnnotations: true,
			OutputDir:         "/data/output",
		},
		Verify: VerifyConfig{
			Enabled:           true,
			LegalVerify:       true,
			SourceVerify:      true,
			TimelinessCheck:   true,
			MinConfidence:     0.7,
			CacheVerifyResult: true,
			CacheTTL:          time.Hour,
		},
		LLM: LLMConfig{
			DefaultProvider: "zhipu",
			DefaultModel:    "glm-4",
			Providers: map[string]*ProviderConfig{
				"zhipu": {
					Name:      "智谱AI",
					Type:      "zhipu",
					Enabled:   true,
					Priority:  1,
					BaseURL:   "https://open.bigmodel.cn/api/paas/v4",
					APIKeyEnv: "ZHIPU_API_KEY",
					Timeout:   120 * time.Second,
					MaxRetries: 3,
					Models: map[string]*ModelConfig{
						"glm-4": {
							Name:             "glm-4",
							DisplayName:      "GLM-4",
							Enabled:          true,
							MaxContextLength: 128000,
							MaxOutputLength:  4096,
							Temperature:      0.7,
							UseCases:         []string{"general", "analysis"},
						},
						"glm-4-flash": {
							Name:             "glm-4-flash",
							DisplayName:      "GLM-4-Flash",
							Enabled:          true,
							MaxContextLength: 128000,
							MaxOutputLength:  4096,
							Temperature:      0.7,
							UseCases:         []string{"simple_query", "fast"},
						},
					},
				},
				"deepseek": {
					Name:      "DeepSeek",
					Type:      "deepseek",
					Enabled:   true,
					Priority:  2,
					BaseURL:   "https://api.deepseek.com/v1",
					APIKeyEnv: "DEEPSEEK_API_KEY",
					Timeout:   120 * time.Second,
					MaxRetries: 3,
				},
				"openai": {
					Name:         "OpenAI",
					Type:         "openai",
					Enabled:      true,
					Priority:     3,
					BaseURL:      "https://api.openai.com/v1",
					APIKeyEnv:    "OPENAI_API_KEY",
					HTTPProxyEnv: "HTTP_PROXY",
					Timeout:      120 * time.Second,
					MaxRetries:   3,
				},
				"anthropic": {
					Name:         "Anthropic",
					Type:         "anthropic",
					Enabled:      true,
					Priority:     4,
					BaseURL:      "https://api.anthropic.com/v1",
					APIKeyEnv:    "ANTHROPIC_API_KEY",
					HTTPProxyEnv: "HTTP_PROXY",
					Timeout:      120 * time.Second,
					MaxRetries:   3,
				},
				"ollama": {
					Name:     "Ollama",
					Type:     "ollama",
					Enabled:  true,
					Priority: 10,
					BaseURL:  "http://localhost:11434",
					Timeout:  300 * time.Second,
				},
			},
			Routing: &RoutingConfig{
				Enabled:  true,
				Strategy: "smart",
				SceneMapping: map[string]*SceneModels{
					"simple_query": {
						Preferred: []string{"glm-4-flash", "deepseek-chat"},
						Fallback:  []string{"glm-4"},
					},
					"legal_analysis": {
						Preferred: []string{"gpt-4-turbo", "claude-3-sonnet"},
						Fallback:  []string{"glm-4", "deepseek-chat"},
					},
					"complex_reasoning": {
						Preferred: []string{"claude-3-opus", "gpt-4-turbo"},
						Fallback:  []string{"deepseek-reasoner", "glm-4"},
					},
					"document_generation": {
						Preferred: []string{"gpt-4-turbo", "glm-4"},
						Fallback:  []string{"deepseek-chat"},
					},
				},
			},
			Fallback: &FallbackConfig{
				Enabled:              true,
				MaxAttempts:          3,
				DelayBetweenAttempts: time.Second,
			},
		},
		Interaction: InteractionConfig{
			RESTServer: &RESTServerConfig{
				Enabled:  true,
				Address:  ":8080",
				BasePath: "/api/v1",
				CORS: &CORSConfig{
					Enabled:      true,
					AllowOrigins: []string{"*"},
					AllowMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
					AllowHeaders: []string{"Content-Type", "Authorization"},
				},
			},
			MCPServer: &MCPServerConfig{
				Enabled:     true,
				Name:        "insurance-expert",
				Version:     "1.0.0",
				Description: "Insurance Expert Agent MCP Server",
				Transport:   "stdio",
			},
		},
	}
}

// LoadConfig loads configuration from a YAML file.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	config := DefaultConfig()
	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, err
	}

	// Apply environment variable overrides
	config.applyEnvOverrides()

	return config, nil
}

// SaveConfig saves configuration to a YAML file.
func SaveConfig(config *Config, path string) error {
	data, err := yaml.Marshal(config)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// applyEnvOverrides applies environment variable overrides to configuration.
func (c *Config) applyEnvOverrides() {
	// Apply LLM provider API key overrides
	for _, provider := range c.LLM.Providers {
		if provider.APIKeyEnv != "" {
			if apiKey := os.Getenv(provider.APIKeyEnv); apiKey != "" {
				provider.APIKey = apiKey
			}
		}
		if provider.HTTPProxyEnv != "" {
			if proxy := os.Getenv(provider.HTTPProxyEnv); proxy != "" {
				provider.HTTPProxy = proxy
			}
		}
	}

	// Apply storage password overrides
	if c.Storage.MySQL != nil {
		if c.Storage.MySQL.PasswordEnv != "" {
			if password := os.Getenv(c.Storage.MySQL.PasswordEnv); password != "" {
				c.Storage.MySQL.Password = password
			}
		}
		if c.Storage.MySQL.HostEnv != "" {
			if host := os.Getenv(c.Storage.MySQL.HostEnv); host != "" {
				c.Storage.MySQL.Host = host
			}
		}
	}

	// Apply Redis password override
	if c.Cache.L2.Redis != nil {
		if c.Cache.L2.Redis.PasswordEnv != "" {
			if password := os.Getenv(c.Cache.L2.Redis.PasswordEnv); password != "" {
				c.Cache.L2.Redis.Password = password
			}
		}
	}
}

// GetEnvOrDefault returns environment variable value or default.
func GetEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// Validate validates the configuration.
func (c *Config) Validate() error {
	// Add validation logic here
	return nil
}
