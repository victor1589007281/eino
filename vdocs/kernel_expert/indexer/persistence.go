// Package indexer 提供Linux内核源码的索引功能
package indexer

import (
	"compress/gzip"
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// IndexMeta 索引元信息
type IndexMeta struct {
	Version        string    `json:"version"`
	SourceHash     string    `json:"source_hash"`
	CreatedAt      time.Time `json:"created_at"`
	LastUpdated    time.Time `json:"last_updated"`
	FileCount      int       `json:"file_count"`
	TotalSize      int64     `json:"total_size"`
	IndexerVersion string    `json:"indexer_version"`
}

// IndexPersistence 索引持久化管理器
type IndexPersistence struct {
	storagePath string
	mu          sync.RWMutex
}

// NewIndexPersistence 创建索引持久化管理器
func NewIndexPersistence(storagePath string) (*IndexPersistence, error) {
	if err := os.MkdirAll(storagePath, 0755); err != nil {
		return nil, fmt.Errorf("create storage path: %w", err)
	}
	return &IndexPersistence{
		storagePath: storagePath,
	}, nil
}

// SaveIndex 保存索引到磁盘
func (p *IndexPersistence) SaveIndex(idx *Index) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 保存倒排索引
	if err := p.saveGob(filepath.Join(p.storagePath, "inverted.idx.gz"), idx.InvertedIndex); err != nil {
		return fmt.Errorf("save inverted index: %w", err)
	}

	// 保存函数摘要
	if err := p.saveGob(filepath.Join(p.storagePath, "functions.idx.gz"), idx.FunctionSummaries); err != nil {
		return fmt.Errorf("save function summaries: %w", err)
	}

	// 保存符号表
	if err := p.saveGob(filepath.Join(p.storagePath, "symbols.idx.gz"), idx.SymbolTable); err != nil {
		return fmt.Errorf("save symbol table: %w", err)
	}

	// 保存调用图
	if err := p.saveGob(filepath.Join(p.storagePath, "callgraph.idx.gz"), idx.CallGraph); err != nil {
		return fmt.Errorf("save call graph: %w", err)
	}

	// 保存文件哈希映射
	if err := p.saveGob(filepath.Join(p.storagePath, "filehash.idx.gz"), idx.FileHashes); err != nil {
		return fmt.Errorf("save file hashes: %w", err)
	}

	// 保存元信息
	if err := p.saveMeta(idx.Meta); err != nil {
		return fmt.Errorf("save meta: %w", err)
	}

	return nil
}

// LoadIndex 从磁盘加载索引
func (p *IndexPersistence) LoadIndex() (*Index, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// 检查索引是否存在
	metaPath := filepath.Join(p.storagePath, "meta.json")
	if _, err := os.Stat(metaPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("index not found")
	}

	idx := &Index{}

	// 加载元信息
	meta, err := p.loadMeta()
	if err != nil {
		return nil, fmt.Errorf("load meta: %w", err)
	}
	idx.Meta = meta

	// 加载倒排索引
	idx.InvertedIndex = make(map[string]*PostingList)
	if err := p.loadGob(filepath.Join(p.storagePath, "inverted.idx.gz"), &idx.InvertedIndex); err != nil {
		return nil, fmt.Errorf("load inverted index: %w", err)
	}

	// 加载函数摘要
	idx.FunctionSummaries = make(map[string]*FunctionSummary)
	if err := p.loadGob(filepath.Join(p.storagePath, "functions.idx.gz"), &idx.FunctionSummaries); err != nil {
		return nil, fmt.Errorf("load function summaries: %w", err)
	}

	// 加载符号表
	idx.SymbolTable = make(map[string]*Symbol)
	if err := p.loadGob(filepath.Join(p.storagePath, "symbols.idx.gz"), &idx.SymbolTable); err != nil {
		return nil, fmt.Errorf("load symbol table: %w", err)
	}

	// 加载调用图
	idx.CallGraph = &CallGraph{}
	if err := p.loadGob(filepath.Join(p.storagePath, "callgraph.idx.gz"), idx.CallGraph); err != nil {
		return nil, fmt.Errorf("load call graph: %w", err)
	}

	// 加载文件哈希映射
	idx.FileHashes = make(map[string]string)
	if err := p.loadGob(filepath.Join(p.storagePath, "filehash.idx.gz"), &idx.FileHashes); err != nil {
		return nil, fmt.Errorf("load file hashes: %w", err)
	}

	return idx, nil
}

// IndexExists 检查索引是否存在
func (p *IndexPersistence) IndexExists() bool {
	metaPath := filepath.Join(p.storagePath, "meta.json")
	_, err := os.Stat(metaPath)
	return err == nil
}

// GetMeta 获取索引元信息
func (p *IndexPersistence) GetMeta() (*IndexMeta, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.loadMeta()
}

// DeleteIndex 删除索引
func (p *IndexPersistence) DeleteIndex() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	files := []string{
		"inverted.idx.gz",
		"functions.idx.gz",
		"symbols.idx.gz",
		"callgraph.idx.gz",
		"filehash.idx.gz",
		"meta.json",
	}

	for _, f := range files {
		path := filepath.Join(p.storagePath, f)
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove %s: %w", f, err)
		}
	}

	return nil
}

// saveGob 使用gob编码并压缩保存
func (p *IndexPersistence) saveGob(path string, data interface{}) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	gzWriter := gzip.NewWriter(file)
	defer gzWriter.Close()

	encoder := gob.NewEncoder(gzWriter)
	return encoder.Encode(data)
}

// loadGob 解压缩并使用gob解码加载
func (p *IndexPersistence) loadGob(path string, data interface{}) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzReader.Close()

	decoder := gob.NewDecoder(gzReader)
	return decoder.Decode(data)
}

// saveMeta 保存元信息
func (p *IndexPersistence) saveMeta(meta *IndexMeta) error {
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(p.storagePath, "meta.json"), data, 0644)
}

// loadMeta 加载元信息
func (p *IndexPersistence) loadMeta() (*IndexMeta, error) {
	data, err := os.ReadFile(filepath.Join(p.storagePath, "meta.json"))
	if err != nil {
		return nil, err
	}
	var meta IndexMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, err
	}
	return &meta, nil
}

// CalculateSourceHash 计算源码目录的哈希值
func CalculateSourceHash(sourcePath string) (string, error) {
	h := sha256.New()

	err := filepath.Walk(sourcePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		// 只处理C/H文件
		ext := filepath.Ext(path)
		if ext != ".c" && ext != ".h" {
			return nil
		}

		// 计算文件哈希
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		if _, err := io.Copy(h, file); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// Index 完整索引结构
type Index struct {
	Meta              *IndexMeta
	InvertedIndex     map[string]*PostingList
	FunctionSummaries map[string]*FunctionSummary
	SymbolTable       map[string]*Symbol
	CallGraph         *CallGraph
	FileHashes        map[string]string
}

// SymbolInfo 符号详细信息
type SymbolInfo struct {
	Name      string
	Type      string // function, struct, typedef, macro, variable
	File      string
	Line      int
	Scope     string
	Signature string
}
