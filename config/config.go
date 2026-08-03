package config

import (
	"fmt"

	"github.com/spf13/viper"
)

// Config 全局配置结构体
type Config struct {
	Server    ServerConfig                      `mapstructure:"server"`
	Providers map[string]map[string]interface{} `mapstructure:"providers"`
}

type ServerConfig struct {
	Port string `mapstructure:"port"`
}

// LoadConfig 读取并解析配置文件
func LoadConfig(path string) (*Config, error) {
	v := viper.New()

	v.SetConfigFile(path)
	v.SetConfigType("yaml")

	// 读取环境变量（可选，方便 Docker 部署时通过环境变量覆盖）
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	return &cfg, nil
}
