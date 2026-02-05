package intent

import (
	"testing"
)

func TestAnalyzer_AnalyzeEmail(t *testing.T) {
	analyzer := NewAnalyzer()

	tests := []struct {
		name        string
		subject     string
		content     string
		attachments []string
		wantIntent  IntentType
	}{
		{
			name:       "财务发票",
			subject:    "您的电子发票",
			content:    "您好，附件是本月的电子发票，金额￥1234.56",
			attachments: []string{"invoice.pdf"},
			wantIntent: IntentFinanceInvoice,
		},
		{
			name:       "工作会议",
			subject:    "会议邀请：项目评审会议",
			content:    "邀请您参加明天下午的项目评审会议，会议室A301",
			attachments: []string{"meeting.ics"},
			wantIntent: IntentWorkMeeting,
		},
		{
			name:       "验证码",
			subject:    "您的验证码",
			content:    "您的验证码是：123456，5分钟内有效，请勿泄露给他人",
			attachments: nil,
			wantIntent: IntentNotifySystem,
		},
		{
			name:       "快递通知",
			subject:    "您的快递已发货",
			content:    "您的包裹已发货，快递单号：SF1234567890，预计3天内送达",
			attachments: nil,
			wantIntent: IntentPersonalDelivery,
		},
		{
			name:       "工作报告",
			subject:    "本周工作周报",
			content:    "本周主要完成了以下工作：1. 项目A开发 2. 文档整理",
			attachments: []string{"report.xlsx"},
			wantIntent: IntentWorkReport,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := analyzer.AnalyzeEmail(tt.subject, tt.content, tt.attachments)
			if result.PrimaryIntent != tt.wantIntent {
				t.Errorf("AnalyzeEmail() primary intent = %v, want %v", result.PrimaryIntent, tt.wantIntent)
			}
		})
	}
}

func TestAnalyzer_ParseSearchQuery(t *testing.T) {
	analyzer := NewAnalyzer()

	tests := []struct {
		name          string
		query         string
		wantIntents   []IntentType
		wantTimeRange bool
	}{
		{
			name:          "发票搜索",
			query:         "查找最近的报销发票",
			wantIntents:   []IntentType{IntentFinanceInvoice, IntentFinanceReimburse},
			wantTimeRange: true,
		},
		{
			name:          "会议搜索",
			query:         "本周的会议邀请",
			wantIntents:   []IntentType{IntentWorkMeeting},
			wantTimeRange: true,
		},
		{
			name:          "快递搜索",
			query:         "查找快递物流信息",
			wantIntents:   []IntentType{IntentPersonalDelivery},
			wantTimeRange: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := analyzer.ParseSearchQuery(tt.query)

			// 检查意图
			for _, wantIntent := range tt.wantIntents {
				found := false
				for _, gotIntent := range result.ParsedIntents {
					if gotIntent == wantIntent {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("ParseSearchQuery() missing intent %v in %v", wantIntent, result.ParsedIntents)
				}
			}

			// 检查时间范围
			if tt.wantTimeRange && result.TimeRange == nil {
				t.Error("ParseSearchQuery() expected time range, got nil")
			}
		})
	}
}

func TestSubjectParser_Parse(t *testing.T) {
	parser := NewSubjectParser()

	tests := []struct {
		name      string
		subject   string
		wantReply bool
		wantFwd   bool
		wantPrio  string
	}{
		{
			name:      "普通邮件",
			subject:   "项目进度更新",
			wantReply: false,
			wantFwd:   false,
			wantPrio:  "normal",
		},
		{
			name:      "回复邮件",
			subject:   "Re: 项目进度更新",
			wantReply: true,
			wantFwd:   false,
			wantPrio:  "normal",
		},
		{
			name:      "转发邮件",
			subject:   "Fwd: 项目进度更新",
			wantReply: false,
			wantFwd:   true,
			wantPrio:  "normal",
		},
		{
			name:      "紧急邮件",
			subject:   "【紧急】服务器故障",
			wantReply: false,
			wantFwd:   false,
			wantPrio:  "high",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.Parse(tt.subject)

			if result.IsReply != tt.wantReply {
				t.Errorf("Parse() IsReply = %v, want %v", result.IsReply, tt.wantReply)
			}

			if result.IsForward != tt.wantFwd {
				t.Errorf("Parse() IsForward = %v, want %v", result.IsForward, tt.wantFwd)
			}

			if result.Priority != tt.wantPrio {
				t.Errorf("Parse() Priority = %v, want %v", result.Priority, tt.wantPrio)
			}
		})
	}
}

func TestContentParser_Parse(t *testing.T) {
	parser := NewContentParser()

	tests := []struct {
		name         string
		content      string
		wantEntities []string
	}{
		{
			name:         "包含验证码",
			content:      "您的验证码是：123456",
			wantEntities: []string{"verify_code"},
		},
		{
			name:         "包含金额",
			content:      "订单金额：￥199.00",
			wantEntities: []string{"money"},
		},
		{
			name:         "包含日期",
			content:      "会议时间：2024-01-15",
			wantEntities: []string{"date"},
		},
		{
			name:         "包含快递单号",
			content:      "快递单号：SF1234567890123",
			wantEntities: []string{"tracking_no"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.Parse(tt.content)

			for _, entity := range tt.wantEntities {
				if _, ok := result.Entities[entity]; !ok {
					t.Errorf("Parse() missing entity %s", entity)
				}
			}
		})
	}
}

func TestAttachmentParser_Parse(t *testing.T) {
	parser := NewAttachmentParser()

	tests := []struct {
		name        string
		attachments []string
		wantDoc     bool
		wantCal     bool
		wantImage   bool
	}{
		{
			name:        "文档附件",
			attachments: []string{"report.pdf", "data.xlsx"},
			wantDoc:     true,
			wantCal:     false,
			wantImage:   false,
		},
		{
			name:        "日历附件",
			attachments: []string{"meeting.ics"},
			wantDoc:     false,
			wantCal:     true,
			wantImage:   false,
		},
		{
			name:        "图片附件",
			attachments: []string{"invoice.jpg", "receipt.png"},
			wantDoc:     false,
			wantCal:     false,
			wantImage:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.Parse(tt.attachments)

			if result.HasDocument != tt.wantDoc {
				t.Errorf("Parse() HasDocument = %v, want %v", result.HasDocument, tt.wantDoc)
			}

			if result.HasCalendar != tt.wantCal {
				t.Errorf("Parse() HasCalendar = %v, want %v", result.HasCalendar, tt.wantCal)
			}

			if result.HasImage != tt.wantImage {
				t.Errorf("Parse() HasImage = %v, want %v", result.HasImage, tt.wantImage)
			}
		})
	}
}
