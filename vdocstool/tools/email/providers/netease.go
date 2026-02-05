// Package providers 网易邮箱配置
package providers

import "github.com/cloudwego/eino/vdocstool/config"

// NetEase163Config 163邮箱配置
var NetEase163Config = config.EmailProviderConfig{
	IMAPServer: "imap.163.com",
	IMAPPort:   993,
	SMTPServer: "smtp.163.com",
	SMTPPort:   465,
	UseSSL:     true,
}

// NetEase126Config 126邮箱配置
var NetEase126Config = config.EmailProviderConfig{
	IMAPServer: "imap.126.com",
	IMAPPort:   993,
	SMTPServer: "smtp.126.com",
	SMTPPort:   465,
	UseSSL:     true,
}

// NetEaseYeahConfig yeah.net邮箱配置
var NetEaseYeahConfig = config.EmailProviderConfig{
	IMAPServer: "imap.yeah.net",
	IMAPPort:   993,
	SMTPServer: "smtp.yeah.net",
	SMTPPort:   465,
	UseSSL:     true,
}

// NetEaseHelp 网易邮箱使用说明
const NetEaseHelp = `
网易邮箱(163/126/yeah.net)配置说明:
1. 登录网易邮箱网页版
2. 进入 设置 -> POP3/SMTP/IMAP
3. 开启 "IMAP/SMTP服务"
4. 设置客户端授权密码

连接信息 (163邮箱):
- IMAP服务器: imap.163.com
- IMAP端口: 993 (SSL)
- SMTP服务器: smtp.163.com
- SMTP端口: 465 (SSL) 或 994 (SSL)

连接信息 (126邮箱):
- IMAP服务器: imap.126.com
- IMAP端口: 993 (SSL)
- SMTP服务器: smtp.126.com
- SMTP端口: 465 (SSL)

注意:
- 需要使用客户端授权密码登录
- 首次使用需要在网页版开启IMAP服务
`
