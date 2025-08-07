package alert

import (
	"crypto/md5"
	"fmt"
	"sync"
	"time"

	"simplied-evm-monitoring-go/internal/models"

	"github.com/sirupsen/logrus"
)

// AlertStatus 告警状态
type AlertStatus string

const (
	AlertStatusPending    AlertStatus = "pending"    // 待处理
	AlertStatusProcessing AlertStatus = "processing" // 处理中
	AlertStatusSent       AlertStatus = "sent"       // 已发送
	AlertStatusFailed     AlertStatus = "failed"     // 发送失败
	AlertStatusDuplicate  AlertStatus = "duplicate"  // 重复告警
)

// TrackingRecord 告警跟踪记录
type TrackingRecord struct {
	Alert       *models.Alert `json:"alert"`
	Status      AlertStatus   `json:"status"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
	SentAt      *time.Time    `json:"sent_at,omitempty"`
	FailedAt    *time.Time    `json:"failed_at,omitempty"`
	RetryCount  int           `json:"retry_count"`
	LastError   string        `json:"last_error,omitempty"`
	ProcessTime time.Duration `json:"process_time"`
	Hash        string        `json:"hash"` // 用于去重
}

// AlertTracker 告警跟踪器
type AlertTracker struct {
	records          map[uint64]*TrackingRecord   // 按ID索引的记录
	hashIndex        map[string]*TrackingRecord // 按哈希索引的记录（用于去重）
	statusIndex      map[AlertStatus][]*TrackingRecord // 按状态索引的记录
	mu               sync.RWMutex
	maxRecords       int           // 最大记录数
	cleanupInterval  time.Duration // 清理间隔
	recordTTL        time.Duration // 记录生存时间
	deduplicationTTL time.Duration // 去重时间窗口
	logger           *logrus.Logger
}

// TrackerConfig 跟踪器配置
type TrackerConfig struct {
	MaxRecords       int           `json:"max_records"`
	CleanupInterval  time.Duration `json:"cleanup_interval"`
	RecordTTL        time.Duration `json:"record_ttl"`
	DeduplicationTTL time.Duration `json:"deduplication_ttl"`
}

// DefaultTrackerConfig 默认配置
func DefaultTrackerConfig() *TrackerConfig {
	return &TrackerConfig{
		MaxRecords:       10000,
		CleanupInterval:  5 * time.Minute,
		RecordTTL:        24 * time.Hour,
		DeduplicationTTL: 1 * time.Hour,
	}
}

// NewAlertTracker 创建新的告警跟踪器
func NewAlertTracker() *AlertTracker {
	config := DefaultTrackerConfig()
	
	tracker := &AlertTracker{
		records:          make(map[uint64]*TrackingRecord),
		hashIndex:        make(map[string]*TrackingRecord),
		statusIndex:      make(map[AlertStatus][]*TrackingRecord),
		maxRecords:       config.MaxRecords,
		cleanupInterval:  config.CleanupInterval,
		recordTTL:        config.RecordTTL,
		deduplicationTTL: config.DeduplicationTTL,
		logger:           logrus.New(),
	}

	// 初始化状态索引
	tracker.statusIndex[AlertStatusPending] = make([]*TrackingRecord, 0)
	tracker.statusIndex[AlertStatusProcessing] = make([]*TrackingRecord, 0)
	tracker.statusIndex[AlertStatusSent] = make([]*TrackingRecord, 0)
	tracker.statusIndex[AlertStatusFailed] = make([]*TrackingRecord, 0)
	tracker.statusIndex[AlertStatusDuplicate] = make([]*TrackingRecord, 0)

	// 启动清理任务
	go tracker.cleanupWorker()

	return tracker
}

// TrackAlert 跟踪告警
func (at *AlertTracker) TrackAlert(alert *models.Alert) *TrackingRecord {
	at.mu.Lock()
	defer at.mu.Unlock()

	// 检查是否已存在
	if existing, exists := at.records[alert.ID]; exists {
		existing.UpdatedAt = time.Now()
		return existing
	}

	// 创建新记录
	record := &TrackingRecord{
		Alert:       alert,
		Status:      AlertStatusPending,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		RetryCount:  0,
		Hash:        at.generateAlertHash(alert),
	}

	// 添加到索引
	at.records[alert.ID] = record
	at.hashIndex[record.Hash] = record
	at.statusIndex[AlertStatusPending] = append(at.statusIndex[AlertStatusPending], record)

	at.logger.WithFields(logrus.Fields{
		"alert_id": alert.ID,
		"hash":     record.Hash,
		"type":     alert.Type,
	}).Debug("Alert tracked")

	return record
}

// MarkAlertProcessing 标记告警为处理中
func (at *AlertTracker) MarkAlertProcessing(alert *models.Alert) {
	at.updateAlertStatus(alert.ID, AlertStatusProcessing)
}

// MarkAlertSent 标记告警为已发送
func (at *AlertTracker) MarkAlertSent(alert *models.Alert) {
	at.mu.Lock()
	defer at.mu.Unlock()

	if record, exists := at.records[alert.ID]; exists {
		now := time.Now()
		record.Status = AlertStatusSent
		record.UpdatedAt = now
		record.SentAt = &now
		record.ProcessTime = now.Sub(record.CreatedAt)

		// 更新状态索引
		at.removeFromStatusIndex(record, record.Status)
		at.statusIndex[AlertStatusSent] = append(at.statusIndex[AlertStatusSent], record)

		at.logger.WithFields(logrus.Fields{
			"alert_id":     alert.ID,
			"process_time": record.ProcessTime,
		}).Debug("Alert marked as sent")
	}
}

// MarkAlertFailed 标记告警为失败
func (at *AlertTracker) MarkAlertFailed(alert *models.Alert, err error) {
	at.mu.Lock()
	defer at.mu.Unlock()

	if record, exists := at.records[alert.ID]; exists {
		now := time.Now()
		record.Status = AlertStatusFailed
		record.UpdatedAt = now
		record.FailedAt = &now
		record.RetryCount++
		if err != nil {
			record.LastError = err.Error()
		}

		// 更新状态索引
		at.removeFromStatusIndex(record, record.Status)
		at.statusIndex[AlertStatusFailed] = append(at.statusIndex[AlertStatusFailed], record)

		at.logger.WithFields(logrus.Fields{
			"alert_id":    alert.ID,
			"retry_count": record.RetryCount,
			"error":       record.LastError,
		}).Debug("Alert marked as failed")
	}
}

// MarkAlertDuplicate 标记告警为重复
func (at *AlertTracker) MarkAlertDuplicate(alert *models.Alert) {
	at.updateAlertStatus(alert.ID, AlertStatusDuplicate)
}

// IsDuplicate 检查告警是否重复
func (at *AlertTracker) IsDuplicate(alert *models.Alert) bool {
	at.mu.RLock()
	defer at.mu.RUnlock()

	hash := at.generateAlertHash(alert)
	
	if existing, exists := at.hashIndex[hash]; exists {
		// 检查是否在去重时间窗口内
		if time.Since(existing.CreatedAt) <= at.deduplicationTTL {
			// 排除失败的告警
			if existing.Status != AlertStatusFailed {
				return true
			}
		}
	}

	return false
}

// GetAlertRecord 获取告警记录
func (at *AlertTracker) GetAlertRecord(alertID uint64) (*TrackingRecord, bool) {
	at.mu.RLock()
	defer at.mu.RUnlock()

	record, exists := at.records[alertID]
	return record, exists
}

// GetAlertsByStatus 按状态获取告警
func (at *AlertTracker) GetAlertsByStatus(status AlertStatus) []*TrackingRecord {
	at.mu.RLock()
	defer at.mu.RUnlock()

	records := at.statusIndex[status]
	result := make([]*TrackingRecord, len(records))
	copy(result, records)
	return result
}

// GetRecentAlerts 获取最近的告警
func (at *AlertTracker) GetRecentAlerts(limit int, since time.Duration) []*TrackingRecord {
	at.mu.RLock()
	defer at.mu.RUnlock()

	var result []*TrackingRecord
	cutoff := time.Now().Add(-since)

	for _, record := range at.records {
		if record.CreatedAt.After(cutoff) {
			result = append(result, record)
		}
		if len(result) >= limit {
			break
		}
	}

	return result
}

// GetStatistics 获取统计信息
func (at *AlertTracker) GetStatistics() map[string]interface{} {
	at.mu.RLock()
	defer at.mu.RUnlock()

	stats := map[string]interface{}{
		"total_records": len(at.records),
		"by_status": map[string]int{
			"pending":    len(at.statusIndex[AlertStatusPending]),
			"processing": len(at.statusIndex[AlertStatusProcessing]),
			"sent":       len(at.statusIndex[AlertStatusSent]),
			"failed":     len(at.statusIndex[AlertStatusFailed]),
			"duplicate":  len(at.statusIndex[AlertStatusDuplicate]),
		},
	}

	// 计算成功率
	total := len(at.records)
	if total > 0 {
		sent := len(at.statusIndex[AlertStatusSent])
		stats["success_rate"] = float64(sent) / float64(total) * 100
	}

	// 计算平均处理时间
	var totalProcessTime time.Duration
	var processedCount int
	for _, record := range at.records {
		if record.Status == AlertStatusSent && record.ProcessTime > 0 {
			totalProcessTime += record.ProcessTime
			processedCount++
		}
	}
	if processedCount > 0 {
		stats["avg_process_time"] = totalProcessTime / time.Duration(processedCount)
	}

	return stats
}

// updateAlertStatus 更新告警状态
func (at *AlertTracker) updateAlertStatus(alertID uint64, status AlertStatus) {
	at.mu.Lock()
	defer at.mu.Unlock()

	if record, exists := at.records[alertID]; exists {
		oldStatus := record.Status
		record.Status = status
		record.UpdatedAt = time.Now()

		// 更新状态索引
		at.removeFromStatusIndex(record, oldStatus)
		at.statusIndex[status] = append(at.statusIndex[status], record)

		at.logger.WithFields(logrus.Fields{
			"alert_id":   alertID,
			"old_status": oldStatus,
			"new_status": status,
		}).Debug("Alert status updated")
	}
}

// removeFromStatusIndex 从状态索引中移除记录
func (at *AlertTracker) removeFromStatusIndex(record *TrackingRecord, status AlertStatus) {
	records := at.statusIndex[status]
	for i, r := range records {
		if r == record {
			// 移除元素
			at.statusIndex[status] = append(records[:i], records[i+1:]...)
			break
		}
	}
}

// generateAlertHash 生成告警哈希（用于去重）
func (at *AlertTracker) generateAlertHash(alert *models.Alert) string {
	// 基于告警类型、严重程度和触发数据生成哈希
	data := fmt.Sprintf("%s:%s:%s", alert.Type, alert.Severity, alert.TriggerData)
	hash := md5.Sum([]byte(data))
	return fmt.Sprintf("%x", hash)
}

// cleanupWorker 清理工作线程
func (at *AlertTracker) cleanupWorker() {
	ticker := time.NewTicker(at.cleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		at.cleanup()
	}
}

// cleanup 清理过期记录
func (at *AlertTracker) cleanup() {
	at.mu.Lock()
	defer at.mu.Unlock()

	now := time.Now()
	var toDelete []uint64

	// 找出过期的记录
	for id, record := range at.records {
		if now.Sub(record.CreatedAt) > at.recordTTL {
			toDelete = append(toDelete, id)
		}
	}

	// 删除过期记录
	for _, id := range toDelete {
		record := at.records[id]
		
		// 从各个索引中删除
		delete(at.records, id)
		delete(at.hashIndex, record.Hash)
		at.removeFromStatusIndex(record, record.Status)
	}

	// 如果记录数量超过限制，删除最老的记录
	if len(at.records) > at.maxRecords {
		// 按创建时间排序，删除最老的记录
		var oldestRecords []*TrackingRecord
		for _, record := range at.records {
			oldestRecords = append(oldestRecords, record)
		}

		// 简单的选择排序，找出最老的记录
		toDeleteCount := len(at.records) - at.maxRecords
		for i := 0; i < toDeleteCount; i++ {
			var oldest *TrackingRecord
			var oldestID uint64
			for id, record := range at.records {
				if oldest == nil || record.CreatedAt.Before(oldest.CreatedAt) {
					oldest = record
					oldestID = id
				}
			}
			if oldest != nil {
				delete(at.records, oldestID)
				delete(at.hashIndex, oldest.Hash)
				at.removeFromStatusIndex(oldest, oldest.Status)
			}
		}
	}

	if len(toDelete) > 0 {
		at.logger.WithField("deleted_count", len(toDelete)).Debug("Cleaned up expired alert records")
	}
}

// Clear 清空所有记录
func (at *AlertTracker) Clear() {
	at.mu.Lock()
	defer at.mu.Unlock()

	at.records = make(map[uint64]*TrackingRecord)
	at.hashIndex = make(map[string]*TrackingRecord)
	
	// 重新初始化状态索引
	at.statusIndex[AlertStatusPending] = make([]*TrackingRecord, 0)
	at.statusIndex[AlertStatusProcessing] = make([]*TrackingRecord, 0)
	at.statusIndex[AlertStatusSent] = make([]*TrackingRecord, 0)
	at.statusIndex[AlertStatusFailed] = make([]*TrackingRecord, 0)
	at.statusIndex[AlertStatusDuplicate] = make([]*TrackingRecord, 0)

	at.logger.Info("Alert tracker cleared")
}
