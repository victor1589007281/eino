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

func TestPromptOptimizeTool_Info(t *testing.T) {
	tool := NewPromptOptimizeTool()

	info, err := tool.Info(context.Background())
	require.NoError(t, err)

	assert.Equal(t, "optimize_prompt", info.Name)
	assert.NotEmpty(t, info.Description)
	assert.Contains(t, info.Parameters, "prompt")
	assert.True(t, info.Parameters["prompt"].Required)
}

func TestPromptOptimizeTool_DetectLanguage(t *testing.T) {
	tool := NewPromptOptimizeTool()

	tests := []struct {
		text     string
		expected string
	}{
		{"一只可爱的猫", "zh"},
		{"A cute cat", "en"},
		{"hello world", "en"},
		{"你好世界", "zh"},
		{"混合 mixed 内容", "zh"},
		{"", "en"},
	}

	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			result := tool.detectLanguage(tt.text)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPromptOptimizeTool_AnalyzePrompt(t *testing.T) {
	tool := NewPromptOptimizeTool()

	tests := []struct {
		prompt   string
		expected []string
	}{
		{"一只可爱的猫", []string{"cat", "cute"}},
		{"卡通风格的狗", []string{"dog", "cartoon"}},
		{"动漫风格的女孩", []string{"anime"}},
		{"赛博朋克城市", []string{"city", "cyberpunk"}},
	}

	for _, tt := range tests {
		t.Run(tt.prompt, func(t *testing.T) {
			elements := tool.analyzePrompt(tt.prompt)
			for _, exp := range tt.expected {
				assert.Contains(t, elements, exp)
			}
		})
	}
}

func TestPromptOptimizeTool_Optimize(t *testing.T) {
	tool := NewPromptOptimizeTool()

	tests := []struct {
		prompt    string
		imageType string
		style     string
		contains  []string
	}{
		{
			prompt:    "一只猫",
			imageType: "static",
			style:     "",
			contains:  []string{"一只猫", "high quality", "detailed"},
		},
		{
			prompt:    "一只猫",
			imageType: "gif",
			style:     "",
			contains:  []string{"animated", "dynamic"},
		},
		{
			prompt:    "一只猫",
			imageType: "sticker",
			style:     "",
			contains:  []string{"sticker", "expressive"},
		},
		{
			prompt:    "一只猫",
			imageType: "static",
			style:     "anime",
			contains:  []string{"anime style"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.imageType, func(t *testing.T) {
			elements := tool.analyzePrompt(tt.prompt)
			result := tool.optimize(tt.prompt, tt.imageType, tt.style, elements)
			for _, exp := range tt.contains {
				assert.Contains(t, result, exp)
			}
		})
	}
}

func TestPromptOptimizeTool_TranslateToEnglish(t *testing.T) {
	tool := NewPromptOptimizeTool()

	tests := []struct {
		input    string
		expected string
	}{
		{"一只可爱的猫", "a cute cat"},
		{"开心的女孩", "happy girl"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := tool.translateToEnglish(tt.input)
			assert.Contains(t, result, tt.expected)
		})
	}
}

func TestPromptOptimizeTool_GenerateSuggestions(t *testing.T) {
	tool := NewPromptOptimizeTool()

	tests := []struct {
		prompt      string
		imageType   string
		expectCount int
	}{
		{"猫", "static", 2},           // 短描述应该有建议
		{"卡通风格的可爱猫咪在草地上玩耍", "static", 1}, // 完善的描述也应该有建议
		{"猫在跳", "gif", 1},            // 动图有动作描述
		{"开心的猫表情", "sticker", 1},      // 表情包有情绪描述
	}

	for _, tt := range tests {
		t.Run(tt.prompt, func(t *testing.T) {
			suggestions := tool.generateSuggestions(tt.prompt, tt.imageType)
			assert.GreaterOrEqual(t, len(suggestions), tt.expectCount)
		})
	}
}

func TestPromptOptimizeTool_InvokableRun(t *testing.T) {
	tool := NewPromptOptimizeTool()

	params := `{"prompt": "一只可爱的猫", "image_type": "static"}`
	result, err := tool.InvokableRun(context.Background(), params)
	require.NoError(t, err)

	var resp PromptOptimizeResult
	err = json.Unmarshal([]byte(result), &resp)
	require.NoError(t, err)

	assert.True(t, resp.Success)
	assert.Equal(t, "一只可爱的猫", resp.OriginalPrompt)
	assert.NotEmpty(t, resp.OptimizedPrompt)
	assert.NotEmpty(t, resp.EnglishPrompt)
	assert.NotEmpty(t, resp.Suggestions)
}

func TestPromptOptimizeTool_InvokableRun_InvalidParams(t *testing.T) {
	tool := NewPromptOptimizeTool()

	result, err := tool.InvokableRun(context.Background(), "invalid json")
	require.NoError(t, err)

	var resp PromptOptimizeResult
	err = json.Unmarshal([]byte(result), &resp)
	require.NoError(t, err)

	assert.False(t, resp.Success)
	assert.NotEmpty(t, resp.Error)
}

func TestPromptOptimizeTool_InvokableRun_WithStyle(t *testing.T) {
	tool := NewPromptOptimizeTool()

	params := `{"prompt": "一只猫", "image_type": "static", "style": "anime"}`
	result, err := tool.InvokableRun(context.Background(), params)
	require.NoError(t, err)

	var resp PromptOptimizeResult
	err = json.Unmarshal([]byte(result), &resp)
	require.NoError(t, err)

	assert.True(t, resp.Success)
	assert.Contains(t, resp.OptimizedPrompt, "anime")
}

func TestPromptOptimizeTool_InvokableRun_EnglishInput(t *testing.T) {
	tool := NewPromptOptimizeTool()

	params := `{"prompt": "a cute cat playing", "language": "en"}`
	result, err := tool.InvokableRun(context.Background(), params)
	require.NoError(t, err)

	var resp PromptOptimizeResult
	err = json.Unmarshal([]byte(result), &resp)
	require.NoError(t, err)

	assert.True(t, resp.Success)
	// 英文输入不需要翻译
	assert.Contains(t, resp.EnglishPrompt, "cat")
}
