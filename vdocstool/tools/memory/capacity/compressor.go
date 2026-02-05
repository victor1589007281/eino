// Package capacity 压缩存储
package capacity

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
)

// Compressor 压缩器
type Compressor struct {
	level int // 压缩级别
}

// NewCompressor 创建压缩器
func NewCompressor() *Compressor {
	return &Compressor{
		level: gzip.DefaultCompression,
	}
}

// SetLevel 设置压缩级别
func (c *Compressor) SetLevel(level int) {
	c.level = level
}

// Compress 压缩数据
func (c *Compressor) Compress(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	writer, err := gzip.NewWriterLevel(&buf, c.level)
	if err != nil {
		return nil, err
	}

	if _, err := writer.Write(data); err != nil {
		writer.Close()
		return nil, err
	}

	if err := writer.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// Decompress 解压数据
func (c *Compressor) Decompress(data []byte) ([]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	return io.ReadAll(reader)
}

// CompressJSON 压缩 JSON 对象
func (c *Compressor) CompressJSON(v interface{}) ([]byte, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return c.Compress(data)
}

// DecompressJSON 解压 JSON 对象
func (c *Compressor) DecompressJSON(data []byte, v interface{}) error {
	decompressed, err := c.Decompress(data)
	if err != nil {
		return err
	}
	return json.Unmarshal(decompressed, v)
}

// CompressionStats 压缩统计
type CompressionStats struct {
	OriginalSize   int64   `json:"original_size"`
	CompressedSize int64   `json:"compressed_size"`
	Ratio          float64 `json:"ratio"` // 压缩比
}

// GetStats 获取压缩统计
func (c *Compressor) GetStats(original, compressed []byte) *CompressionStats {
	originalSize := int64(len(original))
	compressedSize := int64(len(compressed))

	ratio := 0.0
	if originalSize > 0 {
		ratio = float64(compressedSize) / float64(originalSize)
	}

	return &CompressionStats{
		OriginalSize:   originalSize,
		CompressedSize: compressedSize,
		Ratio:          ratio,
	}
}

// ShouldCompress 判断是否应该压缩
func ShouldCompress(data []byte) bool {
	// 小于 1KB 不压缩
	return len(data) >= 1024
}

// EstimateCompressedSize 估算压缩后大小
func EstimateCompressedSize(originalSize int64) int64 {
	// 假设平均压缩比为 0.3
	return int64(float64(originalSize) * 0.3)
}
