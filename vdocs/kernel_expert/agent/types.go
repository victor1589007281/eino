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

// Package agent provides Linux kernel expert agent implementations.
package agent

import (
	"kernel_expert/types"
)

// Type aliases for convenience
type IntentType = types.IntentType
type OutputType = types.OutputType
type CallChainNode = types.CallChainNode

// Re-export intent constants
const (
	IntentConcept      = types.IntentConcept
	IntentFunction     = types.IntentFunction
	IntentCallChain    = types.IntentCallChain
	IntentArchitecture = types.IntentArchitecture
	IntentComparison   = types.IntentComparison
	IntentDebug        = types.IntentDebug
	IntentPerformance  = types.IntentPerformance
	IntentUnknown      = types.IntentUnknown
)

// Re-export output constants
const (
	OutputSummary  = types.OutputSummary
	OutputDocument = types.OutputDocument
)

// TaskStatus represents the status of a task.
type TaskStatus int

const (
	TaskPending TaskStatus = iota
	TaskRunning
	TaskCompleted
	TaskFailed
)

// Task represents a work unit for SubAgents.
type Task struct {
	ID           string
	Type         IntentType
	Priority     int
	Agent        string
	Input        map[string]interface{}
	Output       interface{}
	Status       TaskStatus
	Error        error
	Dependencies []string
}

// TaskPlan represents an execution plan.
type TaskPlan struct {
	Query    string
	Intent   IntentType
	Tasks    []*Task
	Parallel bool
}

// AnalysisResult represents the result of analysis.
type AnalysisResult struct {
	Type    string      `json:"type"`
	Success bool        `json:"success"`
	Data    interface{} `json:"data"`
	Error   string      `json:"error,omitempty"`
	Sources []string    `json:"sources,omitempty"`
}

// FunctionAnalysisResult represents function analysis result.
type FunctionAnalysisResult struct {
	Name        string          `json:"name"`
	File        string          `json:"file"`
	StartLine   int             `json:"start_line"`
	EndLine     int             `json:"end_line"`
	Signature   string          `json:"signature"`
	Description string          `json:"description"`
	Parameters  []ParameterDesc `json:"parameters"`
	ReturnType  string          `json:"return_type"`
	KeyLogic    []string        `json:"key_logic"`
	Complexity  string          `json:"complexity"`
	SourceCode  string          `json:"source_code"`
}

// ParameterDesc describes a function parameter.
type ParameterDesc struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
}

// CallChainResult represents call chain analysis result.
type CallChainResult struct {
	RootFunction string         `json:"root_function"`
	Direction    string         `json:"direction"`
	Depth        int            `json:"depth"`
	Chain        *CallChainNode `json:"chain"`
	KeyFunctions []string       `json:"key_functions"`
}

// ArchitectureResult represents architecture analysis result.
type ArchitectureResult struct {
	Subsystem    string           `json:"subsystem"`
	Overview     string           `json:"overview"`
	Components   []ComponentInfo  `json:"components"`
	Dependencies []DependencyInfo `json:"dependencies"`
	DataFlows    []DataFlowInfo   `json:"data_flows"`
}

// ComponentInfo describes a system component.
type ComponentInfo struct {
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Files        []string `json:"files"`
	KeyFunctions []string `json:"key_functions"`
}

// DependencyInfo describes a dependency relationship.
type DependencyInfo struct {
	From string `json:"from"`
	To   string `json:"to"`
	Type string `json:"type"`
}

// DataFlowInfo describes data flow.
type DataFlowInfo struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
	DataType    string `json:"data_type"`
	Description string `json:"description"`
}

// SearchResult represents a code search result.
type SearchResult struct {
	File      string  `json:"file"`
	Line      int     `json:"line"`
	Content   string  `json:"content"`
	Context   string  `json:"context"`
	Relevance float64 `json:"relevance"`
}
