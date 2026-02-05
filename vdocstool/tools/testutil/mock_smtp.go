// Package testutil SMTP Mock 服务
package testutil

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

// MockSMTPServer 模拟 SMTP 服务器
type MockSMTPServer struct {
	listener net.Listener
	addr     string
	mu       sync.RWMutex

	// 认证信息
	validUsers map[string]string

	// 发送的邮件
	sentMails []*SentMail

	// 状态
	running bool
}

// SentMail 已发送的邮件
type SentMail struct {
	From      string
	To        []string
	Subject   string
	Body      string
	Headers   map[string]string
	Timestamp time.Time
}

// NewMockSMTPServer 创建 Mock SMTP 服务器
func NewMockSMTPServer() *MockSMTPServer {
	return &MockSMTPServer{
		validUsers: make(map[string]string),
		sentMails:  make([]*SentMail, 0),
	}
}

// AddUser 添加有效用户
func (s *MockSMTPServer) AddUser(username, password string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.validUsers[username] = password
}

// Start 启动服务器
func (s *MockSMTPServer) Start() error {
	var err error
	s.listener, err = net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}

	s.addr = s.listener.Addr().String()
	s.running = true

	go s.acceptConnections()
	return nil
}

// Addr 返回服务器地址
func (s *MockSMTPServer) Addr() string {
	return s.addr
}

// Stop 停止服务器
func (s *MockSMTPServer) Stop() {
	s.running = false
	if s.listener != nil {
		s.listener.Close()
	}
}

// acceptConnections 接受连接
func (s *MockSMTPServer) acceptConnections() {
	for s.running {
		conn, err := s.listener.Accept()
		if err != nil {
			continue
		}
		go s.handleConnection(conn)
	}
}

// handleConnection 处理连接
func (s *MockSMTPServer) handleConnection(conn net.Conn) {
	defer conn.Close()

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	// 发送欢迎消息
	writer.WriteString("220 Mock SMTP Server Ready\r\n")
	writer.Flush()

	var currentMail *SentMail
	var inData bool
	var dataBuffer strings.Builder

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}

		line = strings.TrimSpace(line)

		if inData {
			if line == "." {
				// 数据结束
				inData = false
				if currentMail != nil {
					currentMail.Body = dataBuffer.String()
					currentMail.Timestamp = time.Now()
					
					s.mu.Lock()
					s.sentMails = append(s.sentMails, currentMail)
					s.mu.Unlock()
				}
				writer.WriteString("250 OK message queued\r\n")
				writer.Flush()
				currentMail = nil
				dataBuffer.Reset()
				continue
			}
			dataBuffer.WriteString(line)
			dataBuffer.WriteString("\r\n")
			continue
		}

		parts := strings.SplitN(line, " ", 2)
		command := strings.ToUpper(parts[0])
		args := ""
		if len(parts) > 1 {
			args = parts[1]
		}

		switch command {
		case "HELO", "EHLO":
			writer.WriteString("250-Mock SMTP Server\r\n")
			writer.WriteString("250-AUTH PLAIN LOGIN\r\n")
			writer.WriteString("250 OK\r\n")

		case "AUTH":
			// 简化认证：总是成功
			writer.WriteString("235 Authentication successful\r\n")

		case "MAIL":
			// MAIL FROM:<address>
			from := strings.TrimPrefix(strings.ToUpper(args), "FROM:")
			from = strings.Trim(from, "<>")
			currentMail = &SentMail{
				From:    from,
				Headers: make(map[string]string),
			}
			writer.WriteString("250 OK\r\n")

		case "RCPT":
			// RCPT TO:<address>
			to := strings.TrimPrefix(strings.ToUpper(args), "TO:")
			to = strings.Trim(to, "<>")
			if currentMail != nil {
				currentMail.To = append(currentMail.To, to)
			}
			writer.WriteString("250 OK\r\n")

		case "DATA":
			inData = true
			writer.WriteString("354 Start mail input; end with <CRLF>.<CRLF>\r\n")

		case "QUIT":
			writer.WriteString("221 Bye\r\n")
			writer.Flush()
			return

		case "RSET":
			currentMail = nil
			dataBuffer.Reset()
			writer.WriteString("250 OK\r\n")

		case "NOOP":
			writer.WriteString("250 OK\r\n")

		default:
			writer.WriteString(fmt.Sprintf("500 Unknown command: %s\r\n", command))
		}

		writer.Flush()
	}
}

// GetSentMails 获取已发送的邮件
func (s *MockSMTPServer) GetSentMails() []*SentMail {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]*SentMail{}, s.sentMails...)
}

// ClearSentMails 清除已发送的邮件
func (s *MockSMTPServer) ClearSentMails() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sentMails = nil
}

// GetLastSentMail 获取最后发送的邮件
func (s *MockSMTPServer) GetLastSentMail() *SentMail {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.sentMails) == 0 {
		return nil
	}
	return s.sentMails[len(s.sentMails)-1]
}
