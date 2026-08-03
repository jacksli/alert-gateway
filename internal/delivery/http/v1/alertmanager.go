package v1

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

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
		statusText := "🚨 【告警触发】"
		if alert.Status == "resolved" {
			statusText = "✅ 【告警恢复】"
		}

		title := fmt.Sprintf("%s %s", statusText, alert.Labels["alertname"])
		content := fmt.Sprintf("### %s\n\n- **实例**: %s\n- **描述**: %s\n",
			title, alert.Labels["instance"], alert.Annotations["description"])

		// 1. 动态路由场景 A: 如果标签含有 receiver_userid，则通过钉钉应用发私信
		if userid := alert.Labels["receiver_userid"]; userid != "" {
			n := &entity.Notification{
				Title:       title,
				Content:     content,
				ReceiverIDs: []string{userid},
				Severity:    alert.Labels["severity"],
				Status:      alert.Status,
			}
			if err := h.useCase.Dispatch(r.Context(), "dingtalk_robot", n); err != nil {
				log.Printf("推送钉钉私信失败: %v", err)
			}
		}

		// 2. 动态路由场景 B: 如果标签含有 email，发邮件
		if userEmail := alert.Labels["email"]; userEmail != "" {
			n := &entity.Notification{
				Title:       title,
				Content:     content,
				ReceiverIDs: []string{userEmail},
				Severity:    alert.Labels["severity"],
				Status:      alert.Status,
			}
			if err := h.useCase.Dispatch(r.Context(), "email", n); err != nil {
				log.Printf("推送邮件失败: %v", err)
			}
		}

		// 3. 动态路由场景 C: 兜底或公共告警发往钉钉群 (Webhook)
		if alert.Labels["receiver_userid"] == "" && alert.Labels["email"] == "" {
			n := &entity.Notification{
				Title:    title,
				Content:  content,
				Severity: alert.Labels["severity"],
				Status:   alert.Status,
			}
			if err := h.useCase.Dispatch(r.Context(), "dingtalk_webhook", n); err != nil {
				log.Printf("推送钉钉群 Webhook 失败: %v", err)
			}
		}
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"success"}`))
}
