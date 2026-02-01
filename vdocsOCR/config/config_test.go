package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	// Test server config
	if cfg.Server.Port != 8080 {
		t.Errorf("Expected server port 8080, got %d", cfg.Server.Port)
	}
	if cfg.Server.ReadTimeout != 30*time.Second {
		t.Errorf("Expected read timeout 30s, got %v", cfg.Server.ReadTimeout)
	}

	// Test LLM config
	if cfg.LLM.DefaultProvider != "deepseek" {
		t.Errorf("Expected default provider deepseek, got %s", cfg.LLM.DefaultProvider)
	}
	if cfg.LLM.Temperature != 0.1 {
		t.Errorf("Expected temperature 0.1, got %f", cfg.LLM.Temperature)
	}

	// Test OCR config
	if cfg.OCR.Engine != "tesseract" {
		t.Errorf("Expected OCR engine tesseract, got %s", cfg.OCR.Engine)
	}
	if cfg.OCR.Language != "chi_sim+eng" {
		t.Errorf("Expected OCR language chi_sim+eng, got %s", cfg.OCR.Language)
	}
	if cfg.OCR.MaxConcurrency != 4 {
		t.Errorf("Expected max concurrency 4, got %d", cfg.OCR.MaxConcurrency)
	}

	// Test storage config
	if cfg.Storage.Type != "sqlite" {
		t.Errorf("Expected storage type sqlite, got %s", cfg.Storage.Type)
	}

	// Test cache config
	if !cfg.Cache.Enabled {
		t.Error("Expected cache to be enabled")
	}
	if cfg.Cache.Type != "memory" {
		t.Errorf("Expected cache type memory, got %s", cfg.Cache.Type)
	}
}

func TestLoadConfig(t *testing.T) {
	// Create temp config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
server:
  host: "127.0.0.1"
  port: 9090

llm:
  default_provider: "deepseek"
  providers:
    deepseek:
      type: "deepseek"
      enabled: true
      model: "deepseek-chat"
      max_tokens: 2048

ocr:
  engine: "tesseract"
  language: "eng"
  max_concurrency: 2

storage:
  type: "sqlite"
  data_dir: "./test-data"

cache:
  enabled: true
  type: "memory"
`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.Server.Port != 9090 {
		t.Errorf("Expected server port 9090, got %d", cfg.Server.Port)
	}
	if cfg.Server.Host != "127.0.0.1" {
		t.Errorf("Expected server host 127.0.0.1, got %s", cfg.Server.Host)
	}
	if cfg.OCR.Language != "eng" {
		t.Errorf("Expected OCR language eng, got %s", cfg.OCR.Language)
	}
	if cfg.OCR.MaxConcurrency != 2 {
		t.Errorf("Expected max concurrency 2, got %d", cfg.OCR.MaxConcurrency)
	}
}

func TestLoadConfigFromEnv(t *testing.T) {
	// Set environment variables
	os.Setenv("DEEPSEEK_API_KEY", "test-deepseek-key")
	os.Setenv("PORT", "8888")
	os.Setenv("LOG_LEVEL", "debug")
	defer func() {
		os.Unsetenv("DEEPSEEK_API_KEY")
		os.Unsetenv("PORT")
		os.Unsetenv("LOG_LEVEL")
	}()

	cfg, err := LoadConfigFromEnv()
	if err != nil {
		t.Fatalf("Failed to load config from env: %v", err)
	}

	if cfg.LLM.Providers["deepseek"].APIKey != "test-deepseek-key" {
		t.Error("Expected deepseek API key to be set from environment")
	}
	if cfg.Server.Port != 8888 {
		t.Errorf("Expected server port 8888, got %d", cfg.Server.Port)
	}
	if cfg.Logging.Level != "debug" {
		t.Errorf("Expected log level debug, got %s", cfg.Logging.Level)
	}
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name      string
		modify    func(*Config)
		expectErr bool
	}{
		{
			name:      "valid default config",
			modify:    func(c *Config) {},
			expectErr: false,
		},
		{
			name: "invalid port",
			modify: func(c *Config) {
				c.Server.Port = -1
			},
			expectErr: true,
		},
		{
			name: "empty default provider",
			modify: func(c *Config) {
				c.LLM.DefaultProvider = ""
			},
			expectErr: true,
		},
		{
			name: "non-existent default provider",
			modify: func(c *Config) {
				c.LLM.DefaultProvider = "nonexistent"
			},
			expectErr: true,
		},
		{
			name: "no enabled providers",
			modify: func(c *Config) {
				for _, p := range c.LLM.Providers {
					p.Enabled = false
				}
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			tt.modify(cfg)

			err := cfg.Validate()
			if tt.expectErr && err == nil {
				t.Error("Expected validation error, got nil")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("Expected no validation error, got: %v", err)
			}
		})
	}
}

func TestSaveConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "saved_config.yaml")

	cfg := DefaultConfig()
	cfg.Server.Port = 9999
	cfg.LLM.Temperature = 0.5

	if err := cfg.SaveConfig(configPath); err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	// Load saved config
	loaded, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load saved config: %v", err)
	}

	if loaded.Server.Port != 9999 {
		t.Errorf("Expected server port 9999, got %d", loaded.Server.Port)
	}
	if loaded.LLM.Temperature != 0.5 {
		t.Errorf("Expected temperature 0.5, got %f", loaded.LLM.Temperature)
	}
}

func TestGetProviderConfig(t *testing.T) {
	cfg := DefaultConfig()

	// Test existing provider
	provider, err := cfg.GetProviderConfig("deepseek")
	if err != nil {
		t.Fatalf("Failed to get provider config: %v", err)
	}
	if provider.Model != "deepseek-chat" {
		t.Errorf("Expected model deepseek-chat, got %s", provider.Model)
	}

	// Test non-existent provider
	_, err = cfg.GetProviderConfig("nonexistent")
	if err == nil {
		t.Error("Expected error for non-existent provider")
	}
}

func TestGetOCRAPIConfig(t *testing.T) {
	cfg := DefaultConfig()

	// Test existing provider
	provider, err := cfg.GetOCRAPIConfig("baidu")
	if err != nil {
		t.Fatalf("Failed to get OCR API config: %v", err)
	}
	if provider.Provider != "baidu" {
		t.Errorf("Expected provider baidu, got %s", provider.Provider)
	}

	// Test non-existent provider
	_, err = cfg.GetOCRAPIConfig("nonexistent")
	if err == nil {
		t.Error("Expected error for non-existent OCR API provider")
	}
}

func TestInvoiceConfigTypes(t *testing.T) {
	cfg := DefaultConfig()

	expectedTypes := []string{
		"vat_invoice",
		"vat_special_invoice",
		"receipt",
		"taxi_receipt",
		"train_ticket",
		"air_ticket",
		"hotel_invoice",
		"toll_invoice",
	}

	for _, expected := range expectedTypes {
		found := false
		for _, t := range cfg.OCR.InvoiceConfig.Types {
			if t == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected invoice type %s not found", expected)
		}
	}
}

func TestInvoiceExtractFields(t *testing.T) {
	cfg := DefaultConfig()

	expectedFields := []string{
		"invoice_code",
		"invoice_number",
		"invoice_date",
		"amount",
		"total_amount",
	}

	for _, expected := range expectedFields {
		found := false
		for _, f := range cfg.OCR.InvoiceConfig.ExtractFields {
			if f == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected extract field %s not found", expected)
		}
	}
}
