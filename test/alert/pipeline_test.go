package alert

import (
	"testing"
	"time"

	"simplied-evm-monitoring-go/internal/models"
	"simplied-evm-monitoring-go/internal/services/alert"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)



// MockTelegramBot 模拟Telegram Bot
type MockTelegramBot struct {
	sentMessages []string
	shouldFail   bool
}

func (m *MockTelegramBot) Start() error {
	return nil
}

func (m *MockTelegramBot) Stop() error {
	return nil
}

func (m *MockTelegramBot) SendMessage(chatID int64, message string) error {
	if m.shouldFail {
		return assert.AnError
	}
	m.sentMessages = append(m.sentMessages, message)
	return nil
}

func (m *MockTelegramBot) GetStats() map[string]interface{} {
	return map[string]interface{}{}
}

func (m *MockTelegramBot) IsRunning() bool {
	return true
}

// TestAlertPipeline_Basic 测试基本的告警流水线功能
func TestAlertPipeline_Basic(t *testing.T) {
	// 创建模拟的Telegram Bot
	mockBot := &MockTelegramBot{}

	// 创建告警管理器
	manager := alert.NewAlertManager(nil, nil, nil)

	// 创建流水线配置
	config := &alert.PipelineConfig{
		WorkerCount:         2,
		QueueSize:           10,
		BatchSize:           3,
		BatchInterval:       100 * time.Millisecond,
		MaxRetries:          2,
		RetryInterval:       50 * time.Millisecond,
		EnableBatching:      false, // 先测试单个处理
		EnableDeduplication: true,
	}

	// 创建告警流水线
	pipeline := alert.NewAlertPipeline(config, manager, mockBot)

	// 启动流水线
	err := pipeline.Start()
	require.NoError(t, err)
	defer pipeline.Stop()

	// 创建测试告警
	testAlert := &models.Alert{
		BaseModel: models.BaseModel{ID: 1},
		Type:      models.AlertTypeLargeTransfer,
		Severity:  models.SeverityHigh,
		Message:   "Large transfer detected",
		TriggerData: `{
			"transaction_hash": "0x123",
			"from_address": "0xabc",
			"to_address": "0xdef",
			"amount": "1000000000000000000",
			"gas_price": "20000000000"
		}`,
	}

	// 处理告警
	err = pipeline.ProcessAlert(testAlert)
	require.NoError(t, err)

	// 等待处理完成
	time.Sleep(200 * time.Millisecond)

	// 验证告警被处理
	metrics := pipeline.GetMetrics()
	assert.Equal(t, int64(1), metrics.GetAlertsReceived())
	assert.Equal(t, int64(1), metrics.GetAlertsProcessed())
	assert.Equal(t, int64(0), metrics.GetAlertsFailed())

	// 验证告警状态
	tracker := pipeline.GetTracker()
	record, exists := tracker.GetAlertRecord(uint64(testAlert.ID))
	require.True(t, exists)
	assert.Equal(t, alert.AlertStatusSent, record.Status)
	assert.NotNil(t, record.SentAt)

	// 验证消息被发送
	assert.Len(t, mockBot.sentMessages, 1)
}

// TestAlertPipeline_BatchProcessing 测试批量处理
func TestAlertPipeline_BatchProcessing(t *testing.T) {
	mockBot := &MockTelegramBot{}
	manager := alert.NewAlertManager(nil, nil, nil)

	config := &alert.PipelineConfig{
		WorkerCount:         1,
		QueueSize:           20,
		BatchSize:           3,
		BatchInterval:       100 * time.Millisecond,
		MaxRetries:          1,
		RetryInterval:       50 * time.Millisecond,
		EnableBatching:      true,
		EnableDeduplication: false,
	}

	pipeline := alert.NewAlertPipeline(config, manager, mockBot)

	err := pipeline.Start()
	require.NoError(t, err)
	defer pipeline.Stop()

	// 创建多个测试告警
	alerts := []*models.Alert{
		{
			BaseModel: models.BaseModel{
				ID:        uint64(1),
				CreatedAt: time.Now(),
			},
			Type:     models.AlertTypeLargeTransfer,
			Severity: models.SeverityHigh,
			Title:    "Test Alert 1",
			Message:  "Test message 1",
		},
		{
			BaseModel: models.BaseModel{
				ID:        uint64(2),
				CreatedAt: time.Now(),
			},
			Type:     models.AlertTypeLargeTransfer,
			Severity: models.SeverityHigh,
			Title:    "Test Alert 2",
			Message:  "Test message 2",
		},
		{
			BaseModel: models.BaseModel{
				ID:        uint64(3),
				CreatedAt: time.Now(),
			},
			Type:     models.AlertTypeLargeTransfer,
			Severity: models.SeverityHigh,
			Title:    "Test Alert 3",
			Message:  "Test message 3",
		},
		{
			BaseModel: models.BaseModel{
				ID:        uint64(4),
				CreatedAt: time.Now(),
			},
			Type:     models.AlertTypeLargeTransfer,
			Severity: models.SeverityHigh,
			Title:    "Test Alert 4",
			Message:  "Test message 4",
		},
		{
			BaseModel: models.BaseModel{
				ID:        uint64(5),
				CreatedAt: time.Now(),
			},
			Type:     models.AlertTypeLargeTransfer,
			Severity: models.SeverityHigh,
			Title:    "Test Alert 5",
			Message:  "Test message 5",
		},
	}

	// 批量处理告警
	err = pipeline.ProcessAlerts(alerts)
	require.NoError(t, err)

	// 等待批量处理完成
	time.Sleep(300 * time.Millisecond)

	// 验证指标
	metrics := pipeline.GetMetrics()
	assert.Equal(t, int64(5), metrics.GetAlertsReceived())
	assert.True(t, metrics.GetBatchesProcessed() > 0)

	// 验证所有告警都被跟踪
	tracker := pipeline.GetTracker()
	for _, testAlert := range alerts {
		record, exists := tracker.GetAlertRecord(testAlert.ID)
		assert.True(t, exists)
		assert.Equal(t, alert.AlertStatusSent, record.Status)
	}
}

// TestAlertPipeline_Deduplication 测试去重功能
func TestAlertPipeline_Deduplication(t *testing.T) {
	mockBot := &MockTelegramBot{}
	manager := alert.NewAlertManager(nil, nil, nil)

	config := &alert.PipelineConfig{
		WorkerCount:         1,
		QueueSize:           10,
		BatchSize:           3,
		BatchInterval:       100 * time.Millisecond,
		MaxRetries:          1,
		RetryInterval:       50 * time.Millisecond,
		EnableBatching:      false,
		EnableDeduplication: true,
	}

	pipeline := alert.NewAlertPipeline(config, manager, mockBot)

	err := pipeline.Start()
	require.NoError(t, err)
	defer pipeline.Stop()

	// 创建相同的告警
	alert1 := &models.Alert{
		BaseModel: models.BaseModel{
			ID:        uint64(1),
			CreatedAt: time.Now(),
		},
		Type:        models.AlertTypeLargeTransfer,
		Severity:    models.SeverityHigh,
		Title:       "Duplicate test",
		Message:     "Duplicate test",
		TriggerData: `{"amount": "1000", "hash": "0x123"}`,
	}

	alert2 := &models.Alert{
		BaseModel: models.BaseModel{
			ID:        uint64(2),
			CreatedAt: time.Now(),
		},
		Type:        models.AlertTypeLargeTransfer,
		Severity:    models.SeverityHigh,
		Title:       "Duplicate test",
		Message:     "Duplicate test",
		TriggerData: `{"amount": "1000", "hash": "0x123"}`, // 相同的触发数据
	}

	// 处理第一个告警
	err = pipeline.ProcessAlert(alert1)
	require.NoError(t, err)

	// 等待处理完成
	time.Sleep(100 * time.Millisecond)

	// 处理第二个告警（应该被去重）
	err = pipeline.ProcessAlert(alert2)
	require.NoError(t, err)

	// 等待处理完成
	time.Sleep(100 * time.Millisecond)

	// 验证指标
	metrics := pipeline.GetMetrics()
	assert.Equal(t, int64(2), metrics.GetAlertsReceived())
	assert.Equal(t, int64(1), metrics.GetAlertsProcessed()) // 只有一个被处理
	assert.Equal(t, int64(1), metrics.GetAlertsDuplicated())

	// 验证只发送了一条消息
	assert.Len(t, mockBot.sentMessages, 1)
}

// TestAlertPipeline_ErrorHandling 测试错误处理和重试
func TestAlertPipeline_ErrorHandling(t *testing.T) {
	mockBot := &MockTelegramBot{shouldFail: true} // 模拟发送失败
	manager := alert.NewAlertManager(nil, nil, nil)

	config := &alert.PipelineConfig{
		WorkerCount:         1,
		QueueSize:           10,
		BatchSize:           3,
		BatchInterval:       100 * time.Millisecond,
		MaxRetries:          2,
		RetryInterval:       50 * time.Millisecond,
		EnableBatching:      false,
		EnableDeduplication: false,
	}

	pipeline := alert.NewAlertPipeline(config, manager, mockBot)

	err := pipeline.Start()
	require.NoError(t, err)
	defer pipeline.Stop()

	// 创建测试告警
	testAlert := &models.Alert{
		BaseModel: models.BaseModel{
			ID:        uint64(1),
			CreatedAt: time.Now(),
		},
		Type:        models.AlertTypeLargeTransfer,
		Severity:    models.SeverityHigh,
		Title:       "Error test",
		Message:     "Error test",
		TriggerData: `{"test": true}`,
	}

	// 处理告警
	err = pipeline.ProcessAlert(testAlert)
	require.NoError(t, err)

	// 等待重试完成
	time.Sleep(500 * time.Millisecond)

	// 验证指标
	metrics := pipeline.GetMetrics()
	assert.Equal(t, int64(1), metrics.GetAlertsReceived())
	assert.Equal(t, int64(0), metrics.GetAlertsProcessed())
	assert.Equal(t, int64(1), metrics.GetAlertsFailed())

	// 验证告警状态
	tracker := pipeline.GetTracker()
	record, exists := tracker.GetAlertRecord(testAlert.ID)
	require.True(t, exists)
	assert.Equal(t, alert.AlertStatusFailed, record.Status)
	assert.True(t, record.RetryCount > 0)
	assert.NotEmpty(t, record.LastError)
}

// TestAlertPipeline_QueueOverflow 测试队列溢出
func TestAlertPipeline_QueueOverflow(t *testing.T) {
	mockBot := &MockTelegramBot{}
	manager := alert.NewAlertManager(nil, nil, nil)

	config := &alert.PipelineConfig{
		WorkerCount:         1,
		QueueSize:           2, // 很小的队列
		BatchSize:           3,
		BatchInterval:       100 * time.Millisecond,
		MaxRetries:          1,
		RetryInterval:       50 * time.Millisecond,
		EnableBatching:      false,
		EnableDeduplication: false,
	}

	pipeline := alert.NewAlertPipeline(config, manager, mockBot)

	err := pipeline.Start()
	require.NoError(t, err)
	defer pipeline.Stop()

	// 尝试添加超过队列容量的告警
	var processErrors []error
	for i := 0; i < 5; i++ {
		testAlert := &models.Alert{
			BaseModel:   models.BaseModel{ID: uint64(i + 1)},
			Type:        models.AlertTypeLargeTransfer,
			Severity:    models.SeverityLow,
			Message:     "Queue test",
			TriggerData: `{"test": true}`,
		}

		err := pipeline.ProcessAlert(testAlert)
		if err != nil {
			processErrors = append(processErrors, err)
		}
	}

	// 应该有一些告警因为队列满而被拒绝
	assert.True(t, len(processErrors) > 0)

	// 等待处理完成
	time.Sleep(200 * time.Millisecond)

	// 验证有告警被丢弃
	metrics := pipeline.GetMetrics()
	assert.True(t, metrics.GetAlertsDropped() > 0)
}

// TestAlertPipeline_Metrics 测试指标收集
func TestAlertPipeline_Metrics(t *testing.T) {
	mockBot := &MockTelegramBot{}
	manager := alert.NewAlertManager(nil, nil, nil)

	config := alert.DefaultPipelineConfig()
	config.EnableBatching = false

	pipeline := alert.NewAlertPipeline(config, manager, mockBot)

	err := pipeline.Start()
	require.NoError(t, err)
	defer pipeline.Stop()

	// 处理一些告警
	for i := 0; i < 3; i++ {
		testAlert := &models.Alert{
			BaseModel:   models.BaseModel{ID: uint64(i + 1)},
			Type:        models.AlertTypeLargeTransfer,
			Severity:    models.SeverityMedium,
			Message:     "Metrics test",
			TriggerData: `{"test": true}`,
		}

		err := pipeline.ProcessAlert(testAlert)
		require.NoError(t, err)
	}

	// 等待处理完成
	time.Sleep(200 * time.Millisecond)

	// 验证指标
	metrics := pipeline.GetMetrics()
	summary := metrics.GetSummary()

	assert.Equal(t, int64(3), metrics.GetAlertsReceived())
	assert.Equal(t, int64(3), metrics.GetAlertsProcessed())
	assert.True(t, metrics.GetAverageProcessingTime() > 0)
	assert.Equal(t, 100.0, metrics.GetSuccessRate())

	// 验证摘要包含所有必要的字段
	assert.Contains(t, summary, "alerts")
	assert.Contains(t, summary, "performance")
	assert.Contains(t, summary, "queues")
	assert.Contains(t, summary, "window_metrics")
}

// TestAlertPipeline_Lifecycle 测试流水线生命周期
func TestAlertPipeline_Lifecycle(t *testing.T) {
	mockBot := &MockTelegramBot{}
	manager := alert.NewAlertManager(nil, nil, nil)
	config := alert.DefaultPipelineConfig()

	pipeline := alert.NewAlertPipeline(config, manager, mockBot)

	// 初始状态
	assert.False(t, pipeline.IsRunning())

	// 启动
	err := pipeline.Start()
	require.NoError(t, err)
	assert.True(t, pipeline.IsRunning())

	// 重复启动应该失败
	err = pipeline.Start()
	assert.Error(t, err)

	// 停止
	err = pipeline.Stop()
	require.NoError(t, err)
	assert.False(t, pipeline.IsRunning())

	// 重复停止应该失败
	err = pipeline.Stop()
	assert.Error(t, err)
}

// TestAlertTracker_Functionality 测试告警跟踪器功能
func TestAlertTracker_Functionality(t *testing.T) {
	tracker := alert.NewAlertTracker()

	// 创建测试告警
	testAlert := &models.Alert{
		BaseModel: models.BaseModel{
			ID:        uint64(1),
			CreatedAt: time.Now(),
		},
		Type:        models.AlertTypeLargeTransfer,
		Severity:    models.SeverityHigh,
		Title:       "Tracker test",
		Message:     "Tracker test",
		TriggerData: `{"test": true}`,
	}

	// 跟踪告警
	record := tracker.TrackAlert(testAlert)
	assert.NotNil(t, record)
	assert.Equal(t, alert.AlertStatusPending, record.Status)
	assert.NotEmpty(t, record.Hash)

	// 标记为处理中
	tracker.MarkAlertProcessing(testAlert)
	record, exists := tracker.GetAlertRecord(testAlert.ID)
	require.True(t, exists)
	assert.Equal(t, alert.AlertStatusProcessing, record.Status)

	// 标记为已发送
	tracker.MarkAlertSent(testAlert)
	record, exists = tracker.GetAlertRecord(testAlert.ID)
	require.True(t, exists)
	assert.Equal(t, alert.AlertStatusSent, record.Status)
	assert.NotNil(t, record.SentAt)

	// 测试按状态获取告警
	sentAlerts := tracker.GetAlertsByStatus(alert.AlertStatusSent)
	assert.Len(t, sentAlerts, 1)
	assert.Equal(t, testAlert.ID, sentAlerts[0].Alert.ID)

	// 测试统计信息
	stats := tracker.GetStatistics()
	assert.Equal(t, 1, stats["total_records"])
	assert.Equal(t, 1, stats["by_status"].(map[string]int)["sent"])
	assert.Equal(t, 100.0, stats["success_rate"])
}

// TestPipelineMetrics_Functionality 测试流水线指标功能
func TestPipelineMetrics_Functionality(t *testing.T) {
	metrics := alert.NewPipelineMetrics()

	// 测试基本计数器
	metrics.IncrementAlertsReceived()
	metrics.IncrementAlertsReceived()
	metrics.IncrementAlertsProcessed()

	assert.Equal(t, int64(2), metrics.GetAlertsReceived())
	assert.Equal(t, int64(1), metrics.GetAlertsProcessed())
	assert.Equal(t, 50.0, metrics.GetSuccessRate())

	// 测试处理时间
	metrics.UpdateProcessingTime(100 * time.Millisecond)
	metrics.UpdateProcessingTime(200 * time.Millisecond)

	avgTime := metrics.GetAverageProcessingTime()
	assert.Equal(t, 150*time.Millisecond, avgTime)

	maxTime := metrics.GetMaxProcessingTime()
	assert.Equal(t, 200*time.Millisecond, maxTime)

	minTime := metrics.GetMinProcessingTime()
	assert.Equal(t, 100*time.Millisecond, minTime)

	// 测试队列指标
	metrics.UpdateQueueSize(10)
	metrics.UpdateQueueSize(15)
	metrics.UpdateQueueSize(5)

	assert.Equal(t, int64(5), metrics.GetCurrentQueueSize())
	assert.Equal(t, int64(15), metrics.GetMaxQueueSize())

	// 测试摘要
	summary := metrics.GetSummary()
	assert.Contains(t, summary, "alerts")
	assert.Contains(t, summary, "performance")
	assert.Contains(t, summary, "queues")

	// 测试重置
	metrics.Reset()
	assert.Equal(t, int64(0), metrics.GetAlertsReceived())
	assert.Equal(t, int64(0), metrics.GetAlertsProcessed())
}

// BenchmarkAlertPipeline_Processing 性能基准测试
func BenchmarkAlertPipeline_Processing(b *testing.B) {
	mockBot := &MockTelegramBot{}
	manager := alert.NewAlertManager(nil, nil, nil)

	config := &alert.PipelineConfig{
		WorkerCount:         4,
		QueueSize:           1000,
		BatchSize:           10,
		BatchInterval:       10 * time.Millisecond,
		MaxRetries:          1,
		RetryInterval:       1 * time.Millisecond,
		EnableBatching:      false,
		EnableDeduplication: false,
	}

	pipeline := alert.NewAlertPipeline(config, manager, mockBot)

	err := pipeline.Start()
	require.NoError(b, err)
	defer pipeline.Stop()

	// 创建测试告警
	testAlert := &models.Alert{
		BaseModel:   models.BaseModel{ID: 1},
		Type:        models.AlertTypeLargeTransfer,
		Severity:    models.SeverityMedium,
		Message:     "Benchmark test",
		TriggerData: `{"test": true}`,
		CreatedAt:   time.Now(),
	}

	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// 为每次迭代创建新的告警ID
			alert := *testAlert
			alert.ID = uint64(b.N)

			err := pipeline.ProcessAlert(&alert)
			if err != nil {
				b.Error(err)
			}
		}
	})

	// 等待所有告警处理完成
	time.Sleep(100 * time.Millisecond)

	b.StopTimer()

	// 验证处理结果
	metrics := pipeline.GetMetrics()
	b.Logf("Processed %d alerts, Success rate: %.2f%%",
		metrics.GetAlertsProcessed(), metrics.GetSuccessRate())
}
