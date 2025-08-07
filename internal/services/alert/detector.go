package alert

import (
	"context"
	"fmt"
	"math/big"
	"simplied-evm-monitoring-go/internal/models"
	"simplied-evm-monitoring-go/pkg/logger"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/sirupsen/logrus"
)

// LargeTransactionDetector 大额交易检测器
type LargeTransactionDetector struct {
	// 阈值管理器
	thresholdManager *ThresholdManager
	// 去重管理器
	deduplicationManager *DeduplicationManager
	// 检测配置
	config *DetectorConfig
	// 运行状态
	isRunning bool
	mu        sync.RWMutex
	// 停止信号
	stopChan chan struct{}
	// 告警回调函数
	alertCallback func(*models.Alert) error
}

// DetectorConfig 检测器配置
type DetectorConfig struct {
	// ETH 阈值（以太币单位）
	ETHThreshold float64 `json:"eth_threshold"`
	// USD 阈值（美元单位）
	USDThreshold float64 `json:"usd_threshold"`
	// 检测间隔
	DetectionInterval time.Duration `json:"detection_interval"`
	// 是否启用 ETH 阈值检测
	EnableETHThreshold bool `json:"enable_eth_threshold"`
	// 是否启用 USD 阈值检测
	EnableUSDThreshold bool `json:"enable_usd_threshold"`
	// 批处理大小
	BatchSize int `json:"batch_size"`
	// 最大并发数
	MaxConcurrency int `json:"max_concurrency"`
}

// TransactionAlert 交易告警数据
type TransactionAlert struct {
	// 交易哈希
	TransactionHash string `json:"transaction_hash"`
	// 区块号
	BlockNumber uint64 `json:"block_number"`
	// 发送方地址
	FromAddress string `json:"from_address"`
	// 接收方地址
	ToAddress string `json:"to_address"`
	// 交易金额（ETH）
	ValueETH float64 `json:"value_eth"`
	// 交易金额（USD）
	ValueUSD float64 `json:"value_usd"`
	// 触发的阈值类型
	ThresholdType string `json:"threshold_type"`
	// 触发时间
	TriggerTime time.Time `json:"trigger_time"`
	// 额外上下文信息
	Context map[string]interface{} `json:"context"`
}

// NewLargeTransactionDetector 创建大额交易检测器
func NewLargeTransactionDetector(config *DetectorConfig, alertCallback func(*models.Alert) error) *LargeTransactionDetector {
	if config == nil {
		config = &DetectorConfig{
			ETHThreshold:       100.0,    // 默认 100 ETH
			USDThreshold:       100000.0, // 默认 $100,000
			DetectionInterval:  10 * time.Second,
			EnableETHThreshold: true,
			EnableUSDThreshold: false, // 默认关闭 USD 检测（需要价格数据）
			BatchSize:          50,
			MaxConcurrency:     5,
		}
	}

	return &LargeTransactionDetector{
		thresholdManager:     NewThresholdManager(config.ETHThreshold, config.USDThreshold),
		deduplicationManager: NewDeduplicationManager(5 * time.Minute), // 5分钟去重窗口
		config:               config,
		stopChan:             make(chan struct{}),
		alertCallback:        alertCallback,
	}
}

// Start 启动检测器
func (d *LargeTransactionDetector) Start() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.isRunning {
		return fmt.Errorf("detector is already running")
	}

	d.isRunning = true
	logger.WithFields(logrus.Fields{
		"eth_threshold": d.config.ETHThreshold,
		"usd_threshold": d.config.USDThreshold,
		"interval":      d.config.DetectionInterval,
	}).Info("Large transaction detector started")

	return nil
}

// Stop 停止检测器
func (d *LargeTransactionDetector) Stop() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.isRunning {
		return fmt.Errorf("detector is not running")
	}

	close(d.stopChan)
	d.isRunning = false
	logger.Info("Large transaction detector stopped")

	return nil
}

// IsRunning 检查检测器是否运行中
func (d *LargeTransactionDetector) IsRunning() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.isRunning
}

// DetectLargeTransactions 检测大额交易
func (d *LargeTransactionDetector) DetectLargeTransactions(ctx context.Context, transactions []*types.Transaction, receipts []*types.Receipt) ([]*TransactionAlert, error) {
	if !d.IsRunning() {
		return nil, fmt.Errorf("detector is not running")
	}

	var alerts []*TransactionAlert
	var mu sync.Mutex

	// 创建工作池
	semaphore := make(chan struct{}, d.config.MaxConcurrency)
	var wg sync.WaitGroup

	// 处理交易批次
	for i := 0; i < len(transactions); i += d.config.BatchSize {
		logger.WithFields(logrus.Fields{
			"batch_start": i,
			"batch_size":  d.config.BatchSize,
		}).Info("Processing transaction batch")
		end := i + d.config.BatchSize
		if end > len(transactions) {
			end = len(transactions)
		}

		batch := transactions[i:end]
		var batchReceipts []*types.Receipt
		if receipts != nil && len(receipts) > i {
			receiptEnd := end
			if receiptEnd > len(receipts) {
				receiptEnd = len(receipts)
			}
			batchReceipts = receipts[i:receiptEnd]
		}

		wg.Add(1)
		go func(txBatch []*types.Transaction, receiptBatch []*types.Receipt) {
			defer wg.Done()

			// 获取信号量
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			batchAlerts := d.processBatch(ctx, txBatch, receiptBatch)

			mu.Lock()
			alerts = append(alerts, batchAlerts...)
			mu.Unlock()
		}(batch, batchReceipts)
	}

	wg.Wait()

	logger.WithFields(logrus.Fields{
		"processed_transactions": len(transactions),
		"detected_alerts":        len(alerts),
	}).Info("Large transaction detection completed")

	return alerts, nil
}

// processBatch 处理交易批次
func (d *LargeTransactionDetector) processBatch(ctx context.Context, transactions []*types.Transaction, receipts []*types.Receipt) []*TransactionAlert {
	var alerts []*TransactionAlert

	logger.Infof("Processing batch of %d transactions", len(transactions))

	for i, tx := range transactions {
		select {
		case <-ctx.Done():
			return alerts
		case <-d.stopChan:
			return alerts
		default:
		}

		var receipt *types.Receipt
		if receipts != nil && i < len(receipts) {
			receipt = receipts[i]
		}

		alert := d.checkTransaction(tx, receipt)
		if alert != nil {
			alerts = append(alerts, alert)
		}
	}

	return alerts
}

// checkTransaction 检查单个交易
func (d *LargeTransactionDetector) checkTransaction(tx *types.Transaction, receipt *types.Receipt) *TransactionAlert {
	// 计算交易金额（ETH）
	valueETH := d.weiToEther(tx.Value())

	// 检查是否超过 ETH 阈值
	if d.config.EnableETHThreshold && d.thresholdManager.IsAboveETHThreshold(valueETH) {
		// 检查去重
		if d.deduplicationManager.IsDuplicate(tx.Hash().Hex()) {
			logger.WithFields(logrus.Fields{
				"tx_hash": tx.Hash().Hex(),
				"value":   valueETH,
			}).Debug("Duplicate large transaction alert suppressed")
			return nil
		}

		// 记录去重信息
		d.deduplicationManager.RecordAlert(tx.Hash().Hex())

		// 创建告警
		alert := &TransactionAlert{
			TransactionHash: tx.Hash().Hex(),
			FromAddress:     d.extractFromAddress(tx, receipt),
			ValueETH:        valueETH,
			ThresholdType:   "ETH",
			TriggerTime:     time.Now(),
			Context: map[string]interface{}{
				"gas":       tx.Gas(),
				"gas_price": tx.GasPrice().String(),
				"nonce":     tx.Nonce(),
			},
		}

		// 设置接收方地址
		if tx.To() != nil {
			alert.ToAddress = tx.To().Hex()
		} else {
			alert.ToAddress = "contract_creation"
		}

		// 从收据中获取区块信息
		if receipt != nil {
			alert.BlockNumber = receipt.BlockNumber.Uint64()
			alert.Context["block_hash"] = receipt.BlockHash.Hex()
			alert.Context["gas_used"] = receipt.GasUsed
			alert.Context["status"] = receipt.Status
		}

		logger.WithFields(logrus.Fields{
			"tx_hash":   alert.TransactionHash,
			"value_eth": alert.ValueETH,
			"threshold": d.config.ETHThreshold,
			"from":      alert.FromAddress,
			"to":        alert.ToAddress,
			"block":     alert.BlockNumber,
		}).Warn("Large transaction detected")

		return alert
	}

	// TODO: 实现 USD 阈值检测（需要价格数据）
	if d.config.EnableUSDThreshold {
		// 这里需要集成价格服务来获取 ETH/USD 汇率
		// valueUSD := valueETH * ethPrice
		// if d.thresholdManager.IsAboveUSDThreshold(valueUSD) { ... }
	}

	return nil
}

// ProcessAlert 处理告警
func (d *LargeTransactionDetector) ProcessAlert(alert *TransactionAlert) error {
	if d.alertCallback == nil {
		logger.Warn("No alert callback configured, skipping alert processing")
		return nil
	}

	// 转换为标准告警模型
	modelAlert := &models.Alert{
		Type:         models.AlertTypeLargeTransfer,
		Severity:     models.SeverityHigh,
		Title:        fmt.Sprintf("Large Transaction Detected: %.2f ETH", alert.ValueETH),
		Message:      d.formatAlertMessage(alert),
		TriggerValue: alert.ValueETH,
		TriggerTime:  alert.TriggerTime,
		Status:       models.NotificationStatusPending,
	}

	// 设置触发数据
	triggerData := &models.AlertTriggerData{
		SourceType:   "transaction",
		SourceID:     alert.TransactionHash,
		MatchedValue: alert.ValueETH,
		Context: map[string]interface{}{
			"transaction_hash": alert.TransactionHash,
			"block_number":     alert.BlockNumber,
			"from_address":     alert.FromAddress,
			"to_address":       alert.ToAddress,
			"value_eth":        alert.ValueETH,
			"threshold_type":   alert.ThresholdType,
		},
		Timestamp: alert.TriggerTime,
	}

	if err := modelAlert.SetTriggerData(triggerData); err != nil {
		logger.WithFields(logrus.Fields{"error": err}).Error("Failed to set trigger data for alert")
		return err
	}

	// 调用告警回调
	return d.alertCallback(modelAlert)
}

// formatAlertMessage 格式化告警消息
func (d *LargeTransactionDetector) formatAlertMessage(alert *TransactionAlert) string {
	return fmt.Sprintf(
		"🚨 Large Transaction Alert\n\n"+
			"💰 Amount: %.4f ETH\n"+
			"📊 Threshold: %.2f ETH (%s)\n"+
			"📋 Transaction: %s\n"+
			"🏦 From: %s\n"+
			"🎯 To: %s\n"+
			"📦 Block: %d\n"+
			"⏰ Time: %s",
		alert.ValueETH,
		d.config.ETHThreshold,
		alert.ThresholdType,
		alert.TransactionHash,
		alert.FromAddress,
		alert.ToAddress,
		alert.BlockNumber,
		alert.TriggerTime.Format("2006-01-02 15:04:05 UTC"),
	)
}

// UpdateConfig 更新检测器配置
func (d *LargeTransactionDetector) UpdateConfig(config *DetectorConfig) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.config = config
	d.thresholdManager.UpdateThresholds(config.ETHThreshold, config.USDThreshold)

	logger.WithFields(logrus.Fields{
		"eth_threshold": config.ETHThreshold,
		"usd_threshold": config.USDThreshold,
	}).Info("Detector configuration updated")

	return nil
}

// GetConfig 获取当前配置
func (d *LargeTransactionDetector) GetConfig() *DetectorConfig {
	d.mu.RLock()
	defer d.mu.RUnlock()

	// 返回配置副本
	configCopy := *d.config
	return &configCopy
}

// GetStats 获取检测器统计信息
func (d *LargeTransactionDetector) GetStats() map[string]interface{} {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return map[string]interface{}{
		"is_running":         d.isRunning,
		"eth_threshold":      d.config.ETHThreshold,
		"usd_threshold":      d.config.USDThreshold,
		"enable_eth":         d.config.EnableETHThreshold,
		"enable_usd":         d.config.EnableUSDThreshold,
		"detection_interval": d.config.DetectionInterval.String(),
		"dedup_window":       d.deduplicationManager.GetWindowDuration().String(),
		"dedup_cache_size":   d.deduplicationManager.GetCacheSize(),
	}
}

// extractFromAddress 提取发送方地址
func (d *LargeTransactionDetector) extractFromAddress(tx *types.Transaction, receipt *types.Receipt) string {
	// 在实际实现中，我们需要从签名中恢复发送方地址
	// 这里暂时返回一个占位符，实际应该使用 crypto.Sender() 或从其他地方获取
	// TODO: 实现真正的地址恢复逻辑
	return "0x0000000000000000000000000000000000000000" // 占位符
}

// weiToEther 将 Wei 转换为 Ether
func (d *LargeTransactionDetector) weiToEther(wei *big.Int) float64 {
	if wei == nil {
		return 0
	}

	// 1 ETH = 10^18 Wei
	ether := new(big.Float).SetInt(wei)
	ether.Quo(ether, big.NewFloat(1e18))

	result, _ := ether.Float64()
	return result
}
