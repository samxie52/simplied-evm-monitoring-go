package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"simplied-evm-monitoring-go/internal/config"
	"simplied-evm-monitoring-go/internal/handlers"
	"simplied-evm-monitoring-go/internal/services/alert"
	"simplied-evm-monitoring-go/internal/services/ethereum"
	"simplied-evm-monitoring-go/internal/services/telegram"
	"simplied-evm-monitoring-go/pkg/logger"
)

func main() {
	log.Println("Starting Ethereum Alert API Server...")

	// 加载配置
	envLoader := config.NewEnvLoader(".env")
	var cfg config.Config
	
	if err := envLoader.Load(&cfg); err != nil {
		fmt.Printf("Warning: Error loading environment variables: %v\n", err)
		log.Println("Using default configuration...")
		
		// 使用默认配置
		cfg = config.Config{
			App: config.AppConfig{
				Name:        "simplied-evm-monitoring-go",
				Version:     "1.0.0",
				Environment: "development",
				Port:        8080,
				Host:        "localhost",
				Debug:       true,
			},
			Ethereum: config.EthereumConfig{
				URL:            "https://cloudflare-eth.com",
				ClientType:     config.EthereumClientTypeHTTP,
				Network:        "mainnet",
				ChainID:        1,
				Timeout:        10,
				MaxConcurrency: 3,
				RetryAttempts:  2,
				RetryDelay:     2,
				Priority:       1,
			},
			Logging: config.LoggingConfig{
				Level:      "info",
				Format:     "json",
				Output:     "stdout",
				MaxSize:    100,
				MaxBackups: 3,
				MaxAge:     7,
			},
		}
	}

	fmt.Printf("Config loaded: %s v%s (%s)\n", cfg.App.Name, cfg.App.Version, cfg.App.Environment)

	// 初始化日志
	if err := logger.InitLogger(cfg.Logging); err != nil {
		log.Printf("Warning: Error initializing logger: %v", err)
		log.Println("Using default logging...")
	}

	// 创建服务管理器
	alertConfig := &alert.AlertManagerConfig{
		LargeTransactionConfig: &alert.DetectorConfig{
			ETHThreshold:       10.0,
			EnableETHThreshold: true,
			BatchSize:          50,
			MaxConcurrency:     5,
		},
		RuleEngineConfig: &alert.RuleEngineConfig{
			EvaluationInterval:          30 * time.Second,
			MaxConcurrentRules:          10,
			RuleCacheSize:               1000,
			EnablePerformanceMonitoring: true,
			EnablePriorityProcessing:    true,
			CooldownCheckInterval:       5 * time.Second,
		},
		AlertQueueSize: 1000,
		WorkerCount:    5,
		ProcessTimeout: 30 * time.Second,
		BatchSize:      10,
		BatchInterval:  5 * time.Second,
	}

	// 暂时使用 nil 作为 alertStore 和 notificationManager
	alertManager := alert.NewAlertManager(alertConfig, nil, nil)
	
	// 启动告警管理器
	log.Println("Starting Alert manager...")
	if err := alertManager.Start(); err != nil {
		log.Printf("Warning: Failed to start alert manager: %v", err)
		log.Println("API server will continue with limited alert functionality")
	} else {
		log.Println("Alert manager started successfully")
	}

	// 创建 Telegram Bot (如果配置了)
	var telegramBot *telegram.TelegramBot
	var alertPipeline *alert.AlertPipeline
	
	telegramToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	if telegramToken != "" {
		log.Println("Creating Telegram bot...")
		botConfig := &telegram.BotConfig{
			Token:                 telegramToken,
			UpdateTimeout:         60,
			MessageQueueSize:      1000,
			MaxConcurrentMessages: 10,
			AllowedUsers:          []int64{}, // 配置允许的用户ID
			AdminUsers:            []int64{}, // 配置管理员用户ID
		}
		
		var err error
		telegramBot, err = telegram.NewTelegramBot(botConfig)
		if err != nil {
			log.Printf("Warning: Failed to create telegram bot: %v", err)
		} else {
			log.Println("Telegram bot created successfully")
			
			// 启动 Telegram Bot
			if err := telegramBot.Start(); err != nil {
				log.Printf("Warning: Failed to start telegram bot: %v", err)
				telegramBot = nil
			} else {
				log.Println("Telegram bot started successfully")
				
				// 创建告警流水线
				log.Println("Creating alert pipeline...")
				pipelineConfig := alert.DefaultPipelineConfig()
				alertPipeline = alert.NewAlertPipeline(pipelineConfig, alertManager, telegramBot)
				
				// 启动告警流水线
				if err := alertPipeline.Start(); err != nil {
					log.Printf("Warning: Failed to start alert pipeline: %v", err)
					alertPipeline = nil
				} else {
					log.Println("Alert pipeline started successfully")
				}
			}
		}
	} else {
		log.Println("TELEGRAM_BOT_TOKEN not set, skipping Telegram integration")
	}

	// 创建以太坊管理器
	log.Println("Creating Ethereum manager...")
	ethereumManager, err := ethereum.NewManager(&cfg.Ethereum)
	if err != nil {
		log.Printf("Warning: Failed to create ethereum manager: %v", err)
		log.Println("API server will start without ethereum monitoring")
		ethereumManager = nil
	} else {
		log.Println("Ethereum manager created successfully")
		
		// 启动以太坊管理器
		if err := ethereumManager.Start(); err != nil {
			log.Printf("Warning: Failed to start ethereum manager: %v", err)
			log.Println("Continuing without ethereum monitoring")
			ethereumManager = nil
		} else {
			log.Println("Ethereum manager started successfully")
		}
	}

	// 创建服务器配置
	serverConfig := &handlers.ServerConfig{
		Port:            8080,
		ReadTimeout:     30 * time.Second,
		WriteTimeout:    30 * time.Second,
		ShutdownTimeout: 10 * time.Second,
		EnableCORS:      true,
		EnableAuth:      false, // 开发环境下禁用认证
	}

	// 创建并启动服务器
	server := handlers.NewServer(serverConfig, alertManager, ethereumManager)

	// 启动服务器
	log.Printf("Starting API Server on port %d", serverConfig.Port)
	go func() {
		if err := server.Start(); err != nil {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// 等待一点时间让服务器启动
	time.Sleep(2 * time.Second)
	log.Println("Server started successfully")

	// 等待中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
	
	// 停止告警流水线
	if alertPipeline != nil {
		log.Println("Stopping alert pipeline...")
		if err := alertPipeline.Stop(); err != nil {
			log.Printf("Error stopping alert pipeline: %v", err)
		} else {
			log.Println("Alert pipeline stopped")
		}
	}
	
	// 停止 Telegram Bot
	if telegramBot != nil {
		log.Println("Stopping telegram bot...")
		if err := telegramBot.Stop(); err != nil {
			log.Printf("Error stopping telegram bot: %v", err)
		} else {
			log.Println("Telegram bot stopped")
		}
	}
	
	// 停止服务器
	if err := server.Stop(); err != nil {
		log.Printf("Error during server shutdown: %v", err)
	}
	log.Println("Server stopped")
}
