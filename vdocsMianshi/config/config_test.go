package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	assert.NotNil(t, cfg)
	assert.Equal(t, "info", cfg.Log.Level)
	assert.Equal(t, "json", cfg.Log.Format)
	assert.Equal(t, 30, cfg.Agent.MaxIterations)
	assert.Equal(t, 10*time.Minute, cfg.Agent.Timeout)
	assert.True(t, cfg.Agent.EnableStreaming)
}

func TestDefaultConfig_LLMProviders(t *testing.T) {
	cfg := DefaultConfig()

	assert.NotNil(t, cfg.LLM.Providers)
	assert.Contains(t, cfg.LLM.Providers, "zhipu")
	assert.Contains(t, cfg.LLM.Providers, "deepseek")
	assert.Contains(t, cfg.LLM.Providers, "openai")
	assert.Contains(t, cfg.LLM.Providers, "ollama")

	// Check zhipu config
	zhipu := cfg.LLM.Providers["zhipu"]
	assert.True(t, zhipu.Enabled)
	assert.Equal(t, "zhipu", zhipu.Type)
	assert.Equal(t, "glm-4", zhipu.Model)
}

func TestDefaultConfig_InterviewSettings(t *testing.T) {
	cfg := DefaultConfig()

	assert.Equal(t, 60, cfg.Interview.DefaultDuration)
	assert.Equal(t, 20, cfg.Interview.MaxQuestions)
	assert.True(t, cfg.Interview.EnableAdaptive)
	assert.NotNil(t, cfg.Interview.AdaptiveConfig)
	assert.Equal(t, 0.6, cfg.Interview.Scoring.PassThreshold)
}

func TestDefaultConfig_InteractionSettings(t *testing.T) {
	cfg := DefaultConfig()

	assert.NotNil(t, cfg.Interaction.RESTServer)
	assert.True(t, cfg.Interaction.RESTServer.Enabled)
	assert.Equal(t, ":8080", cfg.Interaction.RESTServer.Address)
	assert.Equal(t, "/api/v1", cfg.Interaction.RESTServer.BasePath)
}

func TestLoadConfig_FileNotFound(t *testing.T) {
	_, err := LoadConfig("nonexistent.yaml")
	assert.Error(t, err)
}

func TestLoadConfig_ValidFile(t *testing.T) {
	// Create a temporary config file
	content := `
log:
  level: debug
  format: text
agent:
  max_iterations: 50
  timeout: 5m
llm:
  default_provider: openai
`
	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(content)
	require.NoError(t, err)
	tmpFile.Close()

	// Load config
	cfg, err := LoadConfig(tmpFile.Name())
	require.NoError(t, err)

	assert.Equal(t, "debug", cfg.Log.Level)
	assert.Equal(t, "text", cfg.Log.Format)
	assert.Equal(t, 50, cfg.Agent.MaxIterations)
	assert.Equal(t, "openai", cfg.LLM.DefaultProvider)
}

func TestApplyEnvOverrides(t *testing.T) {
	cfg := DefaultConfig()

	// Set environment variable
	os.Setenv("ZHIPU_API_KEY", "test-api-key")
	defer os.Unsetenv("ZHIPU_API_KEY")

	cfg.applyEnvOverrides()

	zhipu := cfg.LLM.Providers["zhipu"]
	assert.Equal(t, "test-api-key", zhipu.APIKey)
}

func TestSaveConfig(t *testing.T) {
	cfg := DefaultConfig()

	tmpFile, err := os.CreateTemp("", "config-save-*.yaml")
	require.NoError(t, err)
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	// Save config
	err = SaveConfig(cfg, tmpFile.Name())
	require.NoError(t, err)

	// Load and verify
	loadedCfg, err := LoadConfig(tmpFile.Name())
	require.NoError(t, err)

	assert.Equal(t, cfg.Log.Level, loadedCfg.Log.Level)
	assert.Equal(t, cfg.Agent.MaxIterations, loadedCfg.Agent.MaxIterations)
}

func TestConfig_Validate(t *testing.T) {
	cfg := DefaultConfig()
	err := cfg.Validate()
	assert.NoError(t, err)
}
