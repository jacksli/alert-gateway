package v1

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"alert-gateway/internal/entity"
	"alert-gateway/internal/usecase/notifier"
)

type Handler struct {
	useCase *notifier.NotifierUseCase
}

func NewHandler(uc *notifier.NotifierUseCase) *Handler {
	return &Handler{useCase: uc}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload entity.AlertmanagerPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	for _, alert := range payload.Alerts {
		severity := strings.ToLower(alert.Labels["severity"])
		statusText := "🚨 【告警触发】"
		if alert.Status == "resolved" {
			statusText = "✅ 【告警恢复】"
		}

		title := fmt.Sprintf("%s [%s] %s", statusText, strings.ToUpper(severity), alert.Labels["alertname"])
		content := fmt.Sprintf("### %s\n\n- **实例**: %s\n- **严重程度**: %s\n- **详细描述**: %s\n",
			title, alert.Labels["instance"], severity, alert.Annotations["description"])

		notification := &entity.Notification{
			Title:    title,
			Content:  content,
			Severity: severity,
			Status:   alert.Status,
		}

		// 判断是否为高级别告警 (critical 或 high)
		isCritical := severity == "critical" || severity == "high"

		// ---------------------------------------------------------------------
		// 渠道 1：钉钉群 Webhook（公共群里人人可见）
		// 分发策略：所有级别告警（低级别 + 高级别）均发送
		// ---------------------------------------------------------------------
		go dispatchAsync(r.Context(), h.useCase, "dingtalk_webhook", notification, "钉钉群 Webhook")

		// ---------------------------------------------------------------------
		// 渠道 2：邮件 Email
		// 分发策略：所有级别告警均发送（如果配置了接收人邮箱，也可以配置默认公共运维邮箱）
		// ---------------------------------------------------------------------
		userEmail := alert.Labels["email"]
		if userEmail != "" {
			emailNotification := *notification
			emailNotification.ReceiverIDs = []string{userEmail}
			go dispatchAsync(r.Context(), h.useCase, "email", &emailNotification, fmt.Sprintf("邮件 (%s)", userEmail))
		}

		// ---------------------------------------------------------------------
		// 渠道 3：钉钉应用机器人（强提醒私信）
		// 分发策略：只有高级别告警 (critical / high) 且有 receiver_userid 时才发送私信强提醒
		// ---------------------------------------------------------------------
		userid := alert.Labels["receiver_userid"]
		if isCritical && userid != "" {
			privateNotification := *notification
			privateNotification.ReceiverIDs = []string{userid}
			go dispatchAsync(r.Context(), h.useCase, "dingtalk_robot", &privateNotification, fmt.Sprintf("钉钉应用私信 (%s)", userid))
		} else if !isCritical && userid != "" {
			log.Printf("[跳过推送] 当前告警为低级别 (%s)，不触发钉钉个人私信强提醒", severity)
		}
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"success"}`))
}

// 异步调度辅助函数
func dispatchAsync(ctx interface{}, uc *notifier.NotifierUseCase, channel string, n *entity.Notification, label string) {
	// 注意：实际调用时请使用带有 timeout 的 background ctx 或传入的 context
	if err := uc.Dispatch(nil, channel, n); err != nil {
		log.Printf("[推送失败] %s: %v", label, err)
	} else {
		log.Printf("[推送成功] %s", label)
	}
}
