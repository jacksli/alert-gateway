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
	"alert-gateway/internal/pkg/deepseek"
	"alert-gateway/internal/usecase/notifier"
)

// JenkinsPayload 兼容接收 Jenkins 和 GitHub Actions 的 Webhook 数据
type JenkinsPayload struct {
	Name  string `json:"name"` // Job 名称 或 Workflow 名称
	Url   string `json:"url"`  // Job URL 或 Action URL
	Build struct {
		Number   int    `json:"number"`
		Phase    string `json:"phase"`    // COMPLETED / FINALIZED
		Status   string `json:"status"`   // SUCCESS / FAILURE / ABORTED / CANCELLED
		Url      string `json:"url"`      // Console 日志链接 或 Action 日志链接
		Duration int64  `json:"duration"` // 构建耗时(毫秒)
		Scm      struct {
			Branch string `json:"branch"`
			Commit string `json:"commit"`
		} `json:"scm"`
		Parameters map[string]interface{} `json:"parameters"` // 构建参数(如 ENV=prod, open_conversation_id=cidXXXXXX==)
	} `json:"build"`
}

type JenkinsHandler struct {
	useCase        *notifier.NotifierUseCase
	cfg            *config.Config
	deepseekClient *deepseek.Client
}

func NewJenkinsHandler(uc *notifier.NotifierUseCase, cfg *config.Config, ds *deepseek.Client) *JenkinsHandler {
	return &JenkinsHandler{
		useCase:        uc,
		cfg:            cfg,
		deepseekClient: ds,
	}
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

	// 🎯 1. 自动识别来源：GitHub Actions 还是 Jenkins
	source := "Jenkins"
	platformIcon := "🏗️"
	if strings.Contains(payload.Url, "github.com") {
		source = "GitHub"
		platformIcon = "🐙"
	}

	// 🎯 2. 定制多源专属的标题与状态颜色
	var title, headerTag, colorStatus string
	if status == "SUCCESS" {
		headerTag = fmt.Sprintf("🚀 【%s 发版成功】", source)
		colorStatus = "<font color='#52c41a'>SUCCESS</font>"
	} else if status == "FAILURE" {
		headerTag = fmt.Sprintf("🚨 【%s 发版失败】", source)
		colorStatus = "<font color='#f5222d'>FAILURE</font>"
	} else {
		headerTag = fmt.Sprintf("⚠️ 【%s 构建中断】", source)
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

	// 处理日志链接：Jenkins 通常加 "console"，而 GitHub Action 直接是 URL
	logUrl := payload.Build.Url
	if source == "Jenkins" && !strings.HasSuffix(logUrl, "console") {
		if !strings.HasSuffix(logUrl, "/") {
			logUrl += "/"
		}
		logUrl += "console"
	}

	// 🎯 3. 构建失败时调用阿里云 DeepSeek 模型进行智能故障诊断
	var aiAnalysisSection string
	if status == "FAILURE" && h.deepseekClient != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
		analysis, err := h.deepseekClient.AnalyzeFailure(ctx, payload.Name, branch, status)
		cancel()

		if err != nil {
			log.Printf("[DeepSeek 诊断失败]: %v", err)
		} else if analysis != "" {
			aiAnalysisSection = fmt.Sprintf("\n\n🤖 **阿里云 DeepSeek 智能分析排错建议**:\n%s\n", analysis)
		}
	}

	// 🎯 4. 拼装高颜值 Markdown 消息体
	content := fmt.Sprintf(
		"## %s\n\n"+
			"--- \n"+
			"- **构建平台**: %s %s \n"+
			"- **应用/流水线**: `%s` \n"+
			"- **构建编号**: `#%d` \n"+
			"- **构建状态**: %s \n"+
			"- **Git 分支**: `%s` \n"+
			"- **构建耗时**: %s \n"+
			"- **完成时间**: %s \n%s"+
			"--- \n"+
			"🔗 [👉 点击查看构建日志详情](%s)",
		headerTag,
		platformIcon, source,
		payload.Name,
		payload.Build.Number,
		colorStatus,
		branch,
		durationStr,
		time.Now().Format("2006-01-02 15:04:05"),
		aiAnalysisSection,
		logUrl,
	)

	// ---------------------------------------------------------------------
	// 🚀 5. 解析目标群 openConversationId 列表（优先 Parameters，兜底使用配置文件）
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

	// 构造统一的 Notification 实体
	notification := &entity.Notification{
		Title:       title,
		Content:     content,
		Status:      status,
		ReceiverIDs: targetGroupIDs,
	}

	// ---------------------------------------------------------------------
	// 🚀 6. 异步调度企业内部应用群发驱动 (dingtalk_app_group) 进行推送
	// ---------------------------------------------------------------------
	go func(n *entity.Notification) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		logLabel := fmt.Sprintf("钉钉企业应用群发 (目标群数: %d)", len(n.ReceiverIDs))

		if err := h.useCase.Dispatch(ctx, "dingtalk_app_group", n); err != nil {
			log.Printf("[推送失败] %s: %v", logLabel, err)
		} else {
			log.Printf("[推送成功] %s", logLabel)
		}
	}(notification)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}
