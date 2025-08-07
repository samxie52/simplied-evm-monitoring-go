package alert

import (
	"context"
	"fmt"
	"simplied-evm-monitoring-go/internal/models"
	"simplied-evm-monitoring-go/pkg/logger"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// RuleEngine 告警规则引擎
type RuleEngine struct {
	mu sync.RWMutex

	// 核心组件
	evaluator *RuleEvaluator
	rules     map[uint64]*models.AlertRule // 规则缓存，key 为规则ID
	
	// 配置
	config *RuleEngineConfig
	
	// 运行状态
	isRunning bool
	ctx       context.Context
	cancel    context.CancelFunc
	
	// 统计信息
	stats *RuleEngineStats
	
	// 回调函数
	onRuleTriggered func(*models.Alert) error
}

// RuleEngineConfig 规则引擎配置
type RuleEngineConfig struct {
	// 评估间隔
	EvaluationInterval time.Duration `json:"evaluation_interval"`
	
	// 最大并发规则数
	MaxConcurrentRules int `json:"max_concurrent_rules"`
	
	// 规则缓存大小
	RuleCacheSize int `json:"rule_cache_size"`
	
	// 性能监控
	EnablePerformanceMonitoring bool `json:"enable_performance_monitoring"`
	
	// 规则优先级处理
	EnablePriorityProcessing bool `json:"enable_priority_processing"`
	
	// 冷却时间检查间隔
	CooldownCheckInterval time.Duration `json:"cooldown_check_interval"`
}

// RuleEngineStats 规则引擎统计信息
type RuleEngineStats struct {
	mu sync.RWMutex
	
	// 规则统计
	TotalRules       int64 `json:"total_rules"`
	ActiveRules      int64 `json:"active_rules"`
	TriggeredRules   int64 `json:"triggered_rules"`
	
	// 评估统计
	TotalEvaluations int64 `json:"total_evaluations"`
	SuccessEvaluations int64 `json:"success_evaluations"`
	FailedEvaluations  int64 `json:"failed_evaluations"`
	
	// 性能统计
	AverageEvaluationTime time.Duration `json:"average_evaluation_time"`
	MaxEvaluationTime     time.Duration `json:"max_evaluation_time"`
	MinEvaluationTime     time.Duration `json:"min_evaluation_time"`
	
	// 告警统计
	GeneratedAlerts int64 `json:"generated_alerts"`
	
	// 时间戳
	LastEvaluationTime time.Time `json:"last_evaluation_time"`
	StartTime          time.Time `json:"start_time"`
}

// NewRuleEngine 创建新的规则引擎
func NewRuleEngine(config *RuleEngineConfig, onRuleTriggered func(*models.Alert) error) *RuleEngine {
	if config == nil {
		config = &RuleEngineConfig{
			EvaluationInterval:          10 * time.Second,
			MaxConcurrentRules:          10,
			RuleCacheSize:               1000,
			EnablePerformanceMonitoring: true,
			EnablePriorityProcessing:    true,
			CooldownCheckInterval:       5 * time.Second,
		}
	}

	return &RuleEngine{
		evaluator:       NewRuleEvaluator(),
		rules:          make(map[uint64]*models.AlertRule),
		config:         config,
		stats:          &RuleEngineStats{StartTime: time.Now()},
		onRuleTriggered: onRuleTriggered,
	}
}

// Start 启动规则引擎
func (re *RuleEngine) Start() error {
	re.mu.Lock()
	defer re.mu.Unlock()

	if re.isRunning {
		return fmt.Errorf("rule engine is already running")
	}

	re.ctx, re.cancel = context.WithCancel(context.Background())
	re.isRunning = true

	logger.Info("Starting rule engine...")

	// 启动评估循环
	go re.evaluationLoop()
	
	// 启动冷却时间检查循环
	go re.cooldownCheckLoop()

	logger.WithFields(logrus.Fields{
		"evaluation_interval":    re.config.EvaluationInterval,
		"max_concurrent_rules":   re.config.MaxConcurrentRules,
		"cooldown_check_interval": re.config.CooldownCheckInterval,
	}).Info("Rule engine started successfully")

	return nil
}

// Stop 停止规则引擎
func (re *RuleEngine) Stop() error {
	re.mu.Lock()
	defer re.mu.Unlock()

	if !re.isRunning {
		return fmt.Errorf("rule engine is not running")
	}

	logger.Info("Stopping rule engine...")

	re.cancel()
	re.isRunning = false

	logger.Info("Rule engine stopped")
	return nil
}

// IsRunning 检查引擎是否运行中
func (re *RuleEngine) IsRunning() bool {
	re.mu.RLock()
	defer re.mu.RUnlock()
	return re.isRunning
}

// AddRule 添加规则
func (re *RuleEngine) AddRule(rule *models.AlertRule) error {
	if rule == nil {
		return fmt.Errorf("rule cannot be nil")
	}

	if err := rule.Validate(); err != nil {
		return fmt.Errorf("invalid rule: %w", err)
	}

	re.mu.Lock()
	defer re.mu.Unlock()

	re.rules[rule.ID] = rule
	re.updateStats(func(stats *RuleEngineStats) {
		stats.TotalRules++
		if rule.Status == models.AlertStatusActive {
			stats.ActiveRules++
		}
	})

	logger.WithFields(logrus.Fields{
		"rule_id":   rule.ID,
		"rule_name": rule.Name,
		"rule_type": rule.Type,
		"severity":  rule.Severity,
	}).Info("Rule added to engine")

	return nil
}

// RemoveRule 移除规则
func (re *RuleEngine) RemoveRule(ruleID uint64) error {
	re.mu.Lock()
	defer re.mu.Unlock()

	rule, exists := re.rules[ruleID]
	if !exists {
		return fmt.Errorf("rule with ID %d not found", ruleID)
	}

	delete(re.rules, ruleID)
	re.updateStats(func(stats *RuleEngineStats) {
		stats.TotalRules--
		if rule.Status == models.AlertStatusActive {
			stats.ActiveRules--
		}
	})

	logger.WithFields(logrus.Fields{
		"rule_id":   ruleID,
		"rule_name": rule.Name,
	}).Info("Rule removed from engine")

	return nil
}

// UpdateRule 更新规则
func (re *RuleEngine) UpdateRule(rule *models.AlertRule) error {
	if rule == nil {
		return fmt.Errorf("rule cannot be nil")
	}

	if err := rule.Validate(); err != nil {
		return fmt.Errorf("invalid rule: %w", err)
	}

	re.mu.Lock()
	defer re.mu.Unlock()

	oldRule, exists := re.rules[rule.ID]
	if !exists {
		return fmt.Errorf("rule with ID %d not found", rule.ID)
	}

	// 更新统计信息
	re.updateStats(func(stats *RuleEngineStats) {
		if oldRule.Status == models.AlertStatusActive && rule.Status != models.AlertStatusActive {
			stats.ActiveRules--
		} else if oldRule.Status != models.AlertStatusActive && rule.Status == models.AlertStatusActive {
			stats.ActiveRules++
		}
	})

	re.rules[rule.ID] = rule

	logger.WithFields(logrus.Fields{
		"rule_id":   rule.ID,
		"rule_name": rule.Name,
		"old_status": oldRule.Status,
		"new_status": rule.Status,
	}).Info("Rule updated in engine")

	return nil
}

// GetRule 获取规则
func (re *RuleEngine) GetRule(ruleID uint64) (*models.AlertRule, bool) {
	re.mu.RLock()
	defer re.mu.RUnlock()
	
	rule, exists := re.rules[ruleID]
	return rule, exists
}

// GetAllRules 获取所有规则
func (re *RuleEngine) GetAllRules() []*models.AlertRule {
	re.mu.RLock()
	defer re.mu.RUnlock()
	
	rules := make([]*models.AlertRule, 0, len(re.rules))
	for _, rule := range re.rules {
		rules = append(rules, rule)
	}
	return rules
}

// GetActiveRules 获取激活的规则
func (re *RuleEngine) GetActiveRules() []*models.AlertRule {
	re.mu.RLock()
	defer re.mu.RUnlock()
	
	var activeRules []*models.AlertRule
	for _, rule := range re.rules {
		if rule.Status == models.AlertStatusActive {
			activeRules = append(activeRules, rule)
		}
	}
	return activeRules
}

// EvaluateRule 评估单个规则
func (re *RuleEngine) EvaluateRule(ctx context.Context, rule *models.AlertRule, data map[string]interface{}) (*models.Alert, error) {
	if !re.IsRunning() {
		return nil, fmt.Errorf("rule engine is not running")
	}

	startTime := time.Now()
	defer func() {
		evaluationTime := time.Since(startTime)
		re.updateEvaluationStats(evaluationTime, nil)
	}()

	// 检查规则是否可以触发（考虑冷却时间）
	if !rule.CanTrigger() {
		logger.WithFields(logrus.Fields{
			"rule_id": rule.ID,
			"rule_name": rule.Name,
			"last_triggered": rule.LastTriggered,
			"cooldown": rule.Cooldown,
		}).Debug("Rule is in cooldown period")
		return nil, nil
	}

	// 更新最后检查时间
	rule.UpdateLastChecked()

	// 使用评估器评估规则
	triggered, triggerData, err := re.evaluator.EvaluateRule(ctx, rule, data)
	if err != nil {
		re.updateEvaluationStats(0, err)
		return nil, fmt.Errorf("failed to evaluate rule %d: %w", rule.ID, err)
	}

	if !triggered {
		return nil, nil
	}

	// 规则被触发，创建告警
	alert, err := re.createAlert(rule, triggerData)
	if err != nil {
		return nil, fmt.Errorf("failed to create alert for rule %d: %w", rule.ID, err)
	}

	// 更新规则触发统计
	rule.IncrementTriggerCount()
	re.updateStats(func(stats *RuleEngineStats) {
		stats.TriggeredRules++
		stats.GeneratedAlerts++
	})

	logger.WithFields(logrus.Fields{
		"rule_id":      rule.ID,
		"rule_name":    rule.Name,
		"alert_id":     alert.ID,
		"trigger_value": alert.TriggerValue,
		"severity":     alert.Severity,
	}).Info("Rule triggered, alert generated")

	return alert, nil
}

// evaluationLoop 评估循环
func (re *RuleEngine) evaluationLoop() {
	ticker := time.NewTicker(re.config.EvaluationInterval)
	defer ticker.Stop()

	for {
		select {
		case <-re.ctx.Done():
			return
		case <-ticker.C:
			re.evaluateAllRules()
		}
	}
}

// cooldownCheckLoop 冷却时间检查循环
func (re *RuleEngine) cooldownCheckLoop() {
	// 确保冷却检查间隔不为 0
	checkInterval := re.config.CooldownCheckInterval
	if checkInterval <= 0 {
		checkInterval = 5 * time.Second
	}
	
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-re.ctx.Done():
			return
		case <-ticker.C:
			re.checkCooldownExpiry()
		}
	}
}

// evaluateAllRules 评估所有激活的规则
func (re *RuleEngine) evaluateAllRules() {
	activeRules := re.GetActiveRules()
	if len(activeRules) == 0 {
		return
	}

	logger.WithFields(logrus.Fields{"active_rules": len(activeRules)}).Debug("Evaluating all active rules")

	// 使用信号量控制并发数
	semaphore := make(chan struct{}, re.config.MaxConcurrentRules)
	var wg sync.WaitGroup

	for _, rule := range activeRules {
		wg.Add(1)
		go func(r *models.AlertRule) {
			defer wg.Done()
			
			semaphore <- struct{}{} // 获取信号量
			defer func() { <-semaphore }() // 释放信号量

			// 这里需要实际的数据源，暂时使用空数据
			data := make(map[string]interface{})
			
			alert, err := re.EvaluateRule(re.ctx, r, data)
			if err != nil {
				logger.WithFields(logrus.Fields{
					"rule_id": r.ID,
					"error":   err,
				}).Error("Failed to evaluate rule")
				return
			}

			if alert != nil && re.onRuleTriggered != nil {
				if err := re.onRuleTriggered(alert); err != nil {
					logger.WithFields(logrus.Fields{
						"alert_id": alert.ID,
						"rule_id":  r.ID,
						"error":    err,
					}).Error("Failed to handle triggered rule")
				}
			}
		}(rule)
	}

	wg.Wait()
	
	re.updateStats(func(stats *RuleEngineStats) {
		stats.LastEvaluationTime = time.Now()
	})
}

// checkCooldownExpiry 检查冷却时间过期
func (re *RuleEngine) checkCooldownExpiry() {
	re.mu.RLock()
	defer re.mu.RUnlock()

	expiredCount := 0
	for _, rule := range re.rules {
		if rule.LastTriggered != nil {
			cooldownDuration := time.Duration(rule.Cooldown) * time.Second
			if time.Since(*rule.LastTriggered) >= cooldownDuration {
				expiredCount++
			}
		}
	}

	if expiredCount > 0 {
		logger.WithFields(logrus.Fields{"expired_cooldowns": expiredCount}).Debug("Cooldown periods expired")
	}
}

// createAlert 创建告警
func (re *RuleEngine) createAlert(rule *models.AlertRule, triggerData *models.AlertTriggerData) (*models.Alert, error) {
	alert := &models.Alert{
		RuleID:       rule.ID,
		Type:         rule.Type,
		Severity:     rule.Severity,
		Title:        fmt.Sprintf("规则告警: %s", rule.Name),
		Message:      fmt.Sprintf("规则 %s 被触发", rule.Name),
		TriggerTime:  time.Now(),
		Status:       models.NotificationStatusPending,
	}

	// 设置触发数据
	if triggerData != nil {
		if err := alert.SetTriggerData(triggerData); err != nil {
			return nil, fmt.Errorf("failed to set trigger data: %w", err)
		}
		
		// 设置触发值
		if triggerData.MatchedValue != nil {
			if val, ok := triggerData.MatchedValue.(float64); ok {
				alert.TriggerValue = val
			}
		}
	}

	return alert, nil
}

// updateStats 更新统计信息
func (re *RuleEngine) updateStats(updateFunc func(*RuleEngineStats)) {
	re.stats.mu.Lock()
	defer re.stats.mu.Unlock()
	updateFunc(re.stats)
}

// updateEvaluationStats 更新评估统计信息
func (re *RuleEngine) updateEvaluationStats(evaluationTime time.Duration, err error) {
	re.updateStats(func(stats *RuleEngineStats) {
		stats.TotalEvaluations++
		
		if err != nil {
			stats.FailedEvaluations++
		} else {
			stats.SuccessEvaluations++
		}

		if evaluationTime > 0 {
			// 更新评估时间统计
			if stats.MinEvaluationTime == 0 || evaluationTime < stats.MinEvaluationTime {
				stats.MinEvaluationTime = evaluationTime
			}
			if evaluationTime > stats.MaxEvaluationTime {
				stats.MaxEvaluationTime = evaluationTime
			}
			
			// 计算平均评估时间
			if stats.SuccessEvaluations > 0 {
				totalTime := stats.AverageEvaluationTime * time.Duration(stats.SuccessEvaluations-1)
				stats.AverageEvaluationTime = (totalTime + evaluationTime) / time.Duration(stats.SuccessEvaluations)
			}
		}
	})
}

// GetStats 获取统计信息
func (re *RuleEngine) GetStats() map[string]interface{} {
	re.stats.mu.RLock()
	defer re.stats.mu.RUnlock()

	return map[string]interface{}{
		"is_running":               re.IsRunning(),
		"total_rules":              re.stats.TotalRules,
		"active_rules":             re.stats.ActiveRules,
		"triggered_rules":          re.stats.TriggeredRules,
		"total_evaluations":        re.stats.TotalEvaluations,
		"success_evaluations":      re.stats.SuccessEvaluations,
		"failed_evaluations":       re.stats.FailedEvaluations,
		"generated_alerts":         re.stats.GeneratedAlerts,
		"average_evaluation_time":  re.stats.AverageEvaluationTime.String(),
		"max_evaluation_time":      re.stats.MaxEvaluationTime.String(),
		"min_evaluation_time":      re.stats.MinEvaluationTime.String(),
		"last_evaluation_time":     re.stats.LastEvaluationTime,
		"start_time":               re.stats.StartTime,
		"uptime":                   time.Since(re.stats.StartTime).String(),
	}
}

// GetConfig 获取配置
func (re *RuleEngine) GetConfig() *RuleEngineConfig {
	re.mu.RLock()
	defer re.mu.RUnlock()
	
	// 返回配置副本
	configCopy := *re.config
	return &configCopy
}

// UpdateConfig 更新配置
func (re *RuleEngine) UpdateConfig(config *RuleEngineConfig) error {
	if config == nil {
		return fmt.Errorf("config cannot be nil")
	}

	re.mu.Lock()
	defer re.mu.Unlock()

	oldConfig := *re.config
	re.config = config

	logger.WithFields(logrus.Fields{
		"old_evaluation_interval": oldConfig.EvaluationInterval,
		"new_evaluation_interval": config.EvaluationInterval,
		"old_max_concurrent":      oldConfig.MaxConcurrentRules,
		"new_max_concurrent":      config.MaxConcurrentRules,
	}).Info("Rule engine configuration updated")

	return nil
}
