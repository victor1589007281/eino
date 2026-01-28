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

package stats

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/cloudwego/eino/vdocswx/config"
)

// Collector 统计收集器
type Collector struct {
	config     *config.StatsConfig
	tokenStats *TokenStats
	cacheStats *CacheStats
	modelStats *ModelStats
	agentStats *AgentStats
	mu         sync.RWMutex
	stopCh     chan struct{}
}

// TokenStats Token统计
type TokenStats struct {
	TotalInput  int64            `json:"total_input"`
	TotalOutput int64            `json:"total_output"`
	ByModel     map[string]int64 `json:"by_model"`
	ByAgent     map[string]int64 `json:"by_agent"`
	ByRequest   map[string]int64 `json:"by_request"`
	mu          sync.RWMutex
}

// CacheStats 缓存统计
type CacheStats struct {
	TotalHits   int64                    `json:"total_hits"`
	TotalMisses int64                    `json:"total_misses"`
	HitRate     float64                  `json:"hit_rate"`
	ByType      map[string]*CacheTypeStats `json:"by_type"`
	mu          sync.RWMutex
}

// CacheTypeStats 按类型的缓存统计
type CacheTypeStats struct {
	Hits   int64 `json:"hits"`
	Misses int64 `json:"misses"`
	Size   int64 `json:"size"`
}

// ModelStats 模型统计
type ModelStats struct {
	Requests   map[string]int64          `json:"requests"`
	Latency    map[string]time.Duration  `json:"latency"`
	Errors     map[string]int64          `json:"errors"`
	TokenUsage map[string]*TokenUsage    `json:"token_usage"`
	mu         sync.RWMutex
}

// TokenUsage Token使用情况
type TokenUsage struct {
	Input     int64   `json:"input"`
	Output    int64   `json:"output"`
	TotalCost float64 `json:"total_cost"`
}

// AgentStats Agent统计
type AgentStats struct {
	Invocations map[string]int64         `json:"invocations"`
	Latency     map[string]time.Duration `json:"latency"`
	Errors      map[string]int64         `json:"errors"`
	mu          sync.RWMutex
}

// Summary 统计摘要
type Summary struct {
	Timestamp    time.Time    `json:"timestamp"`
	Token        *TokenStats  `json:"token"`
	Cache        *CacheStats  `json:"cache"`
	Model        *ModelStats  `json:"model"`
	Agent        *AgentStats  `json:"agent"`
	TotalCost    float64      `json:"total_cost"`
	TokensSaved  int64        `json:"tokens_saved"`
	CacheHitRate float64      `json:"cache_hit_rate"`
}

// NewCollector 创建统计收集器
func NewCollector(cfg *config.StatsConfig) *Collector {
	c := &Collector{
		config: cfg,
		tokenStats: &TokenStats{
			ByModel:   make(map[string]int64),
			ByAgent:   make(map[string]int64),
			ByRequest: make(map[string]int64),
		},
		cacheStats: &CacheStats{
			ByType: make(map[string]*CacheTypeStats),
		},
		modelStats: &ModelStats{
			Requests:   make(map[string]int64),
			Latency:    make(map[string]time.Duration),
			Errors:     make(map[string]int64),
			TokenUsage: make(map[string]*TokenUsage),
		},
		agentStats: &AgentStats{
			Invocations: make(map[string]int64),
			Latency:     make(map[string]time.Duration),
			Errors:      make(map[string]int64),
		},
		stopCh: make(chan struct{}),
	}

	// 启动定期导出
	if cfg.Enabled {
		go c.exportLoop()
	}

	return c
}

// RecordTokenUsage 记录Token使用
func (c *Collector) RecordTokenUsage(model, agent string, input, output int64) {
	c.tokenStats.mu.Lock()
	defer c.tokenStats.mu.Unlock()

	c.tokenStats.TotalInput += input
	c.tokenStats.TotalOutput += output
	c.tokenStats.ByModel[model] += input + output
	c.tokenStats.ByAgent[agent] += input + output
}

// RecordCacheHit 记录缓存命中
func (c *Collector) RecordCacheHit(cacheType string, hit bool) {
	c.cacheStats.mu.Lock()
	defer c.cacheStats.mu.Unlock()

	if c.cacheStats.ByType[cacheType] == nil {
		c.cacheStats.ByType[cacheType] = &CacheTypeStats{}
	}

	if hit {
		c.cacheStats.TotalHits++
		c.cacheStats.ByType[cacheType].Hits++
	} else {
		c.cacheStats.TotalMisses++
		c.cacheStats.ByType[cacheType].Misses++
	}

	// 更新命中率
	total := c.cacheStats.TotalHits + c.cacheStats.TotalMisses
	if total > 0 {
		c.cacheStats.HitRate = float64(c.cacheStats.TotalHits) / float64(total)
	}
}

// RecordModelRequest 记录模型请求
func (c *Collector) RecordModelRequest(model string, latency time.Duration, err error, input, output int64) {
	c.modelStats.mu.Lock()
	defer c.modelStats.mu.Unlock()

	c.modelStats.Requests[model]++

	// 更新平均延迟
	if c.modelStats.Latency[model] == 0 {
		c.modelStats.Latency[model] = latency
	} else {
		c.modelStats.Latency[model] = (c.modelStats.Latency[model] + latency) / 2
	}

	if err != nil {
		c.modelStats.Errors[model]++
	}

	if c.modelStats.TokenUsage[model] == nil {
		c.modelStats.TokenUsage[model] = &TokenUsage{}
	}
	c.modelStats.TokenUsage[model].Input += input
	c.modelStats.TokenUsage[model].Output += output
}

// RecordAgentInvocation 记录Agent调用
func (c *Collector) RecordAgentInvocation(agent string, latency time.Duration, err error) {
	c.agentStats.mu.Lock()
	defer c.agentStats.mu.Unlock()

	c.agentStats.Invocations[agent]++

	// 更新平均延迟
	if c.agentStats.Latency[agent] == 0 {
		c.agentStats.Latency[agent] = latency
	} else {
		c.agentStats.Latency[agent] = (c.agentStats.Latency[agent] + latency) / 2
	}

	if err != nil {
		c.agentStats.Errors[agent]++
	}
}

// GetSummary 获取统计摘要
func (c *Collector) GetSummary() *Summary {
	c.mu.RLock()
	defer c.mu.RUnlock()

	summary := &Summary{
		Timestamp:    time.Now(),
		CacheHitRate: c.cacheStats.HitRate,
	}

	// 复制token统计
	c.tokenStats.mu.RLock()
	summary.Token = &TokenStats{
		TotalInput:  c.tokenStats.TotalInput,
		TotalOutput: c.tokenStats.TotalOutput,
		ByModel:     copyMap(c.tokenStats.ByModel),
		ByAgent:     copyMap(c.tokenStats.ByAgent),
	}
	c.tokenStats.mu.RUnlock()

	// 复制缓存统计
	c.cacheStats.mu.RLock()
	summary.Cache = &CacheStats{
		TotalHits:   c.cacheStats.TotalHits,
		TotalMisses: c.cacheStats.TotalMisses,
		HitRate:     c.cacheStats.HitRate,
	}
	c.cacheStats.mu.RUnlock()

	// 计算Token节省（基于缓存命中）
	summary.TokensSaved = c.cacheStats.TotalHits * 1000 // 估算

	return summary
}

// Export 导出统计数据
func (c *Collector) Export() error {
	summary := c.GetSummary()

	data, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal stats: %w", err)
	}

	// 确保目录存在
	if err := os.MkdirAll(c.config.ExportPath, 0755); err != nil {
		return fmt.Errorf("failed to create export directory: %w", err)
	}

	filename := filepath.Join(
		c.config.ExportPath,
		fmt.Sprintf("stats_%s.json", time.Now().Format("20060102_150405")),
	)

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write stats file: %w", err)
	}

	return nil
}

// exportLoop 定期导出循环
func (c *Collector) exportLoop() {
	ticker := time.NewTicker(c.config.ExportInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := c.Export(); err != nil {
				fmt.Printf("Failed to export stats: %v\n", err)
			}
		case <-c.stopCh:
			return
		}
	}
}

// Stop 停止收集器
func (c *Collector) Stop() {
	close(c.stopCh)
}

// Reset 重置统计
func (c *Collector) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.tokenStats = &TokenStats{
		ByModel:   make(map[string]int64),
		ByAgent:   make(map[string]int64),
		ByRequest: make(map[string]int64),
	}
	c.cacheStats = &CacheStats{
		ByType: make(map[string]*CacheTypeStats),
	}
	c.modelStats = &ModelStats{
		Requests:   make(map[string]int64),
		Latency:    make(map[string]time.Duration),
		Errors:     make(map[string]int64),
		TokenUsage: make(map[string]*TokenUsage),
	}
	c.agentStats = &AgentStats{
		Invocations: make(map[string]int64),
		Latency:     make(map[string]time.Duration),
		Errors:      make(map[string]int64),
	}
}

func copyMap(m map[string]int64) map[string]int64 {
	result := make(map[string]int64, len(m))
	for k, v := range m {
		result[k] = v
	}
	return result
}
