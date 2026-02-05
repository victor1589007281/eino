// Package providers Gmail配置
package providers

import "github.com/cloudwego/eino/vdocstool/config"

// GmailConfig Gmail配置
var GmailConfig = config.EmailProviderConfig{
	IMAPServer: "imap.gmail.com",
	IMAPPort:   993,
	SMTPServer: "smtp.gmail.com",
	SMTPPort:   587,
	UseSSL:     true,
}

// GmailHelp Gmail使用说明
const GmailHelp = `
Gmail配置说明:
1. 登录 Google 账户
2. 进入 账户设置 -> 安全性
3. 开启两步验证
4. 生成应用专用密码 (App Password)

连接信息:
- IMAP服务器: imap.gmail.com
- IMAP端口: 993 (SSL)
- SMTP服务器: smtp.gmail.com
- SMTP端口: 587 (STARTTLS)

注意:
- 需要使用应用专用密码，不能使用Google账户密码
- 需要开启"允许不太安全的应用"或使用OAuth2
- 国内访问可能需要代理

Gmail IMAP特殊文件夹:
- 收件箱: INBOX
- 已发送: [Gmail]/Sent Mail
- 草稿: [Gmail]/Drafts
- 垃圾邮件: [Gmail]/Spam
- 已删除: [Gmail]/Trash
- 所有邮件: [Gmail]/All Mail
`
