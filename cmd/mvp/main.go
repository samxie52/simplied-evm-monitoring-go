package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"simplied-evm-monitoring-go/internal/config"
	"simplied-evm-monitoring-go/internal/services/ethereum"
	"simplied-evm-monitoring-go/pkg/logger"
	"syscall"
	"time"
)

func main() {

	envLoader := config.NewEnvLoader(".env")

	var cfg config.Config

	if err := envLoader.Load(&cfg); err != nil {
		fmt.Println("Error loading environment variables:", err)
		return
	}

	fmt.Println("Config:", cfg.App.Name)

	if err := logger.InitLogger(cfg.Logging); err != nil {
		fmt.Println("Error initializing logger:", err)
		return
	}

	// 等待服务优雅关闭
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	manager, err := ethereum.NewManager(&cfg.Ethereum)
	if err != nil {
		logger.Error("Failed to create Ethereum manager:", err)
		return
	}
	defer manager.Stop()
	manager.Start()
	// manager.GetAllTransactionsFromLatestBlock()
	logger.WithFields(map[string]interface{}{
		"app_name":  cfg.App.Name,
		"version":   cfg.App.Version,
		"env":       cfg.App.Environment,
		"timestamp": time.Now().Format("2006-01-02 15:04:05"),
	}).Info("Start Simplified EVM Monitoring...")

	// 监听系统信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 等待停止信号
	<-sigChan
	logger.Info("收到停止信号，正在关闭服务...")

	select {
	case <-ctx.Done():
		logger.Info("服务关闭")
	case <-time.After(1 * time.Second):
		logger.Info("服务关闭")
	}
}
