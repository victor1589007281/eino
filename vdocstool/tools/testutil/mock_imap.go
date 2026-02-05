// Package testutil IMAP Mock 服务
package testutil

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

// MockIMAPServer 模拟 IMAP 服务器
type MockIMAPServer struct {
	listener net.Listener
	addr     string
	mu       sync.RWMutex

	// 预设邮件数据
	mailboxes map[string][]*MockEmail

	// 认证信息
	validUsers map[string]string

	// 状态
	running bool
	done    chan struct{}
}

// MockEmail 模拟邮件
type MockEmail struct {
	UID           uint32
	SeqNum        uint32
	Subject       string
	From          string
	To            []string
	Date          time.Time
	Body          string
	HTMLBody      string
	Attachments   []*MockAttachment
	Flags         []string
	HasAttachment bool
}

// MockAttachment 模拟附件
type MockAttachment struct {
	Filename    string
	ContentType string
	Data        []byte
	Size        int
}

// NewMockIMAPServer 创建 Mock IMAP 服务器
func NewMockIMAPServer() *MockIMAPServer {
	return &MockIMAPServer{
		mailboxes:  make(map[string][]*MockEmail),
		validUsers: make(map[string]string),
		done:       make(chan struct{}),
	}
}

// AddUser 添加有效用户
func (s *MockIMAPServer) AddUser(username, password string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.validUsers[username] = password
}

// AddMailbox 添加邮箱
func (s *MockIMAPServer) AddMailbox(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.mailboxes[name] == nil {
		s.mailboxes[name] = []*MockEmail{}
	}
}

// AddEmail 添加邮件到邮箱
func (s *MockIMAPServer) AddEmail(mailbox string, email *MockEmail) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.mailboxes[mailbox] == nil {
		s.mailboxes[mailbox] = []*MockEmail{}
	}
	email.SeqNum = uint32(len(s.mailboxes[mailbox]) + 1)
	if email.UID == 0 {
		email.UID = email.SeqNum
	}
	s.mailboxes[mailbox] = append(s.mailboxes[mailbox], email)
}

// Start 启动服务器
func (s *MockIMAPServer) Start() error {
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
func (s *MockIMAPServer) Addr() string {
	return s.addr
}

// Stop 停止服务器
func (s *MockIMAPServer) Stop() {
	s.running = false
	if s.listener != nil {
		s.listener.Close()
	}
	close(s.done)
}

// acceptConnections 接受连接
func (s *MockIMAPServer) acceptConnections() {
	for s.running {
		conn, err := s.listener.Accept()
		if err != nil {
			continue
		}
		go s.handleConnection(conn)
	}
}

// handleConnection 处理连接
func (s *MockIMAPServer) handleConnection(conn net.Conn) {
	defer conn.Close()

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	// 发送欢迎消息
	writer.WriteString("* OK Mock IMAP Server Ready\r\n")
	writer.Flush()

	var authenticated bool
	var selectedMailbox string
	tagCounter := 0

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, " ", 3)
		if len(parts) < 2 {
			continue
		}

		tag := parts[0]
		command := strings.ToUpper(parts[1])
		args := ""
		if len(parts) > 2 {
			args = parts[2]
		}

		tagCounter++

		switch command {
		case "CAPABILITY":
			writer.WriteString("* CAPABILITY IMAP4rev1 AUTH=PLAIN\r\n")
			writer.WriteString(fmt.Sprintf("%s OK CAPABILITY completed\r\n", tag))

		case "LOGIN":
			argParts := strings.Fields(args)
			if len(argParts) >= 2 {
				username := strings.Trim(argParts[0], "\"")
				password := strings.Trim(argParts[1], "\"")
				
				s.mu.RLock()
				expectedPass, exists := s.validUsers[username]
				s.mu.RUnlock()

				if exists && expectedPass == password {
					authenticated = true
					writer.WriteString(fmt.Sprintf("%s OK LOGIN completed\r\n", tag))
				} else {
					writer.WriteString(fmt.Sprintf("%s NO LOGIN failed\r\n", tag))
				}
			} else {
				writer.WriteString(fmt.Sprintf("%s BAD invalid arguments\r\n", tag))
			}

		case "LIST":
			if !authenticated {
				writer.WriteString(fmt.Sprintf("%s NO not authenticated\r\n", tag))
			} else {
				s.mu.RLock()
				for name := range s.mailboxes {
					writer.WriteString(fmt.Sprintf("* LIST () \"/\" %s\r\n", name))
				}
				s.mu.RUnlock()
				writer.WriteString(fmt.Sprintf("%s OK LIST completed\r\n", tag))
			}

		case "SELECT":
			if !authenticated {
				writer.WriteString(fmt.Sprintf("%s NO not authenticated\r\n", tag))
			} else {
				mailbox := strings.Trim(args, "\"")
				s.mu.RLock()
				emails, exists := s.mailboxes[mailbox]
				s.mu.RUnlock()

				if exists {
					selectedMailbox = mailbox
					writer.WriteString(fmt.Sprintf("* %d EXISTS\r\n", len(emails)))
					writer.WriteString("* 0 RECENT\r\n")
					writer.WriteString("* FLAGS (\\Seen \\Answered \\Flagged \\Deleted \\Draft)\r\n")
					writer.WriteString(fmt.Sprintf("%s OK [READ-WRITE] SELECT completed\r\n", tag))
				} else {
					writer.WriteString(fmt.Sprintf("%s NO mailbox not found\r\n", tag))
				}
			}

		case "SEARCH":
			if !authenticated || selectedMailbox == "" {
				writer.WriteString(fmt.Sprintf("%s NO select a mailbox first\r\n", tag))
			} else {
				s.mu.RLock()
				emails := s.mailboxes[selectedMailbox]
				s.mu.RUnlock()

				var seqNums []string
				for _, email := range emails {
					seqNums = append(seqNums, fmt.Sprintf("%d", email.SeqNum))
				}
				writer.WriteString(fmt.Sprintf("* SEARCH %s\r\n", strings.Join(seqNums, " ")))
				writer.WriteString(fmt.Sprintf("%s OK SEARCH completed\r\n", tag))
			}

		case "FETCH":
			if !authenticated || selectedMailbox == "" {
				writer.WriteString(fmt.Sprintf("%s NO select a mailbox first\r\n", tag))
			} else {
				s.handleFetch(writer, tag, args, selectedMailbox)
			}

		case "UID":
			if !authenticated || selectedMailbox == "" {
				writer.WriteString(fmt.Sprintf("%s NO select a mailbox first\r\n", tag))
			} else {
				subParts := strings.SplitN(args, " ", 2)
				if len(subParts) >= 2 && strings.ToUpper(subParts[0]) == "FETCH" {
					s.handleUIDFetch(writer, tag, subParts[1], selectedMailbox)
				} else {
					writer.WriteString(fmt.Sprintf("%s BAD invalid UID command\r\n", tag))
				}
			}

		case "LOGOUT":
			writer.WriteString("* BYE Mock IMAP server signing off\r\n")
			writer.WriteString(fmt.Sprintf("%s OK LOGOUT completed\r\n", tag))
			writer.Flush()
			return

		default:
			writer.WriteString(fmt.Sprintf("%s BAD unknown command\r\n", tag))
		}

		writer.Flush()
	}
}

// handleFetch 处理 FETCH 命令
func (s *MockIMAPServer) handleFetch(writer *bufio.Writer, tag, args, mailbox string) {
	s.mu.RLock()
	emails := s.mailboxes[mailbox]
	s.mu.RUnlock()

	for _, email := range emails {
		s.writeFetchResponse(writer, email)
	}
	writer.WriteString(fmt.Sprintf("%s OK FETCH completed\r\n", tag))
}

// handleUIDFetch 处理 UID FETCH 命令
func (s *MockIMAPServer) handleUIDFetch(writer *bufio.Writer, tag, args, mailbox string) {
	s.mu.RLock()
	emails := s.mailboxes[mailbox]
	s.mu.RUnlock()

	// 简化处理：返回所有邮件
	for _, email := range emails {
		s.writeFetchResponse(writer, email)
	}
	writer.WriteString(fmt.Sprintf("%s OK UID FETCH completed\r\n", tag))
}

// writeFetchResponse 写入 FETCH 响应
func (s *MockIMAPServer) writeFetchResponse(writer *bufio.Writer, email *MockEmail) {
	// 简化的 FETCH 响应
	flags := strings.Join(email.Flags, " ")
	from := email.From
	to := strings.Join(email.To, ", ")

	response := fmt.Sprintf(`* %d FETCH (UID %d FLAGS (%s) ENVELOPE ("%s" "%s" (("%s" NIL "user" "example.com")) (("%s" NIL "user" "example.com")) NIL (("%s" NIL "dest" "example.com")) NIL NIL NIL "<msg@example.com>"))`,
		email.SeqNum, email.UID, flags, email.Date.Format(time.RFC1123Z), email.Subject, from, from, to)
	
	writer.WriteString(response + "\r\n")
}

// GetMailboxEmails 获取邮箱中的邮件
func (s *MockIMAPServer) GetMailboxEmails(mailbox string) []*MockEmail {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.mailboxes[mailbox]
}
