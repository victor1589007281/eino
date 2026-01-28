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

// Package config provides configuration management for the Linux kernel expert agent.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config represents the application configuration.
type Config struct {
	// Source paths
	SourcePath string `json:"source_path"` // Linux kernel source path
	IndexPath  string `json:"index_path"`  // Index storage path
	SkillPath  string `json:"skill_path"`  // Skill files path

	// Model configuration
	ModelProvider string  `json:"model_provider"` // openai, anthropic, etc.
	ModelName     string  `json:"model_name"`     // gpt-4, claude-3, etc.
	Temperature   float64 `json:"temperature"`
	MaxTokens     int     `json:"max_tokens"`

	// Agent configuration
	MaxIterations int `json:"max_iterations"`
	MaxWorkers    int `json:"max_workers"`

	// Index configuration
	IndexWorkers int `json:"index_workers"`

	// Search configuration
	MaxSearchResults int `json:"max_search_results"`
	SearchTimeout    int `json:"search_timeout"` // seconds

	// Output configuration
	DefaultOutputType string `json:"default_output_type"` // summary or document
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	return &Config{
		SourcePath:        "/Users/huaquan.liang/Documents/GitHub/linux",
		IndexPath:         "./index",
		SkillPath:         "./skills",
		ModelProvider:     "openai",
		ModelName:         "gpt-4-turbo",
		Temperature:       0.1,
		MaxTokens:         4096,
		MaxIterations:     30,
		MaxWorkers:        4,
		IndexWorkers:      4,
		MaxSearchResults:  50,
		SearchTimeout:     30,
		DefaultOutputType: "document",
	}
}

// LoadConfig loads configuration from a file.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return nil, err
	}

	config := DefaultConfig()
	if err := json.Unmarshal(data, config); err != nil {
		return nil, err
	}

	return config, nil
}

// SaveConfig saves configuration to a file.
func SaveConfig(config *Config, path string) error {
	// Create directory if needed
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// LoadFromEnv loads configuration from environment variables.
func LoadFromEnv(config *Config) {
	if v := os.Getenv("KERNEL_SOURCE_PATH"); v != "" {
		config.SourcePath = v
	}
	if v := os.Getenv("KERNEL_INDEX_PATH"); v != "" {
		config.IndexPath = v
	}
	if v := os.Getenv("MODEL_PROVIDER"); v != "" {
		config.ModelProvider = v
	}
	if v := os.Getenv("MODEL_NAME"); v != "" {
		config.ModelName = v
	}
}
