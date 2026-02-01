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

package agent

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestImageType(t *testing.T) {
	assert.Equal(t, ImageType("static"), ImageTypeStatic)
	assert.Equal(t, ImageType("gif"), ImageTypeGIF)
	assert.Equal(t, ImageType("sticker"), ImageTypeSticker)
}

func TestGenerateOptions(t *testing.T) {
	opts := &GenerateOptions{}

	// 测试各个 Option 函数
	WithImageType(ImageTypeGIF)(opts)
	assert.Equal(t, ImageTypeGIF, opts.Type)

	WithSize("512x512")(opts)
	assert.Equal(t, "512x512", opts.Size)

	WithFormat("png")(opts)
	assert.Equal(t, "png", opts.Format)

	WithQuality("hd")(opts)
	assert.Equal(t, "hd", opts.Quality)

	WithText("测试文字")(opts)
	assert.Equal(t, "测试文字", opts.Text)

	WithFPS(30)(opts)
	assert.Equal(t, 30, opts.FPS)

	WithDuration(5)(opts)
	assert.Equal(t, 5, opts.Duration)
}

func TestBuildGenerateMessage(t *testing.T) {
	tests := []struct {
		name     string
		prompt   string
		options  *GenerateOptions
		contains []string
	}{
		{
			name:   "静态图",
			prompt: "一只猫",
			options: &GenerateOptions{
				Type:    ImageTypeStatic,
				Size:    "1024x1024",
				Quality: "hd",
			},
			contains: []string{"一只猫", "1024x1024", "hd"},
		},
		{
			name:   "动图",
			prompt: "猫在跳跃",
			options: &GenerateOptions{
				Type: ImageTypeGIF,
				Size: "768x768",
			},
			contains: []string{"动图", "猫在跳跃", "768x768"},
		},
		{
			name:   "表情包无文字",
			prompt: "开心的猫",
			options: &GenerateOptions{
				Type: ImageTypeSticker,
				Size: "512x512",
			},
			contains: []string{"表情包", "开心的猫", "512x512"},
		},
		{
			name:   "表情包有文字",
			prompt: "惊讶的猫",
			options: &GenerateOptions{
				Type: ImageTypeSticker,
				Size: "512x512",
				Text: "什么?!",
			},
			contains: []string{"表情包", "惊讶的猫", "什么?!", "512x512"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			message := buildGenerateMessage(tt.prompt, tt.options)
			for _, exp := range tt.contains {
				assert.Contains(t, message, exp)
			}
		})
	}
}

func TestExtractMetadata(t *testing.T) {
	response := &AgentResponse{
		Messages: make([]*Message, 3),
		ToolCalls: []*ToolCallResult{
			{ToolName: "tool1"},
			{ToolName: "tool2"},
		},
	}

	metadata := extractMetadata(response)

	assert.Equal(t, 2, metadata["tool_calls_count"])
	assert.Equal(t, 3, metadata["messages_count"])
}

func TestAgentResponse(t *testing.T) {
	response := &AgentResponse{
		Messages:      make([]*Message, 0),
		ToolCalls:     make([]*ToolCallResult, 0),
		ImagePaths:    []string{"/path/to/image.png"},
		FinalResponse: "图片已生成",
	}

	assert.Len(t, response.ImagePaths, 1)
	assert.Equal(t, "/path/to/image.png", response.ImagePaths[0])
	assert.Equal(t, "图片已生成", response.FinalResponse)
}

func TestToolCallResult(t *testing.T) {
	result := &ToolCallResult{
		ToolName: "generate_image",
		CallID:   "call_123",
		Result:   `{"success": true, "image_path": "/tmp/test.png"}`,
	}

	assert.Equal(t, "generate_image", result.ToolName)
	assert.Equal(t, "call_123", result.CallID)
	assert.Contains(t, result.Result, "success")
}

func TestImageResult(t *testing.T) {
	result := &ImageResult{
		Path:   "/tmp/test.png",
		Prompt: "一只可爱的猫",
		Type:   ImageTypeStatic,
		Size:   "1024x1024",
		Format: "png",
		Metadata: map[string]any{
			"model": "dall-e-3",
		},
	}

	assert.Equal(t, "/tmp/test.png", result.Path)
	assert.Equal(t, "一只可爱的猫", result.Prompt)
	assert.Equal(t, ImageTypeStatic, result.Type)
	assert.Equal(t, "1024x1024", result.Size)
	assert.Equal(t, "png", result.Format)
	assert.Equal(t, "dall-e-3", result.Metadata["model"])
}

// Message 类型别名，用于测试
type Message = interface{}
