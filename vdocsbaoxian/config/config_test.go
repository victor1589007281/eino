package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	// Check defaults
	if config.Log.Level != "info" {
		t.Errorf("Expected log level 'info', got '%s'", config.Log.Level)
	}
	if config.Index.ParallelWorkers != 8 {
		t.Errorf("Expected ParallelWorkers 8, got %d", config.Index.ParallelWorkers)
	}
	if config.Cache.L1.MaxSize != 10000 {
		t.Errorf("Expected L1 MaxSize 10000, got %d", config.Cache.L1.MaxSize)
	}
	if config.Storage.Primary != "sqlite" {
		t.Errorf("Expected storage primary 'sqlite', got '%s'", config.Storage.Primary)
	}
	if config.Stats.Token.Enabled != true {
		t.Error("Expected Token stats enabled to be true")
	}
	if config.Agent.MaxConcurrentAgents != 10 {
		t.Errorf("Expected MaxConcurrentAgents 10, got %d", config.Agent.MaxConcurrentAgents)
	}
	if config.LLM.DefaultProvider != "zhipu" {
		t.Errorf("Expected DefaultProvider 'zhipu', got '%s'", config.LLM.DefaultProvider)
	}
}

func TestLoadConfig_NonExistent(t *testing.T) {
	_, err := LoadConfig("/nonexistent/path/config.yaml")
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
}

func TestSaveAndLoadConfig(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Create config
	config := DefaultConfig()
	config.Log.Level = "debug"
	config.Cache.L1.MaxSize = 5000
	config.LLM.DefaultProvider = "deepseek"

	// Save config
	err := SaveConfig(config, configPath)
	if err != nil {
		t.Fatalf("SaveConfig failed: %v", err)
	}

	// Load config
	loaded, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Verify
	if loaded.Log.Level != "debug" {
		t.Errorf("Expected log level 'debug', got '%s'", loaded.Log.Level)
	}
	if loaded.Cache.L1.MaxSize != 5000 {
		t.Errorf("Expected L1 MaxSize 5000, got %d", loaded.Cache.L1.MaxSize)
	}
	if loaded.LLM.DefaultProvider != "deepseek" {
		t.Errorf("Expected provider 'deepseek', got '%s'", loaded.LLM.DefaultProvider)
	}
}

func TestConfigStructure(t *testing.T) {
	config := DefaultConfig()

	// Test LogConfig
	if config.Log.Output == "" {
		t.Error("Expected Log.Output to be set")
	}

	// Test DataSourcesConfig
	if config.DataSources.LegalDB == "" {
		t.Error("Expected DataSources.LegalDB to be set")
	}

	// Test IndexConfig
	if config.Index.Vector.Dimension != 1536 {
		t.Errorf("Expected Vector.Dimension 1536, got %d", config.Index.Vector.Dimension)
	}

	// Test CacheConfig
	if config.Cache.L1.TTL != 5*time.Minute {
		t.Errorf("Expected L1.TTL 5 minutes, got %v", config.Cache.L1.TTL)
	}
	if config.Cache.L2.TTL != 30*time.Minute {
		t.Errorf("Expected L2.TTL 30 minutes, got %v", config.Cache.L2.TTL)
	}

	// Test StorageConfig
	if config.Storage.SQLite == nil {
		t.Error("Expected Storage.SQLite to be set")
	}

	// Test StatsConfig
	if config.Stats.Token.DailyBudget != 1000000 {
		t.Errorf("Expected Token.DailyBudget 1000000, got %d", config.Stats.Token.DailyBudget)
	}

	// Test AgentConfig
	if config.Agent.MaxIterations == 0 {
		t.Error("Expected MaxIterations > 0")
	}

	// Test OutputConfig
	if config.Output.IncludeDiagrams != true {
		t.Error("Expected IncludeDiagrams to be true")
	}

	// Test LLMConfig
	if config.LLM.DefaultProvider == "" {
		t.Error("Expected DefaultProvider to be set")
	}

	// Test VerifyConfig
	if config.Verify.MinConfidence <= 0 {
		t.Error("Expected MinConfidence > 0")
	}
}

func TestProviderConfig(t *testing.T) {
	config := DefaultConfig()

	providers := config.LLM.Providers

	// Check Zhipu
	if zhipu, ok := providers["zhipu"]; ok {
		if zhipu.Type != "zhipu" {
			t.Errorf("Expected Zhipu type 'zhipu', got '%s'", zhipu.Type)
		}
		if zhipu.Models == nil || len(zhipu.Models) == 0 {
			t.Error("Expected Zhipu models to be configured")
		}
	} else {
		t.Error("Expected 'zhipu' provider")
	}

	// Check DeepSeek
	if deepseek, ok := providers["deepseek"]; ok {
		if deepseek.Type != "deepseek" {
			t.Errorf("Expected DeepSeek type 'deepseek', got '%s'", deepseek.Type)
		}
	} else {
		t.Error("Expected 'deepseek' provider")
	}

	// Check OpenAI
	if openai, ok := providers["openai"]; ok {
		if openai.Type != "openai" {
			t.Errorf("Expected OpenAI type 'openai', got '%s'", openai.Type)
		}
	} else {
		t.Error("Expected 'openai' provider")
	}
}

func TestAgentConfigValues(t *testing.T) {
	config := DefaultConfig()

	if config.Agent.MaxIterations != 20 {
		t.Errorf("Expected MaxIterations 20, got %d", config.Agent.MaxIterations)
	}
	if config.Agent.MaxConcurrentAgents != 10 {
		t.Errorf("Expected MaxConcurrentAgents 10, got %d", config.Agent.MaxConcurrentAgents)
	}
	if config.Agent.Timeout != 5*time.Minute {
		t.Errorf("Expected Timeout 5 minutes, got %v", config.Agent.Timeout)
	}
}

func TestValidateConfig(t *testing.T) {
	config := DefaultConfig()

	// Config should be valid
	if config.Log.Level == "" {
		t.Error("Log level should not be empty")
	}
	if config.Storage.Primary == "" {
		t.Error("Storage primary should not be empty")
	}
	if config.LLM.DefaultProvider == "" {
		t.Error("Default provider should not be empty")
	}
}

func TestConfigFromEnv(t *testing.T) {
	// Set environment variables
	os.Setenv("DEEPSEEK_API_KEY", "test-deepseek-key")
	os.Setenv("ZHIPU_API_KEY", "test-zhipu-key")
	defer os.Unsetenv("DEEPSEEK_API_KEY")
	defer os.Unsetenv("ZHIPU_API_KEY")

	config := DefaultConfig()

	// Check API key env references
	if config.LLM.Providers["deepseek"].APIKeyEnv != "DEEPSEEK_API_KEY" {
		t.Errorf("Expected APIKeyEnv 'DEEPSEEK_API_KEY', got '%s'", config.LLM.Providers["deepseek"].APIKeyEnv)
	}
	if config.LLM.Providers["zhipu"].APIKeyEnv != "ZHIPU_API_KEY" {
		t.Errorf("Expected APIKeyEnv 'ZHIPU_API_KEY', got '%s'", config.LLM.Providers["zhipu"].APIKeyEnv)
	}
}

func TestRoutingConfig(t *testing.T) {
	config := DefaultConfig()

	if config.LLM.Routing == nil {
		t.Fatal("Expected Routing config")
	}

	if !config.LLM.Routing.Enabled {
		t.Error("Expected Routing to be enabled")
	}

	if config.LLM.Routing.Strategy != "smart" {
		t.Errorf("Expected Strategy 'smart', got '%s'", config.LLM.Routing.Strategy)
	}

	if len(config.LLM.Routing.SceneMapping) == 0 {
		t.Error("Expected some scene mappings")
	}
}

func TestFallbackConfig(t *testing.T) {
	config := DefaultConfig()

	if config.LLM.Fallback == nil {
		t.Fatal("Expected Fallback config")
	}

	if !config.LLM.Fallback.Enabled {
		t.Error("Expected Fallback to be enabled")
	}

	if config.LLM.Fallback.MaxAttempts != 3 {
		t.Errorf("Expected MaxAttempts 3, got %d", config.LLM.Fallback.MaxAttempts)
	}
}

func TestInteractionConfig(t *testing.T) {
	config := DefaultConfig()

	// Check REST server
	if config.Interaction.RESTServer == nil {
		t.Fatal("Expected RESTServer config")
	}
	if !config.Interaction.RESTServer.Enabled {
		t.Error("Expected RESTServer to be enabled")
	}

	// Check MCP server
	if config.Interaction.MCPServer == nil {
		t.Fatal("Expected MCPServer config")
	}
	if !config.Interaction.MCPServer.Enabled {
		t.Error("Expected MCPServer to be enabled")
	}
}

func TestGetEnvOrDefault(t *testing.T) {
	// Test with env var set
	os.Setenv("TEST_VAR", "test_value")
	defer os.Unsetenv("TEST_VAR")

	result := GetEnvOrDefault("TEST_VAR", "default")
	if result != "test_value" {
		t.Errorf("Expected 'test_value', got '%s'", result)
	}

	// Test with env var not set
	result = GetEnvOrDefault("NON_EXISTENT_VAR", "default")
	if result != "default" {
		t.Errorf("Expected 'default', got '%s'", result)
	}
}

func TestConfigValidate(t *testing.T) {
	config := DefaultConfig()

	err := config.Validate()
	if err != nil {
		t.Errorf("Default config should be valid: %v", err)
	}
}
