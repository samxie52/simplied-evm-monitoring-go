package config

import (
	"time"
)

// ClientType 定义客户端连接类型
type EthereumClientType string

const (
	EthereumClientTypeHTTP      EthereumClientType = "http"      // HTTP连接
	EthereumClientTypeWebSocket EthereumClientType = "websocket" // WebSocket连接
	EthereumClientTypeIPC       EthereumClientType = "ipc"       // IPC连接
)

// Config 应用程序配置结构
type Config struct {
	// 应用程序配置
	App AppConfig `json:"app"`
	// 以太坊配置
	Ethereum EthereumConfig `json:"ethereum"`
	// Telegram配置
	Telegram TelegramConfig `json:"telegram"`
	// 日志配置
	Logging LoggingConfig `json:"logging"`
	// 告警配置
	Alert AlertConfig `json:"alert"`
}

// AppConfig 应用程序基础配置
type AppConfig struct {
	// 应用程序名称
	Name string `json:"name" env:"APP_NAME" validate:"required"`
	// 应用程序版本
	Version string `json:"version" env:"APP_VERSION" validate:"required"`
	// 应用程序运行环境
	Environment string `json:"environment" env:"APP_ENV" validate:"required,oneof=development staging production"`
	// 应用程序监听端口
	Port int `json:"port" env:"APP_PORT" validate:"required,min=1,max=65535"`
	// 应用程序监听主机
	Host string `json:"host" env:"APP_HOST" validate:"required"`
	// 是否启用调试模式
	Debug bool `json:"debug" env:"APP_DEBUG"`
}

// EthereumConfig 以太坊配置
type EthereumConfig struct {
	// 以太坊URL
	URL string `json:"url" env:"ETH_URL" validate:"required"`
	// 以太坊连接类型
	ClientType EthereumClientType `json:"client_type" env:"ETH_CLIENT_TYPE" validate:"required,oneof=http websocket ipc"`
	// 以太坊网络
	Network string `json:"network" env:"ETH_NETWORK" validate:"required,oneof=mainnet goerli sepolia"`
	// 以太坊链ID
	ChainID int64 `json:"chain_id" env:"ETH_CHAIN_ID" validate:"required"`
	// 以太坊超时时间
	Timeout int64 `json:"timeout" env:"ETH_TIMEOUT"`
	// 以太坊最大并发数
	MaxConcurrency int `json:"max_concurrency" env:"ETH_MAX_CONCURRENCY" validate:"min=1"`
	// 以太坊重试次数
	RetryAttempts int `json:"retry_attempts" env:"ETH_RETRY_ATTEMPTS" validate:"min=1"`
	// 以太坊重试延迟时间
	RetryDelay int64 `json:"retry_delay" env:"ETH_RETRY_DELAY" validate:"required"`
	// 以太坊优先级
	Priority int `json:"priority" env:"ETH_PRIORITY" validate:"min=1"`
}

// TelegramConfig Telegram Bot配置
type TelegramConfig struct {
	// Telegram Bot Token
	BotToken string `json:"bot_token" env:"TELEGRAM_BOT_TOKEN" validate:"required"`
	// Telegram Webhook URL
	WebhookURL string `json:"webhook_url" env:"TELEGRAM_WEBHOOK_URL" validate:"url"`
	// Telegram超时时间
	Timeout time.Duration `json:"timeout" env:"TELEGRAM_TIMEOUT"`
}

// LoggingConfig 日志配置
type LoggingConfig struct {
	// 日志级别
	Level string `json:"level" env:"LOG_LEVEL" validate:"required,oneof=debug info warn error fatal panic"`
	// 日志格式
	Format string `json:"format" env:"LOG_FORMAT" validate:"required,oneof=json text"`
	// 日志输出
	Output string `json:"output" env:"LOG_OUTPUT" validate:"required,oneof=stdout stderr file"`
	// 日志文件路径
	FilePath string `json:"file_path" env:"LOG_FILE_PATH"`
	// 日志文件最大大小（MB）
	MaxSize int `json:"max_size" env:"LOG_MAX_SIZE" validate:"min=1"`
	// 日志文件保留的旧文件数量
	MaxBackups int `json:"max_backups" env:"LOG_MAX_BACKUPS" validate:"min=1"`
	// 日志文件保留的旧文件天数
	MaxAge int `json:"max_age" env:"LOG_MAX_AGE" validate:"min=1"`
}

// AlertConfig 告警配置
type AlertConfig struct {
	// 告警冷却时间
	Cooldown time.Duration `json:"cooldown" env:"ALERT_COOLDOWN" validate:"required"`
	// 每小时最大告警次数
	MaxPerHour int `json:"max_per_hour" env:"MAX_ALERTS_PER_HOUR" validate:"min=1"`
	// 告警重试次数
	RetryAttempts int `json:"retry_attempts" env:"ALERT_RETRY_ATTEMPTS"`
	// 告警重试间隔
	RetryInterval time.Duration `json:"retry_interval" env:"ALERT_RETRY_INTERVAL"`
}

// IsProduction 判断是否为生产环境
func (a *AppConfig) IsProduction() bool {
	return a.Environment == "production"
}

// IsDevelopment 判断是否为开发环境
func (a *AppConfig) IsDevelopment() bool {
	return a.Environment == "development"
}
