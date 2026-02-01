/*
 * Copyright 2025 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// StyleAnalyzeTool 风格分析工具
type StyleAnalyzeTool struct{}

// StyleAnalyzeParams 风格分析参数
type StyleAnalyzeParams struct {
	Description string `json:"description" description:"用户的图片需求描述"`
	ImageType   string `json:"image_type,omitempty" description:"图片类型"`
}

// StyleAnalyzeResult 风格分析结果
type StyleAnalyzeResult struct {
	Success            bool                `json:"success"`
	RecommendedStyle   string              `json:"recommended_style"`
	RecommendedSize    string              `json:"recommended_size"`
	RecommendedQuality string              `json:"recommended_quality"`
	StyleOptions       []StyleOption       `json:"style_options"`
	ColorPalette       []string            `json:"color_palette"`
	Composition        CompositionSuggestion `json:"composition"`
	Error              string              `json:"error,omitempty"`
}

// StyleOption 风格选项
type StyleOption struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Score       float64 `json:"score"` // 匹配度 0-1
}

// CompositionSuggestion 构图建议
type CompositionSuggestion struct {
	Type       string   `json:"type"`        // 构图类型
	Ratio      string   `json:"ratio"`       // 宽高比
	FocalPoint string   `json:"focal_point"` // 焦点位置
	Tips       []string `json:"tips"`        // 构图提示
}

// NewStyleAnalyzeTool 创建风格分析工具
func NewStyleAnalyzeTool() *StyleAnalyzeTool {
	return &StyleAnalyzeTool{}
}

// Info 返回工具信息
func (t *StyleAnalyzeTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name:        "analyze_style",
		Description: "分析用户需求并推荐合适的图片风格、尺寸和构图。帮助用户选择最佳的生成参数。",
		Parameters: map[string]*schema.ParameterInfo{
			"description": {
				Type:        schema.String,
				Description: "用户对图片的需求描述",
				Required:    true,
			},
			"image_type": {
				Type:        schema.String,
				Description: "图片类型：static、gif、sticker",
				Required:    false,
			},
		},
	}, nil
}

// InvokableRun 执行风格分析
func (t *StyleAnalyzeTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	var params StyleAnalyzeParams
	if err := json.Unmarshal([]byte(argumentsInJSON), &params); err != nil {
		return t.errorResult(fmt.Errorf("invalid parameters: %w", err))
	}

	if params.ImageType == "" {
		params.ImageType = "static"
	}

	// 分析风格
	styleOptions := t.analyzeStyles(params.Description)
	
	// 推荐主风格
	recommendedStyle := t.getRecommendedStyle(styleOptions)
	
	// 推荐尺寸
	recommendedSize := t.getRecommendedSize(params.Description, params.ImageType)
	
	// 推荐质量
	recommendedQuality := t.getRecommendedQuality(params.Description)
	
	// 推荐配色
	colorPalette := t.getColorPalette(params.Description)
	
	// 构图建议
	composition := t.getCompositionSuggestion(params.Description, params.ImageType)

	result := &StyleAnalyzeResult{
		Success:            true,
		RecommendedStyle:   recommendedStyle,
		RecommendedSize:    recommendedSize,
		RecommendedQuality: recommendedQuality,
		StyleOptions:       styleOptions,
		ColorPalette:       colorPalette,
		Composition:        composition,
	}

	return t.successResult(result)
}

// analyzeStyles 分析可能的风格
func (t *StyleAnalyzeTool) analyzeStyles(description string) []StyleOption {
	styles := []StyleOption{
		{Name: "realistic", Description: "写实风格，照片级真实感", Score: 0.0},
		{Name: "cartoon", Description: "卡通风格，简单明快", Score: 0.0},
		{Name: "anime", Description: "日系动漫风格", Score: 0.0},
		{Name: "oil_painting", Description: "油画风格，艺术感强", Score: 0.0},
		{Name: "watercolor", Description: "水彩风格，柔和通透", Score: 0.0},
		{Name: "sketch", Description: "素描风格，线条简洁", Score: 0.0},
		{Name: "pixel_art", Description: "像素风格，复古游戏感", Score: 0.0},
		{Name: "3d_render", Description: "3D渲染风格", Score: 0.0},
		{Name: "minimalist", Description: "极简风格，简约大气", Score: 0.0},
		{Name: "cyberpunk", Description: "赛博朋克风格，科幻感", Score: 0.0},
	}

	// 根据描述中的关键词计算匹配度
	keywords := map[string][]string{
		"realistic":   {"真实", "照片", "写实", "realistic", "photo", "real"},
		"cartoon":     {"卡通", "动画", "cartoon", "animated"},
		"anime":       {"动漫", "日系", "anime", "manga", "二次元"},
		"oil_painting": {"油画", "绘画", "艺术", "oil", "painting", "art"},
		"watercolor":  {"水彩", "watercolor", "淡雅"},
		"sketch":      {"素描", "线条", "sketch", "线稿"},
		"pixel_art":   {"像素", "pixel", "游戏", "复古"},
		"3d_render":   {"3D", "三维", "render", "建模"},
		"minimalist":  {"简约", "极简", "minimal", "简单"},
		"cyberpunk":   {"赛博", "科幻", "cyber", "punk", "未来"},
	}

	for i, style := range styles {
		score := 0.0
		if kws, ok := keywords[style.Name]; ok {
			for _, kw := range kws {
				if containsIgnoreCase(description, kw) {
					score += 0.3
				}
			}
		}
		// 基础分
		score += 0.1
		if score > 1.0 {
			score = 1.0
		}
		styles[i].Score = score
	}

	// 按分数排序
	for i := 0; i < len(styles); i++ {
		for j := i + 1; j < len(styles); j++ {
			if styles[j].Score > styles[i].Score {
				styles[i], styles[j] = styles[j], styles[i]
			}
		}
	}

	// 只返回前5个
	if len(styles) > 5 {
		styles = styles[:5]
	}

	return styles
}

// getRecommendedStyle 获取推荐风格
func (t *StyleAnalyzeTool) getRecommendedStyle(styles []StyleOption) string {
	if len(styles) > 0 {
		return styles[0].Name
	}
	return "realistic"
}

// getRecommendedSize 获取推荐尺寸
func (t *StyleAnalyzeTool) getRecommendedSize(description, imageType string) string {
	switch imageType {
	case "sticker":
		return "512x512"
	case "gif":
		return "768x768"
	default:
		// 根据描述判断
		if containsIgnoreCase(description, "横") || containsIgnoreCase(description, "宽") || containsIgnoreCase(description, "landscape") {
			return "1792x1024"
		}
		if containsIgnoreCase(description, "竖") || containsIgnoreCase(description, "高") || containsIgnoreCase(description, "portrait") {
			return "1024x1792"
		}
		return "1024x1024"
	}
}

// getRecommendedQuality 获取推荐质量
func (t *StyleAnalyzeTool) getRecommendedQuality(description string) string {
	highQualityKeywords := []string{"高清", "精细", "详细", "4K", "高质量", "HD", "detailed", "high quality"}
	for _, kw := range highQualityKeywords {
		if containsIgnoreCase(description, kw) {
			return "hd"
		}
	}
	return "standard"
}

// getColorPalette 获取推荐配色
func (t *StyleAnalyzeTool) getColorPalette(description string) []string {
	// 默认配色
	palette := []string{"#3B82F6", "#10B981", "#F59E0B", "#EF4444", "#8B5CF6"}

	// 根据关键词调整
	if containsIgnoreCase(description, "暖") || containsIgnoreCase(description, "warm") {
		palette = []string{"#F97316", "#EF4444", "#F59E0B", "#FBBF24", "#FCD34D"}
	} else if containsIgnoreCase(description, "冷") || containsIgnoreCase(description, "cool") {
		palette = []string{"#3B82F6", "#06B6D4", "#8B5CF6", "#6366F1", "#14B8A6"}
	} else if containsIgnoreCase(description, "暗") || containsIgnoreCase(description, "dark") {
		palette = []string{"#1F2937", "#374151", "#4B5563", "#6B7280", "#9CA3AF"}
	} else if containsIgnoreCase(description, "明亮") || containsIgnoreCase(description, "bright") {
		palette = []string{"#FBBF24", "#34D399", "#60A5FA", "#F472B6", "#A78BFA"}
	} else if containsIgnoreCase(description, "自然") || containsIgnoreCase(description, "nature") {
		palette = []string{"#22C55E", "#84CC16", "#A3E635", "#059669", "#0D9488"}
	}

	return palette
}

// getCompositionSuggestion 获取构图建议
func (t *StyleAnalyzeTool) getCompositionSuggestion(description, imageType string) CompositionSuggestion {
	suggestion := CompositionSuggestion{
		Type:       "center",
		Ratio:      "1:1",
		FocalPoint: "center",
		Tips:       make([]string, 0),
	}

	switch imageType {
	case "sticker":
		suggestion.Type = "center"
		suggestion.Ratio = "1:1"
		suggestion.FocalPoint = "center"
		suggestion.Tips = []string{
			"主体居中，占画面60-70%",
			"保持简洁，避免复杂背景",
			"表情清晰可见",
		}
	case "gif":
		suggestion.Type = "rule_of_thirds"
		suggestion.Ratio = "1:1"
		suggestion.FocalPoint = "center"
		suggestion.Tips = []string{
			"预留动作空间",
			"保持循环流畅",
			"避免边缘重要元素",
		}
	default:
		if containsIgnoreCase(description, "人物") || containsIgnoreCase(description, "portrait") {
			suggestion.Type = "portrait"
			suggestion.Ratio = "2:3"
			suggestion.FocalPoint = "upper_third"
			suggestion.Tips = []string{
				"面部位于上三分之一",
				"留出适当头顶空间",
				"眼神方向留白",
			}
		} else if containsIgnoreCase(description, "风景") || containsIgnoreCase(description, "landscape") {
			suggestion.Type = "landscape"
			suggestion.Ratio = "16:9"
			suggestion.FocalPoint = "rule_of_thirds"
			suggestion.Tips = []string{
				"地平线位于上或下三分之一",
				"引导线从前景延伸",
				"远近景层次分明",
			}
		} else {
			suggestion.Tips = []string{
				"遵循三分法则",
				"主体突出",
				"背景简洁",
			}
		}
	}

	return suggestion
}

// containsIgnoreCase 忽略大小写的包含检查
func containsIgnoreCase(s, substr string) bool {
	return len(s) >= len(substr) && 
		(s == substr || 
		 len(substr) == 0 ||
		 findIgnoreCase(s, substr) >= 0)
}

// findIgnoreCase 忽略大小写查找
func findIgnoreCase(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			c1 := s[i+j]
			c2 := substr[j]
			if c1 != c2 {
				if c1 >= 'A' && c1 <= 'Z' {
					c1 += 32
				}
				if c2 >= 'A' && c2 <= 'Z' {
					c2 += 32
				}
				if c1 != c2 {
					match = false
					break
				}
			}
		}
		if match {
			return i
		}
	}
	return -1
}

// errorResult 生成错误结果
func (t *StyleAnalyzeTool) errorResult(err error) (string, error) {
	result := &StyleAnalyzeResult{
		Success: false,
		Error:   err.Error(),
	}
	bytes, _ := json.Marshal(result)
	return string(bytes), nil
}

// successResult 生成成功结果
func (t *StyleAnalyzeTool) successResult(result *StyleAnalyzeResult) (string, error) {
	bytes, err := json.Marshal(result)
	if err != nil {
		return t.errorResult(err)
	}
	return string(bytes), nil
}
