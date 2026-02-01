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
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// PromptOptimizeTool 提示词优化工具
type PromptOptimizeTool struct{}

// PromptOptimizeParams 提示词优化参数
type PromptOptimizeParams struct {
	Prompt    string `json:"prompt" description:"原始提示词"`
	ImageType string `json:"image_type,omitempty" description:"图片类型：static, gif, sticker"`
	Style     string `json:"style,omitempty" description:"期望风格"`
	Language  string `json:"language,omitempty" description:"输入语言：zh, en"`
}

// PromptOptimizeResult 提示词优化结果
type PromptOptimizeResult struct {
	Success          bool     `json:"success"`
	OriginalPrompt   string   `json:"original_prompt"`
	OptimizedPrompt  string   `json:"optimized_prompt"`
	EnglishPrompt    string   `json:"english_prompt"`
	Suggestions      []string `json:"suggestions,omitempty"`
	DetectedElements []string `json:"detected_elements,omitempty"`
	Error            string   `json:"error,omitempty"`
}

// NewPromptOptimizeTool 创建提示词优化工具
func NewPromptOptimizeTool() *PromptOptimizeTool {
	return &PromptOptimizeTool{}
}

// Info 返回工具信息
func (t *PromptOptimizeTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name:        "optimize_prompt",
		Description: "优化图片生成提示词。将简单的中文描述转换为更详细、更适合图片生成的英文提示词。",
		Parameters: map[string]*schema.ParameterInfo{
			"prompt": {
				Type:        schema.String,
				Description: "原始提示词，可以是简单的中文描述",
				Required:    true,
			},
			"image_type": {
				Type:        schema.String,
				Description: "图片类型：static（静态图）、gif（动图）、sticker（表情包）",
				Required:    false,
			},
			"style": {
				Type:        schema.String,
				Description: "期望的风格，如 realistic, anime, cartoon 等",
				Required:    false,
			},
			"language": {
				Type:        schema.String,
				Description: "输入语言：zh（中文）、en（英文），默认自动检测",
				Required:    false,
			},
		},
	}, nil
}

// InvokableRun 执行提示词优化
func (t *PromptOptimizeTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	var params PromptOptimizeParams
	if err := json.Unmarshal([]byte(argumentsInJSON), &params); err != nil {
		return t.errorResult(fmt.Errorf("invalid parameters: %w", err))
	}

	// 检测语言
	if params.Language == "" {
		params.Language = t.detectLanguage(params.Prompt)
	}

	// 设置默认图片类型
	if params.ImageType == "" {
		params.ImageType = "static"
	}

	// 分析原始提示词
	elements := t.analyzePrompt(params.Prompt)

	// 优化提示词
	optimizedPrompt := t.optimize(params.Prompt, params.ImageType, params.Style, elements)

	// 翻译为英文（如果需要）
	englishPrompt := optimizedPrompt
	if params.Language == "zh" {
		englishPrompt = t.translateToEnglish(optimizedPrompt)
	}

	// 生成建议
	suggestions := t.generateSuggestions(params.Prompt, params.ImageType)

	result := &PromptOptimizeResult{
		Success:          true,
		OriginalPrompt:   params.Prompt,
		OptimizedPrompt:  optimizedPrompt,
		EnglishPrompt:    englishPrompt,
		Suggestions:      suggestions,
		DetectedElements: elements,
	}

	return t.successResult(result)
}

// detectLanguage 检测语言
func (t *PromptOptimizeTool) detectLanguage(text string) string {
	// 简单检测是否包含中文字符
	for _, r := range text {
		if r >= 0x4e00 && r <= 0x9fff {
			return "zh"
		}
	}
	return "en"
}

// analyzePrompt 分析提示词中的元素
func (t *PromptOptimizeTool) analyzePrompt(prompt string) []string {
	elements := make([]string, 0)

	// 检测常见元素
	keywords := map[string]string{
		"猫":    "cat",
		"狗":    "dog",
		"人":    "person",
		"风景":   "landscape",
		"城市":   "city",
		"卡通":   "cartoon",
		"动漫":   "anime",
		"可爱":   "cute",
		"酷":    "cool",
		"开心":   "happy",
		"伤心":   "sad",
		"惊讶":   "surprised",
		"生气":   "angry",
		"搞笑":   "funny",
		"日落":   "sunset",
		"海边":   "beach",
		"山":    "mountain",
		"森林":   "forest",
		"星空":   "starry sky",
		"赛博朋克": "cyberpunk",
		"复古":   "vintage",
		"简约":   "minimalist",
	}

	for cn, en := range keywords {
		if strings.Contains(prompt, cn) {
			elements = append(elements, en)
		}
	}

	return elements
}

// optimize 优化提示词
func (t *PromptOptimizeTool) optimize(prompt, imageType, style string, elements []string) string {
	var sb strings.Builder

	// 基础描述
	sb.WriteString(prompt)

	// 添加风格
	if style != "" {
		sb.WriteString(", " + style + " style")
	}

	// 根据图片类型添加修饰
	switch imageType {
	case "static":
		sb.WriteString(", high quality, detailed, professional photography")
	case "gif":
		sb.WriteString(", animated, dynamic, motion blur")
	case "sticker":
		sb.WriteString(", sticker style, simple lines, bold colors, expressive")
	}

	// 添加通用质量修饰
	sb.WriteString(", 4k, high resolution, best quality")

	return sb.String()
}

// translateToEnglish 翻译为英文
func (t *PromptOptimizeTool) translateToEnglish(text string) string {
	// 常用翻译词典
	translations := map[string]string{
		"一只":   "a",
		"一个":   "a",
		"可爱的":  "cute",
		"美丽的":  "beautiful",
		"酷炫的":  "cool",
		"卡通":   "cartoon",
		"动漫":   "anime",
		"写实":   "realistic",
		"猫":    "cat",
		"狗":    "dog",
		"猫咪":   "kitten",
		"小狗":   "puppy",
		"人物":   "character",
		"女孩":   "girl",
		"男孩":   "boy",
		"开心":   "happy",
		"伤心":   "sad",
		"惊讶":   "surprised",
		"生气":   "angry",
		"害羞":   "shy",
		"得意":   "proud",
		"表情":   "expression",
		"正在":   "",
		"在":    "",
		"的":    "",
		"做出":   "making",
		"看着":   "looking at",
		"拿着":   "holding",
		"吃":    "eating",
		"跑":    "running",
		"跳":    "jumping",
		"睡觉":   "sleeping",
		"玩耍":   "playing",
		"背景":   "background",
		"白色":   "white",
		"透明":   "transparent",
		"彩色":   "colorful",
		"简单":   "simple",
		"复杂":   "complex",
		"高质量":  "high quality",
		"细节":   "detailed",
	}

	result := text
	for cn, en := range translations {
		result = strings.ReplaceAll(result, cn, en)
	}

	// 清理多余空格
	result = strings.Join(strings.Fields(result), " ")

	return result
}

// generateSuggestions 生成改进建议
func (t *PromptOptimizeTool) generateSuggestions(prompt, imageType string) []string {
	suggestions := make([]string, 0)

	// 检查提示词长度
	if len(prompt) < 10 {
		suggestions = append(suggestions, "建议添加更多细节描述，如颜色、风格、场景等")
	}

	// 检查是否有风格描述
	hasStyle := false
	styleKeywords := []string{"风格", "style", "卡通", "写实", "动漫", "复古", "现代"}
	for _, kw := range styleKeywords {
		if strings.Contains(prompt, kw) {
			hasStyle = true
			break
		}
	}
	if !hasStyle {
		suggestions = append(suggestions, "建议指定图片风格，如：卡通、写实、动漫等")
	}

	// 根据图片类型给出建议
	switch imageType {
	case "gif":
		if !strings.Contains(prompt, "动") && !strings.Contains(prompt, "motion") {
			suggestions = append(suggestions, "生成动图时，建议描述具体的动作或运动")
		}
	case "sticker":
		if !strings.Contains(prompt, "表情") && !strings.Contains(prompt, "emotion") {
			suggestions = append(suggestions, "生成表情包时，建议明确表情或情绪")
		}
	}

	// 添加通用建议
	if len(suggestions) == 0 {
		suggestions = append(suggestions, "提示词已经很完善！")
	}

	return suggestions
}

// errorResult 生成错误结果
func (t *PromptOptimizeTool) errorResult(err error) (string, error) {
	result := &PromptOptimizeResult{
		Success: false,
		Error:   err.Error(),
	}
	bytes, _ := json.Marshal(result)
	return string(bytes), nil
}

// successResult 生成成功结果
func (t *PromptOptimizeTool) successResult(result *PromptOptimizeResult) (string, error) {
	bytes, err := json.Marshal(result)
	if err != nil {
		return t.errorResult(err)
	}
	return string(bytes), nil
}
