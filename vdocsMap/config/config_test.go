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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig(t *testing.T) {
	// 创建临时配置文件
	content := `
server:
  host: "127.0.0.1"
  port: 9090
  mode: "debug"

llm:
  provider: "openai"
  model: "gpt-4"
  api_key: "test-key"
  max_tokens: 2048
  temperature: 0.8

image:
  static_image:
    provider: "dalle"
    default_size: "512x512"

agent:
  name: "test-agent"
  max_iterations: 5
`

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	err := os.WriteFile(configPath, []byte(content), 0644)
	require.NoError(t, err)

	cfg, err := LoadConfig(configPath)
	require.NoError(t, err)

	assert.Equal(t, "127.0.0.1", cfg.Server.Host)
	assert.Equal(t, 9090, cfg.Server.Port)
	assert.Equal(t, "debug", cfg.Server.Mode)

	assert.Equal(t, "openai", cfg.LLM.Provider)
	assert.Equal(t, "gpt-4", cfg.LLM.Model)
	assert.Equal(t, "test-key", cfg.LLM.APIKey)
	assert.Equal(t, 2048, cfg.LLM.MaxTokens)
	assert.Equal(t, 0.8, cfg.LLM.Temperature)

	assert.Equal(t, "dalle", cfg.Image.StaticImage.Provider)
	assert.Equal(t, "512x512", cfg.Image.StaticImage.DefaultSize)

	assert.Equal(t, "test-agent", cfg.Agent.Name)
	assert.Equal(t, 5, cfg.Agent.MaxIterations)
}

func TestLoadConfig_WithEnvVars(t *testing.T) {
	// 设置环境变量
	os.Setenv("TEST_API_KEY", "env-api-key")
	defer os.Unsetenv("TEST_API_KEY")

	content := `
llm:
  api_key: "${TEST_API_KEY}"
`

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	err := os.WriteFile(configPath, []byte(content), 0644)
	require.NoError(t, err)

	cfg, err := LoadConfig(configPath)
	require.NoError(t, err)

	assert.Equal(t, "env-api-key", cfg.LLM.APIKey)
}

func TestLoadConfig_FileNotFound(t *testing.T) {
	_, err := LoadConfig("/nonexistent/path/config.yaml")
	assert.Error(t, err)
}

func TestLoadConfig_InvalidYAML(t *testing.T) {
	content := `
invalid yaml content
  - not proper
    yaml: format
  invalid
`

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	err := os.WriteFile(configPath, []byte(content), 0644)
	require.NoError(t, err)

	_, err = LoadConfig(configPath)
	assert.Error(t, err)
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	// 验证默认值
	assert.Equal(t, "0.0.0.0", cfg.Server.Host)
	assert.Equal(t, 8080, cfg.Server.Port)
	assert.Equal(t, "release", cfg.Server.Mode)

	assert.Equal(t, 4096, cfg.LLM.MaxTokens)
	assert.Equal(t, 0.7, cfg.LLM.Temperature)
	assert.Equal(t, 60, cfg.LLM.Timeout)

	assert.Equal(t, "1024x1024", cfg.Image.StaticImage.DefaultSize)
	assert.Equal(t, "standard", cfg.Image.StaticImage.Quality)
	assert.Equal(t, 24, cfg.Image.AnimatedGIF.DefaultFPS)
	assert.Equal(t, 10, cfg.Image.AnimatedGIF.MaxDuration)
	assert.Equal(t, "512x512", cfg.Image.Sticker.DefaultSize)

	assert.Equal(t, "image-gen-agent", cfg.Agent.Name)
	assert.Equal(t, 10, cfg.Agent.MaxIterations)
	assert.Equal(t, 3, cfg.Agent.RetryCount)
	assert.Equal(t, 1000, cfg.Agent.RetryDelay)

	assert.Equal(t, "info", cfg.Logging.Level)
	assert.Equal(t, "json", cfg.Logging.Format)
	assert.Equal(t, "stdout", cfg.Logging.OutputPath)
}

func TestSetDefaults(t *testing.T) {
	cfg := &Config{}
	setDefaults(cfg)

	// 确保所有必要的默认值都被设置
	assert.NotEmpty(t, cfg.Server.Host)
	assert.NotZero(t, cfg.Server.Port)
	assert.NotEmpty(t, cfg.Server.Mode)

	assert.NotZero(t, cfg.LLM.MaxTokens)
	assert.NotZero(t, cfg.LLM.Temperature)
	assert.NotZero(t, cfg.LLM.Timeout)

	assert.NotEmpty(t, cfg.Image.StaticImage.DefaultSize)
	assert.NotEmpty(t, cfg.Image.Output.Directory)
}

func TestGetConfigPath(t *testing.T) {
	// 测试环境变量优先
	os.Setenv("IMAGE_GEN_CONFIG", "/custom/path/config.yaml")
	defer os.Unsetenv("IMAGE_GEN_CONFIG")

	path := GetConfigPath()
	assert.Equal(t, "/custom/path/config.yaml", path)
}

func TestGetConfigPath_Default(t *testing.T) {
	os.Unsetenv("IMAGE_GEN_CONFIG")

	// 在没有环境变量和本地文件的情况下
	path := GetConfigPath()
	assert.Contains(t, path, "config.yaml")
}

func TestLoadConfig_PartialConfig(t *testing.T) {
	// 只提供部分配置，其他应该使用默认值
	content := `
server:
  port: 3000
llm:
  provider: "ark"
`

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	err := os.WriteFile(configPath, []byte(content), 0644)
	require.NoError(t, err)

	cfg, err := LoadConfig(configPath)
	require.NoError(t, err)

	// 提供的值
	assert.Equal(t, 3000, cfg.Server.Port)
	assert.Equal(t, "ark", cfg.LLM.Provider)

	// 默认值
	assert.Equal(t, "0.0.0.0", cfg.Server.Host)
	assert.Equal(t, "release", cfg.Server.Mode)
	assert.Equal(t, 4096, cfg.LLM.MaxTokens)
}
