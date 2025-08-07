package ethereum

import (
	"context"
	"fmt"
	"simplied-evm-monitoring-go/pkg/logger"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// HealthChecker 健康检查器
type HealthChecker struct {
	// 客户端
	client *Client
	// 健康检查间隔
	interval time.Duration
	// 停止通道
	stopCh chan struct{}
	// 等待组
	wg sync.WaitGroup
	// 读写锁
	mu sync.RWMutex
	// 是否正在运行
	running bool
}

// HealthCheckResult 健康检查结果
type HealthCheckResult struct {
	// 客户端URL
	ClientURL string `json:"client_url"`
	// 健康状态
	IsHealthy bool `json:"is_healthy"`
	// 错误信息
	Error string `json:"error,omitempty"`
	// 响应时间
	ResponseTime time.Duration `json:"response_time"`
	// 最新区块号
	BlockNumber uint64 `json:"block_number"`
	// 链ID
	ChainID string `json:"chain_id"`
	// 检查时间
	CheckTime time.Time `json:"check_time"`
}

// NewHealthChecker 创建新的健康检查器
func NewHealthChecker(client *Client, interval time.Duration) *HealthChecker {
	return &HealthChecker{
		client:   client,
		interval: interval,
		stopCh:   make(chan struct{}),
	}
}

// Start 启动健康检查
func (hc *HealthChecker) Start() {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	if hc.running {
		return
	}

	hc.running = true
	hc.wg.Add(1)

	go hc.run()

	logger.WithFields(logrus.Fields{
		"interval": hc.interval,
	}).Info("Health checker started")
}

// Stop 停止健康检查
func (hc *HealthChecker) Stop() {
	hc.mu.Lock()
	if !hc.running {
		hc.mu.Unlock()
		return
	}
	hc.running = false
	hc.mu.Unlock()

	close(hc.stopCh)
	hc.wg.Wait()

	logger.Info("Health checker stopped")
}

// run 运行健康检查循环
func (hc *HealthChecker) run() {
	defer hc.wg.Done()

	ticker := time.NewTicker(hc.interval)
	defer ticker.Stop()

	// 立即执行一次健康检查
	hc.checkClient(hc.client)

	for {
		select {
		case <-ticker.C:
			result := hc.checkClient(hc.client)
			logger.WithFields(logrus.Fields{
				"result":    result,
				"timestamp": time.Now().Format("2006-01-02 15:04:05"),
			}).Info("Health check result")
		case <-hc.stopCh:
			return
		}
	}
}

// checkClient 检查单个客户端的健康状态
func (hc *HealthChecker) checkClient(client *Client) *HealthCheckResult {
	result := &HealthCheckResult{
		ClientURL: client.config.URL,
		CheckTime: time.Now(),
	}

	startTime := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 检查连接状态
	if !client.IsHealthy() {
		result.Error = "Client marked as unhealthy"
		result.ResponseTime = time.Since(startTime)
		return result
	}

	// 尝试获取最新区块号
	block, err := client.GetLatestBlock(ctx)
	result.ResponseTime = time.Since(startTime)

	if err != nil {
		result.Error = err.Error()
		// 标记客户端为不健康
		client.mu.Lock()
		client.isHealthy = false
		client.lastError = err
		client.lastCheck = time.Now()
		client.mu.Unlock()
		return result
	}

	// 更新客户端健康状态
	client.mu.Lock()
	client.isHealthy = true
	client.lastError = nil
	client.lastCheck = time.Now()
	client.mu.Unlock()

	result.IsHealthy = true
	result.BlockNumber = block.NumberU64()
	if client.config.ChainID != 0 {
		result.ChainID = fmt.Sprintf("%d", client.config.ChainID)
	}

	return result
}
