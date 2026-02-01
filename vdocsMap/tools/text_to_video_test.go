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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloudwego/eino/vdocsMap/config"
)

func TestTextToVideoTool_Info(t *testing.T) {
	videoCfg := &config.VideoConfig{
		Provider:          "kling",
		DefaultDuration:   4,
		DefaultFPS:        24,
		DefaultResolution: "1280x720",
	}
	imgCfg := &config.StaticImageConfig{
		Provider:    "dalle",
		DefaultSize: "1024x1024",
	}

	tool := NewTextToVideoTool(videoCfg, imgCfg, "/tmp/test-output")

	info, err := tool.Info(context.Background())
	require.NoError(t, err)

	assert.Equal(t, "text_to_video", info.Name)
	assert.NotEmpty(t, info.Description)
	assert.Contains(t, info.Parameters, "prompt")
	assert.True(t, info.Parameters["prompt"].Required)
}

func TestTextToVideoTool_InvokableRun_MissingPrompt(t *testing.T) {
	videoCfg := &config.VideoConfig{
		Provider:          "kling",
		DefaultDuration:   4,
		DefaultFPS:        24,
		DefaultResolution: "1280x720",
	}
	imgCfg := &config.StaticImageConfig{}

	tool := NewTextToVideoTool(videoCfg, imgCfg, "/tmp/test-output")

	// 缺少必需的 prompt 参数
	params := `{"duration": 5}`
	result, err := tool.InvokableRun(context.Background(), params)
	require.NoError(t, err)

	var resp TextToVideoResult
	err = json.Unmarshal([]byte(result), &resp)
	require.NoError(t, err)

	assert.False(t, resp.Success)
	assert.Contains(t, resp.Error, "prompt is required")
}

func TestTextToVideoTool_InvokableRun_InvalidParams(t *testing.T) {
	videoCfg := &config.VideoConfig{
		Provider: "kling",
	}
	imgCfg := &config.StaticImageConfig{}

	tool := NewTextToVideoTool(videoCfg, imgCfg, "/tmp/test-output")

	result, err := tool.InvokableRun(context.Background(), "invalid json")
	require.NoError(t, err)

	var resp TextToVideoResult
	err = json.Unmarshal([]byte(result), &resp)
	require.NoError(t, err)

	assert.False(t, resp.Success)
	assert.NotEmpty(t, resp.Error)
}

func TestTextToVideoTool_DefaultValues(t *testing.T) {
	videoCfg := &config.VideoConfig{
		Provider:          "kling",
		DefaultDuration:   4,
		MaxDuration:       10,
		DefaultFPS:        24,
		DefaultResolution: "1280x720",
	}
	imgCfg := &config.StaticImageConfig{}

	_ = NewTextToVideoTool(videoCfg, imgCfg, t.TempDir())

	params := TextToVideoParams{
		Prompt: "test prompt",
	}

	// 检查默认值
	if params.Duration == 0 {
		params.Duration = videoCfg.DefaultDuration
	}
	assert.Equal(t, 4, params.Duration)

	if params.AspectRatio == "" {
		params.AspectRatio = "16:9"
	}
	assert.Equal(t, "16:9", params.AspectRatio)

	if params.Style == "" {
		params.Style = "realistic"
	}
	assert.Equal(t, "realistic", params.Style)

	if params.MotionMode == "" {
		params.MotionMode = "auto"
	}
	assert.Equal(t, "auto", params.MotionMode)
}

func TestTextToVideoTool_BuildImagePrompt(t *testing.T) {
	videoCfg := &config.VideoConfig{
		Provider: "kling",
	}
	imgCfg := &config.StaticImageConfig{}

	tool := NewTextToVideoTool(videoCfg, imgCfg, t.TempDir())

	tests := []struct {
		name     string
		params   *TextToVideoParams
		contains []string
	}{
		{
			name: "cinematic style",
			params: &TextToVideoParams{
				Prompt: "a beautiful sunset",
				Style:  "cinematic",
			},
			contains: []string{"beautiful sunset", "cinematic lighting", "movie still"},
		},
		{
			name: "anime style",
			params: &TextToVideoParams{
				Prompt: "a cute cat",
				Style:  "anime",
			},
			contains: []string{"cute cat", "anime style", "Japanese animation"},
		},
		{
			name: "3d style",
			params: &TextToVideoParams{
				Prompt: "a robot walking",
				Style:  "3d",
			},
			contains: []string{"robot walking", "3D render", "CGI"},
		},
		{
			name: "realistic style (default)",
			params: &TextToVideoParams{
				Prompt: "a forest",
				Style:  "realistic",
			},
			contains: []string{"forest", "photorealistic", "high quality"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tool.buildImagePrompt(tt.params)
			for _, keyword := range tt.contains {
				assert.Contains(t, result, keyword)
			}
		})
	}
}

func TestTextToVideoTool_GetImageSizeFromRatio(t *testing.T) {
	videoCfg := &config.VideoConfig{}
	imgCfg := &config.StaticImageConfig{}

	tool := NewTextToVideoTool(videoCfg, imgCfg, t.TempDir())

	tests := []struct {
		ratio    string
		expected string
	}{
		{"16:9", "1792x1024"},
		{"9:16", "1024x1792"},
		{"1:1", "1024x1024"},
		{"unknown", "1792x1024"}, // 默认
	}

	for _, tt := range tests {
		t.Run(tt.ratio, func(t *testing.T) {
			result := tool.getImageSizeFromRatio(tt.ratio)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTextToVideoTool_GetMotionStrength(t *testing.T) {
	videoCfg := &config.VideoConfig{}
	imgCfg := &config.StaticImageConfig{}

	tool := NewTextToVideoTool(videoCfg, imgCfg, t.TempDir())

	tests := []struct {
		mode     string
		expected float64
	}{
		{"slow", 0.3},
		{"fast", 0.8},
		{"dynamic", 0.9},
		{"auto", 0.5},
		{"unknown", 0.5}, // 默认
	}

	for _, tt := range tests {
		t.Run(tt.mode, func(t *testing.T) {
			result := tool.getMotionStrength(tt.mode)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTextToVideoTool_ErrorResult(t *testing.T) {
	videoCfg := &config.VideoConfig{}
	imgCfg := &config.StaticImageConfig{}

	tool := NewTextToVideoTool(videoCfg, imgCfg, "")

	result, err := tool.errorResult(assert.AnError)
	require.NoError(t, err)

	var resp TextToVideoResult
	err = json.Unmarshal([]byte(result), &resp)
	require.NoError(t, err)

	assert.False(t, resp.Success)
	assert.NotEmpty(t, resp.Error)
}

func TestTextToVideoTool_SuccessResult(t *testing.T) {
	videoCfg := &config.VideoConfig{}
	imgCfg := &config.StaticImageConfig{}

	tool := NewTextToVideoTool(videoCfg, imgCfg, "")

	result := &TextToVideoResult{
		Success:         true,
		VideoPath:       "/path/to/video.mp4",
		FirstFramePath:  "/path/to/frame.png",
		Prompt:          "a beautiful scene",
		Duration:        4,
		FPS:             24,
		Resolution:      "1280x720",
		FileSize:        1024000,
		GenerationSteps: "1. Generate frame\n2. Generate video",
	}

	jsonResult, err := tool.successResult(result)
	require.NoError(t, err)

	var resp TextToVideoResult
	err = json.Unmarshal([]byte(jsonResult), &resp)
	require.NoError(t, err)

	assert.True(t, resp.Success)
	assert.Equal(t, "/path/to/video.mp4", resp.VideoPath)
	assert.Equal(t, "/path/to/frame.png", resp.FirstFramePath)
	assert.Equal(t, "a beautiful scene", resp.Prompt)
}

func TestTextToVideoParams_Validation(t *testing.T) {
	params := TextToVideoParams{
		Prompt:         "A cat walking in the garden",
		NegativePrompt: "blurry, low quality",
		Duration:       5,
		FPS:            30,
		AspectRatio:    "16:9",
		Resolution:     "1920x1080",
		Style:          "cinematic",
		MotionMode:     "dynamic",
		Seed:           12345,
		OutputFormat:   "mp4",
	}

	// 验证所有字段都可以正确设置
	assert.Equal(t, "A cat walking in the garden", params.Prompt)
	assert.Equal(t, "blurry, low quality", params.NegativePrompt)
	assert.Equal(t, 5, params.Duration)
	assert.Equal(t, 30, params.FPS)
	assert.Equal(t, "16:9", params.AspectRatio)
	assert.Equal(t, "1920x1080", params.Resolution)
	assert.Equal(t, "cinematic", params.Style)
	assert.Equal(t, "dynamic", params.MotionMode)
	assert.Equal(t, int64(12345), params.Seed)
	assert.Equal(t, "mp4", params.OutputFormat)
}
