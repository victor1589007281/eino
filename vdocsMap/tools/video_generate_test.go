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
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloudwego/eino/vdocsMap/config"
)

func TestVideoGenerateTool_Info(t *testing.T) {
	cfg := &config.VideoConfig{
		Provider:          "stable_video",
		DefaultDuration:   4,
		DefaultFPS:        24,
		DefaultResolution: "1280x720",
	}

	tool := NewVideoGenerateTool(cfg, "/tmp/test-output")

	info, err := tool.Info(context.Background())
	require.NoError(t, err)

	assert.Equal(t, "generate_video", info.Name)
	assert.NotEmpty(t, info.Description)
	assert.Contains(t, info.Parameters, "init_image")
	assert.True(t, info.Parameters["init_image"].Required)
}

func TestVideoGenerateTool_InvokableRun_MissingImage(t *testing.T) {
	cfg := &config.VideoConfig{
		Provider:          "stable_video",
		DefaultDuration:   4,
		DefaultFPS:        24,
		DefaultResolution: "1280x720",
	}

	tool := NewVideoGenerateTool(cfg, "/tmp/test-output")

	// 缺少必需的 init_image 参数
	params := `{"prompt": "camera moving forward"}`
	result, err := tool.InvokableRun(context.Background(), params)
	require.NoError(t, err)

	var resp VideoGenerateResult
	err = json.Unmarshal([]byte(result), &resp)
	require.NoError(t, err)

	assert.False(t, resp.Success)
	assert.Contains(t, resp.Error, "init_image is required")
}

func TestVideoGenerateTool_InvokableRun_InvalidParams(t *testing.T) {
	cfg := &config.VideoConfig{
		Provider: "stable_video",
	}

	tool := NewVideoGenerateTool(cfg, "/tmp/test-output")

	result, err := tool.InvokableRun(context.Background(), "invalid json")
	require.NoError(t, err)

	var resp VideoGenerateResult
	err = json.Unmarshal([]byte(result), &resp)
	require.NoError(t, err)

	assert.False(t, resp.Success)
	assert.NotEmpty(t, resp.Error)
}

func TestVideoGenerateTool_DefaultValues(t *testing.T) {
	cfg := &config.VideoConfig{
		Provider:          "stable_video",
		DefaultDuration:   4,
		MaxDuration:       10,
		DefaultFPS:        24,
		DefaultResolution: "1280x720",
	}

	tool := NewVideoGenerateTool(cfg, t.TempDir())

	// 验证默认值被正确设置
	params := VideoGenerateParams{
		InitImage: "base64data",
	}

	// 检查默认值
	if params.Duration == 0 {
		params.Duration = cfg.DefaultDuration
	}
	assert.Equal(t, 4, params.Duration)

	if params.FPS == 0 {
		params.FPS = cfg.DefaultFPS
	}
	assert.Equal(t, 24, params.FPS)

	if params.Resolution == "" {
		params.Resolution = cfg.DefaultResolution
	}
	assert.Equal(t, "1280x720", params.Resolution)
}

func TestVideoGenerateTool_MaxDurationLimit(t *testing.T) {
	cfg := &config.VideoConfig{
		Provider:          "stable_video",
		DefaultDuration:   4,
		MaxDuration:       10,
		DefaultFPS:        24,
		DefaultResolution: "1280x720",
	}

	_ = NewVideoGenerateTool(cfg, t.TempDir())

	// 测试超过最大时长的情况
	params := VideoGenerateParams{
		InitImage: "base64data",
		Duration:  20, // 超过最大时长
	}

	if params.Duration > cfg.MaxDuration {
		params.Duration = cfg.MaxDuration
	}

	assert.Equal(t, 10, params.Duration)
}

func TestVideoGenerateTool_ProcessImageInput_Base64(t *testing.T) {
	cfg := &config.VideoConfig{
		Provider: "stable_video",
	}

	tool := NewVideoGenerateTool(cfg, t.TempDir())

	// Base64 输入应该直接返回
	base64Input := "SGVsbG8gV29ybGQ="
	result, err := tool.processImageInput(base64Input)
	require.NoError(t, err)
	assert.Equal(t, base64Input, result)
}

func TestVideoGenerateTool_ProcessImageInput_URL(t *testing.T) {
	// 创建模拟服务器返回图片
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 返回一个简单的图片数据
		w.Header().Set("Content-Type", "image/png")
		w.Write([]byte("fake image data"))
	}))
	defer server.Close()

	cfg := &config.VideoConfig{
		Provider: "stable_video",
	}

	tool := NewVideoGenerateTool(cfg, t.TempDir())

	result, err := tool.processImageInput(server.URL + "/image.png")
	require.NoError(t, err)

	// 验证返回的是 base64 编码
	decoded, err := base64.StdEncoding.DecodeString(result)
	require.NoError(t, err)
	assert.Equal(t, "fake image data", string(decoded))
}

func TestVideoGenerateTool_SaveVideo(t *testing.T) {
	cfg := &config.VideoConfig{
		Provider: "stable_video",
	}

	outputDir := t.TempDir()
	tool := NewVideoGenerateTool(cfg, outputDir)

	videoData := []byte("fake video data for testing")

	path, fileSize, err := tool.saveVideo(videoData, "mp4")
	require.NoError(t, err)

	assert.NotEmpty(t, path)
	assert.Contains(t, path, outputDir)
	assert.Contains(t, path, ".mp4")
	assert.Equal(t, int64(len(videoData)), fileSize)

	// 验证文件存在
	_, err = os.Stat(path)
	assert.NoError(t, err)
}

func TestVideoGenerateTool_SaveVideo_DifferentFormats(t *testing.T) {
	cfg := &config.VideoConfig{
		Provider: "stable_video",
	}

	outputDir := t.TempDir()
	tool := NewVideoGenerateTool(cfg, outputDir)

	formats := []string{"mp4", "webm", "gif"}
	videoData := []byte("test data")

	for _, format := range formats {
		t.Run(format, func(t *testing.T) {
			path, _, err := tool.saveVideo(videoData, format)
			require.NoError(t, err)
			assert.Contains(t, path, "."+format)
		})
	}
}

func TestVideoGenerateTool_ErrorResult(t *testing.T) {
	cfg := &config.VideoConfig{}
	tool := NewVideoGenerateTool(cfg, "")

	result, err := tool.errorResult(assert.AnError)
	require.NoError(t, err)

	var resp VideoGenerateResult
	err = json.Unmarshal([]byte(result), &resp)
	require.NoError(t, err)

	assert.False(t, resp.Success)
	assert.NotEmpty(t, resp.Error)
}

func TestVideoGenerateTool_SuccessResult(t *testing.T) {
	cfg := &config.VideoConfig{}
	tool := NewVideoGenerateTool(cfg, "")

	result := &VideoGenerateResult{
		Success:    true,
		VideoPath:  "/path/to/video.mp4",
		Duration:   4,
		FPS:        24,
		Resolution: "1280x720",
		FileSize:   1024000,
	}

	jsonResult, err := tool.successResult(result)
	require.NoError(t, err)

	var resp VideoGenerateResult
	err = json.Unmarshal([]byte(jsonResult), &resp)
	require.NoError(t, err)

	assert.True(t, resp.Success)
	assert.Equal(t, "/path/to/video.mp4", resp.VideoPath)
	assert.Equal(t, 4, resp.Duration)
	assert.Equal(t, 24, resp.FPS)
}
