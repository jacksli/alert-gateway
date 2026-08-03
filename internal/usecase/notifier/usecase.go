package notifier

import (
	"context"
	"fmt"
	"log"

	"alert-gateway/internal/entity"
)

type NotifierUseCase struct {
	activeProviders map[string]Provider
}

func NewNotifierUseCase(providerConfigs map[string]map[string]interface{}) (*NotifierUseCase, error) {
	active := make(map[string]Provider)

	for name, cfg := range providerConfigs {
		p, err := GetProvider(name, cfg)
		if err != nil {
			log.Printf("初始化 Provider [%s] 失败: %v", name, err)
			continue
		}
		active[name] = p
		log.Printf("成功载入通知渠道 Provider: [%s]", name)
	}

	return &NotifierUseCase{activeProviders: active}, nil
}

// Dispatch 消息调度分发
func (uc *NotifierUseCase) Dispatch(ctx context.Context, channel string, notification *entity.Notification) error {
	p, ok := uc.activeProviders[channel]
	if !ok {
		return fmt.Errorf("渠道 %s 未激活或未配置", channel)
	}
	return p.Send(ctx, notification)
}
