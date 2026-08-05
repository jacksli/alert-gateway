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

type DingTalkRobotCallback struct {
	ConversationID     string `json:"conversationId"`
	OpenConversationID string `json:"openConversationId"`
	SenderNick         string `json:"senderNick"`
	Text               struct {
		Content string `json:"content"`
	} `json:"text"`
}

type DingTalkRobotHandler struct {
	useCase        *notifier.NotifierUseCase
	cfg            *config.Config
	deepseekClient *deepseek.Client
}

func NewDingTalkRobotHandler(uc *notifier.NotifierUseCase, cfg *config.Config, ds *deepseek.Client) *DingTalkRobotHandler {
	return &DingTalkRobotHandler{
		useCase:        uc,
		cfg:            cfg,
		deepseekClient: ds,
	}
}

func (h *DingTalkRobotHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var callback DingTalkRobotCallback
	if err := json.NewDecoder(r.Body).Decode(&callback); err != nil {
		http.Error(w, "Invalid Payload", http.StatusBadRequest)
		return
	}

	question := strings.TrimSpace(callback.Text.Content)
	if question == "" {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
		return
	}

	targetGroupID := callback.OpenConversationID
	if targetGroupID == "" {
		targetGroupID = callback.ConversationID
	}

	log.Printf("[钉钉机器人收到提问] 来自: %s, 内容: %s", callback.SenderNick, question)

	// 异步调用 AI 防止钉钉 5 秒超时限制
	go h.processAndReply(callback.SenderNick, question, targetGroupID)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

func (h *DingTalkRobotHandler) processAndReply(senderNick, question, groupID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	answer, err := h.deepseekClient.AskQuestion(ctx, senderNick, question)
	if err != nil {
		log.Printf("[DeepSeek 回答失败]: %v", err)
		answer = "🤖 AI 服务响应超时或异常，请稍后再试。"
	}

	content := fmt.Sprintf(
		"### 🤖 阿里云 DeepSeek 智能回答\n\n"+
			"**@%s 你的提问：**\n> %s\n\n"+
			"**AI 回答：**\n%s\n\n"+
			"--- \n"+
			"*回答生成时间：%s*",
		senderNick,
		question,
		answer,
		time.Now().Format("15:04:05"),
	)

	notification := &entity.Notification{
		Title:       "🤖 DeepSeek 智能回答",
		Content:     content,
		ReceiverIDs: []string{groupID},
	}

	dispatchCtx, dispatchCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer dispatchCancel()

	if err := h.useCase.Dispatch(dispatchCtx, "dingtalk_app_group", notification); err != nil {
		log.Printf("[回复钉钉群失败] 群ID %s: %v", groupID, err)
	} else {
		log.Printf("[回复钉钉群成功] 群ID %s", groupID)
	}
}
