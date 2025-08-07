package main

import (
	"fmt"
	"os"
	"os/signal"
	"simplied-evm-monitoring-go/internal/services/telegram"
	"syscall"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/sirupsen/logrus"
)

func main() {
	// 设置日志级别
	logrus.SetLevel(logrus.InfoLevel)

	fmt.Println("🤖 Telegram Bot Demo - Offline Mode")
	fmt.Println("==================================")

	// 演示配置创建
	config := &telegram.BotConfig{
		Token:                 "demo_token_123456789:ABCDEFGHIJKLMNOPQRSTUVWXYZ",
		Debug:                 true,
		UpdateTimeout:         30,
		MessageQueueSize:      1000,
		MaxConcurrentMessages: 10,
		AllowedUsers:          []int64{123456789, 987654321},
		AdminUsers:            []int64{123456789},
	}

	fmt.Printf("✅ Bot Configuration Created:\n")
	fmt.Printf("   - Token: %s...\n", config.Token[:20])
	fmt.Printf("   - Update Timeout: %d seconds\n", config.UpdateTimeout)
	fmt.Printf("   - Message Queue Size: %d\n", config.MessageQueueSize)
	fmt.Printf("   - Max Concurrent Messages: %d\n", config.MaxConcurrentMessages)
	fmt.Printf("   - Allowed Users: %v\n", config.AllowedUsers)
	fmt.Printf("   - Admin Users: %v\n", config.AdminUsers)
	fmt.Println()

	// 演示命令处理器功能
	fmt.Println("📋 Demonstrating Command Handler:")
	commandHandler := telegram.NewCommandHandler()

	// 注册示例命令
	commandHandler.RegisterCommand("demo", func(bot *telegram.TelegramBot, message *tgbotapi.Message, args string) error {
		fmt.Printf("   🎯 Demo command executed with args: %s\n", args)
		return nil
	})

	commandHandler.RegisterCommand("test", func(bot *telegram.TelegramBot, message *tgbotapi.Message, args string) error {
		fmt.Printf("   🧪 Test command executed\n")
		return nil
	})

	// 显示注册的命令
	commands := commandHandler.GetCommands()
	fmt.Printf("   Registered Commands: %v\n", getCommandNames(commands))
	fmt.Println()

	// 演示限流器功能
	fmt.Println("⏱️  Demonstrating Rate Limiter:")
	rateLimitConfig := &telegram.RateLimitConfig{
		RequestsPerSecond: 5,
		BurstSize:         3,
		WindowSize:        time.Second,
	}

	rateLimiter := telegram.NewRateLimiter(rateLimitConfig)
	fmt.Printf("   Rate Limit Config: %d req/sec, burst: %d\n",
		rateLimitConfig.RequestsPerSecond, rateLimitConfig.BurstSize)

	// 测试限流器
	fmt.Printf("   Testing rate limiter (first 3 should be fast):\n")
	for i := 0; i < 5; i++ {
		start := time.Now()
		err := rateLimiter.Wait()
		elapsed := time.Since(start)

		if err != nil {
			fmt.Printf("   ❌ Request %d failed: %v\n", i+1, err)
		} else {
			fmt.Printf("   ✅ Request %d completed in %v\n", i+1, elapsed.Round(time.Millisecond))
		}
	}
	fmt.Println()

	// 演示消息优先级
	fmt.Println("📨 Message Priority Levels:")
	priorities := []telegram.MessagePriority{
		telegram.PriorityLow,
		telegram.PriorityNormal,
		telegram.PriorityHigh,
		telegram.PriorityUrgent,
	}

	for _, priority := range priorities {
		fmt.Printf("   %s: %d\n", getPriorityName(priority), int(priority))
	}
	fmt.Println()

	// 演示统计功能
	fmt.Println("📊 Command Statistics:")
	stats := commandHandler.GetCommandStats()
	for cmd, stat := range stats {
		fmt.Printf("   %s: %d calls, %d success, %d failed\n",
			cmd, stat.TotalCalls, stat.SuccessCalls, stat.FailedCalls)
	}
	fmt.Println()

	// 演示配置验证
	fmt.Println("🔍 Configuration Validation:")

	// 测试有效配置
	fmt.Printf("   ✅ Valid config validation: ")
	if validateConfig(config) {
		fmt.Println("PASSED")
	} else {
		fmt.Println("FAILED")
	}

	// 测试无效配置
	invalidConfig := &telegram.BotConfig{Token: ""}
	fmt.Printf("   ❌ Invalid config validation: ")
	if !validateConfig(invalidConfig) {
		fmt.Println("PASSED (correctly rejected)")
	} else {
		fmt.Println("FAILED (should have been rejected)")
	}
	fmt.Println()

	// 演示错误分类
	fmt.Println("🚨 Error Classification Demo:")
	demoErrors := []error{
		fmt.Errorf("network timeout"),
		fmt.Errorf("unauthorized: invalid token"),
		fmt.Errorf("rate limit exceeded"),
		fmt.Errorf("bad request: invalid chat id"),
	}

	for _, err := range demoErrors {
		errType := classifyError(err)
		fmt.Printf("   Error: '%s' -> Type: %s\n", err.Error(), errType)
	}
	fmt.Println()

	fmt.Println("🎉 Demo completed successfully!")
	fmt.Println("💡 To run with real Telegram API:")
	fmt.Println("   1. Set TELEGRAM_BOT_TOKEN environment variable")
	fmt.Println("   2. Ensure network connectivity to api.telegram.org")
	fmt.Println("   3. Run: go run ./examples/simple_telegram_bot.go")
	fmt.Println()

	// 可选：等待中断信号以保持程序运行
	if len(os.Args) > 1 && os.Args[1] == "--wait" {
		fmt.Println("Press Ctrl+C to exit...")
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		fmt.Println("\nGoodbye! 👋")
	}
}

// 辅助函数

func getCommandNames(commands map[string]string) []string {
	names := make([]string, 0, len(commands))
	for name := range commands {
		names = append(names, name)
	}
	return names
}

func getPriorityName(priority telegram.MessagePriority) string {
	switch priority {
	case telegram.PriorityLow:
		return "Low"
	case telegram.PriorityNormal:
		return "Normal"
	case telegram.PriorityHigh:
		return "High"
	case telegram.PriorityUrgent:
		return "Urgent"
	default:
		return "Unknown"
	}
}

func validateConfig(config *telegram.BotConfig) bool {
	if config == nil {
		return false
	}
	if config.Token == "" {
		return false
	}
	return true
}

func classifyError(err error) string {
	errMsg := err.Error()

	switch {
	case contains(errMsg, "timeout"), contains(errMsg, "network"):
		return "Temporary/Network"
	case contains(errMsg, "unauthorized"), contains(errMsg, "invalid token"):
		return "Permanent/Auth"
	case contains(errMsg, "rate limit"):
		return "Temporary/RateLimit"
	case contains(errMsg, "bad request"):
		return "Permanent/BadRequest"
	default:
		return "Unknown"
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		(len(s) > len(substr) &&
			(s[:len(substr)] == substr ||
				s[len(s)-len(substr):] == substr ||
				containsInMiddle(s, substr))))
}

func containsInMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
