// Package ocr 图像预处理模块
package ocr

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/image/draw"
)

// Preprocessor 图像预处理器
type Preprocessor struct {
	config *PreprocessConfig
}

// PreprocessConfig 预处理配置
type PreprocessConfig struct {
	EnableGrayscale  bool    `json:"enable_grayscale"`   // 转灰度
	EnableBinarize   bool    `json:"enable_binarize"`    // 二值化
	EnableDenoise    bool    `json:"enable_denoise"`     // 降噪
	EnableDeskew     bool    `json:"enable_deskew"`      // 校正倾斜
	BinarizeThreshold int    `json:"binarize_threshold"` // 二值化阈值 (0-255)
	MaxWidth          int    `json:"max_width"`          // 最大宽度 (缩放)
	MaxHeight         int    `json:"max_height"`         // 最大高度 (缩放)
	TargetDPI         int    `json:"target_dpi"`         // 目标DPI
	JPEGQuality       int    `json:"jpeg_quality"`       // JPEG质量
}

// DefaultPreprocessConfig 默认预处理配置
func DefaultPreprocessConfig() *PreprocessConfig {
	return &PreprocessConfig{
		EnableGrayscale:   false,
		EnableBinarize:    false,
		EnableDenoise:     false,
		EnableDeskew:      false,
		BinarizeThreshold: 128,
		MaxWidth:          4096,
		MaxHeight:         4096,
		TargetDPI:         300,
		JPEGQuality:       90,
	}
}

// NewPreprocessor 创建预处理器
func NewPreprocessor(config *PreprocessConfig) *Preprocessor {
	if config == nil {
		config = DefaultPreprocessConfig()
	}
	return &Preprocessor{config: config}
}

// ImageData 图像数据
type ImageData struct {
	Image       image.Image
	Width       int
	Height      int
	Format      string // jpeg, png, gif, etc.
	Base64      string
	FilePath    string
	RawBytes    []byte
}

// LoadImage 加载图像 (支持路径、URL、Base64)
func (p *Preprocessor) LoadImage(ctx context.Context, req *OCRRequest) (*ImageData, error) {
	var imgData *ImageData
	var err error

	if req.ImagePath != "" {
		imgData, err = p.loadFromPath(req.ImagePath)
	} else if req.ImageURL != "" {
		imgData, err = p.loadFromURL(ctx, req.ImageURL)
	} else if req.ImageBase64 != "" {
		imgData, err = p.loadFromBase64(req.ImageBase64)
	} else {
		return nil, fmt.Errorf("no image source provided")
	}

	if err != nil {
		return nil, fmt.Errorf("load image failed: %w", err)
	}

	return imgData, nil
}

// loadFromPath 从文件路径加载
func (p *Preprocessor) loadFromPath(path string) (*ImageData, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open file failed: %w", err)
	}
	defer file.Close()

	rawBytes, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("read file failed: %w", err)
	}

	img, format, err := image.Decode(bytes.NewReader(rawBytes))
	if err != nil {
		return nil, fmt.Errorf("decode image failed: %w", err)
	}

	bounds := img.Bounds()
	return &ImageData{
		Image:    img,
		Width:    bounds.Dx(),
		Height:   bounds.Dy(),
		Format:   format,
		FilePath: path,
		RawBytes: rawBytes,
	}, nil
}

// loadFromURL 从URL加载
func (p *Preprocessor) loadFromURL(ctx context.Context, url string) (*ImageData, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch image failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch image failed: status %d", resp.StatusCode)
	}

	rawBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response failed: %w", err)
	}

	img, format, err := image.Decode(bytes.NewReader(rawBytes))
	if err != nil {
		return nil, fmt.Errorf("decode image failed: %w", err)
	}

	bounds := img.Bounds()
	return &ImageData{
		Image:    img,
		Width:    bounds.Dx(),
		Height:   bounds.Dy(),
		Format:   format,
		RawBytes: rawBytes,
	}, nil
}

// loadFromBase64 从Base64加载
func (p *Preprocessor) loadFromBase64(b64 string) (*ImageData, error) {
	// 移除 data:image/...;base64, 前缀
	if idx := strings.Index(b64, ","); idx != -1 {
		b64 = b64[idx+1:]
	}

	rawBytes, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, fmt.Errorf("decode base64 failed: %w", err)
	}

	img, format, err := image.Decode(bytes.NewReader(rawBytes))
	if err != nil {
		return nil, fmt.Errorf("decode image failed: %w", err)
	}

	bounds := img.Bounds()
	return &ImageData{
		Image:    img,
		Width:    bounds.Dx(),
		Height:   bounds.Dy(),
		Format:   format,
		Base64:   b64,
		RawBytes: rawBytes,
	}, nil
}

// Process 执行预处理
func (p *Preprocessor) Process(ctx context.Context, imgData *ImageData) (*ImageData, error) {
	img := imgData.Image

	// 1. 缩放 (如果超出最大尺寸)
	if imgData.Width > p.config.MaxWidth || imgData.Height > p.config.MaxHeight {
		img = p.resize(img, p.config.MaxWidth, p.config.MaxHeight)
	}

	// 2. 灰度转换
	if p.config.EnableGrayscale {
		img = p.toGrayscale(img)
	}

	// 3. 二值化
	if p.config.EnableBinarize {
		img = p.binarize(img, p.config.BinarizeThreshold)
	}

	// 4. 简单降噪 (中值滤波的简化版)
	if p.config.EnableDenoise {
		img = p.denoise(img)
	}

	// 编码为字节
	bounds := img.Bounds()
	var buf bytes.Buffer
	format := imgData.Format
	
	switch strings.ToLower(format) {
	case "png":
		err := png.Encode(&buf, img)
		if err != nil {
			return nil, fmt.Errorf("encode png failed: %w", err)
		}
	default:
		format = "jpeg"
		err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: p.config.JPEGQuality})
		if err != nil {
			return nil, fmt.Errorf("encode jpeg failed: %w", err)
		}
	}

	rawBytes := buf.Bytes()
	return &ImageData{
		Image:    img,
		Width:    bounds.Dx(),
		Height:   bounds.Dy(),
		Format:   format,
		RawBytes: rawBytes,
		Base64:   base64.StdEncoding.EncodeToString(rawBytes),
	}, nil
}

// resize 等比缩放
func (p *Preprocessor) resize(img image.Image, maxWidth, maxHeight int) image.Image {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()

	// 计算缩放比例
	ratio := 1.0
	if w > maxWidth {
		ratio = float64(maxWidth) / float64(w)
	}
	if float64(h)*ratio > float64(maxHeight) {
		ratio = float64(maxHeight) / float64(h)
	}

	if ratio >= 1.0 {
		return img
	}

	newW := int(float64(w) * ratio)
	newH := int(float64(h) * ratio)

	dst := image.NewRGBA(image.Rect(0, 0, newW, newH))
	draw.BiLinear.Scale(dst, dst.Bounds(), img, bounds, draw.Over, nil)
	return dst
}

// toGrayscale 转灰度
func (p *Preprocessor) toGrayscale(img image.Image) image.Image {
	bounds := img.Bounds()
	gray := image.NewGray(bounds)

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			c := img.At(x, y)
			gray.Set(x, y, color.GrayModel.Convert(c))
		}
	}

	return gray
}

// binarize 二值化
func (p *Preprocessor) binarize(img image.Image, threshold int) image.Image {
	bounds := img.Bounds()
	binary := image.NewGray(bounds)

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			c := color.GrayModel.Convert(img.At(x, y)).(color.Gray)
			if int(c.Y) > threshold {
				binary.SetGray(x, y, color.Gray{255})
			} else {
				binary.SetGray(x, y, color.Gray{0})
			}
		}
	}

	return binary
}

// denoise 简单降噪 (3x3 中值滤波)
func (p *Preprocessor) denoise(img image.Image) image.Image {
	bounds := img.Bounds()
	result := image.NewRGBA(bounds)

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			// 边界像素直接复制
			if x == bounds.Min.X || x == bounds.Max.X-1 ||
				y == bounds.Min.Y || y == bounds.Max.Y-1 {
				result.Set(x, y, img.At(x, y))
				continue
			}

			// 收集 3x3 邻域像素
			var rs, gs, bs []uint32
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					r, g, b, _ := img.At(x+dx, y+dy).RGBA()
					rs = append(rs, r>>8)
					gs = append(gs, g>>8)
					bs = append(bs, b>>8)
				}
			}

			// 取中值
			result.Set(x, y, color.RGBA{
				R: uint8(median(rs)),
				G: uint8(median(gs)),
				B: uint8(median(bs)),
				A: 255,
			})
		}
	}

	return result
}

// median 计算中值
func median(values []uint32) uint32 {
	n := len(values)
	if n == 0 {
		return 0
	}

	// 简单排序
	for i := 0; i < n-1; i++ {
		for j := i + 1; j < n; j++ {
			if values[i] > values[j] {
				values[i], values[j] = values[j], values[i]
			}
		}
	}

	return values[n/2]
}

// AssessQuality 评估图片质量
func (p *Preprocessor) AssessQuality(imgData *ImageData) *ImageQuality {
	img := imgData.Image
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()

	quality := &ImageQuality{}

	// 采样分析 (每隔10个像素采样)
	var totalBrightness float64
	var brightnesses []float64
	var gradients []float64
	sampleCount := 0

	step := 10
	for y := bounds.Min.Y; y < bounds.Max.Y; y += step {
		for x := bounds.Min.X; x < bounds.Max.X; x += step {
			c := color.GrayModel.Convert(img.At(x, y)).(color.Gray)
			brightness := float64(c.Y) / 255.0
			totalBrightness += brightness
			brightnesses = append(brightnesses, brightness)
			sampleCount++

			// 计算梯度 (边缘检测的简化)
			if x+step < bounds.Max.X && y+step < bounds.Max.Y {
				c2 := color.GrayModel.Convert(img.At(x+step, y)).(color.Gray)
				c3 := color.GrayModel.Convert(img.At(x, y+step)).(color.Gray)
				gx := math.Abs(float64(c.Y) - float64(c2.Y))
				gy := math.Abs(float64(c.Y) - float64(c3.Y))
				gradients = append(gradients, math.Sqrt(gx*gx+gy*gy))
			}
		}
	}

	if sampleCount == 0 {
		return quality
	}

	// 亮度
	quality.Brightness = totalBrightness / float64(sampleCount)

	// 对比度 (标准差)
	avgBrightness := quality.Brightness
	var variance float64
	for _, b := range brightnesses {
		variance += (b - avgBrightness) * (b - avgBrightness)
	}
	quality.Contrast = math.Sqrt(variance / float64(len(brightnesses)))

	// 清晰度 (基于梯度)
	if len(gradients) > 0 {
		var avgGradient float64
		for _, g := range gradients {
			avgGradient += g
		}
		avgGradient /= float64(len(gradients))
		quality.Clarity = math.Min(avgGradient/100.0, 1.0) // 归一化到 0-1
	}

	// 噪声估计 (简化版: 高频分量比例)
	quality.NoiseLevel = 1.0 - quality.Clarity

	// 综合评分
	quality.Score = (quality.Clarity*0.4 + quality.Contrast*0.3 + 
		(1.0-math.Abs(quality.Brightness-0.5)*2)*0.3)

	// 倾斜检测 (简化: 基于图像尺寸比例)
	ratio := float64(w) / float64(h)
	if ratio < 0.5 || ratio > 2.0 {
		quality.IsSkewed = true
	}

	return quality
}

// ToBase64 将图像数据转为 Base64
func (p *Preprocessor) ToBase64(imgData *ImageData) (string, error) {
	if imgData.Base64 != "" {
		return imgData.Base64, nil
	}

	if len(imgData.RawBytes) > 0 {
		return base64.StdEncoding.EncodeToString(imgData.RawBytes), nil
	}

	// 重新编码
	var buf bytes.Buffer
	switch strings.ToLower(imgData.Format) {
	case "png":
		err := png.Encode(&buf, imgData.Image)
		if err != nil {
			return "", err
		}
	default:
		err := jpeg.Encode(&buf, imgData.Image, &jpeg.Options{Quality: p.config.JPEGQuality})
		if err != nil {
			return "", err
		}
	}

	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

// SaveToFile 保存图像到文件
func (p *Preprocessor) SaveToFile(imgData *ImageData, path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create directory failed: %w", err)
	}

	if len(imgData.RawBytes) > 0 {
		return os.WriteFile(path, imgData.RawBytes, 0644)
	}

	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create file failed: %w", err)
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".png":
		return png.Encode(file, imgData.Image)
	default:
		return jpeg.Encode(file, imgData.Image, &jpeg.Options{Quality: p.config.JPEGQuality})
	}
}
