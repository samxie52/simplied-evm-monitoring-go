package ethereum

import (
	"context"
	"fmt"
	"simplied-evm-monitoring-go/internal/config"
	"simplied-evm-monitoring-go/pkg/logger"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/sirupsen/logrus"
)

// Client 以太坊客户端封装
type Client struct {
	// config: 客户端配置
	config *config.EthereumConfig
	// ethClient: 以太坊客户端
	ethClient *ethclient.Client
	// rpcClient: RPC客户端
	rpcClient *rpc.Client
	// mu: 读写锁
	mu sync.RWMutex
	// isHealthy: 是否健康
	isHealthy bool
	// lastError: 最后一次错误
	lastError error
	// lastCheck: 最后一次检查时间
	lastCheck time.Time
	// connectedAt: 连接时间
	connectedAt time.Time
	// requestCount: 请求次数
	requestCount int64
	// errorCount: 错误次数
	errorCount int64
}

// ClientStats 客户端统计信息
type ClientStats struct {
	// URL: 客户端URL
	URL string `json:"url"`
	// Type: 客户端类型
	Type config.EthereumClientType `json:"type"`
	// IsHealthy: 是否健康
	IsHealthy bool `json:"is_healthy"`
	// LastError: 最后一次错误
	LastError string `json:"last_error,omitempty"`
	// LastCheck: 最后一次检查时间
	LastCheck time.Time `json:"last_check"`
	// ConnectedAt: 连接时间
	ConnectedAt time.Time `json:"connected_at"`
	// RequestCount: 请求次数
	RequestCount int64 `json:"request_count"`
	// ErrorCount: 错误次数
	ErrorCount int64 `json:"error_count"`
	// Uptime: 运行时间
	Uptime time.Duration `json:"uptime"`
	// ErrorRate: 错误率
	ErrorRate float64 `json:"error_rate"`
}

// NewClient 创建新的以太坊客户端
func NewClient(config *config.EthereumConfig) (*Client, error) {
	if config == nil {
		logger.Error("client config cannot be nil")
		return nil, fmt.Errorf("client config cannot be nil")
	}

	// 检测客户端类型
	if config.ClientType == "" {
		config.ClientType = detectClientType(config.URL)
	}

	client := &Client{
		config: config,
	}

	// 建立连接
	if err := client.Connect(); err != nil {
		return nil, fmt.Errorf("failed to connect to ethereum node: %w", err)
	}

	return client, nil
}

// Connect 建立与以太坊节点的连接
func (c *Client) Connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 设置超时
	ctx, cancel := context.WithTimeout(context.Background(), (time.Second * time.Duration(c.config.Timeout)))
	defer cancel()

	var err error

	// 创建RPC客户端
	// rpc.DialContext(ctx, c.config.URL) 创建一个RPC客户端
	c.rpcClient, err = rpc.DialContext(ctx, c.config.URL)
	if err != nil {
		c.lastError = err
		c.isHealthy = false
		return fmt.Errorf("failed to dial RPC: %w", err)
	}

	// 创建以太坊客户端
	c.ethClient = ethclient.NewClient(c.rpcClient)

	// 验证连接
	if err := c.validateConnection(ctx); err != nil {
		c.Close()
		c.lastError = err
		c.isHealthy = false
		return fmt.Errorf("connection validation failed: %w", err)
	}

	c.connectedAt = time.Now()
	c.isHealthy = true
	c.lastError = nil
	c.lastCheck = time.Now()

	logger.WithFields(logrus.Fields{
		"url":       c.config.URL,
		"type":      c.config.ClientType,
		"chain_id":  c.config.ChainID,
		"timestamp": time.Now().Format("2006-01-02 15:04:05"),
	}).Info("Successfully connected to Ethereum node")

	return nil
}

// Close 关闭客户端连接
func (c *Client) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.rpcClient != nil {
		c.rpcClient.Close()
		c.rpcClient = nil
	}

	c.ethClient = nil
	c.isHealthy = false

	logger.WithFields(map[string]interface{}{
		"url": c.config.URL,
	}).Info("Ethereum client connection closed")
}

// validateConnection 验证连接有效性
func (c *Client) validateConnection(ctx context.Context) error {
	// 获取网络ID
	networkID, err := c.ethClient.NetworkID(ctx)
	if err != nil {
		return fmt.Errorf("failed to get network ID: %w", err)
	}

	// 验证ChainID是否匹配
	if c.config.ChainID > 0 && networkID.Int64() != c.config.ChainID {
		return fmt.Errorf("chain ID mismatch: expected %d, got %d",
			c.config.ChainID, networkID.Int64())
	}

	// 获取最新区块号验证节点同步状态
	_, err = c.ethClient.BlockNumber(ctx)
	if err != nil {
		return fmt.Errorf("failed to get latest block number: %w", err)
	}

	return nil
}

// detectClientType 检测客户端类型
func detectClientType(url string) config.EthereumClientType {
	if strings.HasPrefix(url, "ws://") || strings.HasPrefix(url, "wss://") {
		return config.EthereumClientTypeWebSocket
	}
	if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") {
		return config.EthereumClientTypeHTTP
	}
	if strings.Contains(url, ".ipc") || strings.HasPrefix(url, "/") {
		return config.EthereumClientTypeIPC
	}
	return config.EthereumClientTypeHTTP
}
