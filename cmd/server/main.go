package main

import (
	"alert-gateway/config"
	"log"
	"net/http"

	v1 "alert-gateway/internal/delivery/http/v1"
	"alert-gateway/internal/usecase/notifier"

	// 重点：匿名导入自动激活 Provider 注册逻辑
	_ "alert-gateway/internal/provider/dingtalk_robot"         // 钉钉私信
	_ "alert-gateway/internal/provider/dingtalk_webhook"       // 钉钉群 Webhook
	_ "alert-gateway/internal/provider/email"                  // 邮件 SMTP
	_ "alert-gateway/internal/provider/jenkins_dingtalk_robot" // 🚀 专属 Jenkins 机器人驱动
)

func main() {
	// 1. 从 YAML 配置文件读取配置
	cfg, err := config.LoadConfig("config/config.yaml")
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// 1. 初始化 UseCase
	uc, err := notifier.NewNotifierUseCase(cfg.Providers)
	if err != nil {
		log.Fatalf("UseCase 初始化失败: %v", err)
	}

	// 2. 初始化 Delivery Handler
	handler := v1.NewHandler(uc, cfg)

	// 3. 注册 HTTP 路由
	http.Handle("/api/v1/webhook", handler)

	jenkinsHandler := v1.NewJenkinsHandler(uc, cfg) // 🚀 新增 Jenkins 专属 Handler

	http.Handle("/api/v1/jenkins", jenkinsHandler) // 🚀 监听 Jenkins Webhook

	log.Println("🎉 核心企业告警网关启动完成，监听端口 :8080...")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}
