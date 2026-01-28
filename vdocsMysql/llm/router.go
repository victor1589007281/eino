package llm

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
)

// DefaultModelRouter implements the ModelRouter interface.
type DefaultModelRouter struct {
	mu         sync.RWMutex
	providers  map[string]Provider
	models     map[string]*Model
	rules      []RoutingRule
	defaultProvider string
	defaultModel    string
}

// RoutingRule defines a routing rule.
type RoutingRule struct {
	Name        string
	Pattern     *regexp.Regexp
	Keywords    []string
	Provider    string
	Model       string
	Priority    int
	Condition   func(request *ChatRequest, intent string) bool
}

// RouterConfig contains router configuration.
type RouterConfig struct {
	DefaultProvider string
	DefaultModel    string
	Rules           []RoutingRuleConfig
}

// RoutingRuleConfig defines routing rule configuration.
type RoutingRuleConfig struct {
	Name        string
	Pattern     string
	Keywords    []string
	Provider    string
	Model       string
	Priority    int
}

// NewDefaultModelRouter creates a new default model router.
func NewDefaultModelRouter(config *RouterConfig) (*DefaultModelRouter, error) {
	router := &DefaultModelRouter{
		providers:       make(map[string]Provider),
		models:          make(map[string]*Model),
		rules:           make([]RoutingRule, 0),
		defaultProvider: config.DefaultProvider,
		defaultModel:    config.DefaultModel,
	}

	// Convert config rules to routing rules
	for _, ruleConfig := range config.Rules {
		rule := RoutingRule{
			Name:     ruleConfig.Name,
			Keywords: ruleConfig.Keywords,
			Provider: ruleConfig.Provider,
			Model:    ruleConfig.Model,
			Priority: ruleConfig.Priority,
		}

		if ruleConfig.Pattern != "" {
			pattern, err := regexp.Compile(ruleConfig.Pattern)
			if err != nil {
				return nil, fmt.Errorf("invalid pattern %s: %w", ruleConfig.Pattern, err)
			}
			rule.Pattern = pattern
		}

		router.rules = append(router.rules, rule)
	}

	// Sort rules by priority (higher priority first)
	sort.Slice(router.rules, func(i, j int) bool {
		return router.rules[i].Priority > router.rules[j].Priority
	})

	return router, nil
}

// RegisterProvider registers a provider.
func (r *DefaultModelRouter) RegisterProvider(provider Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[provider.Name()] = provider
}

// RegisterModel registers a model.
func (r *DefaultModelRouter) RegisterModel(model *Model) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.models[model.Name] = model
}

// Route selects the best provider and model for a request.
func (r *DefaultModelRouter) Route(ctx context.Context, request *ChatRequest, intent string) (Provider, string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Try to match rules
	for _, rule := range r.rules {
		if r.matchRule(&rule, request, intent) {
			provider, ok := r.providers[rule.Provider]
			if ok {
				return provider, rule.Model, nil
			}
		}
	}

	// Use default
	provider, ok := r.providers[r.defaultProvider]
	if !ok {
		// Return first available provider
		for _, p := range r.providers {
			return p, r.defaultModel, nil
		}
		return nil, "", fmt.Errorf("no providers available")
	}

	return provider, r.defaultModel, nil
}

func (r *DefaultModelRouter) matchRule(rule *RoutingRule, request *ChatRequest, intent string) bool {
	// Check custom condition
	if rule.Condition != nil {
		return rule.Condition(request, intent)
	}

	// Check pattern match
	if rule.Pattern != nil {
		if rule.Pattern.MatchString(intent) {
			return true
		}
		// Also check message content
		for _, msg := range request.Messages {
			if rule.Pattern.MatchString(msg.Content) {
				return true
			}
		}
	}

	// Check keyword match
	if len(rule.Keywords) > 0 {
		intentLower := strings.ToLower(intent)
		for _, keyword := range rule.Keywords {
			if strings.Contains(intentLower, strings.ToLower(keyword)) {
				return true
			}
		}
		// Check message content
		for _, msg := range request.Messages {
			contentLower := strings.ToLower(msg.Content)
			for _, keyword := range rule.Keywords {
				if strings.Contains(contentLower, strings.ToLower(keyword)) {
					return true
				}
			}
		}
	}

	return false
}

// GetProvider returns a provider by name.
func (r *DefaultModelRouter) GetProvider(name string) (Provider, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	provider, ok := r.providers[name]
	return provider, ok
}

// GetModel returns a model by name.
func (r *DefaultModelRouter) GetModel(name string) (*Model, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	model, ok := r.models[name]
	return model, ok
}

// ListProviders returns all registered provider names.
func (r *DefaultModelRouter) ListProviders() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	return names
}

// ListModels returns all registered model names.
func (r *DefaultModelRouter) ListModels() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.models))
	for name := range r.models {
		names = append(names, name)
	}
	return names
}

// FallbackRouter wraps a router with fallback support.
type FallbackRouter struct {
	primary      *DefaultModelRouter
	fallbackChain []string
	maxAttempts  int
}

// FallbackRouterConfig contains fallback router configuration.
type FallbackRouterConfig struct {
	PrimaryRouter *DefaultModelRouter
	FallbackChain []string
	MaxAttempts   int
}

// NewFallbackRouter creates a new fallback router.
func NewFallbackRouter(config *FallbackRouterConfig) *FallbackRouter {
	maxAttempts := config.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 3
	}

	return &FallbackRouter{
		primary:       config.PrimaryRouter,
		fallbackChain: config.FallbackChain,
		maxAttempts:   maxAttempts,
	}
}

// RegisterProvider delegates to primary router.
func (r *FallbackRouter) RegisterProvider(provider Provider) {
	r.primary.RegisterProvider(provider)
}

// RegisterModel delegates to primary router.
func (r *FallbackRouter) RegisterModel(model *Model) {
	r.primary.RegisterModel(model)
}

// Route attempts to route with fallback.
func (r *FallbackRouter) Route(ctx context.Context, request *ChatRequest, intent string) (Provider, string, error) {
	// Try primary routing
	provider, model, err := r.primary.Route(ctx, request, intent)
	if err == nil && provider != nil {
		return provider, model, nil
	}

	// Try fallback chain
	for _, providerName := range r.fallbackChain {
		if p, ok := r.primary.GetProvider(providerName); ok {
			// Determine model for this provider
			modelName := r.getDefaultModelForProvider(providerName)
			return p, modelName, nil
		}
	}

	return nil, "", fmt.Errorf("no providers available after fallback")
}

func (r *FallbackRouter) getDefaultModelForProvider(provider string) string {
	// Map providers to their default models
	defaults := map[string]string{
		"openai":       "gpt-4-turbo",
		"azure_openai": "gpt-4",
		"wenxin":       "ernie-bot-4",
		"qwen":         "qwen-max",
		"deepseek":     "deepseek-chat",
	}

	if model, ok := defaults[provider]; ok {
		return model
	}
	return ""
}

// SmartRouter implements intelligent routing based on request characteristics.
type SmartRouter struct {
	router       *DefaultModelRouter
	intentModels map[string]string // intent -> preferred model
	costOptimize bool
}

// SmartRouterConfig contains smart router configuration.
type SmartRouterConfig struct {
	Router       *DefaultModelRouter
	IntentModels map[string]string
	CostOptimize bool
}

// NewSmartRouter creates a new smart router.
func NewSmartRouter(config *SmartRouterConfig) *SmartRouter {
	return &SmartRouter{
		router:       config.Router,
		intentModels: config.IntentModels,
		costOptimize: config.CostOptimize,
	}
}

// RegisterProvider delegates to underlying router.
func (r *SmartRouter) RegisterProvider(provider Provider) {
	r.router.RegisterProvider(provider)
}

// RegisterModel delegates to underlying router.
func (r *SmartRouter) RegisterModel(model *Model) {
	r.router.RegisterModel(model)
}

// Route implements intelligent routing.
func (r *SmartRouter) Route(ctx context.Context, request *ChatRequest, intent string) (Provider, string, error) {
	// Check if we have a preferred model for this intent
	if preferredModel, ok := r.intentModels[intent]; ok {
		if model, ok := r.router.GetModel(preferredModel); ok {
			if provider, ok := r.router.GetProvider(model.Provider); ok {
				return provider, preferredModel, nil
			}
		}
	}

	// Estimate context length
	contextLength := r.estimateContextLength(request)

	// If cost optimization is enabled, choose cheaper models for simple queries
	if r.costOptimize {
		if contextLength < 2000 && !r.isComplexQuery(request) {
			// Use a cheaper model
			return r.getCheapestProvider(contextLength)
		}
	}

	// Fall back to default routing
	return r.router.Route(ctx, request, intent)
}

func (r *SmartRouter) estimateContextLength(request *ChatRequest) int {
	length := 0
	for _, msg := range request.Messages {
		length += len(msg.Content)
	}
	// Rough estimate: 4 chars per token
	return length / 4
}

func (r *SmartRouter) isComplexQuery(request *ChatRequest) bool {
	// Check for indicators of complex queries
	complexKeywords := []string{
		"analyze", "compare", "explain in detail", "step by step",
		"comprehensive", "thorough", "all aspects", "深入分析",
		"详细解释", "全面", "完整",
	}

	for _, msg := range request.Messages {
		contentLower := strings.ToLower(msg.Content)
		for _, keyword := range complexKeywords {
			if strings.Contains(contentLower, strings.ToLower(keyword)) {
				return true
			}
		}
	}

	return false
}

func (r *SmartRouter) getCheapestProvider(contextLength int) (Provider, string, error) {
	var cheapestProvider Provider
	var cheapestModel string
	var cheapestCost float64 = -1

	for _, modelName := range r.router.ListModels() {
		model, ok := r.router.GetModel(modelName)
		if !ok {
			continue
		}

		// Skip if context exceeds model limit
		if contextLength > model.MaxContextLength {
			continue
		}

		cost := model.InputCostPer1K
		if cheapestCost < 0 || cost < cheapestCost {
			if provider, ok := r.router.GetProvider(model.Provider); ok {
				cheapestProvider = provider
				cheapestModel = modelName
				cheapestCost = cost
			}
		}
	}

	if cheapestProvider == nil {
		return nil, "", fmt.Errorf("no suitable provider found")
	}

	return cheapestProvider, cheapestModel, nil
}
