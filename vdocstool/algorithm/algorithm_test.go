// Package algorithm 算法层测试
package algorithm

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/eino/vdocstool/algorithm/intent"
)

// skipIfNoExternalServices 检查是否缺少外部服务并跳过测试
func skipIfNoExternalServices(t *testing.T, err error) {
	if err == nil {
		return
	}
	errStr := err.Error()
	if strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "no such host") ||
		strings.Contains(errStr, "connect: connection") ||
		strings.Contains(errStr, "no elasticsearch addresses") {
		t.Skipf("skipping test - external service not available: %v", err)
	}
}

func TestIntentEngine(t *testing.T) {
	// 创建默认配置
	cfg := DefaultConfig()
	cfg.Intent.LLM.Enabled = false // 测试时禁用 LLM
	cfg.Intent.ML.Enabled = false  // 测试时禁用 ML
	cfg.NLP.Tokenizer.Provider = "whitespace" // 使用简单分词避免 gojieba 问题
	// 禁用索引以避免外部依赖
	cfg.Index.Elasticsearch.Addresses = nil
	cfg.Index.Milvus.Address = ""
	cfg.Index.Neo4j.URI = ""

	// 只测试规则引擎
	alg, err := New(cfg)
	skipIfNoExternalServices(t, err)
	if err != nil {
		t.Fatalf("create algorithm failed: %v", err)
	}
	defer alg.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 测试邮件意图识别
	testCases := []struct {
		name     string
		input    string
		domain   string
		expected string
	}{
		{
			name:     "search email",
			input:    "帮我找一下上周的发票邮件",
			domain:   intent.DomainEmail,
			expected: intent.IntentEmailSearch,
		},
		{
			name:     "read email",
			input:    "打开最新的邮件",
			domain:   intent.DomainEmail,
			expected: intent.IntentEmailRead,
		},
		{
			name:     "download attachment",
			input:    "下载那个PDF附件",
			domain:   intent.DomainEmail,
			expected: intent.IntentEmailDownload,
		},
		{
			name:     "memory reference",
			input:    "之前说的那个方案",
			domain:   intent.DomainMemory,
			expected: intent.IntentMemoryReference,
		},
		{
			name:     "topic switch",
			input:    "回到刚才那个话题",
			domain:   intent.DomainMemory,
			expected: intent.IntentMemoryTopicSwitch,
		},
		{
			name:     "web search",
			input:    "帮我搜索一下 Go 语言教程",
			domain:   intent.DomainSearch,
			expected: intent.IntentWebSearch,
		},
		{
			name:     "greeting",
			input:    "你好",
			domain:   intent.DomainGeneral,
			expected: intent.IntentGreet,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := alg.Intent.Recognize(ctx, &intent.Input{
				Text:   tc.input,
				Domain: tc.domain,
			})

			if err != nil {
				t.Errorf("recognize failed: %v", err)
				return
			}

			if result.Intent.Name != tc.expected {
				t.Errorf("expected intent %s, got %s (confidence: %.2f)",
					tc.expected, result.Intent.Name, result.Confidence)
			} else {
				t.Logf("✓ %s -> %s (confidence: %.2f, latency: %v)",
					tc.input, result.Intent.Name, result.Confidence, result.Latency)
			}
		})
	}
}

func TestEmbeddingEngine(t *testing.T) {
	// 跳过需要 API Key 的测试
	t.Skip("requires OpenAI API key")

	cfg := DefaultConfig()
	alg, err := New(cfg)
	if err != nil {
		t.Fatalf("create algorithm failed: %v", err)
	}
	defer alg.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 测试单文本向量化
	vector, err := alg.Embedding.Embed(ctx, "这是一段测试文本")
	if err != nil {
		t.Fatalf("embed failed: %v", err)
	}

	t.Logf("vector dimension: %d", len(vector))
	if len(vector) != cfg.Embedding.Dimension {
		t.Errorf("expected dimension %d, got %d", cfg.Embedding.Dimension, len(vector))
	}
}

func TestNLPPipeline(t *testing.T) {
	cfg := DefaultConfig()
	cfg.NLP.NER.Provider = "rule" // 使用规则 NER
	cfg.NLP.Summarizer.Provider = "extractive" // 使用抽取式摘要
	cfg.NLP.Tokenizer.Provider = "whitespace" // 使用简单分词避免 gojieba 问题
	// 禁用索引以避免外部依赖
	cfg.Index.Elasticsearch.Addresses = nil
	cfg.Index.Milvus.Address = ""
	cfg.Index.Neo4j.URI = ""
	cfg.Intent.LLM.Enabled = false
	cfg.Intent.ML.Enabled = false

	alg, err := New(cfg)
	skipIfNoExternalServices(t, err)
	if err != nil {
		t.Fatalf("create algorithm failed: %v", err)
	}
	defer alg.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	text := `张三在2024年1月15日参加了北京的技术大会。
会议讨论了Redis、MongoDB等技术的最新发展。
预计明年会有更多的技术分享。`

	// 测试分词
	tokens, err := alg.NLP.Tokenize(ctx, text)
	if err != nil {
		t.Errorf("tokenize failed: %v", err)
	} else {
		t.Logf("tokens count: %d", len(tokens))
	}

	// 测试 NER
	entities, err := alg.NLP.ExtractEntities(ctx, text)
	if err != nil {
		t.Errorf("extract entities failed: %v", err)
	} else {
		t.Logf("entities count: %d", len(entities))
		for _, e := range entities {
			t.Logf("  - %s (%s)", e.Text, e.Type)
		}
	}

	// 测试摘要
	summary, err := alg.NLP.Summarize(ctx, text, 50)
	if err != nil {
		t.Errorf("summarize failed: %v", err)
	} else {
		t.Logf("summary: %s", summary)
	}
}

func TestHealthCheck(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Intent.LLM.Enabled = false
	cfg.Intent.ML.Enabled = false
	cfg.Index.Elasticsearch.Addresses = nil // 禁用 ES
	cfg.Index.Milvus.Address = ""          // 禁用 Milvus
	cfg.Index.Neo4j.URI = ""               // 禁用 Neo4j
	cfg.NLP.Tokenizer.Provider = "whitespace" // 使用简单分词避免 gojieba 问题

	alg, err := New(cfg)
	skipIfNoExternalServices(t, err)
	if err != nil {
		t.Fatalf("create algorithm failed: %v", err)
	}
	defer alg.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	status := alg.HealthCheck(ctx)
	t.Logf("health status: %+v", status)
}

func BenchmarkIntentRecognition(b *testing.B) {
	cfg := DefaultConfig()
	cfg.Intent.LLM.Enabled = false
	cfg.Intent.ML.Enabled = false
	cfg.Index.Elasticsearch.Addresses = nil
	cfg.Index.Milvus.Address = ""
	cfg.Index.Neo4j.URI = ""

	alg, err := New(cfg)
	if err != nil {
		b.Skipf("skipping benchmark - external service not available: %v", err)
	}
	defer alg.Close()

	ctx := context.Background()
	input := &intent.Input{
		Text:   "帮我找一下上周的发票邮件",
		Domain: intent.DomainEmail,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = alg.Intent.Recognize(ctx, input)
	}
}
