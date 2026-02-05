// Package providers 邮箱提供商配置
package providers

import "github.com/cloudwego/eino/vdocstool/config"

// QQMailConfig QQ邮箱配置
var QQMailConfig = config.EmailProviderConfig{
	IMAPServer: "imap.qq.com",
	IMAPPort:   993,
	SMTPServer: "smtp.qq.com",
	SMTPPort:   465,
	UseSSL:     true,
}

// QQMailHelp QQ邮箱使用说明
const QQMailHelp = `
QQ邮箱配置说明:
1. 登录 QQ邮箱网页版 (mail.qq.com)
2. 进入 设置 -> 账户
3. 找到 "POP3/IMAP/SMTP/Exchange/CardDAV/CalDAV服务"
4. 开启 "IMAP/SMTP服务"
5. 获取授权码（密码栏使用授权码，非QQ密码）

连接信息:
- IMAP服务器: imap.qq.com
- IMAP端口: 993 (SSL)
- SMTP服务器: smtp.qq.com
- SMTP端口: 465 (SSL) 或 587 (STARTTLS)

注意:
- 需要使用授权码登录，不能使用QQ密码
- 授权码可以在QQ邮箱网页版设置中生成
`
