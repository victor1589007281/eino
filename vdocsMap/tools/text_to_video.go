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

// TextToVideoTool 文生视频工具
type TextToVideoTool struct {
	config    *config.VideoConfig
	outputDir string
	client    *http.Client
	// 图片生成工具（用于先生成首帧）
	imageGenTool *ImageGenerateTool
}

// TextToVideoParams 文生视频参数
type TextToVideoParams struct {
	// 视频描述（必需）
	Prompt string `json:"prompt" description:"视频内容描述，包括场景、动作、风格等"`
	// 负面提示词
	NegativePrompt string `json:"negative_prompt,omitempty" description:"不想要的内容"`
	// 视频时长（秒）
	Duration int `json:"duration,omitempty" description:"视频时长（秒），默认4秒"`
	// 帧率
	FPS int `json:"fps,omitempty" description:"帧率，默认24fps"`
	// 宽高比
	AspectRatio string `json:"aspect_ratio,omitempty" description:"宽高比：16:9, 9:16, 1:1"`
	// 分辨率
	Resolution string `json:"resolution,omitempty" description:"分辨率，如 1280x720"`
	// 风格
	Style string `json:"style,omitempty" description:"视频风格：realistic, anime, cinematic"`
	// 运动模式
	MotionMode string `json:"motion_mode,omitempty" description:"运动模式：auto, slow, fast, dynamic"`
	// 种子值
	Seed int64 `json:"seed,omitempty" description:"随机种子"`
	// 输出格式
	OutputFormat string `json:"output_format,omitempty" description:"输出格式：mp4, webm"`
	// 是否先生成首帧图片
	GenerateFirstFrame bool `json:"generate_first_frame,omitempty" description:"是否先生成首帧图片再生成视频"`
}

// TextToVideoResult 文生视频结果
type TextToVideoResult struct {
	Success         bool   `json:"success"`
	VideoPath       string `json:"video_path,omitempty"`
	FirstFramePath  string `json:"first_frame_path,omitempty"`
	Prompt          string `json:"prompt"`
	Duration        int    `json:"duration"`
	FPS             int    `json:"fps"`
	Resolution      string `json:"resolution"`
	FileSize        int64  `json:"file_size,omitempty"`
	GenerationSteps string `json:"generation_steps,omitempty"`
	Error           string `json:"error,omitempty"`
}

// NewTextToVideoTool 创建文生视频工具
func NewTextToVideoTool(cfg *config.VideoConfig, imgCfg *config.StaticImageConfig, outputDir string) *TextToVideoTool {
	return &TextToVideoTool{
		config:    cfg,
		outputDir: outputDir,
		client: &http.Client{
			Timeout: 600 * time.Second,
		},
		imageGenTool: NewImageGenerateTool(imgCfg, outputDir),
	}
}

// Info 返回工具信息
func (t *TextToVideoTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name:        "text_to_video",
		Description: "文生视频工具。直接根据文字描述生成视频，无需提供图片。支持多种风格和分辨率，适用于创意视频、动画短片等场景。",
		Parameters: map[string]*schema.ParameterInfo{
			"prompt": {
				Type:        schema.String,
				Description: "视频内容描述，包括场景、人物、动作、风格等（必需）",
				Required:    true,
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
			"aspect_ratio": {
				Type:        schema.String,
				Description: "宽高比：16:9（横屏）、9:16（竖屏）、1:1（方形）",
				Required:    false,
			},
			"resolution": {
				Type:        schema.String,
				Description: "分辨率，如 1280x720、1920x1080",
				Required:    false,
			},
			"style": {
				Type:        schema.String,
				Description: "视频风格：realistic（写实）、anime（动漫）、cinematic（电影）、3d（三维）",
				Required:    false,
			},
			"motion_mode": {
				Type:        schema.String,
				Description: "运动模式：auto（自动）、slow（慢速）、fast（快速）、dynamic（动态）",
				Required:    false,
			},
			"output_format": {
				Type:        schema.String,
				Description: "输出格式：mp4（默认）、webm",
				Required:    false,
			},
			"generate_first_frame": {
				Type:        schema.Boolean,
				Description: "是否先生成首帧图片再生成视频（提高质量但耗时更长）",
				Required:    false,
			},
		},
	}, nil
}

// InvokableRun 执行文生视频
func (t *TextToVideoTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	var params TextToVideoParams
	if err := json.Unmarshal([]byte(argumentsInJSON), &params); err != nil {
		return t.errorResult(fmt.Errorf("invalid parameters: %w", err))
	}

	// 验证必需参数
	if params.Prompt == "" {
		return t.errorResult(fmt.Errorf("prompt is required"))
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
	if params.AspectRatio == "" {
		params.AspectRatio = "16:9"
	}
	if params.Resolution == "" {
		params.Resolution = t.config.DefaultResolution
	}
	if params.Style == "" {
		params.Style = "realistic"
	}
	if params.MotionMode == "" {
		params.MotionMode = "auto"
	}
	if params.OutputFormat == "" {
		params.OutputFormat = "mp4"
	}

	// 根据提供商选择生成方式
	switch t.config.Provider {
	case "runway":
		return t.generateWithRunway(ctx, &params)
	case "pika":
		return t.generateWithPika(ctx, &params)
	case "kling":
		return t.generateWithKling(ctx, &params)
	case "minimax":
		return t.generateWithMinimax(ctx, &params)
	case "sora":
		return t.generateWithSora(ctx, &params)
	default:
		// 默认使用两步生成：先生成图片，再生成视频
		return t.generateWithTwoSteps(ctx, &params)
	}
}

// generateWithTwoSteps 两步生成：先生成首帧图片，再图生视频
func (t *TextToVideoTool) generateWithTwoSteps(ctx context.Context, params *TextToVideoParams) (string, error) {
	steps := "1. 生成首帧图片\n2. 图片转视频"

	// 第一步：生成首帧图片
	imageParams := &ImageGenerateParams{
		Prompt:  t.buildImagePrompt(params),
		Size:    t.getImageSizeFromRatio(params.AspectRatio),
		Quality: "hd",
		Style:   params.Style,
	}

	imageParamsJSON, _ := json.Marshal(imageParams)
	imageResult, err := t.imageGenTool.InvokableRun(ctx, string(imageParamsJSON))
	if err != nil {
		return t.errorResult(fmt.Errorf("failed to generate first frame: %w", err))
	}

	var imgResp ImageGenerateResult
	if err := json.Unmarshal([]byte(imageResult), &imgResp); err != nil {
		return t.errorResult(fmt.Errorf("failed to parse image result: %w", err))
	}

	if !imgResp.Success {
		return t.errorResult(fmt.Errorf("failed to generate first frame: %s", imgResp.Error))
	}

	// 第二步：读取图片并转为视频
	imageData, err := os.ReadFile(imgResp.ImagePath)
	if err != nil {
		return t.errorResult(fmt.Errorf("failed to read first frame: %w", err))
	}

	// 创建图生视频工具
	videoGenTool := NewVideoGenerateTool(t.config, t.outputDir)
	videoParams := &VideoGenerateParams{
		InitImage:      fmt.Sprintf("data:image/png;base64,%s", string(imageData)),
		Prompt:         params.Prompt,
		NegativePrompt: params.NegativePrompt,
		Duration:       params.Duration,
		FPS:            params.FPS,
		Resolution:     params.Resolution,
		MotionStrength: t.getMotionStrength(params.MotionMode),
		Seed:           params.Seed,
		OutputFormat:   params.OutputFormat,
	}

	videoParamsJSON, _ := json.Marshal(videoParams)
	videoResult, err := videoGenTool.InvokableRun(ctx, string(videoParamsJSON))
	if err != nil {
		return t.errorResult(fmt.Errorf("failed to generate video: %w", err))
	}

	var vidResp VideoGenerateResult
	if err := json.Unmarshal([]byte(videoResult), &vidResp); err != nil {
		return t.errorResult(fmt.Errorf("failed to parse video result: %w", err))
	}

	if !vidResp.Success {
		return t.errorResult(fmt.Errorf("failed to generate video: %s", vidResp.Error))
	}

	result := &TextToVideoResult{
		Success:         true,
		VideoPath:       vidResp.VideoPath,
		FirstFramePath:  imgResp.ImagePath,
		Prompt:          params.Prompt,
		Duration:        params.Duration,
		FPS:             params.FPS,
		Resolution:      params.Resolution,
		FileSize:        vidResp.FileSize,
		GenerationSteps: steps,
	}

	return t.successResult(result)
}

// generateWithRunway 使用 Runway Gen-3 Alpha 直接文生视频
func (t *TextToVideoTool) generateWithRunway(ctx context.Context, params *TextToVideoParams) (string, error) {
	reqBody := map[string]any{
		"promptText": params.Prompt,
		"duration":   params.Duration,
		"watermark":  false,
	}

	if params.Seed != 0 {
		reqBody["seed"] = params.Seed
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return t.errorResult(err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", t.config.BaseURL+"/v1/text-to-video", bytes.NewReader(bodyBytes))
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

	result := &TextToVideoResult{
		Success:    true,
		VideoPath:  videoPath,
		Prompt:     params.Prompt,
		Duration:   params.Duration,
		FPS:        params.FPS,
		Resolution: params.Resolution,
		FileSize:   fileSize,
	}

	return t.successResult(result)
}

// generateWithPika 使用 Pika Labs 文生视频
func (t *TextToVideoTool) generateWithPika(ctx context.Context, params *TextToVideoParams) (string, error) {
	reqBody := map[string]any{
		"prompt":   params.Prompt,
		"motion":   t.getMotionStrength(params.MotionMode),
		"duration": params.Duration,
	}

	if params.NegativePrompt != "" {
		reqBody["negative_prompt"] = params.NegativePrompt
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return t.errorResult(err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", t.config.BaseURL+"/text2video", bytes.NewReader(bodyBytes))
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

	result := &TextToVideoResult{
		Success:    true,
		VideoPath:  videoPath,
		Prompt:     params.Prompt,
		Duration:   params.Duration,
		FPS:        params.FPS,
		Resolution: params.Resolution,
		FileSize:   fileSize,
	}

	return t.successResult(result)
}

// generateWithKling 使用快手可灵文生视频
func (t *TextToVideoTool) generateWithKling(ctx context.Context, params *TextToVideoParams) (string, error) {
	reqBody := map[string]any{
		"prompt":        params.Prompt,
		"duration":      params.Duration,
		"aspect_ratio":  params.AspectRatio,
		"camera_motion": params.MotionMode,
	}

	if params.NegativePrompt != "" {
		reqBody["negative_prompt"] = params.NegativePrompt
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return t.errorResult(err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", t.config.BaseURL+"/v1/videos/text2video", bytes.NewReader(bodyBytes))
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

	result := &TextToVideoResult{
		Success:    true,
		VideoPath:  videoPath,
		Prompt:     params.Prompt,
		Duration:   params.Duration,
		FPS:        params.FPS,
		Resolution: params.Resolution,
		FileSize:   fileSize,
	}

	return t.successResult(result)
}

// generateWithMinimax 使用 MiniMax 文生视频
func (t *TextToVideoTool) generateWithMinimax(ctx context.Context, params *TextToVideoParams) (string, error) {
	reqBody := map[string]any{
		"model":  "video-01",
		"prompt": params.Prompt,
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

	result := &TextToVideoResult{
		Success:    true,
		VideoPath:  videoPath,
		Prompt:     params.Prompt,
		Duration:   params.Duration,
		FPS:        params.FPS,
		Resolution: params.Resolution,
		FileSize:   fileSize,
	}

	return t.successResult(result)
}

// generateWithSora 使用 OpenAI Sora（预留）
func (t *TextToVideoTool) generateWithSora(ctx context.Context, params *TextToVideoParams) (string, error) {
	// Sora API 实现（等待 API 发布）
	return t.errorResult(fmt.Errorf("Sora API not yet available"))
}

// buildImagePrompt 构建用于生成首帧的图片提示词
func (t *TextToVideoTool) buildImagePrompt(params *TextToVideoParams) string {
	prompt := params.Prompt

	// 添加风格修饰
	switch params.Style {
	case "cinematic":
		prompt += ", cinematic lighting, movie still, dramatic composition"
	case "anime":
		prompt += ", anime style, Japanese animation, vibrant colors"
	case "3d":
		prompt += ", 3D render, CGI, detailed textures"
	default:
		prompt += ", photorealistic, high quality, detailed"
	}

	prompt += ", 8k resolution, professional photography"

	return prompt
}

// getImageSizeFromRatio 根据宽高比获取图片尺寸
func (t *TextToVideoTool) getImageSizeFromRatio(ratio string) string {
	switch ratio {
	case "16:9":
		return "1792x1024"
	case "9:16":
		return "1024x1792"
	case "1:1":
		return "1024x1024"
	default:
		return "1792x1024"
	}
}

// getMotionStrength 根据运动模式获取运动强度
func (t *TextToVideoTool) getMotionStrength(mode string) float64 {
	switch mode {
	case "slow":
		return 0.3
	case "fast":
		return 0.8
	case "dynamic":
		return 0.9
	default:
		return 0.5
	}
}

// pollForVideoResult 轮询等待视频结果
func (t *TextToVideoTool) pollForVideoResult(ctx context.Context, taskID, provider string) ([]byte, error) {
	videoGenTool := NewVideoGenerateTool(t.config, t.outputDir)
	return videoGenTool.pollForVideoResult(ctx, taskID, provider)
}

// saveVideo 保存视频
func (t *TextToVideoTool) saveVideo(data []byte, format string) (string, int64, error) {
	if err := os.MkdirAll(t.outputDir, 0755); err != nil {
		return "", 0, fmt.Errorf("failed to create output directory: %w", err)
	}

	filename := fmt.Sprintf("t2v_%s_%d.%s", uuid.New().String()[:8], time.Now().Unix(), format)
	filePath := filepath.Join(t.outputDir, filename)

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return "", 0, fmt.Errorf("failed to save video: %w", err)
	}

	return filePath, int64(len(data)), nil
}

// errorResult 生成错误结果
func (t *TextToVideoTool) errorResult(err error) (string, error) {
	result := &TextToVideoResult{
		Success: false,
		Error:   err.Error(),
	}
	bytes, _ := json.Marshal(result)
	return string(bytes), nil
}

// successResult 生成成功结果
func (t *TextToVideoTool) successResult(result *TextToVideoResult) (string, error) {
	bytes, err := json.Marshal(result)
	if err != nil {
		return t.errorResult(err)
	}
	return string(bytes), nil
}
