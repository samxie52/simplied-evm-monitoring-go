package alert

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
)

// PipelineMetrics 流水线指标
type PipelineMetrics struct {
	// 告警统计
	alertsReceived   int64 // 接收的告警数
	alertsProcessed  int64 // 处理成功的告警数
	alertsFailed     int64 // 处理失败的告警数
	alertsDropped    int64 // 丢弃的告警数
	alertsDuplicated int64 // 重复的告警数

	// 批量处理统计
	batchesProcessed int64 // 处理的批次数
	batchesFailed    int64 // 失败的批次数

	// 性能指标
	processingTimes      []time.Duration // 处理时间记录
	batchProcessingTimes []time.Duration // 批量处理时间记录
	mu                   sync.RWMutex    // 保护切片的读写

	// 队列指标
	currentQueueSize      int64 // 当前队列大小
	currentBatchQueueSize int64 // 当前批量队列大小
	maxQueueSize          int64 // 最大队列大小
	maxBatchQueueSize     int64 // 最大批量队列大小

	// 时间窗口指标
	windowSize      time.Duration  // 时间窗口大小
	windowStartTime time.Time      // 窗口开始时间
	windowMetrics   *WindowMetrics // 窗口内的指标
	windowMu        sync.RWMutex   // 保护窗口指标

	logger *logrus.Logger
}

// WindowMetrics 时间窗口内的指标
type WindowMetrics struct {
	AlertsReceived    int64         `json:"alerts_received"`
	AlertsProcessed   int64         `json:"alerts_processed"`
	AlertsFailed      int64         `json:"alerts_failed"`
	AlertsDropped     int64         `json:"alerts_dropped"`
	BatchesProcessed  int64         `json:"batches_processed"`
	BatchesFailed     int64         `json:"batches_failed"`
	AvgProcessingTime time.Duration `json:"avg_processing_time"`
	MaxProcessingTime time.Duration `json:"max_processing_time"`
	MinProcessingTime time.Duration `json:"min_processing_time"`
	Throughput        float64       `json:"throughput"` // 每秒处理的告警数
}

// NewPipelineMetrics 创建新的流水线指标
func NewPipelineMetrics() *PipelineMetrics {
	return &PipelineMetrics{
		processingTimes:      make([]time.Duration, 0, 1000),
		batchProcessingTimes: make([]time.Duration, 0, 100),
		windowSize:           5 * time.Minute,
		windowStartTime:      time.Now(),
		windowMetrics:        &WindowMetrics{},
		logger:               logrus.New(),
	}
}

// IncrementAlertsReceived 增加接收的告警数
func (pm *PipelineMetrics) IncrementAlertsReceived() {
	atomic.AddInt64(&pm.alertsReceived, 1)
	pm.updateWindowMetric("received")
}

// IncrementAlertsProcessed 增加处理成功的告警数
func (pm *PipelineMetrics) IncrementAlertsProcessed() {
	atomic.AddInt64(&pm.alertsProcessed, 1)
	pm.updateWindowMetric("processed")
}

// IncrementAlertsFailed 增加处理失败的告警数
func (pm *PipelineMetrics) IncrementAlertsFailed() {
	atomic.AddInt64(&pm.alertsFailed, 1)
	pm.updateWindowMetric("failed")
}

// IncrementAlertsDropped 增加丢弃的告警数
func (pm *PipelineMetrics) IncrementAlertsDropped() {
	atomic.AddInt64(&pm.alertsDropped, 1)
	pm.updateWindowMetric("dropped")
}

// IncrementAlertsDuplicated 增加重复的告警数
func (pm *PipelineMetrics) IncrementAlertsDuplicated() {
	atomic.AddInt64(&pm.alertsDuplicated, 1)
}

// IncrementBatchesProcessed 增加处理的批次数
func (pm *PipelineMetrics) IncrementBatchesProcessed() {
	atomic.AddInt64(&pm.batchesProcessed, 1)
	pm.updateWindowMetric("batch_processed")
}

// IncrementBatchesFailed 增加失败的批次数
func (pm *PipelineMetrics) IncrementBatchesFailed() {
	atomic.AddInt64(&pm.batchesFailed, 1)
	pm.updateWindowMetric("batch_failed")
}

// UpdateProcessingTime 更新处理时间
func (pm *PipelineMetrics) UpdateProcessingTime(duration time.Duration) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.processingTimes = append(pm.processingTimes, duration)

	// 保持最近1000个记录
	if len(pm.processingTimes) > 1000 {
		pm.processingTimes = pm.processingTimes[len(pm.processingTimes)-1000:]
	}

	pm.updateWindowProcessingTime(duration)
}

// UpdateBatchProcessingTime 更新批量处理时间
func (pm *PipelineMetrics) UpdateBatchProcessingTime(duration time.Duration) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.batchProcessingTimes = append(pm.batchProcessingTimes, duration)

	// 保持最近100个记录
	if len(pm.batchProcessingTimes) > 100 {
		pm.batchProcessingTimes = pm.batchProcessingTimes[len(pm.batchProcessingTimes)-100:]
	}
}

// UpdateQueueSize 更新队列大小
func (pm *PipelineMetrics) UpdateQueueSize(size int) {
	newSize := int64(size)
	atomic.StoreInt64(&pm.currentQueueSize, newSize)

	// 更新最大队列大小
	for {
		current := atomic.LoadInt64(&pm.maxQueueSize)
		if newSize <= current {
			break
		}
		if atomic.CompareAndSwapInt64(&pm.maxQueueSize, current, newSize) {
			break
		}
	}
}

// UpdateBatchQueueSize 更新批量队列大小
func (pm *PipelineMetrics) UpdateBatchQueueSize(size int) {
	newSize := int64(size)
	atomic.StoreInt64(&pm.currentBatchQueueSize, newSize)

	// 更新最大批量队列大小
	for {
		current := atomic.LoadInt64(&pm.maxBatchQueueSize)
		if newSize <= current {
			break
		}
		if atomic.CompareAndSwapInt64(&pm.maxBatchQueueSize, current, newSize) {
			break
		}
	}
}

// GetAlertsReceived 获取接收的告警数
func (pm *PipelineMetrics) GetAlertsReceived() int64 {
	return atomic.LoadInt64(&pm.alertsReceived)
}

// GetAlertsProcessed 获取处理成功的告警数
func (pm *PipelineMetrics) GetAlertsProcessed() int64 {
	return atomic.LoadInt64(&pm.alertsProcessed)
}

// GetAlertsFailed 获取处理失败的告警数
func (pm *PipelineMetrics) GetAlertsFailed() int64 {
	return atomic.LoadInt64(&pm.alertsFailed)
}

// GetAlertsDropped 获取丢弃的告警数
func (pm *PipelineMetrics) GetAlertsDropped() int64 {
	return atomic.LoadInt64(&pm.alertsDropped)
}

// GetAlertsDuplicated 获取重复的告警数
func (pm *PipelineMetrics) GetAlertsDuplicated() int64 {
	return atomic.LoadInt64(&pm.alertsDuplicated)
}

// GetBatchesProcessed 获取处理的批次数
func (pm *PipelineMetrics) GetBatchesProcessed() int64 {
	return atomic.LoadInt64(&pm.batchesProcessed)
}

// GetBatchesFailed 获取失败的批次数
func (pm *PipelineMetrics) GetBatchesFailed() int64 {
	return atomic.LoadInt64(&pm.batchesFailed)
}

// GetCurrentQueueSize 获取当前队列大小
func (pm *PipelineMetrics) GetCurrentQueueSize() int64 {
	return atomic.LoadInt64(&pm.currentQueueSize)
}

// GetCurrentBatchQueueSize 获取当前批量队列大小
func (pm *PipelineMetrics) GetCurrentBatchQueueSize() int64 {
	return atomic.LoadInt64(&pm.currentBatchQueueSize)
}

// GetMaxQueueSize 获取最大队列大小
func (pm *PipelineMetrics) GetMaxQueueSize() int64 {
	return atomic.LoadInt64(&pm.maxQueueSize)
}

// GetMaxBatchQueueSize 获取最大批量队列大小
func (pm *PipelineMetrics) GetMaxBatchQueueSize() int64 {
	return atomic.LoadInt64(&pm.maxBatchQueueSize)
}

// GetAverageProcessingTime 获取平均处理时间
func (pm *PipelineMetrics) GetAverageProcessingTime() time.Duration {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	if len(pm.processingTimes) == 0 {
		return 0
	}

	var total time.Duration
	for _, t := range pm.processingTimes {
		total += t
	}

	return total / time.Duration(len(pm.processingTimes))
}

// GetMaxProcessingTime 获取最大处理时间
func (pm *PipelineMetrics) GetMaxProcessingTime() time.Duration {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	if len(pm.processingTimes) == 0 {
		return 0
	}

	var max time.Duration
	for _, t := range pm.processingTimes {
		if t > max {
			max = t
		}
	}

	return max
}

// GetMinProcessingTime 获取最小处理时间
func (pm *PipelineMetrics) GetMinProcessingTime() time.Duration {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	if len(pm.processingTimes) == 0 {
		return 0
	}

	min := pm.processingTimes[0]
	for _, t := range pm.processingTimes {
		if t < min {
			min = t
		}
	}

	return min
}

// GetAverageBatchProcessingTime 获取平均批量处理时间
func (pm *PipelineMetrics) GetAverageBatchProcessingTime() time.Duration {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	if len(pm.batchProcessingTimes) == 0 {
		return 0
	}

	var total time.Duration
	for _, t := range pm.batchProcessingTimes {
		total += t
	}

	return total / time.Duration(len(pm.batchProcessingTimes))
}

// GetSuccessRate 获取成功率
func (pm *PipelineMetrics) GetSuccessRate() float64 {
	received := atomic.LoadInt64(&pm.alertsReceived)
	if received == 0 {
		return 0
	}

	processed := atomic.LoadInt64(&pm.alertsProcessed)
	return float64(processed) / float64(received) * 100
}

// GetThroughput 获取吞吐量（每秒处理的告警数）
func (pm *PipelineMetrics) GetThroughput() float64 {
	pm.windowMu.RLock()
	defer pm.windowMu.RUnlock()

	return pm.windowMetrics.Throughput
}

// GetWindowMetrics 获取时间窗口指标
func (pm *PipelineMetrics) GetWindowMetrics() *WindowMetrics {
	pm.windowMu.RLock()
	defer pm.windowMu.RUnlock()

	// 返回副本
	return &WindowMetrics{
		AlertsReceived:    pm.windowMetrics.AlertsReceived,
		AlertsProcessed:   pm.windowMetrics.AlertsProcessed,
		AlertsFailed:      pm.windowMetrics.AlertsFailed,
		AlertsDropped:     pm.windowMetrics.AlertsDropped,
		BatchesProcessed:  pm.windowMetrics.BatchesProcessed,
		BatchesFailed:     pm.windowMetrics.BatchesFailed,
		AvgProcessingTime: pm.windowMetrics.AvgProcessingTime,
		MaxProcessingTime: pm.windowMetrics.MaxProcessingTime,
		MinProcessingTime: pm.windowMetrics.MinProcessingTime,
		Throughput:        pm.windowMetrics.Throughput,
	}
}

// GetSummary 获取指标摘要
func (pm *PipelineMetrics) GetSummary() map[string]interface{} {
	return map[string]interface{}{
		"alerts": map[string]interface{}{
			"received":     pm.GetAlertsReceived(),
			"processed":    pm.GetAlertsProcessed(),
			"failed":       pm.GetAlertsFailed(),
			"dropped":      pm.GetAlertsDropped(),
			"duplicated":   pm.GetAlertsDuplicated(),
			"success_rate": pm.GetSuccessRate(),
		},
		"batches": map[string]interface{}{
			"processed": pm.GetBatchesProcessed(),
			"failed":    pm.GetBatchesFailed(),
		},
		"performance": map[string]interface{}{
			"avg_processing_time":       pm.GetAverageProcessingTime(),
			"max_processing_time":       pm.GetMaxProcessingTime(),
			"min_processing_time":       pm.GetMinProcessingTime(),
			"avg_batch_processing_time": pm.GetAverageBatchProcessingTime(),
			"throughput":                pm.GetThroughput(),
		},
		"queues": map[string]interface{}{
			"current_queue_size":       pm.GetCurrentQueueSize(),
			"max_queue_size":           pm.GetMaxQueueSize(),
			"current_batch_queue_size": pm.GetCurrentBatchQueueSize(),
			"max_batch_queue_size":     pm.GetMaxBatchQueueSize(),
		},
		"window_metrics": pm.GetWindowMetrics(),
	}
}

// updateWindowMetric 更新时间窗口指标
func (pm *PipelineMetrics) updateWindowMetric(metricType string) {
	pm.windowMu.Lock()
	defer pm.windowMu.Unlock()

	// 检查是否需要重置窗口
	if time.Since(pm.windowStartTime) >= pm.windowSize {
		pm.resetWindow()
	}

	// 更新对应的指标
	switch metricType {
	case "received":
		pm.windowMetrics.AlertsReceived++
	case "processed":
		pm.windowMetrics.AlertsProcessed++
	case "failed":
		pm.windowMetrics.AlertsFailed++
	case "dropped":
		pm.windowMetrics.AlertsDropped++
	case "batch_processed":
		pm.windowMetrics.BatchesProcessed++
	case "batch_failed":
		pm.windowMetrics.BatchesFailed++
	}

	// 更新吞吐量
	elapsed := time.Since(pm.windowStartTime)
	if elapsed > 0 {
		pm.windowMetrics.Throughput = float64(pm.windowMetrics.AlertsProcessed) / elapsed.Seconds()
	}
}

// updateWindowProcessingTime 更新窗口内的处理时间指标
func (pm *PipelineMetrics) updateWindowProcessingTime(duration time.Duration) {
	pm.windowMu.Lock()
	defer pm.windowMu.Unlock()

	// 更新平均处理时间
	if pm.windowMetrics.AvgProcessingTime == 0 {
		pm.windowMetrics.AvgProcessingTime = duration
	} else {
		// 简单的移动平均
		pm.windowMetrics.AvgProcessingTime = (pm.windowMetrics.AvgProcessingTime + duration) / 2
	}

	// 更新最大处理时间
	if duration > pm.windowMetrics.MaxProcessingTime {
		pm.windowMetrics.MaxProcessingTime = duration
	}

	// 更新最小处理时间
	if pm.windowMetrics.MinProcessingTime == 0 || duration < pm.windowMetrics.MinProcessingTime {
		pm.windowMetrics.MinProcessingTime = duration
	}
}

// resetWindow 重置时间窗口
func (pm *PipelineMetrics) resetWindow() {
	pm.windowStartTime = time.Now()
	pm.windowMetrics = &WindowMetrics{}
}

// Reset 重置所有指标
func (pm *PipelineMetrics) Reset() {
	atomic.StoreInt64(&pm.alertsReceived, 0)
	atomic.StoreInt64(&pm.alertsProcessed, 0)
	atomic.StoreInt64(&pm.alertsFailed, 0)
	atomic.StoreInt64(&pm.alertsDropped, 0)
	atomic.StoreInt64(&pm.alertsDuplicated, 0)
	atomic.StoreInt64(&pm.batchesProcessed, 0)
	atomic.StoreInt64(&pm.batchesFailed, 0)
	atomic.StoreInt64(&pm.currentQueueSize, 0)
	atomic.StoreInt64(&pm.currentBatchQueueSize, 0)
	atomic.StoreInt64(&pm.maxQueueSize, 0)
	atomic.StoreInt64(&pm.maxBatchQueueSize, 0)

	pm.mu.Lock()
	pm.processingTimes = pm.processingTimes[:0]
	pm.batchProcessingTimes = pm.batchProcessingTimes[:0]
	pm.mu.Unlock()

	pm.windowMu.Lock()
	pm.resetWindow()
	pm.windowMu.Unlock()

	pm.logger.Info("Pipeline metrics reset")
}

// StartPeriodicLogging 启动定期日志记录
func (pm *PipelineMetrics) StartPeriodicLogging(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for range ticker.C {
			pm.logMetrics()
		}
	}()
}

// logMetrics 记录指标日志
func (pm *PipelineMetrics) logMetrics() {
	summary := pm.GetSummary()

	pm.logger.WithFields(logrus.Fields{
		"alerts_received":     summary["alerts"].(map[string]interface{})["received"],
		"alerts_processed":    summary["alerts"].(map[string]interface{})["processed"],
		"alerts_failed":       summary["alerts"].(map[string]interface{})["failed"],
		"success_rate":        summary["alerts"].(map[string]interface{})["success_rate"],
		"avg_processing_time": summary["performance"].(map[string]interface{})["avg_processing_time"],
		"throughput":          summary["performance"].(map[string]interface{})["throughput"],
		"queue_size":          summary["queues"].(map[string]interface{})["current_queue_size"],
	}).Info("Pipeline metrics summary")
}
