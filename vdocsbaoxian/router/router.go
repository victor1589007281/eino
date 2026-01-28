// Package router provides model routing and selection.
package router

import (
	"context"
	"errors"
	"sync"
	"time"
)

// TaskType represents the type of task.
type TaskType string

const (
	TaskTypeSimple     TaskType = "simple"      // Simple questions, FAQ
	TaskTypeReasoning  TaskType = "reasoning"   // Complex reasoning
	TaskTypeAnalysis   TaskType = "analysis"    // Document analysis
	TaskTypeGeneration TaskType = "generation"  // Content generation
	TaskTypeVerify     TaskType = "verify"      // Fact verification
	TaskTypeEmbed      TaskType = "embed"       // Embedding generation
)

// Provider represents an LLM provider.
type Provider string

const (
	// Chinese providers
	ProviderDeepSeek    Provider = "deepseek"
	ProviderQwen        Provider = "qwen"
	ProviderErnie       Provider = "ernie"
	ProviderGLM         Provider = "glm"
	ProviderMoonshot    Provider = "moonshot"
	ProviderDoubao      Provider = "doubao"
	ProviderMiniMax     Provider = "minimax"
	ProviderBaichuan    Provider = "baichuan"
	Provider01AI        Provider = "01ai"
	ProviderZhipu       Provider = "zhipu"
	
	// International providers
	ProviderOpenAI      Provider = "openai"
	ProviderAnthropic   Provider = "anthropic"
	ProviderGoogle      Provider = "google"
	ProviderCohere      Provider = "cohere"
	
	// Local providers
	ProviderOllama      Provider = "ollama"
	ProviderVLLM        Provider = "vllm"
	ProviderLocalAI     Provider = "localai"
)

// ModelConfig represents model configuration.
type ModelConfig struct {
	Provider     Provider          `json:"provider"`
	Model        string            `json:"model"`
	APIKey       string            `json:"api_key"`
	BaseURL      string            `json:"base_url"`
	MaxTokens    int               `json:"max_tokens"`
	Temperature  float64           `json:"temperature"`
	Timeout      time.Duration     `json:"timeout"`
	RateLimit    int               `json:"rate_limit"`     // requests per minute
	CostPerToken float64           `json:"cost_per_token"` // cost per 1K tokens
	Priority     int               `json:"priority"`       // lower is higher priority
	TaskTypes    []TaskType        `json:"task_types"`     // supported task types
	Enabled      bool              `json:"enabled"`
	Extra        map[string]string `json:"extra"`
}

// RouterConfig represents router configuration.
type RouterConfig struct {
	DefaultProvider Provider               `json:"default_provider"`
	Models          map[string]ModelConfig `json:"models"`
	FallbackOrder   []string               `json:"fallback_order"`
	TaskRouting     map[TaskType][]string  `json:"task_routing"`
	CostOptimize    bool                   `json:"cost_optimize"`
	LoadBalance     bool                   `json:"load_balance"`
}

// Router handles model routing and selection.
type Router struct {
	config   *RouterConfig
	models   map[string]*ModelState
	mu       sync.RWMutex
	statsFunc func(provider string, success bool, tokens int64, duration time.Duration)
}

// ModelState tracks the state of a model.
type ModelState struct {
	Config        ModelConfig
	Available     bool
	LastUsed      time.Time
	RequestCount  int64
	ErrorCount    int64
	TotalTokens   int64
	TotalLatency  time.Duration
	CurrentLoad   int
}

// NewRouter creates a new router.
func NewRouter(config *RouterConfig) *Router {
	r := &Router{
		config: config,
		models: make(map[string]*ModelState),
	}
	
	for name, cfg := range config.Models {
		r.models[name] = &ModelState{
			Config:    cfg,
			Available: cfg.Enabled,
		}
	}
	
	return r
}

// SetStatsFunc sets the statistics function.
func (r *Router) SetStatsFunc(f func(provider string, success bool, tokens int64, duration time.Duration)) {
	r.statsFunc = f
}

// SelectModel selects the best model for a task.
func (r *Router) SelectModel(ctx context.Context, taskType TaskType) (*ModelConfig, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	// Get candidate models for this task type
	candidates := r.getCandidates(taskType)
	if len(candidates) == 0 {
		return nil, errors.New("no available model for task type: " + string(taskType))
	}
	
	// Select based on strategy
	if r.config.CostOptimize {
		return r.selectByCost(candidates)
	}
	
	if r.config.LoadBalance {
		return r.selectByLoad(candidates)
	}
	
	return r.selectByPriority(candidates)
}

func (r *Router) getCandidates(taskType TaskType) []*ModelState {
	var candidates []*ModelState
	
	// Check task routing first
	if modelNames, ok := r.config.TaskRouting[taskType]; ok {
		for _, name := range modelNames {
			if state, exists := r.models[name]; exists && state.Available {
				candidates = append(candidates, state)
			}
		}
	}
	
	// Fall back to all available models if no specific routing
	if len(candidates) == 0 {
		for _, state := range r.models {
			if state.Available && supportsTask(state.Config.TaskTypes, taskType) {
				candidates = append(candidates, state)
			}
		}
	}
	
	return candidates
}

func supportsTask(taskTypes []TaskType, target TaskType) bool {
	if len(taskTypes) == 0 {
		return true // Supports all if not specified
	}
	for _, t := range taskTypes {
		if t == target {
			return true
		}
	}
	return false
}

func (r *Router) selectByPriority(candidates []*ModelState) (*ModelConfig, error) {
	if len(candidates) == 0 {
		return nil, errors.New("no candidates")
	}
	
	best := candidates[0]
	for _, c := range candidates[1:] {
		if c.Config.Priority < best.Config.Priority {
			best = c
		}
	}
	return &best.Config, nil
}

func (r *Router) selectByCost(candidates []*ModelState) (*ModelConfig, error) {
	if len(candidates) == 0 {
		return nil, errors.New("no candidates")
	}
	
	best := candidates[0]
	for _, c := range candidates[1:] {
		if c.Config.CostPerToken < best.Config.CostPerToken {
			best = c
		}
	}
	return &best.Config, nil
}

func (r *Router) selectByLoad(candidates []*ModelState) (*ModelConfig, error) {
	if len(candidates) == 0 {
		return nil, errors.New("no candidates")
	}
	
	best := candidates[0]
	for _, c := range candidates[1:] {
		if c.CurrentLoad < best.CurrentLoad {
			best = c
		}
	}
	return &best.Config, nil
}

// GetFallback returns fallback models in order.
func (r *Router) GetFallback(currentModel string) (*ModelConfig, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	found := false
	for _, name := range r.config.FallbackOrder {
		if name == currentModel {
			found = true
			continue
		}
		if found {
			if state, exists := r.models[name]; exists && state.Available {
				return &state.Config, nil
			}
		}
	}
	
	return nil, errors.New("no fallback available")
}

// RecordUsage records model usage.
func (r *Router) RecordUsage(modelName string, success bool, tokens int64, latency time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	if state, exists := r.models[modelName]; exists {
		state.RequestCount++
		state.TotalTokens += tokens
		state.TotalLatency += latency
		state.LastUsed = time.Now()
		
		if !success {
			state.ErrorCount++
			// Disable if too many errors
			errorRate := float64(state.ErrorCount) / float64(state.RequestCount)
			if errorRate > 0.5 && state.RequestCount > 10 {
				state.Available = false
			}
		}
		
		if r.statsFunc != nil {
			r.statsFunc(modelName, success, tokens, latency)
		}
	}
}

// SetModelAvailability sets the availability of a model.
func (r *Router) SetModelAvailability(modelName string, available bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	if state, exists := r.models[modelName]; exists {
		state.Available = available
	}
}

// IncrementLoad increments the current load of a model.
func (r *Router) IncrementLoad(modelName string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	if state, exists := r.models[modelName]; exists {
		state.CurrentLoad++
	}
}

// DecrementLoad decrements the current load of a model.
func (r *Router) DecrementLoad(modelName string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	if state, exists := r.models[modelName]; exists {
		if state.CurrentLoad > 0 {
			state.CurrentLoad--
		}
	}
}

// GetModelStats returns statistics for all models.
func (r *Router) GetModelStats() map[string]*ModelStats {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	stats := make(map[string]*ModelStats)
	for name, state := range r.models {
		var avgLatency time.Duration
		if state.RequestCount > 0 {
			avgLatency = state.TotalLatency / time.Duration(state.RequestCount)
		}
		
		stats[name] = &ModelStats{
			Provider:     string(state.Config.Provider),
			Model:        state.Config.Model,
			Available:    state.Available,
			RequestCount: state.RequestCount,
			ErrorCount:   state.ErrorCount,
			TotalTokens:  state.TotalTokens,
			AvgLatency:   avgLatency,
			CurrentLoad:  state.CurrentLoad,
			LastUsed:     state.LastUsed,
		}
	}
	
	return stats
}

// ModelStats represents model statistics.
type ModelStats struct {
	Provider     string        `json:"provider"`
	Model        string        `json:"model"`
	Available    bool          `json:"available"`
	RequestCount int64         `json:"request_count"`
	ErrorCount   int64         `json:"error_count"`
	TotalTokens  int64         `json:"total_tokens"`
	AvgLatency   time.Duration `json:"avg_latency"`
	CurrentLoad  int           `json:"current_load"`
	LastUsed     time.Time     `json:"last_used"`
}

// DefaultRouterConfig returns a default router configuration.
func DefaultRouterConfig() *RouterConfig {
	return &RouterConfig{
		DefaultProvider: ProviderDeepSeek,
		Models: map[string]ModelConfig{
			"deepseek-chat": {
				Provider:     ProviderDeepSeek,
				Model:        "deepseek-chat",
				MaxTokens:    4096,
				Temperature:  0.7,
				Timeout:      60 * time.Second,
				RateLimit:    60,
				CostPerToken: 0.001,
				Priority:     1,
				TaskTypes:    []TaskType{TaskTypeSimple, TaskTypeReasoning, TaskTypeAnalysis},
				Enabled:      true,
			},
			"deepseek-reasoner": {
				Provider:     ProviderDeepSeek,
				Model:        "deepseek-reasoner",
				MaxTokens:    8192,
				Temperature:  0.3,
				Timeout:      120 * time.Second,
				RateLimit:    30,
				CostPerToken: 0.002,
				Priority:     2,
				TaskTypes:    []TaskType{TaskTypeReasoning, TaskTypeAnalysis, TaskTypeVerify},
				Enabled:      true,
			},
			"qwen-turbo": {
				Provider:     ProviderQwen,
				Model:        "qwen-turbo",
				MaxTokens:    4096,
				Temperature:  0.7,
				Timeout:      60 * time.Second,
				RateLimit:    60,
				CostPerToken: 0.001,
				Priority:     3,
				TaskTypes:    []TaskType{TaskTypeSimple, TaskTypeGeneration},
				Enabled:      true,
			},
		},
		FallbackOrder: []string{"deepseek-chat", "qwen-turbo", "deepseek-reasoner"},
		TaskRouting: map[TaskType][]string{
			TaskTypeSimple:    {"deepseek-chat", "qwen-turbo"},
			TaskTypeReasoning: {"deepseek-reasoner", "deepseek-chat"},
			TaskTypeAnalysis:  {"deepseek-chat", "deepseek-reasoner"},
			TaskTypeVerify:    {"deepseek-reasoner"},
		},
		CostOptimize: true,
		LoadBalance:  false,
	}
}
