package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// SQLiteStorage implements Storage interface using SQLite.
type SQLiteStorage struct {
	db     *sql.DB
	dbPath string
}

// SQLiteConfig contains SQLite configuration.
type SQLiteConfig struct {
	Path            string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	JournalMode     string
	SynchronousMode string
}

// NewSQLiteStorage creates a new SQLite storage instance.
func NewSQLiteStorage(config *SQLiteConfig) (*SQLiteStorage, error) {
	dsn := fmt.Sprintf("%s?_journal_mode=%s&_synchronous=%s&_foreign_keys=ON",
		config.Path, config.JournalMode, config.SynchronousMode)

	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if config.MaxOpenConns > 0 {
		db.SetMaxOpenConns(config.MaxOpenConns)
	}
	if config.MaxIdleConns > 0 {
		db.SetMaxIdleConns(config.MaxIdleConns)
	}
	if config.ConnMaxLifetime > 0 {
		db.SetConnMaxLifetime(config.ConnMaxLifetime)
	}

	return &SQLiteStorage{
		db:     db,
		dbPath: config.Path,
	}, nil
}

// Init initializes the database schema.
func (s *SQLiteStorage) Init(ctx context.Context) error {
	schema := `
	-- Documents table
	CREATE TABLE IF NOT EXISTS documents (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		file_path TEXT UNIQUE NOT NULL,
		file_hash TEXT NOT NULL,
		file_size INTEGER,
		line_count INTEGER,
		module TEXT,
		language TEXT,
		last_indexed TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		metadata TEXT
	);
	CREATE INDEX IF NOT EXISTS idx_documents_module ON documents(module);
	CREATE INDEX IF NOT EXISTS idx_documents_hash ON documents(file_hash);

	-- Inverted index table
	CREATE TABLE IF NOT EXISTS inverted_index (
		term TEXT NOT NULL,
		doc_id INTEGER NOT NULL,
		frequency INTEGER DEFAULT 1,
		positions TEXT,
		PRIMARY KEY (term, doc_id),
		FOREIGN KEY (doc_id) REFERENCES documents(id) ON DELETE CASCADE
	);
	CREATE INDEX IF NOT EXISTS idx_inverted_term ON inverted_index(term);
	CREATE INDEX IF NOT EXISTS idx_inverted_doc ON inverted_index(doc_id);

	-- N-gram index for substring search
	CREATE TABLE IF NOT EXISTS ngram_index (
		ngram TEXT NOT NULL,
		term TEXT NOT NULL,
		PRIMARY KEY (ngram, term)
	);
	CREATE INDEX IF NOT EXISTS idx_ngram ON ngram_index(ngram);

	-- Function summaries table
	CREATE TABLE IF NOT EXISTS function_summaries (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		qualified_name TEXT,
		file_path TEXT NOT NULL,
		line_start INTEGER,
		line_end INTEGER,
		signature TEXT,
		return_type TEXT,
		parameters TEXT,
		description TEXT,
		tags TEXT,
		module TEXT,
		subsystem TEXT,
		loc INTEGER,
		complexity INTEGER,
		callers TEXT,
		callees TEXT,
		stats TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_functions_name ON function_summaries(name);
	CREATE INDEX IF NOT EXISTS idx_functions_file ON function_summaries(file_path);
	CREATE INDEX IF NOT EXISTS idx_functions_module ON function_summaries(module);

	-- Function FTS for full-text search
	CREATE VIRTUAL TABLE IF NOT EXISTS function_fts USING fts5(
		name, qualified_name, signature, description, tags,
		content='function_summaries',
		content_rowid='rowid'
	);

	-- Triggers for FTS sync
	CREATE TRIGGER IF NOT EXISTS function_fts_insert AFTER INSERT ON function_summaries BEGIN
		INSERT INTO function_fts(rowid, name, qualified_name, signature, description, tags)
		VALUES (NEW.rowid, NEW.name, NEW.qualified_name, NEW.signature, NEW.description, NEW.tags);
	END;
	
	CREATE TRIGGER IF NOT EXISTS function_fts_delete AFTER DELETE ON function_summaries BEGIN
		INSERT INTO function_fts(function_fts, rowid, name, qualified_name, signature, description, tags)
		VALUES ('delete', OLD.rowid, OLD.name, OLD.qualified_name, OLD.signature, OLD.description, OLD.tags);
	END;
	
	CREATE TRIGGER IF NOT EXISTS function_fts_update AFTER UPDATE ON function_summaries BEGIN
		INSERT INTO function_fts(function_fts, rowid, name, qualified_name, signature, description, tags)
		VALUES ('delete', OLD.rowid, OLD.name, OLD.qualified_name, OLD.signature, OLD.description, OLD.tags);
		INSERT INTO function_fts(rowid, name, qualified_name, signature, description, tags)
		VALUES (NEW.rowid, NEW.name, NEW.qualified_name, NEW.signature, NEW.description, NEW.tags);
	END;

	-- Metadata table for storage stats
	CREATE TABLE IF NOT EXISTS storage_metadata (
		key TEXT PRIMARY KEY,
		value TEXT,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
	`

	_, err := s.db.ExecContext(ctx, schema)
	if err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	return nil
}

// Close closes the database connection.
func (s *SQLiteStorage) Close() error {
	return s.db.Close()
}

// BeginTx starts a new transaction.
func (s *SQLiteStorage) BeginTx(ctx context.Context) (Transaction, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	return &sqliteTx{tx: tx}, nil
}

type sqliteTx struct {
	tx *sql.Tx
}

func (t *sqliteTx) Commit() error   { return t.tx.Commit() }
func (t *sqliteTx) Rollback() error { return t.tx.Rollback() }

// Document operations

func (s *SQLiteStorage) CreateDocument(ctx context.Context, doc *Document) error {
	metadata, _ := json.Marshal(doc.Metadata)
	
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO documents (file_path, file_hash, file_size, line_count, module, language, metadata)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, doc.FilePath, doc.FileHash, doc.FileSize, doc.LineCount, doc.Module, doc.Language, string(metadata))
	
	if err != nil {
		return err
	}
	
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	doc.ID = id
	return nil
}

func (s *SQLiteStorage) GetDocument(ctx context.Context, id int64) (*Document, error) {
	doc := &Document{}
	var metadata string
	
	err := s.db.QueryRowContext(ctx, `
		SELECT id, file_path, file_hash, file_size, line_count, module, language, last_indexed, metadata
		FROM documents WHERE id = ?
	`, id).Scan(&doc.ID, &doc.FilePath, &doc.FileHash, &doc.FileSize, &doc.LineCount,
		&doc.Module, &doc.Language, &doc.LastIndexed, &metadata)
	
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	
	if metadata != "" {
		json.Unmarshal([]byte(metadata), &doc.Metadata)
	}
	return doc, nil
}

func (s *SQLiteStorage) GetDocumentByPath(ctx context.Context, filePath string) (*Document, error) {
	doc := &Document{}
	var metadata sql.NullString
	
	err := s.db.QueryRowContext(ctx, `
		SELECT id, file_path, file_hash, file_size, line_count, module, language, last_indexed, metadata
		FROM documents WHERE file_path = ?
	`, filePath).Scan(&doc.ID, &doc.FilePath, &doc.FileHash, &doc.FileSize, &doc.LineCount,
		&doc.Module, &doc.Language, &doc.LastIndexed, &metadata)
	
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	
	if metadata.Valid && metadata.String != "" {
		json.Unmarshal([]byte(metadata.String), &doc.Metadata)
	}
	return doc, nil
}

func (s *SQLiteStorage) UpdateDocument(ctx context.Context, doc *Document) error {
	metadata, _ := json.Marshal(doc.Metadata)
	
	_, err := s.db.ExecContext(ctx, `
		UPDATE documents SET file_hash = ?, file_size = ?, line_count = ?, 
		module = ?, language = ?, last_indexed = CURRENT_TIMESTAMP, metadata = ?
		WHERE id = ?
	`, doc.FileHash, doc.FileSize, doc.LineCount, doc.Module, doc.Language, string(metadata), doc.ID)
	
	return err
}

func (s *SQLiteStorage) DeleteDocument(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM documents WHERE id = ?", id)
	return err
}

func (s *SQLiteStorage) ListDocuments(ctx context.Context, filter *DocumentFilter) ([]*Document, error) {
	query := "SELECT id, file_path, file_hash, file_size, line_count, module, language, last_indexed FROM documents WHERE 1=1"
	args := make([]interface{}, 0)
	
	if filter.Module != "" {
		query += " AND module = ?"
		args = append(args, filter.Module)
	}
	if filter.Language != "" {
		query += " AND language = ?"
		args = append(args, filter.Language)
	}
	if filter.PathLike != "" {
		query += " AND file_path LIKE ?"
		args = append(args, "%"+filter.PathLike+"%")
	}
	
	query += " ORDER BY file_path"
	
	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", filter.Limit)
	}
	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET %d", filter.Offset)
	}
	
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var docs []*Document
	for rows.Next() {
		doc := &Document{}
		if err := rows.Scan(&doc.ID, &doc.FilePath, &doc.FileHash, &doc.FileSize,
			&doc.LineCount, &doc.Module, &doc.Language, &doc.LastIndexed); err != nil {
			return nil, err
		}
		docs = append(docs, doc)
	}
	return docs, nil
}

func (s *SQLiteStorage) BatchCreateDocuments(ctx context.Context, docs []*Document) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	
	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO documents (file_path, file_hash, file_size, line_count, module, language, metadata)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	
	for _, doc := range docs {
		metadata, _ := json.Marshal(doc.Metadata)
		result, err := stmt.ExecContext(ctx, doc.FilePath, doc.FileHash, doc.FileSize,
			doc.LineCount, doc.Module, doc.Language, string(metadata))
		if err != nil {
			return err
		}
		doc.ID, _ = result.LastInsertId()
	}
	
	return tx.Commit()
}

func (s *SQLiteStorage) BatchDeleteDocuments(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	
	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}
	
	_, err := s.db.ExecContext(ctx, 
		fmt.Sprintf("DELETE FROM documents WHERE id IN (%s)", strings.Join(placeholders, ",")),
		args...)
	return err
}

func (s *SQLiteStorage) GetDocumentHash(ctx context.Context, filePath string) (string, error) {
	var hash string
	err := s.db.QueryRowContext(ctx, "SELECT file_hash FROM documents WHERE file_path = ?", filePath).Scan(&hash)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return hash, err
}

func (s *SQLiteStorage) GetAllDocumentHashes(ctx context.Context) (map[string]string, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT file_path, file_hash FROM documents")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	hashes := make(map[string]string)
	for rows.Next() {
		var path, hash string
		if err := rows.Scan(&path, &hash); err != nil {
			return nil, err
		}
		hashes[path] = hash
	}
	return hashes, nil
}

// Inverted index operations

func (s *SQLiteStorage) AddTermOccurrence(ctx context.Context, term string, docID int64, frequency int, positions []Position) error {
	posJSON, _ := json.Marshal(positions)
	
	_, err := s.db.ExecContext(ctx, `
		INSERT OR REPLACE INTO inverted_index (term, doc_id, frequency, positions)
		VALUES (?, ?, ?, ?)
	`, term, docID, frequency, string(posJSON))
	
	return err
}

func (s *SQLiteStorage) GetTermDocuments(ctx context.Context, term string) ([]*TermOccurrence, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT term, doc_id, frequency, positions FROM inverted_index WHERE term = ?
	`, term)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var results []*TermOccurrence
	for rows.Next() {
		occ := &TermOccurrence{}
		var posJSON string
		if err := rows.Scan(&occ.Term, &occ.DocID, &occ.Frequency, &posJSON); err != nil {
			return nil, err
		}
		if posJSON != "" {
			json.Unmarshal([]byte(posJSON), &occ.Positions)
		}
		results = append(results, occ)
	}
	return results, nil
}

func (s *SQLiteStorage) GetDocumentTerms(ctx context.Context, docID int64) ([]*TermOccurrence, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT term, doc_id, frequency, positions FROM inverted_index WHERE doc_id = ?
	`, docID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var results []*TermOccurrence
	for rows.Next() {
		occ := &TermOccurrence{}
		var posJSON string
		if err := rows.Scan(&occ.Term, &occ.DocID, &occ.Frequency, &posJSON); err != nil {
			return nil, err
		}
		if posJSON != "" {
			json.Unmarshal([]byte(posJSON), &occ.Positions)
		}
		results = append(results, occ)
	}
	return results, nil
}

func (s *SQLiteStorage) DeleteDocumentTerms(ctx context.Context, docID int64) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM inverted_index WHERE doc_id = ?", docID)
	return err
}

func (s *SQLiteStorage) BatchAddTermOccurrences(ctx context.Context, occurrences []*TermOccurrence) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	
	stmt, err := tx.PrepareContext(ctx, `
		INSERT OR REPLACE INTO inverted_index (term, doc_id, frequency, positions)
		VALUES (?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	
	for _, occ := range occurrences {
		posJSON, _ := json.Marshal(occ.Positions)
		if _, err := stmt.ExecContext(ctx, occ.Term, occ.DocID, occ.Frequency, string(posJSON)); err != nil {
			return err
		}
	}
	
	return tx.Commit()
}

func (s *SQLiteStorage) SearchTerms(ctx context.Context, query *TermSearchQuery) ([]*SearchResult, error) {
	if len(query.Terms) == 0 {
		return nil, nil
	}
	
	// Build search query with BM25-like scoring
	var sqlQuery string
	var args []interface{}
	
	if query.Operator == SearchOperatorAND {
		// All terms must exist
		subqueries := make([]string, len(query.Terms))
		for i, term := range query.Terms {
			subqueries[i] = "SELECT DISTINCT doc_id FROM inverted_index WHERE term = ?"
			args = append(args, term)
		}
		sqlQuery = fmt.Sprintf(`
			SELECT d.id, d.file_path, SUM(ii.frequency) as score
			FROM documents d
			JOIN inverted_index ii ON d.id = ii.doc_id
			WHERE ii.term IN (%s)
			AND d.id IN (%s)
			GROUP BY d.id
			ORDER BY score DESC
		`, strings.Repeat("?,", len(query.Terms)-1)+"?",
			strings.Join(subqueries, " INTERSECT "))
		
		for _, term := range query.Terms {
			args = append(args, term)
		}
	} else {
		// Any term matches
		placeholders := make([]string, len(query.Terms))
		for i, term := range query.Terms {
			placeholders[i] = "?"
			args = append(args, term)
		}
		sqlQuery = fmt.Sprintf(`
			SELECT d.id, d.file_path, SUM(ii.frequency) as score
			FROM documents d
			JOIN inverted_index ii ON d.id = ii.doc_id
			WHERE ii.term IN (%s)
			GROUP BY d.id
			ORDER BY score DESC
		`, strings.Join(placeholders, ","))
	}
	
	if query.Limit > 0 {
		sqlQuery += fmt.Sprintf(" LIMIT %d", query.Limit)
	}
	
	rows, err := s.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var results []*SearchResult
	for rows.Next() {
		r := &SearchResult{}
		if err := rows.Scan(&r.DocID, &r.FilePath, &r.Score); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, nil
}

func (s *SQLiteStorage) GetTermDocumentCount(ctx context.Context, term string) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, 
		"SELECT COUNT(DISTINCT doc_id) FROM inverted_index WHERE term = ?", term).Scan(&count)
	return count, err
}

func (s *SQLiteStorage) GetTotalDocumentCount(ctx context.Context) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM documents").Scan(&count)
	return count, err
}

// Function summary operations

func (s *SQLiteStorage) CreateFunction(ctx context.Context, fn *FunctionSummary) error {
	params, _ := json.Marshal(fn.Parameters)
	tags, _ := json.Marshal(fn.Tags)
	callers, _ := json.Marshal(fn.Callers)
	callees, _ := json.Marshal(fn.Callees)
	stats, _ := json.Marshal(fn.Stats)
	
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO function_summaries (id, name, qualified_name, file_path, line_start, line_end,
			signature, return_type, parameters, description, tags, module, subsystem, loc, complexity,
			callers, callees, stats)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, fn.ID, fn.Name, fn.QualifiedName, fn.FilePath, fn.LineStart, fn.LineEnd,
		fn.Signature, fn.ReturnType, string(params), fn.Description, string(tags),
		fn.Module, fn.Subsystem, fn.LOC, fn.Complexity, string(callers), string(callees), string(stats))
	
	return err
}

func (s *SQLiteStorage) GetFunction(ctx context.Context, id string) (*FunctionSummary, error) {
	fn := &FunctionSummary{}
	var params, tags, callers, callees, stats sql.NullString
	
	err := s.db.QueryRowContext(ctx, `
		SELECT id, name, qualified_name, file_path, line_start, line_end, signature, return_type,
		parameters, description, tags, module, subsystem, loc, complexity, callers, callees, stats,
		created_at, updated_at
		FROM function_summaries WHERE id = ?
	`, id).Scan(&fn.ID, &fn.Name, &fn.QualifiedName, &fn.FilePath, &fn.LineStart, &fn.LineEnd,
		&fn.Signature, &fn.ReturnType, &params, &fn.Description, &tags, &fn.Module, &fn.Subsystem,
		&fn.LOC, &fn.Complexity, &callers, &callees, &stats, &fn.CreatedAt, &fn.UpdatedAt)
	
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	
	if params.Valid {
		json.Unmarshal([]byte(params.String), &fn.Parameters)
	}
	if tags.Valid {
		json.Unmarshal([]byte(tags.String), &fn.Tags)
	}
	if callers.Valid {
		json.Unmarshal([]byte(callers.String), &fn.Callers)
	}
	if callees.Valid {
		json.Unmarshal([]byte(callees.String), &fn.Callees)
	}
	if stats.Valid {
		json.Unmarshal([]byte(stats.String), &fn.Stats)
	}
	
	return fn, nil
}

func (s *SQLiteStorage) GetFunctionByName(ctx context.Context, name string) ([]*FunctionSummary, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, qualified_name, file_path, line_start, line_end, signature, return_type,
		parameters, description, tags, module, subsystem, loc, complexity, callers, callees, stats,
		created_at, updated_at
		FROM function_summaries WHERE name = ?
	`, name)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	return s.scanFunctions(rows)
}

func (s *SQLiteStorage) scanFunctions(rows *sql.Rows) ([]*FunctionSummary, error) {
	var functions []*FunctionSummary
	for rows.Next() {
		fn := &FunctionSummary{}
		var params, tags, callers, callees, stats sql.NullString
		
		if err := rows.Scan(&fn.ID, &fn.Name, &fn.QualifiedName, &fn.FilePath, &fn.LineStart, &fn.LineEnd,
			&fn.Signature, &fn.ReturnType, &params, &fn.Description, &tags, &fn.Module, &fn.Subsystem,
			&fn.LOC, &fn.Complexity, &callers, &callees, &stats, &fn.CreatedAt, &fn.UpdatedAt); err != nil {
			return nil, err
		}
		
		if params.Valid {
			json.Unmarshal([]byte(params.String), &fn.Parameters)
		}
		if tags.Valid {
			json.Unmarshal([]byte(tags.String), &fn.Tags)
		}
		if callers.Valid {
			json.Unmarshal([]byte(callers.String), &fn.Callers)
		}
		if callees.Valid {
			json.Unmarshal([]byte(callees.String), &fn.Callees)
		}
		if stats.Valid {
			json.Unmarshal([]byte(stats.String), &fn.Stats)
		}
		
		functions = append(functions, fn)
	}
	return functions, nil
}

func (s *SQLiteStorage) UpdateFunction(ctx context.Context, fn *FunctionSummary) error {
	params, _ := json.Marshal(fn.Parameters)
	tags, _ := json.Marshal(fn.Tags)
	callers, _ := json.Marshal(fn.Callers)
	callees, _ := json.Marshal(fn.Callees)
	stats, _ := json.Marshal(fn.Stats)
	
	_, err := s.db.ExecContext(ctx, `
		UPDATE function_summaries SET name = ?, qualified_name = ?, file_path = ?, line_start = ?,
		line_end = ?, signature = ?, return_type = ?, parameters = ?, description = ?, tags = ?,
		module = ?, subsystem = ?, loc = ?, complexity = ?, callers = ?, callees = ?, stats = ?,
		updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, fn.Name, fn.QualifiedName, fn.FilePath, fn.LineStart, fn.LineEnd, fn.Signature,
		fn.ReturnType, string(params), fn.Description, string(tags), fn.Module, fn.Subsystem,
		fn.LOC, fn.Complexity, string(callers), string(callees), string(stats), fn.ID)
	
	return err
}

func (s *SQLiteStorage) DeleteFunction(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM function_summaries WHERE id = ?", id)
	return err
}

func (s *SQLiteStorage) DeleteFunctionsByFile(ctx context.Context, filePath string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM function_summaries WHERE file_path = ?", filePath)
	return err
}

func (s *SQLiteStorage) BatchCreateFunctions(ctx context.Context, fns []*FunctionSummary) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	
	stmt, err := tx.PrepareContext(ctx, `
		INSERT OR REPLACE INTO function_summaries (id, name, qualified_name, file_path, line_start, line_end,
			signature, return_type, parameters, description, tags, module, subsystem, loc, complexity,
			callers, callees, stats)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	
	for _, fn := range fns {
		params, _ := json.Marshal(fn.Parameters)
		tags, _ := json.Marshal(fn.Tags)
		callers, _ := json.Marshal(fn.Callers)
		callees, _ := json.Marshal(fn.Callees)
		stats, _ := json.Marshal(fn.Stats)
		
		if _, err := stmt.ExecContext(ctx, fn.ID, fn.Name, fn.QualifiedName, fn.FilePath,
			fn.LineStart, fn.LineEnd, fn.Signature, fn.ReturnType, string(params), fn.Description,
			string(tags), fn.Module, fn.Subsystem, fn.LOC, fn.Complexity, string(callers),
			string(callees), string(stats)); err != nil {
			return err
		}
	}
	
	return tx.Commit()
}

func (s *SQLiteStorage) SearchFunctions(ctx context.Context, query *FunctionSearchQuery) ([]*FunctionSummary, error) {
	if query.Query != "" {
		// Use FTS5 for full-text search
		rows, err := s.db.QueryContext(ctx, `
			SELECT f.id, f.name, f.qualified_name, f.file_path, f.line_start, f.line_end,
			f.signature, f.return_type, f.parameters, f.description, f.tags, f.module, f.subsystem,
			f.loc, f.complexity, f.callers, f.callees, f.stats, f.created_at, f.updated_at
			FROM function_summaries f
			JOIN function_fts fts ON f.rowid = fts.rowid
			WHERE function_fts MATCH ?
			LIMIT ?
		`, query.Query, query.Limit)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		return s.scanFunctions(rows)
	}
	
	// Build regular SQL query
	sqlQuery := `SELECT id, name, qualified_name, file_path, line_start, line_end,
		signature, return_type, parameters, description, tags, module, subsystem,
		loc, complexity, callers, callees, stats, created_at, updated_at
		FROM function_summaries WHERE 1=1`
	args := make([]interface{}, 0)
	
	if query.Name != "" {
		sqlQuery += " AND (name = ? OR name LIKE ?)"
		args = append(args, query.Name, query.Name+"%")
	}
	if query.Module != "" {
		sqlQuery += " AND module = ?"
		args = append(args, query.Module)
	}
	if query.Subsystem != "" {
		sqlQuery += " AND subsystem = ?"
		args = append(args, query.Subsystem)
	}
	if query.MinLOC > 0 {
		sqlQuery += " AND loc >= ?"
		args = append(args, query.MinLOC)
	}
	if query.MaxLOC > 0 {
		sqlQuery += " AND loc <= ?"
		args = append(args, query.MaxLOC)
	}
	
	sqlQuery += " ORDER BY name"
	
	if query.Limit > 0 {
		sqlQuery += fmt.Sprintf(" LIMIT %d", query.Limit)
	}
	if query.Offset > 0 {
		sqlQuery += fmt.Sprintf(" OFFSET %d", query.Offset)
	}
	
	rows, err := s.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	return s.scanFunctions(rows)
}

func (s *SQLiteStorage) GetFunctionCount(ctx context.Context) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM function_summaries").Scan(&count)
	return count, err
}

func (s *SQLiteStorage) GetFunctionsByModule(ctx context.Context, module string) ([]*FunctionSummary, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, qualified_name, file_path, line_start, line_end, signature, return_type,
		parameters, description, tags, module, subsystem, loc, complexity, callers, callees, stats,
		created_at, updated_at
		FROM function_summaries WHERE module = ?
	`, module)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	return s.scanFunctions(rows)
}

// Vacuum performs database maintenance.
func (s *SQLiteStorage) Vacuum(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, "VACUUM")
	return err
}

// Stats returns storage statistics.
func (s *SQLiteStorage) Stats(ctx context.Context) (*StorageStats, error) {
	stats := &StorageStats{}
	
	s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM documents").Scan(&stats.DocumentCount)
	s.db.QueryRowContext(ctx, "SELECT COUNT(DISTINCT term) FROM inverted_index").Scan(&stats.TermCount)
	s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM function_summaries").Scan(&stats.FunctionCount)
	
	return stats, nil
}

// CallGraph operations - SQLite implementation stores in separate table
// For complex graph queries, BoltDB is recommended

func (s *SQLiteStorage) CreateNode(ctx context.Context, node *CallGraphNode) error {
	return fmt.Errorf("call graph operations should use BoltDB storage")
}

func (s *SQLiteStorage) GetNode(ctx context.Context, id string) (*CallGraphNode, error) {
	return nil, fmt.Errorf("call graph operations should use BoltDB storage")
}

func (s *SQLiteStorage) UpdateNode(ctx context.Context, node *CallGraphNode) error {
	return fmt.Errorf("call graph operations should use BoltDB storage")
}

func (s *SQLiteStorage) DeleteNode(ctx context.Context, id string) error {
	return fmt.Errorf("call graph operations should use BoltDB storage")
}

func (s *SQLiteStorage) CreateEdge(ctx context.Context, edge *CallGraphEdge) error {
	return fmt.Errorf("call graph operations should use BoltDB storage")
}

func (s *SQLiteStorage) GetEdge(ctx context.Context, id string) (*CallGraphEdge, error) {
	return nil, fmt.Errorf("call graph operations should use BoltDB storage")
}

func (s *SQLiteStorage) DeleteEdge(ctx context.Context, id string) error {
	return fmt.Errorf("call graph operations should use BoltDB storage")
}

func (s *SQLiteStorage) GetCallers(ctx context.Context, funcID string) ([]*CallGraphEdge, error) {
	return nil, fmt.Errorf("call graph operations should use BoltDB storage")
}

func (s *SQLiteStorage) GetCallees(ctx context.Context, funcID string) ([]*CallGraphEdge, error) {
	return nil, fmt.Errorf("call graph operations should use BoltDB storage")
}

func (s *SQLiteStorage) GetCallChain(ctx context.Context, funcID string, depth int, direction Direction) ([]*CallGraphNode, []*CallGraphEdge, error) {
	return nil, nil, fmt.Errorf("call graph operations should use BoltDB storage")
}

func (s *SQLiteStorage) BatchCreateNodes(ctx context.Context, nodes []*CallGraphNode) error {
	return fmt.Errorf("call graph operations should use BoltDB storage")
}

func (s *SQLiteStorage) BatchCreateEdges(ctx context.Context, edges []*CallGraphEdge) error {
	return fmt.Errorf("call graph operations should use BoltDB storage")
}

func (s *SQLiteStorage) GetNodesByModule(ctx context.Context, module string) ([]*CallGraphNode, error) {
	return nil, fmt.Errorf("call graph operations should use BoltDB storage")
}

func (s *SQLiteStorage) GetNodesByFile(ctx context.Context, filePath string) ([]*CallGraphNode, error) {
	return nil, fmt.Errorf("call graph operations should use BoltDB storage")
}
