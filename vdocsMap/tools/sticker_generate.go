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
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"

	"github.com/cloudwego/eino/vdocsMap/config"
)

// StickerGenerateTool 表情包生成工具
type StickerGenerateTool struct {
	config    *config.StickerConfig
	outputDir string
	client    *http.Client
}

// StickerGenerateParams 表情包生成参数
type StickerGenerateParams struct {
	Prompt       string `json:"prompt" description:"表情包描述，包括表情、动作等"`
	Text         string `json:"text,omitempty" description:"表情包上的文字"`
	TextPosition string `json:"text_position,omitempty" description:"文字位置：top, bottom, center"`
	Size         string `json:"size,omitempty" description:"尺寸，如 512x512"`
	Style        string `json:"style,omitempty" description:"风格：cartoon, realistic, anime, chibi"`
	Background   string `json:"background,omitempty" description:"背景：transparent, white, colored"`
	Emotion      string `json:"emotion,omitempty" description:"情绪：happy, sad, angry, surprised, cute"`
}

// StickerGenerateResult 表情包生成结果
type StickerGenerateResult struct {
	Success     bool   `json:"success"`
	StickerPath string `json:"sticker_path,omitempty"`
	Prompt      string `json:"prompt"`
	Text        string `json:"text,omitempty"`
	Style       string `json:"style"`
	Error       string `json:"error,omitempty"`
}

// NewStickerGenerateTool 创建表情包生成工具
func NewStickerGenerateTool(cfg *config.StickerConfig, outputDir string) *StickerGenerateTool {
	return &StickerGenerateTool{
		config:    cfg,
		outputDir: outputDir,
		client: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// Info 返回工具信息
func (t *StickerGenerateTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name:        "generate_sticker",
		Description: "生成表情包。可以生成各种风格的表情包，支持添加文字、选择表情和风格。",
		Parameters: map[string]*schema.ParameterInfo{
			"prompt": {
				Type:        schema.String,
				Description: "表情包描述，如'一只可爱的猫咪做出惊讶的表情'",
				Required:    true,
			},
			"text": {
				Type:        schema.String,
				Description: "表情包上显示的文字（可选）",
				Required:    false,
			},
			"text_position": {
				Type:        schema.String,
				Description: "文字位置：top（顶部）、bottom（底部）、center（居中）",
				Required:    false,
			},
			"size": {
				Type:        schema.String,
				Description: "输出尺寸，默认512x512",
				Required:    false,
			},
			"style": {
				Type:        schema.String,
				Description: "风格：cartoon（卡通）、realistic（写实）、anime（动漫）、chibi（Q版）",
				Required:    false,
			},
			"background": {
				Type:        schema.String,
				Description: "背景：transparent（透明）、white（白色）、colored（彩色）",
				Required:    false,
			},
			"emotion": {
				Type:        schema.String,
				Description: "情绪：happy、sad、angry、surprised、cute、cool、funny",
				Required:    false,
			},
		},
	}, nil
}

// InvokableRun 执行表情包生成
func (t *StickerGenerateTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	var params StickerGenerateParams
	if err := json.Unmarshal([]byte(argumentsInJSON), &params); err != nil {
		return t.errorResult(fmt.Errorf("invalid parameters: %w", err))
	}

	// 设置默认值
	if params.Size == "" {
		params.Size = t.config.DefaultSize
	}
	if params.TextPosition == "" {
		params.TextPosition = t.config.TextPosition
	}
	if params.Style == "" {
		params.Style = "cartoon"
	}
	if params.Background == "" {
		params.Background = "transparent"
	}

	// 优化提示词以生成表情包
	optimizedPrompt := t.buildStickerPrompt(&params)

	// 生成表情包
	return t.generateSticker(ctx, optimizedPrompt, &params)
}

// buildStickerPrompt 构建表情包专用提示词
func (t *StickerGenerateTool) buildStickerPrompt(params *StickerGenerateParams) string {
	prompt := params.Prompt

	// 添加风格修饰
	stylePrompt := ""
	switch params.Style {
	case "cartoon":
		stylePrompt = "cartoon style, simple lines, bold colors, sticker design"
	case "realistic":
		stylePrompt = "realistic style, detailed, high quality"
	case "anime":
		stylePrompt = "anime style, Japanese animation, expressive"
	case "chibi":
		stylePrompt = "chibi style, cute, big head, small body, kawaii"
	default:
		stylePrompt = "sticker style, simple, clear"
	}

	// 添加情绪修饰
	emotionPrompt := ""
	switch params.Emotion {
	case "happy":
		emotionPrompt = "happy expression, smiling, joyful"
	case "sad":
		emotionPrompt = "sad expression, tears, melancholy"
	case "angry":
		emotionPrompt = "angry expression, frowning, upset"
	case "surprised":
		emotionPrompt = "surprised expression, wide eyes, shocked"
	case "cute":
		emotionPrompt = "cute expression, adorable, sweet"
	case "cool":
		emotionPrompt = "cool expression, confident, stylish"
	case "funny":
		emotionPrompt = "funny expression, humorous, silly"
	}

	// 添加背景设置
	bgPrompt := ""
	switch params.Background {
	case "transparent":
		bgPrompt = "transparent background, PNG format, no background"
	case "white":
		bgPrompt = "white background, clean"
	case "colored":
		bgPrompt = "colorful background, vibrant"
	}

	// 组合提示词
	fullPrompt := fmt.Sprintf("%s, %s, %s, %s, sticker art, emoji style, expressive, high quality",
		prompt, stylePrompt, emotionPrompt, bgPrompt)

	return fullPrompt
}

// generateSticker 调用 API 生成表情包
func (t *StickerGenerateTool) generateSticker(ctx context.Context, prompt string, params *StickerGenerateParams) (string, error) {
	// 使用 DALL-E 或其他图片生成 API
	reqBody := map[string]any{
		"model":           "dall-e-3",
		"prompt":          prompt,
		"size":            "1024x1024", // DALL-E 3 不支持 512x512
		"quality":         "standard",
		"n":               1,
		"response_format": "b64_json",
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return t.errorResult(err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", t.config.BaseURL+"/images/generations", bytes.NewReader(bodyBytes))
	if err != nil {
		return t.errorResult(err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+t.config.APIKey)

	resp, err := t.client.Do(req)
	if err != nil {
		return t.errorResult(fmt.Errorf("API request failed: %w", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return t.errorResult(fmt.Errorf("API error: %s - %s", resp.Status, string(body)))
	}

	var apiResp struct {
		Data []struct {
			B64JSON string `json:"b64_json"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return t.errorResult(fmt.Errorf("failed to decode response: %w", err))
	}

	if len(apiResp.Data) == 0 {
		return t.errorResult(fmt.Errorf("no sticker generated"))
	}

	// 保存表情包
	stickerPath, err := t.saveSticker(apiResp.Data[0].B64JSON, params)
	if err != nil {
		return t.errorResult(err)
	}

	result := &StickerGenerateResult{
		Success:     true,
		StickerPath: stickerPath,
		Prompt:      params.Prompt,
		Text:        params.Text,
		Style:       params.Style,
	}

	return t.successResult(result)
}

// saveSticker 保存表情包
func (t *StickerGenerateTool) saveSticker(b64Data string, params *StickerGenerateParams) (string, error) {
	// 确保输出目录存在
	if err := os.MkdirAll(t.outputDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create output directory: %w", err)
	}

	// 解码 base64 数据
	imageData, err := base64.StdEncoding.DecodeString(b64Data)
	if err != nil {
		return "", fmt.Errorf("failed to decode image: %w", err)
	}

	// 如果需要添加文字，这里可以调用图片处理库
	// TODO: 实现文字添加功能（使用 gg 或 imaging 库）
	if params.Text != "" && t.config.WithText {
		imageData, err = t.addTextToImage(imageData, params.Text, params.TextPosition)
		if err != nil {
			// 即使添加文字失败，也保存原图
			fmt.Printf("Warning: failed to add text: %v\n", err)
		}
	}

	// 生成文件名
	filename := fmt.Sprintf("sticker_%s_%d.png", uuid.New().String()[:8], time.Now().Unix())
	filePath := filepath.Join(t.outputDir, filename)

	// 保存文件
	if err := os.WriteFile(filePath, imageData, 0644); err != nil {
		return "", fmt.Errorf("failed to save sticker: %w", err)
	}

	return filePath, nil
}

// addTextToImage 在图片上添加文字
func (t *StickerGenerateTool) addTextToImage(imageData []byte, text, position string) ([]byte, error) {
	// TODO: 使用 github.com/fogleman/gg 或其他图片处理库实现
	// 这里返回原始数据，实际实现需要：
	// 1. 解码图片
	// 2. 创建绘图上下文
	// 3. 设置字体和颜色
	// 4. 根据 position 计算文字位置
	// 5. 绘制文字（可能需要描边效果）
	// 6. 编码并返回
	return imageData, nil
}

// errorResult 生成错误结果
func (t *StickerGenerateTool) errorResult(err error) (string, error) {
	result := &StickerGenerateResult{
		Success: false,
		Error:   err.Error(),
	}
	bytes, _ := json.Marshal(result)
	return string(bytes), nil
}

// successResult 生成成功结果
func (t *StickerGenerateTool) successResult(result *StickerGenerateResult) (string, error) {
	bytes, err := json.Marshal(result)
	if err != nil {
		return t.errorResult(err)
	}
	return string(bytes), nil
}
