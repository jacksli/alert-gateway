package config

import (
	"fmt"

	"github.com/spf13/viper"
)

type AuthConfig struct {
	APIToken string `mapstructure:"api_token"`
}

// Config 全局配置结构体
type Config struct {
	Auth      AuthConfig                        `mapstructure:"auth"`
	Server    ServerConfig                      `mapstructure:"server"`
	Providers map[string]map[string]interface{} `mapstructure:"providers"`
	// 修改为 []string 切片，支持多个默认接收人
	DefaultReceiverUserID  []string `mapstructure:"default_receiver_userid"`
	DefaultReceiverEmail   []string `mapstructure:"default_receiver_email"`   // 🚀 新增映射
	DefaultReceiverGroupID []string `mapstructure:"default_receiver_groupid"` // 企业钉钉群id
	AliyunDeepSeekAPIKey   string   `env:"ALIYUN_DEEPSEEK_API_KEY"`
}

type ServerConfig struct {
	Port string `mapstructure:"port"`
}

// DingTalkRobotConfig 定义钉钉应用机器人的专用配置
type DingTalkRobotConfig struct {
	AppKey     string `json:"app_key"`
	AppSecret  string `json:"app_secret"`
	AgentID    int64  `json:"agent_id"`
	EnableDing bool   `json:"enable_ding"`
	DingType   int    `json:"ding_type"` // 1: 应用内 DING; 2: 短信 DING; 3: 电话 DING
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
