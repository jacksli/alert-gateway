package dingtalk_robot

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
		return err
	}

	msgParamObj := map[string]string{
		"title": n.Title,
		"text":  n.Content,
	}
	msgParamBytes, _ := json.Marshal(msgParamObj)

	reqBody := map[string]interface{}{
		"robotCode": d.appKey,
		"targetForSend": map[string]interface{}{
			"userIds": n.ReceiverIDs,
		},
		"msgKey":   "sampleMarkdown",
		"msgParam": string(msgParamBytes),
	}

	bodyBytes, _ := json.Marshal(reqBody)
	apiURL := fmt.Sprintf("https://oapi.dingtalk.com/robot/send?access_token=%s", token)

	resp, err := http.Post(apiURL, "application/json", bytes.NewBuffer(bodyBytes))
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
		return fmt.Errorf("钉钉应用机器人 API 错误 [%d]: %s", res.ErrCode, res.ErrMsg)
	}

	return nil
}

func (d *DingTalkProvider) getAccessToken() (string, error) {
	d.mu.RLock()
	if d.accessToken != "" && time.Now().Before(d.tokenExpire) {
		defer d.mu.RUnlock()
		return d.accessToken, nil
	}
	d.mu.RUnlock()

	d.mu.Lock()
	defer d.mu.Unlock()

	url := fmt.Sprintf("https://oapi.dingtalk.com/gettoken?appkey=%s&appsecret=%s", d.appKey, d.appSecret)
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var res struct {
		ErrCode     int    `json:"errcode"`
		ErrMsg      string `json:"errmsg"`
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&res)

	if res.ErrCode != 0 {
		return "", fmt.Errorf("获取 Token 失败: %s", res.ErrMsg)
	}

	d.accessToken = res.AccessToken
	d.tokenExpire = time.Now().Add(time.Duration(res.ExpiresIn-300) * time.Second)
	return d.accessToken, nil
}
