package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"alert-gateway/internal/entity"
	"alert-gateway/internal/usecase/notifier"
)

const ProviderName = "dingtalk_robot"

type DingTalkRobotProvider struct {
	appKey     string
	appSecret  string
	agentID    int64
	enableDing bool
	dingType   int
	client     *http.Client
	tokenCache tokenCache
}

func init() {
	// ✅ 包装工厂构造函数，适配 notifier.Provider 接口类型
	notifier.Register(ProviderName, func(cfg map[string]interface{}) (notifier.Provider, error) {
		return NewDingTalkRobotProvider(cfg), nil
	})
}

func NewDingTalkRobotProvider(cfg map[string]interface{}) *DingTalkRobotProvider {
	appKey, _ := cfg["app_key"].(string)
	appSecret, _ := cfg["app_secret"].(string)
	agentID := toInt64(cfg["agent_id"])

	if agentID <= 0 {
		panic("invalid dingtalk agent_id")
	}
	enableDing, _ := cfg["enable_ding"].(bool)
	dingType := 1
	if v, ok := cfg["ding_type"].(float64); ok && v > 0 {
		dingType = int(v)
	}

	return &DingTalkRobotProvider{
		appKey:     appKey,
		appSecret:  appSecret,
		agentID:    agentID,
		enableDing: enableDing,
		dingType:   dingType,
		client:     &http.Client{Timeout: 10 * time.Second},
	}
}

// ✅ 补充 Name() 方法，满足 notifier.Provider 接口定义
func (p *DingTalkRobotProvider) Name() string {
	return ProviderName
}

func toInt64(v any) int64 {
	switch t := v.(type) {
	case int:
		return int64(t)
	case int64:
		return t
	case int32:
		return int64(t)
	case float64:
		return int64(t)
	case json.Number:
		i, _ := t.Int64()
		return i
	case string:
		i, _ := strconv.ParseInt(t, 10, 64)
		return i
	default:
		return 0
	}
}

func (p *DingTalkRobotProvider) Send(ctx context.Context, n *entity.Notification) error {
	token, err := p.getAccessToken(ctx)
	if err != nil {
		return fmt.Errorf("获取钉钉 AccessToken 失败: %w", err)
	}

	if len(n.ReceiverIDs) == 0 {
		return fmt.Errorf("缺少接收人 UserID")
	}

	userListStr := strings.Join(n.ReceiverIDs, ",")

	// 构建发送异步工作通知请求 Payload
	payload := map[string]interface{}{
		"agent_id":    p.agentID,
		"userid_list": userListStr,
		"msg": map[string]interface{}{
			"msgtype": "action_card",
			"action_card": map[string]interface{}{
				"title":        n.Title,
				"markdown":     fmt.Sprintf("%s\n\n> ⏰ 触发时间: %s", n.Content, time.Now().Format("15:04:05")),
				"single_title": "查看详情",
				"single_url":   "dingtalk://dingtalkpage/action/openapp", // 点击直达应用内卡片
			},
		},
	}

	bodyBytes, _ := json.Marshal(payload)
	sendURL := fmt.Sprintf("https://oapi.dingtalk.com/topapi/message/corpconversation/asyncsend_v2?access_token=%s", token)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, sendURL, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("调用钉钉工作通知接口失败: %w", err)
	}
	defer resp.Body.Close()

	var res struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
		TaskID  int64  `json:"task_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return err
	}
	if res.ErrCode != 0 {
		return fmt.Errorf("钉钉工作通知推送失败 [%d]: %s", res.ErrCode, res.ErrMsg)
	}

	// ---------------------------------------------------------------------
	// 如果开启了自动 DING 强提醒，调用发送 DING 操作
	// ---------------------------------------------------------------------
	if p.enableDing {
		if err := p.sendDingNotice(ctx, token, userListStr, n); err != nil {
			// DING 提醒失败仅记录日志，不阻断主通知流程
			fmt.Printf("[DING 触发异常] UserIDs: %s, 原因: %v\n", userListStr, err)
		}
	}

	return nil
}

// sendDingNotice 发送 DING 强提醒
func (p *DingTalkRobotProvider) sendDingNotice(ctx context.Context, token, userListStr string, n *entity.Notification) error {
	dingURL := fmt.Sprintf("https://oapi.dingtalk.com/topapi/ding/send?access_token=%s", token)

	dingPayload := map[string]interface{}{
		"open_ding_send_vo": map[string]interface{}{
			"receiver_user_ids": strings.Split(userListStr, ","),
			"content":           fmt.Sprintf("🚨【告警强提醒】%s\n请立即处理！", n.Title),
			"remind_type":       p.dingtypeText(p.dingType), // 1: 应用内; 2: 短信; 3: 电话
		},
	}

	bodyBytes, _ := json.Marshal(dingPayload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, dingURL, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var res struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	_ = json.Unmarshal(body, &res)
	if res.ErrCode != 0 {
		return fmt.Errorf("DING 接口返回错误 [%d]: %s", res.ErrCode, res.ErrMsg)
	}
	return nil
}

func (p *DingTalkRobotProvider) dingtypeText(t int) string {
	switch t {
	case 2:
		return "SMS"
	case 3:
		return "CALL"
	default:
		return "APP"
	}
}
