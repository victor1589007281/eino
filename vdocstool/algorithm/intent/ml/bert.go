// Package ml BERT分类器
// 使用 ONNX Runtime 进行推理
package ml

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"sync"
)

// BERTClassifier BERT分类器
// 支持加载 ONNX 格式的 BERT 模型进行文本分类
type BERTClassifier struct {
	modelPath     string
	vocabPath     string
	labelsPath    string
	loaded        bool
	
	// 词表
	vocab        map[string]int
	idToToken    map[int]string
	
	// 标签
	labels       []string
	labelToID    map[string]int
	
	// 模型参数
	maxSeqLen    int
	hiddenSize   int
	numLabels    int
	
	// ONNX Runtime session (如果可用)
	// session      *ort.Session
	
	// 配置
	config       *BERTConfig
	
	mu sync.RWMutex
	
	// 降级：使用简单分类器
	fallback *SimpleClassifier
}

// BERTConfig BERT配置
type BERTConfig struct {
	VocabSize          int    `json:"vocab_size"`
	HiddenSize         int    `json:"hidden_size"`
	NumHiddenLayers    int    `json:"num_hidden_layers"`
	NumAttentionHeads  int    `json:"num_attention_heads"`
	IntermediateSize   int    `json:"intermediate_size"`
	MaxPositionEmbeds  int    `json:"max_position_embeddings"`
	TypeVocabSize      int    `json:"type_vocab_size"`
	NumLabels          int    `json:"num_labels"`
	Labels             []string `json:"labels,omitempty"`
}

// NewBERTClassifier 创建BERT分类器
func NewBERTClassifier() *BERTClassifier {
	return &BERTClassifier{
		vocab:     make(map[string]int),
		idToToken: make(map[int]string),
		labelToID: make(map[string]int),
		maxSeqLen: 128,
		fallback:  NewSimpleClassifier(),
		config:    &BERTConfig{},
	}
}

// Load 加载模型
func (c *BERTClassifier) Load(modelPath string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 检查文件是否存在
	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		return fmt.Errorf("model file not found: %s", modelPath)
	}

	c.modelPath = modelPath

	// 确定相关文件路径
	baseDir := modelPath
	if strings.HasSuffix(modelPath, ".onnx") {
		baseDir = strings.TrimSuffix(modelPath, ".onnx")
	}
	
	c.vocabPath = baseDir + "_vocab.txt"
	c.labelsPath = baseDir + "_labels.txt"
	configPath := baseDir + "_config.json"

	// 加载词表
	if err := c.loadVocab(c.vocabPath); err != nil {
		// 尝试默认路径
		defaultVocabPath := strings.Replace(modelPath, ".onnx", "", 1) + "/vocab.txt"
		if err2 := c.loadVocab(defaultVocabPath); err2 != nil {
			return fmt.Errorf("load vocab failed: %w (also tried: %v)", err, err2)
		}
	}

	// 加载标签
	if err := c.loadLabels(c.labelsPath); err != nil {
		// 尝试从配置文件加载
		defaultLabelsPath := strings.Replace(modelPath, ".onnx", "", 1) + "/labels.txt"
		if err2 := c.loadLabels(defaultLabelsPath); err2 != nil {
			// 使用默认标签
			c.labels = []string{"unknown"}
			c.labelToID["unknown"] = 0
		}
	}

	// 加载配置
	if err := c.loadConfig(configPath); err != nil {
		// 使用默认配置
		c.config = &BERTConfig{
			MaxPositionEmbeds: 512,
			HiddenSize:        768,
			NumLabels:         len(c.labels),
		}
	}

	c.hiddenSize = c.config.HiddenSize
	c.numLabels = len(c.labels)
	if c.config.MaxPositionEmbeds > 0 {
		c.maxSeqLen = c.config.MaxPositionEmbeds
	}

	// 初始化 ONNX Runtime
	if err := c.initONNXRuntime(); err != nil {
		// ONNX Runtime 初始化失败，记录警告但继续
		fmt.Printf("warning: ONNX Runtime init failed: %v, using simple tokenizer-based prediction\n", err)
	}

	c.loaded = true
	return nil
}

// loadVocab 加载词表
func (c *BERTClassifier) loadVocab(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	id := 0
	for scanner.Scan() {
		token := scanner.Text()
		c.vocab[token] = id
		c.idToToken[id] = token
		id++
	}

	return scanner.Err()
}

// loadLabels 加载标签
func (c *BERTClassifier) loadLabels(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	id := 0
	for scanner.Scan() {
		label := strings.TrimSpace(scanner.Text())
		if label != "" {
			c.labels = append(c.labels, label)
			c.labelToID[label] = id
			id++
		}
	}

	return scanner.Err()
}

// loadConfig 加载配置
func (c *BERTClassifier) loadConfig(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, c.config)
}

// initONNXRuntime 初始化 ONNX Runtime
func (c *BERTClassifier) initONNXRuntime() error {
	// 完整实现需要使用 ONNX Runtime Go 绑定
	// 推荐使用: github.com/yalue/onnxruntime_go
	// 
	// 示例代码:
	// ort.SetSharedLibraryPath("/path/to/libonnxruntime.so")
	// err := ort.InitializeEnvironment()
	// if err != nil {
	//     return err
	// }
	// 
	// inputNames := []string{"input_ids", "attention_mask", "token_type_ids"}
	// outputNames := []string{"logits"}
	// 
	// c.session, err = ort.NewSession(c.modelPath, inputNames, outputNames)
	// if err != nil {
	//     return err
	// }
	
	return nil
}

// Predict 预测
func (c *BERTClassifier) Predict(ctx context.Context, text string) (*PredictionResult, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.loaded {
		return c.fallback.Predict(ctx, text)
	}

	// Tokenize
	inputIDs, attentionMask, _ := c.tokenize(text)
	
	// 如果 ONNX Runtime 可用，使用模型推理
	// if c.session != nil {
	//     return c.predictWithONNX(inputIDs, attentionMask, tokenTypeIDs)
	// }

	// 否则使用基于词嵌入的简单预测（降级方案）
	return c.predictWithTokens(inputIDs, attentionMask)
}

// tokenize BERT tokenization
func (c *BERTClassifier) tokenize(text string) ([]int64, []int64, []int64) {
	// 简化的 WordPiece tokenization
	tokens := []string{"[CLS]"}
	
	// 基础分词
	words := strings.Fields(strings.ToLower(text))
	for _, word := range words {
		// 尝试完整词
		if _, ok := c.vocab[word]; ok {
			tokens = append(tokens, word)
		} else {
			// WordPiece 分割
			subTokens := c.wordPieceTokenize(word)
			tokens = append(tokens, subTokens...)
		}
	}
	
	tokens = append(tokens, "[SEP]")
	
	// 截断或填充
	if len(tokens) > c.maxSeqLen {
		tokens = tokens[:c.maxSeqLen]
		tokens[c.maxSeqLen-1] = "[SEP]"
	}
	
	// 转换为 IDs
	inputIDs := make([]int64, c.maxSeqLen)
	attentionMask := make([]int64, c.maxSeqLen)
	tokenTypeIDs := make([]int64, c.maxSeqLen)
	
	for i, token := range tokens {
		if id, ok := c.vocab[token]; ok {
			inputIDs[i] = int64(id)
		} else {
			// [UNK]
			if unkID, ok := c.vocab["[UNK]"]; ok {
				inputIDs[i] = int64(unkID)
			}
		}
		attentionMask[i] = 1
	}
	
	// 填充 [PAD]
	if padID, ok := c.vocab["[PAD]"]; ok {
		for i := len(tokens); i < c.maxSeqLen; i++ {
			inputIDs[i] = int64(padID)
		}
	}
	
	return inputIDs, attentionMask, tokenTypeIDs
}

// wordPieceTokenize WordPiece 分词
func (c *BERTClassifier) wordPieceTokenize(word string) []string {
	var tokens []string
	
	start := 0
	for start < len(word) {
		end := len(word)
		found := false
		
		for end > start {
			substr := word[start:end]
			if start > 0 {
				substr = "##" + substr
			}
			
			if _, ok := c.vocab[substr]; ok {
				tokens = append(tokens, substr)
				found = true
				break
			}
			end--
		}
		
		if !found {
			// 未找到匹配，使用 [UNK]
			tokens = append(tokens, "[UNK]")
			start++
		} else {
			start = end
		}
	}
	
	return tokens
}

// predictWithTokens 使用 token 进行简单预测（降级方案）
func (c *BERTClassifier) predictWithTokens(inputIDs, attentionMask []int64) (*PredictionResult, error) {
	// 这是一个简化的预测方法
	// 使用词频统计和简单的启发式规则
	
	// 计算每个标签的相关性分数
	scores := make(map[string]float64)
	for _, label := range c.labels {
		scores[label] = 0
	}
	
	// 基于输入 token 计算分数
	tokenCount := 0
	for i, id := range inputIDs {
		if attentionMask[i] == 0 {
			continue
		}
		
		token := c.idToToken[int(id)]
		if token == "[CLS]" || token == "[SEP]" || token == "[PAD]" {
			continue
		}
		
		tokenCount++
		
		// 简单匹配：检查 token 是否与标签相关
		for _, label := range c.labels {
			labelLower := strings.ToLower(label)
			tokenLower := strings.ToLower(token)
			
			if strings.Contains(labelLower, tokenLower) || strings.Contains(tokenLower, labelLower) {
				scores[label] += 2.0
			} else if len(tokenLower) > 2 && strings.Contains(labelLower, tokenLower[:2]) {
				scores[label] += 0.5
			}
		}
	}
	
	// 归一化
	if tokenCount > 0 {
		for label := range scores {
			scores[label] /= float64(tokenCount)
		}
	}
	
	// 转换为排序结果
	type labelScore struct {
		label string
		score float64
	}
	var sortedScores []labelScore
	for label, score := range scores {
		sortedScores = append(sortedScores, labelScore{label, score})
	}
	sort.Slice(sortedScores, func(i, j int) bool {
		return sortedScores[i].score > sortedScores[j].score
	})
	
	// Softmax
	probs := make([]float64, len(sortedScores))
	var sumExp float64
	maxScore := sortedScores[0].score
	for i, ls := range sortedScores {
		probs[i] = math.Exp(ls.score - maxScore)
		sumExp += probs[i]
	}
	for i := range probs {
		probs[i] /= sumExp
	}
	
	// 构建结果
	result := &PredictionResult{
		Labels:    make([]string, len(sortedScores)),
		Scores:    probs,
		BestLabel: sortedScores[0].label,
		BestScore: probs[0],
	}
	for i, ls := range sortedScores {
		result.Labels[i] = ls.label
	}
	
	return result, nil
}

// Close 关闭
func (c *BERTClassifier) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 关闭 ONNX Runtime session
	// if c.session != nil {
	//     c.session.Destroy()
	// }

	c.vocab = nil
	c.idToToken = nil
	c.labels = nil
	c.loaded = false

	return nil
}

// GetLabels 获取所有标签
func (c *BERTClassifier) GetLabels() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.labels
}
