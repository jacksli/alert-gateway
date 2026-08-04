package email

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"strings"

	"alert-gateway/internal/entity"
	"alert-gateway/internal/usecase/notifier"

	"github.com/yuin/goldmark"
)

const ProviderName = "email"

type EmailProvider struct {
	host     string
	port     int
	user     string
	password string
	from     string
}

func init() {
	notifier.Register(ProviderName, NewEmailProvider)
}

func NewEmailProvider(config map[string]interface{}) (notifier.Provider, error) {
	host, _ := config["smtp_host"].(string)
	user, _ := config["smtp_user"].(string)
	password, _ := config["smtp_pass"].(string)
	from, _ := config["from"].(string)

	var port int
	switch v := config["smtp_port"].(type) {
	case int:
		port = v
	case float64:
		port = int(v)
	default:
		port = 465
	}

	if host == "" || user == "" || password == "" {
		return nil, fmt.Errorf("email 渠道缺少必要配置 (smtp_host, smtp_user, smtp_pass)")
	}

	if from == "" {
		from = user
	}

	return &EmailProvider{
		host:     host,
		port:     port,
		user:     user,
		password: password,
		from:     from,
	}, nil
}

func (e *EmailProvider) Name() string {
	return ProviderName
}

func (e *EmailProvider) Send(ctx context.Context, n *entity.Notification) error {
	to := n.ReceiverIDs
	if len(to) == 0 {
		return fmt.Errorf("邮件收件人地址 (ReceiverIDs) 不能为空")
	}

	addr := fmt.Sprintf("%s:%d", e.host, e.port)

	// 🎨 1. 将 n.Content 的 Markdown 内容转换为 HTML Body
	var buf bytes.Buffer
	if err := goldmark.Convert([]byte(n.Content), &buf); err != nil {
		// 如果转换失败，降级为原始文本
		buf.WriteString(n.Content)
	}

	// 🎨 2. 注入精美的响应式 HTML 告警模板
	renderedHTML := buildHTMLTemplate(n.Title, n.Severity, n.Status, buf.String())

	// 3. 构建邮件 Header 与 Body
	headers := make(map[string]string)
	headers["From"] = fmt.Sprintf("告警通知网关 <%s>", e.from)
	headers["To"] = strings.Join(to, ",")
	headers["Subject"] = n.Title
	headers["Content-Type"] = "text/html; charset=UTF-8"

	message := ""
	for k, v := range headers {
		message += fmt.Sprintf("%s: %s\r\n", k, v)
	}
	message += "\r\n" + renderedHTML

	auth := smtp.PlainAuth("", e.user, e.password, e.host)

	// SSL/TLS 465 端口逻辑
	if e.port == 465 {
		tlsconfig := &tls.Config{
			InsecureSkipVerify: true,
			ServerName:         e.host,
		}

		conn, err := tls.Dial("tcp", addr, tlsconfig)
		if err != nil {
			return fmt.Errorf("连接 TLS 邮件服务器失败: %w", err)
		}
		defer conn.Close()

		client, err := smtp.NewClient(conn, e.host)
		if err != nil {
			return fmt.Errorf("创建 SMTP 客户端失败: %w", err)
		}
		defer client.Quit()

		if err = client.Auth(auth); err != nil {
			return fmt.Errorf("SMTP 认证失败: %w", err)
		}

		if err = client.Mail(e.from); err != nil {
			return err
		}

		for _, addr := range to {
			if err = client.Rcpt(addr); err != nil {
				return err
			}
		}

		w, err := client.Data()
		if err != nil {
			return err
		}

		_, err = w.Write([]byte(message))
		if err != nil {
			return err
		}

		return w.Close()
	}

	// 587 / 25 端口逻辑
	hostOnly, _, _ := net.SplitHostPort(addr)
	return smtp.SendMail(addr, smtp.PlainAuth("", e.user, e.password, hostOnly), e.from, to, []byte(message))
}

// 🎨 辅助函数：构造高颜值响应式 Email HTML 模板
func buildHTMLTemplate(title, severity, status, markdownHTML string) string {
	themeColor := "#1890ff" // 默认蓝色 (info)
	statusBadge := "通知"

	sev := strings.ToLower(severity)
	stat := strings.ToLower(status)

	if stat == "resolved" {
		themeColor = "#52c41a" // 绿色
		statusBadge = "已恢复"
	} else if sev == "critical" || sev == "high" {
		themeColor = "#ff4d4f" // 红色
		statusBadge = "紧急告警"
	} else if sev == "warning" || sev == "medium" {
		themeColor = "#faad14" // 黄色
		statusBadge = "警告"
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif; background-color: #f4f5f7; margin: 0; padding: 20px; color: #333; }
        .container { max-width: 650px; margin: 0 auto; background: #ffffff; border-radius: 8px; overflow: hidden; box-shadow: 0 4px 12px rgba(0,0,0,0.08); }
        .header { background-color: %s; padding: 20px 25px; color: #ffffff; display: flex; align-items: center; justify-content: space-between; }
        .header h2 { margin: 0; font-size: 18px; font-weight: 600; line-height: 1.4; }
        .badge { background: rgba(255, 255, 255, 0.25); padding: 4px 10px; border-radius: 4px; font-size: 12px; font-weight: bold; text-transform: uppercase; white-space: nowrap; }
        .content { padding: 25px; font-size: 14px; line-height: 1.6; color: #444; }
        .content h1, .content h2, .content h3 { color: #111; margin-top: 0; border-bottom: 1px solid #eee; padding-bottom: 8px; }
        .content ul { padding-left: 20px; margin: 10px 0; }
        .content li { margin-bottom: 6px; }
        .content code { background: #f0f2f5; color: #d43f3a; padding: 2px 6px; border-radius: 4px; font-family: SFMono-Regular, Consolas, Monaco, monospace; font-size: 13px; }
        .content pre { background: #282c34; color: #abb2bf; padding: 12px; border-radius: 6px; overflow-x: auto; font-family: SFMono-Regular, Consolas, Monaco, monospace; }
        .content a { color: %s; text-decoration: none; font-weight: 500; }
        .content a:hover { text-decoration: underline; }
        .footer { background: #fafafa; border-top: 1px solid #f0f0f0; padding: 15px 25px; text-align: center; font-size: 12px; color: #8c8c8c; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h2>%s</h2>
            <span class="badge">%s</span>
        </div>
        <div class="content">
            %s
        </div>
        <div class="footer">
            来自 <strong>企业级统一告警网关 (Alert Gateway)</strong> • 自动发送，请勿直接回复
        </div>
    </div>
</body>
</html>`, themeColor, themeColor, title, statusBadge, markdownHTML)
}
