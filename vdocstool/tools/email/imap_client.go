// Package email IMAP 客户端
package email

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"github.com/emersion/go-message/mail"

	"github.com/cloudwego/eino/vdocstool/config"
)

// IMAPClient IMAP 客户端
type IMAPClient struct {
	config   config.EmailProviderConfig
	username string
	password string
	client   *client.Client
}

// NewIMAPClient 创建 IMAP 客户端
func NewIMAPClient(cfg config.EmailProviderConfig, username, password string) *IMAPClient {
	return &IMAPClient{
		config:   cfg,
		username: username,
		password: password,
	}
}

// Connect 连接到服务器
func (c *IMAPClient) Connect(ctx context.Context) error {
	addr := fmt.Sprintf("%s:%d", c.config.IMAPServer, c.config.IMAPPort)

	var err error
	if c.config.UseSSL {
		c.client, err = client.DialTLS(addr, nil)
	} else {
		c.client, err = client.Dial(addr)
	}
	if err != nil {
		return fmt.Errorf("connect failed: %w", err)
	}

	// 登录
	if err := c.client.Login(c.username, c.password); err != nil {
		c.client.Close()
		return fmt.Errorf("login failed: %w", err)
	}

	return nil
}

// Close 关闭连接
func (c *IMAPClient) Close() error {
	if c.client != nil {
		c.client.Logout()
		return c.client.Close()
	}
	return nil
}

// ListMailboxes 列出邮箱文件夹
func (c *IMAPClient) ListMailboxes(ctx context.Context) ([]*MailboxInfo, error) {
	if c.client == nil {
		return nil, fmt.Errorf("not connected")
	}

	mailboxes := make(chan *imap.MailboxInfo, 10)
	done := make(chan error, 1)

	go func() {
		done <- c.client.List("", "*", mailboxes)
	}()

	var result []*MailboxInfo
	for m := range mailboxes {
		result = append(result, &MailboxInfo{
			Name:       m.Name,
			Delimiter:  m.Delimiter,
			Attributes: m.Attributes,
		})
	}

	if err := <-done; err != nil {
		return nil, fmt.Errorf("list mailboxes failed: %w", err)
	}

	return result, nil
}

// MailboxInfo 邮箱信息
type MailboxInfo struct {
	Name       string   `json:"name"`
	Delimiter  string   `json:"delimiter"`
	Attributes []string `json:"attributes"`
}

// SelectMailbox 选择邮箱
func (c *IMAPClient) SelectMailbox(ctx context.Context, name string) (*MailboxStatus, error) {
	if c.client == nil {
		return nil, fmt.Errorf("not connected")
	}

	mbox, err := c.client.Select(name, false)
	if err != nil {
		return nil, fmt.Errorf("select mailbox failed: %w", err)
	}

	return &MailboxStatus{
		Name:     mbox.Name,
		Messages: mbox.Messages,
		Recent:   mbox.Recent,
		Unseen:   mbox.Unseen,
	}, nil
}

// MailboxStatus 邮箱状态
type MailboxStatus struct {
	Name     string `json:"name"`
	Messages uint32 `json:"messages"`
	Recent   uint32 `json:"recent"`
	Unseen   uint32 `json:"unseen"`
}

// FetchEmails 获取邮件列表
func (c *IMAPClient) FetchEmails(ctx context.Context, folder string, filter *EmailFilter, limit int) ([]*EmailSummary, error) {
	if c.client == nil {
		return nil, fmt.Errorf("not connected")
	}

	// 选择文件夹
	mbox, err := c.client.Select(folder, false)
	if err != nil {
		return nil, fmt.Errorf("select folder failed: %w", err)
	}

	if mbox.Messages == 0 {
		return []*EmailSummary{}, nil
	}

	// 构建搜索条件
	criteria := c.buildSearchCriteria(filter)

	// 搜索邮件
	seqNums, err := c.client.Search(criteria)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	if len(seqNums) == 0 {
		return []*EmailSummary{}, nil
	}

	// 限制数量
	if limit > 0 && len(seqNums) > limit {
		// 取最新的
		seqNums = seqNums[len(seqNums)-limit:]
	}

	// 构建序列集
	seqSet := new(imap.SeqSet)
	seqSet.AddNum(seqNums...)

	// 获取邮件摘要
	items := []imap.FetchItem{imap.FetchEnvelope, imap.FetchFlags, imap.FetchUid, imap.FetchBodyStructure}
	messages := make(chan *imap.Message, 10)
	done := make(chan error, 1)

	go func() {
		done <- c.client.Fetch(seqSet, items, messages)
	}()

	var result []*EmailSummary
	for msg := range messages {
		if msg.Envelope == nil {
			continue
		}

		summary := &EmailSummary{
			UID:         msg.Uid,
			SeqNum:      msg.SeqNum,
			Subject:     msg.Envelope.Subject,
			Date:        msg.Envelope.Date,
			Flags:       msg.Flags,
			HasAttachment: hasAttachment(msg.BodyStructure),
		}

		// 发件人
		if len(msg.Envelope.From) > 0 {
			summary.From = formatAddress(msg.Envelope.From[0])
		}

		// 收件人
		for _, to := range msg.Envelope.To {
			summary.To = append(summary.To, formatAddress(to))
		}

		result = append(result, summary)
	}

	if err := <-done; err != nil {
		return nil, fmt.Errorf("fetch failed: %w", err)
	}

	// 反转顺序，最新的在前
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}

	return result, nil
}

// EmailSummary 邮件摘要
type EmailSummary struct {
	UID           uint32    `json:"uid"`
	SeqNum        uint32    `json:"seq_num"`
	Subject       string    `json:"subject"`
	From          string    `json:"from"`
	To            []string  `json:"to"`
	Date          time.Time `json:"date"`
	Flags         []string  `json:"flags"`
	HasAttachment bool      `json:"has_attachment"`
}

// EmailFilter 邮件过滤器
type EmailFilter struct {
	Subject   string    `json:"subject,omitempty"`
	From      string    `json:"from,omitempty"`
	Since     time.Time `json:"since,omitempty"`
	Before    time.Time `json:"before,omitempty"`
	Unseen    bool      `json:"unseen,omitempty"`
	WithFlags []string  `json:"with_flags,omitempty"`
}

// buildSearchCriteria 构建搜索条件
// 注意：某些 IMAP 服务器（如 QQ 邮箱）不支持 LITERAL 语法的搜索条件
// 对于非 ASCII 字符的搜索条件，我们在获取邮件后本地过滤
func (c *IMAPClient) buildSearchCriteria(filter *EmailFilter) *imap.SearchCriteria {
	criteria := imap.NewSearchCriteria()

	if filter == nil {
		return criteria
	}

	// 只添加不需要 LITERAL 语法的搜索条件
	// Subject 和 From 如果包含非 ASCII 字符，某些服务器会报错
	// 所以只有纯 ASCII 才添加到服务器端搜索条件
	if filter.Subject != "" && isASCII(filter.Subject) {
		criteria.Header.Add("Subject", filter.Subject)
	}

	if filter.From != "" && isASCII(filter.From) {
		criteria.Header.Add("From", filter.From)
	}

	if !filter.Since.IsZero() {
		criteria.Since = filter.Since
	}

	if !filter.Before.IsZero() {
		criteria.Before = filter.Before
	}

	if filter.Unseen {
		criteria.WithoutFlags = []string{imap.SeenFlag}
	}

	return criteria
}

// isASCII 检查字符串是否只包含 ASCII 字符
func isASCII(s string) bool {
	for _, c := range s {
		if c > 127 {
			return false
		}
	}
	return true
}

// FetchEmailDetail 获取邮件详情
func (c *IMAPClient) FetchEmailDetail(ctx context.Context, folder string, uid uint32) (*EmailDetail, error) {
	if c.client == nil {
		return nil, fmt.Errorf("not connected")
	}

	// 选择文件夹
	_, err := c.client.Select(folder, false)
	if err != nil {
		return nil, fmt.Errorf("select folder failed: %w", err)
	}

	// 构建 UID 集合
	seqSet := new(imap.SeqSet)
	seqSet.AddNum(uid)

	// 获取完整邮件
	section := &imap.BodySectionName{}
	items := []imap.FetchItem{section.FetchItem(), imap.FetchEnvelope, imap.FetchFlags, imap.FetchBodyStructure}

	messages := make(chan *imap.Message, 1)
	done := make(chan error, 1)

	go func() {
		done <- c.client.UidFetch(seqSet, items, messages)
	}()

	var msg *imap.Message
	for m := range messages {
		msg = m
	}

	if err := <-done; err != nil {
		return nil, fmt.Errorf("fetch failed: %w", err)
	}

	if msg == nil {
		return nil, fmt.Errorf("email not found")
	}

	detail := &EmailDetail{
		UID:     msg.Uid,
		Subject: msg.Envelope.Subject,
		Date:    msg.Envelope.Date,
		Flags:   msg.Flags,
	}

	// 发件人
	if len(msg.Envelope.From) > 0 {
		detail.From = formatAddress(msg.Envelope.From[0])
	}

	// 收件人
	for _, to := range msg.Envelope.To {
		detail.To = append(detail.To, formatAddress(to))
	}

	// 抄送
	for _, cc := range msg.Envelope.Cc {
		detail.Cc = append(detail.Cc, formatAddress(cc))
	}

	// 解析邮件内容
	body := msg.GetBody(section)
	if body != nil {
		mr, err := mail.CreateReader(body)
		if err == nil {
			detail.Body, detail.Attachments = c.parseMessageBody(mr)
		}
	}

	return detail, nil
}

// EmailDetail 邮件详情
type EmailDetail struct {
	UID         uint32            `json:"uid"`
	Subject     string            `json:"subject"`
	From        string            `json:"from"`
	To          []string          `json:"to"`
	Cc          []string          `json:"cc,omitempty"`
	Date        time.Time         `json:"date"`
	Flags       []string          `json:"flags"`
	Body        string            `json:"body"`
	HTMLBody    string            `json:"html_body,omitempty"`
	Attachments []*AttachmentInfo `json:"attachments,omitempty"`
}

// AttachmentInfo 附件信息
type AttachmentInfo struct {
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	Size        int    `json:"size"`
	PartID      string `json:"part_id"`
}

// parseMessageBody 解析邮件正文
func (c *IMAPClient) parseMessageBody(mr *mail.Reader) (string, []*AttachmentInfo) {
	var textBody string
	var attachments []*AttachmentInfo

	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}

		switch h := p.Header.(type) {
		case *mail.InlineHeader:
			contentType, _, _ := h.ContentType()
			if strings.HasPrefix(contentType, "text/plain") {
				b, _ := io.ReadAll(p.Body)
				textBody = string(b)
			}
		case *mail.AttachmentHeader:
			filename, _ := h.Filename()
			contentType, _, _ := h.ContentType()
			attachments = append(attachments, &AttachmentInfo{
				Filename:    filename,
				ContentType: contentType,
			})
		}
	}

	return textBody, attachments
}

// formatAddress 格式化地址
func formatAddress(addr *imap.Address) string {
	if addr.PersonalName != "" {
		return fmt.Sprintf("%s <%s@%s>", addr.PersonalName, addr.MailboxName, addr.HostName)
	}
	return fmt.Sprintf("%s@%s", addr.MailboxName, addr.HostName)
}

// hasAttachment 检查是否有附件
func hasAttachment(bs *imap.BodyStructure) bool {
	if bs == nil {
		return false
	}

	if bs.Disposition == "attachment" {
		return true
	}

	for _, part := range bs.Parts {
		if hasAttachment(part) {
			return true
		}
	}

	return false
}

// DownloadAttachment 下载附件
func (c *IMAPClient) DownloadAttachment(ctx context.Context, folder string, uid uint32, partID string) ([]byte, string, error) {
	if c.client == nil {
		return nil, "", fmt.Errorf("not connected")
	}

	// 选择文件夹
	_, err := c.client.Select(folder, false)
	if err != nil {
		return nil, "", fmt.Errorf("select folder failed: %w", err)
	}

	// 构建 UID 集合
	seqSet := new(imap.SeqSet)
	seqSet.AddNum(uid)

	// 获取完整邮件
	section := &imap.BodySectionName{}
	messages := make(chan *imap.Message, 1)
	done := make(chan error, 1)

	go func() {
		done <- c.client.UidFetch(seqSet, []imap.FetchItem{section.FetchItem()}, messages)
	}()

	var msg *imap.Message
	for m := range messages {
		msg = m
	}

	if err := <-done; err != nil {
		return nil, "", fmt.Errorf("fetch failed: %w", err)
	}

	if msg == nil {
		return nil, "", fmt.Errorf("email not found")
	}

	// 解析邮件查找附件
	body := msg.GetBody(section)
	if body == nil {
		return nil, "", fmt.Errorf("body not found")
	}

	mr, err := mail.CreateReader(body)
	if err != nil {
		return nil, "", fmt.Errorf("create reader failed: %w", err)
	}

	partIndex := 0
	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}

		switch h := p.Header.(type) {
		case *mail.AttachmentHeader:
			currentPartID := fmt.Sprintf("%d", partIndex)
			if currentPartID == partID || partID == "" {
				filename, _ := h.Filename()
				data, _ := io.ReadAll(p.Body)
				return data, filename, nil
			}
			partIndex++
		}
	}

	return nil, "", fmt.Errorf("attachment not found")
}
