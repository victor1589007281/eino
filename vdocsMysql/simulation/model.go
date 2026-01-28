// Package simulation provides load simulation and bottleneck analysis.
package simulation

import (
	"encoding/json"
	"time"
)

// SimulationModel represents a complete simulation model.
type SimulationModel struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Version     string                 `json:"version"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	Graph       *SimulationGraph       `json:"graph"`
	StatsConfig *StatsConfig           `json:"stats_config"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// SimulationGraph represents the function call graph for simulation.
type SimulationGraph struct {
	Nodes       map[string]*SimNode `json:"nodes"`
	Edges       map[string]*SimEdge `json:"edges"`
	EntryPoints []string            `json:"entry_points"`
	TotalNodes  int                 `json:"total_nodes"`
	TotalEdges  int                 `json:"total_edges"`
	MaxDepth    int                 `json:"max_depth"`
}

// SimNode represents a function node in the simulation graph.
type SimNode struct {
	ID            string         `json:"id"`
	Name          string         `json:"name"`
	QualifiedName string         `json:"qualified_name"`
	FilePath      string         `json:"file_path"`
	Line          int            `json:"line"`
	Module        string         `json:"module"`
	Subsystem     string         `json:"subsystem"`
	NodeType      NodeType       `json:"node_type"`
	CostModel     *NodeCostModel `json:"cost_model"`
	Stats         *NodeStats     `json:"stats"`
	Visual        *NodeVisual    `json:"visual"`
}

// NodeType categorizes function types.
type NodeType string

const (
	NodeTypeEntry    NodeType = "entry"
	NodeTypeCore     NodeType = "core"
	NodeTypeIO       NodeType = "io"
	NodeTypeLock     NodeType = "lock"
	NodeTypeMemory   NodeType = "memory"
	NodeTypeNetwork  NodeType = "network"
	NodeTypeInternal NodeType = "internal"
)

// NodeCostModel defines the cost model for a node.
type NodeCostModel struct {
	BaseCPUCost       float64            `json:"base_cpu_cost"`
	BaseMemoryCost    float64            `json:"base_memory_cost"`
	BaseIOCost        float64            `json:"base_io_cost"`
	BaseLockCost      float64            `json:"base_lock_cost"`
	ScaleFactors      map[string]float64 `json:"scale_factors"`
	ConcurrencyFactor float64            `json:"concurrency_factor"`
}

// NodeStats holds statistics for a node.
type NodeStats struct {
	CallCount     int64         `json:"call_count"`
	TotalExecTime time.Duration `json:"total_exec_time"`
	AvgExecTime   time.Duration `json:"avg_exec_time"`
	MaxExecTime   time.Duration `json:"max_exec_time"`
	P99ExecTime   time.Duration `json:"p99_exec_time"`
	MemoryAlloc   int64         `json:"memory_alloc"`
	IOBytes       int64         `json:"io_bytes"`
	IOOps         int64         `json:"io_ops"`
	LockWaitTime  time.Duration `json:"lock_wait_time"`
	LockCount     int64         `json:"lock_count"`
	HotScore      float64       `json:"hot_score"`
	IsHotPath     bool          `json:"is_hot_path"`
}

// NodeVisual defines visual properties for rendering.
type NodeVisual struct {
	Color   string  `json:"color"`
	Size    float64 `json:"size"`
	Label   string  `json:"label"`
	Tooltip string  `json:"tooltip"`
	Level   int     `json:"level"`
	Group   string  `json:"group"`
}

// SimEdge represents a call relationship edge.
type SimEdge struct {
	ID          string    `json:"id"`
	FromNode    string    `json:"from_node"`
	ToNode      string    `json:"to_node"`
	CallType    CallType  `json:"call_type"`
	Frequency   int64     `json:"frequency"`
	Probability float64   `json:"probability"`
	Condition   string    `json:"condition,omitempty"`
	EdgeCost    float64   `json:"edge_cost"`
	Index       int       `json:"index"`
}

// CallType categorizes call types.
type CallType string

const (
	CallTypeDirect   CallType = "direct"
	CallTypeVirtual  CallType = "virtual"
	CallTypeCallback CallType = "callback"
	CallTypeAsync    CallType = "async"
)

// StatsConfig defines how MySQL statistics map to the model.
type StatsConfig struct {
	StatusMapping     map[string]*StatusMapping     `json:"status_mapping"`
	PerfSchemaMapping map[string]*PerfSchemaMapping `json:"perf_schema_mapping"`
	CustomMetrics     []CustomMetric                `json:"custom_metrics"`
}

// StatusMapping maps MySQL status variables to nodes.
type StatusMapping struct {
	MySQLVariable string  `json:"mysql_variable"`
	NodePattern   string  `json:"node_pattern"`
	MetricType    string  `json:"metric_type"`
	ScaleFactor   float64 `json:"scale_factor"`
}

// PerfSchemaMapping maps Performance Schema metrics to nodes.
type PerfSchemaMapping struct {
	Query       string            `json:"query"`
	NodeMapping map[string]string `json:"node_mapping"`
}

// CustomMetric defines a custom metric.
type CustomMetric struct {
	Name       string                 `json:"name"`
	Expression string                 `json:"expression"`
	Variables  map[string]interface{} `json:"variables"`
}

// NewSimulationGraph creates a new empty simulation graph.
func NewSimulationGraph() *SimulationGraph {
	return &SimulationGraph{
		Nodes:       make(map[string]*SimNode),
		Edges:       make(map[string]*SimEdge),
		EntryPoints: make([]string, 0),
	}
}

// AddNode adds a node to the graph.
func (g *SimulationGraph) AddNode(node *SimNode) {
	g.Nodes[node.ID] = node
	g.TotalNodes = len(g.Nodes)
}

// AddEdge adds an edge to the graph.
func (g *SimulationGraph) AddEdge(edge *SimEdge) {
	edge.Index = len(g.Edges)
	g.Edges[edge.ID] = edge
	g.TotalEdges = len(g.Edges)
}

// GetOutEdges returns all outgoing edges from a node.
func (g *SimulationGraph) GetOutEdges(nodeID string) []*SimEdge {
	var edges []*SimEdge
	for _, edge := range g.Edges {
		if edge.FromNode == nodeID {
			edges = append(edges, edge)
		}
	}
	return edges
}

// GetInEdges returns all incoming edges to a node.
func (g *SimulationGraph) GetInEdges(nodeID string) []*SimEdge {
	var edges []*SimEdge
	for _, edge := range g.Edges {
		if edge.ToNode == nodeID {
			edges = append(edges, edge)
		}
	}
	return edges
}

// FindNodesByPattern finds nodes matching a pattern.
func (g *SimulationGraph) FindNodesByPattern(pattern string) []*SimNode {
	var nodes []*SimNode
	for _, node := range g.Nodes {
		if matchPattern(node.Name, pattern) || matchPattern(node.QualifiedName, pattern) {
			nodes = append(nodes, node)
		}
	}
	return nodes
}

// matchPattern performs simple pattern matching.
func matchPattern(text, pattern string) bool {
	// Simple contains match for now
	// Could be extended to support regex or glob patterns
	return text == pattern || 
		(len(pattern) > 0 && pattern[len(pattern)-1] == '*' && 
			len(text) >= len(pattern)-1 && 
			text[:len(pattern)-1] == pattern[:len(pattern)-1])
}

// ToJSON serializes the model to JSON.
func (m *SimulationModel) ToJSON() ([]byte, error) {
	return json.MarshalIndent(m, "", "  ")
}

// FromJSON deserializes the model from JSON.
func FromJSON(data []byte) (*SimulationModel, error) {
	var model SimulationModel
	if err := json.Unmarshal(data, &model); err != nil {
		return nil, err
	}
	return &model, nil
}
