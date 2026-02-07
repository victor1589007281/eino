package router

import (
	"testing"

	"github.com/cloudwego/eino/vdocstool/tools/ocr"
)

func TestNewSceneClassifier(t *testing.T) {
	classifier := NewSceneClassifier()
	if classifier == nil {
		t.Fatal("expected classifier")
	}
}

func TestSceneClassifierClassify(t *testing.T) {
	classifier := NewSceneClassifier()

	tests := []struct {
		name     string
		quality  *ocr.ImageQuality
		expected ocr.SceneType
	}{
		{
			name:     "nil quality",
			quality:  nil,
			expected: ocr.SceneGeneral,
		},
		{
			name: "high contrast and sharpness - print",
			quality: &ocr.ImageQuality{
				Contrast:  0.8,
				Sharpness: 0.8,
			},
			expected: ocr.ScenePrint,
		},
		{
			name: "low contrast medium sharpness - handwriting",
			quality: &ocr.ImageQuality{
				Contrast:  0.3,
				Sharpness: 0.5,
			},
			expected: ocr.SceneHandwriting,
		},
		{
			name: "high brightness high score - document",
			quality: &ocr.ImageQuality{
				Brightness: 0.7,
				Score:      0.8,
			},
			expected: ocr.SceneDocument,
		},
		{
			name: "medium values - general",
			quality: &ocr.ImageQuality{
				Contrast:   0.5,
				Sharpness:  0.5,
				Brightness: 0.5,
				Score:      0.5,
			},
			expected: ocr.SceneGeneral,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := classifier.Classify(tc.quality)
			if result != tc.expected {
				t.Errorf("Classify() = %s, expected %s", result, tc.expected)
			}
		})
	}
}

func TestSceneClassifierClassifyWithHints(t *testing.T) {
	classifier := NewSceneClassifier()

	tests := []struct {
		hints    []string
		expected ocr.SceneType
	}{
		{[]string{"发票"}, ocr.SceneInvoice},
		{[]string{"invoice"}, ocr.SceneInvoice},
		{[]string{"身份证"}, ocr.SceneIDCard},
		{[]string{"表格"}, ocr.SceneTable},
		{[]string{"手写"}, ocr.SceneHandwriting},
		{[]string{"文档"}, ocr.SceneDocument},
		{[]string{"unknown"}, ocr.SceneGeneral}, // 回退到基于质量的分类
	}

	for _, tc := range tests {
		result := classifier.ClassifyWithHints(nil, tc.hints)
		if result != tc.expected {
			t.Errorf("ClassifyWithHints(%v) = %s, expected %s", tc.hints, result, tc.expected)
		}
	}
}

func TestSceneClassifierMatchHint(t *testing.T) {
	classifier := NewSceneClassifier()

	tests := []struct {
		hint     string
		expected ocr.SceneType
	}{
		{"发票", ocr.SceneInvoice},
		{"票据", ocr.SceneInvoice},
		{"收据", ocr.SceneInvoice},
		{"invoice", ocr.SceneInvoice},
		{"身份证", ocr.SceneIDCard},
		{"idcard", ocr.SceneIDCard},
		{"证件", ocr.SceneIDCard},
		{"驾照", ocr.SceneIDCard},
		{"表格", ocr.SceneTable},
		{"table", ocr.SceneTable},
		{"excel", ocr.SceneTable},
		{"手写", ocr.SceneHandwriting},
		{"handwriting", ocr.SceneHandwriting},
		{"文档", ocr.SceneDocument},
		{"document", ocr.SceneDocument},
		{"pdf", ocr.SceneDocument},
		{"打印", ocr.ScenePrint},
		{"print", ocr.ScenePrint},
		{"unknown_keyword", ""},
	}

	for _, tc := range tests {
		result := classifier.matchHint(tc.hint)
		if result != tc.expected {
			t.Errorf("matchHint(%s) = %s, expected %s", tc.hint, result, tc.expected)
		}
	}
}

func TestSceneClassifierClassifyByAspectRatio(t *testing.T) {
	classifier := NewSceneClassifier()

	tests := []struct {
		width    int
		height   int
		expected ocr.SceneType
	}{
		{0, 0, ocr.SceneGeneral},         // 无效尺寸
		{210, 297, ocr.SceneDocument},    // A4 纵向 (约 0.707)
		{297, 210, ocr.SceneDocument},    // A4 横向 (约 1.414)
		{856, 540, ocr.SceneIDCard},      // 身份证比例 (约 1.585)
		{90, 50, ocr.SceneIDCard},        // 名片比例 (约 1.8)
		{100, 100, ocr.SceneGeneral},     // 1:1 正方形
	}

	for _, tc := range tests {
		result := classifier.ClassifyByAspectRatio(tc.width, tc.height)
		if result != tc.expected {
			t.Errorf("ClassifyByAspectRatio(%d, %d) = %s, expected %s",
				tc.width, tc.height, result, tc.expected)
		}
	}
}

func TestSceneClassifierClassifyByTextDensity(t *testing.T) {
	classifier := NewSceneClassifier()

	// 根据实际实现调整测试预期值
	// density = textCount / imageArea * 10000
	// density > 5 -> Document
	// density > 2 && <= 5 -> Table
	// density < 1 -> IDCard
	// else -> General

	tests := []struct {
		textCount int
		imageArea int
		expected  ocr.SceneType
	}{
		{0, 0, ocr.SceneGeneral},           // 无效面积
		{100, 10000, ocr.SceneDocument},    // 高密度 (100 per 10k pixels = 100) > 5
		{50, 10000, ocr.SceneDocument},     // 密度 = 50 > 5 -> Document
		{30, 10000, ocr.SceneDocument},     // 密度 = 30 > 5 -> Document
		{5, 10000, ocr.SceneTable},         // 密度 = 5 = 5 -> Table
		{0, 10000, ocr.SceneIDCard},        // 密度 = 0 < 1 -> IDCard
	}

	for _, tc := range tests {
		result := classifier.ClassifyByTextDensity(tc.textCount, tc.imageArea)
		if result != tc.expected {
			t.Errorf("ClassifyByTextDensity(%d, %d) = %s, expected %s",
				tc.textCount, tc.imageArea, result, tc.expected)
		}
	}
}

func TestToLower(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"HELLO", "hello"},
		{"Hello World", "hello world"},
		{"already lower", "already lower"},
		{"", ""},
		{"123ABC", "123abc"},
	}

	for _, tc := range tests {
		result := toLower(tc.input)
		if result != tc.expected {
			t.Errorf("toLower(%q) = %q, expected %q", tc.input, result, tc.expected)
		}
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		s        string
		substr   string
		expected bool
	}{
		{"hello world", "world", true},
		{"hello world", "hello", true},
		{"hello world", "xyz", false},
		{"", "a", false},
		{"a", "abc", false},
		{"abc", "", true},
	}

	for _, tc := range tests {
		result := contains(tc.s, tc.substr)
		if result != tc.expected {
			t.Errorf("contains(%q, %q) = %v, expected %v", tc.s, tc.substr, result, tc.expected)
		}
	}
}
