package dingtalk_robot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"alert-gateway/internal/entity"
	"alert-gateway/internal/usecase/notifier"
)

const ProviderName = "dingtalk_robot"

type DingTalkProvider struct {
	appKey      string
	appSecret   string
	accessToken string
	tokenExpire time.Time
	mu          sync.RWMutex
}

func init() {
	notifier.Register(ProviderName, NewDingTalkProvider)
}

func NewDingTalkProvider(config map[string]interface{}) (notifier.Provider, error) {
	appKey, _ := config["app_key"].(string)
	appSecret, _ := config["app_secret"].(string)

	if appKey == "" || appSecret == "" {
		return nil, fmt.Errorf("dingtalk_robot 配置缺失 app_key 或 app_secret")
	}

	return &DingTalkProvider{
		appKey:    appKey,
		appSecret: appSecret,
	}, nil
}

func (d *DingTalkProvider) Name() string {
	return ProviderName
}

func (d *DingTalkProvider) Send(ctx context.Context, n *entity.Notification) error {
	token, err := d.getAccessToken()
	if err != nil {
		return fmt.Errorf("获取钉钉 Token 失败: %w", err)
	}

	// 1. 构建 Markdown 消息内容
	msgParamObj := map[string]string{
		"title": n.Title,
		"text":  n.Content,
	}
	msgParamBytes, _ := json.Marshal(msgParamObj)

	reqBody := map[string]interface{}{
		"robotCode": d.appKey, // 应用机器人的 AppKey 即为 robotCode
		"userIds":   n.ReceiverIDs,
		"msgKey":    "sampleMarkdown",
		"msgParam":  string(msgParamBytes),
	}

	bodyBytes, _ := json.Marshal(reqBody)

	// 2. 使用新版开放平台 API (api.dingtalk.com)
	apiURL := "https://api.dingtalk.com/v1.0/robot/oToMessages/batchSend"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return err
	}

	// 新版 API 推荐在 Header 中传入 x-acs-dingtalk-access-token
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-acs-dingtalk-access-token", token)

	resp, err := http.DefaultClient.Do(req)
	//log.Println("Access_token:", token)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBuf, _ := io.ReadAll(resp.Body)

	// 校验返回结果
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("钉钉应用机器人发送失败 [HTTP %d]: %s", resp.StatusCode, string(respBuf))
	} else {
		log.Println("钉钉应用机器人发送成功", string(respBuf))
	}

	var res struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	_ = json.Unmarshal(respBuf, &res)

	if res.Code != "" && res.Code != "0" && res.Code != "200" {
		return fmt.Errorf("钉钉应用机器人返回错误 [%s]: %s", res.Code, res.Message)
	}

	return nil
}

// 获取 AccessToken (新版 API 获取方式: POST https://api.dingtalk.com/v1.0/oauth2/accessToken)
func (d *DingTalkProvider) getAccessToken() (string, error) {
	d.mu.RLock()
	if d.accessToken != "" && time.Now().Before(d.tokenExpire) {
		defer d.mu.RUnlock()
		return d.accessToken, nil
	}
	d.mu.RUnlock()

	d.mu.Lock()
	defer d.mu.Unlock()

	reqPayload := map[string]string{
		"appKey":    d.appKey,
		"appSecret": d.appSecret,
	}
	bodyBytes, _ := json.Marshal(reqPayload)

	resp, err := http.Post("https://api.dingtalk.com/v1.0/oauth2/accessToken", "application/json", bytes.NewBuffer(bodyBytes))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var res struct {
		AccessToken string `json:"accessToken"`
		ExpireIn    int    `json:"expireIn"`
		Code        string `json:"code"`
		Message     string `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", err
	}

	if res.AccessToken == "" {
		return "", fmt.Errorf("获取 AccessToken 失败 [%s]: %s", res.Code, res.Message)
	}

	d.accessToken = res.AccessToken
	d.tokenExpire = time.Now().Add(time.Duration(res.ExpireIn-300) * time.Second)
	return d.accessToken, nil
}
