---
name: children-growth-handbook
version: 1.0.0
author: mediacraft
description: |
  儿童成长手册生成器。当用户需要为孩子制作成长记录、成长手册、纪念册时使用此技能。
  支持 5 种风格（森系水彩、北欧极简、绘本故事、可爱卡通、复古日记），
  自动生成文案、合成排版、输出 A4 尺寸 PDF 和图片。
  触发词：成长手册、成长记录、宝宝手册、纪念册、宝宝相册、成长册。
tools: Bash, Python, Read, Write
---

# 儿童成长手册生成器

## 概述

为孩子制作精美的成长手册，从文案生成到图文排版到 PDF 输出，一站式完成。

## 使用流程

### 第一步：收集信息

向用户询问以下信息（缺失的可以跳过）：

| 信息 | 必填 | 示例 |
|-----|------|------|
| 孩子姓名 | ✅ | 小明 |
| 昵称 | ❌ | 明明 |
| 出生日期 | ✅ | 2023-05-15 |
| 风格选择 | ✅ | 见下方5种风格 |
| 照片文件夹 | ✅ | ~/photos/xiaoming/ |
| 页数 | ❌ | 默认 20 页 |
| 特殊主题 | ❌ | 生日、旅行、日常等 |

### 第二步：选择风格

展示以下 5 种风格供用户选择：

1. **森系水彩拼贴** (`forest_watercolor`)
   - 大地色 / 莫兰迪色系
   - 自然元素装饰（树叶、花朵、小动物）
   - 诗意温柔的文案

2. **北欧极简手账** (`nordic_minimal`)
   - 黑白灰为主，点缀少量彩色
   - 几何元素、简洁线条
   - 简洁有力的文案

3. **绘本故事风** (`storybook`)
   - 柔和温暖色调
   - 像讲故事一样的叙事文案
   - 童话般的装饰元素

4. **可爱卡通风** (`cute_cartoon`)
   - 明亮饱和色彩
   - 圆润可爱的装饰
   - 俏皮活泼的文案

5. **复古日记本** (`vintage_journal`)
   - 做旧纸张质感、棕色米色
   - 邮票贴纸、手写字体感
   - 文艺怀旧的文案

### 第三步：生成手册

运行脚本生成手册：

```bash
cd ~/.openclaw/skills/children-growth-handbook/scripts

python handbook_gen.py \
  --name "孩子姓名" \
  --nickname "昵称" \
  --birthday "YYYY-MM-DD" \
  --photos "/path/to/photos/" \
  --style "forest_watercolor" \
  --pages 20 \
  --output "~/.openclaw/workspace/output/handbooks/"
```

### 第四步：预览与调整

- 脚本会逐页生成，每页输出一个 PNG 预览
- 展示第一页预览给用户确认风格
- 用户满意后继续生成剩余页面
- 最终合并为 PDF

### 第五步：输出

最终输出到 `~/.openclaw/workspace/output/handbooks/` 目录：
- `{孩子名}_成长手册.pdf` — 完整 PDF
- `pages/page_001.png` ~ `page_NNN.png` — 单页 PNG

## 页面主题类型

按顺序安排页面主题（可自定义）：

| 主题 | 说明 | 建议照片 |
|-----|------|---------|
| `cover` | 封面 | 1 张最好的照片 |
| `birth` | 出生记录 | 出生当天照片 |
| `monthly` | 月龄记录 | 每月一张 |
| `milestone` | 里程碑（第一次翻身、走路等）| 对应照片 |
| `birthday` | 生日纪念 | 生日照片 |
| `daily` | 日常瞬间 | 日常抓拍 |
| `festival` | 节日记录 | 节日照片 |
| `travel` | 旅行记录 | 旅行照片 |
| `growth_data` | 成长数据（身高体重曲线）| 无需照片 |
| `closing` | 尾页寄语 | 家庭合照 |

## 文案生成规则

使用 LLM 为每页生成文案时，提供以下上下文：

```
你是一位专业的儿童成长手册文案设计师。
风格：{selected_style}

请为以下页面生成内容：
- 孩子姓名：{name}，昵称：{nickname}
- 当前页面主题：{theme}
- 孩子年龄（该页记录时）：{age_at_page}
- 照片描述（如有）：{photo_description}

输出 JSON：
{
  "title": "页面标题（8字以内）",
  "main_text": "主文案（50-100字，温馨有爱）",
  "tags": ["标签1", "标签2", "标签3"],
  "decorations": ["装饰元素1", "装饰元素2"],
  "color_palette": ["#颜色1", "#颜色2", "#颜色3"]
}
```

## 注意事项

- 所有照片仅在本地处理，不上传到任何外部服务
- 文案生成使用阿里百炼 Coding Plan（qwen3.5-plus），包含在用户已购套餐中
- 整本 20 页手册无额外 API 成本
- 批量文案生成时 spawn 多个 Worker 并行（5 页一组）
- 需要安装 Python 依赖：`pip install Pillow reportlab openai`
- 字体文件放在 scripts/fonts/ 目录下
