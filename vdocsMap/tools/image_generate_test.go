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
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloudwego/eino/vdocsMap/config"
)

func TestImageGenerateTool_Info(t *testing.T) {
	cfg := &config.StaticImageConfig{
		Provider:    "dalle",
		DefaultSize: "1024x1024",
		Quality:     "standard",
	}

	tool := NewImageGenerateTool(cfg, "/tmp/test-output")

	info, err := tool.Info(context.Background())
	require.NoError(t, err)

	assert.Equal(t, "generate_image", info.Name)
	assert.NotEmpty(t, info.Description)
	assert.Contains(t, info.Parameters, "prompt")
	assert.True(t, info.Parameters["prompt"].Required)
}

func TestImageGenerateTool_InvokableRun_InvalidParams(t *testing.T) {
	cfg := &config.StaticImageConfig{
		Provider:    "dalle",
		DefaultSize: "1024x1024",
	}

	tool := NewImageGenerateTool(cfg, "/tmp/test-output")

	// 测试无效 JSON
	result, err := tool.InvokableRun(context.Background(), "invalid json")
	require.NoError(t, err) // 工具不应该返回 error，而是在结果中包含错误信息

	var resp ImageGenerateResult
	err = json.Unmarshal([]byte(result), &resp)
	require.NoError(t, err)
	assert.False(t, resp.Success)
	assert.NotEmpty(t, resp.Error)
}

func TestImageGenerateTool_InvokableRun_MockAPI(t *testing.T) {
	// 创建模拟服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.URL.Path, "/images/generations")
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		// 返回模拟响应
		response := map[string]any{
			"data": []map[string]any{
				{
					"b64_json":       "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==",
					"revised_prompt": "A cute cat",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// 创建临时输出目录
	outputDir := t.TempDir()

	cfg := &config.StaticImageConfig{
		Provider:    "dalle",
		Model:       "dall-e-3",
		BaseURL:     server.URL,
		APIKey:      "test-api-key",
		DefaultSize: "1024x1024",
		Quality:     "standard",
	}

	tool := NewImageGenerateTool(cfg, outputDir)

	params := `{"prompt": "a cute cat", "size": "1024x1024"}`
	result, err := tool.InvokableRun(context.Background(), params)
	require.NoError(t, err)

	var resp ImageGenerateResult
	err = json.Unmarshal([]byte(result), &resp)
	require.NoError(t, err)

	assert.True(t, resp.Success)
	assert.NotEmpty(t, resp.ImagePath)
	assert.Equal(t, "a cute cat", resp.Prompt)

	// 验证文件已创建
	_, err = os.Stat(resp.ImagePath)
	assert.NoError(t, err)
}

func TestImageGenerateTool_saveImage(t *testing.T) {
	outputDir := t.TempDir()

	cfg := &config.StaticImageConfig{
		Provider: "dalle",
	}

	tool := NewImageGenerateTool(cfg, outputDir)

	// 1x1 像素的 PNG 图片 base64
	b64Data := "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=="

	path, err := tool.saveImage(b64Data, "test prompt")
	require.NoError(t, err)

	assert.NotEmpty(t, path)
	assert.Contains(t, path, outputDir)
	assert.Contains(t, path, ".png")

	// 验证文件存在
	_, err = os.Stat(path)
	assert.NoError(t, err)
}

func TestImageGenerateTool_errorResult(t *testing.T) {
	cfg := &config.StaticImageConfig{}
	tool := NewImageGenerateTool(cfg, "")

	result, err := tool.errorResult(assert.AnError)
	require.NoError(t, err)

	var resp ImageGenerateResult
	err = json.Unmarshal([]byte(result), &resp)
	require.NoError(t, err)

	assert.False(t, resp.Success)
	assert.NotEmpty(t, resp.Error)
}

func TestImageGenerateTool_DefaultValues(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody map[string]any
		json.NewDecoder(r.Body).Decode(&reqBody)

		// 验证默认值被正确设置
		assert.Equal(t, "1024x1024", reqBody["size"])
		assert.Equal(t, "standard", reqBody["quality"])
		assert.Equal(t, float64(1), reqBody["n"])

		response := map[string]any{
			"data": []map[string]any{
				{
					"b64_json": "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==",
				},
			},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	cfg := &config.StaticImageConfig{
		Provider:    "dalle",
		Model:       "dall-e-3",
		BaseURL:     server.URL,
		APIKey:      "test-key",
		DefaultSize: "1024x1024",
		Quality:     "standard",
	}

	tool := NewImageGenerateTool(cfg, t.TempDir())

	// 只提供必需参数
	params := `{"prompt": "test"}`
	_, err := tool.InvokableRun(context.Background(), params)
	assert.NoError(t, err)
}

func TestImageGenerateTool_OutputDirectory(t *testing.T) {
	outputDir := filepath.Join(t.TempDir(), "nested", "output", "dir")

	cfg := &config.StaticImageConfig{
		Provider: "dalle",
	}

	tool := NewImageGenerateTool(cfg, outputDir)

	// 验证 saveImage 会创建嵌套目录
	b64Data := "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=="

	path, err := tool.saveImage(b64Data, "test")
	require.NoError(t, err)

	assert.Contains(t, path, outputDir)
	_, err = os.Stat(outputDir)
	assert.NoError(t, err)
}
