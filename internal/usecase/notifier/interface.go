package notifier

import (
	"context"
	"fmt"
	"sync"

	"alert-gateway/internal/entity"
)

// Provider 抽象接口：所有通知驱动必须实现该接口
type Provider interface {
	Name() string
	Send(ctx context.Context, notification *entity.Notification) error
}

type ProviderFactory func(config map[string]interface{}) (Provider, error)

var (
	providersMu sync.RWMutex
	factories   = make(map[string]ProviderFactory)
)

// Register 驱动自注册函数（供各个 Provider 包在 init() 中调用）
func Register(name string, factory ProviderFactory) {
	providersMu.Lock()
	defer providersMu.Unlock()
	if factory == nil {
		panic("notifier: Register factory is nil")
	}
	if _, dup := factories[name]; dup {
		panic("notifier: Register called twice for provider " + name)
	}
	factories[name] = factory
}

// GetProvider 根据名称创建驱动实例
func GetProvider(name string, config map[string]interface{}) (Provider, error) {
	providersMu.RLock()
	factory, ok := factories[name]
	providersMu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("notifier: unknown provider %q (check main.go imports)", name)
	}
	return factory(config)
}
