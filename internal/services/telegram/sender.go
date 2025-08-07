package telegram

import (
	"context"
	"fmt"
	"simplied-evm-monitoring-go/internal/models"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// MessageSender 消息发送器
type MessageSender struct {
	mu        sync.RWMutex
	bot       *TelegramBot
	formatter *MessageFormatter
	config    *SenderConfig
	stats     *SenderStats
	
	// 批量发送相关
	batchQueue    chan *BatchItem
	batchTimer    *time.Timer
	currentBatch  []*BatchItem
	batchMutex    sync.Mutex
	
	// 优先级队列
	priorityQueues map[MessagePriority]chan *SendRequest
	
	// 控制
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// SenderConfig 发送器配置
type SenderConfig struct {
	// 批量发送配置
	BatchSize     int
	BatchTimeout  time.Duration
	EnableBatch   bool
	
	// 优先级配置
	PriorityWorkers map[MessagePriority]int
	QueueSizes      map[MessagePriority]int
	
	// 重试配置
	MaxRetries    int
	RetryDelay    time.Duration
	RetryBackoff  float64
	
	// 限流配置
	RateLimit     int // 每秒消息数
	BurstLimit    int
	
	// 超时配置
	SendTimeout   time.Duration
}

// SenderStats 发送器统计
type SenderStats struct {
	mu sync.RWMutex
	
	// 发送统计
	TotalSent      int64
	TotalFailed    int64
	TotalBatched   int64
	TotalRetries   int64
	
	// 优先级统计
	PrioritySent   map[MessagePriority]int64
	PriorityFailed map[MessagePriority]int64
	
	// 性能统计
	AverageLatency time.Duration
	MaxLatency     time.Duration
	MinLatency     time.Duration
	
	// 队列统计
	QueueSizes     map[MessagePriority]int
	MaxQueueSize   int
	
	StartTime      time.Time
}

// SendRequest 发送请求
type SendRequest struct {
	UserID    int64
	Message   string
	Priority  MessagePriority
	Alert     *models.Alert
	Metadata  map[string]interface{}
	Timestamp time.Time
	Retries   int
	Callback  func(error)
}

// BatchItem 批量发送项
type BatchItem struct {
	Request   *SendRequest
	Timestamp time.Time
}

// NewMessageSender 创建新的消息发送器
func NewMessageSender(bot *TelegramBot, formatter *MessageFormatter, config *SenderConfig) *MessageSender {
	if config == nil {
		config = &SenderConfig{
			BatchSize:    10,
			BatchTimeout: 30 * time.Second,
			EnableBatch:  true,
			PriorityWorkers: map[MessagePriority]int{
				PriorityUrgent: 5,
				PriorityHigh:   3,
				PriorityNormal: 2,
				PriorityLow:    1,
			},
			QueueSizes: map[MessagePriority]int{
				PriorityUrgent: 1000,
				PriorityHigh:   500,
				PriorityNormal: 200,
				PriorityLow:    100,
			},
			MaxRetries:   3,
			RetryDelay:   time.Second,
			RetryBackoff: 2.0,
			RateLimit:    30,
			BurstLimit:   10,
			SendTimeout:  30 * time.Second,
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	
	sender := &MessageSender{
		bot:       bot,
		formatter: formatter,
		config:    config,
		ctx:       ctx,
		cancel:    cancel,
		stats: &SenderStats{
			PrioritySent:   make(map[MessagePriority]int64),
			PriorityFailed: make(map[MessagePriority]int64),
			QueueSizes:     make(map[MessagePriority]int),
			StartTime:      time.Now(),
			MinLatency:     time.Hour, // 初始化为一个大值
		},
		priorityQueues: make(map[MessagePriority]chan *SendRequest),
		batchQueue:     make(chan *BatchItem, 1000),
		currentBatch:   make([]*BatchItem, 0, config.BatchSize),
	}

	// 初始化优先级队列
	for priority, size := range config.QueueSizes {
		sender.priorityQueues[priority] = make(chan *SendRequest, size)
	}

	return sender
}

// Start 启动消息发送器
func (s *MessageSender) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	logrus.Info("Starting message sender...")

	// 启动优先级工作器
	for priority, workerCount := range s.config.PriorityWorkers {
		for i := 0; i < workerCount; i++ {
			s.wg.Add(1)
			go s.priorityWorker(priority, i)
		}
		logrus.WithFields(logrus.Fields{
			"priority": priority,
			"workers":  workerCount,
		}).Info("Started priority workers")
	}

	// 启动批量处理器
	if s.config.EnableBatch {
		s.wg.Add(1)
		go s.batchProcessor()
		logrus.Info("Started batch processor")
	}

	// 启动统计更新器
	s.wg.Add(1)
	go s.statsUpdater()

	logrus.Info("Message sender started successfully")
	return nil
}

// Stop 停止消息发送器
func (s *MessageSender) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	logrus.Info("Stopping message sender...")

	// 取消上下文
	s.cancel()

	// 等待所有工作器完成
	s.wg.Wait()

	// 关闭队列
	for _, queue := range s.priorityQueues {
		close(queue)
	}
	close(s.batchQueue)

	logrus.Info("Message sender stopped")
	return nil
}

// SendAlert 发送告警消息
func (s *MessageSender) SendAlert(alert *models.Alert, userIDs []int64, priority MessagePriority) error {
	if alert == nil {
		return fmt.Errorf("alert cannot be nil")
	}

	// 格式化消息
	message, err := s.formatter.FormatAlert(alert)
	if err != nil {
		return fmt.Errorf("failed to format alert message: %w", err)
	}

	// 发送给所有用户
	for _, userID := range userIDs {
		request := &SendRequest{
			UserID:    userID,
			Message:   message,
			Priority:  priority,
			Alert:     alert,
			Timestamp: time.Now(),
		}

		if err := s.enqueueMessage(request); err != nil {
			logrus.WithError(err).WithFields(logrus.Fields{
				"user_id":  userID,
				"alert_id": alert.ID,
			}).Error("Failed to enqueue alert message")
		}
	}

	return nil
}

// SendAlertBatch 批量发送告警消息
func (s *MessageSender) SendAlertBatch(alerts []*models.Alert, userIDs []int64, priority MessagePriority) error {
	if len(alerts) == 0 {
		return fmt.Errorf("alerts list cannot be empty")
	}

	// 格式化批量消息
	messages, err := s.formatter.FormatAlertBatch(alerts)
	if err != nil {
		return fmt.Errorf("failed to format batch alert messages: %w", err)
	}

	// 发送给所有用户
	for _, userID := range userIDs {
		for _, message := range messages {
			request := &SendRequest{
				UserID:    userID,
				Message:   message,
				Priority:  priority,
				Timestamp: time.Now(),
				Metadata: map[string]interface{}{
					"batch_size": len(alerts),
					"batch_type": "alert",
				},
			}

			if err := s.enqueueMessage(request); err != nil {
				logrus.WithError(err).WithField("user_id", userID).Error("Failed to enqueue batch message")
			}
		}
	}

	s.updateBatchStats(int64(len(alerts)))
	return nil
}

// SendMessage 发送普通消息
func (s *MessageSender) SendMessage(userID int64, message string, priority MessagePriority) error {
	request := &SendRequest{
		UserID:    userID,
		Message:   message,
		Priority:  priority,
		Timestamp: time.Now(),
	}

	return s.enqueueMessage(request)
}

// SendMessageWithCallback 发送消息并指定回调
func (s *MessageSender) SendMessageWithCallback(userID int64, message string, priority MessagePriority, callback func(error)) error {
	request := &SendRequest{
		UserID:    userID,
		Message:   message,
		Priority:  priority,
		Timestamp: time.Now(),
		Callback:  callback,
	}

	return s.enqueueMessage(request)
}

// enqueueMessage 将消息加入队列
func (s *MessageSender) enqueueMessage(request *SendRequest) error {
	queue, exists := s.priorityQueues[request.Priority]
	if !exists {
		return fmt.Errorf("unknown priority: %v", request.Priority)
	}

	select {
	case queue <- request:
		s.updateQueueStats(request.Priority, 1)
		return nil
	case <-s.ctx.Done():
		return fmt.Errorf("sender is shutting down")
	default:
		return fmt.Errorf("queue is full for priority: %v", request.Priority)
	}
}

// priorityWorker 优先级工作器
func (s *MessageSender) priorityWorker(priority MessagePriority, workerID int) {
	defer s.wg.Done()

	logrus.WithFields(logrus.Fields{
		"priority":  priority,
		"worker_id": workerID,
	}).Info("Priority worker started")

	queue := s.priorityQueues[priority]
	
	for {
		select {
		case request := <-queue:
			if request != nil {
				s.processMessage(request)
				s.updateQueueStats(priority, -1)
			}
		case <-s.ctx.Done():
			logrus.WithFields(logrus.Fields{
				"priority":  priority,
				"worker_id": workerID,
			}).Info("Priority worker stopped")
			return
		}
	}
}

// processMessage 处理单个消息
func (s *MessageSender) processMessage(request *SendRequest) {
	start := time.Now()
	
	// 检查是否应该批量处理
	if s.config.EnableBatch && s.shouldBatch(request) {
		s.addToBatch(request)
		return
	}

	// 直接发送
	err := s.sendMessage(request)
	
	// 更新统计
	latency := time.Since(start)
	s.updateLatencyStats(latency)
	
	if err != nil {
		s.handleSendError(request, err)
	} else {
		s.updateSendStats(request.Priority, true)
	}

	// 执行回调
	if request.Callback != nil {
		request.Callback(err)
	}
}

// sendMessage 发送消息到 Telegram
func (s *MessageSender) sendMessage(request *SendRequest) error {
	// 使用 Bot 发送消息
	err := s.bot.SendMessage(request.UserID, request.Message)
	if err != nil {
		return fmt.Errorf("failed to send message via bot: %w", err)
	}

	return nil
}

// shouldBatch 检查是否应该批量处理
func (s *MessageSender) shouldBatch(request *SendRequest) bool {
	// 紧急消息不批量处理
	if request.Priority == PriorityUrgent {
		return false
	}

	// 如果是告警消息且启用了批量处理
	return request.Alert != nil && s.config.EnableBatch
}

// addToBatch 添加到批量队列
func (s *MessageSender) addToBatch(request *SendRequest) {
	item := &BatchItem{
		Request:   request,
		Timestamp: time.Now(),
	}

	select {
	case s.batchQueue <- item:
		// 成功添加到批量队列
	case <-s.ctx.Done():
		// 系统关闭，直接发送
		s.sendMessage(request)
	default:
		// 批量队列满了，直接发送
		s.sendMessage(request)
	}
}

// batchProcessor 批量处理器
func (s *MessageSender) batchProcessor() {
	defer s.wg.Done()

	logrus.Info("Batch processor started")
	
	ticker := time.NewTicker(s.config.BatchTimeout)
	defer ticker.Stop()

	for {
		select {
		case item := <-s.batchQueue:
			s.batchMutex.Lock()
			s.currentBatch = append(s.currentBatch, item)
			
			// 检查是否达到批量大小
			if len(s.currentBatch) >= s.config.BatchSize {
				s.processBatch()
			}
			s.batchMutex.Unlock()

		case <-ticker.C:
			// 定时处理批量
			s.batchMutex.Lock()
			if len(s.currentBatch) > 0 {
				s.processBatch()
			}
			s.batchMutex.Unlock()

		case <-s.ctx.Done():
			// 处理剩余的批量
			s.batchMutex.Lock()
			if len(s.currentBatch) > 0 {
				s.processBatch()
			}
			s.batchMutex.Unlock()
			
			logrus.Info("Batch processor stopped")
			return
		}
	}
}

// processBatch 处理批量消息
func (s *MessageSender) processBatch() {
	if len(s.currentBatch) == 0 {
		return
	}

	logrus.WithField("batch_size", len(s.currentBatch)).Info("Processing message batch")

	// 按用户分组
	userGroups := make(map[int64][]*SendRequest)
	for _, item := range s.currentBatch {
		userID := item.Request.UserID
		userGroups[userID] = append(userGroups[userID], item.Request)
	}

	// 为每个用户处理批量消息
	for userID, requests := range userGroups {
		if len(requests) == 1 {
			// 单个消息直接发送
			s.sendMessage(requests[0])
		} else {
			// 多个消息合并发送
			s.sendBatchedMessage(userID, requests)
		}
	}

	// 清空当前批量
	s.currentBatch = s.currentBatch[:0]
	s.updateBatchStats(int64(len(s.currentBatch)))
}

// sendBatchedMessage 发送合并的批量消息
func (s *MessageSender) sendBatchedMessage(userID int64, requests []*SendRequest) {
	// 提取告警
	alerts := make([]*models.Alert, 0, len(requests))
	for _, req := range requests {
		if req.Alert != nil {
			alerts = append(alerts, req.Alert)
		}
	}

	if len(alerts) > 0 {
		// 格式化批量告警消息
		messages, err := s.formatter.FormatAlertBatch(alerts)
		if err != nil {
			logrus.WithError(err).Error("Failed to format batch message")
			// 回退到单独发送
			for _, req := range requests {
				s.sendMessage(req)
			}
			return
		}

		// 发送批量消息
		for _, message := range messages {
			err := s.bot.SendMessage(userID, message)
			if err != nil {
				logrus.WithError(err).WithField("user_id", userID).Error("Failed to send batch message")
			}
		}
	}
}

// handleSendError 处理发送错误
func (s *MessageSender) handleSendError(request *SendRequest, err error) {
	s.updateSendStats(request.Priority, false)

	// 检查是否需要重试
	if request.Retries < s.config.MaxRetries {
		request.Retries++
		
		// 计算重试延迟
		delay := time.Duration(float64(s.config.RetryDelay) * 
			float64(request.Retries) * s.config.RetryBackoff)
		
		logrus.WithFields(logrus.Fields{
			"user_id": request.UserID,
			"retries": request.Retries,
			"delay":   delay,
			"error":   err,
		}).Warn("Retrying message send")

		// 延迟后重新入队
		time.AfterFunc(delay, func() {
			s.enqueueMessage(request)
		})
		
		s.stats.mu.Lock()
		s.stats.TotalRetries++
		s.stats.mu.Unlock()
	} else {
		logrus.WithFields(logrus.Fields{
			"user_id": request.UserID,
			"retries": request.Retries,
			"error":   err,
		}).Error("Message send failed after max retries")
	}
}

// 统计更新方法

// updateSendStats 更新发送统计
func (s *MessageSender) updateSendStats(priority MessagePriority, success bool) {
	s.stats.mu.Lock()
	defer s.stats.mu.Unlock()

	if success {
		s.stats.TotalSent++
		s.stats.PrioritySent[priority]++
	} else {
		s.stats.TotalFailed++
		s.stats.PriorityFailed[priority]++
	}
}

// updateBatchStats 更新批量统计
func (s *MessageSender) updateBatchStats(count int64) {
	s.stats.mu.Lock()
	defer s.stats.mu.Unlock()
	s.stats.TotalBatched += count
}

// updateLatencyStats 更新延迟统计
func (s *MessageSender) updateLatencyStats(latency time.Duration) {
	s.stats.mu.Lock()
	defer s.stats.mu.Unlock()

	// 更新平均延迟（简化计算）
	if s.stats.AverageLatency == 0 {
		s.stats.AverageLatency = latency
	} else {
		s.stats.AverageLatency = (s.stats.AverageLatency + latency) / 2
	}

	// 更新最大延迟
	if latency > s.stats.MaxLatency {
		s.stats.MaxLatency = latency
	}

	// 更新最小延迟
	if latency < s.stats.MinLatency {
		s.stats.MinLatency = latency
	}
}

// updateQueueStats 更新队列统计
func (s *MessageSender) updateQueueStats(priority MessagePriority, delta int) {
	s.stats.mu.Lock()
	defer s.stats.mu.Unlock()

	s.stats.QueueSizes[priority] += delta
	
	// 更新最大队列大小
	totalSize := 0
	for _, size := range s.stats.QueueSizes {
		totalSize += size
	}
	if totalSize > s.stats.MaxQueueSize {
		s.stats.MaxQueueSize = totalSize
	}
}

// statsUpdater 统计更新器
func (s *MessageSender) statsUpdater() {
	defer s.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.logStats()
		case <-s.ctx.Done():
			return
		}
	}
}

// logStats 记录统计信息
func (s *MessageSender) logStats() {
	s.stats.mu.RLock()
	defer s.stats.mu.RUnlock()

	logrus.WithFields(logrus.Fields{
		"total_sent":      s.stats.TotalSent,
		"total_failed":    s.stats.TotalFailed,
		"total_batched":   s.stats.TotalBatched,
		"total_retries":   s.stats.TotalRetries,
		"avg_latency":     s.stats.AverageLatency,
		"max_queue_size":  s.stats.MaxQueueSize,
		"uptime":          time.Since(s.stats.StartTime),
	}).Info("Message sender statistics")
}

// GetStats 获取统计信息
func (s *MessageSender) GetStats() map[string]interface{} {
	s.stats.mu.RLock()
	defer s.stats.mu.RUnlock()

	return map[string]interface{}{
		"total_sent":       s.stats.TotalSent,
		"total_failed":     s.stats.TotalFailed,
		"total_batched":    s.stats.TotalBatched,
		"total_retries":    s.stats.TotalRetries,
		"priority_sent":    s.stats.PrioritySent,
		"priority_failed":  s.stats.PriorityFailed,
		"average_latency":  s.stats.AverageLatency.String(),
		"max_latency":      s.stats.MaxLatency.String(),
		"min_latency":      s.stats.MinLatency.String(),
		"queue_sizes":      s.stats.QueueSizes,
		"max_queue_size":   s.stats.MaxQueueSize,
		"uptime":           time.Since(s.stats.StartTime).String(),
		"start_time":       s.stats.StartTime,
	}
}
