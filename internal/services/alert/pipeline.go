package alert

import (
	"context"
	"fmt"
	"sync"
	"time"

	"simplied-evm-monitoring-go/internal/models"
	"simplied-evm-monitoring-go/internal/services/telegram"

	"github.com/sirupsen/logrus"
)

// PipelineConfig 告警流水线配置
type PipelineConfig struct {
	// 工作线程数
	WorkerCount int `json:"worker_count"`
	// 队列大小
	QueueSize int `json:"queue_size"`
	// 批量处理大小
	BatchSize int `json:"batch_size"`
	// 批量处理间隔
	BatchInterval time.Duration `json:"batch_interval"`
	// 重试次数
	MaxRetries int `json:"max_retries"`
	// 重试间隔
	RetryInterval time.Duration `json:"retry_interval"`
	// 启用批量发送
	EnableBatching bool `json:"enable_batching"`
	// 启用告警去重
	EnableDeduplication bool `json:"enable_deduplication"`
}

// DefaultPipelineConfig 默认配置
func DefaultPipelineConfig() *PipelineConfig {
	return &PipelineConfig{
		WorkerCount:         3,
		QueueSize:           1000,
		BatchSize:           5,
		BatchInterval:       30 * time.Second,
		MaxRetries:          3,
		RetryInterval:       5 * time.Second,
		EnableBatching:      true,
		EnableDeduplication: true,
	}
}

// AlertPipeline 告警流水线
type AlertPipeline struct {
	config    *PipelineConfig
	manager   *AlertManager
	telegram  *telegram.TelegramBot
	formatter *telegram.MessageFormatter
	sender    *telegram.MessageSender
	tracker   *AlertTracker
	metrics   *PipelineMetrics

	// 内部状态
	alertQueue    chan *models.Alert
	batchQueue    chan []*models.Alert
	workers       []chan struct{}
	batchWorkerCh chan struct{}
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
	mu            sync.RWMutex
	running       bool

	logger *logrus.Logger
}

// NewAlertPipeline 创建新的告警流水线
func NewAlertPipeline(config *PipelineConfig, manager *AlertManager, telegramBot *telegram.TelegramBot) *AlertPipeline {
	if config == nil {
		config = DefaultPipelineConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	// 创建格式化器和发送器
	formatter := telegram.NewMessageFormatter(nil)
	sender := telegram.NewMessageSender(telegramBot, formatter, nil)

	pipeline := &AlertPipeline{
		config:     config,
		manager:    manager,
		telegram:   telegramBot,
		formatter:  formatter,
		sender:     sender,
		tracker:    NewAlertTracker(),
		metrics:    NewPipelineMetrics(),
		alertQueue: make(chan *models.Alert, config.QueueSize),
		batchQueue: make(chan []*models.Alert, config.QueueSize/config.BatchSize),
		workers:    make([]chan struct{}, config.WorkerCount),
		ctx:        ctx,
		cancel:     cancel,
		logger:     logrus.New(),
	}

	// 初始化工作线程控制通道
	for i := 0; i < config.WorkerCount; i++ {
		pipeline.workers[i] = make(chan struct{}, 1)
	}
	pipeline.batchWorkerCh = make(chan struct{}, 1)

	return pipeline
}

// Start 启动告警流水线
func (ap *AlertPipeline) Start() error {
	ap.mu.Lock()
	defer ap.mu.Unlock()

	if ap.running {
		return fmt.Errorf("pipeline is already running")
	}

	ap.logger.Info("Starting alert pipeline...")

	// 启动工作线程
	for i := 0; i < ap.config.WorkerCount; i++ {
		ap.wg.Add(1)
		go ap.alertWorker(i)
	}

	// 启动批量处理线程
	if ap.config.EnableBatching {
		ap.wg.Add(1)
		go ap.batchWorkerFunc()
		ap.wg.Add(1)
		go ap.batchCollector()
	}

	// 启动指标收集线程
	ap.wg.Add(1)
	go ap.metricsCollector()

	ap.running = true
	ap.logger.Info("Alert pipeline started successfully")
	return nil
}

// Stop 停止告警流水线
func (ap *AlertPipeline) Stop() error {
	ap.mu.Lock()
	defer ap.mu.Unlock()

	if !ap.running {
		return fmt.Errorf("pipeline is not running")
	}

	ap.logger.Info("Stopping alert pipeline...")

	// 取消上下文
	ap.cancel()

	// 关闭队列
	close(ap.alertQueue)
	if ap.config.EnableBatching {
		close(ap.batchQueue)
	}

	// 等待所有工作线程完成
	ap.wg.Wait()

	ap.running = false
	ap.logger.Info("Alert pipeline stopped successfully")
	return nil
}

// ProcessAlert 处理单个告警
func (ap *AlertPipeline) ProcessAlert(alert *models.Alert) error {
	if alert == nil {
		return fmt.Errorf("alert cannot be nil")
	}

	select {
	case ap.alertQueue <- alert:
		ap.metrics.IncrementAlertsReceived()
		ap.logger.WithFields(logrus.Fields{
			"alert_id":   alert.ID,
			"alert_type": alert.Type,
			"severity":   alert.Severity,
		}).Debug("Alert queued for processing")
		return nil
	case <-ap.ctx.Done():
		return fmt.Errorf("pipeline is shutting down")
	default:
		ap.metrics.IncrementAlertsDropped()
		return fmt.Errorf("alert queue is full")
	}
}

// ProcessAlerts 批量处理告警
func (ap *AlertPipeline) ProcessAlerts(alerts []*models.Alert) error {
	if len(alerts) == 0 {
		return nil
	}

	for _, alert := range alerts {
		// 获取用户ID列表（这里需要根据实际业务逻辑获取）
		userIDs := []int64{} // TODO: 从配置或数据库获取用户列表
		
		// 发送告警
		err := ap.sender.SendAlert(alert, userIDs, telegram.PriorityNormal)
		if err != nil {
			ap.logger.WithError(err).WithField("alert_id", alert.ID).Error("Failed to send alert")
			return err
		}
	}

	return nil
}

// alertWorker 告警处理工作线程
func (ap *AlertPipeline) alertWorker(workerID int) {
	defer ap.wg.Done()

	logger := ap.logger.WithField("worker_id", workerID)
	logger.Info("Alert worker started")

	for {
		select {
		case alert, ok := <-ap.alertQueue:
			if !ok {
				logger.Info("Alert worker stopped")
				return
			}

			ap.processAlertInternal(alert, logger)

		case <-ap.ctx.Done():
			logger.Info("Alert worker stopped due to context cancellation")
			return
		}
	}
}

// processAlertInternal 内部告警处理逻辑
func (ap *AlertPipeline) processAlertInternal(alert *models.Alert, logger *logrus.Entry) {
	startTime := time.Now()

	// 更新告警状态
	ap.tracker.TrackAlert(alert)

	// 去重检查
	if ap.config.EnableDeduplication {
		if ap.tracker.IsDuplicate(alert) {
			logger.WithField("alert_id", alert.ID).Debug("Alert is duplicate, skipping")
			ap.metrics.IncrementAlertsDuplicated()
			return
		}
	}

	// 发送告警
	var err error
	for retry := 0; retry <= ap.config.MaxRetries; retry++ {
		if retry > 0 {
			logger.WithFields(logrus.Fields{
				"alert_id": alert.ID,
				"retry":    retry,
			}).Warn("Retrying alert processing")
			time.Sleep(ap.config.RetryInterval)
		}

		err = ap.sendAlert(alert)
		if err == nil {
			break
		}

		logger.WithError(err).WithFields(logrus.Fields{
			"alert_id": alert.ID,
			"retry":    retry,
		}).Error("Failed to send alert")
	}

	// 更新统计
	duration := time.Since(startTime)
	if err != nil {
		ap.metrics.IncrementAlertsFailed()
		ap.tracker.MarkAlertFailed(alert, err)
	} else {
		ap.metrics.IncrementAlertsProcessed()
		ap.metrics.UpdateProcessingTime(duration)
		ap.tracker.MarkAlertSent(alert)
	}

	logger.WithFields(logrus.Fields{
		"alert_id": alert.ID,
		"duration": duration,
		"success":  err == nil,
	}).Debug("Alert processing completed")
}

// sendAlert 发送告警
func (ap *AlertPipeline) sendAlert(alert *models.Alert) error {
	// 获取用户ID列表（这里需要根据实际业务逻辑获取）
	userIDs := []int64{} // TODO: 从配置或数据库获取用户列表
	
	// 使用消息发送器发送告警
	return ap.sender.SendAlert(alert, userIDs, telegram.PriorityNormal)
}

// batchCollector 批量收集器
func (ap *AlertPipeline) batchCollector() {
	defer ap.wg.Done()

	ticker := time.NewTicker(ap.config.BatchInterval)
	defer ticker.Stop()

	var batch []*models.Alert

	for {
		select {
		case alert, ok := <-ap.alertQueue:
			if !ok {
				// 处理剩余的批量告警
				if len(batch) > 0 {
					ap.processBatch(batch)
				}
				return
			}

			batch = append(batch, alert)

			// 检查是否达到批量大小
			if len(batch) >= ap.config.BatchSize {
				ap.processBatch(batch)
				batch = nil
			}

		case <-ticker.C:
			// 定时处理批量告警
			if len(batch) > 0 {
				ap.processBatch(batch)
				batch = nil
			}

		case <-ap.ctx.Done():
			// 处理剩余的批量告警
			if len(batch) > 0 {
				ap.processBatch(batch)
			}
			return
		}
	}
}

// processBatch 处理批量告警
func (ap *AlertPipeline) processBatch(alerts []*models.Alert) {
	if len(alerts) == 0 {
		return
	}

	select {
	case ap.batchQueue <- alerts:
		ap.logger.WithField("batch_size", len(alerts)).Debug("Batch queued for processing")
	default:
		ap.logger.WithField("batch_size", len(alerts)).Warn("Batch queue is full, processing individually")
		// 如果批量队列满了，回退到单个处理
		for _, alert := range alerts {
			ap.processAlertInternal(alert, ap.logger.WithField("alert_id", alert.ID))
		}
	}
}

// batchWorkerFunc 批量处理工作线程
func (ap *AlertPipeline) batchWorkerFunc() {
	defer ap.wg.Done()

	ap.logger.Info("Batch worker started")

	for {
		select {
		case batch, ok := <-ap.batchQueue:
			if !ok {
				ap.logger.Info("Batch worker stopped")
				return
			}

			ap.processBatchInternal(batch)

		case <-ap.ctx.Done():
			ap.logger.Info("Batch worker stopped due to context cancellation")
			return
		}
	}
}

// processBatchInternal 内部批量处理逻辑
func (ap *AlertPipeline) processBatchInternal(alerts []*models.Alert) {
	startTime := time.Now()

	// 发送批量告警
	err := ap.sender.SendAlertBatch(alerts, []int64{}, telegram.PriorityNormal)

	duration := time.Since(startTime)
	if err != nil {
		ap.logger.WithError(err).WithField("batch_size", len(alerts)).Error("Failed to send batch alerts")
		ap.metrics.IncrementBatchesFailed()

		// 回退到单个处理
		for _, alert := range alerts {
			ap.processAlertInternal(alert, ap.logger.WithField("alert_id", alert.ID))
		}
	} else {
		ap.logger.WithFields(logrus.Fields{
			"batch_size": len(alerts),
			"duration":   duration,
		}).Debug("Batch alerts sent successfully")

		ap.metrics.IncrementBatchesProcessed()
		ap.metrics.UpdateBatchProcessingTime(duration)

		// 更新每个告警的状态
		for _, alert := range alerts {
			ap.tracker.TrackAlert(alert)
			ap.tracker.MarkAlertSent(alert)
		}
	}
}

// metricsCollector 指标收集线程
func (ap *AlertPipeline) metricsCollector() {
	defer ap.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ap.collectMetrics()
		case <-ap.ctx.Done():
			return
		}
	}
}

// collectMetrics 收集指标
func (ap *AlertPipeline) collectMetrics() {
	// 更新队列大小指标
	ap.metrics.UpdateQueueSize(len(ap.alertQueue))
	if ap.config.EnableBatching {
		ap.metrics.UpdateBatchQueueSize(len(ap.batchQueue))
	}

	// 记录指标日志
	ap.logger.WithFields(logrus.Fields{
		"alerts_received":     ap.metrics.GetAlertsReceived(),
		"alerts_processed":    ap.metrics.GetAlertsProcessed(),
		"alerts_failed":       ap.metrics.GetAlertsFailed(),
		"alerts_dropped":      ap.metrics.GetAlertsDropped(),
		"queue_size":          len(ap.alertQueue),
		"avg_processing_time": ap.metrics.GetAverageProcessingTime(),
	}).Info("Pipeline metrics")
}

// GetMetrics 获取流水线指标
func (ap *AlertPipeline) GetMetrics() *PipelineMetrics {
	return ap.metrics
}

// GetTracker 获取告警跟踪器
func (ap *AlertPipeline) GetTracker() *AlertTracker {
	return ap.tracker
}

// IsRunning 检查流水线是否运行中
func (ap *AlertPipeline) IsRunning() bool {
	ap.mu.RLock()
	defer ap.mu.RUnlock()
	return ap.running
}

// GetStatus 获取流水线状态
func (ap *AlertPipeline) GetStatus() map[string]interface{} {
	ap.mu.RLock()
	defer ap.mu.RUnlock()

	return map[string]interface{}{
		"running":             ap.running,
		"worker_count":        ap.config.WorkerCount,
		"queue_size":          len(ap.alertQueue),
		"batch_queue_size":    len(ap.batchQueue),
		"alerts_received":     ap.metrics.GetAlertsReceived(),
		"alerts_processed":    ap.metrics.GetAlertsProcessed(),
		"alerts_failed":       ap.metrics.GetAlertsFailed(),
		"alerts_dropped":      ap.metrics.GetAlertsDropped(),
		"batches_processed":   ap.metrics.GetBatchesProcessed(),
		"batches_failed":      ap.metrics.GetBatchesFailed(),
		"avg_processing_time": ap.metrics.GetAverageProcessingTime(),
	}
}
