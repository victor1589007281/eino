// Package email 邮件工具
package email

import (
	"context"
	"fmt"
	"sync"

	"github.com/cloudwego/eino/vdocstool/config"
	"github.com/cloudwego/eino/vdocstool/mcp"
	"github.com/cloudwego/eino/vdocstool/tools/email/intent"
)

// EmailTool 邮件工具
type EmailTool struct {
	config            config.EmailConfig
	analyzer          *intent.Analyzer
	attachmentManager *AttachmentManager
	
	// 连接池
	connections map[string]*Connection
	connMu      sync.RWMutex
}

// Connection 邮箱连接
type Connection struct {
	Provider string
	Username string
	IMAP     *IMAPClient
	SMTP     *SMTPClient
}

// NewEmailTool 创建邮件工具
func NewEmailTool(cfg config.EmailConfig) (*EmailTool, error) {
	return &EmailTool{
		config:            cfg,
		analyzer:          intent.NewAnalyzer(),
		attachmentManager: NewAttachmentManager(cfg.AttachmentDir),
		connections:       make(map[string]*Connection),
	}, nil
}

// Register 注册到 MCP Server
func (t *EmailTool) Register(server *mcp.Server) {
	// 注册基于意图的邮件搜索
	searchIntentTool := mcp.NewToolBuilder("search_email_by_intent", "基于自然语言意图搜索邮件").
		AddProperty("intent_query", "string", "自然语言查询，如：查找最近的报销发票", true).
		AddProperty("provider", "string", "邮箱提供商(qq/163/126/gmail)", false).
		AddProperty("username", "string", "邮箱账号", true).
		AddProperty("password", "string", "邮箱密码/授权码", true).
		AddProperty("folder", "string", "邮件文件夹，默认INBOX", false).
		AddProperty("limit", "integer", "最大结果数，默认20", false).
		Build()

	server.RegisterTool(searchIntentTool, t.handleSearchByIntent)

	// 注册邮件分析
	analyzeTool := mcp.NewToolBuilder("analyze_email", "分析单封邮件的意图和内容").
		AddProperty("provider", "string", "邮箱提供商", false).
		AddProperty("username", "string", "邮箱账号", true).
		AddProperty("password", "string", "邮箱密码/授权码", true).
		AddProperty("folder", "string", "邮件文件夹", false).
		AddProperty("email_uid", "integer", "邮件UID", true).
		Build()

	server.RegisterTool(analyzeTool, t.handleAnalyzeEmail)

	// 注册附件查找
	findAttachmentsTool := mcp.NewToolBuilder("find_attachments", "按意图查找邮件附件").
		AddProperty("intent_query", "string", "意图查询，如：查找发票附件", true).
		AddProperty("provider", "string", "邮箱提供商", false).
		AddProperty("username", "string", "邮箱账号", true).
		AddProperty("password", "string", "邮箱密码/授权码", true).
		AddProperty("folder", "string", "邮件文件夹", false).
		AddProperty("limit", "integer", "最大结果数", false).
		Build()

	server.RegisterTool(findAttachmentsTool, t.handleFindAttachments)

	// 注册邮件读取
	readTool := mcp.NewToolBuilder("read_email", "读取邮件详情").
		AddProperty("provider", "string", "邮箱提供商", false).
		AddProperty("username", "string", "邮箱账号", true).
		AddProperty("password", "string", "邮箱密码/授权码", true).
		AddProperty("folder", "string", "邮件文件夹", false).
		AddProperty("email_uid", "integer", "邮件UID", true).
		Build()

	server.RegisterTool(readTool, t.handleReadEmail)

	// 注册邮件发送
	sendTool := mcp.NewToolBuilder("send_email", "发送邮件").
		AddProperty("provider", "string", "邮箱提供商", false).
		AddProperty("username", "string", "邮箱账号", true).
		AddProperty("password", "string", "邮箱密码/授权码", true).
		AddProperty("to", "string", "收件人，多个用逗号分隔", true).
		AddProperty("subject", "string", "邮件主题", true).
		AddProperty("body", "string", "邮件正文", true).
		AddProperty("is_html", "boolean", "是否为HTML格式", false).
		Build()

	server.RegisterTool(sendTool, t.handleSendEmail)

	// 注册附件下载
	downloadTool := mcp.NewToolBuilder("download_attachment", "下载邮件附件").
		AddProperty("provider", "string", "邮箱提供商", false).
		AddProperty("username", "string", "邮箱账号", true).
		AddProperty("password", "string", "邮箱密码/授权码", true).
		AddProperty("folder", "string", "邮件文件夹", false).
		AddProperty("email_uid", "integer", "邮件UID", true).
		AddProperty("part_id", "string", "附件部分ID", false).
		Build()

	server.RegisterTool(downloadTool, t.handleDownloadAttachment)
}

// getConnection 获取或创建连接
func (t *EmailTool) getConnection(ctx context.Context, provider, username, password string) (*Connection, error) {
	if provider == "" {
		provider = t.config.DefaultProvider
	}

	key := fmt.Sprintf("%s:%s", provider, username)

	t.connMu.RLock()
	conn, exists := t.connections[key]
	t.connMu.RUnlock()

	if exists {
		return conn, nil
	}

	// 获取提供商配置
	providerCfg, ok := t.config.Providers[provider]
	if !ok {
		return nil, fmt.Errorf("unknown provider: %s", provider)
	}

	// 创建IMAP客户端
	imapClient := NewIMAPClient(providerCfg, username, password)
	if err := imapClient.Connect(ctx); err != nil {
		return nil, fmt.Errorf("IMAP connect failed: %w", err)
	}

	// 创建SMTP客户端
	smtpClient := NewSMTPClient(providerCfg, username, password)

	conn = &Connection{
		Provider: provider,
		Username: username,
		IMAP:     imapClient,
		SMTP:     smtpClient,
	}

	t.connMu.Lock()
	t.connections[key] = conn
	t.connMu.Unlock()

	return conn, nil
}

// handleSearchByIntent 处理基于意图的搜索
func (t *EmailTool) handleSearchByIntent(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	intentQuery, _ := args["intent_query"].(string)
	provider, _ := args["provider"].(string)
	username, _ := args["username"].(string)
	password, _ := args["password"].(string)
	folder, _ := args["folder"].(string)
	if folder == "" {
		folder = "INBOX"
	}

	limit := 20
	if v, ok := args["limit"].(float64); ok && v > 0 {
		limit = int(v)
	}

	// 获取连接
	conn, err := t.getConnection(ctx, provider, username, password)
	if err != nil {
		return nil, err
	}

	// 解析搜索意图
	searchQuery := t.analyzer.ParseSearchQuery(intentQuery)

	// 构建邮件过滤器
	filter := &EmailFilter{}
	if searchQuery.TimeRange != nil {
		filter.Since = searchQuery.TimeRange.Since
		filter.Before = searchQuery.TimeRange.Before
	}
	if len(searchQuery.SubjectKeywords) > 0 {
		filter.Subject = searchQuery.SubjectKeywords[0]
	}

	// 获取邮件列表
	emails, err := conn.IMAP.FetchEmails(ctx, folder, filter, limit*2)
	if err != nil {
		return nil, err
	}

	// 根据意图过滤和排序
	var matchedEmails []*EmailSearchResult
	for _, email := range emails {
		// 分析邮件
		var attachmentNames []string
		if email.HasAttachment {
			attachmentNames = []string{"(has attachments)"}
		}

		analysis := t.analyzer.AnalyzeEmail(email.Subject, "", attachmentNames)

		// 检查意图匹配
		if len(searchQuery.ParsedIntents) > 0 {
			matched := false
			for _, queryIntent := range searchQuery.ParsedIntents {
				if analysis.PrimaryIntent == queryIntent {
					matched = true
					break
				}
				for _, secondaryIntent := range analysis.SecondaryIntents {
					if secondaryIntent == queryIntent {
						matched = true
						break
					}
				}
			}
			if !matched {
				continue
			}
		}

		matchedEmails = append(matchedEmails, &EmailSearchResult{
			Email:           email,
			IntentAnalysis:  analysis,
			MatchScore:      analysis.Confidence,
		})

		if len(matchedEmails) >= limit {
			break
		}
	}

	return SearchByIntentResponse{
		Query:        intentQuery,
		ParsedQuery:  searchQuery,
		Results:      matchedEmails,
		TotalMatched: len(matchedEmails),
	}, nil
}

// EmailSearchResult 邮件搜索结果
type EmailSearchResult struct {
	Email          *EmailSummary         `json:"email"`
	IntentAnalysis *intent.AnalysisResult `json:"intent_analysis"`
	MatchScore     float64               `json:"match_score"`
}

// SearchByIntentResponse 意图搜索响应
type SearchByIntentResponse struct {
	Query        string                `json:"query"`
	ParsedQuery  *intent.SearchQuery   `json:"parsed_query"`
	Results      []*EmailSearchResult  `json:"results"`
	TotalMatched int                   `json:"total_matched"`
}

// handleAnalyzeEmail 处理邮件分析
func (t *EmailTool) handleAnalyzeEmail(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	provider, _ := args["provider"].(string)
	username, _ := args["username"].(string)
	password, _ := args["password"].(string)
	folder, _ := args["folder"].(string)
	if folder == "" {
		folder = "INBOX"
	}
	emailUID, _ := args["email_uid"].(float64)

	// 获取连接
	conn, err := t.getConnection(ctx, provider, username, password)
	if err != nil {
		return nil, err
	}

	// 获取邮件详情
	detail, err := conn.IMAP.FetchEmailDetail(ctx, folder, uint32(emailUID))
	if err != nil {
		return nil, err
	}

	// 提取附件名称
	var attachmentNames []string
	for _, att := range detail.Attachments {
		attachmentNames = append(attachmentNames, att.Filename)
	}

	// 分析邮件
	analysis := t.analyzer.AnalyzeEmail(detail.Subject, detail.Body, attachmentNames)

	return AnalyzeEmailResponse{
		Email:    detail,
		Analysis: analysis,
	}, nil
}

// AnalyzeEmailResponse 邮件分析响应
type AnalyzeEmailResponse struct {
	Email    *EmailDetail          `json:"email"`
	Analysis *intent.AnalysisResult `json:"analysis"`
}

// handleFindAttachments 处理附件查找
func (t *EmailTool) handleFindAttachments(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	intentQuery, _ := args["intent_query"].(string)
	provider, _ := args["provider"].(string)
	username, _ := args["username"].(string)
	password, _ := args["password"].(string)
	folder, _ := args["folder"].(string)
	if folder == "" {
		folder = "INBOX"
	}

	limit := 20
	if v, ok := args["limit"].(float64); ok && v > 0 {
		limit = int(v)
	}

	// 获取连接
	conn, err := t.getConnection(ctx, provider, username, password)
	if err != nil {
		return nil, err
	}

	// 解析查询
	searchQuery := t.analyzer.ParseSearchQuery(intentQuery)

	// 获取邮件列表
	filter := &EmailFilter{}
	if searchQuery.TimeRange != nil {
		filter.Since = searchQuery.TimeRange.Since
	}

	emails, err := conn.IMAP.FetchEmails(ctx, folder, filter, limit*3)
	if err != nil {
		return nil, err
	}

	// 过滤有附件的邮件并获取详情
	var results []*AttachmentSearchResult
	for _, email := range emails {
		if !email.HasAttachment {
			continue
		}

		// 获取邮件详情
		detail, err := conn.IMAP.FetchEmailDetail(ctx, folder, email.UID)
		if err != nil {
			continue
		}

		// 检查附件是否匹配
		for _, att := range detail.Attachments {
			// 根据附件类型过滤
			if len(searchQuery.AttachmentTypes) > 0 {
				matched := false
				for _, wantType := range searchQuery.AttachmentTypes {
					if matchAttachmentType(att.Filename, att.ContentType, wantType) {
						matched = true
						break
					}
				}
				if !matched {
					continue
				}
			}

			results = append(results, &AttachmentSearchResult{
				Attachment: att,
				EmailUID:   email.UID,
				EmailSubject: email.Subject,
				EmailFrom:  email.From,
				EmailDate:  email.Date,
			})

			if len(results) >= limit {
				break
			}
		}

		if len(results) >= limit {
			break
		}
	}

	return FindAttachmentsResponse{
		Query:   intentQuery,
		Results: results,
		Total:   len(results),
	}, nil
}

// AttachmentSearchResult 附件搜索结果
type AttachmentSearchResult struct {
	Attachment   *AttachmentInfo `json:"attachment"`
	EmailUID     uint32          `json:"email_uid"`
	EmailSubject string          `json:"email_subject"`
	EmailFrom    string          `json:"email_from"`
	EmailDate    interface{}     `json:"email_date"`
}

// FindAttachmentsResponse 附件查找响应
type FindAttachmentsResponse struct {
	Query   string                   `json:"query"`
	Results []*AttachmentSearchResult `json:"results"`
	Total   int                      `json:"total"`
}

// matchAttachmentType 匹配附件类型
func matchAttachmentType(filename, contentType, wantType string) bool {
	switch wantType {
	case "pdf":
		return contentType == "application/pdf" || hasExt(filename, ".pdf")
	case "excel":
		return hasExt(filename, ".xls", ".xlsx")
	case "word":
		return hasExt(filename, ".doc", ".docx")
	case "image":
		return hasExt(filename, ".jpg", ".jpeg", ".png", ".gif")
	case "ics":
		return hasExt(filename, ".ics")
	}
	return false
}

func hasExt(filename string, exts ...string) bool {
	for _, ext := range exts {
		if len(filename) > len(ext) && filename[len(filename)-len(ext):] == ext {
			return true
		}
	}
	return false
}

// handleReadEmail 处理邮件读取
func (t *EmailTool) handleReadEmail(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	provider, _ := args["provider"].(string)
	username, _ := args["username"].(string)
	password, _ := args["password"].(string)
	folder, _ := args["folder"].(string)
	if folder == "" {
		folder = "INBOX"
	}
	emailUID, _ := args["email_uid"].(float64)

	// 获取连接
	conn, err := t.getConnection(ctx, provider, username, password)
	if err != nil {
		return nil, err
	}

	return conn.IMAP.FetchEmailDetail(ctx, folder, uint32(emailUID))
}

// handleSendEmail 处理邮件发送
func (t *EmailTool) handleSendEmail(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	provider, _ := args["provider"].(string)
	username, _ := args["username"].(string)
	password, _ := args["password"].(string)
	to, _ := args["to"].(string)
	subject, _ := args["subject"].(string)
	body, _ := args["body"].(string)
	isHTML, _ := args["is_html"].(bool)

	// 获取连接
	conn, err := t.getConnection(ctx, provider, username, password)
	if err != nil {
		return nil, err
	}

	// 解析收件人
	toList := splitEmails(to)

	// 发送邮件
	err = conn.SMTP.SendEmail(ctx, &SendEmailRequest{
		To:      toList,
		Subject: subject,
		Body:    body,
		IsHTML:  isHTML,
	})

	if err != nil {
		return &SendEmailResponse{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	return &SendEmailResponse{
		Success: true,
	}, nil
}

func splitEmails(s string) []string {
	var result []string
	for _, part := range splitString(s, ",", ";") {
		part = trimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

func splitString(s string, seps ...string) []string {
	result := []string{s}
	for _, sep := range seps {
		var newResult []string
		for _, part := range result {
			for _, p := range splitBySep(part, sep) {
				newResult = append(newResult, p)
			}
		}
		result = newResult
	}
	return result
}

func splitBySep(s, sep string) []string {
	var result []string
	start := 0
	for i := 0; i <= len(s)-len(sep); i++ {
		if s[i:i+len(sep)] == sep {
			result = append(result, s[start:i])
			start = i + len(sep)
		}
	}
	result = append(result, s[start:])
	return result
}

func trimSpace(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}

// handleDownloadAttachment 处理附件下载
func (t *EmailTool) handleDownloadAttachment(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	provider, _ := args["provider"].(string)
	username, _ := args["username"].(string)
	password, _ := args["password"].(string)
	folder, _ := args["folder"].(string)
	if folder == "" {
		folder = "INBOX"
	}
	emailUID, _ := args["email_uid"].(float64)
	partID, _ := args["part_id"].(string)

	// 获取连接
	conn, err := t.getConnection(ctx, provider, username, password)
	if err != nil {
		return nil, err
	}

	// 下载附件
	data, filename, err := conn.IMAP.DownloadAttachment(ctx, folder, uint32(emailUID), partID)
	if err != nil {
		return nil, err
	}

	// 保存附件
	saved, err := t.attachmentManager.SaveAttachment(ctx, data, filename, uint32(emailUID))
	if err != nil {
		return nil, err
	}

	return saved, nil
}
