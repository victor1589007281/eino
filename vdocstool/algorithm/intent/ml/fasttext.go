// Package ml FastText分类器
package ml

import (
	"bufio"
	"context"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// FastTextClassifier FastText分类器
// 实现了一个纯 Go 的 FastText 文本分类器
// 支持加载 FastText 文本格式的模型文件 (.vec 或 .bin 转换后的文本格式)
type FastTextClassifier struct {
	modelPath string
	loaded    bool
	
	// 模型参数
	dim        int                      // 向量维度
	wordVecs   map[string][]float32     // 词向量
	labelVecs  map[string][]float32     // 标签向量 (__label__ 前缀)
	labels     []string                 // 所有标签
	
	// 配置
	minCount   int     // 最小词频
	ngrams     int     // n-gram 大小
	
	mu sync.RWMutex
	
	// 降级：使用简单分类器
	fallback *SimpleClassifier
}

// NewFastTextClassifier 创建FastText分类器
func NewFastTextClassifier() *FastTextClassifier {
	return &FastTextClassifier{
		wordVecs:  make(map[string][]float32),
		labelVecs: make(map[string][]float32),
		labels:    make([]string, 0),
		minCount:  1,
		ngrams:    2,
		fallback:  NewSimpleClassifier(),
	}
}

// Load 加载模型
func (c *FastTextClassifier) Load(modelPath string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 检查文件是否存在
	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		return fmt.Errorf("model file not found: %s", modelPath)
	}

	c.modelPath = modelPath

	// 尝试加载文本格式模型 (.vec)
	if strings.HasSuffix(modelPath, ".vec") {
		if err := c.loadTextModel(modelPath); err != nil {
			return fmt.Errorf("load text model: %w", err)
		}
		c.loaded = true
		return nil
	}

	// 尝试加载二进制格式模型 (.bin)
	if strings.HasSuffix(modelPath, ".bin") {
		if err := c.loadBinaryModel(modelPath); err != nil {
			// 二进制格式加载失败，尝试作为文本格式
			if err := c.loadTextModel(modelPath); err != nil {
				return fmt.Errorf("load model: %w", err)
			}
		}
		c.loaded = true
		return nil
	}

	// 默认尝试文本格式
	if err := c.loadTextModel(modelPath); err != nil {
		return fmt.Errorf("load model: %w", err)
	}

	c.loaded = true
	return nil
}

// loadTextModel 加载文本格式模型
func (c *FastTextClassifier) loadTextModel(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	
	// 读取第一行：词数量和维度
	if scanner.Scan() {
		firstLine := scanner.Text()
		parts := strings.Fields(firstLine)
		if len(parts) >= 2 {
			c.dim, _ = strconv.Atoi(parts[1])
		}
	}

	// 读取词向量
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) < c.dim+1 {
			continue
		}

		word := parts[0]
		vec := make([]float32, c.dim)
		for i := 0; i < c.dim && i+1 < len(parts); i++ {
			val, _ := strconv.ParseFloat(parts[i+1], 32)
			vec[i] = float32(val)
		}

		// 检查是否是标签
		if strings.HasPrefix(word, "__label__") {
			label := strings.TrimPrefix(word, "__label__")
			c.labelVecs[label] = vec
			c.labels = append(c.labels, label)
		} else {
			c.wordVecs[word] = vec
		}
	}

	if len(c.labels) == 0 {
		return fmt.Errorf("no labels found in model")
	}

	return scanner.Err()
}

// loadBinaryModel 加载二进制格式模型
// 注意：完整的二进制格式解析较复杂，这里提供简化版本
func (c *FastTextClassifier) loadBinaryModel(path string) error {
	// FastText 二进制格式解析需要更复杂的实现
	// 建议：使用 fasttext 命令行工具将 .bin 转换为 .vec
	// fasttext print-word-vectors model.bin < words.txt > model.vec
	return fmt.Errorf("binary model format not fully supported, please convert to text format using: fasttext print-word-vectors")
}

// Predict 预测
func (c *FastTextClassifier) Predict(ctx context.Context, text string) (*PredictionResult, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.loaded || len(c.labels) == 0 {
		// 未加载模型，使用降级
		return c.fallback.Predict(ctx, text)
	}

	// 预处理文本
	tokens := c.tokenize(text)
	
	// 计算文本向量（词向量平均）
	textVec := c.computeTextVector(tokens)
	if textVec == nil {
		return c.fallback.Predict(ctx, text)
	}

	// 计算与每个标签的相似度
	type labelScore struct {
		label string
		score float64
	}
	
	var scores []labelScore
	for label, labelVec := range c.labelVecs {
		sim := cosineSimilarityF32(textVec, labelVec)
		scores = append(scores, labelScore{label, sim})
	}

	// 按分数排序
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].score > scores[j].score
	})

	if len(scores) == 0 {
		return c.fallback.Predict(ctx, text)
	}

	// 转换类型以匹配 softmax 函数签名
	softmaxScores := make([]struct{ label string; score float64 }, len(scores))
	for i, s := range scores {
		softmaxScores[i] = struct{ label string; score float64 }{s.label, s.score}
	}

	// 转换为概率（softmax）
	probs := softmax(softmaxScores)

	// 构建结果
	result := &PredictionResult{
		Labels:    make([]string, len(probs)),
		Scores:    probs,
		BestLabel: scores[0].label,
		BestScore: probs[0],
	}

	for i, ls := range scores {
		result.Labels[i] = ls.label
	}

	return result, nil
}

// tokenize 分词
func (c *FastTextClassifier) tokenize(text string) []string {
	// 简单分词：按空格和标点
	text = strings.ToLower(text)
	var tokens []string
	
	var current strings.Builder
	for _, r := range text {
		if r == ' ' || r == '\t' || r == '\n' || r == ',' || r == '.' || 
		   r == '!' || r == '?' || r == '；' || r == '，' || r == '。' {
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
		} else {
			current.WriteRune(r)
		}
	}
	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}

	// 生成 n-grams
	if c.ngrams > 1 {
		var ngrams []string
		for i := 0; i < len(tokens); i++ {
			for n := 1; n <= c.ngrams && i+n <= len(tokens); n++ {
				ngram := strings.Join(tokens[i:i+n], " ")
				ngrams = append(ngrams, ngram)
			}
		}
		tokens = append(tokens, ngrams...)
	}

	return tokens
}

// computeTextVector 计算文本向量
func (c *FastTextClassifier) computeTextVector(tokens []string) []float32 {
	if c.dim == 0 {
		return nil
	}

	vec := make([]float32, c.dim)
	count := 0

	for _, token := range tokens {
		if wordVec, ok := c.wordVecs[token]; ok {
			for i := 0; i < c.dim; i++ {
				vec[i] += wordVec[i]
			}
			count++
		}
	}

	if count == 0 {
		return nil
	}

	// 平均
	for i := 0; i < c.dim; i++ {
		vec[i] /= float32(count)
	}

	return vec
}

// cosineSimilarityF32 计算余弦相似度
func cosineSimilarityF32(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}

	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i] * b[i])
		normA += float64(a[i] * a[i])
		normB += float64(b[i] * b[i])
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}

// softmax 将分数转换为概率
func softmax(scores []struct{ label string; score float64 }) []float64 {
	if len(scores) == 0 {
		return nil
	}

	// 找最大值用于数值稳定性
	maxScore := scores[0].score
	for _, s := range scores[1:] {
		if s.score > maxScore {
			maxScore = s.score
		}
	}

	// 计算 exp
	exps := make([]float64, len(scores))
	var sum float64
	for i, s := range scores {
		exps[i] = math.Exp(s.score - maxScore)
		sum += exps[i]
	}

	// 归一化
	probs := make([]float64, len(scores))
	for i := range exps {
		probs[i] = exps[i] / sum
	}

	return probs
}

// Close 关闭
func (c *FastTextClassifier) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.wordVecs = nil
	c.labelVecs = nil
	c.labels = nil
	c.loaded = false

	return nil
}

// GetLabels 获取所有标签
func (c *FastTextClassifier) GetLabels() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.labels
}

// GetDimension 获取向量维度
func (c *FastTextClassifier) GetDimension() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.dim
}
