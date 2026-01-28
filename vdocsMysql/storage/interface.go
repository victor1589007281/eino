// Package storage provides storage abstractions for the MySQL Expert Agent.
package storage

import (
	"context"
	"time"
)

// DocumentStore defines the interface for document storage operations.
type DocumentStore interface {
	// Document operations
	CreateDocument(ctx context.Context, doc *Document) error
	GetDocument(ctx context.Context, id int64) (*Document, error)
	GetDocumentByPath(ctx context.Context, filePath string) (*Document, error)
	UpdateDocument(ctx context.Context, doc *Document) error
	DeleteDocument(ctx context.Context, id int64) error
	ListDocuments(ctx context.Context, filter *DocumentFilter) ([]*Document, error)
	
	// Batch operations
	BatchCreateDocuments(ctx context.Context, docs []*Document) error
	BatchDeleteDocuments(ctx context.Context, ids []int64) error
	
	// Hash check for incremental updates
	GetDocumentHash(ctx context.Context, filePath string) (string, error)
	GetAllDocumentHashes(ctx context.Context) (map[string]string, error)
}

// InvertedIndexStore defines the interface for inverted index operations.
type InvertedIndexStore interface {
	// Index operations
	AddTermOccurrence(ctx context.Context, term string, docID int64, frequency int, positions []Position) error
	GetTermDocuments(ctx context.Context, term string) ([]*TermOccurrence, error)
	GetDocumentTerms(ctx context.Context, docID int64) ([]*TermOccurrence, error)
	DeleteDocumentTerms(ctx context.Context, docID int64) error
	
	// Batch operations
	BatchAddTermOccurrences(ctx context.Context, occurrences []*TermOccurrence) error
	
	// Search
	SearchTerms(ctx context.Context, query *TermSearchQuery) ([]*SearchResult, error)
	
	// Statistics
	GetTermDocumentCount(ctx context.Context, term string) (int, error)
	GetTotalDocumentCount(ctx context.Context) (int, error)
}

// FunctionSummaryStore defines the interface for function summary operations.
type FunctionSummaryStore interface {
	// Function operations
	CreateFunction(ctx context.Context, fn *FunctionSummary) error
	GetFunction(ctx context.Context, id string) (*FunctionSummary, error)
	GetFunctionByName(ctx context.Context, name string) ([]*FunctionSummary, error)
	UpdateFunction(ctx context.Context, fn *FunctionSummary) error
	DeleteFunction(ctx context.Context, id string) error
	DeleteFunctionsByFile(ctx context.Context, filePath string) error
	
	// Batch operations
	BatchCreateFunctions(ctx context.Context, fns []*FunctionSummary) error
	
	// Search
	SearchFunctions(ctx context.Context, query *FunctionSearchQuery) ([]*FunctionSummary, error)
	
	// Statistics
	GetFunctionCount(ctx context.Context) (int, error)
	GetFunctionsByModule(ctx context.Context, module string) ([]*FunctionSummary, error)
}

// CallGraphStore defines the interface for call graph operations.
type CallGraphStore interface {
	// Node operations
	CreateNode(ctx context.Context, node *CallGraphNode) error
	GetNode(ctx context.Context, id string) (*CallGraphNode, error)
	UpdateNode(ctx context.Context, node *CallGraphNode) error
	DeleteNode(ctx context.Context, id string) error
	
	// Edge operations
	CreateEdge(ctx context.Context, edge *CallGraphEdge) error
	GetEdge(ctx context.Context, id string) (*CallGraphEdge, error)
	DeleteEdge(ctx context.Context, id string) error
	
	// Graph traversal
	GetCallers(ctx context.Context, funcID string) ([]*CallGraphEdge, error)
	GetCallees(ctx context.Context, funcID string) ([]*CallGraphEdge, error)
	GetCallChain(ctx context.Context, funcID string, depth int, direction Direction) ([]*CallGraphNode, []*CallGraphEdge, error)
	
	// Batch operations
	BatchCreateNodes(ctx context.Context, nodes []*CallGraphNode) error
	BatchCreateEdges(ctx context.Context, edges []*CallGraphEdge) error
	
	// Module operations
	GetNodesByModule(ctx context.Context, module string) ([]*CallGraphNode, error)
	GetNodesByFile(ctx context.Context, filePath string) ([]*CallGraphNode, error)
}

// Storage combines all storage interfaces.
type Storage interface {
	DocumentStore
	InvertedIndexStore
	FunctionSummaryStore
	CallGraphStore
	
	// Transaction support
	BeginTx(ctx context.Context) (Transaction, error)
	
	// Lifecycle
	Init(ctx context.Context) error
	Close() error
	
	// Maintenance
	Vacuum(ctx context.Context) error
	Stats(ctx context.Context) (*StorageStats, error)
}

// Transaction represents a storage transaction.
type Transaction interface {
	Commit() error
	Rollback() error
}

// Direction represents graph traversal direction.
type Direction string

const (
	DirectionUp   Direction = "up"   // Callers
	DirectionDown Direction = "down" // Callees
	DirectionBoth Direction = "both"
)

// Document represents a source code file.
type Document struct {
	ID          int64     `json:"id"`
	FilePath    string    `json:"file_path"`
	FileHash    string    `json:"file_hash"`
	FileSize    int64     `json:"file_size"`
	LineCount   int       `json:"line_count"`
	Module      string    `json:"module"`
	Language    string    `json:"language"`
	LastIndexed time.Time `json:"last_indexed"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// DocumentFilter defines filter criteria for listing documents.
type DocumentFilter struct {
	Module   string
	Language string
	PathLike string
	Limit    int
	Offset   int
}

// Position represents a position in a document.
type Position struct {
	Line   int `json:"line"`
	Column int `json:"column"`
	Length int `json:"length"`
}

// TermOccurrence represents a term occurrence in a document.
type TermOccurrence struct {
	Term      string     `json:"term"`
	DocID     int64      `json:"doc_id"`
	Frequency int        `json:"frequency"`
	Positions []Position `json:"positions"`
}

// TermSearchQuery defines search parameters for term search.
type TermSearchQuery struct {
	Terms       []string
	Operator    SearchOperator // AND, OR
	FileTypes   []string
	Directories []string
	Limit       int
	Offset      int
}

// SearchOperator defines how multiple terms are combined.
type SearchOperator string

const (
	SearchOperatorAND SearchOperator = "AND"
	SearchOperatorOR  SearchOperator = "OR"
)

// SearchResult represents a search result.
type SearchResult struct {
	DocID     int64      `json:"doc_id"`
	FilePath  string     `json:"file_path"`
	Score     float64    `json:"score"`
	Positions []Position `json:"positions"`
	Snippet   string     `json:"snippet"`
}

// FunctionSummary represents a function summary.
type FunctionSummary struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	QualifiedName string    `json:"qualified_name"`
	FilePath      string    `json:"file_path"`
	LineStart     int       `json:"line_start"`
	LineEnd       int       `json:"line_end"`
	Signature     string    `json:"signature"`
	ReturnType    string    `json:"return_type"`
	Parameters    []Parameter `json:"parameters"`
	Description   string    `json:"description"`
	Tags          []string  `json:"tags"`
	Module        string    `json:"module"`
	Subsystem     string    `json:"subsystem"`
	LOC           int       `json:"loc"`
	Complexity    int       `json:"complexity"`
	Callers       []string  `json:"callers"`
	Callees       []string  `json:"callees"`
	Stats         map[string]interface{} `json:"stats,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// Parameter represents a function parameter.
type Parameter struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// FunctionSearchQuery defines search parameters for function search.
type FunctionSearchQuery struct {
	Query       string   // Free text query
	Name        string   // Exact or prefix match
	Module      string
	Subsystem   string
	Tags        []string
	MinLOC      int
	MaxLOC      int
	Limit       int
	Offset      int
}

// CallGraphNode represents a node in the call graph.
type CallGraphNode struct {
	ID            string                 `json:"id"`
	FunctionID    string                 `json:"function_id"`
	Name          string                 `json:"name"`
	QualifiedName string                 `json:"qualified_name"`
	FilePath      string                 `json:"file_path"`
	Line          int                    `json:"line"`
	Module        string                 `json:"module"`
	Subsystem     string                 `json:"subsystem"`
	NodeType      string                 `json:"node_type"`
	Stats         map[string]interface{} `json:"stats,omitempty"`
}

// CallGraphEdge represents an edge in the call graph.
type CallGraphEdge struct {
	ID          string `json:"id"`
	FromNodeID  string `json:"from_node_id"`
	ToNodeID    string `json:"to_node_id"`
	CallType    string `json:"call_type"` // direct, virtual, callback
	FilePath    string `json:"file_path"`
	Line        int    `json:"line"`
	Frequency   int64  `json:"frequency"`
	Probability float64 `json:"probability"`
	Condition   string `json:"condition,omitempty"`
}

// StorageStats contains storage statistics.
type StorageStats struct {
	DocumentCount     int   `json:"document_count"`
	TermCount         int   `json:"term_count"`
	FunctionCount     int   `json:"function_count"`
	NodeCount         int   `json:"node_count"`
	EdgeCount         int   `json:"edge_count"`
	TotalSize         int64 `json:"total_size"`
	IndexSize         int64 `json:"index_size"`
	LastIndexTime     time.Time `json:"last_index_time"`
}
