package main

import (
	"log"
	"net/http"

	v1 "alert-gateway/internal/delivery/http/v1"
	"alert-gateway/internal/usecase/notifier"

	// 重点：匿名导入自动激活 Provider 注册逻辑
	_ "alert-gateway/internal/provider/dingtalk_robot"   // 钉钉私信
	_ "alert-gateway/internal/provider/dingtalk_webhook" // 钉钉群 Webhook
	_ "alert-gateway/internal/provider/email"            // 邮件 SMTP
)

func main() {
	// 配置示例（生产环境建议读取 config/config.yaml）
	providerConfigs := map[string]map[string]interface{}{
		"dingtalk_robot": {
			"app_key":    "YOUR_DINGTALK_APP_KEY",
			"app_secret": "YOUR_DINGTALK_APP_SECRET",
		},
		"dingtalk_webhook": {
			"webhook_url": "https://oapi.dingtalk.com/robot/send?access_token=YOUR_ACCESS_TOKEN",
			"secret":      "SECxxxxxxx",
		},
		"email": {
			"smtp_host": "smtp.qq.com",
			"smtp_port": 465,
			"smtp_user": "alert@yourcompany.com",
			"smtp_pass": "your_smtp_password",
			"from":      "运维告警中心 <alert@yourcompany.com>",
		},
	}

	// 1. 初始化 UseCase
	uc, err := notifier.NewNotifierUseCase(providerConfigs)
	if err != nil {
		log.Fatalf("UseCase 初始化失败: %v", err)
	}

	// 2. 初始化 Delivery Handler
	handler := v1.NewHandler(uc)

	// 3. 注册 HTTP 路由
	http.Handle("/api/v1/webhook", handler)

	log.Println("🎉 核心企业告警网关启动完成，监听端口 :8080...")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}
