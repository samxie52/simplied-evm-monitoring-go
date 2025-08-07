package ethereum

import (
	"context"
	"simplied-evm-monitoring-go/internal/config"
	"simplied-evm-monitoring-go/pkg/logger"
	"time"

	"github.com/ethereum/go-ethereum/common"
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
	return &Manager{
		client:             client,
		healthChecker:      healthChecker,
		blockService:       blockService,
		gasService:         gasService,
		transactionService: transactionService,
	}, nil
}

// Start 启动管理器
func (m *Manager) Start() {
	m.healthChecker.Start()
}

// Stop 停止管理器
func (m *Manager) Stop() {
	m.healthChecker.Stop()
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

		logger.WithFields(fields).Info("Transaction processed successfully")
	}

}
