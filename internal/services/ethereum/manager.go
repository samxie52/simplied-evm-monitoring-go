package ethereum

import (
	"context"
	"simplied-evm-monitoring-go/internal/config"
	"simplied-evm-monitoring-go/internal/services/alert"
	"simplied-evm-monitoring-go/pkg/logger"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/sirupsen/logrus"
)

// Manager 管理器
type Manager struct {
	// 客户端
	client *Client
	// 健康检查器
	healthChecker *HealthChecker
	// 区块数据服务
	blockService *BlockService
	// Gas价格监控服务
	gasService *GasService
	// 交易数据服务
	transactionService *TransactionService
	// 告警管理器
	alertManager *alert.AlertManager
}

// NewManager 创建新的管理器
func NewManager(config *config.EthereumConfig) (*Manager, error) {
	client, err := NewClient(config)
	if err != nil {
		return nil, err
	}
	healthChecker := NewHealthChecker(client, 3*time.Second)
	blockService := NewBlockService(client)
	gasService := NewGasService(client)
	transactionService := NewTransactionService(client)

	// 创建告警管理器
	alertConfig := &alert.AlertManagerConfig{
		LargeTransactionConfig: &alert.DetectorConfig{
			ETHThreshold:       1.0,   // 100 ETH 阈值
			USDThreshold:       100.0, // $100,000 阈值
			DetectionInterval:  10 * time.Second,
			EnableETHThreshold: true,
			EnableUSDThreshold: true,
			BatchSize:          50,
			MaxConcurrency:     5,
		},
		AlertQueueSize: 1000,
		WorkerCount:    3,
		ProcessTimeout: 30 * time.Second,
	}

	// 暂时使用 nil 作为 alertStore 和 notificationManager
	// 在实际实现中，这些应该是真正的实现
	alertManager := alert.NewAlertManager(alertConfig, nil, nil)

	return &Manager{
		client:             client,
		healthChecker:      healthChecker,
		blockService:       blockService,
		gasService:         gasService,
		transactionService: transactionService,
		alertManager:       alertManager,
	}, nil
}

// Start 启动管理器
func (m *Manager) Start() error {
	m.healthChecker.Start()

	// 启动告警管理器
	if err := m.alertManager.Start(); err != nil {
		logger.WithFields(logrus.Fields{"error": err}).Error("Failed to start alert manager")
		return err
	}

	logger.Info("Ethereum manager started successfully")
	return nil
}

// Stop 停止管理器
func (m *Manager) Stop() error {
	m.healthChecker.Stop()

	// 停止告警管理器
	if err := m.alertManager.Stop(); err != nil {
		logger.WithFields(logrus.Fields{"error": err}).Error("Failed to stop alert manager")
		return err
	}

	logger.Info("Ethereum manager stopped successfully")
	return nil
}

// GetAllTransactionsFromLatestBlock 获取最新区块的交易
func (m *Manager) GetAllTransactionsFromLatestBlock() {
	// 增加超时时间以处理大量交易
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	latestBlock, err := m.blockService.GetLatestBlock(ctx)
	if err != nil {
		logger.Error("Failed to get latest block: ", err)

	}

	latestBlockNumber := latestBlock.Number()
	logger.Info("Latest block number: ", latestBlockNumber)
	block, err := m.blockService.GetBlockByNumber(ctx, latestBlockNumber)
	if err != nil {
		logger.Error("Failed to get block by number: ", err)
	}
	logger.Info("Block included transactions: ", len(block.Transactions()))

	hashes := make([]common.Hash, len(block.Transactions()))
	for i, tx := range block.Transactions() {
		hashes[i] = tx.Hash()
	}

	// 优化批处理参数以减少网络压力
	options := &TransactionSyncOptions{
		BatchSize:       5, // 减少批次大小
		MaxConcurrency:  3, // 减少并发数
		RetryAttempts:   3,
		RetryDelay:      5 * time.Second, // 增加重试延迟
		IncludeReceipts: true,
	}

	// 限制处理的交易数量以避免超时
	maxTransactions := 50
	if len(hashes) > maxTransactions {
		logger.WithFields(logrus.Fields{
			"total_transactions": len(hashes),
			"processing_limit":   maxTransactions,
		}).Info("Limiting transaction processing to avoid timeout")
		hashes = hashes[:maxTransactions]
	}

	result, err := m.transactionService.GetTransactionsByHashes(ctx, hashes, options)
	if err != nil {
		logger.Error("Failed to get transactions by hashes: ", err)
	}

	logger.Info("Get transactions by hashes: ", len(result))

	// 准备交易数据用于告警检测
	transactions := make([]*types.Transaction, len(result))
	receipts := make([]*types.Receipt, len(result))
	for i, tx := range result {
		transactions[i] = tx.Transaction
		if tx.Receipt != nil {
			receipts[i] = tx.Receipt
		}
	}

	// 执行告警检测
	if err := m.alertManager.ProcessTransactions(ctx, transactions, receipts); err != nil {
		logger.WithFields(logrus.Fields{"error": err}).Error("Failed to process transactions for alerts")
	}

	for _, tx := range result {
		// 准备日志字段
		fields := logrus.Fields{
			"hash":     tx.Transaction.Hash().Hex(),
			"value":    tx.Transaction.Value().String(),
			"gas":      tx.Transaction.Gas(),
			"gasPrice": tx.Transaction.GasPrice().String(),
			"nonce":    tx.Transaction.Nonce(),
		}

		// 安全地获取 To 地址（可能为 nil）
		if tx.Transaction.To() != nil {
			fields["to"] = tx.Transaction.To().Hex()
		} else {
			fields["to"] = "contract_creation"
		}

		// 从收据中获取区块信息
		if tx.Receipt != nil {
			fields["blockNumber"] = tx.Receipt.BlockNumber
			fields["blockHash"] = tx.Receipt.BlockHash.Hex()
			fields["gasUsed"] = tx.Receipt.GasUsed
			fields["status"] = tx.Receipt.Status
		}

		// 从区块中获取时间信息（如果有）
		if tx.Block != nil {
			fields["blockTime"] = tx.Block.Time()
		}

		// logger.WithFields(fields).Info("Transaction processed successfully")
	}

}

// IsRunning 检查管理器是否运行中
func (m *Manager) IsRunning() bool {
	return m.client != nil && m.healthChecker != nil && m.blockService != nil && m.gasService != nil && m.transactionService != nil && m.alertManager != nil
}
