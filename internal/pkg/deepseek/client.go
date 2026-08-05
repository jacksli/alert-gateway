package deepseek

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Client struct {
	apiKey string
	apiUrl string
}

func NewClient(apiKey string) *Client {
	return &Client{
		apiKey: apiKey,
		apiUrl: "https://dashscope.aliyuncs.com/compatible-mode/v1/chat/completions",
	}
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Model    string    `json:"model"` // 推荐 deepseek-v3 或 deepseek-r1
	Messages []Message `json:"messages"`
}

type ChatResponse struct {
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`
}

// AnalyzeFailure CI/CD 失败时调用 DeepSeek 进行故障排查建议
func (c *Client) AnalyzeFailure(ctx context.Context, jobName, branch, status string) (string, error) {
	if c.apiKey == "" {
		return "", fmt.Errorf("deepseek api key is empty")
	}

	prompt := fmt.Sprintf(
		"请简短分析 CI/CD 构建失败的原因，并给出 2-3 条排查建议。\n构建项目：%s\nGit分支：%s\n构建状态：%s",
		jobName, branch, status,
	)

	return c.callAPI(ctx, "你是一个资深的 DevOps 和 CI/CD 排错专家，回答精炼简洁，格式使用 Markdown 列表。", prompt, 15*time.Second)
}

// AskQuestion 钉钉群 @ 机器人提问时调用 DeepSeek 生成答案
func (c *Client) AskQuestion(ctx context.Context, username, question string) (string, error) {
	if c.apiKey == "" {
		return "", fmt.Errorf("deepseek api key is empty")
	}

	prompt := fmt.Sprintf("用户 %s 提问：%s", username, question)
	return c.callAPI(ctx, "你是一个智能助手，回答专业、清晰、简洁，支持使用 Markdown 语法格式化输出。", prompt, 45*time.Second)
}

func (c *Client) callAPI(ctx context.Context, systemPrompt, userPrompt string, timeout time.Duration) (string, error) {
	reqBody := ChatRequest{
		Model: "deepseek-v3",
		Messages: []Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.apiUrl, bytes.NewBuffer(data))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("deepseek api error status: %d", resp.StatusCode)
	}

	var chatResp ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return "", err
	}

	if len(chatResp.Choices) > 0 {
		return chatResp.Choices[0].Message.Content, nil
	}

	return "", fmt.Errorf("empty response from deepseek")
}
