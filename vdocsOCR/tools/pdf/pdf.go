// Package pdf provides PDF processing capabilities.
package pdf

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image/png"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino/vdocsOCR/config"
	"github.com/cloudwego/eino/vdocsOCR/tools/ocr"
)

// PDFProcessor handles PDF processing.
type PDFProcessor struct {
	config     *config.PDFConfig
	ocrManager *ocr.OCRManager
	tempDir    string
	mu         sync.Mutex
}

// NewPDFProcessor creates a new PDF processor.
func NewPDFProcessor(cfg *config.PDFConfig, ocrMgr *ocr.OCRManager) (*PDFProcessor, error) {
	tempDir, err := os.MkdirTemp("", "pdf-ocr-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	return &PDFProcessor{
		config:     cfg,
		ocrManager: ocrMgr,
		tempDir:    tempDir,
	}, nil
}

// PDFResult represents the result of PDF processing.
type PDFResult struct {
	FileName    string          `json:"file_name"`
	PageCount   int             `json:"page_count"`
	Pages       []PageResult    `json:"pages"`
	FullText    string          `json:"full_text"`
	Metadata    *PDFMetadata    `json:"metadata,omitempty"`
	Duration    time.Duration   `json:"duration"`
	ProcessedAt time.Time       `json:"processed_at"`
}

// PageResult represents the result for a single page.
type PageResult struct {
	PageNumber int              `json:"page_number"`
	Text       string           `json:"text"`
	Images     []ExtractedImage `json:"images,omitempty"`
	OCRResult  *ocr.OCRResult   `json:"ocr_result,omitempty"`
}

// ExtractedImage represents an extracted image from PDF.
type ExtractedImage struct {
	Index    int    `json:"index"`
	Data     []byte `json:"-"`
	Format   string `json:"format"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	OCRText  string `json:"ocr_text,omitempty"`
}

// PDFMetadata represents PDF document metadata.
type PDFMetadata struct {
	Title        string `json:"title,omitempty"`
	Author       string `json:"author,omitempty"`
	Subject      string `json:"subject,omitempty"`
	Keywords     string `json:"keywords,omitempty"`
	Creator      string `json:"creator,omitempty"`
	Producer     string `json:"producer,omitempty"`
	CreationDate string `json:"creation_date,omitempty"`
	ModDate      string `json:"mod_date,omitempty"`
	PageCount    int    `json:"page_count"`
	FileSize     int64  `json:"file_size"`
}

// ProcessOptions represents PDF processing options.
type ProcessOptions struct {
	PageRange      string `json:"page_range"`       // e.g., "1-5", "1,3,5", "all"
	ExtractText    bool   `json:"extract_text"`
	ExtractImages  bool   `json:"extract_images"`
	OCRScannedPDF  bool   `json:"ocr_scanned_pdf"`
	DPI            int    `json:"dpi"`
	Language       string `json:"language"`
	MaxConcurrency int    `json:"max_concurrency"`
}

// DefaultProcessOptions returns default processing options.
func DefaultProcessOptions() *ProcessOptions {
	return &ProcessOptions{
		PageRange:      "all",
		ExtractText:    true,
		ExtractImages:  true,
		OCRScannedPDF:  true,
		DPI:            300,
		Language:       "chi_sim+eng",
		MaxConcurrency: 4,
	}
}

// ProcessFile processes a PDF file.
func (p *PDFProcessor) ProcessFile(ctx context.Context, filePath string, opts *ProcessOptions) (*PDFResult, error) {
	start := time.Now()

	if opts == nil {
		opts = DefaultProcessOptions()
	}

	// Validate file
	info, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("file not found: %w", err)
	}

	if info.Size() > p.config.MaxFileSize {
		return nil, fmt.Errorf("file size %d exceeds maximum %d", info.Size(), p.config.MaxFileSize)
	}

	// Get PDF metadata
	metadata, err := p.getMetadata(ctx, filePath)
	if err != nil {
		// Continue even if metadata extraction fails
		metadata = &PDFMetadata{
			FileSize: info.Size(),
		}
	}
	metadata.FileSize = info.Size()

	// Parse page range
	pages, err := p.parsePageRange(opts.PageRange, metadata.PageCount)
	if err != nil {
		return nil, fmt.Errorf("invalid page range: %w", err)
	}

	// Check max pages
	if len(pages) > p.config.MaxPages {
		pages = pages[:p.config.MaxPages]
	}

	result := &PDFResult{
		FileName:    filepath.Base(filePath),
		PageCount:   len(pages),
		Pages:       make([]PageResult, 0, len(pages)),
		Metadata:    metadata,
		ProcessedAt: time.Now(),
	}

	// Process pages
	var textBuilder strings.Builder
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, opts.MaxConcurrency)

	pageResults := make([]PageResult, len(pages))

	for i, pageNum := range pages {
		wg.Add(1)
		go func(idx int, page int) {
			defer wg.Done()

			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}

			pageResult, err := p.processPage(ctx, filePath, page, opts)
			if err != nil {
				pageResult = &PageResult{
					PageNumber: page,
					Text:       fmt.Sprintf("[Error processing page %d: %v]", page, err),
				}
			}

			mu.Lock()
			pageResults[idx] = *pageResult
			mu.Unlock()
		}(i, pageNum)
	}

	wg.Wait()

	// Combine results
	for _, pageResult := range pageResults {
		result.Pages = append(result.Pages, pageResult)
		textBuilder.WriteString(pageResult.Text)
		textBuilder.WriteString("\n\n")
	}

	result.FullText = strings.TrimSpace(textBuilder.String())
	result.Duration = time.Since(start)

	return result, nil
}

// ProcessReader processes PDF data from a reader.
func (p *PDFProcessor) ProcessReader(ctx context.Context, r io.Reader, opts *ProcessOptions) (*PDFResult, error) {
	// Write to temp file
	p.mu.Lock()
	tempFile := filepath.Join(p.tempDir, fmt.Sprintf("pdf_%d.pdf", time.Now().UnixNano()))
	p.mu.Unlock()

	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read PDF data: %w", err)
	}

	if err := os.WriteFile(tempFile, data, 0644); err != nil {
		return nil, fmt.Errorf("failed to write temp file: %w", err)
	}
	defer os.Remove(tempFile)

	return p.ProcessFile(ctx, tempFile, opts)
}

// processPage processes a single page.
func (p *PDFProcessor) processPage(ctx context.Context, filePath string, pageNum int, opts *ProcessOptions) (*PageResult, error) {
	result := &PageResult{
		PageNumber: pageNum,
	}

	// Extract text using pdftotext
	if opts.ExtractText {
		text, err := p.extractText(ctx, filePath, pageNum)
		if err == nil {
			result.Text = text
		}
	}

	// If text is empty or OCR for scanned PDF is enabled, try OCR
	if (result.Text == "" || opts.OCRScannedPDF) && p.ocrManager != nil {
		// Convert page to image
		imageData, err := p.convertPageToImage(ctx, filePath, pageNum, opts.DPI)
		if err == nil {
			// Perform OCR
			ocrResult, err := p.ocrManager.Recognize(ctx, imageData, &ocr.RecognizeOptions{
				Language: opts.Language,
				DPI:      opts.DPI,
			})
			if err == nil {
				result.OCRResult = ocrResult
				if result.Text == "" {
					result.Text = ocrResult.Text
				}
			}
		}
	}

	// Extract images
	if opts.ExtractImages {
		images, err := p.extractImages(ctx, filePath, pageNum)
		if err == nil {
			result.Images = images

			// OCR images
			for i := range result.Images {
				if p.ocrManager != nil && len(result.Images[i].Data) > 0 {
					ocrResult, err := p.ocrManager.Recognize(ctx, result.Images[i].Data, &ocr.RecognizeOptions{
						Language: opts.Language,
					})
					if err == nil {
						result.Images[i].OCRText = ocrResult.Text
					}
				}
			}
		}
	}

	return result, nil
}

// extractText extracts text from a PDF page using pdftotext.
func (p *PDFProcessor) extractText(ctx context.Context, filePath string, pageNum int) (string, error) {
	// Check if pdftotext is available
	if _, err := exec.LookPath("pdftotext"); err != nil {
		return "", fmt.Errorf("pdftotext not found")
	}

	args := []string{
		"-f", strconv.Itoa(pageNum),
		"-l", strconv.Itoa(pageNum),
		"-layout",
		filePath,
		"-",
	}

	cmd := exec.CommandContext(ctx, "pdftotext", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("pdftotext failed: %w, stderr: %s", err, stderr.String())
	}

	return strings.TrimSpace(stdout.String()), nil
}

// convertPageToImage converts a PDF page to an image.
func (p *PDFProcessor) convertPageToImage(ctx context.Context, filePath string, pageNum int, dpi int) ([]byte, error) {
	// Check if pdftoppm is available
	if _, err := exec.LookPath("pdftoppm"); err != nil {
		return nil, fmt.Errorf("pdftoppm not found")
	}

	// Create temp file for output
	p.mu.Lock()
	outputPrefix := filepath.Join(p.tempDir, fmt.Sprintf("page_%d_%d", pageNum, time.Now().UnixNano()))
	p.mu.Unlock()

	args := []string{
		"-png",
		"-r", strconv.Itoa(dpi),
		"-f", strconv.Itoa(pageNum),
		"-l", strconv.Itoa(pageNum),
		"-singlefile",
		filePath,
		outputPrefix,
	}

	cmd := exec.CommandContext(ctx, "pdftoppm", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("pdftoppm failed: %w, stderr: %s", err, stderr.String())
	}

	// Read the output image
	outputFile := outputPrefix + ".png"
	defer os.Remove(outputFile)

	data, err := os.ReadFile(outputFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read output image: %w", err)
	}

	return data, nil
}

// extractImages extracts images from a PDF page.
func (p *PDFProcessor) extractImages(ctx context.Context, filePath string, pageNum int) ([]ExtractedImage, error) {
	// Check if pdfimages is available
	if _, err := exec.LookPath("pdfimages"); err != nil {
		return nil, fmt.Errorf("pdfimages not found")
	}

	// Create temp directory for output
	p.mu.Lock()
	outputDir := filepath.Join(p.tempDir, fmt.Sprintf("images_%d_%d", pageNum, time.Now().UnixNano()))
	p.mu.Unlock()

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}
	defer os.RemoveAll(outputDir)

	outputPrefix := filepath.Join(outputDir, "image")

	args := []string{
		"-png",
		"-f", strconv.Itoa(pageNum),
		"-l", strconv.Itoa(pageNum),
		filePath,
		outputPrefix,
	}

	cmd := exec.CommandContext(ctx, "pdfimages", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("pdfimages failed: %w, stderr: %s", err, stderr.String())
	}

	// Read extracted images
	var images []ExtractedImage
	files, err := filepath.Glob(filepath.Join(outputDir, "*.png"))
	if err != nil {
		return nil, err
	}

	for i, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}

		// Get image dimensions
		reader := bytes.NewReader(data)
		img, err := png.DecodeConfig(reader)
		if err != nil {
			continue
		}

		images = append(images, ExtractedImage{
			Index:  i,
			Data:   data,
			Format: "png",
			Width:  img.Width,
			Height: img.Height,
		})
	}

	return images, nil
}

// getMetadata extracts PDF metadata.
func (p *PDFProcessor) getMetadata(ctx context.Context, filePath string) (*PDFMetadata, error) {
	// Check if pdfinfo is available
	if _, err := exec.LookPath("pdfinfo"); err != nil {
		return nil, fmt.Errorf("pdfinfo not found")
	}

	cmd := exec.CommandContext(ctx, "pdfinfo", filePath)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("pdfinfo failed: %w, stderr: %s", err, stderr.String())
	}

	metadata := &PDFMetadata{}
	lines := strings.Split(stdout.String(), "\n")
	for _, line := range lines {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "Title":
			metadata.Title = value
		case "Author":
			metadata.Author = value
		case "Subject":
			metadata.Subject = value
		case "Keywords":
			metadata.Keywords = value
		case "Creator":
			metadata.Creator = value
		case "Producer":
			metadata.Producer = value
		case "CreationDate":
			metadata.CreationDate = value
		case "ModDate":
			metadata.ModDate = value
		case "Pages":
			metadata.PageCount, _ = strconv.Atoi(value)
		}
	}

	return metadata, nil
}

// parsePageRange parses a page range string.
func (p *PDFProcessor) parsePageRange(rangeStr string, totalPages int) ([]int, error) {
	if rangeStr == "" || rangeStr == "all" {
		pages := make([]int, totalPages)
		for i := 0; i < totalPages; i++ {
			pages[i] = i + 1
		}
		return pages, nil
	}

	var pages []int
	parts := strings.Split(rangeStr, ",")

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if strings.Contains(part, "-") {
			// Range
			rangeParts := strings.Split(part, "-")
			if len(rangeParts) != 2 {
				return nil, fmt.Errorf("invalid range: %s", part)
			}

			start, err := strconv.Atoi(strings.TrimSpace(rangeParts[0]))
			if err != nil {
				return nil, fmt.Errorf("invalid range start: %s", rangeParts[0])
			}

			end, err := strconv.Atoi(strings.TrimSpace(rangeParts[1]))
			if err != nil {
				return nil, fmt.Errorf("invalid range end: %s", rangeParts[1])
			}

			if start > end || start < 1 || end > totalPages {
				return nil, fmt.Errorf("invalid range: %d-%d (total pages: %d)", start, end, totalPages)
			}

			for i := start; i <= end; i++ {
				pages = append(pages, i)
			}
		} else {
			// Single page
			page, err := strconv.Atoi(part)
			if err != nil {
				return nil, fmt.Errorf("invalid page number: %s", part)
			}

			if page < 1 || page > totalPages {
				return nil, fmt.Errorf("page %d out of range (total pages: %d)", page, totalPages)
			}

			pages = append(pages, page)
		}
	}

	return pages, nil
}

// Close releases resources.
func (p *PDFProcessor) Close() error {
	if p.tempDir != "" {
		return os.RemoveAll(p.tempDir)
	}
	return nil
}

// PDFResultToJSON converts PDF result to JSON.
func PDFResultToJSON(result *PDFResult) (string, error) {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}
