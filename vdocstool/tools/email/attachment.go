// Package email 附件处理
package email

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

// AttachmentManager 附件管理器
type AttachmentManager struct {
	baseDir string
}

// NewAttachmentManager 创建附件管理器
func NewAttachmentManager(baseDir string) *AttachmentManager {
	return &AttachmentManager{
		baseDir: baseDir,
	}
}

// SaveAttachment 保存附件
func (m *AttachmentManager) SaveAttachment(ctx context.Context, data []byte, filename string, emailUID uint32) (*SavedAttachment, error) {
	// 确保目录存在
	if err := os.MkdirAll(m.baseDir, 0755); err != nil {
		return nil, fmt.Errorf("create directory failed: %w", err)
	}

	// 生成唯一文件名
	ext := filepath.Ext(filename)
	baseName := strings.TrimSuffix(filename, ext)
	uniqueName := fmt.Sprintf("%s_%s_%s%s", baseName, time.Now().Format("20060102"), uuid.New().String()[:8], ext)

	// 完整路径
	fullPath := filepath.Join(m.baseDir, uniqueName)

	// 写入文件
	if err := os.WriteFile(fullPath, data, 0644); err != nil {
		return nil, fmt.Errorf("write file failed: %w", err)
	}

	return &SavedAttachment{
		OriginalName: filename,
		SavedName:    uniqueName,
		FilePath:     fullPath,
		Size:         len(data),
		EmailUID:     emailUID,
		SavedAt:      time.Now(),
	}, nil
}

// SavedAttachment 已保存的附件
type SavedAttachment struct {
	OriginalName string    `json:"original_name"`
	SavedName    string    `json:"saved_name"`
	FilePath     string    `json:"file_path"`
	Size         int       `json:"size"`
	EmailUID     uint32    `json:"email_uid"`
	SavedAt      time.Time `json:"saved_at"`
}

// GetAttachment 获取附件
func (m *AttachmentManager) GetAttachment(filename string) ([]byte, error) {
	fullPath := filepath.Join(m.baseDir, filename)
	return os.ReadFile(fullPath)
}

// DeleteAttachment 删除附件
func (m *AttachmentManager) DeleteAttachment(filename string) error {
	fullPath := filepath.Join(m.baseDir, filename)
	return os.Remove(fullPath)
}

// ListAttachments 列出所有附件
func (m *AttachmentManager) ListAttachments() ([]*AttachmentFile, error) {
	entries, err := os.ReadDir(m.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*AttachmentFile{}, nil
		}
		return nil, err
	}

	var files []*AttachmentFile
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		files = append(files, &AttachmentFile{
			Name:     entry.Name(),
			Size:     info.Size(),
			ModTime:  info.ModTime(),
			FilePath: filepath.Join(m.baseDir, entry.Name()),
		})
	}

	return files, nil
}

// AttachmentFile 附件文件信息
type AttachmentFile struct {
	Name     string    `json:"name"`
	Size     int64     `json:"size"`
	ModTime  time.Time `json:"mod_time"`
	FilePath string    `json:"file_path"`
}

// CleanOldAttachments 清理旧附件
func (m *AttachmentManager) CleanOldAttachments(maxAge time.Duration) (int, error) {
	entries, err := os.ReadDir(m.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}

	cleaned := 0
	cutoff := time.Now().Add(-maxAge)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		if info.ModTime().Before(cutoff) {
			fullPath := filepath.Join(m.baseDir, entry.Name())
			if err := os.Remove(fullPath); err == nil {
				cleaned++
			}
		}
	}

	return cleaned, nil
}

// GetContentType 根据文件扩展名获取内容类型
func GetContentType(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	contentTypes := map[string]string{
		".pdf":  "application/pdf",
		".doc":  "application/msword",
		".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		".xls":  "application/vnd.ms-excel",
		".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		".ppt":  "application/vnd.ms-powerpoint",
		".pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation",
		".txt":  "text/plain",
		".csv":  "text/csv",
		".html": "text/html",
		".jpg":  "image/jpeg",
		".jpeg": "image/jpeg",
		".png":  "image/png",
		".gif":  "image/gif",
		".zip":  "application/zip",
		".rar":  "application/x-rar-compressed",
		".7z":   "application/x-7z-compressed",
		".mp3":  "audio/mpeg",
		".mp4":  "video/mp4",
		".ics":  "text/calendar",
	}

	if ct, ok := contentTypes[ext]; ok {
		return ct
	}
	return "application/octet-stream"
}

// IsDocumentType 检查是否为文档类型
func IsDocumentType(contentType string) bool {
	docTypes := []string{
		"application/pdf",
		"application/msword",
		"application/vnd.openxmlformats-officedocument",
		"text/plain",
		"text/csv",
	}

	for _, dt := range docTypes {
		if strings.HasPrefix(contentType, dt) {
			return true
		}
	}
	return false
}

// IsImageType 检查是否为图片类型
func IsImageType(contentType string) bool {
	return strings.HasPrefix(contentType, "image/")
}

// IsCalendarType 检查是否为日历文件
func IsCalendarType(contentType string) bool {
	return contentType == "text/calendar" || strings.HasSuffix(contentType, ".ics")
}
