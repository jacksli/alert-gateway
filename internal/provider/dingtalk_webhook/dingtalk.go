package dingtalk_webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"alert-gateway/internal/entity"
	"alert-gateway/internal/usecase/notifier"
)

const ProviderName = "dingtalk_webhook"

type DingTalkWebhookProvider struct {
	webhookURL string
	secret     string
}

func init() {
	notifier.Register(ProviderName, NewDingTalkWebhookProvider)
}

func NewDingTalkWebhookProvider(config map[string]interface{}) (notifier.Provider, error) {
	webhookURL, _ := config["webhook_url"].(string)
	secret, _ := config["secret"].(string)

	if webhookURL == "" {
		return nil, fmt.Errorf("dingtalk_webhook 缺少配置 webhook_url")
	}

	return &DingTalkWebhookProvider{
		webhookURL: webhookURL,
		secret:     secret,
	}, nil
}

func (d *DingTalkWebhookProvider) Name() string {
	return ProviderName
}

func (d *DingTalkWebhookProvider) Send(ctx context.Context, n *entity.Notification) error {
	finalURL := d.webhookURL

	if d.secret != "" {
		timestamp := time.Now().UnixNano() / 1e6
		stringToSign := fmt.Sprintf("%d\n%s", timestamp, d.secret)

		h := hmac.New(sha256.New, []byte(d.secret))
		h.Write([]byte(stringToSign))
		sign := base64.StdEncoding.EncodeToString(h.Sum(nil))

		u, err := url.Parse(d.webhookURL)
		if err != nil {
			return fmt.Errorf("解析 Webhook URL 失败: %w", err)
		}

		q := u.Query()
		q.Set("timestamp", fmt.Sprintf("%d", timestamp))
		q.Set("sign", sign)
		u.RawQuery = q.Encode()
		finalURL = u.String()
	}

	payload := map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"title": n.Title,
			"text":  n.Content,
		},
	}

	bodyBytes, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, finalURL, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBuf, _ := io.ReadAll(resp.Body)
	var res struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	_ = json.Unmarshal(respBuf, &res)

	if res.ErrCode != 0 {
		return fmt.Errorf("钉钉 Webhook 错误 [%d]: %s", res.ErrCode, res.ErrMsg)
	}

	return nil
}
