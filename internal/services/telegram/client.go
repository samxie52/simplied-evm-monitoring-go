package telegram

import (
	"fmt"
	"simplied-evm-monitoring-go/pkg/logger"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/sirupsen/logrus"
)

// TelegramClient Telegram API 客户端
type TelegramClient struct {
	mu sync.RWMutex

	// Bot API
	api *tgbotapi.BotAPI

	// 配置
	config *ClientConfig

	// 连接状态
	isConnected bool
	lastPing    time.Time

	// 请求统计
	stats *ClientStats

	// 限流器
	rateLimiter *RateLimiter
}

// ClientConfig 客户端配置
type ClientConfig struct {
	// 请求超时时间
	Timeout time.Duration `json:"timeout"`

	// 最大重试次数
	MaxRetries int `json:"max_retries"`

	// 重试延迟
	RetryDelay time.Duration `json:"retry_delay"`

	// 连接检查间隔
	PingInterval time.Duration `json:"ping_interval"`

	// 限流配置
	RateLimit *RateLimitConfig `json:"rate_limit"`
}

// ClientStats 客户端统计信息
type ClientStats struct {
	mu sync.RWMutex

	// 请求统计
	TotalRequests   int64 `json:"total_requests"`
	SuccessRequests int64 `json:"success_requests"`
	FailedRequests  int64 `json:"failed_requests"`
	RetryRequests   int64 `json:"retry_requests"`

	// 响应时间统计
	AverageResponseTime time.Duration `json:"average_response_time"`
	MaxResponseTime     time.Duration `json:"max_response_time"`
	MinResponseTime     time.Duration `json:"min_response_time"`

	// 错误统计
	TimeoutErrors   int64 `json:"timeout_errors"`
	NetworkErrors   int64 `json:"network_errors"`
	APIErrors       int64 `json:"api_errors"`
	RateLimitErrors int64 `json:"rate_limit_errors"`

	// 连接统计
	ConnectionAttempts int64     `json:"connection_attempts"`
	ConnectionFailures int64     `json:"connection_failures"`
	LastConnected      time.Time `json:"last_connected"`
	LastError          time.Time `json:"last_error"`

	// 时间戳
	StartTime time.Time `json:"start_time"`
}

// RateLimitConfig 限流配置
type RateLimitConfig struct {
	// 每秒请求数限制
	RequestsPerSecond int `json:"requests_per_second"`

	// 突发请求数限制
	BurstSize int `json:"burst_size"`

	// 限流窗口大小
	WindowSize time.Duration `json:"window_size"`
}

// RateLimiter 限流器
type RateLimiter struct {
	mu sync.Mutex

	config     *RateLimitConfig
	tokens     int
	lastRefill time.Time
}

// NewTelegramClient 创建新的 Telegram 客户端
func NewTelegramClient(api *tgbotapi.BotAPI, config *ClientConfig) *TelegramClient {
	if config == nil {
		config = &ClientConfig{
			Timeout:      30 * time.Second,
			MaxRetries:   3,
			RetryDelay:   time.Second,
			PingInterval: 5 * time.Minute,
			RateLimit: &RateLimitConfig{
				RequestsPerSecond: 30,
				BurstSize:         10,
				WindowSize:        time.Second,
			},
		}
	}

	client := &TelegramClient{
		api:         api,
		config:      config,
		isConnected: false,
		stats:       &ClientStats{StartTime: time.Now()},
		rateLimiter: NewRateLimiter(config.RateLimit),
	}

	// 启动连接检查
	go client.connectionChecker()

	logger.WithFields(logrus.Fields{
		"timeout":       config.Timeout,
		"max_retries":   config.MaxRetries,
		"ping_interval": config.PingInterval,
	}).Info("Telegram client created")

	return client
}

// NewRateLimiter 创建新的限流器
func NewRateLimiter(config *RateLimitConfig) *RateLimiter {
	return &RateLimiter{
		config:     config,
		tokens:     config.BurstSize,
		lastRefill: time.Now(),
	}
}

// SendMessage 发送消息
func (tc *TelegramClient) SendMessage(chatID int64, text string, options *MessageSendOptions) (*tgbotapi.Message, error) {
	if options == nil {
		options = &MessageSendOptions{
			ParseMode: "HTML",
		}
	}

	msg := tgbotapi.NewMessage(chatID, text)
	if options.ParseMode != "" {
		msg.ParseMode = options.ParseMode
	}
	if options.ReplyMarkup != nil {
		msg.ReplyMarkup = options.ReplyMarkup
	}
	if options.DisablePreview {
		msg.DisableWebPagePreview = true
	}
	if options.DisableNotification {
		msg.DisableNotification = true
	}

	return tc.sendRequest(msg)
}

// MessageSendOptions 消息发送选项
type MessageSendOptions struct {
	ParseMode           string
	ReplyMarkup         interface{}
	DisablePreview      bool
	DisableNotification bool
}

// EditMessage 编辑消息
func (tc *TelegramClient) EditMessage(chatID int64, messageID int, text string, options *MessageEditOptions) (*tgbotapi.Message, error) {
	if options == nil {
		options = &MessageEditOptions{
			ParseMode: "HTML",
		}
	}

	edit := tgbotapi.NewEditMessageText(chatID, messageID, text)
	if options.ParseMode != "" {
		edit.ParseMode = options.ParseMode
	}
	if options.ReplyMarkup != nil {
		if markup, ok := options.ReplyMarkup.(*tgbotapi.InlineKeyboardMarkup); ok {
			edit.ReplyMarkup = markup
		}
	}

	return tc.sendRequest(edit)
}

// MessageEditOptions 消息编辑选项
type MessageEditOptions struct {
	ParseMode   string
	ReplyMarkup interface{}
}

// DeleteMessage 删除消息
func (tc *TelegramClient) DeleteMessage(chatID int64, messageID int) error {
	delete := tgbotapi.NewDeleteMessage(chatID, messageID)
	_, err := tc.sendRequest(delete)
	return err
}

// SendPhoto 发送图片
func (tc *TelegramClient) SendPhoto(chatID int64, photo interface{}, caption string) (*tgbotapi.Message, error) {
	var msg tgbotapi.PhotoConfig

	switch p := photo.(type) {
	case string:
		msg = tgbotapi.NewPhoto(chatID, tgbotapi.FilePath(p))
	case tgbotapi.FileBytes:
		msg = tgbotapi.NewPhoto(chatID, p)
	case tgbotapi.FileURL:
		msg = tgbotapi.NewPhoto(chatID, p)
	default:
		return nil, fmt.Errorf("unsupported photo type")
	}

	if caption != "" {
		msg.Caption = caption
		msg.ParseMode = "HTML"
	}

	return tc.sendRequest(msg)
}

// SendDocument 发送文档
func (tc *TelegramClient) SendDocument(chatID int64, document interface{}, caption string) (*tgbotapi.Message, error) {
	var msg tgbotapi.DocumentConfig

	switch d := document.(type) {
	case string:
		msg = tgbotapi.NewDocument(chatID, tgbotapi.FilePath(d))
	case tgbotapi.FileBytes:
		msg = tgbotapi.NewDocument(chatID, d)
	case tgbotapi.FileURL:
		msg = tgbotapi.NewDocument(chatID, d)
	default:
		return nil, fmt.Errorf("unsupported document type")
	}

	if caption != "" {
		msg.Caption = caption
		msg.ParseMode = "HTML"
	}

	return tc.sendRequest(msg)
}

// GetMe 获取 Bot 信息
func (tc *TelegramClient) GetMe() (*tgbotapi.User, error) {
	startTime := time.Now()
	defer func() {
		tc.updateResponseTimeStats(time.Since(startTime))
	}()

	// 等待限流器
	if err := tc.rateLimiter.Wait(); err != nil {
		tc.updateStats(func(stats *ClientStats) {
			stats.RateLimitErrors++
		})
		return nil, err
	}

	tc.updateStats(func(stats *ClientStats) {
		stats.TotalRequests++
	})

	user, err := tc.api.GetMe()
	if err != nil {
		tc.updateStats(func(stats *ClientStats) {
			stats.FailedRequests++
			stats.LastError = time.Now()
		})
		return nil, err
	}

	tc.updateStats(func(stats *ClientStats) {
		stats.SuccessRequests++
	})

	return &user, nil
}

// sendRequest 发送请求
func (tc *TelegramClient) sendRequest(c tgbotapi.Chattable) (*tgbotapi.Message, error) {
	startTime := time.Now()
	defer func() {
		tc.updateResponseTimeStats(time.Since(startTime))
	}()

	// 等待限流器
	if err := tc.rateLimiter.Wait(); err != nil {
		tc.updateStats(func(stats *ClientStats) {
			stats.RateLimitErrors++
		})
		return nil, err
	}

	tc.updateStats(func(stats *ClientStats) {
		stats.TotalRequests++
	})

	var lastErr error
	for attempt := 0; attempt <= tc.config.MaxRetries; attempt++ {
		if attempt > 0 {
			tc.updateStats(func(stats *ClientStats) {
				stats.RetryRequests++
			})
			time.Sleep(tc.config.RetryDelay * time.Duration(attempt))
		}

		// ctx, cancel := context.WithTimeout(context.Background(), tc.config.Timeout)

		// 使用上下文发送请求
		response, err := tc.api.Send(c)
		// cancel()

		if err == nil {
			tc.updateStats(func(stats *ClientStats) {
				stats.SuccessRequests++
			})
			return &response, nil
		}

		lastErr = err

		// 检查错误类型
		if tc.isTimeoutError(err) {
			tc.updateStats(func(stats *ClientStats) {
				stats.TimeoutErrors++
			})
		} else if tc.isNetworkError(err) {
			tc.updateStats(func(stats *ClientStats) {
				stats.NetworkErrors++
			})
		} else if tc.isAPIError(err) {
			tc.updateStats(func(stats *ClientStats) {
				stats.APIErrors++
			})
		}

		// 检查是否应该重试
		if !tc.shouldRetry(err) {
			break
		}

		logger.WithFields(logrus.Fields{
			"attempt": attempt + 1,
			"error":   err,
		}).Warn("Request failed, retrying")
	}

	tc.updateStats(func(stats *ClientStats) {
		stats.FailedRequests++
		stats.LastError = time.Now()
	})

	return nil, fmt.Errorf("request failed after %d attempts: %w", tc.config.MaxRetries+1, lastErr)
}

// connectionChecker 连接检查器
func (tc *TelegramClient) connectionChecker() {
	ticker := time.NewTicker(tc.config.PingInterval)
	defer ticker.Stop()

	for range ticker.C {
		tc.checkConnection()
	}
}

// checkConnection 检查连接
func (tc *TelegramClient) checkConnection() {
	tc.updateStats(func(stats *ClientStats) {
		stats.ConnectionAttempts++
	})

	_, err := tc.GetMe()
	if err != nil {
		tc.mu.Lock()
		tc.isConnected = false
		tc.mu.Unlock()

		tc.updateStats(func(stats *ClientStats) {
			stats.ConnectionFailures++
		})

		logger.WithFields(logrus.Fields{
			"error": err,
		}).Error("Telegram connection check failed")
	} else {
		tc.mu.Lock()
		tc.isConnected = true
		tc.lastPing = time.Now()
		tc.mu.Unlock()

		tc.updateStats(func(stats *ClientStats) {
			stats.LastConnected = time.Now()
		})

		logger.Debug("Telegram connection check successful")
	}
}

// IsConnected 检查是否连接
func (tc *TelegramClient) IsConnected() bool {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	return tc.isConnected
}

// GetLastPing 获取最后 ping 时间
func (tc *TelegramClient) GetLastPing() time.Time {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	return tc.lastPing
}

// shouldRetry 检查是否应该重试
func (tc *TelegramClient) shouldRetry(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()

	// 可重试的错误
	retryableErrors := []string{
		"timeout",
		"connection refused",
		"connection reset",
		"temporary failure",
		"service unavailable",
		"bad gateway",
		"gateway timeout",
	}

	for _, retryableErr := range retryableErrors {
		if contains(errStr, retryableErr) {
			return true
		}
	}

	return false
}

// isTimeoutError 检查是否为超时错误
func (tc *TelegramClient) isTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	return contains(err.Error(), "timeout")
}

// isNetworkError 检查是否为网络错误
func (tc *TelegramClient) isNetworkError(err error) bool {
	if err == nil {
		return false
	}

	networkErrors := []string{
		"connection refused",
		"connection reset",
		"network unreachable",
		"host unreachable",
	}

	errStr := err.Error()
	for _, netErr := range networkErrors {
		if contains(errStr, netErr) {
			return true
		}
	}
	return false
}

// isAPIError 检查是否为 API 错误
func (tc *TelegramClient) isAPIError(err error) bool {
	if err == nil {
		return false
	}

	// Telegram API 特定错误
	apiErrors := []string{
		"Bad Request",
		"Unauthorized",
		"Forbidden",
		"Not Found",
		"Too Many Requests",
	}

	errStr := err.Error()
	for _, apiErr := range apiErrors {
		if contains(errStr, apiErr) {
			return true
		}
	}
	return false
}

// Wait 等待限流器允许
func (rl *RateLimiter) Wait() error {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()

	// 计算需要补充的令牌数
	elapsed := now.Sub(rl.lastRefill)
	tokensToAdd := int(elapsed.Seconds() * float64(rl.config.RequestsPerSecond))

	if tokensToAdd > 0 {
		rl.tokens += tokensToAdd
		if rl.tokens > rl.config.BurstSize {
			rl.tokens = rl.config.BurstSize
		}
		rl.lastRefill = now
	}

	// 检查是否有可用令牌
	if rl.tokens <= 0 {
		// 计算需要等待的时间
		waitTime := time.Duration(float64(time.Second) / float64(rl.config.RequestsPerSecond))
		time.Sleep(waitTime)
		rl.tokens = 1
	}

	rl.tokens--
	return nil
}

// updateStats 更新统计信息
func (tc *TelegramClient) updateStats(updateFunc func(*ClientStats)) {
	tc.stats.mu.Lock()
	defer tc.stats.mu.Unlock()
	updateFunc(tc.stats)
}

// updateResponseTimeStats 更新响应时间统计
func (tc *TelegramClient) updateResponseTimeStats(responseTime time.Duration) {
	tc.updateStats(func(stats *ClientStats) {
		if stats.MinResponseTime == 0 || responseTime < stats.MinResponseTime {
			stats.MinResponseTime = responseTime
		}
		if responseTime > stats.MaxResponseTime {
			stats.MaxResponseTime = responseTime
		}

		// 计算平均响应时间
		if stats.SuccessRequests > 0 {
			totalTime := stats.AverageResponseTime * time.Duration(stats.SuccessRequests-1)
			stats.AverageResponseTime = (totalTime + responseTime) / time.Duration(stats.SuccessRequests)
		}
	})
}

// GetStats 获取统计信息
func (tc *TelegramClient) GetStats() map[string]interface{} {
	tc.stats.mu.RLock()
	defer tc.stats.mu.RUnlock()

	return map[string]interface{}{
		"is_connected":          tc.IsConnected(),
		"last_ping":             tc.GetLastPing(),
		"total_requests":        tc.stats.TotalRequests,
		"success_requests":      tc.stats.SuccessRequests,
		"failed_requests":       tc.stats.FailedRequests,
		"retry_requests":        tc.stats.RetryRequests,
		"average_response_time": tc.stats.AverageResponseTime.String(),
		"max_response_time":     tc.stats.MaxResponseTime.String(),
		"min_response_time":     tc.stats.MinResponseTime.String(),
		"timeout_errors":        tc.stats.TimeoutErrors,
		"network_errors":        tc.stats.NetworkErrors,
		"api_errors":            tc.stats.APIErrors,
		"rate_limit_errors":     tc.stats.RateLimitErrors,
		"connection_attempts":   tc.stats.ConnectionAttempts,
		"connection_failures":   tc.stats.ConnectionFailures,
		"last_connected":        tc.stats.LastConnected,
		"last_error":            tc.stats.LastError,
		"start_time":            tc.stats.StartTime,
		"uptime":                time.Since(tc.stats.StartTime).String(),
	}
}

// GetConfig 获取配置
func (tc *TelegramClient) GetConfig() *ClientConfig {
	tc.mu.RLock()
	defer tc.mu.RUnlock()

	// 返回配置副本
	configCopy := *tc.config
	return &configCopy
}
