// Package config provides configuration management for Interview Expert Agent.
package config

import (
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the main configuration structure.
type Config struct {
	Log         LogConfig         `yaml:"log"`
	Agent       AgentConfig       `yaml:"agent"`
	LLM         LLMConfig         `yaml:"llm"`
	Interview   InterviewConfig   `yaml:"interview"`
	Output      OutputConfig      `yaml:"output"`
	Storage     StorageConfig     `yaml:"storage"`
	Interaction InteractionConfig `yaml:"interaction"`
}

// LogConfig defines logging configuration.
type LogConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
	Output string `yaml:"output"`
}

// AgentConfig defines agent behavior configuration.
type AgentConfig struct {
	MaxIterations       int           `yaml:"max_iterations"`
	MaxConcurrentAgents int           `yaml:"max_concurrent_agents"`
	Timeout             time.Duration `yaml:"timeout"`
	EnableStreaming     bool          `yaml:"enable_streaming"`
}

// LLMConfig defines LLM provider configuration.
type LLMConfig struct {
	DefaultProvider string                     `yaml:"default_provider"`
	DefaultModel    string                     `yaml:"default_model"`
	Providers       map[string]*ProviderConfig `yaml:"providers"`
}

// ProviderConfig defines LLM provider configuration.
type ProviderConfig struct {
	Name       string            `yaml:"name"`
	Type       string            `yaml:"type"` // openai, anthropic, zhipu, deepseek, ollama
	Enabled    bool              `yaml:"enabled"`
	Priority   int               `yaml:"priority"`
	BaseURL    string            `yaml:"base_url"`
	APIKey     string            `yaml:"api_key"`
	APIKeyEnv  string            `yaml:"api_key_env"`
	Model      string            `yaml:"model"`
	Timeout    time.Duration     `yaml:"timeout"`
	MaxRetries int               `yaml:"max_retries"`
	HTTPProxy  string            `yaml:"http_proxy,omitempty"`
	Headers    map[string]string `yaml:"headers,omitempty"`
}

// InterviewConfig defines interview-specific configuration.
type InterviewConfig struct {
	DefaultDuration      int                 `yaml:"default_duration_minutes"`
	MaxQuestions         int                 `yaml:"max_questions"`
	SkillCategories      []string            `yaml:"skill_categories"`
	DifficultyWeights    map[string]float64  `yaml:"difficulty_weights"`
	QuestionBankPath     string              `yaml:"question_bank_path"`
	EnableAdaptive       bool                `yaml:"enable_adaptive"`
	AdaptiveConfig       *AdaptiveConfig     `yaml:"adaptive_config,omitempty"`
	Scoring              ScoringConfig       `yaml:"scoring"`
}

// AdaptiveConfig defines adaptive interview configuration.
type AdaptiveConfig struct {
	MinQuestions        int     `yaml:"min_questions"`
	MaxQuestions        int     `yaml:"max_questions"`
	ConfidenceThreshold float64 `yaml:"confidence_threshold"`
	DifficultyAdjust    bool    `yaml:"difficulty_adjust"`
}

// ScoringConfig defines scoring configuration.
type ScoringConfig struct {
	PassThreshold     float64            `yaml:"pass_threshold"`
	WeightByDifficulty bool              `yaml:"weight_by_difficulty"`
	SkillWeights      map[string]float64 `yaml:"skill_weights,omitempty"`
	RoundWeights      map[string]float64 `yaml:"round_weights,omitempty"`
}

// OutputConfig defines output configuration.
type OutputConfig struct {
	Mode            string `yaml:"mode"` // summary, detailed, full
	IncludeDiagrams bool   `yaml:"include_diagrams"`
	DiagramStyle    string `yaml:"diagram_style"` // mermaid, ascii
	OutputDir       string `yaml:"output_dir"`
	ReportFormat    string `yaml:"report_format"` // markdown, html, json
}

// StorageConfig defines storage configuration.
type StorageConfig struct {
	Type      string        `yaml:"type"` // sqlite, memory
	Path      string        `yaml:"path"`
	CacheTTL  time.Duration `yaml:"cache_ttl"`
}

// InteractionConfig defines interaction configuration.
type InteractionConfig struct {
	RESTServer *RESTServerConfig `yaml:"rest_server,omitempty"`
	MCPServer  *MCPServerConfig  `yaml:"mcp_server,omitempty"`
}

// RESTServerConfig defines REST server configuration.
type RESTServerConfig struct {
	Enabled  bool       `yaml:"enabled"`
	Address  string     `yaml:"address"`
	BasePath string     `yaml:"base_path"`
	CORS     *CORSConfig `yaml:"cors,omitempty"`
}

// CORSConfig defines CORS configuration.
type CORSConfig struct {
	Enabled      bool     `yaml:"enabled"`
	AllowOrigins []string `yaml:"allow_origins"`
	AllowMethods []string `yaml:"allow_methods"`
	AllowHeaders []string `yaml:"allow_headers"`
}

// MCPServerConfig defines MCP server configuration.
type MCPServerConfig struct {
	Enabled     bool   `yaml:"enabled"`
	Name        string `yaml:"name"`
	Version     string `yaml:"version"`
	Description string `yaml:"description"`
	Transport   string `yaml:"transport"`
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	return &Config{
		Log: LogConfig{
			Level:  "info",
			Format: "json",
			Output: "stdout",
		},
		Agent: AgentConfig{
			MaxIterations:       30,
			MaxConcurrentAgents: 5,
			Timeout:             10 * time.Minute,
			EnableStreaming:     true,
		},
		LLM: LLMConfig{
			DefaultProvider: "zhipu",
			DefaultModel:    "glm-4",
			Providers: map[string]*ProviderConfig{
				"zhipu": {
					Name:       "智谱AI",
					Type:       "zhipu",
					Enabled:    true,
					Priority:   1,
					BaseURL:    "https://open.bigmodel.cn/api/paas/v4",
					APIKeyEnv:  "ZHIPU_API_KEY",
					Model:      "glm-4",
					Timeout:    120 * time.Second,
					MaxRetries: 3,
				},
				"deepseek": {
					Name:       "DeepSeek",
					Type:       "deepseek",
					Enabled:    true,
					Priority:   2,
					BaseURL:    "https://api.deepseek.com/v1",
					APIKeyEnv:  "DEEPSEEK_API_KEY",
					Model:      "deepseek-chat",
					Timeout:    120 * time.Second,
					MaxRetries: 3,
				},
				"openai": {
					Name:       "OpenAI",
					Type:       "openai",
					Enabled:    true,
					Priority:   3,
					BaseURL:    "https://api.openai.com/v1",
					APIKeyEnv:  "OPENAI_API_KEY",
					Model:      "gpt-4-turbo",
					Timeout:    120 * time.Second,
					MaxRetries: 3,
				},
				"ollama": {
					Name:     "Ollama",
					Type:     "ollama",
					Enabled:  true,
					Priority: 10,
					BaseURL:  "http://localhost:11434",
					Model:    "llama3",
					Timeout:  300 * time.Second,
				},
			},
		},
		Interview: InterviewConfig{
			DefaultDuration: 60,
			MaxQuestions:    20,
			SkillCategories: []string{
				"programming",
				"framework",
				"database",
				"system_design",
				"algorithm",
				"soft_skills",
				"leadership",
			},
			DifficultyWeights: map[string]float64{
				"basic":        1.0,
				"intermediate": 1.5,
				"advanced":     2.0,
				"expert":       2.5,
			},
			EnableAdaptive: true,
			AdaptiveConfig: &AdaptiveConfig{
				MinQuestions:        5,
				MaxQuestions:        15,
				ConfidenceThreshold: 0.8,
				DifficultyAdjust:    true,
			},
			Scoring: ScoringConfig{
				PassThreshold:      0.6,
				WeightByDifficulty: true,
				RoundWeights: map[string]float64{
					"screening":      0.15,
					"technical":      0.40,
					"behavioral":     0.20,
					"system_design":  0.25,
				},
			},
		},
		Output: OutputConfig{
			Mode:            "detailed",
			IncludeDiagrams: true,
			DiagramStyle:    "mermaid",
			OutputDir:       "./output",
			ReportFormat:    "markdown",
		},
		Storage: StorageConfig{
			Type:     "sqlite",
			Path:     "./data/interview.db",
			CacheTTL: 24 * time.Hour,
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
				Name:        "interview-expert",
				Version:     "1.0.0",
				Description: "Interview Expert Agent MCP Server",
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

// applyEnvOverrides applies environment variable overrides.
func (c *Config) applyEnvOverrides() {
	for _, provider := range c.LLM.Providers {
		if provider.APIKeyEnv != "" {
			if apiKey := os.Getenv(provider.APIKeyEnv); apiKey != "" {
				provider.APIKey = apiKey
			}
		}
	}
}

// Validate validates the configuration.
func (c *Config) Validate() error {
	// Add validation logic here
	return nil
}
