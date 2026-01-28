package grammar

import (
	"context"
	"testing"

	"github.com/cloudwego/eino/vdocswx/agent"
)

func TestGrammarAgent_Name(t *testing.T) {
	ga := NewGrammarAgent(nil)
	if ga.Name() != "grammar" {
		t.Errorf("Expected name 'grammar', got '%s'", ga.Name())
	}
}

func TestGrammarAgent_CanHandle(t *testing.T) {
	ga := NewGrammarAgent(nil)

	tests := []struct {
		articleType agent.ArticleType
		expected    bool
	}{
		{agent.ArticleTypeTech, true},
		{agent.ArticleTypeNonTech, true},
		{agent.ArticleTypeTutorial, true},
		{agent.ArticleTypeNews, true},
		{agent.ArticleTypeUnknown, true},
	}

	for _, tt := range tests {
		result := ga.CanHandle(tt.articleType)
		if result != tt.expected {
			t.Errorf("CanHandle(%v) = %v, expected %v", tt.articleType, result, tt.expected)
		}
	}
}

func TestGrammarAgent_applyRules_ChinesePunctuation(t *testing.T) {
	ga := NewGrammarAgent(nil)

	tests := []struct {
		name     string
		input    string
		expected string
		hasChange bool
	}{
		{
			name:     "Double punctuation",
			input:    "这是一个测试。。",
			expected: "这是一个测试。",
			hasChange: true,
		},
		{
			name:     "Chinese-English space",
			input:    "这是一个test测试",
			expected: "这是一个 test 测试",
			hasChange: true,
		},
		{
			name:     "Double de",
			input:    "这是的的一个测试",
			expected: "这是的一个测试",
			hasChange: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			changes, result := ga.applyRules(tt.input)
			
			if result != tt.expected {
				t.Errorf("applyRules(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
			
			if tt.hasChange && len(changes) == 0 {
				t.Errorf("Expected changes for input %q, but got none", tt.input)
			}
		})
	}
}

func TestGrammarAgent_CheckCommonErrors(t *testing.T) {
	ga := NewGrammarAgent(nil)

	tests := []struct {
		name      string
		input     string
		errorWord string
	}{
		{
			name:      "Account typo",
			input:     "请登录您的帐号",
			errorWord: "帐号",
		},
		{
			name:      "Login typo",
			input:     "请登陆系统",
			errorWord: "登陆",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			changes := ga.CheckCommonErrors(tt.input)
			
			found := false
			for _, change := range changes {
				if change.Original == tt.errorWord {
					found = true
					break
				}
			}
			
			if !found {
				t.Errorf("Expected to find error for '%s' in input '%s'", tt.errorWord, tt.input)
			}
		})
	}
}

func TestGrammarAgent_checkQuotePairs(t *testing.T) {
	ga := NewGrammarAgent(nil)

	tests := []struct {
		name        string
		input       string
		expectError bool
	}{
		{
			name:        "Balanced quotes",
			input:       "他说"你好"",
			expectError: false,
		},
		{
			name:        "Unbalanced quotes - missing right",
			input:       "他说"你好",
			expectError: true,
		},
		{
			name:        "Unbalanced quotes - missing left",
			input:       "他说你好"",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, changes := ga.checkQuotePairs(tt.input)
			
			hasError := len(changes) > 0
			if hasError != tt.expectError {
				t.Errorf("checkQuotePairs(%q) error = %v, expected %v", tt.input, hasError, tt.expectError)
			}
		})
	}
}

func TestGrammarAgent_Process_WithoutLLM(t *testing.T) {
	// 测试不依赖LLM的基础功能
	ga := NewGrammarAgent(nil)
	ctx := context.Background()

	input := "这是一个帐号登陆的功能,用户可以做为管理员登陆系统."
	
	result, err := ga.Process(ctx, input, agent.ArticleTypeNonTech)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result, got nil")
	}

	if result.AgentName != "grammar" {
		t.Errorf("Expected agent name 'grammar', got '%s'", result.AgentName)
	}

	// 应该检测到一些基础规则错误
	// 注意：由于LLM为nil，LLM检查会失败，但规则检查应该能工作
}
