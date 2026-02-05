// Package ml 简单分类器（降级方案）
package ml

import (
	"context"
	"strings"

	"github.com/cloudwego/eino/vdocstool/algorithm/types"
)

// SimpleClassifier 简单分类器（基于关键词）
type SimpleClassifier struct {
	intentKeywords map[string][]string
}

// NewSimpleClassifier 创建简单分类器
func NewSimpleClassifier() *SimpleClassifier {
	c := &SimpleClassifier{
		intentKeywords: make(map[string][]string),
	}
	c.initKeywords()
	return c
}

// initKeywords 初始化关键词
func (c *SimpleClassifier) initKeywords() {
	// 邮件意图
	c.intentKeywords[types.IntentEmailSearch] = []string{
		"搜索邮件", "查找邮件", "找邮件", "邮件搜索", "search email", "find email",
		"搜一下邮件", "帮我找邮件", "有没有邮件",
	}
	c.intentKeywords[types.IntentEmailRead] = []string{
		"打开邮件", "阅读邮件", "查看邮件", "看邮件", "read email", "open email",
		"最新邮件", "最近邮件",
	}
	c.intentKeywords[types.IntentEmailDownload] = []string{
		"下载附件", "保存附件", "附件下载", "download attachment",
		"获取附件", "把附件保存",
	}
	c.intentKeywords[types.IntentEmailSummarize] = []string{
		"邮件总结", "邮件摘要", "总结邮件", "summarize email",
	}

	// 记忆意图
	c.intentKeywords[types.IntentMemoryReference] = []string{
		"之前说的", "刚才提到", "上次讨论", "前面说的",
		"earlier mentioned", "previously discussed",
	}
	c.intentKeywords[types.IntentMemoryTopicSwitch] = []string{
		"回到话题", "继续之前", "接着说", "go back to",
	}
	c.intentKeywords[types.IntentMemoryNewTopic] = []string{
		"新话题", "换个话题", "另外一个问题", "by the way",
	}

	// 搜索意图
	c.intentKeywords[types.IntentWebSearch] = []string{
		"搜索", "搜一下", "查一下", "帮我查", "百度", "谷歌",
		"google", "bing", "search for",
	}
	c.intentKeywords[types.IntentWebSearchNews] = []string{
		"最新消息", "新闻", "资讯", "热点", "news", "latest",
	}

	// 通用意图
	c.intentKeywords[types.IntentGreet] = []string{
		"你好", "您好", "hello", "hi", "嗨", "早上好", "下午好", "晚上好",
	}
	c.intentKeywords[types.IntentHelp] = []string{
		"帮助", "help", "怎么用", "如何使用", "功能", "能做什么",
	}
	c.intentKeywords[types.IntentConfirm] = []string{
		"是", "是的", "对", "好的", "可以", "ok", "yes", "确认",
	}
	c.intentKeywords[types.IntentCancel] = []string{
		"取消", "不要", "不用", "算了", "no", "cancel",
	}
}

// Predict 预测
func (c *SimpleClassifier) Predict(ctx context.Context, text string) (*PredictionResult, error) {
	textLower := strings.ToLower(text)
	
	// 计算每个意图的匹配分数
	scores := make(map[string]float64)
	
	for intentName, keywords := range c.intentKeywords {
		for _, kw := range keywords {
			if strings.Contains(textLower, strings.ToLower(kw)) {
				// 根据关键词长度给分（更长的关键词匹配更精确）
				score := float64(len(kw)) / float64(len(text)) * 0.5
				if score < 0.3 {
					score = 0.3
				}
				if score > 0.8 {
					score = 0.8
				}
				if scores[intentName] < score {
					scores[intentName] = score
				}
			}
		}
	}

	// 找出最佳匹配
	var labels []string
	var scoreValues []float64
	var bestLabel string
	var bestScore float64

	for label, score := range scores {
		labels = append(labels, label)
		scoreValues = append(scoreValues, score)
		if score > bestScore {
			bestScore = score
			bestLabel = label
		}
	}

	if bestLabel == "" {
		bestLabel = types.IntentUnknown
		bestScore = 0
	}

	return &PredictionResult{
		Labels:    labels,
		Scores:    scoreValues,
		BestLabel: bestLabel,
		BestScore: bestScore,
	}, nil
}

// Load 加载模型（简单分类器不需要）
func (c *SimpleClassifier) Load(modelPath string) error {
	return nil
}

// Close 关闭
func (c *SimpleClassifier) Close() error {
	return nil
}
