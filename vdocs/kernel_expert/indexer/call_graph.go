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

package indexer

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"
)

// CallGraph represents function call relationships.
type CallGraph struct {
	Nodes   map[string]*CallGraphNode `json:"nodes"`   // function -> node
	Callers map[string][]string       `json:"callers"` // callee -> list of callers
	Callees map[string][]string       `json:"callees"` // caller -> list of callees
	Edges   []*CallEdge               `json:"edges"`
	mu      sync.RWMutex              // unexported, not serialized
}

// NewCallGraph creates a new call graph.
func NewCallGraph() *CallGraph {
	return &CallGraph{
		Nodes:   make(map[string]*CallGraphNode),
		Callers: make(map[string][]string),
		Callees: make(map[string][]string),
		Edges:   make([]*CallEdge, 0),
	}
}

// AddNode adds a node to the call graph.
func (cg *CallGraph) AddNode(node *CallGraphNode) {
	cg.mu.Lock()
	defer cg.mu.Unlock()
	cg.Nodes[node.Function] = node
}

// AddEdge adds a call edge to the graph.
func (cg *CallGraph) AddEdge(edge *CallEdge) {
	cg.mu.Lock()
	defer cg.mu.Unlock()

	cg.Edges = append(cg.Edges, edge)
	cg.Callers[edge.Callee] = append(cg.Callers[edge.Callee], edge.Caller)
	cg.Callees[edge.Caller] = append(cg.Callees[edge.Caller], edge.Callee)

	// Update node degrees
	if node, exists := cg.Nodes[edge.Caller]; exists {
		node.OutDegree++
	}
	if node, exists := cg.Nodes[edge.Callee]; exists {
		node.InDegree++
	}
}

// GetCallers returns all functions that call the given function.
func (cg *CallGraph) GetCallers(function string) []string {
	cg.mu.RLock()
	defer cg.mu.RUnlock()
	return cg.Callers[function]
}

// GetCallees returns all functions called by the given function.
func (cg *CallGraph) GetCallees(function string) []string {
	cg.mu.RLock()
	defer cg.mu.RUnlock()
	return cg.Callees[function]
}

// GetNode returns the node for a function.
func (cg *CallGraph) GetNode(function string) *CallGraphNode {
	cg.mu.RLock()
	defer cg.mu.RUnlock()
	return cg.Nodes[function]
}

// CallChainNode represents a node in a call chain.
type CallChainNode struct {
	Function string           `json:"function"`
	File     string           `json:"file"`
	Line     int              `json:"line"`
	Depth    int              `json:"depth"`
	Children []*CallChainNode `json:"children,omitempty"`
}

// GetCallChain returns the call chain starting from a function.
func (cg *CallGraph) GetCallChain(function string, direction string, maxDepth int) *CallChainNode {
	cg.mu.RLock()
	defer cg.mu.RUnlock()

	visited := make(map[string]bool)
	return cg.buildCallChain(function, direction, 0, maxDepth, visited)
}

func (cg *CallGraph) buildCallChain(function string, direction string, depth, maxDepth int, visited map[string]bool) *CallChainNode {
	if depth >= maxDepth || visited[function] {
		return nil
	}

	visited[function] = true
	node := cg.Nodes[function]

	chainNode := &CallChainNode{
		Function: function,
		Depth:    depth,
		Children: make([]*CallChainNode, 0),
	}

	if node != nil {
		chainNode.File = node.File
		chainNode.Line = node.Line
	}

	var related []string
	if direction == "callees" || direction == "both" {
		related = cg.Callees[function]
	} else {
		related = cg.Callers[function]
	}

	for _, rel := range related {
		child := cg.buildCallChain(rel, direction, depth+1, maxDepth, visited)
		if child != nil {
			chainNode.Children = append(chainNode.Children, child)
		}
	}

	delete(visited, function)
	return chainNode
}

// GetStats returns call graph statistics.
func (cg *CallGraph) GetStats() (nodeCount, edgeCount int) {
	cg.mu.RLock()
	defer cg.mu.RUnlock()
	return len(cg.Nodes), len(cg.Edges)
}

// CscopeClient provides interface to cscope for call analysis.
type CscopeClient struct {
	cscopePath string
	sourcePath string
	dbPath     string
}

// NewCscopeClient creates a new cscope client.
func NewCscopeClient(cscopePath, sourcePath, dbPath string) *CscopeClient {
	return &CscopeClient{
		cscopePath: cscopePath,
		sourcePath: sourcePath,
		dbPath:     dbPath,
	}
}

// BuildDatabase builds the cscope database.
func (c *CscopeClient) BuildDatabase(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, c.cscopePath, "-b", "-R", "-k", "-f", c.dbPath)
	cmd.Dir = c.sourcePath
	return cmd.Run()
}

// FindCallers finds all callers of a function using cscope.
func (c *CscopeClient) FindCallers(ctx context.Context, function string) ([]*CallEdge, error) {
	// cscope -d -L3 function_name
	// -d: don't rebuild database
	// -L3: find functions calling this function
	cmd := exec.CommandContext(ctx, c.cscopePath, "-d", "-f", c.dbPath, "-L3", function)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("cscope error: %w", err)
	}

	return parseCscopeOutput(string(output), function, true), nil
}

// FindCallees finds all functions called by a function using cscope.
func (c *CscopeClient) FindCallees(ctx context.Context, function string) ([]*CallEdge, error) {
	// cscope -d -L2 function_name
	// -L2: find functions called by this function
	cmd := exec.CommandContext(ctx, c.cscopePath, "-d", "-f", c.dbPath, "-L2", function)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("cscope error: %w", err)
	}

	return parseCscopeOutput(string(output), function, false), nil
}

// FindDefinition finds the definition of a function.
func (c *CscopeClient) FindDefinition(ctx context.Context, function string) (*CallGraphNode, error) {
	// cscope -d -L1 function_name
	// -L1: find definition
	cmd := exec.CommandContext(ctx, c.cscopePath, "-d", "-f", c.dbPath, "-L1", function)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("cscope error: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) == 0 || lines[0] == "" {
		return nil, nil
	}

	parts := strings.Fields(lines[0])
	if len(parts) < 3 {
		return nil, nil
	}

	line, _ := strconv.Atoi(parts[2])
	return &CallGraphNode{
		Function: function,
		File:     parts[0],
		Line:     line,
	}, nil
}

// parseCscopeOutput parses cscope output.
// Format: file function line text
func parseCscopeOutput(output string, function string, isCaller bool) []*CallEdge {
	edges := make([]*CallEdge, 0)
	scanner := bufio.NewScanner(strings.NewReader(output))

	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) < 3 {
			continue
		}

		file := parts[0]
		funcName := parts[1]
		lineNum, _ := strconv.Atoi(parts[2])

		edge := &CallEdge{}
		if isCaller {
			edge.Caller = funcName
			edge.Callee = function
			edge.CallerFile = file
			edge.CallerLine = lineNum
		} else {
			edge.Caller = function
			edge.Callee = funcName
		}

		edges = append(edges, edge)
	}

	return edges
}
