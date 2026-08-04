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

const ProviderName = "dingtalk_app_group"

// TokenCache 结构体，用于缓存 AccessToken
type tokenCache struct {
	token    string
	expireAt time.Time
	mu       sync.RWMutex
}

type DingTalkAppGroupProvider struct {
	appKey    string
	appSecret string
	robotCode string // 机器人的 RobotCode（通常就是 AppKey）

	// Token 缓存与 HTTP 客户端
	tokenCache tokenCache
	client     *http.Client
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
		client: &http.Client{
			Timeout: 10 * time.Second, // 设置合理的 HTTP 超时
		},
	}, nil
}

func (p *DingTalkAppGroupProvider) Name() string {
	return ProviderName
}

func (p *DingTalkAppGroupProvider) getAccessToken(ctx context.Context) (string, error) {
	// ---------- Fast Path (读锁) ----------
	p.tokenCache.mu.RLock()
	if p.tokenCache.token != "" && time.Now().Before(p.tokenCache.expireAt) {
		token := p.tokenCache.token
		p.tokenCache.mu.RUnlock()
		return token, nil
	}
	p.tokenCache.mu.RUnlock()

	// ---------- Refresh (写锁) ----------
	p.tokenCache.mu.Lock()
	defer p.tokenCache.mu.Unlock()

	// Double Check 防止并发重复刷新
	if p.tokenCache.token != "" && time.Now().Before(p.tokenCache.expireAt) {
		return p.tokenCache.token, nil
	}

	url := fmt.Sprintf(
		"https://oapi.dingtalk.com/gettoken?appkey=%s&appsecret=%s",
		p.appKey,
		p.appSecret,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		ErrCode     int    `json:"errcode"`
		ErrMsg      string `json:"errmsg"`
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if result.ErrCode != 0 {
		return "", fmt.Errorf(
			"dingtalk gettoken failed [%d]: %s",
			result.ErrCode,
			result.ErrMsg,
		)
	}

	p.tokenCache.token = result.AccessToken

	expire := result.ExpiresIn
	if expire <= 300 {
		expire = 7200
	}

	// 提前 5 分钟刷新
	p.tokenCache.expireAt = time.Now().Add(
		time.Duration(expire-300) * time.Second,
	)

	return result.AccessToken, nil
}

func (p *DingTalkAppGroupProvider) Send(ctx context.Context, n *entity.Notification) error {
	// n.ReceiverIDs 中存储目标群的 openConversationId（群ID）
	if len(n.ReceiverIDs) == 0 {
		return fmt.Errorf("接收群组 open_conversation_id (ReceiverIDs) 不能为空")
	}

	// 🎯 修复点：调用本结构体的 getAccessToken 方法获取 token
	accessToken, err := p.getAccessToken(ctx)
	if err != nil {
		return fmt.Errorf("获取钉钉 access_token 失败: %w", err)
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

		// 🎯 修复点：使用带超时的 p.client 替代 http.DefaultClient
		resp, err := p.client.Do(req)
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
