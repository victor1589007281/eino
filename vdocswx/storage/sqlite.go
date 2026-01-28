// Package storage 提供数据存储功能
package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// SQLiteStorage SQLite存储实现
type SQLiteStorage struct {
	db *sql.DB
}

// Article 文章实体
type Article struct {
	ID          string            `json:"id"`
	Title       string            `json:"title"`
	Content     string            `json:"content"`
	Type        string            `json:"type"`
	Metadata    map[string]string `json:"metadata"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// PolishRecord 润色记录
type PolishRecord struct {
	ID              string    `json:"id"`
	ArticleID       string    `json:"article_id"`
	OriginalContent string    `json:"original_content"`
	PolishedContent string    `json:"polished_content"`
	Changes         string    `json:"changes"` // JSON
	TokensUsed      int       `json:"tokens_used"`
	CreatedAt       time.Time `json:"created_at"`
}

// Session 会话实体
type Session struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	History   string    `json:"history"` // JSON
	Draft     string    `json:"draft"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// NewSQLiteStorage 创建SQLite存储
func NewSQLiteStorage(dbPath string) (*SQLiteStorage, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	
	storage := &SQLiteStorage{db: db}
	
	if err := storage.initTables(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to init tables: %w", err)
	}
	
	return storage, nil
}

// initTables 初始化表结构
func (s *SQLiteStorage) initTables() error {
	tables := []string{
		`CREATE TABLE IF NOT EXISTS articles (
			id TEXT PRIMARY KEY,
			title TEXT NOT NULL,
			content TEXT NOT NULL,
			type TEXT DEFAULT 'unknown',
			metadata TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS polish_records (
			id TEXT PRIMARY KEY,
			article_id TEXT NOT NULL,
			original_content TEXT NOT NULL,
			polished_content TEXT NOT NULL,
			changes TEXT,
			tokens_used INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (article_id) REFERENCES articles(id)
		)`,
		`CREATE TABLE IF NOT EXISTS sessions (
			id TEXT PRIMARY KEY,
			user_id TEXT,
			history TEXT,
			draft TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS index_terms (
			term TEXT NOT NULL,
			doc_id TEXT NOT NULL,
			positions TEXT,
			tf REAL,
			PRIMARY KEY (term, doc_id)
		)`,
		`CREATE TABLE IF NOT EXISTS cache_entries (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			expires_at DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_articles_type ON articles(type)`,
		`CREATE INDEX IF NOT EXISTS idx_polish_records_article ON polish_records(article_id)`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_user ON sessions(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_index_terms_doc ON index_terms(doc_id)`,
		`CREATE INDEX IF NOT EXISTS idx_cache_expires ON cache_entries(expires_at)`,
	}
	
	for _, table := range tables {
		if _, err := s.db.Exec(table); err != nil {
			return fmt.Errorf("failed to create table: %w", err)
		}
	}
	
	return nil
}

// Close 关闭数据库连接
func (s *SQLiteStorage) Close() error {
	return s.db.Close()
}

// SaveArticle 保存文章
func (s *SQLiteStorage) SaveArticle(ctx context.Context, article *Article) error {
	metadata, err := json.Marshal(article.Metadata)
	if err != nil {
		return err
	}
	
	query := `INSERT OR REPLACE INTO articles (id, title, content, type, metadata, updated_at)
	          VALUES (?, ?, ?, ?, ?, ?)`
	
	_, err = s.db.ExecContext(ctx, query, 
		article.ID, article.Title, article.Content, article.Type, 
		string(metadata), time.Now())
	
	return err
}

// GetArticle 获取文章
func (s *SQLiteStorage) GetArticle(ctx context.Context, id string) (*Article, error) {
	query := `SELECT id, title, content, type, metadata, created_at, updated_at 
	          FROM articles WHERE id = ?`
	
	var article Article
	var metadata string
	
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&article.ID, &article.Title, &article.Content, &article.Type,
		&metadata, &article.CreatedAt, &article.UpdatedAt)
	
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	
	if metadata != "" {
		json.Unmarshal([]byte(metadata), &article.Metadata)
	}
	
	return &article, nil
}

// ListArticles 列出文章
func (s *SQLiteStorage) ListArticles(ctx context.Context, limit, offset int) ([]*Article, error) {
	query := `SELECT id, title, content, type, metadata, created_at, updated_at 
	          FROM articles ORDER BY updated_at DESC LIMIT ? OFFSET ?`
	
	rows, err := s.db.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var articles []*Article
	for rows.Next() {
		var article Article
		var metadata string
		
		err := rows.Scan(&article.ID, &article.Title, &article.Content, &article.Type,
			&metadata, &article.CreatedAt, &article.UpdatedAt)
		if err != nil {
			return nil, err
		}
		
		if metadata != "" {
			json.Unmarshal([]byte(metadata), &article.Metadata)
		}
		
		articles = append(articles, &article)
	}
	
	return articles, nil
}

// DeleteArticle 删除文章
func (s *SQLiteStorage) DeleteArticle(ctx context.Context, id string) error {
	query := `DELETE FROM articles WHERE id = ?`
	_, err := s.db.ExecContext(ctx, query, id)
	return err
}

// SavePolishRecord 保存润色记录
func (s *SQLiteStorage) SavePolishRecord(ctx context.Context, record *PolishRecord) error {
	query := `INSERT INTO polish_records (id, article_id, original_content, polished_content, changes, tokens_used)
	          VALUES (?, ?, ?, ?, ?, ?)`
	
	_, err := s.db.ExecContext(ctx, query,
		record.ID, record.ArticleID, record.OriginalContent, 
		record.PolishedContent, record.Changes, record.TokensUsed)
	
	return err
}

// GetPolishRecords 获取文章的润色记录
func (s *SQLiteStorage) GetPolishRecords(ctx context.Context, articleID string) ([]*PolishRecord, error) {
	query := `SELECT id, article_id, original_content, polished_content, changes, tokens_used, created_at
	          FROM polish_records WHERE article_id = ? ORDER BY created_at DESC`
	
	rows, err := s.db.QueryContext(ctx, query, articleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var records []*PolishRecord
	for rows.Next() {
		var record PolishRecord
		err := rows.Scan(&record.ID, &record.ArticleID, &record.OriginalContent,
			&record.PolishedContent, &record.Changes, &record.TokensUsed, &record.CreatedAt)
		if err != nil {
			return nil, err
		}
		records = append(records, &record)
	}
	
	return records, nil
}

// SaveSession 保存会话
func (s *SQLiteStorage) SaveSession(ctx context.Context, session *Session) error {
	query := `INSERT OR REPLACE INTO sessions (id, user_id, history, draft, updated_at)
	          VALUES (?, ?, ?, ?, ?)`
	
	_, err := s.db.ExecContext(ctx, query,
		session.ID, session.UserID, session.History, session.Draft, time.Now())
	
	return err
}

// GetSession 获取会话
func (s *SQLiteStorage) GetSession(ctx context.Context, id string) (*Session, error) {
	query := `SELECT id, user_id, history, draft, created_at, updated_at
	          FROM sessions WHERE id = ?`
	
	var session Session
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&session.ID, &session.UserID, &session.History, &session.Draft,
		&session.CreatedAt, &session.UpdatedAt)
	
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	
	return &session, nil
}

// DeleteSession 删除会话
func (s *SQLiteStorage) DeleteSession(ctx context.Context, id string) error {
	query := `DELETE FROM sessions WHERE id = ?`
	_, err := s.db.ExecContext(ctx, query, id)
	return err
}

// SaveCacheEntry 保存缓存条目
func (s *SQLiteStorage) SaveCacheEntry(ctx context.Context, key, value string, expiresAt *time.Time) error {
	query := `INSERT OR REPLACE INTO cache_entries (key, value, expires_at)
	          VALUES (?, ?, ?)`
	
	_, err := s.db.ExecContext(ctx, query, key, value, expiresAt)
	return err
}

// GetCacheEntry 获取缓存条目
func (s *SQLiteStorage) GetCacheEntry(ctx context.Context, key string) (string, bool, error) {
	query := `SELECT value, expires_at FROM cache_entries WHERE key = ?`
	
	var value string
	var expiresAt sql.NullTime
	
	err := s.db.QueryRowContext(ctx, query, key).Scan(&value, &expiresAt)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	
	// 检查是否过期
	if expiresAt.Valid && expiresAt.Time.Before(time.Now()) {
		// 删除过期条目
		s.DeleteCacheEntry(ctx, key)
		return "", false, nil
	}
	
	return value, true, nil
}

// DeleteCacheEntry 删除缓存条目
func (s *SQLiteStorage) DeleteCacheEntry(ctx context.Context, key string) error {
	query := `DELETE FROM cache_entries WHERE key = ?`
	_, err := s.db.ExecContext(ctx, query, key)
	return err
}

// CleanExpiredCache 清理过期缓存
func (s *SQLiteStorage) CleanExpiredCache(ctx context.Context) (int64, error) {
	query := `DELETE FROM cache_entries WHERE expires_at < ?`
	result, err := s.db.ExecContext(ctx, query, time.Now())
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// SaveIndexTerm 保存索引词项
func (s *SQLiteStorage) SaveIndexTerm(ctx context.Context, term, docID string, positions []int, tf float64) error {
	positionsJSON, err := json.Marshal(positions)
	if err != nil {
		return err
	}
	
	query := `INSERT OR REPLACE INTO index_terms (term, doc_id, positions, tf)
	          VALUES (?, ?, ?, ?)`
	
	_, err = s.db.ExecContext(ctx, query, term, docID, string(positionsJSON), tf)
	return err
}

// GetIndexTerms 获取词项的文档列表
func (s *SQLiteStorage) GetIndexTerms(ctx context.Context, term string) ([]struct {
	DocID     string
	Positions []int
	TF        float64
}, error) {
	query := `SELECT doc_id, positions, tf FROM index_terms WHERE term = ?`
	
	rows, err := s.db.QueryContext(ctx, query, term)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var results []struct {
		DocID     string
		Positions []int
		TF        float64
	}
	
	for rows.Next() {
		var docID, positionsJSON string
		var tf float64
		
		if err := rows.Scan(&docID, &positionsJSON, &tf); err != nil {
			return nil, err
		}
		
		var positions []int
		json.Unmarshal([]byte(positionsJSON), &positions)
		
		results = append(results, struct {
			DocID     string
			Positions []int
			TF        float64
		}{
			DocID:     docID,
			Positions: positions,
			TF:        tf,
		})
	}
	
	return results, nil
}

// DeleteIndexByDoc 删除文档的索引
func (s *SQLiteStorage) DeleteIndexByDoc(ctx context.Context, docID string) error {
	query := `DELETE FROM index_terms WHERE doc_id = ?`
	_, err := s.db.ExecContext(ctx, query, docID)
	return err
}
