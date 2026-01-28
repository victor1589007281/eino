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
	"context"
	"encoding/gob"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// IndexBuilder builds and manages the complete index.
type IndexBuilder struct {
	sourcePath string
	indexPath  string

	inverted  *InvertedIndex
	functions *FunctionSummaryDB
	symbols   *SymbolTable
	callGraph *CallGraph

	workers int
	mu      sync.Mutex
}

// NewIndexBuilder creates a new index builder.
func NewIndexBuilder(sourcePath, indexPath string, workers int) *IndexBuilder {
	if workers <= 0 {
		workers = 4
	}
	return &IndexBuilder{
		sourcePath: sourcePath,
		indexPath:  indexPath,
		inverted:   NewInvertedIndex(),
		functions:  NewFunctionSummaryDB(),
		symbols:    NewSymbolTable(),
		callGraph:  NewCallGraph(),
		workers:    workers,
	}
}

// Build builds the complete index.
func (b *IndexBuilder) Build(ctx context.Context) error {
	startTime := time.Now()
	fmt.Printf("Starting index build for %s\n", b.sourcePath)

	// Collect files to index
	files, err := b.collectFiles()
	if err != nil {
		return fmt.Errorf("failed to collect files: %w", err)
	}
	fmt.Printf("Found %d files to index\n", len(files))

	// Build inverted index in parallel
	if err := b.buildInvertedIndex(ctx, files); err != nil {
		return fmt.Errorf("failed to build inverted index: %w", err)
	}

	// Parse ctags if available
	ctagsFile := filepath.Join(b.indexPath, "tags")
	if _, err := os.Stat(ctagsFile); err == nil {
		if err := b.loadCTags(ctagsFile); err != nil {
			fmt.Printf("Warning: failed to load ctags: %v\n", err)
		}
	}

	// Save index
	if err := b.Save(); err != nil {
		return fmt.Errorf("failed to save index: %w", err)
	}

	elapsed := time.Since(startTime)
	fmt.Printf("Index build completed in %v\n", elapsed)
	b.printStats()

	return nil
}

// collectFiles collects all C source files.
func (b *IndexBuilder) collectFiles() ([]string, error) {
	files := make([]string, 0)
	extensions := map[string]bool{
		".c": true, ".h": true, ".S": true,
	}

	err := filepath.Walk(b.sourcePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}
		if info.IsDir() {
			// Skip some directories
			name := info.Name()
			if name == ".git" || name == "Documentation" || name == "tools" {
				return filepath.SkipDir
			}
			return nil
		}

		ext := filepath.Ext(path)
		if extensions[ext] {
			files = append(files, path)
		}
		return nil
	})

	return files, err
}

// buildInvertedIndex builds the inverted index in parallel.
func (b *IndexBuilder) buildInvertedIndex(ctx context.Context, files []string) error {
	fileCh := make(chan string, len(files))
	errCh := make(chan error, b.workers)
	var wg sync.WaitGroup

	// Start workers
	for i := 0; i < b.workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for file := range fileCh {
				select {
				case <-ctx.Done():
					return
				default:
				}

				if err := b.indexFile(file); err != nil {
					errCh <- fmt.Errorf("indexing %s: %w", file, err)
				}
			}
		}()
	}

	// Send files to workers
	for _, file := range files {
		fileCh <- file
	}
	close(fileCh)

	wg.Wait()
	close(errCh)

	// Collect errors
	var errs []error
	for err := range errCh {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return fmt.Errorf("indexing errors: %v", errs)
	}

	return nil
}

// indexFile indexes a single file.
func (b *IndexBuilder) indexFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		return err
	}

	info, _ := file.Stat()
	relPath, _ := filepath.Rel(b.sourcePath, path)

	b.mu.Lock()
	b.inverted.AddDocument(relPath, string(content), info.Size())
	b.mu.Unlock()

	return nil
}

// loadCTags loads symbols from ctags file.
func (b *IndexBuilder) loadCTags(path string) error {
	symbols, err := ParseCTagsFile(path)
	if err != nil {
		return err
	}

	for _, sym := range symbols {
		b.symbols.AddSymbol(sym)

		// Add functions to function summary DB
		if sym.Kind == SymbolFunction {
			funcInfo := &FunctionInfo{
				Name:      sym.Name,
				File:      sym.File,
				StartLine: sym.Line,
				Signature: sym.Pattern,
			}
			b.functions.AddFunction(funcInfo)
		}
	}

	return nil
}

// Save saves the index to disk.
func (b *IndexBuilder) Save() error {
	if err := os.MkdirAll(b.indexPath, 0755); err != nil {
		return err
	}

	// Save inverted index
	if err := b.saveGob(filepath.Join(b.indexPath, "inverted.gob"), b.inverted); err != nil {
		return fmt.Errorf("saving inverted index: %w", err)
	}

	// Save function summary
	if err := b.saveGob(filepath.Join(b.indexPath, "functions.gob"), b.functions); err != nil {
		return fmt.Errorf("saving functions: %w", err)
	}

	// Save symbol table
	if err := b.saveGob(filepath.Join(b.indexPath, "symbols.gob"), b.symbols); err != nil {
		return fmt.Errorf("saving symbols: %w", err)
	}

	// Save call graph
	if err := b.saveGob(filepath.Join(b.indexPath, "callgraph.gob"), b.callGraph); err != nil {
		return fmt.Errorf("saving call graph: %w", err)
	}

	// Save stats
	docCount, termCount := b.inverted.GetStats()
	funcCount, _ := b.functions.GetStats()
	symbolCount, _ := b.symbols.GetStats()

	stats := &IndexStats{
		FileCount:      docCount,
		TermCount:      termCount,
		FunctionCount:  funcCount,
		SymbolCount:    symbolCount,
		LastBuild:      time.Now(),
		IndexerVersion: "1.0.0",
	}

	if err := b.saveGob(filepath.Join(b.indexPath, "stats.gob"), stats); err != nil {
		return fmt.Errorf("saving stats: %w", err)
	}

	return nil
}

// Load loads the index from disk.
func (b *IndexBuilder) Load() error {
	// Load inverted index
	if err := b.loadGob(filepath.Join(b.indexPath, "inverted.gob"), b.inverted); err != nil {
		return fmt.Errorf("loading inverted index: %w", err)
	}

	// Load function summary
	if err := b.loadGob(filepath.Join(b.indexPath, "functions.gob"), b.functions); err != nil {
		return fmt.Errorf("loading functions: %w", err)
	}

	// Load symbol table
	if err := b.loadGob(filepath.Join(b.indexPath, "symbols.gob"), b.symbols); err != nil {
		return fmt.Errorf("loading symbols: %w", err)
	}

	// Load call graph
	if err := b.loadGob(filepath.Join(b.indexPath, "callgraph.gob"), b.callGraph); err != nil {
		return fmt.Errorf("loading call graph: %w", err)
	}

	return nil
}

func (b *IndexBuilder) saveGob(path string, data interface{}) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	return gob.NewEncoder(file).Encode(data)
}

func (b *IndexBuilder) loadGob(path string, data interface{}) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	return gob.NewDecoder(file).Decode(data)
}

func (b *IndexBuilder) printStats() {
	docCount, termCount := b.inverted.GetStats()
	funcCount, fileCount := b.functions.GetStats()
	symCount, _ := b.symbols.GetStats()
	nodeCount, edgeCount := b.callGraph.GetStats()

	fmt.Println("Index Statistics:")
	fmt.Printf("  Documents: %d\n", docCount)
	fmt.Printf("  Terms: %d\n", termCount)
	fmt.Printf("  Functions: %d in %d files\n", funcCount, fileCount)
	fmt.Printf("  Symbols: %d\n", symCount)
	fmt.Printf("  Call Graph: %d nodes, %d edges\n", nodeCount, edgeCount)
}

// GetInvertedIndex returns the inverted index.
func (b *IndexBuilder) GetInvertedIndex() *InvertedIndex {
	return b.inverted
}

// GetFunctionSummaryDB returns the function summary database.
func (b *IndexBuilder) GetFunctionSummaryDB() *FunctionSummaryDB {
	return b.functions
}

// GetSymbolTable returns the symbol table.
func (b *IndexBuilder) GetSymbolTable() *SymbolTable {
	return b.symbols
}

// GetCallGraph returns the call graph.
func (b *IndexBuilder) GetCallGraph() *CallGraph {
	return b.callGraph
}

// GetIndex returns a complete Index structure for persistence.
func (b *IndexBuilder) GetIndex() *Index {
	docCount, _ := b.inverted.GetStats()

	return &Index{
		Meta: &IndexMeta{
			Version:        "1.0.0",
			SourceHash:     "",
			CreatedAt:      time.Now(),
			LastUpdated:    time.Now(),
			FileCount:      docCount,
			TotalSize:      0,
			IndexerVersion: IndexerVersion,
		},
		InvertedIndex:     make(map[string]*PostingList),
		FunctionSummaries: make(map[string]*FunctionSummary),
		SymbolTable:       make(map[string]*Symbol),
		CallGraph:         NewCallGraph(),
		FileHashes:        make(map[string]string),
	}
}

// GenerateCTags generates ctags for the source code.
func GenerateCTags(ctx context.Context, sourcePath, outputPath string) error {
	args := []string{
		"-R",
		"--fields=+Kn",
		"--c-kinds=+pxdm",
		"--languages=C,C++",
		"-f", outputPath,
		sourcePath,
	}

	cmd := exec.CommandContext(ctx, "ctags", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ctags error: %w, output: %s", err, string(output))
	}

	return nil
}

// GenerateCscope generates cscope database.
func GenerateCscope(ctx context.Context, sourcePath, dbPath string) error {
	// Find all C files
	var files []string
	err := filepath.Walk(sourcePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			ext := filepath.Ext(path)
			if ext == ".c" || ext == ".h" {
				files = append(files, path)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	// Write file list
	listFile := filepath.Join(filepath.Dir(dbPath), "cscope.files")
	f, err := os.Create(listFile)
	if err != nil {
		return err
	}
	defer f.Close()

	for _, file := range files {
		fmt.Fprintln(f, file)
	}

	// Build cscope database
	cmd := exec.CommandContext(ctx, "cscope", "-b", "-k", "-i", listFile, "-f", dbPath)
	cmd.Dir = sourcePath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("cscope error: %w, output: %s", err, string(output))
	}

	return nil
}

// SimpleIndexManager manages the complete index (used by agent).
type SimpleIndexManager struct {
	sourcePath string
	indexPath  string

	inverted  *InvertedIndex
	functions *FunctionSummaryDB
	symbols   *SymbolTable
	callGraph *CallGraph
	cscope    *CscopeClient
}

// NewSimpleIndexManager creates a new simple index manager.
func NewSimpleIndexManager(sourcePath, indexPath string) *SimpleIndexManager {
	cscopeDB := filepath.Join(indexPath, "cscope.out")
	return &SimpleIndexManager{
		sourcePath: sourcePath,
		indexPath:  indexPath,
		inverted:   NewInvertedIndex(),
		functions:  NewFunctionSummaryDB(),
		symbols:    NewSymbolTable(),
		callGraph:  NewCallGraph(),
		cscope:     NewCscopeClient("cscope", sourcePath, cscopeDB),
	}
}

// Initialize initializes the index manager.
func (m *SimpleIndexManager) Initialize(ctx context.Context) error {
	// Try to load existing index
	builder := NewIndexBuilder(m.sourcePath, m.indexPath, 4)
	if err := builder.Load(); err == nil {
		m.inverted = builder.GetInvertedIndex()
		m.functions = builder.GetFunctionSummaryDB()
		m.symbols = builder.GetSymbolTable()
		m.callGraph = builder.GetCallGraph()
		return nil
	}

	// Build new index
	return builder.Build(ctx)
}

// Search searches the index.
func (m *SimpleIndexManager) Search(keywords []string, operator string, limit int) []*SearchResult {
	return m.inverted.Search(keywords, operator, limit)
}

// GetFunction returns function information.
func (m *SimpleIndexManager) GetFunction(name string) []*FunctionInfo {
	return m.functions.GetFunction(name)
}

// GetSymbol returns symbol information.
func (m *SimpleIndexManager) GetSymbol(name string) []*Symbol {
	return m.symbols.GetSymbol(name)
}

// GetCallChain returns the call chain for a function.
func (m *SimpleIndexManager) GetCallChain(function string, direction string, maxDepth int) *CallChainNode {
	return m.callGraph.GetCallChain(function, direction, maxDepth)
}

// GetSourcePath returns the source path.
func (m *SimpleIndexManager) GetSourcePath() string {
	return m.sourcePath
}

// ReadFile reads a source file.
func (m *SimpleIndexManager) ReadFile(relPath string, startLine, endLine int) ([]string, error) {
	fullPath := filepath.Join(m.sourcePath, relPath)

	// Clean the path
	if strings.HasPrefix(relPath, "/") {
		fullPath = relPath
	}

	file, err := os.Open(fullPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(content), "\n")

	if startLine <= 0 {
		startLine = 1
	}
	if endLine <= 0 || endLine > len(lines) {
		endLine = len(lines)
	}

	if startLine > len(lines) {
		return []string{}, nil
	}

	return lines[startLine-1 : endLine], nil
}
