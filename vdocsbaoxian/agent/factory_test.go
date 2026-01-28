package agent

import (
	"context"
	"testing"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/vdocsbaoxian/config"
)

func createTestConfig() *config.Config {
	cfg := config.DefaultConfig()
	// Clear providers to avoid initialization errors in tests
	cfg.LLM.Providers = make(map[string]*config.ProviderConfig)
	return cfg
}

func TestNewAgentFactory_NoProviders(t *testing.T) {
	cfg := createTestConfig()

	_, err := NewAgentFactory(cfg)
	// Should fail because no providers are available
	if err == nil {
		t.Error("Expected error when no providers available")
	}
}

func TestAgentFactory_GetModel_NotFound(t *testing.T) {
	cfg := config.DefaultConfig()
	// Add a disabled provider
	cfg.LLM.Providers = map[string]*config.ProviderConfig{
		"test": {
			Name:    "Test",
			Type:    "openai",
			Enabled: false,
		},
	}

	factory, err := NewAgentFactory(cfg)
	if err == nil {
		_, err = factory.GetModel("nonexistent")
		if err == nil {
			t.Error("Expected error when model not found")
		}
	}
}

func TestSubAgentInterface(t *testing.T) {
	// Test SubAgent interface implementation
	var agent SubAgent = &mockSubAgent{
		name: "test-agent",
		desc: "A test agent",
	}

	ctx := context.Background()
	
	if agent.Name(ctx) != "test-agent" {
		t.Errorf("Expected name 'test-agent', got '%s'", agent.Name(ctx))
	}
	if agent.Description(ctx) != "A test agent" {
		t.Errorf("Expected description 'A test agent', got '%s'", agent.Description(ctx))
	}
	if !agent.CanHandle(ctx, IntentLegalConsult) {
		t.Error("Expected CanHandle to return true")
	}
}

// Mock SubAgent for testing
type mockSubAgent struct {
	name string
	desc string
}

func (a *mockSubAgent) Name(ctx context.Context) string {
	return a.name
}

func (a *mockSubAgent) Description(ctx context.Context) string {
	return a.desc
}

func (a *mockSubAgent) Run(ctx context.Context, input *adk.AgentInput, options ...adk.AgentRunOption) *adk.AsyncIterator[*adk.AgentEvent] {
	// Return empty iterator for test
	iter, gen := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
	gen.Close()
	return iter
}

func (a *mockSubAgent) CanHandle(ctx context.Context, intent IntentType) bool {
	return true
}

func (a *mockSubAgent) GetCapabilities() []string {
	return []string{"test"}
}

func (a *mockSubAgent) GetDataSources() []string {
	return []string{"test_db"}
}

func TestAgentConfig(t *testing.T) {
	agentConfig := config.AgentConfig{
		MaxIterations:       20,
		MaxConcurrentAgents: 10,
		Timeout:             5 * time.Minute,
		EnableStreaming:     true,
		VerifyMode:          true,
	}

	if agentConfig.MaxIterations != 20 {
		t.Errorf("Expected MaxIterations 20, got %d", agentConfig.MaxIterations)
	}
	if agentConfig.MaxConcurrentAgents != 10 {
		t.Errorf("Expected MaxConcurrentAgents 10, got %d", agentConfig.MaxConcurrentAgents)
	}
	if agentConfig.Timeout != 5*time.Minute {
		t.Errorf("Expected Timeout 5 minutes, got %v", agentConfig.Timeout)
	}
}

func TestProviderConfigTypes(t *testing.T) {
	providerTypes := []string{"openai", "anthropic", "zhipu", "deepseek", "ollama"}

	for _, pType := range providerTypes {
		cfg := &config.ProviderConfig{
			Name:     pType,
			Type:     pType,
			Enabled:  true,
			Priority: 1,
		}

		if cfg.Type != pType {
			t.Errorf("Expected type '%s', got '%s'", pType, cfg.Type)
		}
	}
}

func TestAgentFactoryClose(t *testing.T) {
	cfg := createTestConfig()
	// Add a provider that will fail
	cfg.LLM.Providers = map[string]*config.ProviderConfig{
		"test": {
			Name:    "Test",
			Type:    "unknown", // This will fail
			Enabled: true,
		},
	}

	factory, err := NewAgentFactory(cfg)
	if err != nil {
		// Expected - no valid providers
		return
	}

	err = factory.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}
}

func TestIntentTypeString(t *testing.T) {
	tests := []struct {
		intent   IntentType
		expected string
	}{
		{IntentLegalConsult, "legal_consult"},
		{IntentProductAnalysis, "product_analysis"},
		{IntentClaimGuidance, "claim_guidance"},
		{IntentUserMatch, "user_match"},
		{IntentHealthData, "health_data"},
		{IntentComparison, "comparison"},
		{IntentGeneralQuery, "general_query"},
	}

	for _, tt := range tests {
		if got := tt.intent.String(); got != tt.expected {
			t.Errorf("IntentType(%d).String() = %s, want %s", tt.intent, got, tt.expected)
		}
	}
}

func TestIntentResult(t *testing.T) {
	result := IntentResult{
		Intent:     IntentLegalConsult,
		Confidence: 0.95,
		Entities: map[string][]string{
			"law_name": {"保险法"},
		},
	}

	if result.Intent != IntentLegalConsult {
		t.Errorf("Expected IntentLegalConsult, got %v", result.Intent)
	}
	if result.Confidence != 0.95 {
		t.Errorf("Expected confidence 0.95, got %f", result.Confidence)
	}
}

func BenchmarkSubAgentName(b *testing.B) {
	agent := &mockSubAgent{
		name: "bench-agent",
		desc: "Benchmark agent",
	}
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = agent.Name(ctx)
	}
}
