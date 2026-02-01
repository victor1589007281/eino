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
)

func TestStyleAnalyzeTool_Info(t *testing.T) {
	tool := NewStyleAnalyzeTool()

	info, err := tool.Info(context.Background())
	require.NoError(t, err)

	assert.Equal(t, "analyze_style", info.Name)
	assert.NotEmpty(t, info.Description)
	assert.Contains(t, info.Parameters, "description")
	assert.True(t, info.Parameters["description"].Required)
}

func TestStyleAnalyzeTool_AnalyzeStyles(t *testing.T) {
	tool := NewStyleAnalyzeTool()

	tests := []struct {
		description   string
		expectedFirst string
	}{
		{"动漫风格的女孩", "anime"},
		{"写实照片风格的风景", "realistic"},
		{"卡通风格的猫", "cartoon"},
		{"赛博朋克城市", "cyberpunk"},
		{"简约风格的设计", "minimalist"},
		{"像素游戏风格", "pixel_art"},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			styles := tool.analyzeStyles(tt.description)
			require.NotEmpty(t, styles)
			assert.Equal(t, tt.expectedFirst, styles[0].Name)
		})
	}
}

func TestStyleAnalyzeTool_GetRecommendedSize(t *testing.T) {
	tool := NewStyleAnalyzeTool()

	tests := []struct {
		description string
		imageType   string
		expected    string
	}{
		{"表情包", "sticker", "512x512"},
		{"动图", "gif", "768x768"},
		{"横向风景", "static", "1792x1024"},
		{"竖向人像", "static", "1024x1792"},
		{"普通图片", "static", "1024x1024"},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			result := tool.getRecommendedSize(tt.description, tt.imageType)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestStyleAnalyzeTool_GetRecommendedQuality(t *testing.T) {
	tool := NewStyleAnalyzeTool()

	tests := []struct {
		description string
		expected    string
	}{
		{"高清图片", "hd"},
		{"4K壁纸", "hd"},
		{"详细的插画", "hd"},
		{"普通图片", "standard"},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			result := tool.getRecommendedQuality(tt.description)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestStyleAnalyzeTool_GetColorPalette(t *testing.T) {
	tool := NewStyleAnalyzeTool()

	tests := []struct {
		description string
		keyword     string
	}{
		{"暖色调", "#F97316"},
		{"冷色调", "#3B82F6"},
		{"暗色系", "#1F2937"},
		{"明亮的", "#FBBF24"},
		{"自然风格", "#22C55E"},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			palette := tool.getColorPalette(tt.description)
			assert.Contains(t, palette, tt.keyword)
		})
	}
}

func TestStyleAnalyzeTool_GetCompositionSuggestion(t *testing.T) {
	tool := NewStyleAnalyzeTool()

	tests := []struct {
		description string
		imageType   string
		expectType  string
	}{
		{"表情包", "sticker", "center"},
		{"动图", "gif", "rule_of_thirds"},
		{"人物肖像", "static", "portrait"},
		{"风景照片", "static", "landscape"},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			suggestion := tool.getCompositionSuggestion(tt.description, tt.imageType)
			assert.Equal(t, tt.expectType, suggestion.Type)
			assert.NotEmpty(t, suggestion.Tips)
		})
	}
}

func TestStyleAnalyzeTool_InvokableRun(t *testing.T) {
	tool := NewStyleAnalyzeTool()

	params := `{"description": "动漫风格的可爱女孩", "image_type": "static"}`
	result, err := tool.InvokableRun(context.Background(), params)
	require.NoError(t, err)

	var resp StyleAnalyzeResult
	err = json.Unmarshal([]byte(result), &resp)
	require.NoError(t, err)

	assert.True(t, resp.Success)
	assert.NotEmpty(t, resp.RecommendedStyle)
	assert.NotEmpty(t, resp.RecommendedSize)
	assert.NotEmpty(t, resp.RecommendedQuality)
	assert.NotEmpty(t, resp.StyleOptions)
	assert.NotEmpty(t, resp.ColorPalette)
	assert.NotEmpty(t, resp.Composition.Tips)
}

func TestStyleAnalyzeTool_InvokableRun_InvalidParams(t *testing.T) {
	tool := NewStyleAnalyzeTool()

	result, err := tool.InvokableRun(context.Background(), "invalid json")
	require.NoError(t, err)

	var resp StyleAnalyzeResult
	err = json.Unmarshal([]byte(result), &resp)
	require.NoError(t, err)

	assert.False(t, resp.Success)
	assert.NotEmpty(t, resp.Error)
}

func TestContainsIgnoreCase(t *testing.T) {
	tests := []struct {
		s      string
		substr string
		result bool
	}{
		{"Hello World", "hello", true},
		{"Hello World", "WORLD", true},
		{"Hello World", "xyz", false},
		{"", "test", false},
		{"test", "", true},
		{"写实风格", "写实", true},
	}

	for _, tt := range tests {
		t.Run(tt.s+"_"+tt.substr, func(t *testing.T) {
			assert.Equal(t, tt.result, containsIgnoreCase(tt.s, tt.substr))
		})
	}
}

func TestStyleAnalyzeTool_DefaultImageType(t *testing.T) {
	tool := NewStyleAnalyzeTool()

	// 不提供 image_type 应该使用默认值 "static"
	params := `{"description": "一只猫"}`
	result, err := tool.InvokableRun(context.Background(), params)
	require.NoError(t, err)

	var resp StyleAnalyzeResult
	err = json.Unmarshal([]byte(result), &resp)
	require.NoError(t, err)

	assert.True(t, resp.Success)
	// 默认尺寸应该是静态图的尺寸
	assert.Equal(t, "1024x1024", resp.RecommendedSize)
}
