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

// Package indexer provides code indexing capabilities for Linux kernel source.
package indexer

import (
	"time"
)

// Document represents an indexed source file.
type Document struct {
	ID       int            `json:"id"`
	Path     string         `json:"path"`
	Size     int64          `json:"size"`
	ModTime  time.Time      `json:"mod_time"`
	Terms    map[string]int `json:"terms"` // term -> frequency
	Language string         `json:"language"`
}

// Posting represents a single posting in the inverted index.
type Posting struct {
	DocID     int   `json:"doc_id"`
	Frequency int   `json:"frequency"`
	Positions []int `json:"positions"`
}

// PostingList represents all postings for a term.
type PostingList struct {
	Term     string     `json:"term"`
	DocFreq  int        `json:"doc_freq"`
	Postings []*Posting `json:"postings"`
}

// FunctionInfo represents detailed information about a function.
type FunctionInfo struct {
	Name       string          `json:"name"`
	File       string          `json:"file"`
	StartLine  int             `json:"start_line"`
	EndLine    int             `json:"end_line"`
	Signature  string          `json:"signature"`
	Parameters []ParameterInfo `json:"parameters"`
	ReturnType string          `json:"return_type"`
	Comments   string          `json:"comments"`
	IsStatic   bool            `json:"is_static"`
	IsInline   bool            `json:"is_inline"`
	Complexity int             `json:"complexity"` // cyclomatic complexity
	LOC        int             `json:"loc"`        // lines of code
}

// FunctionSummary represents function summary for persistence.
type FunctionSummary struct {
	Name       string   `json:"name"`
	File       string   `json:"file"`
	StartLine  int      `json:"start_line"`
	EndLine    int      `json:"end_line"`
	Signature  string   `json:"signature"`
	ReturnType string   `json:"return_type"`
	Parameters []string `json:"parameters"`
	Summary    string   `json:"summary"`
	CallCount  int      `json:"call_count"`
}

// ParameterInfo represents function parameter information.
type ParameterInfo struct {
	Name string `json:"name"`
	Type string `json:"type"`
	Desc string `json:"desc"`
}

// Symbol represents a code symbol (function, struct, macro, etc.).
type Symbol struct {
	Name    string            `json:"name"`
	File    string            `json:"file"`
	Line    int               `json:"line"`
	Kind    SymbolKind        `json:"kind"`
	Pattern string            `json:"pattern"`
	Extras  map[string]string `json:"extras"`
}

// SymbolKind represents the type of symbol.
type SymbolKind string

const (
	SymbolFunction  SymbolKind = "function"
	SymbolStruct    SymbolKind = "struct"
	SymbolMacro     SymbolKind = "macro"
	SymbolTypedef   SymbolKind = "typedef"
	SymbolEnum      SymbolKind = "enum"
	SymbolVariable  SymbolKind = "variable"
	SymbolPrototype SymbolKind = "prototype"
)

// CallGraphNode represents a node in the call graph.
type CallGraphNode struct {
	Function  string `json:"function"`
	File      string `json:"file"`
	Line      int    `json:"line"`
	InDegree  int    `json:"in_degree"`
	OutDegree int    `json:"out_degree"`
}

// CallEdge represents a call relationship.
type CallEdge struct {
	Caller     string `json:"caller"`
	Callee     string `json:"callee"`
	CallerFile string `json:"caller_file"`
	CallerLine int    `json:"caller_line"`
}

// SearchResult represents a search result.
type SearchResult struct {
	File       string  `json:"file"`
	Line       int     `json:"line"`
	Content    string  `json:"content"`
	Context    string  `json:"context"`
	Score      float64 `json:"score"`
	Highlights []int   `json:"highlights"`
}

// Note: IndexStats is defined in manager.go

// Token represents a tokenized term with position.
type Token struct {
	Term     string `json:"term"`
	Position int    `json:"position"`
	Line     int    `json:"line"`
	Column   int    `json:"column"`
}
