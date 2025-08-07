package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"simplied-evm-monitoring-go/internal/handlers"
	"simplied-evm-monitoring-go/internal/services/alert"
	"simplied-evm-monitoring-go/internal/services/ethereum"
)

func main() {
	log.Println("Starting Ethereum Alert API Server...")

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

	// 暂时使用 nil ethereum manager 以简化启动
	var ethereumManager *ethereum.Manager = nil

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
	if err := server.Stop(); err != nil {
		log.Printf("Error during server shutdown: %v", err)
	}
	log.Println("Server stopped")
}
