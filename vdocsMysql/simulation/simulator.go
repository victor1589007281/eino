package simulation

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"sort"
	"strings"
	"time"
)

// WorkloadSpec defines the workload specification for simulation.
type WorkloadSpec struct {
	Name            string             `json:"name"`
	Description     string             `json:"description"`
	Type            WorkloadType       `json:"type"`
	QPS             int                `json:"qps"`
	Concurrency     int                `json:"concurrency"`
	Duration        time.Duration      `json:"duration"`
	SQLDistribution map[string]float64 `json:"sql_distribution"`
	TableSizes      map[string]int64   `json:"table_sizes"`
	IndexUsage      map[string]float64 `json:"index_usage"`
	BufferPoolHitRatio float64         `json:"buffer_pool_hit_ratio"`
	EntryPoints     []EntryPointSpec   `json:"entry_points"`
}

// WorkloadType categorizes workload types.
type WorkloadType string

const (
	WorkloadTypeOLTP   WorkloadType = "oltp"
	WorkloadTypeOLAP   WorkloadType = "olap"
	WorkloadTypeMixed  WorkloadType = "mixed"
	WorkloadTypeCustom WorkloadType = "custom"
)

// EntryPointSpec defines a specific entry point with call count.
type EntryPointSpec struct {
	FunctionName string                 `json:"function_name"`
	CallCount    int64                  `json:"call_count"`
	Parameters   map[string]interface{} `json:"parameters"`
}

// SimulationResult holds the results of a simulation run.
type SimulationResult struct {
	Spec        *WorkloadSpec                   `json:"spec"`
	StartTime   time.Time                       `json:"start_time"`
	EndTime     time.Time                       `json:"end_time"`
	Model       *SimulationModel                `json:"-"`
	NodeStats   map[string]*SimulatedNodeStats  `json:"node_stats"`
	EdgeStats   map[string]*SimulatedEdgeStats  `json:"edge_stats"`
	Bottlenecks []Bottleneck                    `json:"bottlenecks"`
	HotPaths    []HotPath                       `json:"hot_paths"`
	Summary     *SimulationSummary              `json:"summary"`
}

// SimulatedNodeStats holds simulated statistics for a node.
type SimulatedNodeStats struct {
	NodeID     string        `json:"node_id"`
	NodeName   string        `json:"node_name"`
	CallCount  int64         `json:"call_count"`
	CPUTime    time.Duration `json:"cpu_time"`
	IOTime     time.Duration `json:"io_time"`
	LockTime   time.Duration `json:"lock_time"`
	MemoryUsed int64         `json:"memory_used"`
	TotalTime  time.Duration `json:"total_time"`
	IsHotPath  bool          `json:"is_hot_path"`
}

// SimulatedEdgeStats holds simulated statistics for an edge.
type SimulatedEdgeStats struct {
	EdgeID    string `json:"edge_id"`
	CallCount int64  `json:"call_count"`
	IsHotPath bool   `json:"is_hot_path"`
}

// Bottleneck represents a detected performance bottleneck.
type Bottleneck struct {
	Type        BottleneckType     `json:"type"`
	NodeID      string             `json:"node_id"`
	NodeName    string             `json:"node_name"`
	Severity    BottleneckSeverity `json:"severity"`
	Metric      string             `json:"metric"`
	Value       float64            `json:"value"`
	Percentage  float64            `json:"percentage"`
	Description string             `json:"description"`
	Suggestion  string             `json:"suggestion"`
}

// BottleneckType categorizes bottleneck types.
type BottleneckType string

const (
	BottleneckCPU    BottleneckType = "cpu"
	BottleneckIO     BottleneckType = "io"
	BottleneckLock   BottleneckType = "lock"
	BottleneckMemory BottleneckType = "memory"
)

// BottleneckSeverity indicates the severity level.
type BottleneckSeverity string

const (
	SeverityCritical BottleneckSeverity = "critical"
	SeverityHigh     BottleneckSeverity = "high"
	SeverityMedium   BottleneckSeverity = "medium"
	SeverityLow      BottleneckSeverity = "low"
)

// HotPath represents a frequently executed path.
type HotPath struct {
	Nodes     []HotPathNode `json:"nodes"`
	TotalTime time.Duration `json:"total_time"`
	CallCount int64         `json:"call_count"`
	Score     float64       `json:"score"`
}

// HotPathNode is a node in a hot path.
type HotPathNode struct {
	NodeID string        `json:"node_id"`
	Name   string        `json:"name"`
	Time   time.Duration `json:"time"`
}

// SimulationSummary provides a summary of the simulation.
type SimulationSummary struct {
	TotalCalls    int64         `json:"total_calls"`
	TotalCPUTime  time.Duration `json:"total_cpu_time"`
	TotalIOTime   time.Duration `json:"total_io_time"`
	TotalLockTime time.Duration `json:"total_lock_time"`
	TopCPUNodes   []string      `json:"top_cpu_nodes"`
	TopIONodes    []string      `json:"top_io_nodes"`
	TopLockNodes  []string      `json:"top_lock_nodes"`
}

// LoadSimulator performs load simulation on the model.
type LoadSimulator struct {
	model     *SimulationModel
	costModel *CostModel
	rng       *rand.Rand
}

// NewLoadSimulator creates a new load simulator.
func NewLoadSimulator(model *SimulationModel) *LoadSimulator {
	return &LoadSimulator{
		model:     model,
		costModel: NewCostModel(model),
		rng:       rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// Simulate runs the simulation with the given workload spec.
func (s *LoadSimulator) Simulate(ctx context.Context, spec *WorkloadSpec) (*SimulationResult, error) {
	result := &SimulationResult{
		Spec:      spec,
		StartTime: time.Now(),
		Model:     s.model,
		NodeStats: make(map[string]*SimulatedNodeStats),
		EdgeStats: make(map[string]*SimulatedEdgeStats),
	}

	// Initialize stats
	for nodeID, node := range s.model.Graph.Nodes {
		result.NodeStats[nodeID] = &SimulatedNodeStats{
			NodeID:   nodeID,
			NodeName: node.Name,
		}
	}
	for edgeID := range s.model.Graph.Edges {
		result.EdgeStats[edgeID] = &SimulatedEdgeStats{
			EdgeID: edgeID,
		}
	}

	// Determine entry points
	entryPoints := s.determineEntryPoints(spec)

	// Execute simulation for each entry point
	for _, entry := range entryPoints {
		for i := int64(0); i < entry.CallCount; i++ {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
				s.simulatePath(entry.FunctionName, spec, result)
			}
		}
	}

	result.EndTime = time.Now()

	// Analyze results
	s.analyzeBottlenecks(result)
	s.identifyHotPaths(result)
	s.generateSummary(result)

	return result, nil
}

// determineEntryPoints calculates entry points based on workload spec.
func (s *LoadSimulator) determineEntryPoints(spec *WorkloadSpec) []EntryPointSpec {
	entries := make([]EntryPointSpec, 0)

	if spec.SQLDistribution != nil {
		totalCalls := int64(spec.QPS) * int64(spec.Duration.Seconds())

		sqlToFunc := map[string]string{
			"SELECT": "Sql_cmd_select::execute",
			"INSERT": "Sql_cmd_insert::execute",
			"UPDATE": "Sql_cmd_update::execute",
			"DELETE": "Sql_cmd_delete::execute",
			"COMMIT": "trans_commit",
		}

		for sqlType, ratio := range spec.SQLDistribution {
			if funcName, ok := sqlToFunc[sqlType]; ok {
				entries = append(entries, EntryPointSpec{
					FunctionName: funcName,
					CallCount:    int64(float64(totalCalls) * ratio),
				})
			}
		}
	}

	// Add custom entry points
	entries = append(entries, spec.EntryPoints...)

	return entries
}

// simulatePath simulates a single execution path.
func (s *LoadSimulator) simulatePath(entryFunc string, spec *WorkloadSpec, result *SimulationResult) {
	visited := make(map[string]bool)
	s.traverse(entryFunc, spec, result, visited, 0)
}

// traverse recursively traverses the call graph.
func (s *LoadSimulator) traverse(nodeID string, spec *WorkloadSpec, result *SimulationResult, visited map[string]bool, depth int) {
	if depth > 50 || visited[nodeID] {
		return
	}

	node := s.model.Graph.Nodes[nodeID]
	if node == nil {
		return
	}

	visited[nodeID] = true

	// Calculate node cost
	cost := s.costModel.CalculateNodeCost(node, spec)

	// Update stats
	stats := result.NodeStats[nodeID]
	if stats == nil {
		stats = &SimulatedNodeStats{NodeID: nodeID, NodeName: node.Name}
		result.NodeStats[nodeID] = stats
	}
	stats.CallCount++
	stats.CPUTime += cost.CPUTime
	stats.IOTime += cost.IOTime
	stats.LockTime += cost.LockTime
	stats.MemoryUsed += cost.MemoryUsed
	stats.TotalTime += cost.CPUTime + cost.IOTime + cost.LockTime

	// Traverse children based on probability
	for _, edge := range s.model.Graph.GetOutEdges(nodeID) {
		if s.rng.Float64() < edge.Probability {
			// Update edge stats
			if edgeStats := result.EdgeStats[edge.ID]; edgeStats != nil {
				edgeStats.CallCount++
			}

			s.traverse(edge.ToNode, spec, result, visited, depth+1)
		}
	}
}

// analyzeBottlenecks identifies performance bottlenecks.
func (s *LoadSimulator) analyzeBottlenecks(result *SimulationResult) {
	thresholds := &BottleneckThresholds{
		CPUHotRatio:  0.1,
		IOHotRatio:   0.15,
		LockHotRatio: 0.2,
		MinCallCount: 100,
	}

	// Calculate totals
	var totalCPU, totalIO, totalLock time.Duration
	for _, stats := range result.NodeStats {
		totalCPU += stats.CPUTime
		totalIO += stats.IOTime
		totalLock += stats.LockTime
	}

	bottlenecks := make([]Bottleneck, 0)

	for nodeID, stats := range result.NodeStats {
		if stats.CallCount < thresholds.MinCallCount {
			continue
		}

		// CPU bottleneck
		if totalCPU > 0 {
			cpuRatio := float64(stats.CPUTime) / float64(totalCPU)
			if cpuRatio > thresholds.CPUHotRatio {
				bottlenecks = append(bottlenecks, Bottleneck{
					Type:        BottleneckCPU,
					NodeID:      nodeID,
					NodeName:    stats.NodeName,
					Severity:    calculateSeverity(cpuRatio),
					Metric:      "cpu_time",
					Value:       float64(stats.CPUTime),
					Percentage:  cpuRatio * 100,
					Description: fmt.Sprintf("函数 %s 占用 %.2f%% 的CPU时间", stats.NodeName, cpuRatio*100),
					Suggestion:  "检查是否可以添加索引或使用缓存",
				})
			}
		}

		// IO bottleneck
		if totalIO > 0 {
			ioRatio := float64(stats.IOTime) / float64(totalIO)
			if ioRatio > thresholds.IOHotRatio {
				bottlenecks = append(bottlenecks, Bottleneck{
					Type:        BottleneckIO,
					NodeID:      nodeID,
					NodeName:    stats.NodeName,
					Severity:    calculateSeverity(ioRatio),
					Metric:      "io_time",
					Value:       float64(stats.IOTime),
					Percentage:  ioRatio * 100,
					Description: fmt.Sprintf("函数 %s 占用 %.2f%% 的IO时间", stats.NodeName, ioRatio*100),
					Suggestion:  "增加Buffer Pool大小或优化IO访问模式",
				})
			}
		}

		// Lock bottleneck
		if totalLock > 0 {
			lockRatio := float64(stats.LockTime) / float64(totalLock)
			if lockRatio > thresholds.LockHotRatio {
				bottlenecks = append(bottlenecks, Bottleneck{
					Type:        BottleneckLock,
					NodeID:      nodeID,
					NodeName:    stats.NodeName,
					Severity:    calculateSeverity(lockRatio),
					Metric:      "lock_time",
					Value:       float64(stats.LockTime),
					Percentage:  lockRatio * 100,
					Description: fmt.Sprintf("函数 %s 占用 %.2f%% 的锁等待时间", stats.NodeName, lockRatio*100),
					Suggestion:  "优化事务提交时机或降低隔离级别",
				})
			}
		}
	}

	// Sort by percentage
	sort.Slice(bottlenecks, func(i, j int) bool {
		return bottlenecks[i].Percentage > bottlenecks[j].Percentage
	})

	result.Bottlenecks = bottlenecks
}

// identifyHotPaths finds frequently executed paths.
func (s *LoadSimulator) identifyHotPaths(result *SimulationResult) {
	// Find top nodes by call count
	type nodeScore struct {
		ID    string
		Score float64
	}

	scores := make([]nodeScore, 0)
	for id, stats := range result.NodeStats {
		if stats.CallCount > 0 {
			scores = append(scores, nodeScore{
				ID:    id,
				Score: float64(stats.CallCount) * float64(stats.TotalTime),
			})
		}
	}

	sort.Slice(scores, func(i, j int) bool {
		return scores[i].Score > scores[j].Score
	})

	// Mark hot paths
	hotNodeIDs := make(map[string]bool)
	for i := 0; i < min(10, len(scores)); i++ {
		hotNodeIDs[scores[i].ID] = true
		if stats := result.NodeStats[scores[i].ID]; stats != nil {
			stats.IsHotPath = true
		}
	}

	// Build hot paths
	hotPaths := make([]HotPath, 0)
	for _, entry := range s.model.Graph.EntryPoints {
		path := s.buildHotPath(entry, result, hotNodeIDs)
		if len(path.Nodes) > 0 {
			hotPaths = append(hotPaths, path)
		}
	}

	sort.Slice(hotPaths, func(i, j int) bool {
		return hotPaths[i].Score > hotPaths[j].Score
	})

	if len(hotPaths) > 5 {
		hotPaths = hotPaths[:5]
	}

	result.HotPaths = hotPaths
}

// buildHotPath builds a hot path starting from an entry point.
func (s *LoadSimulator) buildHotPath(entryID string, result *SimulationResult, hotNodeIDs map[string]bool) HotPath {
	path := HotPath{
		Nodes: make([]HotPathNode, 0),
	}

	visited := make(map[string]bool)
	s.buildPathRecursive(entryID, result, hotNodeIDs, visited, &path)

	return path
}

func (s *LoadSimulator) buildPathRecursive(nodeID string, result *SimulationResult, hotNodeIDs map[string]bool, visited map[string]bool, path *HotPath) {
	if visited[nodeID] || len(path.Nodes) > 10 {
		return
	}
	visited[nodeID] = true

	stats := result.NodeStats[nodeID]
	if stats == nil {
		return
	}

	node := HotPathNode{
		NodeID: nodeID,
		Name:   stats.NodeName,
		Time:   stats.TotalTime,
	}
	path.Nodes = append(path.Nodes, node)
	path.TotalTime += stats.TotalTime
	path.CallCount += stats.CallCount
	path.Score += float64(stats.CallCount) * float64(stats.TotalTime)

	// Find hottest child
	var hottestChild string
	var maxScore float64

	for _, edge := range s.model.Graph.GetOutEdges(nodeID) {
		if childStats := result.NodeStats[edge.ToNode]; childStats != nil && hotNodeIDs[edge.ToNode] {
			score := float64(childStats.CallCount) * float64(childStats.TotalTime)
			if score > maxScore {
				maxScore = score
				hottestChild = edge.ToNode
			}
		}
	}

	if hottestChild != "" {
		s.buildPathRecursive(hottestChild, result, hotNodeIDs, visited, path)
	}
}

// generateSummary generates a summary of the simulation.
func (s *LoadSimulator) generateSummary(result *SimulationResult) {
	summary := &SimulationSummary{}

	for _, stats := range result.NodeStats {
		summary.TotalCalls += stats.CallCount
		summary.TotalCPUTime += stats.CPUTime
		summary.TotalIOTime += stats.IOTime
		summary.TotalLockTime += stats.LockTime
	}

	// Find top nodes
	type nodeMetric struct {
		Name  string
		Value time.Duration
	}

	cpuNodes := make([]nodeMetric, 0)
	ioNodes := make([]nodeMetric, 0)
	lockNodes := make([]nodeMetric, 0)

	for _, stats := range result.NodeStats {
		cpuNodes = append(cpuNodes, nodeMetric{stats.NodeName, stats.CPUTime})
		ioNodes = append(ioNodes, nodeMetric{stats.NodeName, stats.IOTime})
		lockNodes = append(lockNodes, nodeMetric{stats.NodeName, stats.LockTime})
	}

	sort.Slice(cpuNodes, func(i, j int) bool { return cpuNodes[i].Value > cpuNodes[j].Value })
	sort.Slice(ioNodes, func(i, j int) bool { return ioNodes[i].Value > ioNodes[j].Value })
	sort.Slice(lockNodes, func(i, j int) bool { return lockNodes[i].Value > lockNodes[j].Value })

	for i := 0; i < min(5, len(cpuNodes)); i++ {
		summary.TopCPUNodes = append(summary.TopCPUNodes, cpuNodes[i].Name)
	}
	for i := 0; i < min(5, len(ioNodes)); i++ {
		summary.TopIONodes = append(summary.TopIONodes, ioNodes[i].Name)
	}
	for i := 0; i < min(5, len(lockNodes)); i++ {
		summary.TopLockNodes = append(summary.TopLockNodes, lockNodes[i].Name)
	}

	result.Summary = summary
}

// BottleneckThresholds defines thresholds for bottleneck detection.
type BottleneckThresholds struct {
	CPUHotRatio  float64
	IOHotRatio   float64
	LockHotRatio float64
	MinCallCount int64
}

func calculateSeverity(ratio float64) BottleneckSeverity {
	if ratio > 0.5 {
		return SeverityCritical
	}
	if ratio > 0.3 {
		return SeverityHigh
	}
	if ratio > 0.15 {
		return SeverityMedium
	}
	return SeverityLow
}

// CostModel calculates costs for nodes.
type CostModel struct {
	model *SimulationModel
}

// NewCostModel creates a new cost model.
func NewCostModel(model *SimulationModel) *CostModel {
	return &CostModel{model: model}
}

// NodeCost represents the calculated cost of a node.
type NodeCost struct {
	CPUTime    time.Duration
	IOTime     time.Duration
	LockTime   time.Duration
	MemoryUsed int64
}

// CalculateNodeCost calculates the cost for a node.
func (c *CostModel) CalculateNodeCost(node *SimNode, spec *WorkloadSpec) *NodeCost {
	cost := &NodeCost{}

	if node.CostModel == nil {
		// Default cost
		cost.CPUTime = 10 * time.Microsecond
		return cost
	}

	cm := node.CostModel

	// Base costs
	cost.CPUTime = time.Duration(cm.BaseCPUCost * float64(time.Microsecond))
	cost.IOTime = time.Duration(cm.BaseIOCost * float64(time.Microsecond))
	cost.LockTime = time.Duration(cm.BaseLockCost * float64(time.Microsecond))
	cost.MemoryUsed = int64(cm.BaseMemoryCost)

	// Apply scale factors
	for varName, factor := range cm.ScaleFactors {
		switch varName {
		case "table_size":
			if size, ok := spec.TableSizes["default"]; ok {
				scaleFactor := math.Log10(float64(size + 1))
				cost.CPUTime = time.Duration(float64(cost.CPUTime) * scaleFactor * factor)
				cost.IOTime = time.Duration(float64(cost.IOTime) * scaleFactor * factor)
			}
		case "buffer_pool_miss":
			missRatio := 1.0 - spec.BufferPoolHitRatio
			cost.IOTime = time.Duration(float64(cost.IOTime) * (1 + missRatio*10) * factor)
		}
	}

	// Concurrency penalty
	if spec.Concurrency > 1 && cm.ConcurrencyFactor > 0 {
		penalty := 1 + cm.ConcurrencyFactor*math.Log10(float64(spec.Concurrency))
		cost.LockTime = time.Duration(float64(cost.LockTime) * penalty)
	}

	return cost
}

// GenerateMermaidDiagram generates a Mermaid diagram from the result.
func GenerateMermaidDiagram(result *SimulationResult) string {
	var sb strings.Builder

	sb.WriteString("graph TD\n")

	// Group nodes by module
	modules := make(map[string][]string)
	for nodeID, node := range result.Model.Graph.Nodes {
		modules[node.Module] = append(modules[node.Module], nodeID)
	}

	// Generate subgraphs
	for module, nodeIDs := range modules {
		sb.WriteString(fmt.Sprintf("    subgraph \"%s\"\n", module))

		for _, nodeID := range nodeIDs {
			stats := result.NodeStats[nodeID]
			node := result.Model.Graph.Nodes[nodeID]
			if stats == nil || node == nil {
				continue
			}

			label := fmt.Sprintf("**%s**<br/>调用: %d", node.Name, stats.CallCount)
			sb.WriteString(fmt.Sprintf("        %s[%s]\n", nodeID, label))

			// Style based on hot path
			color := "#dfe6e9"
			if stats.IsHotPath {
				color = "#e74c3c"
			} else {
				switch node.NodeType {
				case NodeTypeEntry:
					color = "#ff6b6b"
				case NodeTypeIO:
					color = "#45b7d1"
				case NodeTypeLock:
					color = "#f9ca24"
				}
			}
			sb.WriteString(fmt.Sprintf("        style %s fill:%s,stroke:#333,stroke-width:2px,color:#000\n", nodeID, color))
		}

		sb.WriteString("    end\n")
	}

	// Generate edges
	for _, edge := range result.Model.Graph.Edges {
		sb.WriteString(fmt.Sprintf("    %s --> %s\n", edge.FromNode, edge.ToNode))
	}

	return sb.String()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
