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

// GIFGenerateTool 动图生成工具
type GIFGenerateTool struct {
	config    *config.AnimatedGIFConfig
	outputDir string
	client    *http.Client
}

// GIFGenerateParams 动图生成参数
type GIFGenerateParams struct {
	Prompt      string  `json:"prompt" description:"动图描述，包括动作、场景等"`
	InitImage   string  `json:"init_image,omitempty" description:"初始图片的base64编码或URL（可选）"`
	Duration    int     `json:"duration,omitempty" description:"动图时长（秒），默认3秒"`
	FPS         int     `json:"fps,omitempty" description:"帧率，默认24fps"`
	Size        string  `json:"size,omitempty" description:"尺寸，如 768x768"`
	MotionScale float64 `json:"motion_scale,omitempty" description:"动作幅度，0.1-1.0"`
}

// GIFGenerateResult 动图生成结果
type GIFGenerateResult struct {
	Success   bool   `json:"success"`
	GIFPath   string `json:"gif_path,omitempty"`
	VideoPath string `json:"video_path,omitempty"`
	Prompt    string `json:"prompt"`
	Duration  int    `json:"duration"`
	FPS       int    `json:"fps"`
	Error     string `json:"error,omitempty"`
}

// NewGIFGenerateTool 创建动图生成工具
func NewGIFGenerateTool(cfg *config.AnimatedGIFConfig, outputDir string) *GIFGenerateTool {
	return &GIFGenerateTool{
		config:    cfg,
		outputDir: outputDir,
		client: &http.Client{
			Timeout: 300 * time.Second, // 视频生成需要更长时间
		},
	}
}

// Info 返回工具信息
func (t *GIFGenerateTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name:        "generate_gif",
		Description: "生成动态图片（GIF动图）。根据文字描述生成短视频或动图，支持从静态图片生成动画效果。",
		Parameters: map[string]*schema.ParameterInfo{
			"prompt": {
				Type:        schema.String,
				Description: "动图描述，包括场景、动作、风格等。例如：'一只猫在跳跃，卡通风格'",
				Required:    true,
			},
			"init_image": {
				Type:        schema.String,
				Description: "初始图片（可选），可以是base64编码的图片数据或图片URL，用于图生视频",
				Required:    false,
			},
			"duration": {
				Type:        schema.Integer,
				Description: "动图时长（秒），默认3秒，最长10秒",
				Required:    false,
			},
			"fps": {
				Type:        schema.Integer,
				Description: "帧率，默认24fps",
				Required:    false,
			},
			"size": {
				Type:        schema.String,
				Description: "输出尺寸，如 768x768",
				Required:    false,
			},
			"motion_scale": {
				Type:        schema.Number,
				Description: "动作幅度，0.1-1.0，数值越大动作越明显",
				Required:    false,
			},
		},
	}, nil
}

// InvokableRun 执行动图生成
func (t *GIFGenerateTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	var params GIFGenerateParams
	if err := json.Unmarshal([]byte(argumentsInJSON), &params); err != nil {
		return t.errorResult(fmt.Errorf("invalid parameters: %w", err))
	}

	// 设置默认值
	if params.Duration == 0 {
		params.Duration = 3
	}
	if params.Duration > t.config.MaxDuration {
		params.Duration = t.config.MaxDuration
	}
	if params.FPS == 0 {
		params.FPS = t.config.DefaultFPS
	}
	if params.Size == "" {
		params.Size = t.config.DefaultSize
	}
	if params.MotionScale == 0 {
		params.MotionScale = 0.5
	}

	// 根据提供商调用不同的 API
	switch t.config.Provider {
	case "stable_video":
		return t.generateWithStableVideo(ctx, &params)
	case "runway":
		return t.generateWithRunway(ctx, &params)
	case "pika":
		return t.generateWithPika(ctx, &params)
	default:
		return t.generateWithStableVideo(ctx, &params)
	}
}

// generateWithStableVideo 使用 Stability AI Video API 生成动图
func (t *GIFGenerateTool) generateWithStableVideo(ctx context.Context, params *GIFGenerateParams) (string, error) {
	// 如果没有初始图片，先生成一张
	var imageData string
	if params.InitImage == "" {
		// 调用文生图先生成初始帧
		imageData = "" // 这里需要实际实现
	} else {
		imageData = params.InitImage
	}

	// 构建请求
	reqBody := map[string]any{
		"image":        imageData,
		"cfg_scale":    2.5,
		"motion_bucket_id": int(params.MotionScale * 255),
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return t.errorResult(err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", t.config.BaseURL+"/image-to-video", bytes.NewReader(bodyBytes))
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

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		return t.errorResult(fmt.Errorf("API error: %s - %s", resp.Status, string(body)))
	}

	var apiResp struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return t.errorResult(fmt.Errorf("failed to decode response: %w", err))
	}

	// 轮询等待结果
	videoData, err := t.pollForResult(ctx, apiResp.ID)
	if err != nil {
		return t.errorResult(err)
	}

	// 保存视频并转换为 GIF
	gifPath, err := t.saveAsGIF(videoData, params.Prompt, params.FPS)
	if err != nil {
		return t.errorResult(err)
	}

	result := &GIFGenerateResult{
		Success:  true,
		GIFPath:  gifPath,
		Prompt:   params.Prompt,
		Duration: params.Duration,
		FPS:      params.FPS,
	}

	return t.successResult(result)
}

// generateWithRunway 使用 Runway API 生成动图
func (t *GIFGenerateTool) generateWithRunway(ctx context.Context, params *GIFGenerateParams) (string, error) {
	// Runway API 实现
	// 类似的实现逻辑
	return t.errorResult(fmt.Errorf("Runway provider not implemented yet"))
}

// generateWithPika 使用 Pika API 生成动图
func (t *GIFGenerateTool) generateWithPika(ctx context.Context, params *GIFGenerateParams) (string, error) {
	// Pika API 实现
	return t.errorResult(fmt.Errorf("Pika provider not implemented yet"))
}

// pollForResult 轮询等待生成结果
func (t *GIFGenerateTool) pollForResult(ctx context.Context, generationID string) ([]byte, error) {
	maxAttempts := 60
	interval := 5 * time.Second

	for i := 0; i < maxAttempts; i++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(interval):
		}

		req, err := http.NewRequestWithContext(ctx, "GET", t.config.BaseURL+"/image-to-video/result/"+generationID, nil)
		if err != nil {
			return nil, err
		}

		req.Header.Set("Authorization", "Bearer "+t.config.APIKey)
		req.Header.Set("Accept", "video/*")

		resp, err := t.client.Do(req)
		if err != nil {
			continue
		}

		if resp.StatusCode == http.StatusAccepted {
			resp.Body.Close()
			continue // 还在处理中
		}

		if resp.StatusCode == http.StatusOK {
			data, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			return data, err
		}

		resp.Body.Close()
	}

	return nil, fmt.Errorf("generation timeout")
}

// saveAsGIF 保存为 GIF 文件
func (t *GIFGenerateTool) saveAsGIF(videoData []byte, prompt string, fps int) (string, error) {
	// 确保输出目录存在
	if err := os.MkdirAll(t.outputDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create output directory: %w", err)
	}

	// 先保存为 mp4
	videoFilename := fmt.Sprintf("video_%s_%d.mp4", uuid.New().String()[:8], time.Now().Unix())
	videoPath := filepath.Join(t.outputDir, videoFilename)
	if err := os.WriteFile(videoPath, videoData, 0644); err != nil {
		return "", fmt.Errorf("failed to save video: %w", err)
	}

	// TODO: 使用 ffmpeg 或其他工具转换为 GIF
	// 这里简化处理，直接返回视频路径
	// 实际生产中需要调用 ffmpeg 进行转换

	gifFilename := fmt.Sprintf("gif_%s_%d.gif", uuid.New().String()[:8], time.Now().Unix())
	gifPath := filepath.Join(t.outputDir, gifFilename)

	// 模拟转换（实际需要调用 ffmpeg）
	_ = gifPath

	return videoPath, nil
}

// saveFromBase64 从 base64 保存
func (t *GIFGenerateTool) saveFromBase64(b64Data string) (string, error) {
	if err := os.MkdirAll(t.outputDir, 0755); err != nil {
		return "", err
	}

	data, err := base64.StdEncoding.DecodeString(b64Data)
	if err != nil {
		return "", err
	}

	filename := fmt.Sprintf("gif_%s_%d.gif", uuid.New().String()[:8], time.Now().Unix())
	filePath := filepath.Join(t.outputDir, filename)

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return "", err
	}

	return filePath, nil
}

// errorResult 生成错误结果
func (t *GIFGenerateTool) errorResult(err error) (string, error) {
	result := &GIFGenerateResult{
		Success: false,
		Error:   err.Error(),
	}
	bytes, _ := json.Marshal(result)
	return string(bytes), nil
}

// successResult 生成成功结果
func (t *GIFGenerateTool) successResult(result *GIFGenerateResult) (string, error) {
	bytes, err := json.Marshal(result)
	if err != nil {
		return t.errorResult(err)
	}
	return string(bytes), nil
}
