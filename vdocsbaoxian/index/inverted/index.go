// Package inverted provides inverted index implementation.
package inverted

import (
	"context"
	"crypto/md5"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
	_ "github.com/mattn/go-sqlite3"
)

// DocType represents the type of document.
type DocType string

const (
	DocTypeLegal   DocType = "legal"   // 法律法规
	DocTypeProduct DocType = "product" // 产品条款
	DocTypeClaim   DocType = "claim"   // 理赔案例
	DocTypeHealth  DocType = "health"  // 健康数据
	DocTypeWeb     DocType = "web"     // 网页内容
)

// IndexRecord represents an index record.
type IndexRecord struct {
	Term       string     `json:"term"`
	DocID      string     `json:"doc_id"`
	DocType    DocType    `json:"doc_type"`
	Positions  []Position `json:"positions"`
	TF         float64    `json:"tf"`
	IDF        float64    `json:"idf"`
	Timestamp  time.Time  `json:"timestamp"`
	ValidUntil *time.Time `json:"valid_until,omitempty"`
}

// Position represents a position in the document.
type Position struct {
	Section   int    `json:"section"`
	Paragraph int    `json:"paragraph"`
	Sentence  int    `json:"sentence"`
	Offset    int    `json:"offset"`
	Length    int    `json:"length"`
	Context   string `json:"context,omitempty"`
}

// SearchOptions represents search options.
type SearchOptions struct {
	DocType    DocType
	ValidOnly  bool
	MaxResults int
	MinScore   float64
	CacheTTL   time.Duration
}

// SearchResult represents a search result.
type SearchResult struct {
	Query   string
	Results []*DocScore
	Total   int
	Took    time.Duration
}

// DocScore represents a document score.
type DocScore struct {
	DocID        string
	DocType      DocType
	Score        float64
	MatchedTerms []string
	Positions    []Position
	Highlights   []string
}

// IndexStats represents index statistics.
type IndexStats struct {
	TotalDocs     int64
	TotalTerms    int64
	AvgDocLength  float64
	LastUpdated   time.Time
	CacheHits     int64
	CacheMisses   int64
}

// InvertedIndex represents the inverted index.
type InvertedIndex struct {
	db        *sql.DB
	tokenizer Tokenizer
	cache     *lru.Cache[string, *SearchResult]
	stats     *IndexStats
	mu        sync.RWMutex
	
	// BM25 parameters
	k1        float64
	b         float64
	avgDocLen float64
	docCount  int64
}

// Tokenizer defines the interface for tokenization.
type Tokenizer interface {
	Tokenize(text string) []Token
}

// Token represents a token.
type Token struct {
	Term           string
	Offset         int
	Length         int
	IsProfessional bool
}

// Config represents index configuration.
type Config struct {
	DBPath      string
	CacheSize   int
	K1          float64
	B           float64
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	return &Config{
		DBPath:    "index.db",
		CacheSize: 10000,
		K1:        1.2,
		B:         0.75,
	}
}

// NewInvertedIndex creates a new inverted index.
func NewInvertedIndex(cfg *Config, tokenizer Tokenizer) (*InvertedIndex, error) {
	db, err := sql.Open("sqlite3", cfg.DBPath+"?_journal_mode=WAL&_synchronous=NORMAL")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	cache, err := lru.New[string, *SearchResult](cfg.CacheSize)
	if err != nil {
		return nil, fmt.Errorf("failed to create cache: %w", err)
	}

	idx := &InvertedIndex{
		db:        db,
		tokenizer: tokenizer,
		cache:     cache,
		stats:     &IndexStats{},
		k1:        cfg.K1,
		b:         cfg.B,
	}

	if err := idx.initSchema(); err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	if err := idx.loadStats(); err != nil {
		return nil, fmt.Errorf("failed to load stats: %w", err)
	}

	return idx, nil
}

// initSchema initializes the database schema.
func (idx *InvertedIndex) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS inverted_index (
		term TEXT NOT NULL,
		doc_id TEXT NOT NULL,
		doc_type TEXT NOT NULL,
		positions TEXT,
		tf REAL,
		idf REAL,
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
		valid_until DATETIME,
		PRIMARY KEY (term, doc_id)
	);
	
	CREATE INDEX IF NOT EXISTS idx_term ON inverted_index(term);
	CREATE INDEX IF NOT EXISTS idx_doc_id ON inverted_index(doc_id);
	CREATE INDEX IF NOT EXISTS idx_doc_type ON inverted_index(doc_type);
	CREATE INDEX IF NOT EXISTS idx_valid ON inverted_index(valid_until);
	
	CREATE TABLE IF NOT EXISTS documents (
		doc_id TEXT PRIMARY KEY,
		doc_type TEXT NOT NULL,
		title TEXT,
		content_length INTEGER,
		term_count INTEGER,
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
		valid_until DATETIME,
		metadata TEXT
	);
	
	CREATE TABLE IF NOT EXISTS index_stats (
		key TEXT PRIMARY KEY,
		value TEXT,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	`
	
	_, err := idx.db.Exec(schema)
	return err
}

// loadStats loads index statistics.
func (idx *InvertedIndex) loadStats() error {
	var count int64
	err := idx.db.QueryRow("SELECT COUNT(DISTINCT doc_id) FROM documents").Scan(&count)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	idx.docCount = count

	var avgLen float64
	err = idx.db.QueryRow("SELECT AVG(content_length) FROM documents").Scan(&avgLen)
	if err != nil && err != sql.ErrNoRows {
		avgLen = 1000 // default
	}
	idx.avgDocLen = avgLen

	return nil
}

// Add adds a document to the index.
func (idx *InvertedIndex) Add(ctx context.Context, doc *Document) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	// Tokenize content
	tokens := idx.tokenizer.Tokenize(doc.Content)
	if len(tokens) == 0 {
		return nil
	}

	// Calculate term frequencies
	termFreq := make(map[string]int)
	termPositions := make(map[string][]Position)

	for i, token := range tokens {
		termFreq[token.Term]++
		termPositions[token.Term] = append(termPositions[token.Term], Position{
			Offset: token.Offset,
			Length: token.Length,
		})
		_ = i
	}

	// Begin transaction
	tx, err := idx.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Insert document
	_, err = tx.ExecContext(ctx, `
		INSERT OR REPLACE INTO documents (doc_id, doc_type, title, content_length, term_count, valid_until, metadata)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, doc.ID, doc.Type, doc.Title, len(doc.Content), len(tokens), doc.ValidUntil, "")
	if err != nil {
		return err
	}

	// Insert index records
	stmt, err := tx.PrepareContext(ctx, `
		INSERT OR REPLACE INTO inverted_index (term, doc_id, doc_type, positions, tf, valid_until)
		VALUES (?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	totalTerms := len(tokens)
	for term, freq := range termFreq {
		positions, _ := json.Marshal(termPositions[term])
		tf := float64(freq) / float64(totalTerms)

		_, err = stmt.ExecContext(ctx, term, doc.ID, doc.Type, string(positions), tf, doc.ValidUntil)
		if err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	// Invalidate cache
	idx.cache.Purge()

	// Update stats
	idx.docCount++
	idx.stats.TotalDocs = idx.docCount
	idx.stats.LastUpdated = time.Now()

	return nil
}

// Search searches the index.
func (idx *InvertedIndex) Search(ctx context.Context, query string, opts SearchOptions) (*SearchResult, error) {
	start := time.Now()

	// Check cache
	cacheKey := idx.buildCacheKey(query, opts)
	if result, ok := idx.cache.Get(cacheKey); ok {
		idx.stats.CacheHits++
		return result, nil
	}
	idx.stats.CacheMisses++

	// Tokenize query
	tokens := idx.tokenizer.Tokenize(query)
	if len(tokens) == 0 {
		return &SearchResult{Query: query}, nil
	}

	// Search for each term
	docScores := make(map[string]*DocScore)

	for _, token := range tokens {
		records, err := idx.getRecords(ctx, token.Term, opts)
		if err != nil {
			return nil, err
		}

		for _, record := range records {
			// Apply validity filter
			if opts.ValidOnly && record.ValidUntil != nil && record.ValidUntil.Before(time.Now()) {
				continue
			}

			// Apply type filter
			if opts.DocType != "" && record.DocType != opts.DocType {
				continue
			}

			// Calculate BM25 score
			score := idx.calculateBM25(record, len(tokens))

			// Boost for professional terms
			if token.IsProfessional {
				score *= 1.5
			}

			if existing, ok := docScores[record.DocID]; ok {
				existing.Score += score
				existing.MatchedTerms = append(existing.MatchedTerms, token.Term)
				existing.Positions = append(existing.Positions, record.Positions...)
			} else {
				docScores[record.DocID] = &DocScore{
					DocID:        record.DocID,
					DocType:      record.DocType,
					Score:        score,
					MatchedTerms: []string{token.Term},
					Positions:    record.Positions,
				}
			}
		}
	}

	// Sort by score
	results := make([]*DocScore, 0, len(docScores))
	for _, ds := range docScores {
		if opts.MinScore > 0 && ds.Score < opts.MinScore {
			continue
		}
		results = append(results, ds)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	// Limit results
	if opts.MaxResults > 0 && len(results) > opts.MaxResults {
		results = results[:opts.MaxResults]
	}

	result := &SearchResult{
		Query:   query,
		Results: results,
		Total:   len(docScores),
		Took:    time.Since(start),
	}

	// Cache result
	if opts.CacheTTL > 0 {
		idx.cache.Add(cacheKey, result)
	}

	return result, nil
}

// getRecords retrieves index records for a term.
func (idx *InvertedIndex) getRecords(ctx context.Context, term string, opts SearchOptions) ([]*IndexRecord, error) {
	query := "SELECT term, doc_id, doc_type, positions, tf, idf, timestamp, valid_until FROM inverted_index WHERE term = ?"
	args := []interface{}{term}

	if opts.DocType != "" {
		query += " AND doc_type = ?"
		args = append(args, opts.DocType)
	}

	rows, err := idx.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []*IndexRecord
	for rows.Next() {
		var record IndexRecord
		var positionsJSON string
		var validUntil sql.NullTime

		err := rows.Scan(
			&record.Term,
			&record.DocID,
			&record.DocType,
			&positionsJSON,
			&record.TF,
			&record.IDF,
			&record.Timestamp,
			&validUntil,
		)
		if err != nil {
			return nil, err
		}

		if positionsJSON != "" {
			json.Unmarshal([]byte(positionsJSON), &record.Positions)
		}

		if validUntil.Valid {
			record.ValidUntil = &validUntil.Time
		}

		records = append(records, &record)
	}

	return records, nil
}

// calculateBM25 calculates BM25 score.
func (idx *InvertedIndex) calculateBM25(record *IndexRecord, queryLen int) float64 {
	tf := record.TF
	idf := record.IDF

	// If IDF is not set, calculate a simple version
	if idf == 0 {
		idf = math.Log((float64(idx.docCount) + 1) / 2)
	}

	// BM25 formula
	score := idf * (tf * (idx.k1 + 1)) / (tf + idx.k1*(1-idx.b+idx.b*(float64(queryLen)/idx.avgDocLen)))

	return score
}

// buildCacheKey builds a cache key.
func (idx *InvertedIndex) buildCacheKey(query string, opts SearchOptions) string {
	h := md5.New()
	h.Write([]byte(query))
	h.Write([]byte(string(opts.DocType)))
	if opts.ValidOnly {
		h.Write([]byte("valid"))
	}
	return hex.EncodeToString(h.Sum(nil))
}

// Delete deletes a document from the index.
func (idx *InvertedIndex) Delete(ctx context.Context, docID string) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	tx, err := idx.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, "DELETE FROM inverted_index WHERE doc_id = ?", docID)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, "DELETE FROM documents WHERE doc_id = ?", docID)
	if err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	idx.cache.Purge()
	idx.docCount--

	return nil
}

// UpdateIDF updates IDF values for all terms.
func (idx *InvertedIndex) UpdateIDF(ctx context.Context) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	// Get document counts per term
	rows, err := idx.db.QueryContext(ctx, `
		SELECT term, COUNT(DISTINCT doc_id) as doc_freq
		FROM inverted_index
		GROUP BY term
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	tx, err := idx.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, "UPDATE inverted_index SET idf = ? WHERE term = ?")
	if err != nil {
		return err
	}
	defer stmt.Close()

	for rows.Next() {
		var term string
		var docFreq int64
		if err := rows.Scan(&term, &docFreq); err != nil {
			return err
		}

		// Calculate IDF: log((N - n + 0.5) / (n + 0.5))
		idf := math.Log((float64(idx.docCount) - float64(docFreq) + 0.5) / (float64(docFreq) + 0.5))
		if idf < 0 {
			idf = 0
		}

		_, err = stmt.ExecContext(ctx, idf, term)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// GetStats returns index statistics.
func (idx *InvertedIndex) GetStats() *IndexStats {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	
	stats := *idx.stats
	stats.TotalDocs = idx.docCount
	return &stats
}

// Close closes the index.
func (idx *InvertedIndex) Close() error {
	return idx.db.Close()
}

// Document represents a document to be indexed.
type Document struct {
	ID         string
	Type       DocType
	Title      string
	Content    string
	ValidUntil *time.Time
	Metadata   map[string]string
}
