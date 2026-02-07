// Package router 场景分类器
package router

import (
	"github.com/cloudwego/eino/vdocstool/tools/ocr"
)

// SceneClassifier 场景分类器
type SceneClassifier struct {
	// 基于规则的简单分类器
	// 后续可以替换为 ML 模型
}

// NewSceneClassifier 创建场景分类器
func NewSceneClassifier() *SceneClassifier {
	return &SceneClassifier{}
}

// Classify 根据图像质量特征分类场景
func (c *SceneClassifier) Classify(quality *ocr.ImageQuality) ocr.SceneType {
	if quality == nil {
		return ocr.SceneGeneral
	}

	// 基于图像特征的简单规则分类
	
	// 高对比度 + 高清晰度 = 打印体
	if quality.Contrast > 0.7 && quality.Sharpness > 0.7 {
		return ocr.ScenePrint
	}

	// 低对比度 + 中等清晰度 = 可能是手写
	if quality.Contrast < 0.4 && quality.Sharpness > 0.3 && quality.Sharpness < 0.6 {
		return ocr.SceneHandwriting
	}

	// 高亮度 + 低噪声 = 文档扫描
	if quality.Brightness > 0.6 && quality.Score > 0.7 {
		return ocr.SceneDocument
	}

	// 默认返回通用场景
	return ocr.SceneGeneral
}

// ClassifyWithHints 带提示词的分类
func (c *SceneClassifier) ClassifyWithHints(quality *ocr.ImageQuality, hints []string) ocr.SceneType {
	// 检查提示词中的关键字
	for _, hint := range hints {
		scene := c.matchHint(hint)
		if scene != "" {
			return scene
		}
	}

	// 回退到基于质量的分类
	return c.Classify(quality)
}

// matchHint 匹配提示词
func (c *SceneClassifier) matchHint(hint string) ocr.SceneType {
	// 关键词映射
	keywords := map[string]ocr.SceneType{
		"发票":   ocr.SceneInvoice,
		"invoice": ocr.SceneInvoice,
		"票据":   ocr.SceneInvoice,
		"收据":   ocr.SceneInvoice,
		
		"身份证":  ocr.SceneIDCard,
		"idcard":  ocr.SceneIDCard,
		"驾照":   ocr.SceneIDCard,
		"证件":   ocr.SceneIDCard,
		
		"表格":   ocr.SceneTable,
		"table":  ocr.SceneTable,
		"excel":  ocr.SceneTable,
		
		"手写":   ocr.SceneHandwriting,
		"handwriting": ocr.SceneHandwriting,
		
		"文档":   ocr.SceneDocument,
		"document": ocr.SceneDocument,
		"pdf":    ocr.SceneDocument,
		
		"打印":   ocr.ScenePrint,
		"print":  ocr.ScenePrint,
	}

	// 简单的关键词匹配
	hintLower := toLower(hint)
	for keyword, scene := range keywords {
		if contains(hintLower, keyword) {
			return scene
		}
	}

	return ""
}

// ClassifyByAspectRatio 根据宽高比分类
func (c *SceneClassifier) ClassifyByAspectRatio(width, height int) ocr.SceneType {
	if width == 0 || height == 0 {
		return ocr.SceneGeneral
	}

	ratio := float64(width) / float64(height)

	// A4 纵向比例约 0.707
	if ratio > 0.65 && ratio < 0.75 {
		return ocr.SceneDocument
	}

	// A4 横向比例约 1.414
	if ratio > 1.35 && ratio < 1.5 {
		return ocr.SceneDocument
	}

	// 身份证比例约 1.585
	if ratio > 1.5 && ratio < 1.7 {
		return ocr.SceneIDCard
	}

	// 名片比例约 1.8
	if ratio > 1.7 && ratio < 1.9 {
		return ocr.SceneIDCard
	}

	return ocr.SceneGeneral
}

// ClassifyByTextDensity 根据文本密度分类
func (c *SceneClassifier) ClassifyByTextDensity(textCount int, imageArea int) ocr.SceneType {
	if imageArea == 0 {
		return ocr.SceneGeneral
	}

	density := float64(textCount) / float64(imageArea) * 10000 // 每万像素的文本块数

	// 高密度文本通常是文档
	if density > 5 {
		return ocr.SceneDocument
	}

	// 中等密度可能是表格
	if density > 2 && density <= 5 {
		return ocr.SceneTable
	}

	// 低密度可能是证件或发票（大块空白+少量关键信息）
	if density < 1 {
		return ocr.SceneIDCard
	}

	return ocr.SceneGeneral
}

// 辅助函数

func toLower(s string) string {
	result := make([]byte, len(s))
	for i, c := range []byte(s) {
		if c >= 'A' && c <= 'Z' {
			result[i] = c + 32
		} else {
			result[i] = c
		}
	}
	return string(result)
}

func contains(s, substr string) bool {
	if len(substr) > len(s) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
