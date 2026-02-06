// Package io 批量IO操作
package io

import (
	"context"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
)

// BatchReader 批量读取器
type BatchReader struct {
	// 配置
	maxBatchSize int
	timeout      time.Duration
}

// NewBatchReader 创建批量读取器
func NewBatchReader(maxBatchSize int, timeout time.Duration) *BatchReader {
	return &BatchReader{
		maxBatchSize: maxBatchSize,
		timeout:      timeout,
	}
}

// BatchReadRequest 批量读取请求
type BatchReadRequest struct {
	L1Keys []L1ReadKey
	L2Keys []L2ReadKey
	L3Keys []L3ReadKey
}

// L1ReadKey L1 读取键
type L1ReadKey struct {
	SessionID string
	MessageID string
}

// L2ReadKey L2 读取键
type L2ReadKey struct {
	CapsuleID string
	Fields    []string
}

// L3ReadKey L3 读取键
type L3ReadKey struct {
	ArchiveID string
	ChunkIDs  []string
}

// BatchReadResult 批量读取结果
type BatchReadResult struct {
	L1Results map[string]*Message
	L2Results map[string]*Capsule
	L3Results map[string]*ArchiveChunk
	Errors    []error
}

// Message 消息
type Message struct {
	ID        string
	SessionID string
	Content   string
	Role      string
	Timestamp time.Time
	TokenCount int
}

// Capsule 胶囊
type Capsule struct {
	ID        string
	SessionID string
	Title     string
	Summary   string
	Messages  []*Message
}

// ArchiveChunk 归档块
type ArchiveChunk struct {
	ID      string
	Content string
}

// L1Reader L1 读取器接口
type L1Reader interface {
	MGet(ctx context.Context, keys []string) (map[string][]byte, error)
}

// L2Reader L2 读取器接口
type L2Reader interface {
	BatchGet(ctx context.Context, ids []string) ([]*Capsule, error)
}

// L3Reader L3 读取器接口
type L3Reader interface {
	BatchFetch(ctx context.Context, archiveID string, chunkIDs []string) ([]*ArchiveChunk, error)
}

// BatchRead 批量读取
func (r *BatchReader) BatchRead(ctx context.Context, req *BatchReadRequest, l1 L1Reader, l2 L2Reader, l3 L3Reader) (*BatchReadResult, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	result := &BatchReadResult{
		L1Results: make(map[string]*Message),
		L2Results: make(map[string]*Capsule),
		L3Results: make(map[string]*ArchiveChunk),
	}

	g, ctx := errgroup.WithContext(ctx)

	// L1 批量读取
	if len(req.L1Keys) > 0 && l1 != nil {
		g.Go(func() error {
			keys := make([]string, len(req.L1Keys))
			for i, k := range req.L1Keys {
				keys[i] = k.SessionID + ":" + k.MessageID
			}

			data, err := l1.MGet(ctx, keys)
			if err != nil {
				return err
			}

			// 解析结果
			for key, value := range data {
				if value != nil {
					msg := &Message{} // 简化：实际需要反序列化
					result.L1Results[key] = msg
				}
			}
			return nil
		})
	}

	// L2 批量读取
	if len(req.L2Keys) > 0 && l2 != nil {
		g.Go(func() error {
			ids := make([]string, len(req.L2Keys))
			for i, k := range req.L2Keys {
				ids[i] = k.CapsuleID
			}

			capsules, err := l2.BatchGet(ctx, ids)
			if err != nil {
				return err
			}

			for _, capsule := range capsules {
				result.L2Results[capsule.ID] = capsule
			}
			return nil
		})
	}

	// L3 批量读取
	if len(req.L3Keys) > 0 && l3 != nil {
		for _, key := range req.L3Keys {
			key := key
			g.Go(func() error {
				chunks, err := l3.BatchFetch(ctx, key.ArchiveID, key.ChunkIDs)
				if err != nil {
					return err
				}

				for _, chunk := range chunks {
					result.L3Results[chunk.ID] = chunk
				}
				return nil
			})
		}
	}

	if err := g.Wait(); err != nil {
		result.Errors = append(result.Errors, err)
	}

	return result, nil
}

// BatchWriter 批量写入器
type BatchWriter struct {
	// 写入缓冲
	buffer *WriteBuffer

	// 配置
	maxBufferSize  int
	flushInterval  time.Duration

	// 写入器
	l1Writer L1Writer

	// 控制
	stopCh chan struct{}
	wg     sync.WaitGroup
}

// WriteBuffer 写入缓冲
type WriteBuffer struct {
	mu       sync.Mutex
	l1Buffer []*L1WriteItem
}

// L1WriteItem L1 写入项
type L1WriteItem struct {
	Key     string
	Value   []byte
	TTL     time.Duration
}

// L1Writer L1 写入器接口
type L1Writer interface {
	Pipeline(ctx context.Context, items []*L1WriteItem) error
}

// NewBatchWriter 创建批量写入器
func NewBatchWriter(maxBufferSize int, flushInterval time.Duration, l1Writer L1Writer) *BatchWriter {
	return &BatchWriter{
		buffer:        &WriteBuffer{},
		maxBufferSize: maxBufferSize,
		flushInterval: flushInterval,
		l1Writer:      l1Writer,
		stopCh:        make(chan struct{}),
	}
}

// Start 启动写入器
func (w *BatchWriter) Start() {
	w.wg.Add(1)
	go w.flushLoop()
}

// Stop 停止写入器
func (w *BatchWriter) Stop() {
	close(w.stopCh)
	w.wg.Wait()
	w.Flush(context.Background())
}

// BufferedWrite 缓冲写入
func (w *BatchWriter) BufferedWrite(item *L1WriteItem) {
	w.buffer.mu.Lock()
	w.buffer.l1Buffer = append(w.buffer.l1Buffer, item)
	bufferSize := len(w.buffer.l1Buffer)
	w.buffer.mu.Unlock()

	// 缓冲区满时触发刷新
	if bufferSize >= w.maxBufferSize {
		go w.Flush(context.Background())
	}
}

// Flush 刷新缓冲区
func (w *BatchWriter) Flush(ctx context.Context) error {
	w.buffer.mu.Lock()
	items := w.buffer.l1Buffer
	w.buffer.l1Buffer = nil
	w.buffer.mu.Unlock()

	if len(items) == 0 {
		return nil
	}

	return w.l1Writer.Pipeline(ctx, items)
}

// flushLoop 定时刷新循环
func (w *BatchWriter) flushLoop() {
	defer w.wg.Done()

	ticker := time.NewTicker(w.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			w.Flush(context.Background())
		case <-w.stopCh:
			return
		}
	}
}

// BatchVectorOps 批量向量操作
type BatchVectorOps struct {
	embedder  Embedder
	batchSize int
}

// Embedder 向量生成器接口
type Embedder interface {
	BatchEmbed(ctx context.Context, texts []string) ([][]float32, error)
}

// NewBatchVectorOps 创建批量向量操作
func NewBatchVectorOps(embedder Embedder, batchSize int) *BatchVectorOps {
	return &BatchVectorOps{
		embedder:  embedder,
		batchSize: batchSize,
	}
}

// BatchEmbed 批量生成向量
func (b *BatchVectorOps) BatchEmbed(ctx context.Context, texts []string) ([][]float32, error) {
	results := make([][]float32, len(texts))

	// 分批处理
	for i := 0; i < len(texts); i += b.batchSize {
		end := i + b.batchSize
		if end > len(texts) {
			end = len(texts)
		}

		batch := texts[i:end]
		embeddings, err := b.embedder.BatchEmbed(ctx, batch)
		if err != nil {
			return nil, err
		}

		copy(results[i:end], embeddings)
	}

	return results, nil
}

// VectorItem 向量项
type VectorItem struct {
	ID           string
	Vector       []float32
	MetadataJSON string
}

// VectorStore 向量存储接口
type VectorStore interface {
	BatchInsert(ctx context.Context, collection string, items []*VectorItem) error
	BatchSearch(ctx context.Context, collection string, vectors [][]float32, topK int) ([][]SearchHit, error)
}

// SearchHit 搜索命中
type SearchHit struct {
	ID    string
	Score float64
}

// BatchSearch 批量向量搜索
func (b *BatchVectorOps) BatchSearch(ctx context.Context, store VectorStore, collection string, queries [][]float32, topK int) ([][]SearchHit, error) {
	return store.BatchSearch(ctx, collection, queries, topK)
}
