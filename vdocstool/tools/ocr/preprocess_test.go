package ocr

import (
	"context"
	"image"
	"image/color"
	"testing"
)

func TestDefaultPreprocessConfig(t *testing.T) {
	config := DefaultPreprocessConfig()

	if config.EnableGrayscale {
		t.Error("EnableGrayscale should be false by default")
	}

	if config.EnableBinarize {
		t.Error("EnableBinarize should be false by default")
	}

	if config.MaxWidth != 4096 {
		t.Errorf("expected MaxWidth=4096, got %d", config.MaxWidth)
	}

	if config.MaxHeight != 4096 {
		t.Errorf("expected MaxHeight=4096, got %d", config.MaxHeight)
	}

	if config.JPEGQuality != 90 {
		t.Errorf("expected JPEGQuality=90, got %d", config.JPEGQuality)
	}
}

func TestNewPreprocessor(t *testing.T) {
	// 测试默认配置
	p := NewPreprocessor(nil)
	if p == nil {
		t.Fatal("NewPreprocessor returned nil")
	}

	// 测试自定义配置
	config := &PreprocessConfig{
		EnableGrayscale: true,
		MaxWidth:        2048,
	}
	p = NewPreprocessor(config)
	if p.config.MaxWidth != 2048 {
		t.Errorf("expected MaxWidth=2048, got %d", p.config.MaxWidth)
	}
}

func TestLoadImageNoSource(t *testing.T) {
	p := NewPreprocessor(nil)
	ctx := context.Background()

	_, err := p.LoadImage(ctx, &OCRRequest{})
	if err == nil {
		t.Error("expected error for empty request")
	}
}

func TestLoadImageInvalidPath(t *testing.T) {
	p := NewPreprocessor(nil)
	ctx := context.Background()

	_, err := p.LoadImage(ctx, &OCRRequest{ImagePath: "/nonexistent/path/image.jpg"})
	if err == nil {
		t.Error("expected error for invalid path")
	}
}

func TestLoadImageInvalidBase64(t *testing.T) {
	p := NewPreprocessor(nil)
	ctx := context.Background()

	_, err := p.LoadImage(ctx, &OCRRequest{ImageBase64: "not-valid-base64!@#$"})
	if err == nil {
		t.Error("expected error for invalid base64")
	}
}

func TestToGrayscale(t *testing.T) {
	p := NewPreprocessor(nil)

	// 创建测试图像
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			img.Set(x, y, color.RGBA{R: 255, G: 128, B: 64, A: 255})
		}
	}

	gray := p.toGrayscale(img)
	grayImg, ok := gray.(*image.Gray)
	if !ok {
		t.Fatal("expected *image.Gray")
	}

	// 检查是否是灰度图
	bounds := grayImg.Bounds()
	if bounds.Dx() != 100 || bounds.Dy() != 100 {
		t.Errorf("expected 100x100, got %dx%d", bounds.Dx(), bounds.Dy())
	}
}

func TestBinarize(t *testing.T) {
	p := NewPreprocessor(nil)

	// 创建测试图像
	img := image.NewGray(image.Rect(0, 0, 10, 10))
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			if x < 5 {
				img.SetGray(x, y, color.Gray{50}) // 暗
			} else {
				img.SetGray(x, y, color.Gray{200}) // 亮
			}
		}
	}

	binary := p.binarize(img, 128)
	binaryImg, ok := binary.(*image.Gray)
	if !ok {
		t.Fatal("expected *image.Gray")
	}

	// 检查二值化结果
	darkPixel := binaryImg.GrayAt(0, 0)
	lightPixel := binaryImg.GrayAt(9, 0)

	if darkPixel.Y != 0 {
		t.Errorf("expected dark pixel=0, got %d", darkPixel.Y)
	}
	if lightPixel.Y != 255 {
		t.Errorf("expected light pixel=255, got %d", lightPixel.Y)
	}
}

func TestResize(t *testing.T) {
	p := NewPreprocessor(nil)

	// 创建大图像
	img := image.NewRGBA(image.Rect(0, 0, 8000, 6000))

	resized := p.resize(img, 4096, 4096)
	bounds := resized.Bounds()

	if bounds.Dx() > 4096 || bounds.Dy() > 4096 {
		t.Errorf("resized image too large: %dx%d", bounds.Dx(), bounds.Dy())
	}

	// 检查比例保持
	originalRatio := float64(8000) / float64(6000)
	resizedRatio := float64(bounds.Dx()) / float64(bounds.Dy())
	diff := originalRatio - resizedRatio
	if diff < -0.1 || diff > 0.1 {
		t.Errorf("aspect ratio not preserved: original=%f, resized=%f", originalRatio, resizedRatio)
	}
}

func TestResizeNoChange(t *testing.T) {
	p := NewPreprocessor(nil)

	// 创建小图像
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))

	resized := p.resize(img, 4096, 4096)
	bounds := resized.Bounds()

	// 小于限制的图像不应该被缩放
	if bounds.Dx() != 100 || bounds.Dy() != 100 {
		t.Errorf("small image should not be resized: %dx%d", bounds.Dx(), bounds.Dy())
	}
}

func TestDenoise(t *testing.T) {
	p := NewPreprocessor(nil)

	// 创建测试图像
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			img.Set(x, y, color.RGBA{R: 128, G: 128, B: 128, A: 255})
		}
	}
	// 添加噪点
	img.Set(5, 5, color.RGBA{R: 255, G: 0, B: 0, A: 255})

	denoised := p.denoise(img)
	
	if denoised == nil {
		t.Fatal("denoise returned nil")
	}

	bounds := denoised.Bounds()
	if bounds.Dx() != 10 || bounds.Dy() != 10 {
		t.Errorf("expected 10x10, got %dx%d", bounds.Dx(), bounds.Dy())
	}
}

func TestMedian(t *testing.T) {
	tests := []struct {
		input    []uint32
		expected uint32
	}{
		{[]uint32{1, 2, 3, 4, 5}, 3},
		{[]uint32{5, 1, 3, 2, 4}, 3},
		{[]uint32{1, 1, 1}, 1},
		{[]uint32{100}, 100},
		{[]uint32{}, 0},
	}

	for _, tt := range tests {
		// 复制切片避免修改
		input := make([]uint32, len(tt.input))
		copy(input, tt.input)
		
		result := median(input)
		if result != tt.expected {
			t.Errorf("median(%v): expected %d, got %d", tt.input, tt.expected, result)
		}
	}
}

func TestAssessQuality(t *testing.T) {
	p := NewPreprocessor(nil)

	// 创建测试图像
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			// 创建渐变图像
			gray := uint8(x * 255 / 100)
			img.Set(x, y, color.RGBA{R: gray, G: gray, B: gray, A: 255})
		}
	}

	imgData := &ImageData{
		Image:  img,
		Width:  100,
		Height: 100,
	}

	quality := p.AssessQuality(imgData)

	if quality == nil {
		t.Fatal("AssessQuality returned nil")
	}

	// 检查质量指标范围
	if quality.Score < 0 || quality.Score > 1 {
		t.Errorf("Score out of range: %f", quality.Score)
	}

	if quality.Clarity < 0 || quality.Clarity > 1 {
		t.Errorf("Clarity out of range: %f", quality.Clarity)
	}

	if quality.Brightness < 0 || quality.Brightness > 1 {
		t.Errorf("Brightness out of range: %f", quality.Brightness)
	}

	if quality.Contrast < 0 || quality.Contrast > 1 {
		t.Errorf("Contrast out of range: %f", quality.Contrast)
	}
}

func TestAssessQualityEmptyImage(t *testing.T) {
	p := NewPreprocessor(nil)

	img := image.NewRGBA(image.Rect(0, 0, 0, 0))
	imgData := &ImageData{
		Image:  img,
		Width:  0,
		Height: 0,
	}

	quality := p.AssessQuality(imgData)

	if quality == nil {
		t.Fatal("AssessQuality returned nil for empty image")
	}
}

func TestToBase64(t *testing.T) {
	p := NewPreprocessor(nil)

	// 创建测试图像
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			img.Set(x, y, color.RGBA{R: 128, G: 128, B: 128, A: 255})
		}
	}

	imgData := &ImageData{
		Image:  img,
		Width:  10,
		Height: 10,
		Format: "jpeg",
	}

	b64, err := p.ToBase64(imgData)
	if err != nil {
		t.Fatalf("ToBase64 failed: %v", err)
	}

	if b64 == "" {
		t.Error("ToBase64 returned empty string")
	}
}

func TestToBase64WithExisting(t *testing.T) {
	p := NewPreprocessor(nil)

	imgData := &ImageData{
		Base64: "existing_base64_string",
	}

	b64, err := p.ToBase64(imgData)
	if err != nil {
		t.Fatalf("ToBase64 failed: %v", err)
	}

	if b64 != "existing_base64_string" {
		t.Errorf("expected existing base64, got %s", b64)
	}
}

func TestToBase64WithRawBytes(t *testing.T) {
	p := NewPreprocessor(nil)

	imgData := &ImageData{
		RawBytes: []byte{0x89, 0x50, 0x4E, 0x47}, // PNG magic bytes
	}

	b64, err := p.ToBase64(imgData)
	if err != nil {
		t.Fatalf("ToBase64 failed: %v", err)
	}

	if b64 == "" {
		t.Error("ToBase64 returned empty string")
	}
}

func TestProcess(t *testing.T) {
	config := &PreprocessConfig{
		EnableGrayscale: true,
		MaxWidth:        50,
		MaxHeight:       50,
		JPEGQuality:     80,
	}
	p := NewPreprocessor(config)

	// 创建测试图像
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			img.Set(x, y, color.RGBA{R: 255, G: 128, B: 64, A: 255})
		}
	}

	imgData := &ImageData{
		Image:  img,
		Width:  100,
		Height: 100,
		Format: "jpeg",
	}

	ctx := context.Background()
	processed, err := p.Process(ctx, imgData)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	// 检查缩放
	if processed.Width > 50 || processed.Height > 50 {
		t.Errorf("image should be resized to max 50x50, got %dx%d", processed.Width, processed.Height)
	}

	// 检查有输出
	if len(processed.RawBytes) == 0 {
		t.Error("processed image has no raw bytes")
	}

	if processed.Base64 == "" {
		t.Error("processed image has no base64")
	}
}

func TestProcessPNG(t *testing.T) {
	config := DefaultPreprocessConfig()
	p := NewPreprocessor(config)

	// 创建测试图像
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	imgData := &ImageData{
		Image:  img,
		Width:  10,
		Height: 10,
		Format: "png",
	}

	ctx := context.Background()
	processed, err := p.Process(ctx, imgData)
	if err != nil {
		t.Fatalf("Process PNG failed: %v", err)
	}

	if processed.Format != "png" {
		t.Errorf("expected format=png, got %s", processed.Format)
	}
}

func TestImageDataFields(t *testing.T) {
	imgData := &ImageData{
		Width:    100,
		Height:   200,
		Format:   "jpeg",
		FilePath: "/path/to/image.jpg",
	}

	if imgData.Width != 100 {
		t.Errorf("expected Width=100, got %d", imgData.Width)
	}

	if imgData.Height != 200 {
		t.Errorf("expected Height=200, got %d", imgData.Height)
	}

	if imgData.Format != "jpeg" {
		t.Errorf("expected Format=jpeg, got %s", imgData.Format)
	}
}

func TestSkewDetection(t *testing.T) {
	p := NewPreprocessor(nil)

	// 测试正常比例图像
	normalImg := &ImageData{
		Image:  image.NewRGBA(image.Rect(0, 0, 100, 100)),
		Width:  100,
		Height: 100,
	}
	quality := p.AssessQuality(normalImg)
	if quality.IsSkewed {
		t.Error("normal image should not be detected as skewed")
	}

	// 测试极端比例图像
	skewedImg := &ImageData{
		Image:  image.NewRGBA(image.Rect(0, 0, 100, 300)),
		Width:  100,
		Height: 300,
	}
	quality = p.AssessQuality(skewedImg)
	if !quality.IsSkewed {
		t.Error("extreme aspect ratio image should be detected as potentially skewed")
	}
}
