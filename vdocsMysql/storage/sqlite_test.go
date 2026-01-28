package storage

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestSQLiteStorage(t *testing.T) {
	// Create temp database
	tmpFile, err := os.CreateTemp("", "test_sqlite_*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// Create storage
	storage, err := NewSQLiteStorage(&SQLiteConfig{
		Path:            tmpFile.Name(),
		MaxOpenConns:    5,
		MaxIdleConns:    2,
		ConnMaxLifetime: time.Hour,
		JournalMode:     "WAL",
		SynchronousMode: "NORMAL",
	})
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	ctx := context.Background()

	// Initialize schema
	if err := storage.Init(ctx); err != nil {
		t.Fatalf("Failed to init storage: %v", err)
	}

	// Test document operations
	t.Run("DocumentCRUD", func(t *testing.T) {
		testDocumentCRUD(t, storage, ctx)
	})

	// Test inverted index operations
	t.Run("InvertedIndex", func(t *testing.T) {
		testInvertedIndex(t, storage, ctx)
	})

	// Test function summary operations
	t.Run("FunctionSummary", func(t *testing.T) {
		testFunctionSummary(t, storage, ctx)
	})

	// Test batch operations
	t.Run("BatchOperations", func(t *testing.T) {
		testBatchOperations(t, storage, ctx)
	})

	// Test search operations
	t.Run("SearchOperations", func(t *testing.T) {
		testSearchOperations(t, storage, ctx)
	})
}

func testDocumentCRUD(t *testing.T, s *SQLiteStorage, ctx context.Context) {
	// Create document
	doc := &Document{
		FilePath:    "test/path/file.cc",
		FileHash:    "abc123",
		FileSize:    1000,
		LineCount:   100,
		Module:      "SQL",
		Language:    "C++",
		LastIndexed: time.Now(),
		Metadata:    map[string]interface{}{"key": "value"},
	}

	if err := s.CreateDocument(ctx, doc); err != nil {
		t.Fatalf("Failed to create document: %v", err)
	}

	if doc.ID == 0 {
		t.Error("Document ID should be set after creation")
	}

	// Get document by ID
	retrieved, err := s.GetDocument(ctx, doc.ID)
	if err != nil {
		t.Fatalf("Failed to get document: %v", err)
	}
	if retrieved == nil {
		t.Fatal("Document not found")
	}
	if retrieved.FilePath != doc.FilePath {
		t.Errorf("Expected file path %s, got %s", doc.FilePath, retrieved.FilePath)
	}

	// Get document by path
	retrieved, err = s.GetDocumentByPath(ctx, doc.FilePath)
	if err != nil {
		t.Fatalf("Failed to get document by path: %v", err)
	}
	if retrieved == nil {
		t.Fatal("Document not found by path")
	}

	// Update document
	doc.FileHash = "def456"
	if err := s.UpdateDocument(ctx, doc); err != nil {
		t.Fatalf("Failed to update document: %v", err)
	}

	// Verify update
	retrieved, _ = s.GetDocument(ctx, doc.ID)
	if retrieved.FileHash != "def456" {
		t.Error("Document hash not updated")
	}

	// Get document hash
	hash, err := s.GetDocumentHash(ctx, doc.FilePath)
	if err != nil {
		t.Fatalf("Failed to get document hash: %v", err)
	}
	if hash != "def456" {
		t.Errorf("Expected hash def456, got %s", hash)
	}

	// Delete document
	if err := s.DeleteDocument(ctx, doc.ID); err != nil {
		t.Fatalf("Failed to delete document: %v", err)
	}

	// Verify deletion
	retrieved, _ = s.GetDocument(ctx, doc.ID)
	if retrieved != nil {
		t.Error("Document should be deleted")
	}
}

func testInvertedIndex(t *testing.T, s *SQLiteStorage, ctx context.Context) {
	// Create a document first
	doc := &Document{
		FilePath: "test/inverted/file.cc",
		FileHash: "idx123",
		FileSize: 500,
		Module:   "Test",
		Language: "C++",
	}
	if err := s.CreateDocument(ctx, doc); err != nil {
		t.Fatalf("Failed to create document: %v", err)
	}

	// Add term occurrence
	positions := []Position{
		{Line: 10, Column: 5, Length: 10},
		{Line: 20, Column: 15, Length: 10},
	}
	if err := s.AddTermOccurrence(ctx, "testTerm", doc.ID, 2, positions); err != nil {
		t.Fatalf("Failed to add term occurrence: %v", err)
	}

	// Get term documents
	occurrences, err := s.GetTermDocuments(ctx, "testTerm")
	if err != nil {
		t.Fatalf("Failed to get term documents: %v", err)
	}
	if len(occurrences) != 1 {
		t.Errorf("Expected 1 occurrence, got %d", len(occurrences))
	}
	if occurrences[0].Frequency != 2 {
		t.Errorf("Expected frequency 2, got %d", occurrences[0].Frequency)
	}
	if len(occurrences[0].Positions) != 2 {
		t.Errorf("Expected 2 positions, got %d", len(occurrences[0].Positions))
	}

	// Get document terms
	terms, err := s.GetDocumentTerms(ctx, doc.ID)
	if err != nil {
		t.Fatalf("Failed to get document terms: %v", err)
	}
	if len(terms) != 1 {
		t.Errorf("Expected 1 term, got %d", len(terms))
	}

	// Get term document count
	count, err := s.GetTermDocumentCount(ctx, "testTerm")
	if err != nil {
		t.Fatalf("Failed to get term document count: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected count 1, got %d", count)
	}

	// Delete document terms
	if err := s.DeleteDocumentTerms(ctx, doc.ID); err != nil {
		t.Fatalf("Failed to delete document terms: %v", err)
	}

	// Verify deletion
	terms, _ = s.GetDocumentTerms(ctx, doc.ID)
	if len(terms) != 0 {
		t.Error("Terms should be deleted")
	}

	// Cleanup
	s.DeleteDocument(ctx, doc.ID)
}

func testFunctionSummary(t *testing.T, s *SQLiteStorage, ctx context.Context) {
	fn := &FunctionSummary{
		ID:            "test_func_1",
		Name:          "test_function",
		QualifiedName: "namespace::test_function",
		FilePath:      "test/func/file.cc",
		LineStart:     10,
		LineEnd:       50,
		Signature:     "int test_function(int arg)",
		ReturnType:    "int",
		Parameters: []Parameter{
			{Name: "arg", Type: "int"},
		},
		Description: "A test function",
		Tags:        []string{"test", "example"},
		Module:      "Test",
		Subsystem:   "Unit",
		LOC:         40,
		Complexity:  5,
		Callers:     []string{"caller1"},
		Callees:     []string{"callee1"},
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Create function
	if err := s.CreateFunction(ctx, fn); err != nil {
		t.Fatalf("Failed to create function: %v", err)
	}

	// Get function
	retrieved, err := s.GetFunction(ctx, fn.ID)
	if err != nil {
		t.Fatalf("Failed to get function: %v", err)
	}
	if retrieved == nil {
		t.Fatal("Function not found")
	}
	if retrieved.Name != fn.Name {
		t.Errorf("Expected name %s, got %s", fn.Name, retrieved.Name)
	}
	if len(retrieved.Parameters) != 1 {
		t.Errorf("Expected 1 parameter, got %d", len(retrieved.Parameters))
	}

	// Get function by name
	funcs, err := s.GetFunctionByName(ctx, fn.Name)
	if err != nil {
		t.Fatalf("Failed to get function by name: %v", err)
	}
	if len(funcs) != 1 {
		t.Errorf("Expected 1 function, got %d", len(funcs))
	}

	// Update function
	fn.Description = "Updated description"
	if err := s.UpdateFunction(ctx, fn); err != nil {
		t.Fatalf("Failed to update function: %v", err)
	}

	// Verify update
	retrieved, _ = s.GetFunction(ctx, fn.ID)
	if retrieved.Description != "Updated description" {
		t.Error("Function description not updated")
	}

	// Get function count
	count, err := s.GetFunctionCount(ctx)
	if err != nil {
		t.Fatalf("Failed to get function count: %v", err)
	}
	if count < 1 {
		t.Errorf("Expected at least 1 function, got %d", count)
	}

	// Get functions by module
	funcs, err = s.GetFunctionsByModule(ctx, "Test")
	if err != nil {
		t.Fatalf("Failed to get functions by module: %v", err)
	}
	if len(funcs) < 1 {
		t.Error("Expected at least 1 function in module")
	}

	// Delete function
	if err := s.DeleteFunction(ctx, fn.ID); err != nil {
		t.Fatalf("Failed to delete function: %v", err)
	}

	// Verify deletion
	retrieved, _ = s.GetFunction(ctx, fn.ID)
	if retrieved != nil {
		t.Error("Function should be deleted")
	}
}

func testBatchOperations(t *testing.T, s *SQLiteStorage, ctx context.Context) {
	// Batch create documents
	docs := []*Document{
		{FilePath: "batch/file1.cc", FileHash: "hash1", FileSize: 100, Module: "M1", Language: "C++"},
		{FilePath: "batch/file2.cc", FileHash: "hash2", FileSize: 200, Module: "M2", Language: "C++"},
		{FilePath: "batch/file3.cc", FileHash: "hash3", FileSize: 300, Module: "M1", Language: "C++"},
	}

	if err := s.BatchCreateDocuments(ctx, docs); err != nil {
		t.Fatalf("Failed to batch create documents: %v", err)
	}

	// Verify IDs are set
	for _, doc := range docs {
		if doc.ID == 0 {
			t.Error("Document ID should be set after batch creation")
		}
	}

	// List documents with filter
	filter := &DocumentFilter{
		Module: "M1",
		Limit:  10,
	}
	filtered, err := s.ListDocuments(ctx, filter)
	if err != nil {
		t.Fatalf("Failed to list documents: %v", err)
	}
	if len(filtered) != 2 {
		t.Errorf("Expected 2 documents with module M1, got %d", len(filtered))
	}

	// Batch add term occurrences
	occurrences := []*TermOccurrence{
		{Term: "term1", DocID: docs[0].ID, Frequency: 1, Positions: []Position{{Line: 1}}},
		{Term: "term2", DocID: docs[0].ID, Frequency: 2, Positions: []Position{{Line: 2}, {Line: 3}}},
		{Term: "term1", DocID: docs[1].ID, Frequency: 1, Positions: []Position{{Line: 5}}},
	}

	if err := s.BatchAddTermOccurrences(ctx, occurrences); err != nil {
		t.Fatalf("Failed to batch add term occurrences: %v", err)
	}

	// Verify term occurrences
	term1Docs, _ := s.GetTermDocuments(ctx, "term1")
	if len(term1Docs) != 2 {
		t.Errorf("Expected term1 in 2 documents, got %d", len(term1Docs))
	}

	// Batch create functions
	funcs := []*FunctionSummary{
		{ID: "batch_fn1", Name: "func1", FilePath: "batch/file1.cc", Module: "M1"},
		{ID: "batch_fn2", Name: "func2", FilePath: "batch/file1.cc", Module: "M1"},
	}

	if err := s.BatchCreateFunctions(ctx, funcs); err != nil {
		t.Fatalf("Failed to batch create functions: %v", err)
	}

	// Verify functions created
	for _, fn := range funcs {
		retrieved, _ := s.GetFunction(ctx, fn.ID)
		if retrieved == nil {
			t.Errorf("Function %s not found after batch creation", fn.ID)
		}
	}

	// Batch delete documents
	ids := []int64{docs[0].ID, docs[1].ID, docs[2].ID}
	if err := s.BatchDeleteDocuments(ctx, ids); err != nil {
		t.Fatalf("Failed to batch delete documents: %v", err)
	}

	// Verify deletion
	for _, id := range ids {
		doc, _ := s.GetDocument(ctx, id)
		if doc != nil {
			t.Errorf("Document %d should be deleted", id)
		}
	}

	// Cleanup functions
	for _, fn := range funcs {
		s.DeleteFunction(ctx, fn.ID)
	}
}

func testSearchOperations(t *testing.T, s *SQLiteStorage, ctx context.Context) {
	// Create test data
	docs := []*Document{
		{FilePath: "search/sql/parse.cc", FileHash: "s1", Module: "SQL", Language: "C++"},
		{FilePath: "search/innodb/trx.cc", FileHash: "s2", Module: "InnoDB", Language: "C++"},
	}
	s.BatchCreateDocuments(ctx, docs)

	// Add terms
	s.AddTermOccurrence(ctx, "mysql_parse", docs[0].ID, 3, []Position{{Line: 10}})
	s.AddTermOccurrence(ctx, "execute", docs[0].ID, 5, []Position{{Line: 20}})
	s.AddTermOccurrence(ctx, "trx_commit", docs[1].ID, 2, []Position{{Line: 30}})
	s.AddTermOccurrence(ctx, "execute", docs[1].ID, 1, []Position{{Line: 40}})

	// Search with OR operator
	query := &TermSearchQuery{
		Terms:    []string{"mysql_parse", "trx_commit"},
		Operator: SearchOperatorOR,
		Limit:    10,
	}
	results, err := s.SearchTerms(ctx, query)
	if err != nil {
		t.Fatalf("Failed to search terms: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("Expected 2 results with OR, got %d", len(results))
	}

	// Search with AND operator (common term)
	query = &TermSearchQuery{
		Terms:    []string{"execute"},
		Operator: SearchOperatorAND,
		Limit:    10,
	}
	results, err = s.SearchTerms(ctx, query)
	if err != nil {
		t.Fatalf("Failed to search terms: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("Expected 2 results for 'execute', got %d", len(results))
	}

	// Create functions for FTS test
	funcs := []*FunctionSummary{
		{
			ID:          "search_fn1",
			Name:        "mysql_parse",
			Description: "Parse SQL statement",
			FilePath:    "search/sql/parse.cc",
			Module:      "SQL",
			Tags:        []string{"parser", "sql"},
		},
		{
			ID:          "search_fn2",
			Name:        "trx_commit",
			Description: "Commit transaction",
			FilePath:    "search/innodb/trx.cc",
			Module:      "InnoDB",
			Tags:        []string{"transaction", "commit"},
		},
	}
	s.BatchCreateFunctions(ctx, funcs)

	// Search functions by name
	fnQuery := &FunctionSearchQuery{
		Name:  "mysql_parse",
		Limit: 10,
	}
	fnResults, err := s.SearchFunctions(ctx, fnQuery)
	if err != nil {
		t.Fatalf("Failed to search functions: %v", err)
	}
	if len(fnResults) != 1 {
		t.Errorf("Expected 1 function result, got %d", len(fnResults))
	}

	// Search functions by module
	fnQuery = &FunctionSearchQuery{
		Module: "InnoDB",
		Limit:  10,
	}
	fnResults, err = s.SearchFunctions(ctx, fnQuery)
	if err != nil {
		t.Fatalf("Failed to search functions by module: %v", err)
	}
	if len(fnResults) != 1 {
		t.Errorf("Expected 1 function in InnoDB, got %d", len(fnResults))
	}

	// Cleanup
	for _, doc := range docs {
		s.DeleteDocumentTerms(ctx, doc.ID)
		s.DeleteDocument(ctx, doc.ID)
	}
	for _, fn := range funcs {
		s.DeleteFunction(ctx, fn.ID)
	}
}

func TestSQLiteStorageStats(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test_stats_*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	storage, err := NewSQLiteStorage(&SQLiteConfig{
		Path:        tmpFile.Name(),
		JournalMode: "WAL",
	})
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	ctx := context.Background()
	storage.Init(ctx)

	// Add some data
	docs := []*Document{
		{FilePath: "stats/f1.cc", FileHash: "h1"},
		{FilePath: "stats/f2.cc", FileHash: "h2"},
	}
	storage.BatchCreateDocuments(ctx, docs)

	storage.AddTermOccurrence(ctx, "term1", docs[0].ID, 1, nil)
	storage.AddTermOccurrence(ctx, "term2", docs[1].ID, 1, nil)

	storage.CreateFunction(ctx, &FunctionSummary{ID: "stats_fn1", Name: "fn1"})

	// Get stats
	stats, err := storage.Stats(ctx)
	if err != nil {
		t.Fatalf("Failed to get stats: %v", err)
	}

	if stats.DocumentCount != 2 {
		t.Errorf("Expected 2 documents, got %d", stats.DocumentCount)
	}
	if stats.TermCount != 2 {
		t.Errorf("Expected 2 terms, got %d", stats.TermCount)
	}
	if stats.FunctionCount != 1 {
		t.Errorf("Expected 1 function, got %d", stats.FunctionCount)
	}
}

func TestSQLiteStorageVacuum(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test_vacuum_*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	storage, err := NewSQLiteStorage(&SQLiteConfig{
		Path:        tmpFile.Name(),
		JournalMode: "WAL",
	})
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	ctx := context.Background()
	storage.Init(ctx)

	// Vacuum should not error
	if err := storage.Vacuum(ctx); err != nil {
		t.Errorf("Vacuum failed: %v", err)
	}
}
