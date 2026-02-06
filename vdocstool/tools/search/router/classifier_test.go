package router

import (
	"testing"
)

func TestQueryClassifier_Classify(t *testing.T) {
	classifier := NewQueryClassifier()

	tests := []struct {
		name     string
		query    string
		expected QueryType
	}{
		{
			name:     "技术查询 - Go语言",
			query:    "golang context package usage",
			expected: QueryTypeTechnical,
		},
		{
			name:     "技术查询 - API",
			query:    "how to use REST API authentication",
			expected: QueryTypeTechnical,
		},
		{
			name:     "新闻查询",
			query:    "latest news about AI today",
			expected: QueryTypeNews,
		},
		{
			name:     "学术查询",
			query:    "research paper on machine learning algorithms",
			expected: QueryTypeAcademic,
		},
		{
			name:     "购物查询",
			query:    "best price for iPhone 15 Pro buy",
			expected: QueryTypeShopping,
		},
		{
			name:     "招聘查询",
			query:    "software engineer jobs hiring salary",
			expected: QueryTypeJob,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifier.Classify(tt.query)
			if result.PrimaryType != tt.expected {
				t.Errorf("Classify(%q) = %v, want %v", tt.query, result.PrimaryType, tt.expected)
			}
		})
	}
}

func TestQueryClassifier_ClassifyResult(t *testing.T) {
	classifier := NewQueryClassifier()

	result := classifier.Classify("golang programming tutorial")

	// 检查结果结构
	if result == nil {
		t.Fatal("Classify returned nil")
	}

	if result.Confidence < 0 || result.Confidence > 1 {
		t.Errorf("Confidence %v should be between 0 and 1", result.Confidence)
	}

	if result.Features == nil {
		t.Error("Features should not be nil")
	}
}

func TestQueryClassifier_LanguageDetection(t *testing.T) {
	classifier := NewQueryClassifier()

	tests := []struct {
		name     string
		query    string
		expected string
	}{
		{
			name:     "英文查询",
			query:    "golang programming tutorial",
			expected: "en",
		},
		{
			name:     "中文查询",
			query:    "如何学习Go语言编程",
			expected: "zh",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifier.Classify(tt.query)
			if result.Language != tt.expected {
				t.Errorf("Language for %q = %s, want %s", tt.query, result.Language, tt.expected)
			}
		})
	}
}

func TestGetQueryTypeMatchScore(t *testing.T) {
	tests := []struct {
		name       string
		engineName string
		queryType  QueryType
		minScore   float64
		maxScore   float64
	}{
		{
			name:       "Serper技术查询",
			engineName: "serper",
			queryType:  QueryTypeTechnical,
			minScore:   0.8,
			maxScore:   1.0,
		},
		{
			name:       "Exa学术查询",
			engineName: "exa",
			queryType:  QueryTypeAcademic,
			minScore:   0.9,
			maxScore:   1.0,
		},
		{
			name:       "Tavily新闻查询",
			engineName: "tavily",
			queryType:  QueryTypeNews,
			minScore:   0.8,
			maxScore:   1.0,
		},
		{
			name:       "DuckDuckGo通用查询",
			engineName: "duckduckgo",
			queryType:  QueryTypeGeneral,
			minScore:   0.6,
			maxScore:   0.9,
		},
		{
			name:       "未知引擎",
			engineName: "unknown",
			queryType:  QueryTypeGeneral,
			minScore:   0.4,
			maxScore:   0.6,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := GetQueryTypeMatchScore(tt.engineName, tt.queryType)
			if score < tt.minScore || score > tt.maxScore {
				t.Errorf("GetQueryTypeMatchScore(%q, %v) = %v, want between %v and %v",
					tt.engineName, tt.queryType, score, tt.minScore, tt.maxScore)
			}
		})
	}
}

func TestQueryClassifier_CodePatternDetection(t *testing.T) {
	classifier := NewQueryClassifier()

	tests := []struct {
		name        string
		query       string
		hasCodeFlag bool
	}{
		{
			name:        "Go函数",
			query:       "func main() error handling",
			hasCodeFlag: true,
		},
		{
			name:        "Python函数",
			query:       "def calculate() python",
			hasCodeFlag: true,
		},
		{
			name:        "类定义",
			query:       "class UserService implementation",
			hasCodeFlag: true,
		},
		{
			name:        "普通查询",
			query:       "how to learn programming",
			hasCodeFlag: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifier.Classify(tt.query)
			hasCode, _ := result.Features["has_code_pattern"]
			if hasCode != tt.hasCodeFlag {
				t.Errorf("Code pattern for %q: got %v, want %v", tt.query, hasCode, tt.hasCodeFlag)
			}
		})
	}
}
