// Package storage provides data storage capabilities.
package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// StorageType represents the storage type.
type StorageType string

const (
	StorageTypeSQLite StorageType = "sqlite"
	StorageTypeBoltDB StorageType = "boltdb"
	StorageTypeRedis  StorageType = "redis"
	StorageTypeMySQL  StorageType = "mysql"
)

// StorageConfig represents storage configuration.
type StorageConfig struct {
	Type     StorageType `json:"type"`
	Path     string      `json:"path"`      // For SQLite/BoltDB
	Host     string      `json:"host"`      // For Redis/MySQL
	Port     int         `json:"port"`      // For Redis/MySQL
	Database string      `json:"database"`  // For MySQL
	User     string      `json:"user"`      // For MySQL
	Password string      `json:"password"`  // For Redis/MySQL
	PoolSize int         `json:"pool_size"` // Connection pool size
}

// Storage defines the storage interface.
type Storage interface {
	Get(ctx context.Context, collection, key string) ([]byte, error)
	Set(ctx context.Context, collection, key string, value []byte) error
	Delete(ctx context.Context, collection, key string) error
	List(ctx context.Context, collection string, prefix string, limit int) ([]string, error)
	Query(ctx context.Context, collection string, query map[string]interface{}, limit int) ([][]byte, error)
	Close() error
}

// SQLiteStorage implements SQLite-based storage.
type SQLiteStorage struct {
	db     *sql.DB
	path   string
	mu     sync.RWMutex
}

// NewSQLiteStorage creates a new SQLite storage.
func NewSQLiteStorage(path string) (*SQLiteStorage, error) {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}
	
	// Create default table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS kv_store (
			collection TEXT NOT NULL,
			key TEXT NOT NULL,
			value BLOB,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (collection, key)
		);
		CREATE INDEX IF NOT EXISTS idx_collection ON kv_store(collection);
		CREATE INDEX IF NOT EXISTS idx_key ON kv_store(key);
	`)
	if err != nil {
		db.Close()
		return nil, err
	}
	
	return &SQLiteStorage{
		db:   db,
		path: path,
	}, nil
}

// Get retrieves a value from storage.
func (s *SQLiteStorage) Get(ctx context.Context, collection, key string) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	var value []byte
	err := s.db.QueryRowContext(ctx,
		"SELECT value FROM kv_store WHERE collection = ? AND key = ?",
		collection, key).Scan(&value)
	
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return value, err
}

// Set stores a value.
func (s *SQLiteStorage) Set(ctx context.Context, collection, key string, value []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	_, err := s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO kv_store (collection, key, value, updated_at) 
		 VALUES (?, ?, ?, CURRENT_TIMESTAMP)`,
		collection, key, value)
	return err
}

// Delete removes a value.
func (s *SQLiteStorage) Delete(ctx context.Context, collection, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	_, err := s.db.ExecContext(ctx,
		"DELETE FROM kv_store WHERE collection = ? AND key = ?",
		collection, key)
	return err
}

// List lists keys in a collection.
func (s *SQLiteStorage) List(ctx context.Context, collection string, prefix string, limit int) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	query := "SELECT key FROM kv_store WHERE collection = ?"
	args := []interface{}{collection}
	
	if prefix != "" {
		query += " AND key LIKE ?"
		args = append(args, prefix+"%")
	}
	
	query += " ORDER BY key"
	
	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
	}
	
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var keys []string
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}
	
	return keys, rows.Err()
}

// Query queries values by criteria.
func (s *SQLiteStorage) Query(ctx context.Context, collection string, query map[string]interface{}, limit int) ([][]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	// For SQLite, we just list all values in collection
	// More sophisticated querying would require JSON functions
	sqlQuery := "SELECT value FROM kv_store WHERE collection = ?"
	args := []interface{}{collection}
	
	if limit > 0 {
		sqlQuery += " LIMIT ?"
		args = append(args, limit)
	}
	
	rows, err := s.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var values [][]byte
	for rows.Next() {
		var value []byte
		if err := rows.Scan(&value); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	
	return values, rows.Err()
}

// Close closes the storage.
func (s *SQLiteStorage) Close() error {
	return s.db.Close()
}

// Document represents a stored document.
type Document struct {
	ID          string                 `json:"id"`
	Collection  string                 `json:"collection"`
	Content     string                 `json:"content"`
	Metadata    map[string]interface{} `json:"metadata"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	Timeliness  *TimelinessInfo        `json:"timeliness,omitempty"`
}

// TimelinessInfo represents timeliness information.
type TimelinessInfo struct {
	Source       string    `json:"source"`
	PublishDate  time.Time `json:"publish_date"`
	EffectDate   time.Time `json:"effect_date,omitempty"`
	ExpireDate   time.Time `json:"expire_date,omitempty"`
	LastVerified time.Time `json:"last_verified"`
	IsValid      bool      `json:"is_valid"`
}

// DocumentStore provides document storage with timeliness tracking.
type DocumentStore struct {
	storage Storage
}

// NewDocumentStore creates a new document store.
func NewDocumentStore(storage Storage) *DocumentStore {
	return &DocumentStore{storage: storage}
}

// SaveDocument saves a document.
func (ds *DocumentStore) SaveDocument(ctx context.Context, doc *Document) error {
	if doc.ID == "" {
		return errors.New("document ID is required")
	}
	
	doc.UpdatedAt = time.Now()
	if doc.CreatedAt.IsZero() {
		doc.CreatedAt = doc.UpdatedAt
	}
	
	data, err := json.Marshal(doc)
	if err != nil {
		return err
	}
	
	return ds.storage.Set(ctx, doc.Collection, doc.ID, data)
}

// GetDocument retrieves a document.
func (ds *DocumentStore) GetDocument(ctx context.Context, collection, id string) (*Document, error) {
	data, err := ds.storage.Get(ctx, collection, id)
	if err != nil {
		return nil, err
	}
	if data == nil {
		return nil, nil
	}
	
	var doc Document
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	
	return &doc, nil
}

// DeleteDocument deletes a document.
func (ds *DocumentStore) DeleteDocument(ctx context.Context, collection, id string) error {
	return ds.storage.Delete(ctx, collection, id)
}

// ListDocuments lists documents in a collection.
func (ds *DocumentStore) ListDocuments(ctx context.Context, collection string, prefix string, limit int) ([]*Document, error) {
	keys, err := ds.storage.List(ctx, collection, prefix, limit)
	if err != nil {
		return nil, err
	}
	
	var docs []*Document
	for _, key := range keys {
		doc, err := ds.GetDocument(ctx, collection, key)
		if err != nil {
			continue
		}
		if doc != nil {
			docs = append(docs, doc)
		}
	}
	
	return docs, nil
}

// QueryDocuments queries documents.
func (ds *DocumentStore) QueryDocuments(ctx context.Context, collection string, query map[string]interface{}, limit int) ([]*Document, error) {
	values, err := ds.storage.Query(ctx, collection, query, limit)
	if err != nil {
		return nil, err
	}
	
	var docs []*Document
	for _, data := range values {
		var doc Document
		if err := json.Unmarshal(data, &doc); err != nil {
			continue
		}
		docs = append(docs, &doc)
	}
	
	return docs, nil
}

// CheckTimeliness checks document timeliness.
func (ds *DocumentStore) CheckTimeliness(doc *Document) bool {
	if doc.Timeliness == nil {
		return true // No timeliness info, assume valid
	}
	
	now := time.Now()
	
	// Check if expired
	if !doc.Timeliness.ExpireDate.IsZero() && now.After(doc.Timeliness.ExpireDate) {
		return false
	}
	
	// Check if effect date has passed
	if !doc.Timeliness.EffectDate.IsZero() && now.Before(doc.Timeliness.EffectDate) {
		return false
	}
	
	return doc.Timeliness.IsValid
}

// UpdateTimeliness updates document timeliness.
func (ds *DocumentStore) UpdateTimeliness(ctx context.Context, collection, id string, info *TimelinessInfo) error {
	doc, err := ds.GetDocument(ctx, collection, id)
	if err != nil {
		return err
	}
	if doc == nil {
		return fmt.Errorf("document not found: %s/%s", collection, id)
	}
	
	info.LastVerified = time.Now()
	doc.Timeliness = info
	
	return ds.SaveDocument(ctx, doc)
}

// LawDocument represents a law/regulation document.
type LawDocument struct {
	Document
	LawName     string `json:"law_name"`
	ArticleNo   string `json:"article_no"`
	Content     string `json:"content"`
	Status      string `json:"status"` // valid, amended, repealed
	Issuer      string `json:"issuer"`
	IssueDate   time.Time `json:"issue_date"`
	EffectDate  time.Time `json:"effect_date"`
}

// ProductDocument represents an insurance product document.
type ProductDocument struct {
	Document
	ProductName   string `json:"product_name"`
	ProductCode   string `json:"product_code"`
	InsuranceType string `json:"insurance_type"`
	Company       string `json:"company"`
	Version       string `json:"version"`
	EffectDate    time.Time `json:"effect_date"`
	Terms         map[string]string `json:"terms"`
}

// ClaimCaseDocument represents a claim case document.
type ClaimCaseDocument struct {
	Document
	CaseID        string `json:"case_id"`
	InsuranceType string `json:"insurance_type"`
	ClaimType     string `json:"claim_type"`
	Outcome       string `json:"outcome"` // approved, rejected, partial
	ClaimAmount   float64 `json:"claim_amount"`
	PaidAmount    float64 `json:"paid_amount"`
	CaseDate      time.Time `json:"case_date"`
	Summary       string `json:"summary"`
	KeyFacts      string `json:"key_facts"`
	Reason        string `json:"reason"`
}

// NewStorage creates storage based on configuration.
func NewStorage(config *StorageConfig) (Storage, error) {
	switch config.Type {
	case StorageTypeSQLite:
		return NewSQLiteStorage(config.Path)
	default:
		return nil, fmt.Errorf("unsupported storage type: %s", config.Type)
	}
}
