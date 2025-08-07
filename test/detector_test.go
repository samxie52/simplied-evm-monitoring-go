package alert

import (
	"context"
	"math/big"
	"simplied-evm-monitoring-go/internal/models"
	"simplied-evm-monitoring-go/internal/services/alert"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLargeTransactionDetector_Basic(t *testing.T) {
	// 创建测试配置
	config := &alert.DetectorConfig{
		ETHThreshold:       10.0, // 10 ETH 阈值
		USDThreshold:       10000.0,
		DetectionInterval:  5 * time.Second,
		EnableETHThreshold: true,
		EnableUSDThreshold: false,
		BatchSize:          10,
		MaxConcurrency:     2,
	}

	// 创建告警回调
	var receivedAlerts []*models.Alert
	alertCallback := func(alert *models.Alert) error {
		receivedAlerts = append(receivedAlerts, alert)
		return nil
	}

	// 创建检测器
	detector := alert.NewLargeTransactionDetector(config, alertCallback)
	require.NotNil(t, detector)

	// 启动检测器
	err := detector.Start()
	require.NoError(t, err)
	assert.True(t, detector.IsRunning())

	// 停止检测器
	err = detector.Stop()
	require.NoError(t, err)
	assert.False(t, detector.IsRunning())
}

func TestLargeTransactionDetector_DetectLargeTransactions(t *testing.T) {
	config := &alert.DetectorConfig{
		ETHThreshold:       5.0, // 5 ETH 阈值
		EnableETHThreshold: true,
		EnableUSDThreshold: false,
		BatchSize:          10,
		MaxConcurrency:     2,
	}

	var receivedAlerts []*models.Alert
	alertCallback := func(alert *models.Alert) error {
		receivedAlerts = append(receivedAlerts, alert)
		return nil
	}

	detector := alert.NewLargeTransactionDetector(config, alertCallback)
	err := detector.Start()
	require.NoError(t, err)
	defer detector.Stop()

	// 创建测试交易
	transactions := []*types.Transaction{
		// 小额交易 (1 ETH)
		createTestTransaction(big.NewInt(1e18), common.HexToAddress("0x1234")),
		// 大额交易 (10 ETH)
		createTestTransaction(mustParseBigInt("10000000000000000000"), common.HexToAddress("0x5678")), // 10 * 1e18
		// 超大额交易 (100 ETH)
		createTestTransaction(mustParseBigInt("100000000000000000000"), common.HexToAddress("0x9abc")), // 100 * 1e18
	}

	// 执行检测
	ctx := context.Background()
	alerts, err := detector.DetectLargeTransactions(ctx, transactions, nil)
	require.NoError(t, err)

	// 验证结果
	assert.Len(t, alerts, 2) // 应该检测到 2 个大额交易

	// 验证第一个告警 (10 ETH)
	assert.Equal(t, 10.0, alerts[0].ValueETH)
	assert.Equal(t, "ETH", alerts[0].ThresholdType)

	// 验证第二个告警 (100 ETH)
	assert.Equal(t, 100.0, alerts[1].ValueETH)
	assert.Equal(t, "ETH", alerts[1].ThresholdType)
}

func TestLargeTransactionDetector_Deduplication(t *testing.T) {
	config := &alert.DetectorConfig{
		ETHThreshold:       1.0, // 1 ETH 阈值
		EnableETHThreshold: true,
		BatchSize:          10,
		MaxConcurrency:     2,
	}

	detector := alert.NewLargeTransactionDetector(config, nil)
	err := detector.Start()
	require.NoError(t, err)
	defer detector.Stop()

	// 创建相同的交易
	tx := createTestTransaction(big.NewInt(5e18), common.HexToAddress("0x1234"))
	transactions := []*types.Transaction{tx, tx} // 重复的交易

	// 第一次检测
	ctx := context.Background()
	alerts1, err := detector.DetectLargeTransactions(ctx, transactions, nil)
	require.NoError(t, err)
	assert.Len(t, alerts1, 1) // 第一次应该检测到

	// 第二次检测（应该被去重）
	alerts2, err := detector.DetectLargeTransactions(ctx, transactions, nil)
	require.NoError(t, err)
	assert.Len(t, alerts2, 0) // 第二次应该被去重
}

func TestLargeTransactionDetector_ThresholdUpdate(t *testing.T) {
	config := &alert.DetectorConfig{
		ETHThreshold:       10.0,
		EnableETHThreshold: true,
		BatchSize:          10,
		MaxConcurrency:     2,
	}

	detector := alert.NewLargeTransactionDetector(config, nil)
	err := detector.Start()
	require.NoError(t, err)
	defer detector.Stop()

	// 验证初始配置
	currentConfig := detector.GetConfig()
	assert.Equal(t, 10.0, currentConfig.ETHThreshold)

	// 更新配置
	newConfig := &alert.DetectorConfig{
		ETHThreshold:       5.0,
		EnableETHThreshold: true,
		BatchSize:          10,
		MaxConcurrency:     2,
	}
	err = detector.UpdateConfig(newConfig)
	require.NoError(t, err)

	// 验证配置已更新
	updatedConfig := detector.GetConfig()
	assert.Equal(t, 5.0, updatedConfig.ETHThreshold)
}

func TestLargeTransactionDetector_Stats(t *testing.T) {
	config := &alert.DetectorConfig{
		ETHThreshold:       10.0,
		EnableETHThreshold: true,
		BatchSize:          10,
		MaxConcurrency:     2,
	}

	detector := alert.NewLargeTransactionDetector(config, nil)

	// 获取统计信息
	stats := detector.GetStats()
	assert.NotNil(t, stats)
	assert.Equal(t, false, stats["is_running"])
	assert.Equal(t, 10.0, stats["eth_threshold"])
	assert.Equal(t, true, stats["enable_eth"])

	// 启动后再次检查
	err := detector.Start()
	require.NoError(t, err)
	defer detector.Stop()

	stats = detector.GetStats()
	assert.Equal(t, true, stats["is_running"])
}

func TestThresholdManager_Basic(t *testing.T) {
	tm := alert.NewThresholdManager(10.0, 50000.0)

	// 测试 ETH 阈值
	assert.True(t, tm.IsAboveETHThreshold(15.0))
	assert.False(t, tm.IsAboveETHThreshold(5.0))
	assert.True(t, tm.IsAboveETHThreshold(10.0)) // 等于阈值也应该触发

	// 测试 USD 阈值
	assert.True(t, tm.IsAboveUSDThreshold(60000.0))
	assert.False(t, tm.IsAboveUSDThreshold(30000.0))

	// 测试阈值更新
	err := tm.UpdateThresholds(20.0, 100000.0)
	require.NoError(t, err)
	assert.Equal(t, 20.0, tm.GetETHThreshold())
	assert.Equal(t, 100000.0, tm.GetUSDThreshold())
}

func TestThresholdManager_ThresholdLevels(t *testing.T) {
	tm := alert.NewThresholdManager(10.0, 50000.0)

	// 测试阈值级别
	assert.Equal(t, alert.ThresholdLevelLow, tm.GetThresholdLevel(12.0, alert.ThresholdTypeETH))       // 1.2x
	assert.Equal(t, alert.ThresholdLevelMedium, tm.GetThresholdLevel(25.0, alert.ThresholdTypeETH))    // 2.5x
	assert.Equal(t, alert.ThresholdLevelHigh, tm.GetThresholdLevel(60.0, alert.ThresholdTypeETH))      // 6x
	assert.Equal(t, alert.ThresholdLevelCritical, tm.GetThresholdLevel(150.0, alert.ThresholdTypeETH)) // 15x
}

func TestDeduplicationManager_Basic(t *testing.T) {
	dm := alert.NewDeduplicationManager(5 * time.Minute)
	defer dm.Stop()

	// 测试第一次记录
	assert.False(t, dm.IsDuplicate("alert1"))
	dm.RecordAlert("alert1")
	assert.True(t, dm.IsDuplicate("alert1"))

	// 测试不同的告警
	assert.False(t, dm.IsDuplicate("alert2"))

	// 测试缓存大小
	assert.Equal(t, 1, dm.GetCacheSize())

	// 测试记录获取
	record, exists := dm.GetRecord("alert1")
	assert.True(t, exists)
	assert.Equal(t, "alert1", record.AlertID)
	assert.Equal(t, 1, record.TriggerCount)

	// 测试重复记录
	dm.RecordAlert("alert1")
	record, exists = dm.GetRecord("alert1")
	assert.True(t, exists)
	assert.Equal(t, 2, record.TriggerCount)
}

func TestDeduplicationManager_ContentBased(t *testing.T) {
	dm := alert.NewDeduplicationManager(5 * time.Minute)
	defer dm.Stop()

	content1 := map[string]interface{}{
		"tx_hash": "0x123",
		"value":   100.0,
	}

	content2 := map[string]interface{}{
		"tx_hash": "0x456",
		"value":   200.0,
	}

	// 测试基于内容的去重
	assert.False(t, dm.IsDuplicateWithContent("alert1", content1))
	dm.RecordAlertWithContent("alert1", content1)
	assert.True(t, dm.IsDuplicateWithContent("alert2", content1)) // 相同内容

	// 不同内容不应该被去重
	assert.False(t, dm.IsDuplicateWithContent("alert3", content2))
}

// 辅助函数：创建测试交易
func createTestTransaction(value *big.Int, to common.Address) *types.Transaction {
	return types.NewTransaction(
		0,                       // nonce
		to,                      // to
		value,                   // value
		21000,                   // gas limit
		big.NewInt(20000000000), // gas price (20 gwei)
		nil,                     // data
	)
}

// 基准测试
func BenchmarkLargeTransactionDetector_DetectLargeTransactions(b *testing.B) {
	config := &alert.DetectorConfig{
		ETHThreshold:       10.0,
		EnableETHThreshold: true,
		BatchSize:          100,
		MaxConcurrency:     5,
	}

	detector := alert.NewLargeTransactionDetector(config, nil)
	detector.Start()
	defer detector.Stop()

	// 创建测试交易
	transactions := make([]*types.Transaction, 1000)
	for i := 0; i < 1000; i++ {
		value := big.NewInt(int64(i) * 1e18) // i ETH
		transactions[i] = createTestTransaction(value, common.HexToAddress("0x1234"))
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := detector.DetectLargeTransactions(ctx, transactions, nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// mustParseBigInt 解析字符串为大整数，失败时 panic
func mustParseBigInt(s string) *big.Int {
	result, ok := new(big.Int).SetString(s, 10)
	if !ok {
		panic("failed to parse big int: " + s)
	}
	return result
}
