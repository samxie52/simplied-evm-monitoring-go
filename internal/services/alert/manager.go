package alert

import (
	"context"
	"fmt"
	"simplied-evm-monitoring-go/internal/models"
	"simplied-evm-monitoring-go/pkg/logger"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/sirupsen/logrus"
)

// AlertManager 告警管理器
type AlertManager struct {
	// 大额交易检测器
	largeTransactionDetector *LargeTransactionDetector
	// 告警规则引擎
	ruleEngine *RuleEngine
	// 告警存储
	alertStore AlertStore
	// 通知管理器
	notificationManager NotificationManager
	// 配置
	config *AlertManagerConfig
	// 运行状态
	isRunning bool
	mu        sync.RWMutex
	// 停止信号
	stopChan chan struct{}
	// 告警队列
	alertQueue chan *models.Alert
	// 工作协程数量
	workerCount int
}

// AlertManagerConfig 告警管理器配置
type AlertManagerConfig struct {
	// 大额交易检测配置
	LargeTransactionConfig *DetectorConfig `json:"large_transaction_config"`
	// 规则引擎配置
	RuleEngineConfig *RuleEngineConfig `json:"rule_engine_config"`
	// 告警队列大小
	AlertQueueSize int `json:"alert_queue_size"`
	// 工作协程数量
	WorkerCount int `json:"worker_count"`
	// 告警处理超时
	ProcessTimeout time.Duration `json:"process_timeout"`
	// 批处理大小
	BatchSize int `json:"batch_size"`
	// 批处理间隔
	BatchInterval time.Duration `json:"batch_interval"`
}

// AlertStore 告警存储接口
type AlertStore interface {
	// SaveAlert 保存告警
	SaveAlert(ctx context.Context, alert *models.Alert) error
	// GetAlert 获取告警
	GetAlert(ctx context.Context, id uint64) (*models.Alert, error)
	// ListAlerts 列出告警
	ListAlerts(ctx context.Context, params *models.AlertQueryParams) ([]*models.Alert, error)
	// UpdateAlertStatus 更新告警状态
	UpdateAlertStatus(ctx context.Context, id uint64, status models.NotificationStatus) error
}

// NotificationManager 通知管理器接口
type NotificationManager interface {
	// SendNotification 发送通知
	SendNotification(ctx context.Context, alert *models.Alert) error
	// GetSupportedChannels 获取支持的通知渠道
	GetSupportedChannels() []models.NotificationChannel
}



// NewAlertManager 创建告警管理器
func NewAlertManager(config *AlertManagerConfig, alertStore AlertStore, notificationManager NotificationManager) *AlertManager {
	if config == nil {
		config = &AlertManagerConfig{
			LargeTransactionConfig: &DetectorConfig{
				ETHThreshold:       10.0,
				EnableETHThreshold: true,
				BatchSize:          50,
				MaxConcurrency:     5,
			},
			RuleEngineConfig: &RuleEngineConfig{
				EvaluationInterval:          10 * time.Second,
				MaxConcurrentRules:          10,
				RuleCacheSize:               1000,
				EnablePerformanceMonitoring: true,
				EnablePriorityProcessing:    true,
				CooldownCheckInterval:       5 * time.Second,
			},
			AlertQueueSize: 1000,
			WorkerCount:    5,
			ProcessTimeout: 30 * time.Second,
			BatchSize:      10,
			BatchInterval:  5 * time.Second,
		}
	}

	am := &AlertManager{
		alertStore:          alertStore,
		notificationManager: notificationManager,
		config:              config,
		stopChan:            make(chan struct{}),
		alertQueue:          make(chan *models.Alert, config.AlertQueueSize),
		workerCount:         config.WorkerCount,
	}

	// 创建大额交易检测器
	am.largeTransactionDetector = NewLargeTransactionDetector(
		config.LargeTransactionConfig,
		am.handleAlert,
	)

	// 创建规则引擎
	am.ruleEngine = NewRuleEngine(
		config.RuleEngineConfig,
		am.handleAlert,
	)

	return am
}

// Start 启动告警管理器
func (am *AlertManager) Start() error {
	am.mu.Lock()
	defer am.mu.Unlock()

	if am.isRunning {
		return fmt.Errorf("alert manager is already running")
	}

	// 启动大额交易检测器
	if err := am.largeTransactionDetector.Start(); err != nil {
		return fmt.Errorf("failed to start large transaction detector: %w", err)
	}

	// 启动规则引擎
	if err := am.ruleEngine.Start(); err != nil {
		return fmt.Errorf("failed to start rule engine: %w", err)
	}

	// 启动告警处理工作协程
	for i := 0; i < am.workerCount; i++ {
		go am.alertWorker(i)
	}

	am.isRunning = true

	logger.WithFields(logrus.Fields{
		"worker_count":    am.workerCount,
		"queue_size":      am.config.AlertQueueSize,
		"process_timeout": am.config.ProcessTimeout,
	}).Info("Alert manager started")

	return nil
}

// Stop 停止告警管理器
func (am *AlertManager) Stop() error {
	am.mu.Lock()
	defer am.mu.Unlock()

	if !am.isRunning {
		return fmt.Errorf("alert manager is not running")
	}

	// 停止大额交易检测器
	if err := am.largeTransactionDetector.Stop(); err != nil {
		logger.WithFields(logrus.Fields{"error": err}).Error("Failed to stop large transaction detector")
	}

	// 停止规则引擎
	if err := am.ruleEngine.Stop(); err != nil {
		logger.WithFields(logrus.Fields{"error": err}).Error("Failed to stop rule engine")
	}

	// 发送停止信号
	close(am.stopChan)

	// 等待告警队列处理完成
	close(am.alertQueue)

	am.isRunning = false

	logger.Info("Alert manager stopped")

	return nil
}

// IsRunning 检查是否运行中
func (am *AlertManager) IsRunning() bool {
	am.mu.RLock()
	defer am.mu.RUnlock()
	return am.isRunning
}

// ProcessTransactions 处理交易数据
func (am *AlertManager) ProcessTransactions(ctx context.Context, transactions []*types.Transaction, receipts []*types.Receipt) error {
	if !am.IsRunning() {
		return fmt.Errorf("alert manager is not running")
	}

	// 使用大额交易检测器处理交易
	alerts, err := am.largeTransactionDetector.DetectLargeTransactions(ctx, transactions, receipts)
	if err != nil {
		return fmt.Errorf("failed to detect large transactions: %w", err)
	}

	// 处理检测到的告警
	for _, alert := range alerts {
		if err := am.largeTransactionDetector.ProcessAlert(alert); err != nil {
			logger.WithFields(logrus.Fields{
				"error":   err,
				"tx_hash": alert.TransactionHash,
			}).Error("Failed to process alert")
		}
	}

	logger.WithFields(logrus.Fields{
		"processed_transactions": len(transactions),
		"detected_alerts":        len(alerts),
	}).Debug("Transactions processed for alerts")

	return nil
}

// handleAlert 处理告警
func (am *AlertManager) handleAlert(alert *models.Alert) error {
	select {
	case am.alertQueue <- alert:
		logger.WithFields(logrus.Fields{
			"alert_type": alert.Type,
			"severity":   alert.Severity,
		}).Debug("Alert queued for processing")
		return nil
	case <-time.After(5 * time.Second):
		return fmt.Errorf("alert queue is full, dropping alert")
	}
}

// alertWorker 告警处理工作协程
func (am *AlertManager) alertWorker(workerID int) {
	logger.WithFields(logrus.Fields{
		"worker_id": workerID,
	}).Debug("Alert worker started")

	for {
		select {
		case alert, ok := <-am.alertQueue:
			if !ok {
				logger.WithFields(logrus.Fields{
					"worker_id": workerID,
				}).Debug("Alert worker stopped")
				return
			}

			if err := am.processAlert(alert); err != nil {
				logger.WithFields(logrus.Fields{
					"worker_id":  workerID,
					"alert_type": alert.Type,
					"error":      err,
				}).Error("Failed to process alert")
			}

		case <-am.stopChan:
			logger.WithFields(logrus.Fields{
				"worker_id": workerID,
			}).Debug("Alert worker received stop signal")
			return
		}
	}
}

// processAlert 处理单个告警
func (am *AlertManager) processAlert(alert *models.Alert) error {
	ctx, cancel := context.WithTimeout(context.Background(), am.config.ProcessTimeout)
	defer cancel()

	// 保存告警到存储
	if am.alertStore != nil {
		if err := am.alertStore.SaveAlert(ctx, alert); err != nil {
			return fmt.Errorf("failed to save alert: %w", err)
		}
	}

	// 发送通知
	if am.notificationManager != nil {
		if err := am.notificationManager.SendNotification(ctx, alert); err != nil {
			// 更新告警状态为发送失败
			if am.alertStore != nil {
				am.alertStore.UpdateAlertStatus(ctx, alert.ID, models.NotificationStatusFailed)
			}
			return fmt.Errorf("failed to send notification: %w", err)
		}

		// 更新告警状态为已发送
		if am.alertStore != nil {
			am.alertStore.UpdateAlertStatus(ctx, alert.ID, models.NotificationStatusSent)
		}
	}

	logger.WithFields(logrus.Fields{
		"alert_id":   alert.ID,
		"alert_type": alert.Type,
		"severity":   alert.Severity,
	}).Info("Alert processed successfully")

	return nil
}

// GetLargeTransactionDetector 获取大额交易检测器
func (am *AlertManager) GetLargeTransactionDetector() *LargeTransactionDetector {
	return am.largeTransactionDetector
}

// UpdateLargeTransactionConfig 更新大额交易检测配置
func (am *AlertManager) UpdateLargeTransactionConfig(config *DetectorConfig) error {
	if am.largeTransactionDetector == nil {
		return fmt.Errorf("large transaction detector not initialized")
	}

	return am.largeTransactionDetector.UpdateConfig(config)
}

// GetStats 获取告警管理器统计信息
func (am *AlertManager) GetStats() map[string]interface{} {
	am.mu.RLock()
	defer am.mu.RUnlock()

	stats := map[string]interface{}{
		"is_running":      am.isRunning,
		"worker_count":    am.workerCount,
		"queue_size":      am.config.AlertQueueSize,
		"queue_length":    len(am.alertQueue),
		"process_timeout": am.config.ProcessTimeout.String(),
	}

	// 添加大额交易检测器统计
	if am.largeTransactionDetector != nil {
		stats["large_transaction_detector"] = am.largeTransactionDetector.GetStats()
	}

	// 添加规则引擎统计
	if am.ruleEngine != nil {
		stats["rule_engine"] = am.ruleEngine.GetStats()
	}

	return stats
}

// ========== 规则引擎管理方法 ==========

// GetRuleEngine 获取规则引擎
func (am *AlertManager) GetRuleEngine() *RuleEngine {
	return am.ruleEngine
}

// AddRule 添加告警规则
func (am *AlertManager) AddRule(rule *models.AlertRule) error {
	if am.ruleEngine == nil {
		return fmt.Errorf("rule engine not initialized")
	}

	return am.ruleEngine.AddRule(rule)
}

// RemoveRule 移除告警规则
func (am *AlertManager) RemoveRule(ruleID uint64) error {
	if am.ruleEngine == nil {
		return fmt.Errorf("rule engine not initialized")
	}

	return am.ruleEngine.RemoveRule(ruleID)
}

// UpdateRule 更新告警规则
func (am *AlertManager) UpdateRule(rule *models.AlertRule) error {
	if am.ruleEngine == nil {
		return fmt.Errorf("rule engine not initialized")
	}

	return am.ruleEngine.UpdateRule(rule)
}

// GetRule 获取告警规则
func (am *AlertManager) GetRule(ruleID uint64) (*models.AlertRule, bool) {
	if am.ruleEngine == nil {
		return nil, false
	}

	return am.ruleEngine.GetRule(ruleID)
}

// GetAllRules 获取所有规则
func (am *AlertManager) GetAllRules() []*models.AlertRule {
	if am.ruleEngine == nil {
		return nil
	}

	return am.ruleEngine.GetAllRules()
}

// GetActiveRules 获取激活的规则
func (am *AlertManager) GetActiveRules() []*models.AlertRule {
	if am.ruleEngine == nil {
		return nil
	}

	return am.ruleEngine.GetActiveRules()
}

// EvaluateRule 评估单个规则
func (am *AlertManager) EvaluateRule(ctx context.Context, rule *models.AlertRule, data map[string]interface{}) (*models.Alert, error) {
	if am.ruleEngine == nil {
		return nil, fmt.Errorf("rule engine not initialized")
	}

	return am.ruleEngine.EvaluateRule(ctx, rule, data)
}

// UpdateRuleEngineConfig 更新规则引擎配置
func (am *AlertManager) UpdateRuleEngineConfig(config *RuleEngineConfig) error {
	if am.ruleEngine == nil {
		return fmt.Errorf("rule engine not initialized")
	}

	return am.ruleEngine.UpdateConfig(config)
}

// ProcessRuleEvaluation 处理规则评估数据
func (am *AlertManager) ProcessRuleEvaluation(ctx context.Context, data map[string]interface{}) error {
	if am.ruleEngine == nil {
		return fmt.Errorf("rule engine not initialized")
	}

	if !am.ruleEngine.IsRunning() {
		return fmt.Errorf("rule engine is not running")
	}

	// 获取激活的规则
	activeRules := am.ruleEngine.GetActiveRules()
	if len(activeRules) == 0 {
		return nil
	}

	// 评估所有激活规则
	for _, rule := range activeRules {
		alert, err := am.ruleEngine.EvaluateRule(ctx, rule, data)
		if err != nil {
			logger.WithFields(logrus.Fields{
				"rule_id": rule.ID,
				"error":   err,
			}).Error("Failed to evaluate rule")
			continue
		}

		if alert != nil {
			// 规则被触发，处理告警
			if err := am.handleAlert(alert); err != nil {
				logger.WithFields(logrus.Fields{
					"alert_id": alert.ID,
					"rule_id":  rule.ID,
					"error":    err,
				}).Error("Failed to handle rule-triggered alert")
			}
		}
	}

	return nil
}

// GetQueueStatus 获取队列状态
func (am *AlertManager) GetQueueStatus() map[string]interface{} {
	am.mu.RLock()
	defer am.mu.RUnlock()

	return map[string]interface{}{
		"queue_capacity": am.config.AlertQueueSize,
		"queue_length":   len(am.alertQueue),
		"queue_usage":    float64(len(am.alertQueue)) / float64(am.config.AlertQueueSize) * 100,
	}
}

// HealthCheck 健康检查
func (am *AlertManager) HealthCheck() error {
	if !am.IsRunning() {
		return fmt.Errorf("alert manager is not running")
	}

	// 检查队列是否接近满载
	queueUsage := float64(len(am.alertQueue)) / float64(am.config.AlertQueueSize) * 100
	if queueUsage > 90 {
		return fmt.Errorf("alert queue usage is too high: %.2f%%", queueUsage)
	}

	// 检查大额交易检测器
	if am.largeTransactionDetector != nil && !am.largeTransactionDetector.IsRunning() {
		return fmt.Errorf("large transaction detector is not running")
	}

	return nil
}
