package ethereum

import (
	"context"
	"fmt"
	"math/big"
	"simplied-evm-monitoring-go/pkg/logger"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/sirupsen/logrus"
)

// TransactionService 交易数据服务
type TransactionService struct {
	client *Client
}

// TransactionWithReceipt 包含收据的交易
type TransactionWithReceipt struct {
	// 交易
	Transaction *types.Transaction `json:"transaction"`
	// 收据
	Receipt *types.Receipt `json:"receipt"`
	// 区块
	Block *types.Block `json:"block,omitempty"`
	// 是否是未确认的交易
	IsPending bool `json:"is_pending"`
}

// TransactionBatch 交易批次
type TransactionBatch struct {
	// 交易列表
	Transactions []*TransactionWithReceipt `json:"transactions"`
	// 交易哈希列表
	Hashes []common.Hash `json:"hashes"`
	// 错误信息
	Error error `json:"error,omitempty"`
}

// TransactionFilter 交易过滤器
type TransactionFilter struct {
	// 发送地址
	FromAddress *common.Address `json:"from_address,omitempty"`
	// 接收地址
	ToAddress *common.Address `json:"to_address,omitempty"`
	// 最小值
	MinValue *big.Int `json:"min_value,omitempty"`
	// 最大值
	MaxValue *big.Int `json:"max_value,omitempty"`
	// 最小Gas价格
	MinGasPrice *big.Int `json:"min_gas_price,omitempty"`
	// 最大Gas价格
	MaxGasPrice *big.Int `json:"max_gas_price,omitempty"`
	// 是否为合约交易
	ContractOnly bool `json:"contract_only"`
	// 是否成功交易
	SuccessOnly bool `json:"success_only"`
	// 是否失败交易
	FailedOnly bool `json:"failed_only"`
}

// TransactionSyncOptions 交易同步选项
type TransactionSyncOptions struct {
	// 批次大小
	BatchSize int `json:"batch_size"`
	// 最大并发数
	MaxConcurrency int `json:"max_concurrency"`
	// 重试次数
	RetryAttempts int `json:"retry_attempts"`
	// 重试延迟
	RetryDelay time.Duration `json:"retry_delay"`
	// 是否包含收据
	IncludeReceipts bool `json:"include_receipts"`
	// 是否包含区块
	IncludeBlocks bool `json:"include_blocks"`
	// 过滤器
	Filter *TransactionFilter `json:"filter,omitempty"`
	// 是否验证完整性
	VerifyIntegrity bool `json:"verify_integrity"`
}

// NewTransactionService 创建新的交易数据服务
func NewTransactionService(client *Client) *TransactionService {
	return &TransactionService{
		client: client,
	}
}

// GetTransactionByHash 根据交易哈希获取交易
func (ts *TransactionService) GetTransactionByHash(ctx context.Context, hash common.Hash) (*TransactionWithReceipt, error) {
	tx, isPending, err := ts.client.GetTransactionByHash(ctx, hash)
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction: %w", err)
	}

	result := &TransactionWithReceipt{
		Transaction: tx,
		IsPending:   isPending,
	}

	// 如果不是pending交易，获取收据
	if !isPending {
		receipt, err := ts.client.GetTransactionReceipt(ctx, hash)
		if err != nil {
			logger.WithFields(logrus.Fields{
				"hash":  hash.Hex(),
				"error": err,
			}).Warn("Failed to get transaction receipt")
		} else {
			result.Receipt = receipt
		}
	}

	return result, nil
}

// GetTransactionsByHashes 批量获取交易
func (ts *TransactionService) GetTransactionsByHashes(ctx context.Context, hashes []common.Hash, options *TransactionSyncOptions) ([]*TransactionWithReceipt, error) {
	if len(hashes) == 0 {
		return nil, nil
	}

	if options == nil {
		options = &TransactionSyncOptions{
			BatchSize:       20,
			MaxConcurrency:  10,
			RetryAttempts:   3,
			RetryDelay:      time.Second,
			IncludeReceipts: true,
		}
	}

	logger.WithFields(logrus.Fields{
		"total_hashes":     len(hashes),
		"batch_size":       options.BatchSize,
		"max_concurrency":  options.MaxConcurrency,
		"include_receipts": options.IncludeReceipts,
	}).Info("Starting batch transaction retrieval")

	// 创建批次
	batches := ts.createTransactionBatches(hashes, options.BatchSize)

	// 并发获取批次
	transactions, err := ts.fetchTransactionBatchesConcurrently(ctx, batches, options)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch transaction batches: %w", err)
	}

	// 应用过滤器
	if options.Filter != nil {
		transactions = ts.applyTransactionFilter(transactions, options.Filter)
	}

	logger.WithFields(logrus.Fields{
		"retrieved_transactions": len(transactions),
		"expected_transactions":  len(hashes),
	}).Info("Batch transaction retrieval completed")

	return transactions, nil
}

// createTransactionBatches 创建交易批次
func (ts *TransactionService) createTransactionBatches(hashes []common.Hash, batchSize int) [][]common.Hash {
	var batches [][]common.Hash

	for i := 0; i < len(hashes); i += batchSize {
		end := i + batchSize
		if end > len(hashes) {
			end = len(hashes)
		}
		batches = append(batches, hashes[i:end])
	}

	return batches
}

// fetchTransactionBatchesConcurrently 并发获取交易批次
func (ts *TransactionService) fetchTransactionBatchesConcurrently(ctx context.Context, batches [][]common.Hash, options *TransactionSyncOptions) ([]*TransactionWithReceipt, error) {
	semaphore := make(chan struct{}, options.MaxConcurrency)
	results := make(chan *TransactionBatch, len(batches))
	var wg sync.WaitGroup

	// 启动工作协程
	for _, batch := range batches {
		wg.Add(1)
		go func(hashes []common.Hash) {
			defer wg.Done()

			// 获取信号量
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			result := ts.fetchTransactionBatch(ctx, hashes, options)
			results <- result
		}(batch)
	}

	// 等待所有批次完成
	go func() {
		wg.Wait()
		close(results)
	}()

	// 收集结果
	var allTransactions []*TransactionWithReceipt
	var errors []error

	for result := range results {
		if result.Error != nil {
			errors = append(errors, result.Error)
			continue
		}

		allTransactions = append(allTransactions, result.Transactions...)
	}

	if len(errors) > 0 {
		return nil, fmt.Errorf("batch fetch errors: %v", errors)
	}

	return allTransactions, nil
}

// fetchTransactionBatch 获取单个交易批次
func (ts *TransactionService) fetchTransactionBatch(ctx context.Context, hashes []common.Hash, options *TransactionSyncOptions) *TransactionBatch {
	result := &TransactionBatch{
		Hashes: hashes,
	}

	var transactions []*TransactionWithReceipt

	for _, hash := range hashes {
		var txWithReceipt *TransactionWithReceipt
		var err error

		// 重试机制
		for attempt := 0; attempt <= options.RetryAttempts; attempt++ {
			txWithReceipt, err = ts.fetchSingleTransaction(ctx, hash, options)
			if err == nil {
				break
			}

			if attempt < options.RetryAttempts {
				logger.WithFields(logrus.Fields{
					"hash":    hash.Hex(),
					"attempt": attempt + 1,
					"error":   err,
				}).Warn("Failed to fetch transaction, retrying")

				select {
				case <-ctx.Done():
					result.Error = ctx.Err()
					return result
				case <-time.After(options.RetryDelay * time.Duration(attempt+1)):
				}
			}
		}

		if err != nil {
			logger.WithFields(logrus.Fields{
				"hash":  hash.Hex(),
				"error": err,
			}).Error("Failed to fetch transaction after all retries")
			continue
		}

		transactions = append(transactions, txWithReceipt)
	}

	result.Transactions = transactions
	return result
}

// fetchSingleTransaction 获取单个交易
func (ts *TransactionService) fetchSingleTransaction(ctx context.Context, hash common.Hash, options *TransactionSyncOptions) (*TransactionWithReceipt, error) {
	tx, isPending, err := ts.client.GetTransactionByHash(ctx, hash)
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction: %w", err)
	}

	result := &TransactionWithReceipt{
		Transaction: tx,
		IsPending:   isPending,
	}

	// 获取收据
	if options.IncludeReceipts && !isPending {
		receipt, err := ts.client.GetTransactionReceipt(ctx, hash)
		if err != nil {
			return nil, fmt.Errorf("failed to get transaction receipt: %w", err)
		}
		result.Receipt = receipt
	}

	// 获取区块信息
	if options.IncludeBlocks && !isPending && result.Receipt != nil {
		block, err := ts.client.GetBlockByNumber(ctx, result.Receipt.BlockNumber)
		if err != nil {
			logger.WithFields(logrus.Fields{
				"hash":         hash.Hex(),
				"block_number": result.Receipt.BlockNumber,
				"error":        err,
			}).Warn("Failed to get block for transaction")
		} else {
			result.Block = block
		}
	}

	return result, nil
}

// GetTransactionsFromBlock 从区块中获取所有交易
func (ts *TransactionService) GetTransactionsFromBlock(ctx context.Context, block *types.Block, options *TransactionSyncOptions) ([]*TransactionWithReceipt, error) {
	if block == nil {
		return nil, fmt.Errorf("block cannot be nil")
	}

	transactions := block.Transactions()
	if len(transactions) == 0 {
		return nil, nil
	}

	if options == nil {
		options = &TransactionSyncOptions{
			IncludeReceipts: true,
			MaxConcurrency:  10,
		}
	}

	logger.WithFields(logrus.Fields{
		"block_number":      block.NumberU64(),
		"block_hash":        block.Hash().Hex(),
		"transaction_count": len(transactions),
	}).Info("Extracting transactions from block")

	var results []*TransactionWithReceipt
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, options.MaxConcurrency)
	resultsCh := make(chan *TransactionWithReceipt, len(transactions))

	// 并发处理交易
	for _, tx := range transactions {
		wg.Add(1)
		go func(transaction *types.Transaction) {
			defer wg.Done()

			// 获取信号量
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			result := &TransactionWithReceipt{
				Transaction: transaction,
				Block:       block,
				IsPending:   false,
			}

			// 获取收据
			if options.IncludeReceipts {
				receipt, err := ts.client.GetTransactionReceipt(ctx, transaction.Hash())
				if err != nil {
					logger.WithFields(logrus.Fields{
						"tx_hash": transaction.Hash().Hex(),
						"error":   err,
					}).Warn("Failed to get transaction receipt")
				} else {
					result.Receipt = receipt
				}
			}

			resultsCh <- result
		}(tx)
	}

	// 等待所有交易处理完成
	go func() {
		wg.Wait()
		close(resultsCh)
	}()

	// 收集结果
	for result := range resultsCh {
		results = append(results, result)
	}

	// 应用过滤器
	if options.Filter != nil {
		results = ts.applyTransactionFilter(results, options.Filter)
	}

	return results, nil
}

// applyTransactionFilter 应用交易过滤器
func (ts *TransactionService) applyTransactionFilter(transactions []*TransactionWithReceipt, filter *TransactionFilter) []*TransactionWithReceipt {
	var filtered []*TransactionWithReceipt

	for _, tx := range transactions {
		if ts.matchesFilter(tx, filter) {
			filtered = append(filtered, tx)
		}
	}

	return filtered
}

// matchesFilter 检查交易是否匹配过滤器
func (ts *TransactionService) matchesFilter(tx *TransactionWithReceipt, filter *TransactionFilter) bool {
	transaction := tx.Transaction
	receipt := tx.Receipt

	// 检查发送地址
	if filter.FromAddress != nil {
		from, err := types.Sender(types.LatestSignerForChainID(transaction.ChainId()), transaction)
		if err != nil || from != *filter.FromAddress {
			return false
		}
	}

	// 检查接收地址
	if filter.ToAddress != nil {
		if transaction.To() == nil || *transaction.To() != *filter.ToAddress {
			return false
		}
	}

	// 检查交易金额
	if filter.MinValue != nil && transaction.Value().Cmp(filter.MinValue) < 0 {
		return false
	}
	if filter.MaxValue != nil && transaction.Value().Cmp(filter.MaxValue) > 0 {
		return false
	}

	// 检查Gas价格
	if filter.MinGasPrice != nil && transaction.GasPrice().Cmp(filter.MinGasPrice) < 0 {
		return false
	}
	if filter.MaxGasPrice != nil && transaction.GasPrice().Cmp(filter.MaxGasPrice) > 0 {
		return false
	}

	// 检查是否为合约交易
	if filter.ContractOnly && transaction.To() != nil {
		return false
	}

	// 检查交易状态
	if receipt != nil {
		if filter.SuccessOnly && receipt.Status != types.ReceiptStatusSuccessful {
			return false
		}
		if filter.FailedOnly && receipt.Status == types.ReceiptStatusSuccessful {
			return false
		}
	}

	return true
}

// AnalyzeTransactionGas 分析交易Gas使用情况
func (ts *TransactionService) AnalyzeTransactionGas(tx *TransactionWithReceipt) map[string]interface{} {
	if tx.Transaction == nil {
		return nil
	}

	analysis := make(map[string]interface{})

	// 基本Gas信息
	analysis["gas_limit"] = tx.Transaction.Gas()
	analysis["gas_price"] = tx.Transaction.GasPrice()
	analysis["gas_fee_cap"] = tx.Transaction.GasFeeCap()
	analysis["gas_tip_cap"] = tx.Transaction.GasTipCap()

	// 如果有收据，计算实际使用的Gas
	if tx.Receipt != nil {
		gasUsed := tx.Receipt.GasUsed
		gasLimit := tx.Transaction.Gas()

		analysis["gas_used"] = gasUsed
		analysis["gas_efficiency"] = float64(gasUsed) / float64(gasLimit)
		analysis["gas_saved"] = gasLimit - gasUsed

		// 计算实际费用
		actualFee := new(big.Int).Mul(tx.Transaction.GasPrice(), big.NewInt(int64(gasUsed)))
		analysis["actual_fee"] = actualFee

		// 计算最大可能费用
		maxFee := new(big.Int).Mul(tx.Transaction.GasPrice(), big.NewInt(int64(gasLimit)))
		analysis["max_fee"] = maxFee
		analysis["fee_saved"] = new(big.Int).Sub(maxFee, actualFee)
	}

	return analysis
}

// IsHighValueTransaction 检查是否为高价值交易
func (ts *TransactionService) IsHighValueTransaction(tx *TransactionWithReceipt, threshold *big.Int) bool {
	if tx.Transaction == nil || threshold == nil {
		return false
	}

	return tx.Transaction.Value().Cmp(threshold) >= 0
}

// IsContractCreation 检查是否为合约创建交易
func (ts *TransactionService) IsContractCreation(tx *TransactionWithReceipt) bool {
	if tx.Transaction == nil {
		return false
	}

	return tx.Transaction.To() == nil
}
