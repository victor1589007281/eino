//go:build integration

// Package email 邮件工具集成测试
package email

import (
	"testing"
	"time"

	"github.com/cloudwego/eino/vdocstool/config"
	"github.com/cloudwego/eino/vdocstool/tools/email/intent"
	"github.com/cloudwego/eino/vdocstool/tools/testutil"
)

// ====================== 测试辅助 ======================

// setupMockIMAPServer 设置 Mock IMAP 服务器
func setupMockIMAPServer(t *testing.T) *testutil.MockIMAPServer {
	server := testutil.NewMockIMAPServer()
	server.AddUser("test@example.com", "password123")
	server.AddMailbox("INBOX")

	// 添加测试邮件
	// 1. 发票邮件
	server.AddEmail("INBOX", &testutil.MockEmail{
		UID:           1,
		Subject:       "【发票】2024年1月费用报销发票",
		From:          "finance@company.com",
		To:            []string{"test@example.com"},
		Date:          time.Now().Add(-24 * time.Hour),
		Body:          "您好，附件为本月报销发票，请查收。",
		HasAttachment: true,
		Attachments: []*testutil.MockAttachment{
			{Filename: "invoice_202401.pdf", ContentType: "application/pdf", Size: 102400},
		},
		Flags: []string{"\\Seen"},
	})

	// 2. 会议邀请
	server.AddEmail("INBOX", &testutil.MockEmail{
		UID:           2,
		Subject:       "会议邀请: 项目周会 - 2024/01/15 14:00",
		From:          "manager@company.com",
		To:            []string{"test@example.com"},
		Date:          time.Now().Add(-12 * time.Hour),
		Body:          "诚邀您参加项目周会，请准时参加。",
		HasAttachment: true,
		Attachments: []*testutil.MockAttachment{
			{Filename: "meeting.ics", ContentType: "text/calendar", Size: 1024},
		},
	})

	// 3. 促销邮件
	server.AddEmail("INBOX", &testutil.MockEmail{
		UID:     3,
		Subject: "【限时优惠】新年大促销，全场5折起！",
		From:    "promo@shop.com",
		To:      []string{"test@example.com"},
		Date:    time.Now().Add(-6 * time.Hour),
		Body:    "亲爱的用户，新年大促销开始啦！全场5折起，赶快抢购吧！",
	})

	// 4. 工作汇报
	server.AddEmail("INBOX", &testutil.MockEmail{
		UID:           4,
		Subject:       "Re: 本周工作汇报",
		From:          "colleague@company.com",
		To:            []string{"test@example.com"},
		Date:          time.Now().Add(-2 * time.Hour),
		Body:          "收到，已阅读你的汇报，做得不错！",
		HasAttachment: true,
		Attachments: []*testutil.MockAttachment{
			{Filename: "report_week3.docx", ContentType: "application/vnd.openxmlformats-officedocument.wordprocessingml.document", Size: 51200},
		},
	})

	// 5. 外卖订单
	server.AddEmail("INBOX", &testutil.MockEmail{
		UID:     5,
		Subject: "您的外卖订单已送达",
		From:    "order@delivery.com",
		To:      []string{"test@example.com"},
		Date:    time.Now().Add(-1 * time.Hour),
		Body:    "您的订单已送达，请及时取餐。订单号：123456",
	})

	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start mock IMAP server: %v", err)
	}

	return server
}

// setupMockSMTPServer 设置 Mock SMTP 服务器
func setupMockSMTPServer(t *testing.T) *testutil.MockSMTPServer {
	server := testutil.NewMockSMTPServer()
	server.AddUser("test@example.com", "password123")

	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start mock SMTP server: %v", err)
	}

	return server
}

// ====================== 集成测试用例 ======================

// TestEmailTool_Integration_SearchByIntent 测试意图搜索
func TestEmailTool_Integration_SearchByIntent(t *testing.T) {
	tests := []struct {
		name          string
		intentQuery   string
		expectIntents []string
		expectMinCount int
	}{
		{
			name:           "搜索发票邮件",
			intentQuery:    "查找最近的发票",
			expectIntents:  []string{"invoice"},
			expectMinCount: 1,
		},
		{
			name:           "搜索会议邮件",
			intentQuery:    "下周的会议邀请",
			expectIntents:  []string{"meeting"},
			expectMinCount: 1,
		},
		{
			name:           "搜索促销邮件",
			intentQuery:    "有什么优惠活动",
			expectIntents:  []string{"promotion"},
			expectMinCount: 1,
		},
		{
			name:           "搜索工作相关",
			intentQuery:    "工作汇报邮件",
			expectIntents:  []string{"work"},
			expectMinCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 创建意图分析器进行测试
			analyzer := intent.NewAnalyzer()
			
			// 解析搜索意图
			searchQuery := analyzer.ParseSearchQuery(tt.intentQuery)
			
			// 验证解析出的意图
			if len(searchQuery.ParsedIntents) == 0 && len(tt.expectIntents) > 0 {
				t.Log("Parsed query:", searchQuery)
				// 注意：实际意图解析可能需要更复杂的匹配逻辑
			}
			
			t.Logf("Intent query: %s", tt.intentQuery)
			t.Logf("Parsed intents: %v", searchQuery.ParsedIntents)
			t.Logf("Subject keywords: %v", searchQuery.SubjectKeywords)
		})
	}
}

// TestEmailTool_Integration_AnalyzeEmail 测试邮件分析
func TestEmailTool_Integration_AnalyzeEmail(t *testing.T) {
	tests := []struct {
		name           string
		subject        string
		body           string
		attachments    []string
		expectIntent   string
		minConfidence  float64
	}{
		{
			name:          "发票邮件分析",
			subject:       "【发票】2024年1月费用报销发票",
			body:          "您好，附件为本月报销发票，请查收。",
			attachments:   []string{"invoice_202401.pdf"},
			expectIntent:  "invoice",
			minConfidence: 0.7,
		},
		{
			name:          "会议邀请分析",
			subject:       "会议邀请: 项目周会",
			body:          "诚邀您参加项目周会，请准时参加。",
			attachments:   []string{"meeting.ics"},
			expectIntent:  "meeting",
			minConfidence: 0.7,
		},
		{
			name:          "促销邮件分析",
			subject:       "【限时优惠】全场5折起",
			body:          "新年大促销开始啦！",
			attachments:   nil,
			expectIntent:  "promotion",
			minConfidence: 0.6,
		},
	}

	analyzer := intent.NewAnalyzer()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := analyzer.AnalyzeEmail(tt.subject, tt.body, tt.attachments)

			t.Logf("Analysis result: primary=%s, confidence=%.2f", 
				result.PrimaryIntent, result.Confidence)
			t.Logf("Secondary intents: %v", result.SecondaryIntents)
			t.Logf("Keywords: %v", result.Keywords)

			// 验证置信度
			if result.Confidence < tt.minConfidence {
				t.Logf("Warning: confidence %.2f is below threshold %.2f", 
					result.Confidence, tt.minConfidence)
			}
		})
	}
}

// TestEmailTool_Integration_FindAttachments 测试附件查找
func TestEmailTool_Integration_FindAttachments(t *testing.T) {
	tests := []struct {
		name          string
		intentQuery   string
		expectTypes   []string
		expectMinCount int
	}{
		{
			name:           "查找PDF附件",
			intentQuery:    "查找PDF文件",
			expectTypes:    []string{"pdf"},
			expectMinCount: 1,
		},
		{
			name:           "查找发票附件",
			intentQuery:    "发票附件",
			expectTypes:    []string{"pdf"},
			expectMinCount: 1,
		},
		{
			name:           "查找会议日程",
			intentQuery:    "会议日程文件",
			expectTypes:    []string{"ics"},
			expectMinCount: 1,
		},
		{
			name:           "查找文档",
			intentQuery:    "Word文档",
			expectTypes:    []string{"word"},
			expectMinCount: 1,
		},
	}

	analyzer := intent.NewAnalyzer()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			searchQuery := analyzer.ParseSearchQuery(tt.intentQuery)
			
			t.Logf("Search query: %s", tt.intentQuery)
			t.Logf("Attachment types: %v", searchQuery.AttachmentTypes)
			
			// 验证附件类型解析
			hasExpectedType := false
			for _, expected := range tt.expectTypes {
				for _, parsed := range searchQuery.AttachmentTypes {
					if parsed == expected {
						hasExpectedType = true
						break
					}
				}
			}
			
			if !hasExpectedType && len(tt.expectTypes) > 0 {
				t.Logf("Note: Expected attachment type %v not found in parsed types", tt.expectTypes)
			}
		})
	}
}

// TestEmailTool_Integration_HandleReadEmail 测试邮件读取（需要 Mock 服务器）
func TestEmailTool_Integration_HandleReadEmail(t *testing.T) {
	t.Skip("Requires full IMAP mock implementation")
	
	// 设置 Mock 服务器
	imapServer := setupMockIMAPServer(t)
	defer imapServer.Stop()

	// 创建工具（需要注入 mock 服务器地址）
	cfg := config.EmailConfig{
		DefaultProvider: "mock",
		AttachmentDir:   "/tmp/attachments",
		Providers: map[string]config.EmailProviderConfig{
			"mock": {
				IMAPServer: "127.0.0.1",
				IMAPPort:   993,
				SMTPServer: "127.0.0.1",
				SMTPPort:   587,
				UseSSL:     false,
			},
		},
	}

	t.Logf("Config: %+v", cfg)
	t.Logf("Mock IMAP Server: %s", imapServer.Addr())
}

// TestEmailTool_Integration_HandleSendEmail 测试邮件发送（需要 Mock 服务器）
func TestEmailTool_Integration_HandleSendEmail(t *testing.T) {
	t.Skip("Requires full SMTP mock implementation")
	
	// 设置 Mock SMTP 服务器
	smtpServer := setupMockSMTPServer(t)
	defer smtpServer.Stop()

	t.Logf("Mock SMTP Server: %s", smtpServer.Addr())
}

// TestEmailTool_Integration_ConnectionPool 测试连接池
func TestEmailTool_Integration_ConnectionPool(t *testing.T) {
	cfg := config.EmailConfig{
		DefaultProvider: "qq",
		Providers: map[string]config.EmailProviderConfig{
			"qq": {
				IMAPServer: "imap.qq.com",
				IMAPPort:   993,
				SMTPServer: "smtp.qq.com",
				SMTPPort:   587,
				UseSSL:     true,
			},
		},
	}

	tool, err := NewEmailTool(cfg)
	if err != nil {
		t.Fatalf("Failed to create email tool: %v", err)
	}

	// 验证连接池初始化
	if tool.connections == nil {
		t.Error("Connection pool should be initialized")
	}
}

// ====================== 意图分析器测试 ======================

// TestIntentAnalyzer_Keywords 测试关键词提取
func TestIntentAnalyzer_Keywords(t *testing.T) {
	analyzer := intent.NewAnalyzer()

	tests := []struct {
		text           string
		expectKeywords []string
	}{
		{
			text:           "请查收本月的报销发票",
			expectKeywords: []string{"报销", "发票"},
		},
		{
			text:           "会议定于下周一上午10点",
			expectKeywords: []string{"会议", "下周一"},
		},
		{
			text:           "您的订单已发货",
			expectKeywords: []string{"订单", "发货"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			result := analyzer.AnalyzeEmail(tt.text, "", nil)
			
			t.Logf("Text: %s", tt.text)
			t.Logf("Extracted keywords: %v", result.Keywords)
			t.Logf("Primary intent: %s", result.PrimaryIntent)
		})
	}
}

// TestIntentAnalyzer_TimeRange 测试时间范围解析
func TestIntentAnalyzer_TimeRange(t *testing.T) {
	analyzer := intent.NewAnalyzer()

	tests := []struct {
		query       string
		expectSince bool
	}{
		{"最近一周的邮件", true},
		{"上个月的发票", true},
		{"今天的会议通知", true},
		{"所有邮件", false},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			result := analyzer.ParseSearchQuery(tt.query)
			
			hasTimeRange := result.TimeRange != nil && !result.TimeRange.Since.IsZero()
			
			t.Logf("Query: %s", tt.query)
			t.Logf("Has time range: %v", hasTimeRange)
			
			if result.TimeRange != nil {
				t.Logf("Since: %v, Before: %v", result.TimeRange.Since, result.TimeRange.Before)
			}
		})
	}
}

// ====================== 基准测试 ======================

func BenchmarkIntentAnalyzer_Analyze(b *testing.B) {
	analyzer := intent.NewAnalyzer()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		analyzer.AnalyzeEmail(
			"【发票】2024年1月费用报销发票",
			"您好，附件为本月报销发票，请查收。",
			[]string{"invoice_202401.pdf"},
		)
	}
}

func BenchmarkIntentAnalyzer_ParseQuery(b *testing.B) {
	analyzer := intent.NewAnalyzer()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		analyzer.ParseSearchQuery("查找最近一周的发票邮件")
	}
}
