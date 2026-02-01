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

// ImageGenerateTool 图片生成工具
type ImageGenerateTool struct {
	config    *config.StaticImageConfig
	outputDir string
	client    *http.Client
}

// ImageGenerateParams 图片生成参数
type ImageGenerateParams struct {
	Prompt   string `json:"prompt" description:"图片描述，越详细越好"`
	Size     string `json:"size,omitempty" description:"图片尺寸，如 1024x1024, 512x512"`
	Quality  string `json:"quality,omitempty" description:"图片质量：standard 或 hd"`
	Style    string `json:"style,omitempty" description:"图片风格：vivid 或 natural"`
	N        int    `json:"n,omitempty" description:"生成数量，默认为1"`
}

// ImageGenerateResult 图片生成结果
type ImageGenerateResult struct {
	Success   bool     `json:"success"`
	ImagePath string   `json:"image_path,omitempty"`
	ImageURL  string   `json:"image_url,omitempty"`
	Prompt    string   `json:"prompt"`
	Size      string   `json:"size"`
	Error     string   `json:"error,omitempty"`
}

// NewImageGenerateTool 创建图片生成工具
func NewImageGenerateTool(cfg *config.StaticImageConfig, outputDir string) *ImageGenerateTool {
	return &ImageGenerateTool{
		config:    cfg,
		outputDir: outputDir,
		client: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// Info 返回工具信息
func (t *ImageGenerateTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name:        "generate_image",
		Description: "生成静态图片。根据文字描述生成高质量的图片，支持各种风格和尺寸。",
		Parameters: map[string]*schema.ParameterInfo{
			"prompt": {
				Type:        schema.String,
				Description: "图片描述，越详细越好。包括主题、风格、颜色、构图等信息。",
				Required:    true,
			},
			"size": {
				Type:        schema.String,
				Description: "图片尺寸，可选值：1024x1024（默认）、1024x1792、1792x1024",
				Required:    false,
			},
			"quality": {
				Type:        schema.String,
				Description: "图片质量，可选值：standard（默认）、hd",
				Required:    false,
			},
			"style": {
				Type:        schema.String,
				Description: "图片风格，可选值：vivid（生动）、natural（自然）",
				Required:    false,
			},
			"n": {
				Type:        schema.Integer,
				Description: "生成图片数量，默认为1，最大为4",
				Required:    false,
			},
		},
	}, nil
}

// InvokableRun 执行图片生成
func (t *ImageGenerateTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	var params ImageGenerateParams
	if err := json.Unmarshal([]byte(argumentsInJSON), &params); err != nil {
		return t.errorResult(fmt.Errorf("invalid parameters: %w", err))
	}

	// 设置默认值
	if params.Size == "" {
		params.Size = t.config.DefaultSize
	}
	if params.Quality == "" {
		params.Quality = t.config.Quality
	}
	if params.N == 0 {
		params.N = 1
	}

	// 根据提供商调用不同的 API
	switch t.config.Provider {
	case "dalle":
		return t.generateWithDALLE(ctx, &params)
	case "stability":
		return t.generateWithStability(ctx, &params)
	default:
		return t.generateWithDALLE(ctx, &params)
	}
}

// generateWithDALLE 使用 DALL-E API 生成图片
func (t *ImageGenerateTool) generateWithDALLE(ctx context.Context, params *ImageGenerateParams) (string, error) {
	// 构建请求
	reqBody := map[string]any{
		"model":           t.config.Model,
		"prompt":          params.Prompt,
		"size":            params.Size,
		"quality":         params.Quality,
		"n":               params.N,
		"response_format": "b64_json",
	}
	if params.Style != "" {
		reqBody["style"] = params.Style
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
			B64JSON       string `json:"b64_json"`
			URL           string `json:"url"`
			RevisedPrompt string `json:"revised_prompt"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return t.errorResult(fmt.Errorf("failed to decode response: %w", err))
	}

	if len(apiResp.Data) == 0 {
		return t.errorResult(fmt.Errorf("no image generated"))
	}

	// 保存图片
	imagePath, err := t.saveImage(apiResp.Data[0].B64JSON, params.Prompt)
	if err != nil {
		return t.errorResult(err)
	}

	result := &ImageGenerateResult{
		Success:   true,
		ImagePath: imagePath,
		Prompt:    params.Prompt,
		Size:      params.Size,
	}

	return t.successResult(result)
}

// generateWithStability 使用 Stability AI API 生成图片
func (t *ImageGenerateTool) generateWithStability(ctx context.Context, params *ImageGenerateParams) (string, error) {
	// 构建请求
	reqBody := map[string]any{
		"text_prompts": []map[string]any{
			{"text": params.Prompt, "weight": 1},
		},
		"cfg_scale":     7,
		"height":        1024,
		"width":         1024,
		"samples":       params.N,
		"steps":         30,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return t.errorResult(err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", t.config.BaseURL+"/generation/stable-diffusion-xl-1024-v1-0/text-to-image", bytes.NewReader(bodyBytes))
	if err != nil {
		return t.errorResult(err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+t.config.APIKey)
	req.Header.Set("Accept", "application/json")

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
		Artifacts []struct {
			Base64       string `json:"base64"`
			FinishReason string `json:"finishReason"`
		} `json:"artifacts"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return t.errorResult(fmt.Errorf("failed to decode response: %w", err))
	}

	if len(apiResp.Artifacts) == 0 {
		return t.errorResult(fmt.Errorf("no image generated"))
	}

	// 保存图片
	imagePath, err := t.saveImage(apiResp.Artifacts[0].Base64, params.Prompt)
	if err != nil {
		return t.errorResult(err)
	}

	result := &ImageGenerateResult{
		Success:   true,
		ImagePath: imagePath,
		Prompt:    params.Prompt,
		Size:      params.Size,
	}

	return t.successResult(result)
}

// saveImage 保存图片到本地
func (t *ImageGenerateTool) saveImage(b64Data string, prompt string) (string, error) {
	// 确保输出目录存在
	if err := os.MkdirAll(t.outputDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create output directory: %w", err)
	}

	// 解码 base64 数据
	imageData, err := base64.StdEncoding.DecodeString(b64Data)
	if err != nil {
		return "", fmt.Errorf("failed to decode image: %w", err)
	}

	// 生成文件名
	filename := fmt.Sprintf("image_%s_%d.png", uuid.New().String()[:8], time.Now().Unix())
	filePath := filepath.Join(t.outputDir, filename)

	// 保存文件
	if err := os.WriteFile(filePath, imageData, 0644); err != nil {
		return "", fmt.Errorf("failed to save image: %w", err)
	}

	return filePath, nil
}

// errorResult 生成错误结果
func (t *ImageGenerateTool) errorResult(err error) (string, error) {
	result := &ImageGenerateResult{
		Success: false,
		Error:   err.Error(),
	}
	bytes, _ := json.Marshal(result)
	return string(bytes), nil
}

// successResult 生成成功结果
func (t *ImageGenerateTool) successResult(result *ImageGenerateResult) (string, error) {
	bytes, err := json.Marshal(result)
	if err != nil {
		return t.errorResult(err)
	}
	return string(bytes), nil
}
