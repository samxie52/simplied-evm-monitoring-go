package alert

import (
	"testing"
	"time"

	"simplied-evm-monitoring-go/internal/models"
	"simplied-evm-monitoring-go/internal/services/alert"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAlertTracker_Basic 测试基本的告警跟踪功能
func TestAlertTracker_Basic(t *testing.T) {
	tracker := alert.NewAlertTracker()

	// 创建测试告警
	testAlert := &models.Alert{
		BaseModel: models.BaseModel{
			ID:        uint64(1),
			CreatedAt: time.Now(),
		},
		Type:        models.AlertTypeLargeTransfer,
		Severity:    models.SeverityHigh,
		Title:       "Test Alert",
		Message:     "Test message",
		TriggerData: `{"test": true}`,
	}

	// 跟踪告警
	record := tracker.TrackAlert(testAlert)
	require.NotNil(t, record)

	// 验证告警记录
	record, exists := tracker.GetAlertRecord(testAlert.ID)
	require.True(t, exists)
	assert.Equal(t, testAlert.ID, record.Alert.ID)
	assert.Equal(t, alert.AlertStatusPending, record.Status)
	assert.NotNil(t, record.CreatedAt)

	// 标记为已发送
	tracker.MarkAlertSent(testAlert)

	// 验证状态更新
	record, exists = tracker.GetAlertRecord(testAlert.ID)
	require.True(t, exists)
	assert.Equal(t, alert.AlertStatusSent, record.Status)
	assert.NotNil(t, record.SentAt)
}

// TestAlertTracker_Deduplication 测试去重功能
func TestAlertTracker_Deduplication(t *testing.T) {
	tracker := alert.NewAlertTracker()

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

	// 跟踪第一个告警
	record1 := tracker.TrackAlert(alert1)
	require.NotNil(t, record1)

	// 检查第二个告警是否重复
	isDuplicate := tracker.IsDuplicate(alert2)
	assert.True(t, isDuplicate)

	// 跟踪第二个告警
	record2 := tracker.TrackAlert(alert2)
	require.NotNil(t, record2)

	// 标记为重复
	tracker.MarkAlertDuplicate(alert2)

	record, exists := tracker.GetAlertRecord(alert2.ID)
	require.True(t, exists)
	assert.Equal(t, alert.AlertStatusDuplicate, record.Status)
}

// TestPipelineMetrics_Basic 测试基本的指标收集
func TestPipelineMetrics_Basic(t *testing.T) {
	metrics := alert.NewPipelineMetrics()

	// 测试告警计数
	metrics.IncrementAlertsReceived()
	metrics.IncrementAlertsReceived()
	metrics.IncrementAlertsProcessed()

	assert.Equal(t, int64(2), metrics.GetAlertsReceived())
	assert.Equal(t, int64(1), metrics.GetAlertsProcessed())
	assert.Equal(t, int64(0), metrics.GetAlertsDropped())

	// 测试批量处理统计
	metrics.IncrementBatchesProcessed()

	assert.Equal(t, int64(1), metrics.GetBatchesProcessed())

	// 测试处理时间
	processingTime := 100 * time.Millisecond
	metrics.UpdateProcessingTime(processingTime)

	assert.Equal(t, processingTime, metrics.GetAverageProcessingTime())
}

// TestAlertManager_Basic 测试基本的告警管理功能
func TestAlertManager_Basic(t *testing.T) {
	// 创建基本的告警管理器配置
	config := &alert.AlertManagerConfig{
		WorkerCount:     2,
		AlertQueueSize:  100,
		ProcessTimeout:  time.Second * 30,
		BatchSize:       10,
		BatchInterval:   time.Minute,
	}

	// 创建告警管理器
	manager := alert.NewAlertManager(config, nil, nil)
	require.NotNil(t, manager)

	// 验证配置
	assert.Equal(t, 2, config.WorkerCount)
	assert.Equal(t, 100, config.AlertQueueSize)
}
