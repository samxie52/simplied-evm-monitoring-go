package main

import (
	"fmt"
	"simplied-evm-monitoring-go/internal/config"
	"simplied-evm-monitoring-go/pkg/logger"
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

	logger.WithFields(map[string]interface{}{
		"app_name":  cfg.App.Name,
		"version":   cfg.App.Version,
		"env":       cfg.App.Environment,
		"timestamp": time.Now().Format("2006-01-02 15:04:05"),
	}).Info("Start Simplified EVM Monitoring...")
}
