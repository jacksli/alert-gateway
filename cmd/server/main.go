package main

import (
	"log"
	"net/http"

	"alert-gateway/config"
	v1 "alert-gateway/internal/delivery/http/v1"
	"alert-gateway/internal/delivery/middleware"
	"alert-gateway/internal/pkg/deepseek"
	"alert-gateway/internal/usecase/notifier"

	// 匿名导入驱动包
	_ "alert-gateway/internal/provider/dingtalk_app_group" // 🚀 内部应用发钉钉群
	_ "alert-gateway/internal/provider/dingtalk_robot"
	_ "alert-gateway/internal/provider/dingtalk_webhook"
	_ "alert-gateway/internal/provider/email"
	_ "alert-gateway/internal/provider/jenkins_dingtalk_robot"
)

func main() {
	// 1. 读取 YAML 配置文件
	cfg, err := config.LoadConfig("config/config.yaml")
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// 2. 初始化 UseCase
	uc, err := notifier.NewNotifierUseCase(cfg.Providers)
	if err != nil {
		log.Fatalf("UseCase 初始化失败: %v", err)
	}

	// 3. 初始化 阿里云 DeepSeek API 客户端
	dsClient := deepseek.NewClient(cfg.AliyunDeepSeekAPIKey)

	// 4. 初始化 Delivery Handlers
	alertHandler := v1.NewHandler(uc, cfg)
	jenkinsHandler := v1.NewJenkinsHandler(uc, cfg, dsClient)
	dingtalkRobotHandler := v1.NewDingTalkRobotHandler(uc, cfg, dsClient)

	// 5. 创建 HTTP 路由 multiplexer
	mux := http.NewServeMux()
	mux.Handle("/api/v1/webhook", alertHandler)
	mux.Handle("/api/v1/jenkins", jenkinsHandler)
	mux.Handle("/api/v1/dingtalk/robot", dingtalkRobotHandler)

	// 健康检查接口（可选）
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"up"}`))
	})

	// 🎯 6. 使用中间件对全局所有路由统一开启 Token 校验
	protectedHandler := middleware.TokenAuthMiddleware(cfg.Auth.APIToken, mux)

	log.Printf("🎉 告警网关启动完成，已开启全局 Token 认证，监听端口 :8080...")
	log.Printf("👉 Webhook 接口: /api/v1/webhook")
	log.Printf("👉 CI/CD 接口: /api/v1/jenkins")
	log.Printf("👉 钉钉机器人回调: /api/v1/dingtalk/robot")

	if err := http.ListenAndServe(":8080", protectedHandler); err != nil {
		log.Fatal(err)
	}
}
