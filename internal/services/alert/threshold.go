package alert

import (
	"fmt"
	"simplied-evm-monitoring-go/pkg/logger"
	"sync"

	"github.com/sirupsen/logrus"
)

// ThresholdManager 阈值管理器
type ThresholdManager struct {
	// ETH 阈值
	ethThreshold float64
	// USD 阈值
	usdThreshold float64
	// 动态阈值配置
	dynamicConfig *DynamicThresholdConfig
	// 读写锁
	mu sync.RWMutex
}

// DynamicThresholdConfig 动态阈值配置
type DynamicThresholdConfig struct {
	// 是否启用动态阈值
	Enabled bool `json:"enabled"`
	// 基础倍数
	BaseMultiplier float64 `json:"base_multiplier"`
	// 时间窗口内的交易量阈值
	VolumeThreshold int `json:"volume_threshold"`
	// 时间窗口（秒）
	TimeWindow int `json:"time_window"`
	// 最小阈值
	MinThreshold float64 `json:"min_threshold"`
	// 最大阈值
	MaxThreshold float64 `json:"max_threshold"`
}

// ThresholdType 阈值类型
type ThresholdType string

const (
	ThresholdTypeETH ThresholdType = "ETH"
	ThresholdTypeUSD ThresholdType = "USD"
)

// ThresholdLevel 阈值级别
type ThresholdLevel string

const (
	ThresholdLevelLow    ThresholdLevel = "low"
	ThresholdLevelMedium ThresholdLevel = "medium"
	ThresholdLevelHigh   ThresholdLevel = "high"
	ThresholdLevelCritical ThresholdLevel = "critical"
)

// ThresholdConfig 阈值配置
type ThresholdConfig struct {
	Type      ThresholdType  `json:"type"`
	Level     ThresholdLevel `json:"level"`
	Value     float64        `json:"value"`
	Enabled   bool           `json:"enabled"`
	CreatedAt int64          `json:"created_at"`
	UpdatedAt int64          `json:"updated_at"`
}

// NewThresholdManager 创建阈值管理器
func NewThresholdManager(ethThreshold, usdThreshold float64) *ThresholdManager {
	return &ThresholdManager{
		ethThreshold: ethThreshold,
		usdThreshold: usdThreshold,
		dynamicConfig: &DynamicThresholdConfig{
			Enabled:         false,
			BaseMultiplier:  1.0,
			VolumeThreshold: 100,
			TimeWindow:      3600, // 1小时
			MinThreshold:    10.0,
			MaxThreshold:    10000.0,
		},
	}
}

// IsAboveETHThreshold 检查是否超过 ETH 阈值
func (tm *ThresholdManager) IsAboveETHThreshold(value float64) bool {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	
	threshold := tm.ethThreshold
	
	// 如果启用动态阈值，计算动态阈值
	if tm.dynamicConfig.Enabled {
		threshold = tm.calculateDynamicThreshold(threshold, ThresholdTypeETH)
	}
	
	isAbove := value >= threshold
	
	if isAbove {
		logger.WithFields(logrus.Fields{
			"value":     value,
			"threshold": threshold,
			"type":      "ETH",
			"dynamic":   tm.dynamicConfig.Enabled,
		}).Debug("Value above ETH threshold")
	}
	
	return isAbove
}

// IsAboveUSDThreshold 检查是否超过 USD 阈值
func (tm *ThresholdManager) IsAboveUSDThreshold(value float64) bool {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	
	threshold := tm.usdThreshold
	
	// 如果启用动态阈值，计算动态阈值
	if tm.dynamicConfig.Enabled {
		threshold = tm.calculateDynamicThreshold(threshold, ThresholdTypeUSD)
	}
	
	isAbove := value >= threshold
	
	if isAbove {
		logger.WithFields(logrus.Fields{
			"value":     value,
			"threshold": threshold,
			"type":      "USD",
			"dynamic":   tm.dynamicConfig.Enabled,
		}).Debug("Value above USD threshold")
	}
	
	return isAbove
}

// GetETHThreshold 获取当前 ETH 阈值
func (tm *ThresholdManager) GetETHThreshold() float64 {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	
	if tm.dynamicConfig.Enabled {
		return tm.calculateDynamicThreshold(tm.ethThreshold, ThresholdTypeETH)
	}
	
	return tm.ethThreshold
}

// GetUSDThreshold 获取当前 USD 阈值
func (tm *ThresholdManager) GetUSDThreshold() float64 {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	
	if tm.dynamicConfig.Enabled {
		return tm.calculateDynamicThreshold(tm.usdThreshold, ThresholdTypeUSD)
	}
	
	return tm.usdThreshold
}

// UpdateThresholds 更新阈值
func (tm *ThresholdManager) UpdateThresholds(ethThreshold, usdThreshold float64) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	
	if ethThreshold < 0 || usdThreshold < 0 {
		return fmt.Errorf("thresholds must be non-negative")
	}
	
	oldETH := tm.ethThreshold
	oldUSD := tm.usdThreshold
	
	tm.ethThreshold = ethThreshold
	tm.usdThreshold = usdThreshold
	
	logger.WithFields(logrus.Fields{
		"old_eth_threshold": oldETH,
		"new_eth_threshold": ethThreshold,
		"old_usd_threshold": oldUSD,
		"new_usd_threshold": usdThreshold,
	}).Info("Thresholds updated")
	
	return nil
}

// UpdateETHThreshold 更新 ETH 阈值
func (tm *ThresholdManager) UpdateETHThreshold(threshold float64) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	
	if threshold < 0 {
		return fmt.Errorf("ETH threshold must be non-negative")
	}
	
	oldThreshold := tm.ethThreshold
	tm.ethThreshold = threshold
	
	logger.WithFields(logrus.Fields{
		"old_threshold": oldThreshold,
		"new_threshold": threshold,
		"type":          "ETH",
	}).Info("ETH threshold updated")
	
	return nil
}

// UpdateUSDThreshold 更新 USD 阈值
func (tm *ThresholdManager) UpdateUSDThreshold(threshold float64) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	
	if threshold < 0 {
		return fmt.Errorf("USD threshold must be non-negative")
	}
	
	oldThreshold := tm.usdThreshold
	tm.usdThreshold = threshold
	
	logger.WithFields(logrus.Fields{
		"old_threshold": oldThreshold,
		"new_threshold": threshold,
		"type":          "USD",
	}).Info("USD threshold updated")
	
	return nil
}

// EnableDynamicThreshold 启用动态阈值
func (tm *ThresholdManager) EnableDynamicThreshold(config *DynamicThresholdConfig) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	
	if config == nil {
		return fmt.Errorf("dynamic threshold config cannot be nil")
	}
	
	if err := tm.validateDynamicConfig(config); err != nil {
		return fmt.Errorf("invalid dynamic threshold config: %w", err)
	}
	
	tm.dynamicConfig = config
	tm.dynamicConfig.Enabled = true
	
	logger.WithFields(logrus.Fields{
		"base_multiplier":  config.BaseMultiplier,
		"volume_threshold": config.VolumeThreshold,
		"time_window":      config.TimeWindow,
		"min_threshold":    config.MinThreshold,
		"max_threshold":    config.MaxThreshold,
	}).Info("Dynamic threshold enabled")
	
	return nil
}

// DisableDynamicThreshold 禁用动态阈值
func (tm *ThresholdManager) DisableDynamicThreshold() {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	
	tm.dynamicConfig.Enabled = false
	
	logger.Info("Dynamic threshold disabled")
}

// GetThresholdLevel 获取阈值级别
func (tm *ThresholdManager) GetThresholdLevel(value float64, thresholdType ThresholdType) ThresholdLevel {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	
	var baseThreshold float64
	switch thresholdType {
	case ThresholdTypeETH:
		baseThreshold = tm.ethThreshold
	case ThresholdTypeUSD:
		baseThreshold = tm.usdThreshold
	default:
		return ThresholdLevelLow
	}
	
	// 计算相对于基础阈值的倍数
	multiplier := value / baseThreshold
	
	switch {
	case multiplier >= 10.0:
		return ThresholdLevelCritical
	case multiplier >= 5.0:
		return ThresholdLevelHigh
	case multiplier >= 2.0:
		return ThresholdLevelMedium
	case multiplier >= 1.0:
		return ThresholdLevelLow
	default:
		return ThresholdLevelLow
	}
}

// GetThresholdConfigs 获取所有阈值配置
func (tm *ThresholdManager) GetThresholdConfigs() map[string]*ThresholdConfig {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	
	configs := make(map[string]*ThresholdConfig)
	
	configs["eth"] = &ThresholdConfig{
		Type:    ThresholdTypeETH,
		Value:   tm.ethThreshold,
		Enabled: true,
	}
	
	configs["usd"] = &ThresholdConfig{
		Type:    ThresholdTypeUSD,
		Value:   tm.usdThreshold,
		Enabled: true,
	}
	
	return configs
}

// GetDynamicConfig 获取动态阈值配置
func (tm *ThresholdManager) GetDynamicConfig() *DynamicThresholdConfig {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	
	// 返回配置副本
	config := *tm.dynamicConfig
	return &config
}

// calculateDynamicThreshold 计算动态阈值
func (tm *ThresholdManager) calculateDynamicThreshold(baseThreshold float64, thresholdType ThresholdType) float64 {
	// 简化的动态阈值计算
	// 实际实现中可能需要考虑历史交易量、市场波动等因素
	
	dynamicThreshold := baseThreshold * tm.dynamicConfig.BaseMultiplier
	
	// 确保在最小和最大阈值范围内
	if dynamicThreshold < tm.dynamicConfig.MinThreshold {
		dynamicThreshold = tm.dynamicConfig.MinThreshold
	}
	if dynamicThreshold > tm.dynamicConfig.MaxThreshold {
		dynamicThreshold = tm.dynamicConfig.MaxThreshold
	}
	
	logger.WithFields(logrus.Fields{
		"base_threshold":    baseThreshold,
		"dynamic_threshold": dynamicThreshold,
		"multiplier":        tm.dynamicConfig.BaseMultiplier,
		"type":              thresholdType,
	}).Debug("Dynamic threshold calculated")
	
	return dynamicThreshold
}

// validateDynamicConfig 验证动态阈值配置
func (tm *ThresholdManager) validateDynamicConfig(config *DynamicThresholdConfig) error {
	if config.BaseMultiplier <= 0 {
		return fmt.Errorf("base multiplier must be positive")
	}
	
	if config.VolumeThreshold <= 0 {
		return fmt.Errorf("volume threshold must be positive")
	}
	
	if config.TimeWindow <= 0 {
		return fmt.Errorf("time window must be positive")
	}
	
	if config.MinThreshold < 0 {
		return fmt.Errorf("min threshold must be non-negative")
	}
	
	if config.MaxThreshold <= config.MinThreshold {
		return fmt.Errorf("max threshold must be greater than min threshold")
	}
	
	return nil
}

// GetStats 获取阈值管理器统计信息
func (tm *ThresholdManager) GetStats() map[string]interface{} {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	
	stats := map[string]interface{}{
		"eth_threshold":    tm.ethThreshold,
		"usd_threshold":    tm.usdThreshold,
		"dynamic_enabled":  tm.dynamicConfig.Enabled,
	}
	
	if tm.dynamicConfig.Enabled {
		stats["dynamic_config"] = map[string]interface{}{
			"base_multiplier":  tm.dynamicConfig.BaseMultiplier,
			"volume_threshold": tm.dynamicConfig.VolumeThreshold,
			"time_window":      tm.dynamicConfig.TimeWindow,
			"min_threshold":    tm.dynamicConfig.MinThreshold,
			"max_threshold":    tm.dynamicConfig.MaxThreshold,
		}
		
		stats["current_eth_threshold"] = tm.calculateDynamicThreshold(tm.ethThreshold, ThresholdTypeETH)
		stats["current_usd_threshold"] = tm.calculateDynamicThreshold(tm.usdThreshold, ThresholdTypeUSD)
	}
	
	return stats
}
