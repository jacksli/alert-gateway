package email

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"strings"

	"alert-gateway/internal/entity"
	"alert-gateway/internal/usecase/notifier"
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

	headers := make(map[string]string)
	headers["From"] = e.from
	headers["To"] = strings.Join(to, ",")
	headers["Subject"] = n.Title
	headers["Content-Type"] = "text/html; charset=UTF-8"

	message := ""
	for k, v := range headers {
		message += fmt.Sprintf("%s: %s\r\n", k, v)
	}
	htmlBody := fmt.Sprintf("<pre style='font-family: sans-serif;'>%s</pre>", n.Content)
	message += "\r\n" + htmlBody

	auth := smtp.PlainAuth("", e.user, e.password, e.host)

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

	hostOnly, _, _ := net.SplitHostPort(addr)
	return smtp.SendMail(addr, smtp.PlainAuth("", e.user, e.password, hostOnly), e.from, to, []byte(message))
}
