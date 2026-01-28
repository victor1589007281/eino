// Package index provides indexing functionality for MySQL source code.
package index

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"

	"github.com/cloudwego/eino/vdocsMysql/config"
	"github.com/cloudwego/eino/vdocsMysql/storage"
)

// Indexer provides source code indexing functionality.
type Indexer struct {
	config       *config.IndexConfig
	storage      *storage.SQLiteStorage
	graphStorage *storage.BoltDBStorage
	sourcePath   string
	
	// Tokenizer settings
	minTokenLen int
	maxTokenLen int
	stopWords   map[string]bool
	
	// Progress tracking
	totalFiles     int64
	processedFiles int64
	lastIndexTime  time.Time
	
	// Concurrency control
	workerPool chan struct{}
	mu         sync.RWMutex
}

// IndexerConfig contains configuration for the indexer.
type IndexerConfig struct {
	Config       *config.IndexConfig
	Storage      *storage.SQLiteStorage
	GraphStorage *storage.BoltDBStorage
	SourcePath   string
}

// NewIndexer creates a new indexer instance.
func NewIndexer(cfg *IndexerConfig) *Indexer {
	stopWords := map[string]bool{
		"if": true, "else": true, "for": true, "while": true, "do": true,
		"switch": true, "case": true, "break": true, "continue": true, "return": true,
		"void": true, "int": true, "char": true, "bool": true, "float": true, "double": true,
		"const": true, "static": true, "extern": true, "inline": true,
		"struct": true, "class": true, "enum": true, "union": true,
		"public": true, "private": true, "protected": true,
		"true": true, "false": true, "null": true, "nullptr": true,
		"sizeof": true, "typeof": true, "this": true,
	}

	workers := cfg.Config.ParallelWorkers
	if workers <= 0 {
		workers = 8
	}

	return &Indexer{
		config:       cfg.Config,
		storage:      cfg.Storage,
		graphStorage: cfg.GraphStorage,
		sourcePath:   cfg.SourcePath,
		minTokenLen:  2,
		maxTokenLen:  64,
		stopWords:    stopWords,
		workerPool:   make(chan struct{}, workers),
	}
}

// IndexResult contains the result of an indexing operation.
type IndexResult struct {
	TotalFiles     int
	IndexedFiles   int
	UpdatedFiles   int
	DeletedFiles   int
	TotalFunctions int
	Duration       time.Duration
	Errors         []error
}

// BuildIndex builds or rebuilds the complete index.
func (idx *Indexer) BuildIndex(ctx context.Context) (*IndexResult, error) {
	startTime := time.Now()
	result := &IndexResult{}

	// Get existing document hashes for incremental update
	existingHashes, err := idx.storage.GetAllDocumentHashes(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get existing hashes: %w", err)
	}

	// Scan source directory
	files, err := idx.scanSourceFiles(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to scan source files: %w", err)
	}
	result.TotalFiles = len(files)
	atomic.StoreInt64(&idx.totalFiles, int64(len(files)))

	// Categorize files
	toIndex := make([]string, 0)
	toUpdate := make([]string, 0)
	processedPaths := make(map[string]bool)

	for _, file := range files {
		relPath, _ := filepath.Rel(idx.sourcePath, file)
		processedPaths[relPath] = true

		hash, err := idx.computeFileHash(file)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("hash error for %s: %w", file, err))
			continue
		}

		if existingHash, exists := existingHashes[relPath]; exists {
			if existingHash != hash {
				toUpdate = append(toUpdate, file)
			}
		} else {
			toIndex = append(toIndex, file)
		}
	}

	// Find deleted files
	toDelete := make([]string, 0)
	for path := range existingHashes {
		if !processedPaths[path] {
			toDelete = append(toDelete, path)
		}
	}

	// Process new files
	if len(toIndex) > 0 {
		indexed, errs := idx.indexFiles(ctx, toIndex)
		result.IndexedFiles = indexed
		result.Errors = append(result.Errors, errs...)
	}

	// Process updated files
	if len(toUpdate) > 0 {
		updated, errs := idx.updateFiles(ctx, toUpdate)
		result.UpdatedFiles = updated
		result.Errors = append(result.Errors, errs...)
	}

	// Process deleted files
	if len(toDelete) > 0 {
		deleted, errs := idx.deleteFiles(ctx, toDelete)
		result.DeletedFiles = deleted
		result.Errors = append(result.Errors, errs...)
	}

	// Get function count
	result.TotalFunctions, _ = idx.storage.GetFunctionCount(ctx)

	result.Duration = time.Since(startTime)
	idx.lastIndexTime = time.Now()

	return result, nil
}

// scanSourceFiles scans the source directory for indexable files.
func (idx *Indexer) scanSourceFiles(ctx context.Context) ([]string, error) {
	var files []string

	includePatterns := []string{"*.cc", "*.cpp", "*.h", "*.hpp", "*.c"}
	excludePatterns := []string{"unittest", "test", "mysql-test", "boost", "extra"}

	err := filepath.Walk(idx.sourcePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if info.IsDir() {
			// Skip excluded directories
			for _, pattern := range excludePatterns {
				if strings.Contains(path, pattern) {
					return filepath.SkipDir
				}
			}
			return nil
		}

		// Check file extension
		ext := strings.ToLower(filepath.Ext(path))
		matched := false
		for _, pattern := range includePatterns {
			if strings.HasSuffix(pattern, ext) {
				matched = true
				break
			}
		}

		if matched {
			files = append(files, path)
		}

		return nil
	})

	return files, err
}

// computeFileHash computes SHA256 hash of a file.
func (idx *Indexer) computeFileHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// indexFiles indexes a batch of new files.
func (idx *Indexer) indexFiles(ctx context.Context, files []string) (int, []error) {
	var wg sync.WaitGroup
	var mu sync.Mutex
	var indexed int
	var errors []error

	for _, file := range files {
		select {
		case <-ctx.Done():
			return indexed, append(errors, ctx.Err())
		case idx.workerPool <- struct{}{}:
		}

		wg.Add(1)
		go func(f string) {
			defer wg.Done()
			defer func() { <-idx.workerPool }()

			if err := idx.indexFile(ctx, f); err != nil {
				mu.Lock()
				errors = append(errors, fmt.Errorf("index error for %s: %w", f, err))
				mu.Unlock()
			} else {
				mu.Lock()
				indexed++
				mu.Unlock()
			}

			atomic.AddInt64(&idx.processedFiles, 1)
		}(file)
	}

	wg.Wait()
	return indexed, errors
}

// indexFile indexes a single file.
func (idx *Indexer) indexFile(ctx context.Context, path string) error {
	relPath, err := filepath.Rel(idx.sourcePath, path)
	if err != nil {
		relPath = path
	}

	// Read file
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	// Compute hash
	hash := sha256.Sum256(content)
	hashStr := hex.EncodeToString(hash[:])

	// Count lines
	lineCount := strings.Count(string(content), "\n") + 1

	// Detect module
	module := idx.detectModule(relPath)

	// Get file info
	fileInfo, _ := os.Stat(path)
	fileSize := int64(0)
	if fileInfo != nil {
		fileSize = fileInfo.Size()
	}

	// Create document record
	doc := &storage.Document{
		FilePath:    relPath,
		FileHash:    hashStr,
		FileSize:    fileSize,
		LineCount:   lineCount,
		Module:      module,
		Language:    idx.detectLanguage(path),
		LastIndexed: time.Now(),
	}

	if err := idx.storage.CreateDocument(ctx, doc); err != nil {
		return fmt.Errorf("failed to create document: %w", err)
	}

	// Tokenize and index content
	tokens := idx.tokenize(string(content))
	termFreq := make(map[string]*termInfo)

	for _, token := range tokens {
		if info, exists := termFreq[token.term]; exists {
			info.frequency++
			info.positions = append(info.positions, storage.Position{
				Line:   token.line,
				Column: token.column,
				Length: len(token.term),
			})
		} else {
			termFreq[token.term] = &termInfo{
				frequency: 1,
				positions: []storage.Position{{
					Line:   token.line,
					Column: token.column,
					Length: len(token.term),
				}},
			}
		}
	}

	// Batch insert term occurrences
	occurrences := make([]*storage.TermOccurrence, 0, len(termFreq))
	for term, info := range termFreq {
		occurrences = append(occurrences, &storage.TermOccurrence{
			Term:      term,
			DocID:     doc.ID,
			Frequency: info.frequency,
			Positions: info.positions,
		})
	}

	if err := idx.storage.BatchAddTermOccurrences(ctx, occurrences); err != nil {
		return fmt.Errorf("failed to add term occurrences: %w", err)
	}

	// Extract and index functions
	functions := idx.extractFunctions(string(content), relPath, module)
	if len(functions) > 0 {
		if err := idx.storage.BatchCreateFunctions(ctx, functions); err != nil {
			return fmt.Errorf("failed to create functions: %w", err)
		}
	}

	return nil
}

type termInfo struct {
	frequency int
	positions []storage.Position
}

type tokenWithPosition struct {
	term   string
	line   int
	column int
}

// tokenize tokenizes the content into searchable terms.
func (idx *Indexer) tokenize(content string) []tokenWithPosition {
	var tokens []tokenWithPosition
	lines := strings.Split(content, "\n")

	// Regex patterns for identifiers
	identifierRe := regexp.MustCompile(`[a-zA-Z_][a-zA-Z0-9_]*`)

	for lineNum, line := range lines {
		// Remove comments
		line = removeComments(line)

		// Find all identifiers
		matches := identifierRe.FindAllStringIndex(line, -1)
		for _, match := range matches {
			term := line[match[0]:match[1]]
			
			// Filter
			if len(term) < idx.minTokenLen || len(term) > idx.maxTokenLen {
				continue
			}
			if idx.stopWords[strings.ToLower(term)] {
				continue
			}
			if isAllDigits(term) {
				continue
			}

			tokens = append(tokens, tokenWithPosition{
				term:   term,
				line:   lineNum + 1,
				column: match[0],
			})

			// Also add camelCase/snake_case parts
			parts := splitIdentifier(term)
			for _, part := range parts {
				if len(part) >= idx.minTokenLen && !idx.stopWords[strings.ToLower(part)] {
					tokens = append(tokens, tokenWithPosition{
						term:   strings.ToLower(part),
						line:   lineNum + 1,
						column: match[0],
					})
				}
			}
		}
	}

	return tokens
}

// extractFunctions extracts function definitions from source code.
func (idx *Indexer) extractFunctions(content, filePath, module string) []*storage.FunctionSummary {
	var functions []*storage.FunctionSummary

	// Regex patterns for function definitions
	// C/C++ function pattern: return_type function_name(params) {
	funcPattern := regexp.MustCompile(`(?m)^[\t ]*(?:(?:static|inline|virtual|explicit|constexpr|const)\s+)*` +
		`([\w:*&<>\s,]+?)\s+` +                           // Return type
		`([\w:]+)\s*` +                                    // Function name
		`\(([\w\s,*&<>:=\[\]]*)\)\s*` +                   // Parameters
		`(?:const\s*)?(?:override\s*)?(?:final\s*)?` +    // Modifiers
		`(?:\s*(?:->|:)[^{]*)?` +                         // Trailing return or initializer
		`\s*\{`)

	lines := strings.Split(content, "\n")
	lineOffsets := make([]int, len(lines)+1)
	offset := 0
	for i, line := range lines {
		lineOffsets[i] = offset
		offset += len(line) + 1
	}
	lineOffsets[len(lines)] = offset

	matches := funcPattern.FindAllStringSubmatchIndex(content, -1)

	for _, match := range matches {
		if len(match) < 8 {
			continue
		}

		returnType := strings.TrimSpace(content[match[2]:match[3]])
		funcName := content[match[4]:match[5]]
		params := ""
		if match[6] >= 0 && match[7] >= 0 {
			params = strings.TrimSpace(content[match[6]:match[7]])
		}

		// Skip if it looks like a control statement
		if isControlStatement(funcName) {
			continue
		}

		// Find line number
		lineStart := findLineNumber(match[0], lineOffsets)
		
		// Find end of function (simple brace matching)
		lineEnd := lineStart
		braceCount := 0
		started := false
		for i := match[0]; i < len(content); i++ {
			if content[i] == '{' {
				braceCount++
				started = true
			} else if content[i] == '}' {
				braceCount--
				if started && braceCount == 0 {
					lineEnd = findLineNumber(i, lineOffsets)
					break
				}
			}
		}

		// Parse parameters
		parameters := parseParameters(params)

		// Generate unique ID
		id := fmt.Sprintf("%s:%s:%d", filePath, funcName, lineStart)

		fn := &storage.FunctionSummary{
			ID:            id,
			Name:          funcName,
			QualifiedName: funcName,
			FilePath:      filePath,
			LineStart:     lineStart,
			LineEnd:       lineEnd,
			Signature:     fmt.Sprintf("%s %s(%s)", returnType, funcName, params),
			ReturnType:    returnType,
			Parameters:    parameters,
			Module:        module,
			Subsystem:     detectSubsystem(filePath),
			LOC:           lineEnd - lineStart + 1,
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		}

		functions = append(functions, fn)
	}

	return functions
}

// updateFiles updates existing indexed files.
func (idx *Indexer) updateFiles(ctx context.Context, files []string) (int, []error) {
	var wg sync.WaitGroup
	var mu sync.Mutex
	var updated int
	var errors []error

	for _, file := range files {
		select {
		case <-ctx.Done():
			return updated, append(errors, ctx.Err())
		case idx.workerPool <- struct{}{}:
		}

		wg.Add(1)
		go func(f string) {
			defer wg.Done()
			defer func() { <-idx.workerPool }()

			relPath, _ := filepath.Rel(idx.sourcePath, f)

			// Delete old index data
			doc, _ := idx.storage.GetDocumentByPath(ctx, relPath)
			if doc != nil {
				idx.storage.DeleteDocumentTerms(ctx, doc.ID)
				idx.storage.DeleteDocument(ctx, doc.ID)
				idx.storage.DeleteFunctionsByFile(ctx, relPath)
			}

			// Re-index
			if err := idx.indexFile(ctx, f); err != nil {
				mu.Lock()
				errors = append(errors, fmt.Errorf("update error for %s: %w", f, err))
				mu.Unlock()
			} else {
				mu.Lock()
				updated++
				mu.Unlock()
			}

			atomic.AddInt64(&idx.processedFiles, 1)
		}(file)
	}

	wg.Wait()
	return updated, errors
}

// deleteFiles removes indexed data for deleted files.
func (idx *Indexer) deleteFiles(ctx context.Context, paths []string) (int, []error) {
	var deleted int
	var errors []error

	for _, path := range paths {
		doc, err := idx.storage.GetDocumentByPath(ctx, path)
		if err != nil {
			errors = append(errors, err)
			continue
		}
		if doc == nil {
			continue
		}

		// Delete term occurrences
		if err := idx.storage.DeleteDocumentTerms(ctx, doc.ID); err != nil {
			errors = append(errors, err)
		}

		// Delete document
		if err := idx.storage.DeleteDocument(ctx, doc.ID); err != nil {
			errors = append(errors, err)
		}

		// Delete functions
		if err := idx.storage.DeleteFunctionsByFile(ctx, path); err != nil {
			errors = append(errors, err)
		}

		deleted++
	}

	return deleted, errors
}

// detectModule detects the module based on file path.
func (idx *Indexer) detectModule(path string) string {
	if strings.Contains(path, "storage/innobase") {
		return "InnoDB"
	}
	if strings.Contains(path, "sql/") {
		return "SQL"
	}
	if strings.Contains(path, "plugin/") {
		return "Plugin"
	}
	if strings.Contains(path, "client/") {
		return "Client"
	}
	if strings.Contains(path, "include/") {
		return "Core"
	}
	if strings.Contains(path, "mysys/") {
		return "MySys"
	}
	return "Other"
}

// detectLanguage detects the programming language.
func (idx *Indexer) detectLanguage(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".c":
		return "C"
	case ".cc", ".cpp", ".cxx":
		return "C++"
	case ".h", ".hpp", ".hxx":
		return "Header"
	default:
		return "Unknown"
	}
}

// Progress returns the current indexing progress.
func (idx *Indexer) Progress() (processed, total int64) {
	return atomic.LoadInt64(&idx.processedFiles), atomic.LoadInt64(&idx.totalFiles)
}

// Helper functions

func removeComments(line string) string {
	// Simple comment removal (doesn't handle all cases)
	if idx := strings.Index(line, "//"); idx >= 0 {
		line = line[:idx]
	}
	return line
}

func isAllDigits(s string) bool {
	for _, r := range s {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

func splitIdentifier(s string) []string {
	// Split camelCase and snake_case
	var parts []string
	var current strings.Builder

	for i, r := range s {
		if r == '_' {
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
		} else if unicode.IsUpper(r) && i > 0 {
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
			current.WriteRune(r)
		} else {
			current.WriteRune(r)
		}
	}

	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	return parts
}

func isControlStatement(name string) bool {
	controls := map[string]bool{
		"if": true, "else": true, "for": true, "while": true,
		"switch": true, "case": true, "do": true,
	}
	return controls[name]
}

func findLineNumber(offset int, lineOffsets []int) int {
	for i := 0; i < len(lineOffsets)-1; i++ {
		if offset >= lineOffsets[i] && offset < lineOffsets[i+1] {
			return i + 1
		}
	}
	return len(lineOffsets)
}

func parseParameters(params string) []storage.Parameter {
	var result []storage.Parameter
	if params == "" {
		return result
	}

	// Split by comma (simplified, doesn't handle templates)
	parts := strings.Split(params, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || part == "void" {
			continue
		}

		// Try to extract type and name
		tokens := strings.Fields(part)
		if len(tokens) == 0 {
			continue
		}

		param := storage.Parameter{}
		if len(tokens) == 1 {
			param.Type = tokens[0]
		} else {
			param.Name = tokens[len(tokens)-1]
			// Remove * and & from name
			param.Name = strings.TrimPrefix(param.Name, "*")
			param.Name = strings.TrimPrefix(param.Name, "&")
			param.Type = strings.Join(tokens[:len(tokens)-1], " ")
		}

		result = append(result, param)
	}

	return result
}

func detectSubsystem(path string) string {
	subsystems := map[string]string{
		"trx":       "Transaction",
		"lock":      "Locking",
		"buf":       "BufferPool",
		"log":       "Logging",
		"fil":       "FileIO",
		"fsp":       "FileSpace",
		"page":      "PageManager",
		"row":       "Row",
		"dict":      "DataDict",
		"btr":       "BTree",
		"sql_parse": "Parser",
		"sql_lex":   "Lexer",
		"opt":       "Optimizer",
		"join":      "JoinExecutor",
		"handler":   "Handler",
	}

	for pattern, name := range subsystems {
		if strings.Contains(path, pattern) {
			return name
		}
	}

	return "General"
}

// Search provides search functionality over the index.

// SearchQuery represents a search query.
type SearchQuery struct {
	Query       string
	FileTypes   []string
	Directories []string
	Limit       int
}

// SearchResult represents a search result.
type SearchResult struct {
	FilePath  string
	Line      int
	Content   string
	Score     float64
	Snippet   string
	Function  string
	Module    string
}

// Search searches the index.
func (idx *Indexer) Search(ctx context.Context, query *SearchQuery) ([]*SearchResult, error) {
	// Tokenize query
	tokens := make([]string, 0)
	for _, token := range idx.tokenize(query.Query) {
		tokens = append(tokens, token.term)
	}

	if len(tokens) == 0 {
		return nil, nil
	}

	// Search using storage
	termQuery := &storage.TermSearchQuery{
		Terms:       tokens,
		Operator:    storage.SearchOperatorOR,
		FileTypes:   query.FileTypes,
		Directories: query.Directories,
		Limit:       query.Limit,
	}

	storageResults, err := idx.storage.SearchTerms(ctx, termQuery)
	if err != nil {
		return nil, err
	}

	// Convert to SearchResult
	results := make([]*SearchResult, 0, len(storageResults))
	for _, sr := range storageResults {
		doc, _ := idx.storage.GetDocument(ctx, sr.DocID)
		
		result := &SearchResult{
			FilePath: sr.FilePath,
			Score:    sr.Score,
		}
		
		if doc != nil {
			result.Module = doc.Module
		}

		// Get snippet from file
		if len(sr.Positions) > 0 {
			result.Line = sr.Positions[0].Line
			result.Snippet = idx.getSnippet(filepath.Join(idx.sourcePath, sr.FilePath), sr.Positions[0].Line)
		}

		results = append(results, result)
	}

	return results, nil
}

// getSnippet retrieves a code snippet around the given line.
func (idx *Indexer) getSnippet(path string, line int) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	lineNum := 0
	var lines []string

	start := line - 2
	end := line + 2
	if start < 1 {
		start = 1
	}

	for scanner.Scan() {
		lineNum++
		if lineNum >= start && lineNum <= end {
			lines = append(lines, scanner.Text())
		}
		if lineNum > end {
			break
		}
	}

	return strings.Join(lines, "\n")
}

// SearchFunctions searches for functions by name or pattern.
func (idx *Indexer) SearchFunctions(ctx context.Context, query string, limit int) ([]*storage.FunctionSummary, error) {
	return idx.storage.SearchFunctions(ctx, &storage.FunctionSearchQuery{
		Query: query,
		Limit: limit,
	})
}

// GetFunctionByName gets functions by exact name.
func (idx *Indexer) GetFunctionByName(ctx context.Context, name string) ([]*storage.FunctionSummary, error) {
	return idx.storage.GetFunctionByName(ctx, name)
}
