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

// VideoGenerateTool 视频生成工具（图生视频）
type VideoGenerateTool struct {
	config    *config.VideoConfig
	outputDir string
	client    *http.Client
}

// VideoGenerateParams 视频生成参数
type VideoGenerateParams struct {
	// 初始图片（必需）- base64 编码或 URL
	InitImage string `json:"init_image" description:"初始图片，base64编码或URL"`
	// 提示词 - 描述视频动作
	Prompt string `json:"prompt,omitempty" description:"视频动作描述，如'镜头缓慢推进'"`
	// 负面提示词
	NegativePrompt string `json:"negative_prompt,omitempty" description:"不想要的内容"`
	// 视频时长（秒）
	Duration int `json:"duration,omitempty" description:"视频时长（秒），默认4秒"`
	// 帧率
	FPS int `json:"fps,omitempty" description:"帧率，默认24fps"`
	// 分辨率
	Resolution string `json:"resolution,omitempty" description:"分辨率，如 1280x720"`
	// 运动强度
	MotionStrength float64 `json:"motion_strength,omitempty" description:"运动强度，0.1-1.0"`
	// 种子值
	Seed int64 `json:"seed,omitempty" description:"随机种子，用于复现结果"`
	// 输出格式
	OutputFormat string `json:"output_format,omitempty" description:"输出格式：mp4, webm, gif"`
}

// VideoGenerateResult 视频生成结果
type VideoGenerateResult struct {
	Success    bool   `json:"success"`
	VideoPath  string `json:"video_path,omitempty"`
	VideoURL   string `json:"video_url,omitempty"`
	Duration   int    `json:"duration"`
	FPS        int    `json:"fps"`
	Resolution string `json:"resolution"`
	FileSize   int64  `json:"file_size,omitempty"`
	Error      string `json:"error,omitempty"`
}

// NewVideoGenerateTool 创建视频生成工具
func NewVideoGenerateTool(cfg *config.VideoConfig, outputDir string) *VideoGenerateTool {
	return &VideoGenerateTool{
		config:    cfg,
		outputDir: outputDir,
		client: &http.Client{
			Timeout: 600 * time.Second, // 视频生成需要更长时间
		},
	}
}

// Info 返回工具信息
func (t *VideoGenerateTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name:        "generate_video",
		Description: "图生视频工具。将静态图片转换为动态视频，支持指定运动效果、时长和分辨率。适用于让图片动起来、创建动态壁纸等场景。",
		Parameters: map[string]*schema.ParameterInfo{
			"init_image": {
				Type:        schema.String,
				Description: "初始图片，可以是 base64 编码的图片数据或图片 URL（必需）",
				Required:    true,
			},
			"prompt": {
				Type:        schema.String,
				Description: "视频动作描述，如'镜头缓慢推进'、'人物微笑'、'风吹树叶'等",
				Required:    false,
			},
			"negative_prompt": {
				Type:        schema.String,
				Description: "不想要的内容，如'模糊、变形、低质量'",
				Required:    false,
			},
			"duration": {
				Type:        schema.Integer,
				Description: "视频时长（秒），范围1-10秒，默认4秒",
				Required:    false,
			},
			"fps": {
				Type:        schema.Integer,
				Description: "帧率，默认24fps",
				Required:    false,
			},
			"resolution": {
				Type:        schema.String,
				Description: "分辨率，如 1280x720、1920x1080",
				Required:    false,
			},
			"motion_strength": {
				Type:        schema.Number,
				Description: "运动强度，0.1-1.0，数值越大动作越明显",
				Required:    false,
			},
			"output_format": {
				Type:        schema.String,
				Description: "输出格式：mp4（默认）、webm、gif",
				Required:    false,
			},
		},
	}, nil
}

// InvokableRun 执行视频生成
func (t *VideoGenerateTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	var params VideoGenerateParams
	if err := json.Unmarshal([]byte(argumentsInJSON), &params); err != nil {
		return t.errorResult(fmt.Errorf("invalid parameters: %w", err))
	}

	// 验证必需参数
	if params.InitImage == "" {
		return t.errorResult(fmt.Errorf("init_image is required"))
	}

	// 设置默认值
	if params.Duration == 0 {
		params.Duration = t.config.DefaultDuration
	}
	if params.Duration > t.config.MaxDuration {
		params.Duration = t.config.MaxDuration
	}
	if params.FPS == 0 {
		params.FPS = t.config.DefaultFPS
	}
	if params.Resolution == "" {
		params.Resolution = t.config.DefaultResolution
	}
	if params.MotionStrength == 0 {
		params.MotionStrength = 0.5
	}
	if params.OutputFormat == "" {
		params.OutputFormat = "mp4"
	}

	// 根据提供商调用不同的 API
	switch t.config.Provider {
	case "runway":
		return t.generateWithRunway(ctx, &params)
	case "pika":
		return t.generateWithPika(ctx, &params)
	case "stable_video":
		return t.generateWithStableVideo(ctx, &params)
	case "kling":
		return t.generateWithKling(ctx, &params)
	case "minimax":
		return t.generateWithMinimax(ctx, &params)
	default:
		return t.generateWithStableVideo(ctx, &params)
	}
}

// generateWithStableVideo 使用 Stability AI 的 Stable Video Diffusion
func (t *VideoGenerateTool) generateWithStableVideo(ctx context.Context, params *VideoGenerateParams) (string, error) {
	// 处理图片数据
	imageData, err := t.processImageInput(params.InitImage)
	if err != nil {
		return t.errorResult(fmt.Errorf("failed to process image: %w", err))
	}

	// 构建请求
	reqBody := map[string]any{
		"image":            imageData,
		"cfg_scale":        2.5,
		"motion_bucket_id": int(params.MotionStrength * 255),
	}

	if params.Seed != 0 {
		reqBody["seed"] = params.Seed
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
	videoData, err := t.pollForVideoResult(ctx, apiResp.ID, "stable_video")
	if err != nil {
		return t.errorResult(err)
	}

	// 保存视频
	videoPath, fileSize, err := t.saveVideo(videoData, params.OutputFormat)
	if err != nil {
		return t.errorResult(err)
	}

	result := &VideoGenerateResult{
		Success:    true,
		VideoPath:  videoPath,
		Duration:   params.Duration,
		FPS:        params.FPS,
		Resolution: params.Resolution,
		FileSize:   fileSize,
	}

	return t.successResult(result)
}

// generateWithRunway 使用 Runway Gen-3
func (t *VideoGenerateTool) generateWithRunway(ctx context.Context, params *VideoGenerateParams) (string, error) {
	imageData, err := t.processImageInput(params.InitImage)
	if err != nil {
		return t.errorResult(fmt.Errorf("failed to process image: %w", err))
	}

	reqBody := map[string]any{
		"promptImage": imageData,
		"seed":        params.Seed,
		"watermark":   false,
		"duration":    params.Duration,
	}

	if params.Prompt != "" {
		reqBody["promptText"] = params.Prompt
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return t.errorResult(err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", t.config.BaseURL+"/v1/image-to-video", bytes.NewReader(bodyBytes))
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

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return t.errorResult(fmt.Errorf("API error: %s - %s", resp.Status, string(body)))
	}

	var apiResp struct {
		ID string `json:"id"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return t.errorResult(fmt.Errorf("failed to decode response: %w", err))
	}

	// 轮询等待结果
	videoData, err := t.pollForVideoResult(ctx, apiResp.ID, "runway")
	if err != nil {
		return t.errorResult(err)
	}

	videoPath, fileSize, err := t.saveVideo(videoData, params.OutputFormat)
	if err != nil {
		return t.errorResult(err)
	}

	result := &VideoGenerateResult{
		Success:    true,
		VideoPath:  videoPath,
		Duration:   params.Duration,
		FPS:        params.FPS,
		Resolution: params.Resolution,
		FileSize:   fileSize,
	}

	return t.successResult(result)
}

// generateWithPika 使用 Pika Labs
func (t *VideoGenerateTool) generateWithPika(ctx context.Context, params *VideoGenerateParams) (string, error) {
	imageData, err := t.processImageInput(params.InitImage)
	if err != nil {
		return t.errorResult(fmt.Errorf("failed to process image: %w", err))
	}

	reqBody := map[string]any{
		"image":    imageData,
		"motion":   params.MotionStrength,
		"duration": params.Duration,
	}

	if params.Prompt != "" {
		reqBody["prompt"] = params.Prompt
	}
	if params.NegativePrompt != "" {
		reqBody["negative_prompt"] = params.NegativePrompt
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return t.errorResult(err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", t.config.BaseURL+"/generate", bytes.NewReader(bodyBytes))
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

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		return t.errorResult(fmt.Errorf("API error: %s - %s", resp.Status, string(body)))
	}

	var apiResp struct {
		JobID string `json:"job_id"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return t.errorResult(fmt.Errorf("failed to decode response: %w", err))
	}

	videoData, err := t.pollForVideoResult(ctx, apiResp.JobID, "pika")
	if err != nil {
		return t.errorResult(err)
	}

	videoPath, fileSize, err := t.saveVideo(videoData, params.OutputFormat)
	if err != nil {
		return t.errorResult(err)
	}

	result := &VideoGenerateResult{
		Success:    true,
		VideoPath:  videoPath,
		Duration:   params.Duration,
		FPS:        params.FPS,
		Resolution: params.Resolution,
		FileSize:   fileSize,
	}

	return t.successResult(result)
}

// generateWithKling 使用快手可灵
func (t *VideoGenerateTool) generateWithKling(ctx context.Context, params *VideoGenerateParams) (string, error) {
	imageData, err := t.processImageInput(params.InitImage)
	if err != nil {
		return t.errorResult(fmt.Errorf("failed to process image: %w", err))
	}

	reqBody := map[string]any{
		"image":         imageData,
		"prompt":        params.Prompt,
		"duration":      params.Duration,
		"aspect_ratio":  "16:9",
		"camera_motion": "auto",
	}

	if params.NegativePrompt != "" {
		reqBody["negative_prompt"] = params.NegativePrompt
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return t.errorResult(err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", t.config.BaseURL+"/v1/videos/image2video", bytes.NewReader(bodyBytes))
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
		Data struct {
			TaskID string `json:"task_id"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return t.errorResult(fmt.Errorf("failed to decode response: %w", err))
	}

	videoData, err := t.pollForVideoResult(ctx, apiResp.Data.TaskID, "kling")
	if err != nil {
		return t.errorResult(err)
	}

	videoPath, fileSize, err := t.saveVideo(videoData, params.OutputFormat)
	if err != nil {
		return t.errorResult(err)
	}

	result := &VideoGenerateResult{
		Success:    true,
		VideoPath:  videoPath,
		Duration:   params.Duration,
		FPS:        params.FPS,
		Resolution: params.Resolution,
		FileSize:   fileSize,
	}

	return t.successResult(result)
}

// generateWithMinimax 使用 MiniMax 视频生成
func (t *VideoGenerateTool) generateWithMinimax(ctx context.Context, params *VideoGenerateParams) (string, error) {
	imageData, err := t.processImageInput(params.InitImage)
	if err != nil {
		return t.errorResult(fmt.Errorf("failed to process image: %w", err))
	}

	reqBody := map[string]any{
		"model":       "video-01",
		"first_frame": imageData,
		"prompt":      params.Prompt,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return t.errorResult(err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", t.config.BaseURL+"/video_generation", bytes.NewReader(bodyBytes))
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
		TaskID string `json:"task_id"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return t.errorResult(fmt.Errorf("failed to decode response: %w", err))
	}

	videoData, err := t.pollForVideoResult(ctx, apiResp.TaskID, "minimax")
	if err != nil {
		return t.errorResult(err)
	}

	videoPath, fileSize, err := t.saveVideo(videoData, params.OutputFormat)
	if err != nil {
		return t.errorResult(err)
	}

	result := &VideoGenerateResult{
		Success:    true,
		VideoPath:  videoPath,
		Duration:   params.Duration,
		FPS:        params.FPS,
		Resolution: params.Resolution,
		FileSize:   fileSize,
	}

	return t.successResult(result)
}

// processImageInput 处理图片输入（URL 或 base64）
func (t *VideoGenerateTool) processImageInput(input string) (string, error) {
	// 如果是 URL，下载并转为 base64
	if len(input) > 4 && (input[:4] == "http" || input[:5] == "https") {
		resp, err := t.client.Get(input)
		if err != nil {
			return "", fmt.Errorf("failed to download image: %w", err)
		}
		defer resp.Body.Close()

		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", fmt.Errorf("failed to read image data: %w", err)
		}

		return base64.StdEncoding.EncodeToString(data), nil
	}

	// 假设已经是 base64
	return input, nil
}

// pollForVideoResult 轮询等待视频生成结果
func (t *VideoGenerateTool) pollForVideoResult(ctx context.Context, taskID, provider string) ([]byte, error) {
	maxAttempts := 120 // 最多等待 10 分钟
	interval := 5 * time.Second

	for i := 0; i < maxAttempts; i++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(interval):
		}

		var req *http.Request
		var err error

		// 根据不同提供商构建查询请求
		switch provider {
		case "stable_video":
			req, err = http.NewRequestWithContext(ctx, "GET", t.config.BaseURL+"/image-to-video/result/"+taskID, nil)
			req.Header.Set("Accept", "video/*")
		case "runway":
			req, err = http.NewRequestWithContext(ctx, "GET", t.config.BaseURL+"/v1/tasks/"+taskID, nil)
		case "pika":
			req, err = http.NewRequestWithContext(ctx, "GET", t.config.BaseURL+"/job/"+taskID, nil)
		case "kling":
			req, err = http.NewRequestWithContext(ctx, "GET", t.config.BaseURL+"/v1/videos/image2video/"+taskID, nil)
		case "minimax":
			req, err = http.NewRequestWithContext(ctx, "GET", t.config.BaseURL+"/query/video_generation?task_id="+taskID, nil)
		default:
			req, err = http.NewRequestWithContext(ctx, "GET", t.config.BaseURL+"/result/"+taskID, nil)
		}

		if err != nil {
			return nil, err
		}

		req.Header.Set("Authorization", "Bearer "+t.config.APIKey)

		resp, err := t.client.Do(req)
		if err != nil {
			continue
		}

		if resp.StatusCode == http.StatusAccepted {
			resp.Body.Close()
			continue // 还在处理中
		}

		if resp.StatusCode == http.StatusOK {
			// 检查是否直接返回视频数据
			contentType := resp.Header.Get("Content-Type")
			if contentType == "video/mp4" || contentType == "video/webm" {
				data, err := io.ReadAll(resp.Body)
				resp.Body.Close()
				return data, err
			}

			// 否则解析 JSON 响应获取视频 URL
			var result struct {
				Video    string `json:"video"`
				VideoURL string `json:"video_url"`
				Output   string `json:"output"`
				Data     struct {
					VideoURL string `json:"video_url"`
					URL      string `json:"url"`
				} `json:"data"`
				FileID string `json:"file_id"`
			}

			if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
				resp.Body.Close()
				continue
			}
			resp.Body.Close()

			// 获取视频 URL
			videoURL := result.Video
			if videoURL == "" {
				videoURL = result.VideoURL
			}
			if videoURL == "" {
				videoURL = result.Output
			}
			if videoURL == "" {
				videoURL = result.Data.VideoURL
			}
			if videoURL == "" {
				videoURL = result.Data.URL
			}

			if videoURL != "" {
				// 下载视频
				return t.downloadVideo(ctx, videoURL)
			}
		}

		resp.Body.Close()
	}

	return nil, fmt.Errorf("video generation timeout")
}

// downloadVideo 下载视频
func (t *VideoGenerateTool) downloadVideo(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to download video: %s", resp.Status)
	}

	return io.ReadAll(resp.Body)
}

// saveVideo 保存视频文件
func (t *VideoGenerateTool) saveVideo(data []byte, format string) (string, int64, error) {
	if err := os.MkdirAll(t.outputDir, 0755); err != nil {
		return "", 0, fmt.Errorf("failed to create output directory: %w", err)
	}

	filename := fmt.Sprintf("video_%s_%d.%s", uuid.New().String()[:8], time.Now().Unix(), format)
	filePath := filepath.Join(t.outputDir, filename)

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return "", 0, fmt.Errorf("failed to save video: %w", err)
	}

	return filePath, int64(len(data)), nil
}

// errorResult 生成错误结果
func (t *VideoGenerateTool) errorResult(err error) (string, error) {
	result := &VideoGenerateResult{
		Success: false,
		Error:   err.Error(),
	}
	bytes, _ := json.Marshal(result)
	return string(bytes), nil
}

// successResult 生成成功结果
func (t *VideoGenerateTool) successResult(result *VideoGenerateResult) (string, error) {
	bytes, err := json.Marshal(result)
	if err != nil {
		return t.errorResult(err)
	}
	return string(bytes), nil
}
