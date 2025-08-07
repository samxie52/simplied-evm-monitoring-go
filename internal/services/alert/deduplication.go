package alert

import (
	"crypto/sha256"
	"fmt"
	"simplied-evm-monitoring-go/pkg/logger"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// DeduplicationManager 去重管理器
type DeduplicationManager struct {
	// 去重缓存
	cache map[string]*AlertRecord
	// 去重窗口时间
	windowDuration time.Duration
	// 读写锁
	mu sync.RWMutex
	// 清理定时器
	cleanupTicker *time.Ticker
	// 停止信号
	stopChan chan struct{}
	// 运行状态
	isRunning bool
}

// AlertRecord 告警记录
type AlertRecord struct {
	// 告警ID
	AlertID string `json:"alert_id"`
	// 告警哈希
	AlertHash string `json:"alert_hash"`
	// 首次触发时间
	FirstTriggered time.Time `json:"first_triggered"`
	// 最后触发时间
	LastTriggered time.Time `json:"last_triggered"`
	// 触发次数
	TriggerCount int `json:"trigger_count"`
	// 告警数据
	AlertData map[string]interface{} `json:"alert_data"`
}

// DeduplicationConfig 去重配置
type DeduplicationConfig struct {
	// 去重窗口时间
	WindowDuration time.Duration `json:"window_duration"`
	// 清理间隔
	CleanupInterval time.Duration `json:"cleanup_interval"`
	// 最大缓存大小
	MaxCacheSize int `json:"max_cache_size"`
	// 是否启用基于内容的去重
	EnableContentDedup bool `json:"enable_content_dedup"`
	// 是否启用基于时间的去重
	EnableTimeDedup bool `json:"enable_time_dedup"`
}

// NewDeduplicationManager 创建去重管理器
func NewDeduplicationManager(windowDuration time.Duration) *DeduplicationManager {
	dm := &DeduplicationManager{
		cache:          make(map[string]*AlertRecord),
		windowDuration: windowDuration,
		stopChan:       make(chan struct{}),
	}
	
	// 启动清理定时器
	dm.startCleanup()
	
	return dm
}

// NewDeduplicationManagerWithConfig 使用配置创建去重管理器
func NewDeduplicationManagerWithConfig(config *DeduplicationConfig) *DeduplicationManager {
	if config == nil {
		config = &DeduplicationConfig{
			WindowDuration:     5 * time.Minute,
			CleanupInterval:    1 * time.Minute,
			MaxCacheSize:       10000,
			EnableContentDedup: true,
			EnableTimeDedup:    true,
		}
	}
	
	dm := &DeduplicationManager{
		cache:          make(map[string]*AlertRecord),
		windowDuration: config.WindowDuration,
		stopChan:       make(chan struct{}),
	}
	
	// 启动清理定时器
	dm.startCleanupWithInterval(config.CleanupInterval)
	
	return dm
}

// IsDuplicate 检查是否为重复告警
func (dm *DeduplicationManager) IsDuplicate(alertID string) bool {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	
	record, exists := dm.cache[alertID]
	if !exists {
		return false
	}
	
	// 检查是否在去重窗口内
	if time.Since(record.LastTriggered) <= dm.windowDuration {
		logger.WithFields(logrus.Fields{
			"alert_id":       alertID,
			"last_triggered": record.LastTriggered,
			"window":         dm.windowDuration,
			"trigger_count":  record.TriggerCount,
		}).Debug("Duplicate alert detected")
		return true
	}
	
	return false
}

// IsDuplicateWithContent 基于内容检查是否为重复告警
func (dm *DeduplicationManager) IsDuplicateWithContent(alertID string, content map[string]interface{}) bool {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	
	// 生成内容哈希
	contentHash := dm.generateContentHash(content)
	
	// 检查是否存在相同内容的告警
	for _, record := range dm.cache {
		if record.AlertHash == contentHash && time.Since(record.LastTriggered) <= dm.windowDuration {
			logger.WithFields(logrus.Fields{
				"alert_id":     alertID,
				"content_hash": contentHash,
				"existing_id":  record.AlertID,
				"window":       dm.windowDuration,
			}).Debug("Duplicate alert detected by content")
			return true
		}
	}
	
	return false
}

// RecordAlert 记录告警
func (dm *DeduplicationManager) RecordAlert(alertID string) {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	
	now := time.Now()
	
	if record, exists := dm.cache[alertID]; exists {
		// 更新现有记录
		record.LastTriggered = now
		record.TriggerCount++
		
		logger.WithFields(logrus.Fields{
			"alert_id":      alertID,
			"trigger_count": record.TriggerCount,
		}).Debug("Alert record updated")
	} else {
		// 创建新记录
		dm.cache[alertID] = &AlertRecord{
			AlertID:        alertID,
			FirstTriggered: now,
			LastTriggered:  now,
			TriggerCount:   1,
			AlertData:      make(map[string]interface{}),
		}
		
		logger.WithFields(logrus.Fields{
			"alert_id": alertID,
		}).Debug("New alert record created")
	}
}

// RecordAlertWithContent 记录带内容的告警
func (dm *DeduplicationManager) RecordAlertWithContent(alertID string, content map[string]interface{}) {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	
	now := time.Now()
	contentHash := dm.generateContentHash(content)
	
	if record, exists := dm.cache[alertID]; exists {
		// 更新现有记录
		record.LastTriggered = now
		record.TriggerCount++
		record.AlertHash = contentHash
		record.AlertData = content
	} else {
		// 创建新记录
		dm.cache[alertID] = &AlertRecord{
			AlertID:        alertID,
			AlertHash:      contentHash,
			FirstTriggered: now,
			LastTriggered:  now,
			TriggerCount:   1,
			AlertData:      content,
		}
	}
	
	logger.WithFields(logrus.Fields{
		"alert_id":     alertID,
		"content_hash": contentHash,
	}).Debug("Alert record with content created/updated")
}

// GetRecord 获取告警记录
func (dm *DeduplicationManager) GetRecord(alertID string) (*AlertRecord, bool) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	
	record, exists := dm.cache[alertID]
	if !exists {
		return nil, false
	}
	
	// 返回记录副本
	recordCopy := *record
	recordCopy.AlertData = make(map[string]interface{})
	for k, v := range record.AlertData {
		recordCopy.AlertData[k] = v
	}
	
	return &recordCopy, true
}

// RemoveRecord 移除告警记录
func (dm *DeduplicationManager) RemoveRecord(alertID string) {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	
	if _, exists := dm.cache[alertID]; exists {
		delete(dm.cache, alertID)
		logger.WithFields(logrus.Fields{
			"alert_id": alertID,
		}).Debug("Alert record removed")
	}
}

// GetCacheSize 获取缓存大小
func (dm *DeduplicationManager) GetCacheSize() int {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	return len(dm.cache)
}

// GetWindowDuration 获取去重窗口时间
func (dm *DeduplicationManager) GetWindowDuration() time.Duration {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	return dm.windowDuration
}

// UpdateWindowDuration 更新去重窗口时间
func (dm *DeduplicationManager) UpdateWindowDuration(duration time.Duration) {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	
	oldDuration := dm.windowDuration
	dm.windowDuration = duration
	
	logger.WithFields(logrus.Fields{
		"old_duration": oldDuration,
		"new_duration": duration,
	}).Info("Deduplication window duration updated")
}

// Clear 清空缓存
func (dm *DeduplicationManager) Clear() {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	
	cacheSize := len(dm.cache)
	dm.cache = make(map[string]*AlertRecord)
	
	logger.WithFields(logrus.Fields{
		"cleared_records": cacheSize,
	}).Info("Deduplication cache cleared")
}

// GetStats 获取去重统计信息
func (dm *DeduplicationManager) GetStats() map[string]interface{} {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	
	totalTriggers := 0
	oldestRecord := time.Now()
	newestRecord := time.Time{}
	
	for _, record := range dm.cache {
		totalTriggers += record.TriggerCount
		if record.FirstTriggered.Before(oldestRecord) {
			oldestRecord = record.FirstTriggered
		}
		if record.LastTriggered.After(newestRecord) {
			newestRecord = record.LastTriggered
		}
	}
	
	stats := map[string]interface{}{
		"cache_size":      len(dm.cache),
		"window_duration": dm.windowDuration.String(),
		"total_triggers":  totalTriggers,
		"is_running":      dm.isRunning,
	}
	
	if len(dm.cache) > 0 {
		stats["oldest_record"] = oldestRecord
		stats["newest_record"] = newestRecord
	}
	
	return stats
}

// Stop 停止去重管理器
func (dm *DeduplicationManager) Stop() {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	
	if dm.isRunning {
		close(dm.stopChan)
		if dm.cleanupTicker != nil {
			dm.cleanupTicker.Stop()
		}
		dm.isRunning = false
		
		logger.Info("Deduplication manager stopped")
	}
}

// startCleanup 启动清理定时器
func (dm *DeduplicationManager) startCleanup() {
	dm.startCleanupWithInterval(1 * time.Minute)
}

// startCleanupWithInterval 使用指定间隔启动清理定时器
func (dm *DeduplicationManager) startCleanupWithInterval(interval time.Duration) {
	dm.cleanupTicker = time.NewTicker(interval)
	dm.isRunning = true
	
	go func() {
		for {
			select {
			case <-dm.cleanupTicker.C:
				dm.cleanup()
			case <-dm.stopChan:
				return
			}
		}
	}()
	
	logger.WithFields(logrus.Fields{
		"cleanup_interval": interval,
		"window_duration":  dm.windowDuration,
	}).Info("Deduplication cleanup started")
}

// cleanup 清理过期记录
func (dm *DeduplicationManager) cleanup() {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	
	now := time.Now()
	expiredCount := 0
	
	for alertID, record := range dm.cache {
		if now.Sub(record.LastTriggered) > dm.windowDuration {
			delete(dm.cache, alertID)
			expiredCount++
		}
	}
	
	if expiredCount > 0 {
		logger.WithFields(logrus.Fields{
			"expired_records": expiredCount,
			"remaining_records": len(dm.cache),
		}).Debug("Expired alert records cleaned up")
	}
}

// generateContentHash 生成内容哈希
func (dm *DeduplicationManager) generateContentHash(content map[string]interface{}) string {
	// 简化的哈希生成，实际实现中可能需要更复杂的逻辑
	hash := sha256.New()
	
	// 按键排序以确保一致性
	keys := make([]string, 0, len(content))
	for k := range content {
		keys = append(keys, k)
	}
	
	for _, key := range keys {
		hash.Write([]byte(fmt.Sprintf("%s:%v", key, content[key])))
	}
	
	return fmt.Sprintf("%x", hash.Sum(nil))[:16] // 取前16位
}

// GetExpiredRecords 获取过期记录
func (dm *DeduplicationManager) GetExpiredRecords() []*AlertRecord {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	
	now := time.Now()
	var expiredRecords []*AlertRecord
	
	for _, record := range dm.cache {
		if now.Sub(record.LastTriggered) > dm.windowDuration {
			recordCopy := *record
			recordCopy.AlertData = make(map[string]interface{})
			for k, v := range record.AlertData {
				recordCopy.AlertData[k] = v
			}
			expiredRecords = append(expiredRecords, &recordCopy)
		}
	}
	
	return expiredRecords
}

// GetActiveRecords 获取活跃记录
func (dm *DeduplicationManager) GetActiveRecords() []*AlertRecord {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	
	now := time.Now()
	var activeRecords []*AlertRecord
	
	for _, record := range dm.cache {
		if now.Sub(record.LastTriggered) <= dm.windowDuration {
			recordCopy := *record
			recordCopy.AlertData = make(map[string]interface{})
			for k, v := range record.AlertData {
				recordCopy.AlertData[k] = v
			}
			activeRecords = append(activeRecords, &recordCopy)
		}
	}
	
	return activeRecords
}
