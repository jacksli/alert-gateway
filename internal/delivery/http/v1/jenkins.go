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

// JenkinsPayload 接收 Jenkins Webhook 的完整数据
type JenkinsPayload struct {
	Name  string `json:"name"` // Job 名称
	Url   string `json:"url"`  // Job URL
	Build struct {
		Number   int    `json:"number"`
		Phase    string `json:"phase"`    // COMPLETED / FINALIZED
		Status   string `json:"status"`   // SUCCESS / FAILURE / ABORTED
		Url      string `json:"url"`      // Console 日志链接
		Duration int64  `json:"duration"` // 构建耗时(毫秒)
		Scm      struct {
			Branch string `json:"branch"`
			Commit string `json:"commit"`
		} `json:"scm"`
		Parameters map[string]interface{} `json:"parameters"` // 构建参数(如 ENV=prod, GROUP_ID=cidXXXXXX==)
	} `json:"build"`
}

type JenkinsHandler struct {
	useCase *notifier.NotifierUseCase
	cfg     *config.Config
}

func NewJenkinsHandler(uc *notifier.NotifierUseCase, cfg *config.Config) *JenkinsHandler {
	return &JenkinsHandler{useCase: uc, cfg: cfg}
}

func (h *JenkinsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload JenkinsPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid Payload", http.StatusBadRequest)
		return
	}

	// 仅在构建完成时发送
	if payload.Build.Phase != "COMPLETED" && payload.Build.Phase != "FINALIZED" {
		w.WriteHeader(http.StatusOK)
		return
	}

	status := strings.ToUpper(payload.Build.Status)

	// 🎯 1. 定制 Jenkins 专属的标题与图标
	var title, headerTag, colorStatus string
	if status == "SUCCESS" {
		headerTag = "🚀 【Jenkins 发版成功】"
		colorStatus = "<font color='#52c41a'>SUCCESS</font>"
	} else if status == "FAILURE" {
		headerTag = "🚨 【Jenkins 发版失败】"
		colorStatus = "<font color='#f5222d'>FAILURE</font>"
	} else {
		headerTag = "⚠️ 【Jenkins 构建中断】"
		colorStatus = "<font color='#faad14'>" + status + "</font>"
	}
	title = fmt.Sprintf("%s %s (#%d)", headerTag, payload.Name, payload.Build.Number)

	// 计算构建耗时
	durationStr := fmt.Sprintf("%ds", payload.Build.Duration/1000)
	if payload.Build.Duration == 0 {
		durationStr = "未知"
	}

	branch := payload.Build.Scm.Branch
	if branch == "" {
		branch = "main"
	}

	// 🎯 2. 拼装 Jenkins 专属的高颜值 Markdown 消息体
	content := fmt.Sprintf(
		"## %s\n\n"+
			"--- \n"+
			"- **应用名称**: `%s` \n"+
			"- **构建编号**: `#%d` \n"+
			"- **构建状态**: %s \n"+
			"- **Git 分支**: `%s` \n"+
			"- **构建耗时**: %s \n"+
			"- **完成时间**: %s \n\n"+
			"--- \n"+
			"🔗 [👉 点击查看 Jenkins 构建日志](%sconsole)",
		headerTag,
		payload.Name,
		payload.Build.Number,
		colorStatus,
		branch,
		durationStr,
		time.Now().Format("2006-01-02 15:04:05"),
		payload.Build.Url,
	)

	// ---------------------------------------------------------------------
	// 🚀 3. 解析目标群 openConversationId 列表（优先 Parameters，兜底使用配置文件）
	// ---------------------------------------------------------------------
	var targetGroupIDs []string
	var rawGroupID string

	// 从 parameters 中查找 group_id / open_conversation_id / receiver_group_id
	for key, val := range payload.Build.Parameters {
		k := strings.ToLower(key)
		if k == "group_id" || k == "open_conversation_id" || k == "receiver_group_id" {
			if strVal, ok := val.(string); ok {
				rawGroupID = strVal
				break
			}
		}
	}

	if rawGroupID != "" {
		// 支持逗号分隔传多个群 ID，例如 "cidXXX==, cidYYY=="
		for _, id := range strings.Split(rawGroupID, ",") {
			if trimmed := strings.TrimSpace(id); trimmed != "" {
				targetGroupIDs = append(targetGroupIDs, trimmed)
			}
		}
	} else {
		// 兜底使用配置文件中的默认接收群切片
		targetGroupIDs = h.cfg.DefaultReceiverGroupID
	}

	// 构造统一的 Notification 实体（ReceiverIDs 此时存的是群 openConversationId）
	notification := &entity.Notification{
		Title:       title,
		Content:     content,
		Status:      status,
		ReceiverIDs: targetGroupIDs,
	}

	// ---------------------------------------------------------------------
	// 🚀 4. 异步调度企业内部应用群发驱动 (dingtalk_app_group) 进行推送
	// ---------------------------------------------------------------------
	go func(n *entity.Notification) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		logLabel := fmt.Sprintf("钉钉企业应用群发 (目标群数: %d)", len(n.ReceiverIDs))

		// 🎯 驱动名称切换为 dingtalk_app_group
		if err := h.useCase.Dispatch(ctx, "dingtalk_app_group", n); err != nil {
			log.Printf("[推送失败] %s: %v", logLabel, err)
		} else {
			log.Printf("[推送成功] %s", logLabel)
		}
	}(notification)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}
