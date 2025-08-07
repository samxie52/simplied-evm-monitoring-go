package telegram

import (
	"context"
	"fmt"
	"simplied-evm-monitoring-go/pkg/logger"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/sirupsen/logrus"
)

// TelegramBot Telegram Bot 服务
type TelegramBot struct {
	mu sync.RWMutex

	// 核心组件
	api    *tgbotapi.BotAPI
	client *TelegramClient

	// 配置
	config *BotConfig

	// 运行状态
	isRunning bool
	ctx       context.Context
	cancel    context.CancelFunc

	// 命令处理器
	commandHandler *CommandHandler

	// 统计信息
	stats *BotStats

	// 消息队列
	messageQueue chan MessageRequest

	// 重试配置
	retryConfig *RetryConfig
}

// BotConfig Telegram Bot 配置
type BotConfig struct {
	// Bot Token
	Token string `json:"token" validate:"required"`

	// 调试模式
	Debug bool `json:"debug"`

	// 更新超时时间
	UpdateTimeout int `json:"update_timeout"`

	// 允许的用户ID列表
	AllowedUsers []int64 `json:"allowed_users"`

	// 管理员用户ID列表
	AdminUsers []int64 `json:"admin_users"`

	// 消息队列大小
	MessageQueueSize int `json:"message_queue_size"`

	// 并发处理数
	MaxConcurrentMessages int `json:"max_concurrent_messages"`
}

// BotStats Bot 统计信息
type BotStats struct {
	mu sync.RWMutex

	// 消息统计
	TotalMessages    int64 `json:"total_messages"`
	SentMessages     int64 `json:"sent_messages"`
	ReceivedMessages int64 `json:"received_messages"`
	FailedMessages   int64 `json:"failed_messages"`

	// 命令统计
	TotalCommands   int64 `json:"total_commands"`
	SuccessCommands int64 `json:"success_commands"`
	FailedCommands  int64 `json:"failed_commands"`
	UnknownCommands int64 `json:"unknown_commands"`

	// 用户统计
	ActiveUsers  int64 `json:"active_users"`
	BlockedUsers int64 `json:"blocked_users"`

	// 性能统计
	AverageResponseTime time.Duration `json:"average_response_time"`
	MaxResponseTime     time.Duration `json:"max_response_time"`
	MinResponseTime     time.Duration `json:"min_response_time"`

	// 时间戳
	StartTime       time.Time `json:"start_time"`
	LastMessageTime time.Time `json:"last_message_time"`
	LastCommandTime time.Time `json:"last_command_time"`
}

// MessageRequest 消息发送请求
type MessageRequest struct {
	ChatID      int64
	Text        string
	ParseMode   string
	ReplyMarkup interface{}
	Priority    MessagePriority
	Retry       int
	CreatedAt   time.Time
	Callback    func(error)
}

// MessagePriority 消息优先级
type MessagePriority int

const (
	PriorityLow MessagePriority = iota
	PriorityNormal
	PriorityHigh
	PriorityUrgent
)

// RetryConfig 重试配置
type RetryConfig struct {
	MaxRetries      int           `json:"max_retries"`
	InitialDelay    time.Duration `json:"initial_delay"`
	MaxDelay        time.Duration `json:"max_delay"`
	BackoffFactor   float64       `json:"backoff_factor"`
	RetryableErrors []string      `json:"retryable_errors"`
}

// NewTelegramBot 创建新的 Telegram Bot
func NewTelegramBot(config *BotConfig) (*TelegramBot, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	if config.Token == "" {
		return nil, fmt.Errorf("bot token is required")
	}

	// 设置默认配置
	if config.UpdateTimeout == 0 {
		config.UpdateTimeout = 60
	}
	if config.MessageQueueSize == 0 {
		config.MessageQueueSize = 1000
	}
	if config.MaxConcurrentMessages == 0 {
		config.MaxConcurrentMessages = 10
	}

	// 创建 Bot API
	api, err := tgbotapi.NewBotAPI(config.Token)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot API: %w", err)
	}

	api.Debug = config.Debug

	// 创建客户端
	client := NewTelegramClient(api, &ClientConfig{
		Timeout:    30 * time.Second,
		MaxRetries: 3,
		RetryDelay: time.Second,
	})

	// 创建命令处理器
	commandHandler := NewCommandHandler()

	// 创建重试配置
	retryConfig := &RetryConfig{
		MaxRetries:    3,
		InitialDelay:  time.Second,
		MaxDelay:      30 * time.Second,
		BackoffFactor: 2.0,
		RetryableErrors: []string{
			"Too Many Requests",
			"Bad Gateway",
			"Service Unavailable",
			"Gateway Timeout",
		},
	}

	bot := &TelegramBot{
		api:            api,
		client:         client,
		config:         config,
		commandHandler: commandHandler,
		stats:          &BotStats{StartTime: time.Now()},
		messageQueue:   make(chan MessageRequest, config.MessageQueueSize),
		retryConfig:    retryConfig,
	}

	// 注册默认命令
	bot.registerDefaultCommands()

	logger.WithFields(logrus.Fields{
		"bot_username": api.Self.UserName,
		"bot_id":       api.Self.ID,
	}).Info("Telegram bot created successfully")

	return bot, nil
}

// Start 启动 Telegram Bot
func (tb *TelegramBot) Start() error {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	if tb.isRunning {
		return fmt.Errorf("telegram bot is already running")
	}

	tb.ctx, tb.cancel = context.WithCancel(context.Background())
	tb.isRunning = true

	logger.Info("Starting Telegram bot...")

	// 启动消息处理队列
	go tb.messageProcessor()

	// 启动更新接收循环
	go tb.updateReceiver()

	logger.WithFields(logrus.Fields{
		"bot_username":   tb.api.Self.UserName,
		"update_timeout": tb.config.UpdateTimeout,
		"queue_size":     tb.config.MessageQueueSize,
		"max_concurrent": tb.config.MaxConcurrentMessages,
	}).Info("Telegram bot started successfully")

	return nil
}

// Stop 停止 Telegram Bot
func (tb *TelegramBot) Stop() error {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	if !tb.isRunning {
		return fmt.Errorf("telegram bot is not running")
	}

	logger.Info("Stopping Telegram bot...")

	tb.cancel()
	tb.isRunning = false

	// 关闭消息队列
	close(tb.messageQueue)

	logger.Info("Telegram bot stopped")
	return nil
}

// IsRunning 检查 Bot 是否运行中
func (tb *TelegramBot) IsRunning() bool {
	tb.mu.RLock()
	defer tb.mu.RUnlock()
	return tb.isRunning
}

// SendMessage 发送消息
func (tb *TelegramBot) SendMessage(chatID int64, text string) error {
	return tb.SendMessageWithOptions(chatID, text, &MessageOptions{
		ParseMode: "HTML",
		Priority:  PriorityNormal,
	})
}

// MessageOptions 消息选项
type MessageOptions struct {
	ParseMode   string
	ReplyMarkup interface{}
	Priority    MessagePriority
	Callback    func(error)
}

// SendMessageWithOptions 发送带选项的消息
func (tb *TelegramBot) SendMessageWithOptions(chatID int64, text string, options *MessageOptions) error {
	if !tb.IsRunning() {
		return fmt.Errorf("telegram bot is not running")
	}

	if options == nil {
		options = &MessageOptions{
			ParseMode: "HTML",
			Priority:  PriorityNormal,
		}
	}

	request := MessageRequest{
		ChatID:      chatID,
		Text:        text,
		ParseMode:   options.ParseMode,
		ReplyMarkup: options.ReplyMarkup,
		Priority:    options.Priority,
		CreatedAt:   time.Now(),
		Callback:    options.Callback,
	}

	select {
	case tb.messageQueue <- request:
		return nil
	default:
		return fmt.Errorf("message queue is full")
	}
}

// updateReceiver 更新接收器
func (tb *TelegramBot) updateReceiver() {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = tb.config.UpdateTimeout

	updates := tb.api.GetUpdatesChan(u)

	for {
		select {
		case <-tb.ctx.Done():
			return
		case update := <-updates:
			go tb.handleUpdate(update)
		}
	}
}

// handleUpdate 处理更新
func (tb *TelegramBot) handleUpdate(update tgbotapi.Update) {
	startTime := time.Now()
	defer func() {
		responseTime := time.Since(startTime)
		tb.updateResponseTimeStats(responseTime)
	}()

	tb.updateStats(func(stats *BotStats) {
		stats.ReceivedMessages++
		stats.LastMessageTime = time.Now()
	})

	// 检查用户权限
	var userID int64
	if update.Message != nil {
		userID = update.Message.From.ID
	} else if update.CallbackQuery != nil {
		userID = update.CallbackQuery.From.ID
	}

	if !tb.isUserAllowed(userID) {
		logger.WithFields(logrus.Fields{
			"user_id": userID,
		}).Warn("Unauthorized user attempted to use bot")
		return
	}

	// 处理消息
	if update.Message != nil {
		tb.handleMessage(update.Message)
	}

	// 处理回调查询
	if update.CallbackQuery != nil {
		tb.handleCallbackQuery(update.CallbackQuery)
	}
}

// handleMessage 处理消息
func (tb *TelegramBot) handleMessage(message *tgbotapi.Message) {
	if message.IsCommand() {
		tb.handleCommand(message)
	} else {
		tb.handleTextMessage(message)
	}
}

// handleCommand 处理命令
func (tb *TelegramBot) handleCommand(message *tgbotapi.Message) {
	command := message.Command()
	args := message.CommandArguments()

	tb.updateStats(func(stats *BotStats) {
		stats.TotalCommands++
		stats.LastCommandTime = time.Now()
	})

	logger.WithFields(logrus.Fields{
		"user_id":  message.From.ID,
		"username": message.From.UserName,
		"command":  command,
		"args":     args,
	}).Info("Processing command")

	err := tb.commandHandler.HandleCommand(tb, message, command, args)
	if err != nil {
		tb.updateStats(func(stats *BotStats) {
			stats.FailedCommands++
		})

		logger.WithFields(logrus.Fields{
			"command": command,
			"error":   err,
		}).Error("Failed to handle command")

		tb.SendMessage(message.Chat.ID, fmt.Sprintf("❌ 命令执行失败: %s", err.Error()))
	} else {
		tb.updateStats(func(stats *BotStats) {
			stats.SuccessCommands++
		})
	}
}

// handleTextMessage 处理文本消息
func (tb *TelegramBot) handleTextMessage(message *tgbotapi.Message) {
	// 处理普通文本消息
	logger.WithFields(logrus.Fields{
		"user_id": message.From.ID,
		"text":    message.Text,
	}).Debug("Received text message")

	// 可以在这里添加自然语言处理逻辑
}

// handleCallbackQuery 处理回调查询
func (tb *TelegramBot) handleCallbackQuery(callback *tgbotapi.CallbackQuery) {
	// 处理内联键盘回调
	logger.WithFields(logrus.Fields{
		"user_id": callback.From.ID,
		"data":    callback.Data,
	}).Debug("Received callback query")

	// 确认回调查询
	config := tgbotapi.NewCallback(callback.ID, "")
	tb.api.Request(config)
}

// messageProcessor 消息处理器
func (tb *TelegramBot) messageProcessor() {
	semaphore := make(chan struct{}, tb.config.MaxConcurrentMessages)

	for {
		select {
		case <-tb.ctx.Done():
			return
		case request, ok := <-tb.messageQueue:
			if !ok {
				return
			}

			// 获取信号量
			semaphore <- struct{}{}
			go func(req MessageRequest) {
				defer func() { <-semaphore }()
				tb.processMessageRequest(req)
			}(request)
		}
	}
}

// processMessageRequest 处理消息请求
func (tb *TelegramBot) processMessageRequest(request MessageRequest) {
	var err error
	maxRetries := tb.retryConfig.MaxRetries
	delay := tb.retryConfig.InitialDelay

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(delay)
			delay = time.Duration(float64(delay) * tb.retryConfig.BackoffFactor)
			if delay > tb.retryConfig.MaxDelay {
				delay = tb.retryConfig.MaxDelay
			}
		}

		err = tb.sendMessageDirect(request)
		if err == nil {
			tb.updateStats(func(stats *BotStats) {
				stats.SentMessages++
			})
			if request.Callback != nil {
				request.Callback(nil)
			}
			return
		}

		// 检查是否为可重试错误
		if !tb.isRetryableError(err) {
			break
		}

		logger.WithFields(logrus.Fields{
			"attempt": attempt + 1,
			"error":   err,
			"delay":   delay,
		}).Warn("Message send failed, retrying")
	}

	// 所有重试都失败了
	tb.updateStats(func(stats *BotStats) {
		stats.FailedMessages++
	})

	logger.WithFields(logrus.Fields{
		"chat_id": request.ChatID,
		"error":   err,
	}).Error("Failed to send message after all retries")

	if request.Callback != nil {
		request.Callback(err)
	}
}

// sendMessageDirect 直接发送消息
func (tb *TelegramBot) sendMessageDirect(request MessageRequest) error {
	msg := tgbotapi.NewMessage(request.ChatID, request.Text)

	if request.ParseMode != "" {
		msg.ParseMode = request.ParseMode
	}

	if request.ReplyMarkup != nil {
		msg.ReplyMarkup = request.ReplyMarkup
	}

	_, err := tb.api.Send(msg)
	return err
}

// isRetryableError 检查是否为可重试错误
func (tb *TelegramBot) isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()
	for _, retryableErr := range tb.retryConfig.RetryableErrors {
		if contains(errStr, retryableErr) {
			return true
		}
	}
	return false
}

// isUserAllowed 检查用户是否被允许
func (tb *TelegramBot) isUserAllowed(userID int64) bool {
	if len(tb.config.AllowedUsers) == 0 {
		return true // 如果没有限制，允许所有用户
	}

	for _, allowedID := range tb.config.AllowedUsers {
		if userID == allowedID {
			return true
		}
	}
	return false
}

// IsAdmin 检查用户是否为管理员
func (tb *TelegramBot) IsAdmin(userID int64) bool {
	for _, adminID := range tb.config.AdminUsers {
		if userID == adminID {
			return true
		}
	}
	return false
}

// registerDefaultCommands 注册默认命令
func (tb *TelegramBot) registerDefaultCommands() {
	tb.commandHandler.RegisterCommand("start", tb.handleStartCommand)
	tb.commandHandler.RegisterCommand("help", tb.handleHelpCommand)
	tb.commandHandler.RegisterCommand("status", tb.handleStatusCommand)
}

// handleStartCommand 处理 /start 命令
func (tb *TelegramBot) handleStartCommand(bot *TelegramBot, message *tgbotapi.Message, args string) error {
	welcomeText := fmt.Sprintf(`
🤖 <b>欢迎使用以太坊监控机器人！</b>

你好 %s！我是你的以太坊区块链监控助手。

<b>可用命令：</b>
/help - 显示帮助信息
/status - 查看系统状态

开始使用吧！
`, message.From.FirstName)

	return bot.SendMessage(message.Chat.ID, welcomeText)
}

// handleHelpCommand 处理 /help 命令
func (tb *TelegramBot) handleHelpCommand(bot *TelegramBot, message *tgbotapi.Message, args string) error {
	helpText := `
📖 <b>帮助信息</b>

<b>基础命令：</b>
/start - 开始使用机器人
/help - 显示此帮助信息
/status - 查看系统运行状态

<b>功能说明：</b>
• 实时监控以太坊区块链
• 大额交易告警
• 自定义告警规则
• 系统状态监控

如需更多帮助，请联系管理员。
`

	return bot.SendMessage(message.Chat.ID, helpText)
}

// handleStatusCommand 处理 /status 命令
func (tb *TelegramBot) handleStatusCommand(bot *TelegramBot, message *tgbotapi.Message, args string) error {
	stats := tb.GetStats()

	statusText := fmt.Sprintf(`
📊 <b>系统状态</b>

<b>Bot 状态：</b>
• 运行状态: %s
• 运行时间: %s
• 最后消息时间: %s

<b>消息统计：</b>
• 总消息数: %d
• 发送消息: %d
• 接收消息: %d
• 失败消息: %d

<b>命令统计：</b>
• 总命令数: %d
• 成功命令: %d
• 失败命令: %d

<b>性能统计：</b>
• 平均响应时间: %s
• 最大响应时间: %s
`,
		getStatusEmoji(tb.IsRunning()),
		stats["uptime"],
		formatTime(stats["last_message_time"].(time.Time)),
		stats["total_messages"],
		stats["sent_messages"],
		stats["received_messages"],
		stats["failed_messages"],
		stats["total_commands"],
		stats["success_commands"],
		stats["failed_commands"],
		stats["average_response_time"],
		stats["max_response_time"],
	)

	return bot.SendMessage(message.Chat.ID, statusText)
}

// updateStats 更新统计信息
func (tb *TelegramBot) updateStats(updateFunc func(*BotStats)) {
	tb.stats.mu.Lock()
	defer tb.stats.mu.Unlock()
	updateFunc(tb.stats)
}

// updateResponseTimeStats 更新响应时间统计
func (tb *TelegramBot) updateResponseTimeStats(responseTime time.Duration) {
	tb.updateStats(func(stats *BotStats) {
		if stats.MinResponseTime == 0 || responseTime < stats.MinResponseTime {
			stats.MinResponseTime = responseTime
		}
		if responseTime > stats.MaxResponseTime {
			stats.MaxResponseTime = responseTime
		}

		// 计算平均响应时间
		totalMessages := stats.TotalMessages
		if totalMessages > 0 {
			totalTime := stats.AverageResponseTime * time.Duration(totalMessages-1)
			stats.AverageResponseTime = (totalTime + responseTime) / time.Duration(totalMessages)
		} else {
			stats.AverageResponseTime = responseTime
		}

		stats.TotalMessages++
	})
}

// GetStats 获取统计信息
func (tb *TelegramBot) GetStats() map[string]interface{} {
	tb.stats.mu.RLock()
	defer tb.stats.mu.RUnlock()

	return map[string]interface{}{
		"is_running":            tb.IsRunning(),
		"total_messages":        tb.stats.TotalMessages,
		"sent_messages":         tb.stats.SentMessages,
		"received_messages":     tb.stats.ReceivedMessages,
		"failed_messages":       tb.stats.FailedMessages,
		"total_commands":        tb.stats.TotalCommands,
		"success_commands":      tb.stats.SuccessCommands,
		"failed_commands":       tb.stats.FailedCommands,
		"unknown_commands":      tb.stats.UnknownCommands,
		"active_users":          tb.stats.ActiveUsers,
		"blocked_users":         tb.stats.BlockedUsers,
		"average_response_time": tb.stats.AverageResponseTime.String(),
		"max_response_time":     tb.stats.MaxResponseTime.String(),
		"min_response_time":     tb.stats.MinResponseTime.String(),
		"start_time":            tb.stats.StartTime,
		"last_message_time":     tb.stats.LastMessageTime,
		"last_command_time":     tb.stats.LastCommandTime,
		"uptime":                time.Since(tb.stats.StartTime).String(),
	}
}

// GetConfig 获取配置
func (tb *TelegramBot) GetConfig() *BotConfig {
	tb.mu.RLock()
	defer tb.mu.RUnlock()

	// 返回配置副本
	configCopy := *tb.config
	return &configCopy
}

// 工具函数

// contains 检查字符串是否包含子字符串
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			len(s) > len(substr) && s[1:len(substr)+1] == substr))
}

// getStatusEmoji 获取状态表情符号
func getStatusEmoji(isRunning bool) string {
	if isRunning {
		return "🟢 运行中"
	}
	return "🔴 已停止"
}

// formatTime 格式化时间
func formatTime(t time.Time) string {
	if t.IsZero() {
		return "从未"
	}
	return t.Format("2006-01-02 15:04:05")
}
