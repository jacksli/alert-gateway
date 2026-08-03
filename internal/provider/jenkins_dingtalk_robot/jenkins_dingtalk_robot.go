package jenkins_dingtalk_robot

import (
	provider "alert-gateway/internal/provider/dingtalk_robot"
	"alert-gateway/internal/usecase/notifier"
)

const ProviderName = "jenkins_dingtalk_robot"

func init() {
	// 复用基础的 DingTalkRobotProvider，但使用专属名称 jenkins_dingtalk_robot 注册
	notifier.Register(ProviderName, func(cfg map[string]interface{}) (notifier.Provider, error) {
		return provider.NewDingTalkRobotProvider(cfg), nil
	})
}
