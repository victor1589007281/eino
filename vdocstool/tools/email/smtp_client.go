// Package email SMTP 客户端
package email

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/smtp"
	"strings"

	"github.com/cloudwego/eino/vdocstool/config"
)

// SMTPClient SMTP 客户端
type SMTPClient struct {
	config   config.EmailProviderConfig
	username string
	password string
}

// NewSMTPClient 创建 SMTP 客户端
func NewSMTPClient(cfg config.EmailProviderConfig, username, password string) *SMTPClient {
	return &SMTPClient{
		config:   cfg,
		username: username,
		password: password,
	}
}

// SendEmail 发送邮件
func (c *SMTPClient) SendEmail(ctx context.Context, req *SendEmailRequest) error {
	addr := fmt.Sprintf("%s:%d", c.config.SMTPServer, c.config.SMTPPort)

	// 构建邮件内容
	msg := c.buildMessage(req)

	// 根据端口选择连接方式
	if c.config.SMTPPort == 465 {
		// SSL 连接
		return c.sendWithSSL(addr, msg, req.To)
	}

	// STARTTLS 连接
	return c.sendWithSTARTTLS(addr, msg, req.To)
}

// sendWithSSL 使用 SSL 发送
func (c *SMTPClient) sendWithSSL(addr string, msg []byte, to []string) error {
	tlsConfig := &tls.Config{
		ServerName: c.config.SMTPServer,
	}

	conn, err := tls.Dial("tcp", addr, tlsConfig)
	if err != nil {
		return fmt.Errorf("dial failed: %w", err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, c.config.SMTPServer)
	if err != nil {
		return fmt.Errorf("create client failed: %w", err)
	}
	defer client.Close()

	// 认证
	auth := smtp.PlainAuth("", c.username, c.password, c.config.SMTPServer)
	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("auth failed: %w", err)
	}

	// 设置发件人
	if err := client.Mail(c.username); err != nil {
		return fmt.Errorf("mail failed: %w", err)
	}

	// 设置收件人
	for _, rcpt := range to {
		if err := client.Rcpt(rcpt); err != nil {
			return fmt.Errorf("rcpt failed: %w", err)
		}
	}

	// 写入邮件内容
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("data failed: %w", err)
	}

	_, err = w.Write(msg)
	if err != nil {
		return fmt.Errorf("write failed: %w", err)
	}

	if err := w.Close(); err != nil {
		return fmt.Errorf("close failed: %w", err)
	}

	return client.Quit()
}

// sendWithSTARTTLS 使用 STARTTLS 发送
func (c *SMTPClient) sendWithSTARTTLS(addr string, msg []byte, to []string) error {
	client, err := smtp.Dial(addr)
	if err != nil {
		return fmt.Errorf("dial failed: %w", err)
	}
	defer client.Close()

	// STARTTLS
	tlsConfig := &tls.Config{
		ServerName: c.config.SMTPServer,
	}
	if err := client.StartTLS(tlsConfig); err != nil {
		return fmt.Errorf("starttls failed: %w", err)
	}

	// 认证
	auth := smtp.PlainAuth("", c.username, c.password, c.config.SMTPServer)
	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("auth failed: %w", err)
	}

	// 设置发件人
	if err := client.Mail(c.username); err != nil {
		return fmt.Errorf("mail failed: %w", err)
	}

	// 设置收件人
	for _, rcpt := range to {
		if err := client.Rcpt(rcpt); err != nil {
			return fmt.Errorf("rcpt failed: %w", err)
		}
	}

	// 写入邮件内容
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("data failed: %w", err)
	}

	_, err = w.Write(msg)
	if err != nil {
		return fmt.Errorf("write failed: %w", err)
	}

	if err := w.Close(); err != nil {
		return fmt.Errorf("close failed: %w", err)
	}

	return client.Quit()
}

// buildMessage 构建邮件消息
func (c *SMTPClient) buildMessage(req *SendEmailRequest) []byte {
	var builder strings.Builder

	// 头部
	builder.WriteString(fmt.Sprintf("From: %s\r\n", c.username))
	builder.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(req.To, ", ")))

	if len(req.Cc) > 0 {
		builder.WriteString(fmt.Sprintf("Cc: %s\r\n", strings.Join(req.Cc, ", ")))
	}

	builder.WriteString(fmt.Sprintf("Subject: %s\r\n", req.Subject))
	builder.WriteString("MIME-Version: 1.0\r\n")

	if req.IsHTML {
		builder.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	} else {
		builder.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	}

	builder.WriteString("\r\n")
	builder.WriteString(req.Body)

	return []byte(builder.String())
}

// SendEmailRequest 发送邮件请求
type SendEmailRequest struct {
	To      []string `json:"to"`
	Cc      []string `json:"cc,omitempty"`
	Subject string   `json:"subject"`
	Body    string   `json:"body"`
	IsHTML  bool     `json:"is_html"`
}

// SendEmailResponse 发送邮件响应
type SendEmailResponse struct {
	Success   bool   `json:"success"`
	MessageID string `json:"message_id,omitempty"`
	Error     string `json:"error,omitempty"`
}
