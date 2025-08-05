package ethereum

import (
	"fmt"
	"math/big"
	"simplied-evm-monitoring-go/pkg/logger"
	"sort"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/sirupsen/logrus"
)

// GasService Gas价格监控服务
type GasService struct {
	client *Client
}

// GasPriceInfo Gas价格信息
type GasPriceInfo struct {
	Standard  *big.Int  `json:"standard"` // 标准Gas价格
	Fast      *big.Int  `json:"fast"`     // 快速Gas价格
	Instant   *big.Int  `json:"instant"`  // 即时Gas价格
	BaseFee   *big.Int  `json:"base_fee"` // EIP-1559基础费用
	Priority  *big.Int  `json:"priority"` // 优先费用
	Timestamp time.Time `json:"timestamp"`
}

// GasPriceHistory Gas价格历史
type GasPriceHistory struct {
	// Gas价格列表
	Prices []*GasPriceInfo `json:"prices"`
	// 开始时间
	StartTime time.Time `json:"start_time"`
	// 结束时间
	EndTime time.Time `json:"end_time"`
	// 采样间隔
	Interval time.Duration `json:"interval"`
}

// GasPriceStats Gas价格统计
type GasPriceStats struct {
	// 最小Gas价格
	Min *big.Int `json:"min"`
	// 最大Gas价格
	Max *big.Int `json:"max"`
	// 平均Gas价格
	Average *big.Int `json:"average"`
	// 中位数Gas价格
	Median *big.Int `json:"median"`
	// 标准差
	StdDev float64 `json:"std_dev"`
	// 样本数量
	Samples int `json:"samples"`
	// 时间范围
	TimeRange time.Duration `json:"time_range"`
}

// GasEstimate Gas估算结果
type GasEstimate struct {
	// 交易Gas限制
	GasLimit uint64 `json:"gas_limit"`
	// Gas价格
	GasPrice *big.Int `json:"gas_price"`
	// 最大费用
	MaxFeePerGas *big.Int `json:"max_fee_per_gas"`
	// 最大优先费用
	MaxPriorityFeePerGas *big.Int `json:"max_priority_fee_per_gas"`
	// 估算费用
	EstimatedCost *big.Int `json:"estimated_cost"`
	// 置信度
	Confidence float64 `json:"confidence"`
}

// GasMonitorOptions Gas监控选项
type GasMonitorOptions struct {
	// 采样大小
	SampleSize int `json:"sample_size"`
	// 采样间隔
	SampleInterval time.Duration `json:"sample_interval"`
	// 历史持续时间
	HistoryDuration time.Duration `json:"history_duration"`
	// 百分位数
	Percentiles []float64 `json:"percentiles"`
}

// NewGasService 创建新的Gas价格服务
func NewGasService(client *Client) *GasService {
	return &GasService{
		client: client,
	}
}

// GetCurrentGasPrice 获取当前Gas价格
func (gs *GasService) GetCurrentGasPrice() (*big.Int, error) {
	return gs.client.GetGasPrice()
}

// GetGasPriceInfo 获取详细的Gas价格信息
func (gs *GasService) GetGasPriceInfo() (*GasPriceInfo, error) {
	// 获取最新区块
	latestBlock, err := gs.client.GetLatestBlock()
	if err != nil {
		return nil, fmt.Errorf("failed to get latest block: %w", err)
	}

	info := &GasPriceInfo{
		Timestamp: time.Now(),
	}

	// 获取基础Gas价格
	gasPrice, err := gs.client.GetGasPrice()
	if err != nil {
		return nil, fmt.Errorf("failed to get gas price: %w", err)
	}
	info.Standard = gasPrice

	// 如果是EIP-1559区块，获取基础费用
	if latestBlock.BaseFee() != nil {
		info.BaseFee = latestBlock.BaseFee()

		// 计算不同优先级的Gas价格
		info.Fast = new(big.Int).Add(info.BaseFee, new(big.Int).Mul(info.BaseFee, big.NewInt(2)))    // BaseFee + 200%
		info.Instant = new(big.Int).Add(info.BaseFee, new(big.Int).Mul(info.BaseFee, big.NewInt(5))) // BaseFee + 500%

		// 估算优先费用
		info.Priority = gs.estimatePriorityFee(latestBlock)
	} else {
		// 对于非EIP-1559区块，使用传统Gas价格计算
		info.Fast = new(big.Int).Mul(gasPrice, big.NewInt(12))
		info.Fast.Div(info.Fast, big.NewInt(10)) // gasPrice * 1.2

		info.Instant = new(big.Int).Mul(gasPrice, big.NewInt(15))
		info.Instant.Div(info.Instant, big.NewInt(10)) // gasPrice * 1.5
	}

	return info, nil
}

// estimatePriorityFee 估算优先费用
func (gs *GasService) estimatePriorityFee(block *types.Block) *big.Int {
	if block == nil || len(block.Transactions()) == 0 {
		return big.NewInt(1000000000) // 1 Gwei默认值
	}

	var priorityFees []*big.Int

	// 分析最近交易的优先费用
	for _, tx := range block.Transactions() {
		if tx.Type() == types.DynamicFeeTxType {
			priorityFees = append(priorityFees, tx.GasTipCap())
		}
	}

	if len(priorityFees) == 0 {
		return big.NewInt(1000000000) // 1 Gwei默认值
	}

	// 计算中位数优先费用
	sort.Slice(priorityFees, func(i, j int) bool {
		return priorityFees[i].Cmp(priorityFees[j]) < 0
	})

	medianIndex := len(priorityFees) / 2
	if len(priorityFees)%2 == 0 {
		// 偶数个元素，取中间两个的平均值
		sum := new(big.Int).Add(priorityFees[medianIndex-1], priorityFees[medianIndex])
		return sum.Div(sum, big.NewInt(2))
	}

	return priorityFees[medianIndex]
}

// MonitorGasPrice 监控Gas价格
func (gs *GasService) MonitorGasPrice(options *GasMonitorOptions) {
	if options == nil {
		options = &GasMonitorOptions{
			SampleInterval: 5 * time.Second,
		}
	}

	// 价格通道 max buffer 100 表示最多缓存100个价格信息
	priceCh := make(chan *GasPriceInfo, 100)

	go func() {
		defer close(priceCh)

		ticker := time.NewTicker(options.SampleInterval)
		defer ticker.Stop()

		for {
			select {
			case <-gs.client.ctx.Done():
				return
			case <-ticker.C:
				priceInfo, err := gs.GetGasPriceInfo()
				if err != nil {
					logger.Error("failed to get gas price info: ", err)
					continue
				}
				select {
				case priceCh <- priceInfo:
				case <-gs.client.ctx.Done():
					return
				default:
					logger.Warn("gas price channel is full, dropping price info")
				}

			}
		}
	}()
}

// GetOptimalGasPrice 获取最优Gas价格
func (gs *GasService) GetOptimalGasPrice(urgency string) (*big.Int, error) {
	priceInfo, err := gs.GetGasPriceInfo()
	if err != nil {
		return nil, err
	}

	switch urgency {
	case "slow", "standard":
		return priceInfo.Standard, nil
	case "fast":
		return priceInfo.Fast, nil
	case "instant":
		return priceInfo.Instant, nil
	default:
		return priceInfo.Standard, nil
	}
}

// AnalyzeRecentBlocks 分析最近区块的Gas使用情况
func (gs *GasService) AnalyzeRecentBlocks(blockCount int) (*GasPriceStats, error) {
	if blockCount <= 0 {
		blockCount = 10
	}

	latestBlock, err := gs.client.GetLatestBlock()
	if err != nil {
		return nil, fmt.Errorf("failed to get latest block: %w", err)
	}

	startBlock := new(big.Int).Sub(latestBlock.Number(), big.NewInt(int64(blockCount-1)))
	if startBlock.Sign() < 0 {
		startBlock = big.NewInt(0)
	}

	logger.WithFields(logrus.Fields{
		"start_block": startBlock.String(),
		"end_block":   latestBlock.Number().String(),
		"block_count": blockCount,
	}).Info("Analyzing recent blocks for gas statistics")

	var gasPrices []*big.Int
	var wg sync.WaitGroup
	priceCh := make(chan *big.Int, blockCount)
	semaphore := make(chan struct{}, 5) // 限制并发数

	// 并发获取区块Gas价格
	for i := 0; i < blockCount; i++ {
		wg.Add(1)
		go func(blockNum *big.Int) {
			defer wg.Done()

			// 获取信号量
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			block, err := gs.client.GetBlockByNumber(blockNum)
			if err != nil {
				logger.WithFields(logrus.Fields{
					"block_number": blockNum.String(),
					"error":        err,
				}).Warn("Failed to get block for gas analysis")
				return
			}

			// 分析区块中的Gas价格
			blockGasPrices := gs.extractGasPricesFromBlock(block)
			for _, price := range blockGasPrices {
				priceCh <- price
			}
		}(new(big.Int).Add(startBlock, big.NewInt(int64(i))))
	}

	// 等待所有协程完成
	go func() {
		wg.Wait()
		close(priceCh)
	}()

	// 收集Gas价格
	for price := range priceCh {
		gasPrices = append(gasPrices, price)
	}

	if len(gasPrices) == 0 {
		return nil, fmt.Errorf("no gas prices found in recent blocks")
	}

	// 计算统计信息
	stats := gs.calculateGasStats(gasPrices)
	stats.TimeRange = time.Duration(blockCount) * 12 * time.Second // 假设12秒出块时间

	return stats, nil
}

// extractGasPricesFromBlock 从区块中提取Gas价格
func (gs *GasService) extractGasPricesFromBlock(block *types.Block) []*big.Int {
	var prices []*big.Int

	for _, tx := range block.Transactions() {
		switch tx.Type() {
		case types.LegacyTxType, types.AccessListTxType:
			prices = append(prices, tx.GasPrice())
		case types.DynamicFeeTxType:
			// 对于EIP-1559交易，使用有效Gas价格
			if block.BaseFee() != nil {
				effectiveGasPrice := new(big.Int).Add(block.BaseFee(), tx.GasTipCap())
				if effectiveGasPrice.Cmp(tx.GasFeeCap()) > 0 {
					effectiveGasPrice = tx.GasFeeCap()
				}
				prices = append(prices, effectiveGasPrice)
			} else {
				prices = append(prices, tx.GasFeeCap())
			}
		}
	}

	return prices
}

// calculateGasStats 计算Gas价格统计信息
func (gs *GasService) calculateGasStats(prices []*big.Int) *GasPriceStats {
	if len(prices) == 0 {
		return &GasPriceStats{}
	}

	// 排序
	sort.Slice(prices, func(i, j int) bool {
		return prices[i].Cmp(prices[j]) < 0
	})

	stats := &GasPriceStats{
		Min:     new(big.Int).Set(prices[0]),
		Max:     new(big.Int).Set(prices[len(prices)-1]),
		Samples: len(prices),
	}

	// 计算平均值
	sum := big.NewInt(0)
	for _, price := range prices {
		sum.Add(sum, price)
	}
	stats.Average = new(big.Int).Div(sum, big.NewInt(int64(len(prices))))

	// 计算中位数
	medianIndex := len(prices) / 2
	if len(prices)%2 == 0 {
		medianSum := new(big.Int).Add(prices[medianIndex-1], prices[medianIndex])
		stats.Median = medianSum.Div(medianSum, big.NewInt(2))
	} else {
		stats.Median = new(big.Int).Set(prices[medianIndex])
	}

	// 计算标准差
	stats.StdDev = gs.calculateStandardDeviation(prices, stats.Average)

	return stats
}

// calculateStandardDeviation 计算标准差
func (gs *GasService) calculateStandardDeviation(prices []*big.Int, mean *big.Int) float64 {
	if len(prices) <= 1 {
		return 0
	}

	var sumSquaredDiff float64
	meanFloat, _ := mean.Float64()

	for _, price := range prices {
		priceFloat, _ := price.Float64()
		diff := priceFloat - meanFloat
		sumSquaredDiff += diff * diff
	}

	variance := sumSquaredDiff / float64(len(prices)-1)
	return variance // 返回方差，如果需要标准差可以开平方根
}

// EstimateGasForTransaction 估算交易Gas费用
func (gs *GasService) EstimateGasForTransaction(from, to *string, value *big.Int, data []byte) (*GasEstimate, error) {
	var gasLimit uint64
	var err error

	// 估算Gas限制
	err = gs.client.ExecuteWithRetry(func() error {
		ethClient := gs.client.GetEthClient()
		if ethClient == nil {
			return fmt.Errorf("eth client is nil")
		}

		// 构造调用消息
		msg := map[string]interface{}{
			"value": value,
			"data":  data,
		}
		if from != nil {
			msg["from"] = *from
		}
		if to != nil {
			msg["to"] = *to
		}

		// 估算Gas
		gasLimit, err = ethClient.EstimateGas(gs.client.ctx, ethereum.CallMsg{
			From:  common.HexToAddress(*from),
			To:    (*common.Address)(nil),
			Value: value,
			Data:  data,
		})
		return err
	})

	if err != nil {
		return nil, fmt.Errorf("failed to estimate gas: %w", err)
	}

	// 获取当前Gas价格信息
	gasPriceInfo, err := gs.GetGasPriceInfo()
	if err != nil {
		return nil, fmt.Errorf("failed to get gas price info: %w", err)
	}

	estimate := &GasEstimate{
		GasLimit:   gasLimit,
		GasPrice:   gasPriceInfo.Standard,
		Confidence: 0.8, // 80%置信度
	}

	// 如果支持EIP-1559
	if gasPriceInfo.BaseFee != nil {
		estimate.MaxFeePerGas = gasPriceInfo.Fast
		estimate.MaxPriorityFeePerGas = gasPriceInfo.Priority
		estimate.EstimatedCost = new(big.Int).Mul(big.NewInt(int64(gasLimit)), gasPriceInfo.Fast)
	} else {
		estimate.EstimatedCost = new(big.Int).Mul(big.NewInt(int64(gasLimit)), gasPriceInfo.Standard)
	}

	return estimate, nil
}
