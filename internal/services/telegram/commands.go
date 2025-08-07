package telegram

import (
	"fmt"
	"simplied-evm-monitoring-go/pkg/logger"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/sirupsen/logrus"
)

// CommandHandler 命令处理器
type CommandHandler struct {
	mu sync.RWMutex

	// 命令映射
	commands map[string]CommandFunc

	// 命令描述
	descriptions map[string]string

	// 命令权限
	permissions map[string]CommandPermission

	// 命令统计
	stats map[string]*CommandStats
}

// CommandFunc 命令处理函数类型
type CommandFunc func(bot *TelegramBot, message *tgbotapi.Message, args string) error

// CommandPermission 命令权限
type CommandPermission int

const (
	PermissionPublic CommandPermission = iota // 公开命令
	PermissionUser                            // 需要用户权限
	PermissionAdmin                           // 需要管理员权限
)

// CommandStats 命令统计信息
type CommandStats struct {
	TotalCalls   int64 `json:"total_calls"`
	SuccessCalls int64 `json:"success_calls"`
	FailedCalls  int64 `json:"failed_calls"`
	LastUsed     int64 `json:"last_used"`
	AverageTime  int64 `json:"average_time"`
}

// NewCommandHandler 创建新的命令处理器
func NewCommandHandler() *CommandHandler {
	return &CommandHandler{
		commands:     make(map[string]CommandFunc),
		descriptions: make(map[string]string),
		permissions:  make(map[string]CommandPermission),
		stats:        make(map[string]*CommandStats),
	}
}

// RegisterCommand 注册命令
func (ch *CommandHandler) RegisterCommand(command string, handler CommandFunc) {
	ch.RegisterCommandWithOptions(command, handler, &CommandOptions{
		Description: fmt.Sprintf("Execute %s command", command),
		Permission:  PermissionUser,
	})
}

// CommandOptions 命令选项
type CommandOptions struct {
	Description string
	Permission  CommandPermission
}

// RegisterCommandWithOptions 注册带选项的命令
func (ch *CommandHandler) RegisterCommandWithOptions(command string, handler CommandFunc, options *CommandOptions) {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	ch.commands[command] = handler

	if options != nil {
		ch.descriptions[command] = options.Description
		ch.permissions[command] = options.Permission
	} else {
		ch.descriptions[command] = fmt.Sprintf("Execute %s command", command)
		ch.permissions[command] = PermissionUser
	}

	ch.stats[command] = &CommandStats{}

	logger.WithFields(logrus.Fields{
		"command":     command,
		"description": ch.descriptions[command],
		"permission":  ch.permissions[command],
	}).Info("Command registered")
}

// UnregisterCommand 注销命令
func (ch *CommandHandler) UnregisterCommand(command string) {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	delete(ch.commands, command)
	delete(ch.descriptions, command)
	delete(ch.permissions, command)
	delete(ch.stats, command)

	logger.WithFields(logrus.Fields{
		"command": command,
	}).Info("Command unregistered")
}

// HandleCommand 处理命令
func (ch *CommandHandler) HandleCommand(bot *TelegramBot, message *tgbotapi.Message, command string, args string) error {
	ch.mu.RLock()
	handler, exists := ch.commands[command]
	permission := ch.permissions[command]
	ch.mu.RUnlock()

	if !exists {
		ch.updateCommandStats(command, false)
		return ch.handleUnknownCommand(bot, message, command)
	}

	// 检查权限
	if !ch.checkPermission(bot, message.From.ID, permission) {
		ch.updateCommandStats(command, false)
		return ch.handlePermissionDenied(bot, message, command)
	}

	// 执行命令
	startTime := getCurrentTimeMillis()
	err := handler(bot, message, args)
	executionTime := getCurrentTimeMillis() - startTime

	// 更新统计
	ch.updateCommandStatsWithTime(command, err == nil, executionTime)

	if err != nil {
		logger.WithFields(logrus.Fields{
			"command": command,
			"user_id": message.From.ID,
			"error":   err,
		}).Error("Command execution failed")
	} else {
		logger.WithFields(logrus.Fields{
			"command":        command,
			"user_id":        message.From.ID,
			"execution_time": executionTime,
		}).Info("Command executed successfully")
	}

	return err
}

// checkPermission 检查权限
func (ch *CommandHandler) checkPermission(bot *TelegramBot, userID int64, permission CommandPermission) bool {
	switch permission {
	case PermissionPublic:
		return true
	case PermissionUser:
		return true // 所有通过 isUserAllowed 检查的用户都有用户权限
	case PermissionAdmin:
		return bot.IsAdmin(userID)
	default:
		return false
	}
}

// handleUnknownCommand 处理未知命令
func (ch *CommandHandler) handleUnknownCommand(bot *TelegramBot, message *tgbotapi.Message, command string) error {
	logger.WithFields(logrus.Fields{
		"command": command,
		"user_id": message.From.ID,
	}).Warn("Unknown command received")

	responseText := fmt.Sprintf(`
❓ <b>未知命令: /%s</b>

使用 /help 查看可用命令列表。
`, command)

	return bot.SendMessage(message.Chat.ID, responseText)
}

// handlePermissionDenied 处理权限拒绝
func (ch *CommandHandler) handlePermissionDenied(bot *TelegramBot, message *tgbotapi.Message, command string) error {
	logger.WithFields(logrus.Fields{
		"command": command,
		"user_id": message.From.ID,
	}).Warn("Permission denied for command")

	responseText := fmt.Sprintf(`
🚫 <b>权限不足</b>

你没有权限执行命令 /%s。

如需帮助，请联系管理员。
`, command)

	return bot.SendMessage(message.Chat.ID, responseText)
}

// GetCommands 获取所有命令
func (ch *CommandHandler) GetCommands() map[string]string {
	ch.mu.RLock()
	defer ch.mu.RUnlock()

	commands := make(map[string]string)
	for cmd, desc := range ch.descriptions {
		commands[cmd] = desc
	}
	return commands
}

// GetCommandsForUser 获取用户可用的命令
func (ch *CommandHandler) GetCommandsForUser(bot *TelegramBot, userID int64) map[string]string {
	ch.mu.RLock()
	defer ch.mu.RUnlock()

	commands := make(map[string]string)
	for cmd, desc := range ch.descriptions {
		permission := ch.permissions[cmd]
		if ch.checkPermission(bot, userID, permission) {
			commands[cmd] = desc
		}
	}
	return commands
}

// GetCommandStats 获取命令统计
func (ch *CommandHandler) GetCommandStats() map[string]*CommandStats {
	ch.mu.RLock()
	defer ch.mu.RUnlock()

	stats := make(map[string]*CommandStats)
	for cmd, stat := range ch.stats {
		statCopy := *stat
		stats[cmd] = &statCopy
	}
	return stats
}

// updateCommandStats 更新命令统计
func (ch *CommandHandler) updateCommandStats(command string, success bool) {
	ch.updateCommandStatsWithTime(command, success, 0)
}

// updateCommandStatsWithTime 更新命令统计（带执行时间）
func (ch *CommandHandler) updateCommandStatsWithTime(command string, success bool, executionTime int64) {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	stats, exists := ch.stats[command]
	if !exists {
		stats = &CommandStats{}
		ch.stats[command] = stats
	}

	stats.TotalCalls++
	stats.LastUsed = getCurrentTimeMillis()

	if success {
		stats.SuccessCalls++
	} else {
		stats.FailedCalls++
	}

	if executionTime > 0 {
		if stats.AverageTime == 0 {
			stats.AverageTime = executionTime
		} else {
			stats.AverageTime = (stats.AverageTime + executionTime) / 2
		}
	}
}

// RegisterDefaultCommands 注册默认命令
func (ch *CommandHandler) RegisterDefaultCommands(bot *TelegramBot) {
	// 基础命令
	ch.RegisterCommandWithOptions("start", bot.handleStartCommand, &CommandOptions{
		Description: "开始使用机器人",
		Permission:  PermissionPublic,
	})

	ch.RegisterCommandWithOptions("help", bot.handleHelpCommand, &CommandOptions{
		Description: "显示帮助信息",
		Permission:  PermissionPublic,
	})

	ch.RegisterCommandWithOptions("status", bot.handleStatusCommand, &CommandOptions{
		Description: "查看系统状态",
		Permission:  PermissionUser,
	})

	// 管理员命令
	ch.RegisterCommandWithOptions("stats", ch.handleStatsCommand, &CommandOptions{
		Description: "查看详细统计信息",
		Permission:  PermissionAdmin,
	})

	ch.RegisterCommandWithOptions("users", ch.handleUsersCommand, &CommandOptions{
		Description: "查看用户信息",
		Permission:  PermissionAdmin,
	})

	ch.RegisterCommandWithOptions("broadcast", ch.handleBroadcastCommand, &CommandOptions{
		Description: "广播消息",
		Permission:  PermissionAdmin,
	})
}

// handleStatsCommand 处理统计命令
func (ch *CommandHandler) handleStatsCommand(bot *TelegramBot, message *tgbotapi.Message, args string) error {
	botStats := bot.GetStats()
	commandStats := ch.GetCommandStats()

	statsText := fmt.Sprintf(`
📈 <b>详细统计信息</b>

<b>Bot 统计：</b>
• 运行时间: %s
• 总消息数: %d
• 发送成功: %d
• 发送失败: %d
• 平均响应: %s

<b>命令统计：</b>
`,
		botStats["uptime"],
		botStats["total_messages"],
		botStats["sent_messages"],
		botStats["failed_messages"],
		botStats["average_response_time"],
	)

	// 添加每个命令的统计
	for cmd, stats := range commandStats {
		if stats.TotalCalls > 0 {
			successRate := float64(stats.SuccessCalls) / float64(stats.TotalCalls) * 100
			statsText += fmt.Sprintf("• /%s: %d次 (%.1f%% 成功)\n",
				cmd, stats.TotalCalls, successRate)
		}
	}

	return bot.SendMessage(message.Chat.ID, statsText)
}

// handleUsersCommand 处理用户命令
func (ch *CommandHandler) handleUsersCommand(bot *TelegramBot, message *tgbotapi.Message, args string) error {
	config := bot.GetConfig()

	usersText := fmt.Sprintf(`
👥 <b>用户管理</b>

<b>允许的用户数量:</b> %d
<b>管理员数量:</b> %d

<b>管理员列表:</b>
`, len(config.AllowedUsers), len(config.AdminUsers))

	for _, adminID := range config.AdminUsers {
		usersText += fmt.Sprintf("• %d\n", adminID)
	}

	return bot.SendMessage(message.Chat.ID, usersText)
}

// handleBroadcastCommand 处理广播命令
func (ch *CommandHandler) handleBroadcastCommand(bot *TelegramBot, message *tgbotapi.Message, args string) error {
	if args == "" {
		return bot.SendMessage(message.Chat.ID, "❌ 请提供要广播的消息内容\n\n用法: /broadcast <消息内容>")
	}

	config := bot.GetConfig()
	if len(config.AllowedUsers) == 0 {
		return bot.SendMessage(message.Chat.ID, "❌ 没有配置允许的用户列表")
	}

	broadcastText := fmt.Sprintf(`
📢 <b>系统广播</b>

%s

<i>来自管理员的消息</i>
`, args)

	successCount := 0
	failCount := 0

	// 向所有允许的用户发送消息
	for _, userID := range config.AllowedUsers {
		err := bot.SendMessage(userID, broadcastText)
		if err != nil {
			failCount++
			logger.WithFields(logrus.Fields{
				"user_id": userID,
				"error":   err,
			}).Error("Failed to send broadcast message")
		} else {
			successCount++
		}
	}

	resultText := fmt.Sprintf(`
✅ <b>广播完成</b>

• 成功发送: %d
• 发送失败: %d
• 总用户数: %d
`, successCount, failCount, len(config.AllowedUsers))

	return bot.SendMessage(message.Chat.ID, resultText)
}

// 工具函数

// getCurrentTimeMillis 获取当前时间毫秒数
func getCurrentTimeMillis() int64 {
	return time.Now().UnixNano() / int64(time.Millisecond)
}
