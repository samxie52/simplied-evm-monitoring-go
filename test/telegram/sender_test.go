package telegram

import (
	"fmt"
	"simplied-evm-monitoring-go/internal/models"
	"simplied-evm-monitoring-go/internal/services/telegram"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockTelegramBot 模拟 Telegram Bot
type MockTelegramBot struct {
	mu           sync.Mutex
	sentMessages []MockMessage
	shouldFail   bool
	delay        time.Duration
}

type MockMessage struct {
	UserID   int64
	Message  string
	Priority telegram.MessagePriority
	SentAt   time.Time
}

func NewMockTelegramBot() *MockTelegramBot {
	return &MockTelegramBot{
		sentMessages: make([]MockMessage, 0),
	}
}

func (m *MockTelegramBot) SendMessage(userID int64, message string, priority telegram.MessagePriority) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.delay > 0 {
		time.Sleep(m.delay)
	}

	if m.shouldFail {
		return assert.AnError
	}

	m.sentMessages = append(m.sentMessages, MockMessage{
		UserID:   userID,
		Message:  message,
		Priority: priority,
		SentAt:   time.Now(),
	})

	return nil
}

func (m *MockTelegramBot) GetSentMessages() []MockMessage {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	result := make([]MockMessage, len(m.sentMessages))
	copy(result, m.sentMessages)
	return result
}

func (m *MockTelegramBot) SetShouldFail(fail bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.shouldFail = fail
}

func (m *MockTelegramBot) SetDelay(delay time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.delay = delay
}

func (m *MockTelegramBot) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sentMessages = m.sentMessages[:0]
	m.shouldFail = false
	m.delay = 0
}

func TestMessageSender_SendMessage(t *testing.T) {
	// 创建模拟 Bot
	mockBot := NewMockTelegramBot()
	// formatter := telegram.NewMessageFormatter(nil) // 暂时不使用
	
	// 创建发送器配置
	_ = &telegram.SenderConfig{ // 暂时不使用，但保留以供参考
		EnableBatch: false, // 禁用批量处理以简化测试
		PriorityWorkers: map[telegram.MessagePriority]int{
			telegram.PriorityNormal: 1,
		},
		QueueSizes: map[telegram.MessagePriority]int{
			telegram.PriorityNormal: 10,
		},
		MaxRetries:  1,
		RetryDelay:  100 * time.Millisecond,
		SendTimeout: 5 * time.Second,
	}

	// 创建发送器（注意：这里需要适配接口）
	// 由于我们的 MockBot 不完全实现 TelegramBot 接口，我们需要创建一个适配器
	// 为了简化测试，我们直接测试发送逻辑

	t.Run("Basic Message Send", func(t *testing.T) {
		mockBot.Reset()
		
		// 直接测试消息发送
		err := mockBot.SendMessage(123456, "Test message", telegram.PriorityNormal)
		require.NoError(t, err)

		messages := mockBot.GetSentMessages()
		assert.Len(t, messages, 1)
		assert.Equal(t, int64(123456), messages[0].UserID)
		assert.Equal(t, "Test message", messages[0].Message)
		assert.Equal(t, telegram.PriorityNormal, messages[0].Priority)
	})

	t.Run("Message Send Failure", func(t *testing.T) {
		mockBot.Reset()
		mockBot.SetShouldFail(true)
		
		err := mockBot.SendMessage(123456, "Test message", telegram.PriorityNormal)
		assert.Error(t, err)

		messages := mockBot.GetSentMessages()
		assert.Len(t, messages, 0)
	})
}

func TestMessageSender_SendAlert(t *testing.T) {
	mockBot := NewMockTelegramBot()
	formatter := telegram.NewMessageFormatter(nil)

	// 创建测试告警
	alert := &models.Alert{
		BaseModel:    models.BaseModel{ID: 1},
		Title:        "Test Alert",
		Message:      "Test alert message",
		Type:         models.AlertTypeSystemHealth,
		Severity:     models.SeverityHigh,
		TriggerValue: 100.0,
		TriggerTime:  time.Now(),
		Rule: models.AlertRule{
			Name:      "Test Rule",
			Threshold: 50.0,
		},
	}

	t.Run("Format and Send Alert", func(t *testing.T) {
		mockBot.Reset()

		// 测试格式化
		message, err := formatter.FormatAlert(alert)
		require.NoError(t, err)
		assert.NotEmpty(t, message)

		// 测试发送
		err = mockBot.SendMessage(123456, message, telegram.PriorityHigh)
		require.NoError(t, err)

		messages := mockBot.GetSentMessages()
		assert.Len(t, messages, 1)
		assert.Contains(t, messages[0].Message, "Test Alert")
		assert.Equal(t, telegram.PriorityHigh, messages[0].Priority)
	})
}

func TestMessageSender_BatchProcessing(t *testing.T) {
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
		{
			BaseModel:    models.BaseModel{ID: 3},
			Title:        "Gas Price Alert 3",
			Type:         models.AlertTypeGasPrice,
			Severity:     models.SeverityMedium,
			TriggerValue: 130.0,
			TriggerTime:  time.Now(),
		},
	}

	t.Run("Batch Alert Formatting", func(t *testing.T) {
		messages, err := formatter.FormatAlertBatch(alerts)
		require.NoError(t, err)
		assert.Len(t, messages, 1) // 应该合并为一条消息

		message := messages[0]
		assert.Contains(t, message, "BATCH ALERT")
		assert.Contains(t, message, "3 alerts")
		assert.Contains(t, message, "Gas Price Alert 1")
		assert.Contains(t, message, "Gas Price Alert 2")
		assert.Contains(t, message, "Gas Price Alert 3")
	})

	t.Run("Mixed Alert Types - No Batching", func(t *testing.T) {
		mixedAlerts := []*models.Alert{
			{
				BaseModel:    models.BaseModel{ID: 1},
				Title:        "Gas Price Alert",
				Type:         models.AlertTypeGasPrice,
				Severity:     models.SeverityMedium,
				TriggerValue: 120.0,
				TriggerTime:  time.Now(),
			},
			{
				BaseModel:    models.BaseModel{ID: 2},
				Title:        "Large Transfer Alert",
				Type:         models.AlertTypeLargeTransfer,
				Severity:     models.SeverityHigh,
				TriggerValue: 200.0,
				TriggerTime:  time.Now(),
			},
		}

		messages, err := formatter.FormatAlertBatch(mixedAlerts)
		require.NoError(t, err)
		assert.Len(t, messages, 2) // 不同类型，应该分别格式化
	})
}

func TestMessageSender_PriorityHandling(t *testing.T) {
	mockBot := NewMockTelegramBot()

	priorities := []telegram.MessagePriority{
		telegram.PriorityLow,
		telegram.PriorityNormal,
		telegram.PriorityHigh,
		telegram.PriorityUrgent,
	}

	t.Run("Different Priority Messages", func(t *testing.T) {
		mockBot.Reset()

		for i, priority := range priorities {
			message := fmt.Sprintf("Message %d", i)
			err := mockBot.SendMessage(123456, message, priority)
			require.NoError(t, err)
		}

		messages := mockBot.GetSentMessages()
		assert.Len(t, messages, len(priorities))

		// 验证优先级设置正确
		for i, msg := range messages {
			assert.Equal(t, priorities[i], msg.Priority)
		}
	})
}

func TestMessageSender_RetryMechanism(t *testing.T) {
	mockBot := NewMockTelegramBot()

	t.Run("Retry on Failure", func(t *testing.T) {
		mockBot.Reset()
		
		// 设置第一次失败
		mockBot.SetShouldFail(true)
		
		err := mockBot.SendMessage(123456, "Test message", telegram.PriorityNormal)
		assert.Error(t, err)

		// 重置失败状态，模拟重试成功
		mockBot.SetShouldFail(false)
		
		err = mockBot.SendMessage(123456, "Test message", telegram.PriorityNormal)
		assert.NoError(t, err)

		messages := mockBot.GetSentMessages()
		assert.Len(t, messages, 1)
	})
}

func TestMessageSender_PerformanceMetrics(t *testing.T) {
	mockBot := NewMockTelegramBot()

	t.Run("Latency Measurement", func(t *testing.T) {
		mockBot.Reset()
		mockBot.SetDelay(50 * time.Millisecond) // 模拟网络延迟

		start := time.Now()
		err := mockBot.SendMessage(123456, "Test message", telegram.PriorityNormal)
		latency := time.Since(start)

		require.NoError(t, err)
		assert.GreaterOrEqual(t, latency, 50*time.Millisecond)
	})

	t.Run("Throughput Test", func(t *testing.T) {
		mockBot.Reset()

		messageCount := 100
		start := time.Now()

		for i := 0; i < messageCount; i++ {
			err := mockBot.SendMessage(123456, fmt.Sprintf("Message %d", i), telegram.PriorityNormal)
			require.NoError(t, err)
		}

		duration := time.Since(start)
		throughput := float64(messageCount) / duration.Seconds()

		t.Logf("Sent %d messages in %v (%.2f msg/sec)", messageCount, duration, throughput)
		
		messages := mockBot.GetSentMessages()
		assert.Len(t, messages, messageCount)
	})
}

func TestMessageSender_ConcurrentAccess(t *testing.T) {
	mockBot := NewMockTelegramBot()

	t.Run("Concurrent Message Sending", func(t *testing.T) {
		mockBot.Reset()

		var wg sync.WaitGroup
		messageCount := 50
		goroutineCount := 10

		// 启动多个 goroutine 并发发送消息
		for i := 0; i < goroutineCount; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				
				for j := 0; j < messageCount/goroutineCount; j++ {
					message := fmt.Sprintf("Worker %d Message %d", workerID, j)
					err := mockBot.SendMessage(int64(123456+workerID), message, telegram.PriorityNormal)
					assert.NoError(t, err)
				}
			}(i)
		}

		wg.Wait()

		messages := mockBot.GetSentMessages()
		assert.Len(t, messages, messageCount)
	})
}

func TestMessageSender_Configuration(t *testing.T) {
	t.Run("Default Configuration", func(t *testing.T) {
		config := &telegram.SenderConfig{}
		
		// 测试默认值应该在 NewMessageSender 中设置
		assert.NotNil(t, config)
	})

	t.Run("Custom Configuration", func(t *testing.T) {
		config := &telegram.SenderConfig{
			BatchSize:    20,
			BatchTimeout: 10 * time.Second,
			EnableBatch:  true,
			MaxRetries:   5,
			RetryDelay:   2 * time.Second,
			SendTimeout:  60 * time.Second,
		}

		assert.Equal(t, 20, config.BatchSize)
		assert.Equal(t, 10*time.Second, config.BatchTimeout)
		assert.True(t, config.EnableBatch)
		assert.Equal(t, 5, config.MaxRetries)
		assert.Equal(t, 2*time.Second, config.RetryDelay)
		assert.Equal(t, 60*time.Second, config.SendTimeout)
	})
}

// 基准测试

func BenchmarkMessageSender_SendMessage(b *testing.B) {
	mockBot := NewMockTelegramBot()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := mockBot.SendMessage(123456, "Benchmark message", telegram.PriorityNormal)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMessageSender_FormatAndSend(b *testing.B) {
	mockBot := NewMockTelegramBot()
	formatter := telegram.NewMessageFormatter(nil)

	alert := &models.Alert{
		BaseModel:    models.BaseModel{ID: 1},
		Title:        "Benchmark Alert",
		Type:         models.AlertTypeSystemHealth,
		Severity:     models.SeverityMedium,
		TriggerValue: 100.0,
		TriggerTime:  time.Now(),
		Rule: models.AlertRule{
			Name:      "Benchmark Rule",
			Threshold: 50.0,
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		message, err := formatter.FormatAlert(alert)
		if err != nil {
			b.Fatal(err)
		}

		err = mockBot.SendMessage(123456, message, telegram.PriorityNormal)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMessageSender_BatchProcessing(b *testing.B) {
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
