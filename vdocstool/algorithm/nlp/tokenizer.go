// Package nlp 分词器实现
package nlp

import (
	"context"
	"strings"
	"sync"
	"unicode"

	"github.com/cloudwego/eino/vdocstool/algorithm/types"
	"github.com/yanyiwu/gojieba"
)

// NewTokenizer 创建分词器
func NewTokenizer(cfg *types.TokenizerConfig) (Tokenizer, error) {
	switch cfg.Provider {
	case "jieba":
		return NewJiebaTokenizer(cfg)
	case "whitespace":
		return NewWhitespaceTokenizer(), nil
	default:
		// 默认尝试使用结巴分词，失败则降级到空格分词
		t, err := NewJiebaTokenizer(cfg)
		if err != nil {
			return NewWhitespaceTokenizer(), nil
		}
		return t, nil
	}
}

// JiebaTokenizer 结巴分词器
type JiebaTokenizer struct {
	config *types.TokenizerConfig
	jieba  *gojieba.Jieba
	mu     sync.RWMutex
}

// NewJiebaTokenizer 创建结巴分词器
func NewJiebaTokenizer(cfg *types.TokenizerConfig) (*JiebaTokenizer, error) {
	t := &JiebaTokenizer{
		config: cfg,
	}

	// 初始化结巴分词
	// 如果指定了字典路径，使用自定义字典
	if cfg.DictPath != "" {
		t.jieba = gojieba.NewJieba(cfg.DictPath)
	} else {
		// 使用默认字典
		t.jieba = gojieba.NewJieba()
	}

	return t, nil
}

// Tokenize 分词
func (t *JiebaTokenizer) Tokenize(ctx context.Context, text string) ([]*Token, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.jieba == nil {
		return nil, nil
	}

	// 使用搜索引擎模式分词（粒度较细）
	words := t.jieba.CutForSearch(text, true)
	
	var tokens []*Token
	pos := 0
	
	for _, word := range words {
		// 跳过空白
		word = strings.TrimSpace(word)
		if word == "" {
			continue
		}
		
		// 计算位置
		start := strings.Index(text[pos:], word)
		if start >= 0 {
			start += pos
			end := start + len(word)
			
			tokens = append(tokens, &Token{
				Text:   word,
				Start:  start,
				End:    end,
				Weight: calculateWeight(word),
				POS:    "", // gojieba 的 CutForSearch 不返回词性
			})
			
			pos = end
		}
	}
	
	return tokens, nil
}

// TokenizeWithPOS 带词性的分词
func (t *JiebaTokenizer) TokenizeWithPOS(ctx context.Context, text string) ([]*Token, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.jieba == nil {
		return nil, nil
	}

	// 使用词性标注
	wordTags := t.jieba.Tag(text)
	
	var tokens []*Token
	pos := 0
	
	for _, wt := range wordTags {
		parts := strings.Split(wt, "/")
		if len(parts) != 2 {
			continue
		}
		
		word := parts[0]
		tag := parts[1]
		
		word = strings.TrimSpace(word)
		if word == "" {
			continue
		}
		
		start := strings.Index(text[pos:], word)
		if start >= 0 {
			start += pos
			end := start + len(word)
			
			tokens = append(tokens, &Token{
				Text:   word,
				Start:  start,
				End:    end,
				Weight: calculateWeightWithPOS(word, tag),
				POS:    tag,
			})
			
			pos = end
		}
	}
	
	return tokens, nil
}

// ExtractKeywords 提取关键词 (TF-IDF)
func (t *JiebaTokenizer) ExtractKeywords(ctx context.Context, text string, topK int) ([]string, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.jieba == nil {
		return nil, nil
	}

	// 使用 TF-IDF 提取关键词
	keywords := t.jieba.ExtractWithWeight(text, topK)
	
	result := make([]string, 0, len(keywords))
	for _, kw := range keywords {
		result = append(result, kw.Word)
	}
	
	return result, nil
}

// AddWord 添加自定义词
func (t *JiebaTokenizer) AddWord(word string, freq int, tag string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.jieba != nil {
		t.jieba.AddWord(word)
	}
}

// Close 关闭
func (t *JiebaTokenizer) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.jieba != nil {
		t.jieba.Free()
		t.jieba = nil
	}
	return nil
}

// WhitespaceTokenizer 简单空格分词器
type WhitespaceTokenizer struct{}

// NewWhitespaceTokenizer 创建空格分词器
func NewWhitespaceTokenizer() *WhitespaceTokenizer {
	return &WhitespaceTokenizer{}
}

// Tokenize 分词
func (t *WhitespaceTokenizer) Tokenize(ctx context.Context, text string) ([]*Token, error) {
	var tokens []*Token
	
	// 按空格和标点分割
	var current strings.Builder
	var start int
	
	runes := []rune(text)
	for i, r := range runes {
		if unicode.IsSpace(r) || unicode.IsPunct(r) {
			if current.Len() > 0 {
				word := current.String()
				tokens = append(tokens, &Token{
					Text:   word,
					Start:  start,
					End:    i,
					Weight: calculateWeight(word),
				})
				current.Reset()
			}
			start = i + 1
		} else {
			if current.Len() == 0 {
				start = i
			}
			current.WriteRune(r)
		}
	}
	
	// 处理最后一个词
	if current.Len() > 0 {
		word := current.String()
		tokens = append(tokens, &Token{
			Text:   word,
			Start:  start,
			End:    len(runes),
			Weight: calculateWeight(word),
		})
	}
	
	return tokens, nil
}

// Close 关闭
func (t *WhitespaceTokenizer) Close() error {
	return nil
}

// calculateWeight 计算词权重（简单实现）
func calculateWeight(word string) float64 {
	// 停用词列表
	stopWords := map[string]bool{
		"的": true, "了": true, "是": true, "在": true, "我": true,
		"有": true, "和": true, "就": true, "不": true, "人": true,
		"都": true, "一": true, "个": true, "上": true, "也": true,
		"很": true, "到": true, "说": true, "要": true, "去": true,
		"你": true, "会": true, "着": true, "没有": true, "看": true,
		"好": true, "自己": true, "这": true, "那": true, "什么": true,
		"the": true, "a": true, "an": true, "is": true, "are": true,
		"to": true, "of": true, "and": true, "in": true, "that": true,
		"for": true, "it": true, "with": true, "as": true, "was": true,
		"be": true, "on": true, "at": true, "by": true, "this": true,
	}

	wordLower := strings.ToLower(word)
	
	// 停用词权重低
	if stopWords[wordLower] {
		return 0.1
	}
	
	// 单字符权重低
	runeCount := len([]rune(word))
	if runeCount == 1 {
		return 0.2
	}
	
	// 纯数字权重中等
	isNumber := true
	for _, r := range word {
		if !unicode.IsDigit(r) {
			isNumber = false
			break
		}
	}
	if isNumber {
		return 0.5
	}
	
	// 较长的词权重高
	if runeCount >= 4 {
		return 0.9
	} else if runeCount >= 2 {
		return 0.7
	}
	
	return 0.5
}

// calculateWeightWithPOS 根据词性计算权重
func calculateWeightWithPOS(word, pos string) float64 {
	// 基础权重
	base := calculateWeight(word)
	
	// 根据词性调整
	// 名词、动词、形容词权重较高
	switch {
	case strings.HasPrefix(pos, "n"): // 名词
		return base * 1.2
	case strings.HasPrefix(pos, "v"): // 动词
		return base * 1.1
	case strings.HasPrefix(pos, "a"): // 形容词
		return base * 1.0
	case strings.HasPrefix(pos, "d"): // 副词
		return base * 0.8
	case strings.HasPrefix(pos, "p"): // 介词
		return base * 0.3
	case strings.HasPrefix(pos, "c"): // 连词
		return base * 0.2
	case strings.HasPrefix(pos, "u"): // 助词
		return base * 0.1
	case strings.HasPrefix(pos, "x"): // 标点
		return 0
	default:
		return base
	}
}
