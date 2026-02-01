/*
 * Copyright 2025 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config 全局配置结构
type Config struct {
	Server   ServerConfig   `yaml:"server"`
	LLM      LLMConfig      `yaml:"llm"`
	Image    ImageConfig    `yaml:"image"`
	Video    VideoConfig    `yaml:"video"`
	Storage  StorageConfig  `yaml:"storage"`
	Agent    AgentConfig    `yaml:"agent"`
	Logging  LoggingConfig  `yaml:"logging"`
}

// ServerConfig HTTP 服务器配置
type ServerConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
	Mode string `yaml:"mode"` // debug, release
}

// LLMConfig 大语言模型配置
type LLMConfig struct {
	Provider    string            `yaml:"provider"`     // openai, ark, qwen, deepseek
	Model       string            `yaml:"model"`
	APIKey      string            `yaml:"api_key"`
	BaseURL     string            `yaml:"base_url"`
	MaxTokens   int               `yaml:"max_tokens"`
	Temperature float64           `yaml:"temperature"`
	Timeout     int               `yaml:"timeout"`      // 秒
	Providers   map[string]LLMProviderConfig `yaml:"providers"`
}

// LLMProviderConfig 单个 LLM 提供商配置
type LLMProviderConfig struct {
	APIKey    string  `yaml:"api_key"`
	BaseURL   string  `yaml:"base_url"`
	Model     string  `yaml:"model"`
	Enabled   bool    `yaml:"enabled"`
}

// ImageConfig 图像生成配置
type ImageConfig struct {
	// 静态图生成
	StaticImage StaticImageConfig `yaml:"static_image"`
	// 动图生成
	AnimatedGIF AnimatedGIFConfig `yaml:"animated_gif"`
	// 表情包生成
	Sticker     StickerConfig     `yaml:"sticker"`
	// 输出配置
	Output      OutputConfig      `yaml:"output"`
}

// StaticImageConfig 静态图配置
type StaticImageConfig struct {
	Provider    string `yaml:"provider"`     // stability, dalle, midjourney
	APIKey      string `yaml:"api_key"`
	BaseURL     string `yaml:"base_url"`
	Model       string `yaml:"model"`
	DefaultSize string `yaml:"default_size"` // 1024x1024
	Quality     string `yaml:"quality"`      // standard, hd
}

// AnimatedGIFConfig 动图配置
type AnimatedGIFConfig struct {
	Provider     string `yaml:"provider"`      // runway, pika, stable_video
	APIKey       string `yaml:"api_key"`
	BaseURL      string `yaml:"base_url"`
	DefaultFPS   int    `yaml:"default_fps"`
	MaxDuration  int    `yaml:"max_duration"`  // 秒
	DefaultSize  string `yaml:"default_size"`
}

// StickerConfig 表情包配置
type StickerConfig struct {
	Provider     string `yaml:"provider"`
	APIKey       string `yaml:"api_key"`
	BaseURL      string `yaml:"base_url"`
	DefaultSize  string `yaml:"default_size"`  // 512x512
	WithText     bool   `yaml:"with_text"`     // 是否支持添加文字
	TextPosition string `yaml:"text_position"` // top, bottom, center
}

// OutputConfig 输出配置
type OutputConfig struct {
	Directory   string `yaml:"directory"`    // 输出目录
	MaxFileSize int64  `yaml:"max_file_size"` // 最大文件大小（字节）
	Format      string `yaml:"format"`        // png, jpg, gif, webp
}

// VideoConfig 视频生成配置
type VideoConfig struct {
	// 视频生成提供商
	Provider          string `yaml:"provider"`           // runway, pika, kling, minimax, stable_video
	APIKey            string `yaml:"api_key"`
	BaseURL           string `yaml:"base_url"`
	// 默认参数
	DefaultDuration   int    `yaml:"default_duration"`   // 默认时长（秒）
	MaxDuration       int    `yaml:"max_duration"`       // 最大时长（秒）
	DefaultFPS        int    `yaml:"default_fps"`        // 默认帧率
	DefaultResolution string `yaml:"default_resolution"` // 默认分辨率
	// 高级选项
	EnableTwoStep     bool   `yaml:"enable_two_step"`    // 是否启用两步生成（先图后视频）
	// 多提供商配置
	Providers         map[string]VideoProviderConfig `yaml:"providers"`
}

// VideoProviderConfig 视频提供商配置
type VideoProviderConfig struct {
	APIKey       string `yaml:"api_key"`
	BaseURL      string `yaml:"base_url"`
	Enabled      bool   `yaml:"enabled"`
	MaxDuration  int    `yaml:"max_duration"`
	SupportI2V   bool   `yaml:"support_i2v"`   // 支持图生视频
	SupportT2V   bool   `yaml:"support_t2v"`   // 支持文生视频
}

// StorageConfig 存储配置
type StorageConfig struct {
	Type     string `yaml:"type"`       // local, s3, oss
	BasePath string `yaml:"base_path"`
	// S3/OSS 配置
	Endpoint  string `yaml:"endpoint"`
	Bucket    string `yaml:"bucket"`
	AccessKey string `yaml:"access_key"`
	SecretKey string `yaml:"secret_key"`
	Region    string `yaml:"region"`
}

// AgentConfig Agent 配置
type AgentConfig struct {
	Name           string   `yaml:"name"`
	MaxIterations  int      `yaml:"max_iterations"`
	SystemPrompt   string   `yaml:"system_prompt"`
	EnabledTools   []string `yaml:"enabled_tools"`
	RetryCount     int      `yaml:"retry_count"`
	RetryDelay     int      `yaml:"retry_delay"` // 毫秒
}

// LoggingConfig 日志配置
type LoggingConfig struct {
	Level      string `yaml:"level"`       // debug, info, warn, error
	Format     string `yaml:"format"`      // json, text
	OutputPath string `yaml:"output_path"` // stdout, file path
	MaxSize    int    `yaml:"max_size"`    // MB
	MaxBackups int    `yaml:"max_backups"`
	MaxAge     int    `yaml:"max_age"`     // days
}

// LoadConfig 从文件加载配置
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// 支持环境变量替换
	data = []byte(os.ExpandEnv(string(data)))

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	// 设置默认值
	setDefaults(&cfg)

	return &cfg, nil
}

// setDefaults 设置默认配置值
func setDefaults(cfg *Config) {
	// Server 默认值
	if cfg.Server.Host == "" {
		cfg.Server.Host = "0.0.0.0"
	}
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 8080
	}
	if cfg.Server.Mode == "" {
		cfg.Server.Mode = "release"
	}

	// LLM 默认值
	if cfg.LLM.MaxTokens == 0 {
		cfg.LLM.MaxTokens = 4096
	}
	if cfg.LLM.Temperature == 0 {
		cfg.LLM.Temperature = 0.7
	}
	if cfg.LLM.Timeout == 0 {
		cfg.LLM.Timeout = 60
	}

	// Image 默认值
	if cfg.Image.StaticImage.DefaultSize == "" {
		cfg.Image.StaticImage.DefaultSize = "1024x1024"
	}
	if cfg.Image.StaticImage.Quality == "" {
		cfg.Image.StaticImage.Quality = "standard"
	}
	if cfg.Image.AnimatedGIF.DefaultFPS == 0 {
		cfg.Image.AnimatedGIF.DefaultFPS = 24
	}
	if cfg.Image.AnimatedGIF.MaxDuration == 0 {
		cfg.Image.AnimatedGIF.MaxDuration = 10
	}
	if cfg.Image.Sticker.DefaultSize == "" {
		cfg.Image.Sticker.DefaultSize = "512x512"
	}
	if cfg.Image.Output.Directory == "" {
		cfg.Image.Output.Directory = "./output"
	}
	if cfg.Image.Output.Format == "" {
		cfg.Image.Output.Format = "png"
	}

	// Video 默认值
	if cfg.Video.DefaultDuration == 0 {
		cfg.Video.DefaultDuration = 4
	}
	if cfg.Video.MaxDuration == 0 {
		cfg.Video.MaxDuration = 10
	}
	if cfg.Video.DefaultFPS == 0 {
		cfg.Video.DefaultFPS = 24
	}
	if cfg.Video.DefaultResolution == "" {
		cfg.Video.DefaultResolution = "1280x720"
	}

	// Agent 默认值
	if cfg.Agent.Name == "" {
		cfg.Agent.Name = "image-gen-agent"
	}
	if cfg.Agent.MaxIterations == 0 {
		cfg.Agent.MaxIterations = 10
	}
	if cfg.Agent.RetryCount == 0 {
		cfg.Agent.RetryCount = 3
	}
	if cfg.Agent.RetryDelay == 0 {
		cfg.Agent.RetryDelay = 1000
	}

	// Logging 默认值
	if cfg.Logging.Level == "" {
		cfg.Logging.Level = "info"
	}
	if cfg.Logging.Format == "" {
		cfg.Logging.Format = "json"
	}
	if cfg.Logging.OutputPath == "" {
		cfg.Logging.OutputPath = "stdout"
	}
}

// GetConfigPath 获取配置文件路径
func GetConfigPath() string {
	// 优先使用环境变量
	if path := os.Getenv("IMAGE_GEN_CONFIG"); path != "" {
		return path
	}

	// 其次使用当前目录
	if _, err := os.Stat("config.yaml"); err == nil {
		return "config.yaml"
	}

	// 最后使用默认路径
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".image-gen-agent", "config.yaml")
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	cfg := &Config{}
	setDefaults(cfg)
	return cfg
}
