// Package statistics 提供统计功能
package statistics

import (
	"sync"
	"time"
)

// TokenCounter Token计数器
type TokenCounter struct {
	mu       sync.RWMutex
	counters map[string]*ModelTokenStats
	pricing  map[string]TokenPricing
}

// ModelTokenStats 模型Token统计
type ModelTokenStats struct {
	Model        string    `json:"model"`
	InputTokens  int64     `json:"input_tokens"`
	OutputTokens int64     `json:"output_tokens"`
	TotalTokens  int64     `json:"total_tokens"`
	TotalCost    float64   `json:"total_cost"`
	RequestCount int64     `json:"request_count"`
	LastUpdated  time.Time `json:"last_updated"`
}

// TokenPricing Token定价 (每1000 tokens)
type TokenPricing struct {
	InputPricePerK  float64 `json:"input_price_per_k"`
	OutputPricePerK float64 `json:"output_price_per_k"`
}

// DefaultPricing 默认定价表
var DefaultPricing = map[string]TokenPricing{
	// OpenAI
	"gpt-4-turbo":   {InputPricePerK: 0.01, OutputPricePerK: 0.03},
	"gpt-4o":        {InputPricePerK: 0.005, OutputPricePerK: 0.015},
	"gpt-4o-mini":   {InputPricePerK: 0.00015, OutputPricePerK: 0.0006},
	"gpt-3.5-turbo": {InputPricePerK: 0.0005, OutputPricePerK: 0.0015},

	// Anthropic
	"claude-3-opus":   {InputPricePerK: 0.015, OutputPricePerK: 0.075},
	"claude-3-sonnet": {InputPricePerK: 0.003, OutputPricePerK: 0.015},
	"claude-3-haiku":  {InputPricePerK: 0.00025, OutputPricePerK: 0.00125},

	// 通义千问
	"qwen-turbo": {InputPricePerK: 0.001, OutputPricePerK: 0.002},
	"qwen-plus":  {InputPricePerK: 0.004, OutputPricePerK: 0.012},
	"qwen-max":   {InputPricePerK: 0.02, OutputPricePerK: 0.06},
	"qwen-long":  {InputPricePerK: 0.0005, OutputPricePerK: 0.002},

	// 智谱
	"glm-4":  {InputPricePerK: 0.01, OutputPricePerK: 0.01},
	"glm-4v": {InputPricePerK: 0.01, OutputPricePerK: 0.01},

	// Moonshot
	"moonshot-v1-8k":   {InputPricePerK: 0.012, OutputPricePerK: 0.012},
	"moonshot-v1-32k":  {InputPricePerK: 0.024, OutputPricePerK: 0.024},
	"moonshot-v1-128k": {InputPricePerK: 0.06, OutputPricePerK: 0.06},

	// DeepSeek
	"deepseek-chat":  {InputPricePerK: 0.001, OutputPricePerK: 0.002},
	"deepseek-coder": {InputPricePerK: 0.001, OutputPricePerK: 0.002},

	// 百川
	"baichuan-turbo": {InputPricePerK: 0.008, OutputPricePerK: 0.008},

	// 文心一言
	"ernie-4.0": {InputPricePerK: 0.12, OutputPricePerK: 0.12},
	"ernie-3.5": {InputPricePerK: 0.008, OutputPricePerK: 0.008},

	// 本地模型
	"local":  {InputPricePerK: 0, OutputPricePerK: 0},
	"ollama": {InputPricePerK: 0, OutputPricePerK: 0},
}

// NewTokenCounter 创建Token计数器
func NewTokenCounter() *TokenCounter {
	return &TokenCounter{
		counters: make(map[string]*ModelTokenStats),
		pricing:  DefaultPricing,
	}
}

// Record 记录Token使用
func (c *TokenCounter) Record(model string, inputTokens, outputTokens int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	stats, ok := c.counters[model]
	if !ok {
		stats = &ModelTokenStats{Model: model}
		c.counters[model] = stats
	}

	stats.InputTokens += int64(inputTokens)
	stats.OutputTokens += int64(outputTokens)
	stats.TotalTokens += int64(inputTokens + outputTokens)
	stats.RequestCount++
	stats.LastUpdated = time.Now()

	// 计算成本
	if pricing, ok := c.pricing[model]; ok {
		inputCost := float64(inputTokens) / 1000.0 * pricing.InputPricePerK
		outputCost := float64(outputTokens) / 1000.0 * pricing.OutputPricePerK
		stats.TotalCost += inputCost + outputCost
	}
}

// GetStats 获取模型统计
func (c *TokenCounter) GetStats(model string) *ModelTokenStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if stats, ok := c.counters[model]; ok {
		return &ModelTokenStats{
			Model:        stats.Model,
			InputTokens:  stats.InputTokens,
			OutputTokens: stats.OutputTokens,
			TotalTokens:  stats.TotalTokens,
			TotalCost:    stats.TotalCost,
			RequestCount: stats.RequestCount,
			LastUpdated:  stats.LastUpdated,
		}
	}
	return nil
}

// GetAllStats 获取所有统计
func (c *TokenCounter) GetAllStats() map[string]*ModelTokenStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make(map[string]*ModelTokenStats)
	for model, stats := range c.counters {
		result[model] = &ModelTokenStats{
			Model:        stats.Model,
			InputTokens:  stats.InputTokens,
			OutputTokens: stats.OutputTokens,
			TotalTokens:  stats.TotalTokens,
			TotalCost:    stats.TotalCost,
			RequestCount: stats.RequestCount,
			LastUpdated:  stats.LastUpdated,
		}
	}
	return result
}

// GetTotalCost 获取总成本
func (c *TokenCounter) GetTotalCost() float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var total float64
	for _, stats := range c.counters {
		total += stats.TotalCost
	}
	return total
}

// GetTotalTokens 获取总Token数
func (c *TokenCounter) GetTotalTokens() (input, output, total int64) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for _, stats := range c.counters {
		input += stats.InputTokens
		output += stats.OutputTokens
		total += stats.TotalTokens
	}
	return
}

// SetPricing 设置定价
func (c *TokenCounter) SetPricing(model string, pricing TokenPricing) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pricing[model] = pricing
}

// Reset 重置统计
func (c *TokenCounter) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.counters = make(map[string]*ModelTokenStats)
}

// TokenUsageReport Token使用报告
type TokenUsageReport struct {
	Period       string                       `json:"period"`
	StartTime    time.Time                    `json:"start_time"`
	EndTime      time.Time                    `json:"end_time"`
	ByModel      map[string]*ModelTokenStats  `json:"by_model"`
	ByIntent     map[string]*IntentTokenStats `json:"by_intent,omitempty"`
	TotalInput   int64                        `json:"total_input"`
	TotalOutput  int64                        `json:"total_output"`
	TotalCost    float64                      `json:"total_cost"`
	CacheSavings int64                        `json:"cache_savings"`
}

// IntentTokenStats 按意图统计
type IntentTokenStats struct {
	Intent        string  `json:"intent"`
	TokenCount    int64   `json:"token_count"`
	AvgPerRequest float64 `json:"avg_per_request"`
	RequestCount  int64   `json:"request_count"`
}

// GenerateReport 生成报告
func (c *TokenCounter) GenerateReport(startTime, endTime time.Time) *TokenUsageReport {
	c.mu.RLock()
	defer c.mu.RUnlock()

	report := &TokenUsageReport{
		Period:    "custom",
		StartTime: startTime,
		EndTime:   endTime,
		ByModel:   make(map[string]*ModelTokenStats),
	}

	for model, stats := range c.counters {
		if stats.LastUpdated.After(startTime) && stats.LastUpdated.Before(endTime) {
			report.ByModel[model] = stats
			report.TotalInput += stats.InputTokens
			report.TotalOutput += stats.OutputTokens
			report.TotalCost += stats.TotalCost
		}
	}

	return report
}
