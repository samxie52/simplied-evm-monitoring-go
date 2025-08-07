# Telegram Bot Integration

本文档描述了如何设置和使用 Ethereum 监控系统的 Telegram Bot 集成功能。

## 功能概述

Telegram Bot 集成提供以下功能：

1. **实时警报通知** - 当警报规则被触发时，自动发送通知到 Telegram
2. **命令交互** - 通过 Telegram 命令查看系统状态和管理警报
3. **用户权限管理** - 支持用户访问控制和管理员权限
4. **消息优先级** - 根据警报严重程度设置消息优先级
5. **限流和重试** - 内置限流机制和消息发送重试逻辑

## 快速开始

### 1. 创建 Telegram Bot

1. 在 Telegram 中找到 [@BotFather](https://t.me/botfather)
2. 发送 `/newbot` 命令
3. 按照提示设置 Bot 名称和用户名
4. 获取 Bot Token（格式类似：`123456789:ABCDEFGHIJKLMNOPQRSTUVWXYZ`）

### 2. 配置环境变量

```bash
export TELEGRAM_BOT_TOKEN="your_bot_token_here"
```

### 3. 运行集成示例

```bash
cd examples
go run telegram_alert_integration.go
```

## 配置选项

### BotConfig 结构

```go
type BotConfig struct {
    Token                string        // Bot Token（必需）
    Debug                bool          // 调试模式
    UpdateTimeout        int           // 更新超时时间（秒）
    MessageQueueSize     int           // 消息队列大小
    MaxConcurrentMessages int          // 最大并发消息数
    AllowedUsers         []int64       // 允许的用户 ID 列表
    AdminUsers           []int64       // 管理员用户 ID 列表
}
```

### 默认配置值

- `UpdateTimeout`: 60 秒
- `MessageQueueSize`: 1000
- `MaxConcurrentMessages`: 10
- `Debug`: false

## 内置命令

### 基础命令

- `/start` - 启动 Bot 并显示欢迎消息
- `/help` - 显示帮助信息和可用命令列表
- `/status` - 显示 Bot 运行状态

### 管理员命令

- `/admin_stats` - 显示详细的系统统计信息
- `/admin_users` - 显示用户列表和权限信息

### 自定义命令（在集成示例中）

- `/alerts` - 显示当前活跃的警报规则
- `/stats` - 显示系统统计信息

## 警报消息格式

警报消息会自动格式化为易读的 Telegram 消息：

```
🚨 *ALERT: Large Transfer Detected*

**Severity:** High
**Type:** LargeTransfer
**Description:** Transaction exceeds 100 ETH threshold
**Trigger Value:** 150.5 ETH
**Threshold:** 100 ETH
**Time:** 2025-01-15 14:30:25 UTC
```

## API 使用示例

### 创建和启动 Bot

```go
package main

import (
    "log"
    "simplied-evm-monitoring-go/internal/services/telegram"
)

func main() {
    // 创建配置
    config := &telegram.BotConfig{
        Token:                "your_bot_token",
        Debug:                false,
        UpdateTimeout:        30,
        MessageQueueSize:     1000,
        MaxConcurrentMessages: 10,
        AllowedUsers:         []int64{123456789}, // 你的用户 ID
        AdminUsers:           []int64{123456789}, // 管理员 ID
    }

    // 创建 Bot
    bot, err := telegram.NewTelegramBot(config)
    if err != nil {
        log.Fatal(err)
    }

    // 启动 Bot
    if err := bot.Start(); err != nil {
        log.Fatal(err)
    }
    defer bot.Stop()

    // Bot 现在正在运行...
}
```

### 发送消息

```go
// 发送普通消息
err := bot.SendMessage(chatID, "Hello from monitoring system!", telegram.PriorityNormal)

// 发送高优先级消息
err := bot.SendMessage(chatID, "🚨 Critical Alert!", telegram.PriorityHigh)

// 发送紧急消息
err := bot.SendMessage(chatID, "🆘 System Emergency!", telegram.PriorityUrgent)
```

### 注册自定义命令

```go
commandHandler := bot.GetCommandHandler()

commandHandler.RegisterCommand("custom", func(bot *telegram.TelegramBot, message *tgbotapi.Message, args string) error {
    response := "This is a custom command response"
    return bot.SendMessage(message.Chat.ID, response, telegram.PriorityNormal)
})
```

## 权限管理

### 用户权限级别

1. **普通用户** - 在 `AllowedUsers` 列表中的用户
   - 可以接收警报通知
   - 可以使用基础命令

2. **管理员用户** - 在 `AdminUsers` 列表中的用户
   - 拥有普通用户的所有权限
   - 可以使用管理员命令
   - 可以查看详细的系统统计

### 获取用户 ID

要获取 Telegram 用户 ID，可以：

1. 向 [@userinfobot](https://t.me/userinfobot) 发送任何消息
2. 或者在 Bot 日志中查看接收到的消息

## 性能和限制

### 限流配置

```go
type RateLimitConfig struct {
    RequestsPerSecond int           // 每秒请求数限制
    BurstSize         int           // 突发请求大小
    WindowSize        time.Duration // 时间窗口大小
}
```

### Telegram API 限制

- 每个 Bot 每秒最多 30 个请求
- 每个聊天每秒最多 1 个请求
- 消息长度限制：4096 个字符

### 性能基准

基于测试结果：

- 命令处理：~107 ns/op
- 限流器处理：~5.2 ms/op（包含等待时间）

## 错误处理和重试

### 重试配置

```go
type RetryConfig struct {
    MaxRetries    int           // 最大重试次数
    InitialDelay  time.Duration // 初始延迟
    MaxDelay      time.Duration // 最大延迟
    BackoffFactor float64       // 退避因子
}
```

### 错误分类

- **临时错误** - 网络超时、限流等，会自动重试
- **永久错误** - 无效 Token、权限不足等，不会重试
- **用户错误** - 用户阻止 Bot、聊天不存在等

## 监控和统计

### Bot 统计信息

```go
stats := bot.GetStats()
// 包含以下信息：
// - is_running: Bot 运行状态
// - total_messages: 总消息数
// - sent_messages: 已发送消息数
// - received_messages: 已接收消息数
// - failed_messages: 失败消息数
// - total_commands: 总命令数
// - success_commands: 成功命令数
// - failed_commands: 失败命令数
// - start_time: 启动时间
// - uptime: 运行时长
```

### 命令统计

```go
commandStats := bot.GetCommandHandler().GetCommandStats()
// 每个命令的统计信息：
// - total_calls: 总调用次数
// - success_calls: 成功调用次数
// - failed_calls: 失败调用次数
// - avg_duration: 平均执行时间
```

## 安全考虑

1. **Token 安全** - 永远不要在代码中硬编码 Bot Token
2. **用户验证** - 始终验证用户权限
3. **输入验证** - 验证和清理用户输入
4. **日志安全** - 不要在日志中记录敏感信息

## 故障排除

### 常见问题

1. **Bot 无法启动**
   - 检查 Token 是否正确
   - 确保网络连接正常
   - 查看错误日志

2. **消息发送失败**
   - 检查用户是否阻止了 Bot
   - 验证聊天 ID 是否正确
   - 检查消息格式是否符合 Telegram 要求

3. **命令不响应**
   - 确保用户在允许列表中
   - 检查命令是否正确注册
   - 查看命令处理错误日志

### 调试模式

启用调试模式以获取详细日志：

```go
config := &telegram.BotConfig{
    Token: "your_token",
    Debug: true, // 启用调试模式
}
```

## 部署建议

1. **使用环境变量** 管理配置
2. **设置适当的日志级别** 用于生产环境
3. **监控 Bot 健康状态** 和性能指标
4. **定期备份** 用户配置和统计数据
5. **实施优雅关闭** 处理系统重启

## 扩展功能

### 计划中的功能

- [ ] 消息模板系统
- [ ] 多语言支持
- [ ] 警报规则的 Telegram 界面管理
- [ ] 图表和可视化支持
- [ ] 群组聊天支持
- [ ] Webhook 模式支持

### 自定义扩展

你可以通过以下方式扩展 Bot 功能：

1. 注册自定义命令处理器
2. 实现自定义消息格式化器
3. 添加自定义权限检查逻辑
4. 集成外部服务和 API

## 相关文档

- [Alert Rule Engine](./alert_rule_engine.md)
- [API Documentation](./api_documentation.md)
- [Development Guide](./development_guide.md)
