package dingtalk_app_group

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"alert-gateway/internal/entity"
	"alert-gateway/internal/usecase/notifier"
)

const ProviderName = "jenkins_dingtalk_app_group"

type DingTalkAppGroupProvider struct {
	appKey    string
	appSecret string
	robotCode string // 机器人的 RobotCode（通常就是 AppKey）

	// Token 缓存
	accessToken string
	tokenExpire time.Time
	mu          sync.Mutex
}

func init() {
	notifier.Register(ProviderName, NewDingTalkAppGroupProvider)
}

func NewDingTalkAppGroupProvider(config map[string]interface{}) (notifier.Provider, error) {
	appKey, _ := config["app_key"].(string)
	appSecret, _ := config["app_secret"].(string)
	robotCode, _ := config["robot_code"].(string)

	if robotCode == "" {
		robotCode = appKey // 默认 RobotCode 通常为 AppKey
	}

	if appKey == "" || appSecret == "" {
		return nil, fmt.Errorf("dingtalk_app_group 缺少必要配置 (app_key, app_secret)")
	}

	return &DingTalkAppGroupProvider{
		appKey:    appKey,
		appSecret: appSecret,
		robotCode: robotCode,
	}, nil
}

func (p *DingTalkAppGroupProvider) Name() string {
	return ProviderName
}

// 获取/刷新 access_token
func (p *DingTalkAppGroupProvider) getAccessToken(ctx context.Context) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 未过期则复用
	if p.accessToken != "" && time.Now().Before(p.tokenExpire) {
		return p.accessToken, nil
	}

	url := fmt.Sprintf("https://oapi.dingtalk.com/gettoken?appkey=%s&appsecret=%s", p.appKey, p.appSecret)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("获取 access_token 失败: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		ErrCode     int    `json:"errcode"`
		ErrMsg      string `json:"errmsg"`
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if result.ErrCode != 0 {
		return "", fmt.Errorf("获取 access_token 报错: %s", result.ErrMsg)
	}

	p.accessToken = result.AccessToken
	// 提前 200 秒刷新
	p.tokenExpire = time.Now().Add(time.Duration(result.ExpiresIn-200) * time.Second)
	return p.accessToken, nil
}

func (p *DingTalkAppGroupProvider) Send(ctx context.Context, n *entity.Notification) error {
	// n.ReceiverIDs 中存储目标群的 openConversationId（群ID）
	if len(n.ReceiverIDs) == 0 {
		return fmt.Errorf("接收群组 open_conversation_id (ReceiverIDs) 不能为空")
	}

	accessToken, err := p.getAccessToken(ctx)
	if err != nil {
		return err
	}

	// 钉钉新版 OpenAPI: 发送群消息
	apiURL := "https://api.dingtalk.com/v1.0/robot/groupMessages/send"

	for _, openConvID := range n.ReceiverIDs {
		payload := map[string]interface{}{
			"openConversationId": openConvID,
			"robotCode":          p.robotCode,
			"msgKey":             "sampleMarkdown", // Markdown 消息类型
			"msgParam": jsonStringify(map[string]string{
				"title": n.Title,
				"text":  n.Content,
			}),
		}

		bodyBytes, _ := json.Marshal(payload)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewBuffer(bodyBytes))
		if err != nil {
			return err
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-acs-dingtalk-access-token", accessToken)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return fmt.Errorf("发送群消息 HTTP 请求失败: %w", err)
		}

		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("发送群消息失败 [%d]: %s", resp.StatusCode, string(respBody))
		}
	}

	return nil
}

func jsonStringify(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}
