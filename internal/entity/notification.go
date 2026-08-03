package entity

import "time"

// Alertmanager 接收的原始 Payload 实体
type AlertmanagerPayload struct {
	Alerts []Alert `json:"alerts"`
}

type Alert struct {
	Status      string            `json:"status"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	StartsAt    time.Time         `json:"startsAt"`
}

// 内部统一解耦后的通用通知对象
type Notification struct {
	Title       string
	Content     string
	ReceiverIDs []string // 目标接收人 UserID / 邮箱列表
	Severity    string
	Status      string
}
