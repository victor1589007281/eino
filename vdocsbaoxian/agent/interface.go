// Package agent provides the core agent implementations for Insurance Expert.
package agent

import (
	"context"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
)

// IntentType represents the type of user intent.
type IntentType int

const (
	// IntentLegalConsult represents legal consultation intent.
	IntentLegalConsult IntentType = iota
	// IntentProductAnalysis represents product analysis intent.
	IntentProductAnalysis
	// IntentClaimGuidance represents claim guidance intent.
	IntentClaimGuidance
	// IntentUserMatch represents user matching intent.
	IntentUserMatch
	// IntentHealthData represents health data analysis intent.
	IntentHealthData
	// IntentComparison represents product comparison intent.
	IntentComparison
	// IntentGeneralQuery represents general query intent.
	IntentGeneralQuery
)

// String returns the string representation of IntentType.
func (i IntentType) String() string {
	switch i {
	case IntentLegalConsult:
		return "legal_consult"
	case IntentProductAnalysis:
		return "product_analysis"
	case IntentClaimGuidance:
		return "claim_guidance"
	case IntentUserMatch:
		return "user_match"
	case IntentHealthData:
		return "health_data"
	case IntentComparison:
		return "comparison"
	case IntentGeneralQuery:
		return "general_query"
	default:
		return "unknown"
	}
}

// SubAgent defines the interface for sub-agents.
type SubAgent interface {
	adk.Agent

	// CanHandle returns true if the agent can handle the given intent.
	CanHandle(ctx context.Context, intent IntentType) bool

	// GetCapabilities returns the capabilities of the agent.
	GetCapabilities() []string

	// GetDataSources returns the data sources required by the agent.
	GetDataSources() []string
}

// IntentResult represents the result of intent recognition.
type IntentResult struct {
	Intent      IntentType             `json:"intent"`
	Confidence  float64                `json:"confidence"`
	Entities    map[string][]string    `json:"entities"`
	SubIntents  []IntentType           `json:"sub_intents,omitempty"`
	Context     map[string]interface{} `json:"context,omitempty"`
}

// ExecutionPlan represents the execution plan for a query.
type ExecutionPlan struct {
	Intent         IntentType
	Steps          []PlanStep
	ParallelGroups [][]int
	VerifySteps    []int
	Priority       int
}

// PlanStep represents a step in the execution plan.
type PlanStep struct {
	ID          string
	Name        string
	Agent       string
	Tool        string
	InputFrom   []string
	Params      map[string]interface{}
	NeedVerify  bool
	Timeout     int // seconds
}

// AnalysisResult represents the result of analysis.
type AnalysisResult struct {
	Question      string
	Intent        IntentType
	Summary       string
	Details       string
	LegalRefs     []LegalReference
	ProductRefs   []ProductReference
	CaseRefs      []CaseReference
	VerifyResults []VerifyResult
	Diagrams      []Diagram
	Metadata      map[string]interface{}
}

// LegalReference represents a legal reference.
type LegalReference struct {
	LawName      string `json:"law_name"`
	Article      string `json:"article"`
	Content      string `json:"content"`
	VerifyStatus string `json:"verify_status"` // verified, unverified, non-compliant
	Source       string `json:"source"`
	ValidDate    string `json:"valid_date"`
}

// ProductReference represents a product reference.
type ProductReference struct {
	ProductName string `json:"product_name"`
	Company     string `json:"company"`
	Clause      string `json:"clause"`
	ValidDate   string `json:"valid_date"`
	Source      string `json:"source"`
}

// CaseReference represents a case reference.
type CaseReference struct {
	CaseID    string  `json:"case_id"`
	Summary   string  `json:"summary"`
	Outcome   string  `json:"outcome"`
	Relevance float64 `json:"relevance"`
	Source    string  `json:"source"`
}

// VerifyResult represents a verification result.
type VerifyResult struct {
	Content       string           `json:"content"`
	Verified      bool             `json:"verified"`
	Confidence    float64          `json:"confidence"`
	Details       []VerifyDetail   `json:"details"`
	Warnings      []string         `json:"warnings,omitempty"`
	Sources       []VerifiedSource `json:"sources"`
}

// VerifyDetail represents verification detail.
type VerifyDetail struct {
	Claim      string `json:"claim"`
	Status     string `json:"status"` // verified, unverified, partial
	Evidence   string `json:"evidence"`
	Source     string `json:"source"`
	Timeliness string `json:"timeliness"` // current, outdated, unknown
}

// VerifiedSource represents a verified source.
type VerifiedSource struct {
	URL       string `json:"url"`
	Title     string `json:"title"`
	Authority string `json:"authority"` // official, authoritative, general
	ValidDate string `json:"valid_date"`
}

// Diagram represents a diagram in the output.
type Diagram struct {
	Type    DiagramType `json:"type"`
	Title   string      `json:"title"`
	Content string      `json:"content"` // Mermaid code
}

// DiagramType represents the type of diagram.
type DiagramType string

const (
	DiagramTypeComparison DiagramType = "comparison"
	DiagramTypeFlow       DiagramType = "flow"
	DiagramTypeTimeline   DiagramType = "timeline"
	DiagramTypeStructure  DiagramType = "structure"
	DiagramTypeSequence   DiagramType = "sequence"
)

// Entity represents an extracted entity.
type Entity struct {
	Type  string  `json:"type"`
	Value string  `json:"value"`
	Start int     `json:"start"`
	End   int     `json:"end"`
	Score float64 `json:"score"`
}

// UserProfile represents a user profile.
type UserProfile struct {
	Age          int      `json:"age,omitempty"`
	Gender       string   `json:"gender,omitempty"`
	Occupation   string   `json:"occupation,omitempty"`
	HealthStatus string   `json:"health_status,omitempty"`
	Budget       float64  `json:"budget,omitempty"`
	Diseases     []string `json:"diseases,omitempty"`
	Preferences  []string `json:"preferences,omitempty"`
}

// QueryContext represents the context of a query.
type QueryContext struct {
	InsuranceType string       `json:"insurance_type"` // life, health, property, auto
	Scenario      string       `json:"scenario"`       // purchase, claim, cancel, renew
	UserProfile   *UserProfile `json:"user_profile,omitempty"`
	Urgency       string       `json:"urgency"`
	OutputType    string       `json:"output_type"` // summary, document
}

// AgentMessage represents a message in agent communication.
type AgentMessage struct {
	Role    string               `json:"role"`
	Content string               `json:"content"`
	Tools   []*schema.ToolInfo   `json:"tools,omitempty"`
	Extra   map[string]interface{} `json:"extra,omitempty"`
}

// AgentResponse represents a response from an agent.
type AgentResponse struct {
	Content       string           `json:"content"`
	Analysis      *AnalysisResult  `json:"analysis,omitempty"`
	ToolCalls     []ToolCall       `json:"tool_calls,omitempty"`
	VerifyResults []VerifyResult   `json:"verify_results,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// ToolCall represents a tool call.
type ToolCall struct {
	ID       string                 `json:"id"`
	Name     string                 `json:"name"`
	Args     map[string]interface{} `json:"args"`
	Result   string                 `json:"result,omitempty"`
}
