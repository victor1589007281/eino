// Package indexer 提供Linux内核源码的索引功能
package indexer

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

// IndexManager 索引管理器
type IndexManager struct {
	sourcePath  string
	persistence *IndexPersistence
	index       *Index
	builder     *IndexBuilder

	config *IndexConfig
	cron   *cron.Cron

	mu        sync.RWMutex
	building  bool
	lastBuild time.Time

	// 回调
	onBuildStart    func()
	onBuildComplete func(duration time.Duration, err error)
}

// IndexConfig 索引配置
type IndexConfig struct {
	AutoBuild            bool   `json:"auto_build"`
	IncrementalThreshold int    `json:"incremental_threshold"`
	RebuildCron          string `json:"rebuild_cron"`
	Compression          bool   `json:"compression"`
}

// NewIndexManager 创建索引管理器
func NewIndexManager(sourcePath, storagePath string, config *IndexConfig) (*IndexManager, error) {
	persistence, err := NewIndexPersistence(storagePath)
	if err != nil {
		return nil, fmt.Errorf("create persistence: %w", err)
	}

	builder := NewIndexBuilder(sourcePath, storagePath, 4)

	m := &IndexManager{
		sourcePath:  sourcePath,
		persistence: persistence,
		builder:     builder,
		config:      config,
	}

	// 设置定时重建
	if config.RebuildCron != "" {
		m.cron = cron.New()
		_, err := m.cron.AddFunc(config.RebuildCron, func() {
			if err := m.RebuildIndex(context.Background()); err != nil {
				log.Printf("Scheduled index rebuild failed: %v", err)
			}
		})
		if err != nil {
			return nil, fmt.Errorf("setup cron: %w", err)
		}
	}

	return m, nil
}

// Start 启动索引管理器
func (m *IndexManager) Start(ctx context.Context) error {
	// 检查是否存在索引
	if m.persistence.IndexExists() {
		// 加载现有索引
		log.Println("Loading existing index...")
		idx, err := m.persistence.LoadIndex()
		if err != nil {
			log.Printf("Failed to load index: %v, will rebuild", err)
		} else {
			// 验证索引
			if err := m.validateIndex(idx); err != nil {
				log.Printf("Index validation failed: %v, will rebuild", err)
			} else {
				m.mu.Lock()
				m.index = idx
				m.mu.Unlock()
				log.Println("Index loaded successfully")

				// 启动定时任务
				if m.cron != nil {
					m.cron.Start()
				}
				return nil
			}
		}
	}

	// 自动构建索引
	if m.config.AutoBuild {
		log.Println("Building index...")
		if err := m.BuildIndex(ctx); err != nil {
			return fmt.Errorf("build index: %w", err)
		}
	}

	// 启动定时任务
	if m.cron != nil {
		m.cron.Start()
	}

	return nil
}

// Stop 停止索引管理器
func (m *IndexManager) Stop() {
	if m.cron != nil {
		m.cron.Stop()
	}
}

// BuildIndex 构建索引
func (m *IndexManager) BuildIndex(ctx context.Context) error {
	m.mu.Lock()
	if m.building {
		m.mu.Unlock()
		return fmt.Errorf("index building in progress")
	}
	m.building = true
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		m.building = false
		m.mu.Unlock()
	}()

	startTime := time.Now()
	if m.onBuildStart != nil {
		m.onBuildStart()
	}

	// 构建索引
	if err := m.builder.Build(ctx); err != nil {
		if m.onBuildComplete != nil {
			m.onBuildComplete(time.Since(startTime), err)
		}
		return fmt.Errorf("build: %w", err)
	}

	// 从builder获取构建的索引并保存
	idx := m.builder.GetIndex()
	if err := m.persistence.SaveIndex(idx); err != nil {
		if m.onBuildComplete != nil {
			m.onBuildComplete(time.Since(startTime), err)
		}
		return fmt.Errorf("save: %w", err)
	}

	// 更新内存索引
	m.mu.Lock()
	m.index = idx
	m.lastBuild = time.Now()
	m.mu.Unlock()

	duration := time.Since(startTime)
	if m.onBuildComplete != nil {
		m.onBuildComplete(duration, nil)
	}

	log.Printf("Index built successfully in %v", duration)
	return nil
}

// RebuildIndex 重建索引
func (m *IndexManager) RebuildIndex(ctx context.Context) error {
	// 删除旧索引
	if err := m.persistence.DeleteIndex(); err != nil {
		return fmt.Errorf("delete old index: %w", err)
	}

	// 构建新索引
	return m.BuildIndex(ctx)
}

// IncrementalUpdate 增量更新索引
func (m *IndexManager) IncrementalUpdate(ctx context.Context, changedFiles []string) error {
	m.mu.Lock()
	if m.building {
		m.mu.Unlock()
		return fmt.Errorf("index building in progress")
	}

	// 检查变更量是否超过阈值
	if len(changedFiles) >= m.config.IncrementalThreshold {
		m.mu.Unlock()
		log.Printf("Changed files (%d) exceed threshold (%d), performing full rebuild",
			len(changedFiles), m.config.IncrementalThreshold)
		return m.RebuildIndex(ctx)
	}

	m.building = true
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		m.building = false
		m.mu.Unlock()
	}()

	// 执行增量更新 (简化实现：直接重建)
	if err := m.builder.Build(ctx); err != nil {
		return fmt.Errorf("incremental update: %w", err)
	}
	m.index = m.builder.GetIndex()

	// 保存更新后的索引
	if err := m.persistence.SaveIndex(m.index); err != nil {
		return fmt.Errorf("save: %w", err)
	}

	// 更新元信息
	m.index.Meta.LastUpdated = time.Now()

	log.Printf("Index incrementally updated for %d files", len(changedFiles))
	return nil
}

// GetIndex 获取当前索引
func (m *IndexManager) GetIndex() *Index {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.index
}

// IsReady 检查索引是否就绪
func (m *IndexManager) IsReady() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.index != nil
}

// IsBuilding 检查是否正在构建
func (m *IndexManager) IsBuilding() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.building
}

// GetMeta 获取索引元信息
func (m *IndexManager) GetMeta() *IndexMeta {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.index == nil {
		return nil
	}
	return m.index.Meta
}

// validateIndex 验证索引
func (m *IndexManager) validateIndex(idx *Index) error {
	if idx.Meta == nil {
		return fmt.Errorf("missing meta")
	}

	// 检查索引版本
	if idx.Meta.IndexerVersion != IndexerVersion {
		return fmt.Errorf("version mismatch: %s != %s", idx.Meta.IndexerVersion, IndexerVersion)
	}

	// 检查源码哈希
	currentHash, err := CalculateSourceHash(m.sourcePath)
	if err != nil {
		return fmt.Errorf("calculate source hash: %w", err)
	}

	if idx.Meta.SourceHash != currentHash {
		return fmt.Errorf("source hash mismatch")
	}

	return nil
}

// SetBuildCallbacks 设置构建回调
func (m *IndexManager) SetBuildCallbacks(onStart func(), onComplete func(time.Duration, error)) {
	m.onBuildStart = onStart
	m.onBuildComplete = onComplete
}

// GetStats 获取索引统计
func (m *IndexManager) GetStats() *IndexStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.index == nil {
		return nil
	}

	return &IndexStats{
		FileCount:      m.index.Meta.FileCount,
		TotalSize:      m.index.Meta.TotalSize,
		TermCount:      len(m.index.InvertedIndex),
		FunctionCount:  len(m.index.FunctionSummaries),
		SymbolCount:    len(m.index.SymbolTable),
		LastBuild:      m.lastBuild,
		IndexerVersion: m.index.Meta.IndexerVersion,
	}
}

// IndexStats 索引统计
type IndexStats struct {
	FileCount      int       `json:"file_count"`
	TotalSize      int64     `json:"total_size"`
	TermCount      int       `json:"term_count"`
	FunctionCount  int       `json:"function_count"`
	SymbolCount    int       `json:"symbol_count"`
	LastBuild      time.Time `json:"last_build"`
	IndexerVersion string    `json:"indexer_version"`
}

// IndexerVersion 索引器版本
const IndexerVersion = "1.0.0"
