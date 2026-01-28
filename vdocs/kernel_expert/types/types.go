// Package types defines shared types used across packages.
package types

// IntentType represents the type of user intent.
type IntentType string

const (
	IntentConcept      IntentType = "concept"
	IntentFunction     IntentType = "function"
	IntentCallChain    IntentType = "callchain"
	IntentArchitecture IntentType = "architecture"
	IntentComparison   IntentType = "comparison"
	IntentDebug        IntentType = "debug"
	IntentPerformance  IntentType = "performance"
	IntentUnknown      IntentType = "unknown"
)

// OutputType represents the type of output.
type OutputType string

const (
	OutputSummary  OutputType = "summary"
	OutputDocument OutputType = "document"
)

// CallChainNode represents a node in the call chain.
type CallChainNode struct {
	Function string           `json:"function"`
	File     string           `json:"file"`
	Line     int              `json:"line"`
	Depth    int              `json:"depth"`
	Children []*CallChainNode `json:"children,omitempty"`
}
