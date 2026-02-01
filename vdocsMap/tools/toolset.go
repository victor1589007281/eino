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
	"github.com/cloudwego/eino/components/tool"

	"github.com/cloudwego/eino/vdocsMap/config"
)

// ToolSet 工具集管理
type ToolSet struct {
	config         *config.Config
	tools          []tool.BaseTool
	imageGenTool   *ImageGenerateTool
	gifGenTool     *GIFGenerateTool
	stickerTool    *StickerGenerateTool
	videoGenTool   *VideoGenerateTool
	textToVideoTool *TextToVideoTool
	promptOptTool  *PromptOptimizeTool
	styleTool      *StyleAnalyzeTool
}

// NewToolSet 创建工具集
func NewToolSet(cfg *config.Config) (*ToolSet, error) {
	ts := &ToolSet{
		config: cfg,
		tools:  make([]tool.BaseTool, 0),
	}

	// 初始化各个工具
	if err := ts.initTools(); err != nil {
		return nil, err
	}

	return ts, nil
}

// initTools 初始化所有工具
func (ts *ToolSet) initTools() error {
	outputDir := ts.config.Image.Output.Directory

	// 图片生成工具
	ts.imageGenTool = NewImageGenerateTool(&ts.config.Image.StaticImage, outputDir)
	ts.tools = append(ts.tools, ts.imageGenTool)

	// GIF 生成工具
	ts.gifGenTool = NewGIFGenerateTool(&ts.config.Image.AnimatedGIF, outputDir)
	ts.tools = append(ts.tools, ts.gifGenTool)

	// 表情包生成工具
	ts.stickerTool = NewStickerGenerateTool(&ts.config.Image.Sticker, outputDir)
	ts.tools = append(ts.tools, ts.stickerTool)

	// 视频生成工具（图生视频）
	ts.videoGenTool = NewVideoGenerateTool(&ts.config.Video, outputDir)
	ts.tools = append(ts.tools, ts.videoGenTool)

	// 文生视频工具
	ts.textToVideoTool = NewTextToVideoTool(&ts.config.Video, &ts.config.Image.StaticImage, outputDir)
	ts.tools = append(ts.tools, ts.textToVideoTool)

	// 提示词优化工具
	ts.promptOptTool = NewPromptOptimizeTool()
	ts.tools = append(ts.tools, ts.promptOptTool)

	// 风格分析工具
	ts.styleTool = NewStyleAnalyzeTool()
	ts.tools = append(ts.tools, ts.styleTool)

	return nil
}

// GetTools 获取所有工具
func (ts *ToolSet) GetTools() []tool.BaseTool {
	return ts.tools
}

// GetTool 根据名称获取工具
func (ts *ToolSet) GetTool(name string) tool.BaseTool {
	for _, t := range ts.tools {
		if it, ok := t.(tool.InvokableTool); ok {
			_ = it // 类型检查
		}
	}
	return nil
}

// Close 关闭工具集
func (ts *ToolSet) Close() error {
	// 清理资源
	return nil
}
