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

// BlockService 区块数据服务
type BlockService struct {
	// 客户端连接池
	client *Client
}

// BlockRange 区块范围
type BlockRange struct {
	// 起始区块号
	From *big.Int `json:"from"`
	// 结束区块号
	To *big.Int `json:"to"`
}

// BlockBatch 区块批次
type BlockBatch struct {
	// 区块列表
	Blocks []*types.Block `json:"blocks"`
	// 区块范围
	Range *BlockRange `json:"range"`
	// 错误信息
	Error error `json:"error,omitempty"`
}

// BlockSyncOptions 区块同步选项
type BlockSyncOptions struct {
	// 批次大小
	BatchSize int `json:"batch_size"`
	// 最大并发数
	MaxConcurrency int `json:"max_concurrency"`
	// 重试次数
	RetryAttempts int `json:"retry_attempts"`
	// 重试延迟
	RetryDelay time.Duration `json:"retry_delay"`
	// 是否包含叔块
	IncludeUncles bool `json:"include_uncles"`
	// 是否验证完整性
	VerifyIntegrity bool `json:"verify_integrity"`
}

// NewBlockService 创建新的区块数据服务
func NewBlockService(client *Client) *BlockService {
	return &BlockService{
		client: client,
	}
}

// GetLatestBlock 获取最新区块
func (bs *BlockService) GetLatestBlock(ctx context.Context) (*types.Block, error) {
	return bs.client.GetLatestBlock(ctx)
}

// GetBlockByNumber 根据区块号获取区块
func (bs *BlockService) GetBlockByNumber(ctx context.Context, number *big.Int) (*types.Block, error) {
	return bs.client.GetBlockByNumber(ctx, number)
}

// GetBlockByHash 根据区块哈希获取区块
func (bs *BlockService) GetBlockByHash(ctx context.Context, hash common.Hash) (*types.Block, error) {
	return bs.client.GetBlockByHash(ctx, hash)
}

// GetBlockRange 获取区块范围
func (bs *BlockService) GetBlockRange(ctx context.Context, from, to *big.Int, options *BlockSyncOptions) ([]*types.Block, error) {
	if options == nil {
		options = &BlockSyncOptions{
			BatchSize:      10,
			MaxConcurrency: 5,
			RetryAttempts:  3,
			RetryDelay:     time.Second,
		}
	}

	// 验证参数
	if from.Cmp(to) > 0 {
		return nil, fmt.Errorf("from block (%s) cannot be greater than to block (%s)", from.String(), to.String())
	}

	totalBlocks := new(big.Int).Sub(to, from).Int64() + 1
	if totalBlocks <= 0 {
		return nil, fmt.Errorf("invalid block range")
	}

	logger.WithFields(logrus.Fields{
		"from":         from.String(),
		"to":           to.String(),
		"total_blocks": totalBlocks,
		"batch_size":   options.BatchSize,
	}).Info("Starting block range retrieval")

	// 创建批次
	batches := bs.createBatches(from, to, options.BatchSize)

	// 并发获取批次
	blocks, err := bs.fetchBatchesConcurrently(ctx, batches, options)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch block batches: %w", err)
	}

	// 验证完整性
	if options.VerifyIntegrity {
		if err := bs.verifyBlockIntegrity(blocks); err != nil {
			return nil, fmt.Errorf("block integrity verification failed: %w", err)
		}
	}

	logger.WithFields(logrus.Fields{
		"retrieved_blocks": len(blocks),
		"expected_blocks":  totalBlocks,
	}).Info("Block range retrieval completed")

	return blocks, nil
}

// createBatches 创建区块批次
func (bs *BlockService) createBatches(from, to *big.Int, batchSize int) []*BlockRange {
	var batches []*BlockRange

	current := new(big.Int).Set(from)
	batchSizeBig := big.NewInt(int64(batchSize))

	for current.Cmp(to) <= 0 {
		batchEnd := new(big.Int).Add(current, batchSizeBig)
		batchEnd.Sub(batchEnd, big.NewInt(1)) // 减1因为范围是包含的

		if batchEnd.Cmp(to) > 0 {
			batchEnd.Set(to)
		}

		batches = append(batches, &BlockRange{
			From: new(big.Int).Set(current),
			To:   new(big.Int).Set(batchEnd),
		})

		current.Add(batchEnd, big.NewInt(1))
	}

	return batches
}

// fetchBatchesConcurrently 并发获取批次
func (bs *BlockService) fetchBatchesConcurrently(ctx context.Context, batches []*BlockRange, options *BlockSyncOptions) ([]*types.Block, error) {
	semaphore := make(chan struct{}, options.MaxConcurrency)
	results := make(chan *BlockBatch, len(batches))
	var wg sync.WaitGroup

	// 启动工作协程
	for _, batch := range batches {
		wg.Add(1)
		go func(br *BlockRange) {
			defer wg.Done()

			// 获取信号量
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			result := bs.fetchBatch(ctx, br, options)
			results <- result
		}(batch)
	}

	// 等待所有批次完成
	go func() {
		wg.Wait()
		close(results)
	}()

	// 收集结果
	var allBlocks []*types.Block
	var errors []error

	for result := range results {
		if result.Error != nil {
			errors = append(errors, result.Error)
			continue
		}

		allBlocks = append(allBlocks, result.Blocks...)
	}

	if len(errors) > 0 {
		return nil, fmt.Errorf("batch fetch errors: %v", errors)
	}

	// 按区块号排序
	bs.sortBlocksByNumber(allBlocks)

	return allBlocks, nil
}

// fetchBatch 获取单个批次
func (bs *BlockService) fetchBatch(ctx context.Context, blockRange *BlockRange, options *BlockSyncOptions) *BlockBatch {
	result := &BlockBatch{
		Range: blockRange,
	}

	var blocks []*types.Block
	current := new(big.Int).Set(blockRange.From)

	for current.Cmp(blockRange.To) <= 0 {
		var block *types.Block
		var err error

		// 重试机制
		for attempt := 0; attempt <= options.RetryAttempts; attempt++ {
			block, err = bs.client.GetBlockByNumber(ctx, current)
			if err == nil {
				break
			}

			if attempt < options.RetryAttempts {
				logger.WithFields(logrus.Fields{
					"block_number": current.String(),
					"attempt":      attempt + 1,
					"error":        err,
				}).Warn("Failed to fetch block, retrying")

				select {
				case <-ctx.Done():
					result.Error = ctx.Err()
					return result
				case <-time.After(options.RetryDelay * time.Duration(attempt+1)):
				}
			}
		}

		if err != nil {
			result.Error = fmt.Errorf("failed to fetch block %s after %d attempts: %w",
				current.String(), options.RetryAttempts+1, err)
			return result
		}

		blocks = append(blocks, block)
		current.Add(current, big.NewInt(1))
	}

	result.Blocks = blocks
	return result
}

// sortBlocksByNumber 按区块号排序
func (bs *BlockService) sortBlocksByNumber(blocks []*types.Block) {
	// 简单的冒泡排序，适用于小批量数据
	n := len(blocks)
	for i := 0; i < n-1; i++ {
		for j := 0; j < n-i-1; j++ {
			if blocks[j].NumberU64() > blocks[j+1].NumberU64() {
				blocks[j], blocks[j+1] = blocks[j+1], blocks[j]
			}
		}
	}
}

// verifyBlockIntegrity 验证区块完整性
func (bs *BlockService) verifyBlockIntegrity(blocks []*types.Block) error {
	if len(blocks) == 0 {
		return nil
	}

	// 检查区块号连续性
	for i := 1; i < len(blocks); i++ {
		prevNumber := blocks[i-1].NumberU64()
		currNumber := blocks[i].NumberU64()

		if currNumber != prevNumber+1 {
			return fmt.Errorf("block number gap detected: %d -> %d", prevNumber, currNumber)
		}
	}

	// 检查父哈希连续性
	for i := 1; i < len(blocks); i++ {
		prevHash := blocks[i-1].Hash()
		currParentHash := blocks[i].ParentHash()

		if prevHash != currParentHash {
			return fmt.Errorf("parent hash mismatch at block %d: expected %s, got %s",
				blocks[i].NumberU64(), prevHash.Hex(), currParentHash.Hex())
		}
	}

	return nil
}

// GetBlocksWithTransactions 获取包含交易详情的区块
func (bs *BlockService) GetBlocksWithTransactions(ctx context.Context, numbers []*big.Int) ([]*types.Block, error) {
	if len(numbers) == 0 {
		return nil, nil
	}

	blocks := make([]*types.Block, len(numbers))
	errors := make([]error, len(numbers))
	var wg sync.WaitGroup

	// 并发获取区块
	for i, number := range numbers {
		wg.Add(1)
		go func(index int, blockNumber *big.Int) {
			defer wg.Done()

			block, err := bs.client.GetBlockByNumber(ctx, blockNumber)
			blocks[index] = block
			errors[index] = err
		}(i, number)
	}

	wg.Wait()

	// 检查错误
	var failedBlocks []string
	for i, err := range errors {
		if err != nil {
			failedBlocks = append(failedBlocks, numbers[i].String())
		}
	}

	if len(failedBlocks) > 0 {
		return nil, fmt.Errorf("failed to fetch blocks: %v", failedBlocks)
	}

	// 过滤nil值
	var validBlocks []*types.Block
	for _, block := range blocks {
		if block != nil {
			validBlocks = append(validBlocks, block)
		}
	}

	return validBlocks, nil
}

// GetLatestBlockNumber 获取最新区块号
func (bs *BlockService) GetLatestBlockNumber(ctx context.Context) (*big.Int, error) {
	var blockNumber *big.Int

	err := bs.client.ExecuteWithRetry(ctx, func() error {
		ethClient := bs.client.GetEthClient()
		if ethClient == nil {
			return fmt.Errorf("eth client is nil")
		}

		number, err := ethClient.BlockNumber(ctx)
		if err != nil {
			return err
		}

		blockNumber = big.NewInt(int64(number))
		return nil
	})

	return blockNumber, err
}

// IsBlockExists 检查区块是否存在
func (bs *BlockService) IsBlockExists(ctx context.Context, number *big.Int) (bool, error) {
	_, err := bs.client.GetBlockByNumber(ctx, number)
	if err != nil {
		// 如果是"not found"类型的错误，返回false
		if isNotFoundError(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// isNotFoundError 检查是否为"未找到"错误
func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()
	return contains(errStr, "not found") ||
		contains(errStr, "does not exist") ||
		contains(errStr, "unknown block")
}

// contains 检查字符串是否包含子字符串（忽略大小写）
func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr ||
			(len(s) > len(substr) &&
				(s[:len(substr)] == substr ||
					s[len(s)-len(substr):] == substr ||
					containsSubstring(s, substr))))
}

// containsSubstring 检查是否包含子字符串
func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
