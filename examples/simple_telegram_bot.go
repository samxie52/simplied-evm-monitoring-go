package main

import (
	"log"
	"os"
	"os/signal"
	"simplied-evm-monitoring-go/internal/services/telegram"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
)

func main() {
	// 设置日志级别
	logrus.SetLevel(logrus.InfoLevel)

	// 从环境变量获取 Telegram Bot Token
	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	if botToken == "" {
		log.Fatal("TELEGRAM_BOT_TOKEN environment variable is required")
	}

	// 创建 Telegram Bot 配置
	config := &telegram.BotConfig{
		Token:                 botToken,
		Debug:                 false,
		UpdateTimeout:         30,
		MessageQueueSize:      1000,
		MaxConcurrentMessages: 10,
		AllowedUsers:          []int64{}, // 在实际使用中设置允许的用户 ID
		AdminUsers:            []int64{}, // 在实际使用中设置管理员用户 ID
	}

	// 创建 Telegram Bot
	bot, err := telegram.NewTelegramBot(config)
	if err != nil {
		log.Fatalf("Failed to create Telegram bot: %v", err)
	}

	// 启动 Bot
	if err := bot.Start(); err != nil {
		log.Fatalf("Failed to start Telegram bot: %v", err)
	}
	defer bot.Stop()

	logrus.Info("Telegram Bot started successfully")
	logrus.Info("Bot is running and ready to receive messages")

	// 定期发送统计信息（演示用）
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				stats := bot.GetStats()
				logrus.WithFields(logrus.Fields{
					"total_messages":    stats["total_messages"],
					"sent_messages":     stats["sent_messages"],
					"received_messages": stats["received_messages"],
					"total_commands":    stats["total_commands"],
					"uptime":            stats["uptime"],
				}).Info("Bot statistics")
			}
		}
	}()

	// 等待中断信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	logrus.Info("Shutting down...")
}
