package telegram

import (
	"simplied-evm-monitoring-go/internal/models"
	"simplied-evm-monitoring-go/internal/services/telegram"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMessageFormatter_FormatAlert(t *testing.T) {
	// 创建格式化器
	config := &telegram.FormatterConfig{
		EnableMarkdown:          true,
		IncludeTransactionLinks: true,
		EtherscanBaseURL:        "https://etherscan.io",
		EnableEmojis:            true,
		MaxMessageLength:        2000,
	}
	formatter := telegram.NewMessageFormatter(config)

	// 创建测试告警
	alert := &models.Alert{
		BaseModel:    models.BaseModel{ID: 1},
		Title:        "Large Transfer Detected",
		Message:      "Large ETH transfer detected",
		Type:         models.AlertTypeLargeTransfer,
		Severity:     models.SeverityHigh,
		TriggerValue: 150.5,
		TriggerTime:  time.Now(),
		Status:       models.NotificationStatusPending,
		Rule: models.AlertRule{
			BaseModel: models.BaseModel{ID: 1},
			Name:      "Large Transfer Monitor",
			Threshold: 100.0,
			Operator:  models.OpGreaterThan,
		},
	}

	// 测试格式化
	message, err := formatter.FormatAlert(alert)
	require.NoError(t, err)
	assert.NotEmpty(t, message)

	// 验证消息内容
	assert.Contains(t, message, "LARGE TRANSFER ALERT")
	assert.Contains(t, message, "150.5")
	assert.Contains(t, message, "Large Transfer Monitor")
	assert.Contains(t, message, "⚠️") // 高严重级别的表情符号
}

func TestMessageFormatter_FormatAlertBatch(t *testing.T) {
	formatter := telegram.NewMessageFormatter(nil)

	// 创建多个相似告警
	alerts := []*models.Alert{
		{
			BaseModel:    models.BaseModel{ID: 1},
			Title:        "Gas Price Alert 1",
			Type:         models.AlertTypeGasPrice,
			Severity:     models.SeverityMedium,
			TriggerValue: 120.0,
			TriggerTime:  time.Now(),
		},
		{
			BaseModel:    models.BaseModel{ID: 2},
			Title:        "Gas Price Alert 2",
			Type:         models.AlertTypeGasPrice,
			Severity:     models.SeverityMedium,
			TriggerValue: 125.0,
			TriggerTime:  time.Now(),
		},
	}

	// 测试批量格式化
	messages, err := formatter.FormatAlertBatch(alerts)
	require.NoError(t, err)
	assert.Len(t, messages, 1) // 应该合并为一条消息

	message := messages[0]
	assert.Contains(t, message, "BATCH ALERT")
	assert.Contains(t, message, "2 alerts")
}

func TestMessageFormatter_DifferentAlertTypes(t *testing.T) {
	formatter := telegram.NewMessageFormatter(nil)

	testCases := []struct {
		name      string
		alertType models.AlertType
		expected  string
	}{
		{
			name:      "Large Transfer",
			alertType: models.AlertTypeLargeTransfer,
			expected:  "LARGE TRANSFER ALERT",
		},
		{
			name:      "Gas Price",
			alertType: models.AlertTypeGasPrice,
			expected:  "GAS PRICE ALERT",
		},
		{
			name:      "Network Congestion",
			alertType: models.AlertTypeNetworkCongestion,
			expected:  "NETWORK CONGESTION ALERT",
		},
		{
			name:      "Contract Event",
			alertType: models.AlertTypeContractEvent,
			expected:  "CONTRACT EVENT ALERT",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			alert := &models.Alert{
				BaseModel:    models.BaseModel{ID: 1},
				Title:        "Test Alert",
				Type:         tc.alertType,
				Severity:     models.SeverityMedium,
				TriggerValue: 100.0,
				TriggerTime:  time.Now(),
			}

			message, err := formatter.FormatAlert(alert)
			require.NoError(t, err)
			assert.Contains(t, message, tc.expected)
		})
	}
}

func TestMessageFormatter_SeverityEmojis(t *testing.T) {
	config := &telegram.FormatterConfig{
		EnableEmojis: true,
	}
	formatter := telegram.NewMessageFormatter(config)

	testCases := []struct {
		severity models.AlertSeverity
		emoji    string
	}{
		{models.SeverityCritical, "🚨"},
		{models.SeverityHigh, "⚠️"},
		{models.SeverityMedium, "⚡"},
		{models.SeverityLow, "ℹ️"},
	}

	for _, tc := range testCases {
		t.Run(string(tc.severity), func(t *testing.T) {
			alert := &models.Alert{
				BaseModel:    models.BaseModel{ID: 1},
				Title:        "Test Alert",
				Type:         models.AlertTypeSystemHealth,
				Severity:     tc.severity,
				TriggerValue: 100.0,
				TriggerTime:  time.Now(),
			}

			message, err := formatter.FormatAlert(alert)
			require.NoError(t, err)
			assert.Contains(t, message, tc.emoji)
		})
	}
}

func TestMessageFormatter_DisableEmojis(t *testing.T) {
	config := &telegram.FormatterConfig{
		EnableEmojis: false,
	}
	formatter := telegram.NewMessageFormatter(config)

	alert := &models.Alert{
		BaseModel:    models.BaseModel{ID: 1},
		Title:        "Test Alert",
		Type:         models.AlertTypeSystemHealth,
		Severity:     models.SeverityCritical,
		TriggerValue: 100.0,
		TriggerTime:  time.Now(),
	}

	message, err := formatter.FormatAlert(alert)
	require.NoError(t, err)

	// 不应该包含表情符号
	assert.NotContains(t, message, "🚨")
	assert.NotContains(t, message, "⚠️")
}

func TestMessageFormatter_DisableMarkdown(t *testing.T) {
	config := &telegram.FormatterConfig{
		EnableMarkdown: false,
	}
	formatter := telegram.NewMessageFormatter(config)

	alert := &models.Alert{
		BaseModel:    models.BaseModel{ID: 1},
		Title:        "Test Alert",
		Type:         models.AlertTypeSystemHealth,
		Severity:     models.SeverityCritical,
		TriggerValue: 100.0,
		TriggerTime:  time.Now(),
	}

	message, err := formatter.FormatAlert(alert)
	require.NoError(t, err)

	// 不应该包含 Markdown 格式
	assert.NotContains(t, message, "**")
	assert.NotContains(t, message, "*")
}

func TestMessageFormatter_TransactionLinks(t *testing.T) {
	config := &telegram.FormatterConfig{
		IncludeTransactionLinks: true,
		EtherscanBaseURL:        "https://etherscan.io",
		MaxMessageLength:        2000, // 设置较大的最大长度
	}
	formatter := telegram.NewMessageFormatter(config)

	alert := &models.Alert{
		BaseModel:    models.BaseModel{ID: 1},
		Title:        "Test Alert",
		Type:         models.AlertTypeLargeTransfer,
		Severity:     models.SeverityHigh,
		TriggerValue: 100.0,
		TriggerTime:  time.Now(),
		TriggerData:  `{"source_type":"transaction","source_id":"0x1234567890abcdef"}`,
	}

	message, err := formatter.FormatAlert(alert)
	require.NoError(t, err)

	// 应该包含交易链接
	assert.Contains(t, message, "etherscan.io/tx/0x1234567890abcdef")
}

func TestMessageFormatter_MessageTruncation(t *testing.T) {
	config := &telegram.FormatterConfig{
		MaxMessageLength: 100, // 设置很小的最大长度
	}
	formatter := telegram.NewMessageFormatter(config)

	alert := &models.Alert{
		BaseModel:    models.BaseModel{ID: 1},
		Title:        "Very Long Alert Title That Should Be Truncated Because It Exceeds The Maximum Length",
		Message:      "This is a very long message that should definitely be truncated when the formatter processes it",
		Type:         models.AlertTypeSystemHealth,
		Severity:     models.SeverityCritical,
		TriggerValue: 100.0,
		TriggerTime:  time.Now(),
	}

	message, err := formatter.FormatAlert(alert)
	require.NoError(t, err)

	// 消息应该被截断
	assert.LessOrEqual(t, len(message), config.MaxMessageLength)
	assert.Contains(t, message, "truncated")
}

func TestMessageFormatter_RuleStatus(t *testing.T) {
	formatter := telegram.NewMessageFormatter(nil)

	rule := &models.AlertRule{
		BaseModel:    models.BaseModel{ID: 1, CreatedAt: time.Now().Add(-24 * time.Hour), UpdatedAt: time.Now()},
		Name:         "Test Rule",
		Type:         models.AlertTypeGasPrice,
		Status:       models.AlertStatusActive,
		Severity:     models.SeverityMedium,
		Threshold:    100.0,
		Operator:     models.OpGreaterThan,
		TriggerCount: 5,
	}

	message, err := formatter.FormatRuleStatus(rule)
	require.NoError(t, err)
	assert.NotEmpty(t, message)

	assert.Contains(t, message, "RULE STATUS")
	assert.Contains(t, message, "Test Rule")
	assert.Contains(t, message, "100")
	assert.Contains(t, message, "5") // trigger count
}

func TestMessageFormatter_SystemStatus(t *testing.T) {
	formatter := telegram.NewMessageFormatter(nil)

	stats := map[string]interface{}{
		"active_rules":     10,
		"total_alerts":     100,
		"processed_alerts": 95,
		"failed_alerts":    5,
		"success_rate":     95.0,
		"total_messages":   200,
		"sent_messages":    190,
		"failed_messages":  10,
		"total_commands":   50,
		"uptime":           "24h30m",
		"avg_response_time": 25,
		"queue_size":       5,
		"memory_usage":     128,
		"cpu_usage":        15,
		"timestamp":        time.Now().Format("2006-01-02 15:04:05"),
	}

	message, err := formatter.FormatSystemStatus(stats)
	require.NoError(t, err)
	assert.NotEmpty(t, message)

	assert.Contains(t, message, "SYSTEM STATUS")
	assert.Contains(t, message, "10")   // active rules
	assert.Contains(t, message, "95%") // success rate
}

func TestMessageFormatter_NilAlert(t *testing.T) {
	formatter := telegram.NewMessageFormatter(nil)

	message, err := formatter.FormatAlert(nil)
	assert.Error(t, err)
	assert.Empty(t, message)
	assert.Contains(t, err.Error(), "alert cannot be nil")
}

func TestMessageFormatter_EmptyAlertBatch(t *testing.T) {
	formatter := telegram.NewMessageFormatter(nil)

	messages, err := formatter.FormatAlertBatch([]*models.Alert{})
	assert.Error(t, err)
	assert.Nil(t, messages)
	assert.Contains(t, err.Error(), "alerts list cannot be empty")
}

// 基准测试

func BenchmarkMessageFormatter_FormatAlert(b *testing.B) {
	formatter := telegram.NewMessageFormatter(nil)

	alert := &models.Alert{
		BaseModel:    models.BaseModel{ID: 1},
		Title:        "Benchmark Alert",
		Type:         models.AlertTypeLargeTransfer,
		Severity:     models.SeverityHigh,
		TriggerValue: 100.0,
		TriggerTime:  time.Now(),
		Rule: models.AlertRule{
			Name:      "Benchmark Rule",
			Threshold: 50.0,
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := formatter.FormatAlert(alert)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMessageFormatter_FormatAlertBatch(b *testing.B) {
	formatter := telegram.NewMessageFormatter(nil)

	alerts := make([]*models.Alert, 10)
	for i := 0; i < 10; i++ {
		alerts[i] = &models.Alert{
			BaseModel:    models.BaseModel{ID: uint64(i + 1)},
			Title:        "Benchmark Alert",
			Type:         models.AlertTypeGasPrice,
			Severity:     models.SeverityMedium,
			TriggerValue: float64(100 + i),
			TriggerTime:  time.Now(),
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := formatter.FormatAlertBatch(alerts)
		if err != nil {
			b.Fatal(err)
		}
	}
}
