package v1

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"alert-gateway/config"
	"alert-gateway/internal/entity"
	"alert-gateway/internal/usecase/notifier"
)

// AWSSNSPayload 匹配 AWS SNS HTTP/HTTPS 端点推送的标准 JSON 结构
type AWSSNSPayload struct {
	Type             string `json:"Type"`
	MessageId        string `json:"MessageId"`
	TopicArn         string `json:"TopicArn"`
	Message          string `json:"Message"`
	SubscribeURL     string `json:"SubscribeURL"`
	Timestamp        string `json:"Timestamp"`
	SignatureVersion string `json:"SignatureVersion"`
	Signature        string `json:"Signature"`
	SigningCertURL   string `json:"SigningCertURL"`
}

// CloudWatchAlarmMessage 匹配 CloudWatch Alarm 在 SNS Message 字段中的嵌入 JSON 结构
type CloudWatchAlarmMessage struct {
	AlarmName        string `json:"AlarmName"`
	AlarmDescription string `json:"AlarmDescription"`
	AWSAccountId     string `json:"AWSAccountId"`
	NewStateValue    string `json:"NewStateValue"`  // ALARM / OK / INSUFFICIENT_DATA
	NewStateReason   string `json:"NewStateReason"` // 触发告警的原因/具体日志
	StateChangeTime  string `json:"StateChangeTime"`
	Region           string `json:"Region"`
}

type AWSSNSHandler struct {
	useCase *notifier.NotifierUseCase
	cfg     *config.Config
}

func NewAWSSNSHandler(uc *notifier.NotifierUseCase, cfg *config.Config) *AWSSNSHandler {
	return &AWSSNSHandler{useCase: uc, cfg: cfg}
}

func (h *AWSSNSHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload AWSSNSPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid SNS Payload", http.StatusBadRequest)
		return
	}

	// 🎯 1. 自动处理 AWS SNS 首次订阅确认 (SubscriptionConfirmation)
	if payload.Type == "SubscriptionConfirmation" {
		log.Printf("[AWS SNS] 收到订阅确认请求，自动访问 SubscribeURL: %s", payload.SubscribeURL)
		go func(url string) {
			resp, err := http.Get(url)
			if err != nil {
				log.Printf("[AWS SNS] 订阅确认失败: %v", err)
				return
			}
			defer resp.Body.Close()
			log.Printf("[AWS SNS] 订阅确认成功！HTTP Code: %d", resp.StatusCode)
		}(payload.SubscribeURL)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"subscribed"}`))
		return
	}

	// 🎯 2. 处理告警消息 (Notification)
	if payload.Type == "Notification" {
		var alarmMsg CloudWatchAlarmMessage
		// 尝试解析 SNS Message 字符串里的 CloudWatch Alarm JSON 数据
		if err := json.Unmarshal([]byte(payload.Message), &alarmMsg); err != nil {
			// 如果不是 JSON，作为纯文本日志告警处理
			alarmMsg.AlarmName = "AWS CloudWatch Logs 告警"
			alarmMsg.NewStateReason = payload.Message
			alarmMsg.NewStateValue = "ALARM"
		}

		// 格式化状态与颜色
		status := strings.ToUpper(alarmMsg.NewStateValue)
		var statusHeader, statusColor string
		if status == "ALARM" {
			statusHeader = "🚨 【AWS CloudWatch 告警触发】"
			statusColor = "<font color='#f5222d'>ALARM</font>"
		} else if status == "OK" {
			statusHeader = "✅ 【AWS CloudWatch 告警恢复】"
			statusColor = "<font color='#52c41a'>OK</font>"
		} else {
			statusHeader = "⚠️ 【AWS CloudWatch 告警状态更新】"
			statusColor = "<font color='#faad14'>" + status + "</font>"
		}

		title := fmt.Sprintf("%s %s", statusHeader, alarmMsg.AlarmName)

		// 拼装 Markdown 告警模板
		content := fmt.Sprintf(
			"## %s\n\n"+
				"--- \n"+
				"- **告警名称**: `%s` \n"+
				"- **告警状态**: %s \n"+
				"- **AWS 账号**: `%s` \n"+
				"- **所属区域**: `%s` \n"+
				"- **触发时间**: %s \n\n"+
				"--- \n"+
				"**详细日志 / 触发原因**:\n```text\n%s\n```",
			statusHeader,
			alarmMsg.AlarmName,
			statusColor,
			alarmMsg.AWSAccountId,
			alarmMsg.Region,
			time.Now().Format("2006-01-02 15:04:05"),
			alarmMsg.NewStateReason,
		)

		notification := &entity.Notification{
			Title:       title,
			Content:     content,
			Status:      status,
			ReceiverIDs: h.cfg.DefaultReceiverGroupID,
		}

		// 🎯 3. 异步分发：将驱动名称切换为 dingtalk_robot
		go func(n *entity.Notification) {
			ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
			defer cancel()

			if err := h.useCase.Dispatch(ctx, "dingtalk_robot", n); err != nil {
				log.Printf("[AWS dingtalk_robot 推送失败]: %v", err)
			} else {
				log.Printf("[AWS dingtalk_robot 推送成功]")
			}
		}(notification)
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}
