package v1

import (
	"alert-gateway/config"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"alert-gateway/internal/entity"
	"alert-gateway/internal/usecase/notifier"
)

type Handler struct {
	useCase *notifier.NotifierUseCase
	cfg     *config.Config
}

func NewHandler(uc *notifier.NotifierUseCase, cfg *config.Config) *Handler {
	return &Handler{
		useCase: uc,
		cfg:     cfg,
	}
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

		isCritical := severity == "critical" || severity == "high"

		// 1. 钉钉群 Webhook（所有级别均推送）
		go dispatchAsync(h.useCase, "dingtalk_webhook", notification, "钉钉群 Webhook")

		// 2. 邮件 Email（所有级别均推送，如果指定了邮箱）
		userEmail := alert.Labels["email"]
		if userEmail != "" {
			emailNotification := *notification
			emailNotification.ReceiverIDs = []string{userEmail}
			go dispatchAsync(h.useCase, "email", &emailNotification, fmt.Sprintf("邮件 (%s)", userEmail))
		}

		// ---------------------------------------------------------------------
		// 3. 钉钉应用机器人私信（仅高级别 critical/high 触发）
		// 解析目标 UserID 列表：优先使用告警标签（支持逗号分隔多个），无标签时使用配置文件中的切片
		// ---------------------------------------------------------------------
		var targetUserIDs []string

		if rawUserID := alert.Labels["receiver_userid"]; rawUserID != "" {
			// 如果 Alertmanager 标签里通过逗号传了多个（如 "user1,user2"），拆分为切片
			for _, id := range strings.Split(rawUserID, ",") {
				if trimmed := strings.TrimSpace(id); trimmed != "" {
					targetUserIDs = append(targetUserIDs, trimmed)
				}
			}
		} else {
			// 兜底使用配置文件中的默认 UserID 切片
			targetUserIDs = h.cfg.DefaultReceiverUserID
		}

		if isCritical && len(targetUserIDs) > 0 {
			privateNotification := *notification
			privateNotification.ReceiverIDs = targetUserIDs

			logLabel := fmt.Sprintf("钉钉应用私信 (共 %d 人: %s)", len(targetUserIDs), strings.Join(targetUserIDs, ","))
			go dispatchAsync(h.useCase, "dingtalk_robot", &privateNotification, logLabel)
		} else if !isCritical && len(targetUserIDs) > 0 {
			log.Printf("[跳过推送] 当前告警级别为 (%s)，不触发钉钉个人私信强提醒", severity)
		}
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"success"}`))
}

func dispatchAsync(uc *notifier.NotifierUseCase, channel string, n *entity.Notification, label string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := uc.Dispatch(ctx, channel, n); err != nil {
		log.Printf("[推送失败] %s: %v", label, err)
	} else {
		log.Printf("[推送成功] %s", label)
	}
}
