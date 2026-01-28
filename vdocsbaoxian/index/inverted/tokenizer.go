package inverted

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// ChineseTokenizer implements Chinese text tokenization.
type ChineseTokenizer struct {
	customDict map[string]int
	stopWords  map[string]bool
}

// InsuranceDict contains insurance-specific terms.
var InsuranceDict = map[string]int{
	// 法规术语
	"保险法":       100,
	"保险条例":     100,
	"健康保险管理办法": 100,
	"人身保险":     100,
	"财产保险":     100,
	
	// 产品术语
	"犹豫期":       100,
	"等待期":       100,
	"免赔额":       100,
	"保险责任":     100,
	"除外责任":     100,
	"责任免除":     100,
	"保额":         100,
	"保费":         100,
	"保障期":       100,
	"缴费期":       100,
	"现金价值":     100,
	"退保":         100,
	"续保":         100,
	"观察期":       100,
	
	// 人员术语
	"投保人":       100,
	"被保险人":     100,
	"受益人":       100,
	"保险人":       100,
	
	// 险种术语
	"重疾险":       100,
	"重大疾病保险": 100,
	"医疗险":       100,
	"寿险":         100,
	"人寿保险":     100,
	"意外险":       100,
	"年金险":       100,
	"分红险":       100,
	"万能险":       100,
	"投连险":       100,
	
	// 理赔术语
	"理赔":         100,
	"报案":         100,
	"索赔":         100,
	"赔付":         100,
	"给付":         100,
	"核保":         100,
	"承保":         100,
	"拒赔":         100,
	
	// 疾病术语
	"甲状腺癌":     100,
	"原位癌":       100,
	"恶性肿瘤":     100,
	"心肌梗塞":     100,
	"脑中风":       100,
	"冠状动脉":     100,
	"糖尿病":       100,
	"高血压":       100,
	"心脏病":       100,
	"肝硬化":       100,
	"肾衰竭":       100,
	
	// 健康告知
	"如实告知":     100,
	"健康告知":     100,
	"既往症":       100,
	"带病投保":     100,
	"健康状况":     100,
	
	// 其他
	"保险合同":     100,
	"保险单":       100,
	"保险条款":     100,
	"特别约定":     100,
	"批单":         100,
}

// DefaultStopWords contains common Chinese stop words.
var DefaultStopWords = map[string]bool{
	"的": true, "是": true, "在": true, "了": true, "和": true,
	"与": true, "或": true, "等": true, "有": true, "这": true,
	"那": true, "个": true, "为": true, "以": true, "及": true,
	"其": true, "中": true, "上": true, "下": true, "对": true,
	"但": true, "也": true, "就": true, "而": true, "到": true,
	"从": true, "被": true, "把": true, "给": true, "让": true,
	"向": true, "往": true, "于": true, "由": true, "因": true,
	"所": true, "之": true, "则": true, "如": true, "若": true,
	"当": true, "将": true, "会": true, "能": true, "可": true,
	"应": true, "须": true, "需": true, "要": true, "请": true,
	"吗": true, "呢": true, "吧": true, "啊": true, "哦": true,
	"嗯": true, "哈": true, "呀": true, "啦": true, "哟": true,
}

// NewChineseTokenizer creates a new Chinese tokenizer.
func NewChineseTokenizer() *ChineseTokenizer {
	return &ChineseTokenizer{
		customDict: InsuranceDict,
		stopWords:  DefaultStopWords,
	}
}

// NewChineseTokenizerWithDict creates a tokenizer with custom dictionary.
func NewChineseTokenizerWithDict(customDict map[string]int, stopWords map[string]bool) *ChineseTokenizer {
	// Merge with default insurance dict
	dict := make(map[string]int)
	for k, v := range InsuranceDict {
		dict[k] = v
	}
	for k, v := range customDict {
		dict[k] = v
	}

	// Merge with default stop words
	stops := make(map[string]bool)
	for k, v := range DefaultStopWords {
		stops[k] = v
	}
	for k, v := range stopWords {
		stops[k] = v
	}

	return &ChineseTokenizer{
		customDict: dict,
		stopWords:  stops,
	}
}

// Tokenize tokenizes the text.
func (t *ChineseTokenizer) Tokenize(text string) []Token {
	if text == "" {
		return nil
	}

	tokens := make([]Token, 0)
	offset := 0

	// First, extract professional terms using dictionary matching
	remaining := text
	for len(remaining) > 0 {
		matched := false
		
		// Try to match longest term first
		for termLen := minInt(len(remaining), 20); termLen > 1; termLen-- {
			substr := remaining[:termLen]
			if _, ok := t.customDict[substr]; ok {
				tokens = append(tokens, Token{
					Term:           substr,
					Offset:         offset,
					Length:         len(substr),
					IsProfessional: true,
				})
				offset += len(substr)
				remaining = remaining[termLen:]
				matched = true
				break
			}
		}

		if !matched {
			// Get next character/word
			r, size := utf8.DecodeRuneInString(remaining)
			if r == utf8.RuneError {
				remaining = remaining[1:]
				offset++
				continue
			}

			char := remaining[:size]
			
			// Skip whitespace and punctuation
			if unicode.IsSpace(r) || unicode.IsPunct(r) {
				remaining = remaining[size:]
				offset += size
				continue
			}

			// For Chinese characters, treat each as a potential token
			if unicode.Is(unicode.Han, r) {
				// Try to form bi-grams for better matching
				if len(remaining) > size {
					nextR, nextSize := utf8.DecodeRuneInString(remaining[size:])
					if unicode.Is(unicode.Han, nextR) {
						bigram := remaining[:size+nextSize]
						if !t.stopWords[bigram] {
							tokens = append(tokens, Token{
								Term:           bigram,
								Offset:         offset,
								Length:         size + nextSize,
								IsProfessional: false,
							})
						}
					}
				}
				
				// Also add single character if not a stop word
				if !t.stopWords[char] {
					tokens = append(tokens, Token{
						Term:           char,
						Offset:         offset,
						Length:         size,
						IsProfessional: false,
					})
				}
			} else if unicode.IsLetter(r) || unicode.IsDigit(r) {
				// For English/numbers, collect the whole word
				end := size
				for end < len(remaining) {
					nextR, nextSize := utf8.DecodeRuneInString(remaining[end:])
					if !unicode.IsLetter(nextR) && !unicode.IsDigit(nextR) {
						break
					}
					end += nextSize
				}
				
				word := strings.ToLower(remaining[:end])
				if !t.stopWords[word] && len(word) > 1 {
					tokens = append(tokens, Token{
						Term:           word,
						Offset:         offset,
						Length:         end,
						IsProfessional: false,
					})
				}
				
				offset += end
				remaining = remaining[end:]
				continue
			}

			remaining = remaining[size:]
			offset += size
		}
	}

	// Deduplicate tokens while preserving order
	seen := make(map[string]bool)
	uniqueTokens := make([]Token, 0)
	for _, token := range tokens {
		key := token.Term + string(rune(token.Offset))
		if !seen[key] {
			seen[key] = true
			uniqueTokens = append(uniqueTokens, token)
		}
	}

	return uniqueTokens
}

// AddTerm adds a term to the dictionary.
func (t *ChineseTokenizer) AddTerm(term string, weight int) {
	t.customDict[term] = weight
}

// RemoveTerm removes a term from the dictionary.
func (t *ChineseTokenizer) RemoveTerm(term string) {
	delete(t.customDict, term)
}

// AddStopWord adds a stop word.
func (t *ChineseTokenizer) AddStopWord(word string) {
	t.stopWords[word] = true
}

// RemoveStopWord removes a stop word.
func (t *ChineseTokenizer) RemoveStopWord(word string) {
	delete(t.stopWords, word)
}

// IsProfessionalTerm checks if a term is a professional term.
func (t *ChineseTokenizer) IsProfessionalTerm(term string) bool {
	_, ok := t.customDict[term]
	return ok
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
