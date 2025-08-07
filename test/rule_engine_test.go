package alert

import (
	"context"
	"simplied-evm-monitoring-go/internal/models"
	"simplied-evm-monitoring-go/internal/services/alert"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRuleEngine_Basic(t *testing.T) {
	// 创建规则引擎配置
	config := &alert.RuleEngineConfig{
		EvaluationInterval:          5 * time.Second,
		MaxConcurrentRules:          5,
		RuleCacheSize:               100,
		EnablePerformanceMonitoring: true,
		EnablePriorityProcessing:    true,
		CooldownCheckInterval:       2 * time.Second,
	}

	var receivedAlerts []*models.Alert
	alertCallback := func(alert *models.Alert) error {
		receivedAlerts = append(receivedAlerts, alert)
		return nil
	}

	// 创建规则引擎
	engine := alert.NewRuleEngine(config, alertCallback)
	require.NotNil(t, engine)

	// 启动引擎
	err := engine.Start()
	require.NoError(t, err)
	assert.True(t, engine.IsRunning())

	// 停止引擎
	err = engine.Stop()
	require.NoError(t, err)
	assert.False(t, engine.IsRunning())
}

func TestRuleEngine_RuleManagement(t *testing.T) {
	config := &alert.RuleEngineConfig{
		EvaluationInterval: 10 * time.Second,
		MaxConcurrentRules: 5,
	}

	engine := alert.NewRuleEngine(config, nil)
	require.NotNil(t, engine)

	// 创建测试规则
	rule := createTestRule("test_rule", models.AlertTypeLargeTransfer, 100.0)

	// 添加规则
	err := engine.AddRule(rule)
	require.NoError(t, err)

	// 获取规则
	retrievedRule, exists := engine.GetRule(rule.ID)
	assert.True(t, exists)
	assert.Equal(t, rule.Name, retrievedRule.Name)

	// 获取所有规则
	allRules := engine.GetAllRules()
	assert.Len(t, allRules, 1)

	// 获取激活规则
	activeRules := engine.GetActiveRules()
	assert.Len(t, activeRules, 1)

	// 更新规则
	rule.Threshold = 200.0
	err = engine.UpdateRule(rule)
	require.NoError(t, err)

	retrievedRule, exists = engine.GetRule(rule.ID)
	assert.True(t, exists)
	assert.Equal(t, 200.0, retrievedRule.Threshold)

	// 移除规则
	err = engine.RemoveRule(rule.ID)
	require.NoError(t, err)

	_, exists = engine.GetRule(rule.ID)
	assert.False(t, exists)
}

func TestRuleEngine_RuleEvaluation(t *testing.T) {
	config := &alert.RuleEngineConfig{
		EvaluationInterval: 10 * time.Second,
		MaxConcurrentRules: 5,
	}

	var receivedAlerts []*models.Alert
	alertCallback := func(alert *models.Alert) error {
		receivedAlerts = append(receivedAlerts, alert)
		return nil
	}

	engine := alert.NewRuleEngine(config, alertCallback)
	require.NotNil(t, engine)

	err := engine.Start()
	require.NoError(t, err)
	defer engine.Stop()

	// 创建大额转账规则
	rule := createTestRule("large_transfer_rule", models.AlertTypeLargeTransfer, 50.0)
	err = engine.AddRule(rule)
	require.NoError(t, err)

	// 创建测试数据 - 触发规则
	data := map[string]interface{}{
		"transaction_value": 100.0,
		"transaction_hash":  "0x123456789",
		"from_address":      "0xabc",
		"to_address":        "0xdef",
	}

	ctx := context.Background()
	alert, err := engine.EvaluateRule(ctx, rule, data)
	require.NoError(t, err)
	require.NotNil(t, alert)

	assert.Equal(t, rule.ID, alert.RuleID)
	assert.Equal(t, rule.Type, alert.Type)
	assert.Equal(t, rule.Severity, alert.Severity)
	assert.Equal(t, 100.0, alert.TriggerValue)

	// 创建测试数据 - 不触发规则
	smallData := map[string]interface{}{
		"transaction_value": 10.0,
		"transaction_hash":  "0x987654321",
	}

	alert, err = engine.EvaluateRule(ctx, rule, smallData)
	require.NoError(t, err)
	assert.Nil(t, alert)
}

func TestRuleEngine_CooldownPeriod(t *testing.T) {
	config := &alert.RuleEngineConfig{
		EvaluationInterval:    1 * time.Second,
		MaxConcurrentRules:    5,
		CooldownCheckInterval: 1 * time.Second,
	}

	var receivedAlerts []*models.Alert
	alertCallback := func(alert *models.Alert) error {
		receivedAlerts = append(receivedAlerts, alert)
		return nil
	}

	engine := alert.NewRuleEngine(config, alertCallback)
	require.NotNil(t, engine)

	err := engine.Start()
	require.NoError(t, err)
	defer engine.Stop()

	// 创建有冷却时间的规则
	rule := createTestRule("cooldown_rule", models.AlertTypeLargeTransfer, 50.0)
	rule.Cooldown = 3 // 3秒冷却时间
	err = engine.AddRule(rule)
	require.NoError(t, err)

	data := map[string]interface{}{
		"transaction_value": 100.0,
		"transaction_hash":  "0x123456789",
	}

	ctx := context.Background()

	// 第一次评估应该触发
	alert1, err := engine.EvaluateRule(ctx, rule, data)
	require.NoError(t, err)
	require.NotNil(t, alert1)

	// 立即再次评估应该被冷却时间阻止
	alert2, err := engine.EvaluateRule(ctx, rule, data)
	require.NoError(t, err)
	assert.Nil(t, alert2)

	// 等待冷却时间过期
	time.Sleep(4 * time.Second)

	// 现在应该可以再次触发
	alert3, err := engine.EvaluateRule(ctx, rule, data)
	require.NoError(t, err)
	require.NotNil(t, alert3)
}

func TestRuleEngine_MultipleConditions(t *testing.T) {
	config := &alert.RuleEngineConfig{
		EvaluationInterval: 10 * time.Second,
		MaxConcurrentRules: 5,
	}

	engine := alert.NewRuleEngine(config, nil)
	require.NotNil(t, engine)

	err := engine.Start()
	require.NoError(t, err)
	defer engine.Stop()

	// 创建多条件规则
	rule := &models.AlertRule{
		Name:        "multi_condition_rule",
		Type:        models.AlertTypeCustom,
		Severity:    models.SeverityMedium,
		Status:      models.AlertStatusActive,
		Threshold:   100.0,
		Operator:    models.OpGreaterThan,
		TimeWindow:  60,
		Cooldown:    0,
		UserID:      1,
		// 创建测试用户对象
		User: models.User{
			Username: "testuser",
			Email:    "test@example.com",
			Password: "password123",
			Role:     models.RoleUser,
			Status:   models.UserStatusActive,
			Timezone: "UTC",
			Language: "en",
		},
	}
	rule.ID = 1
	rule.User.ID = 1

	// 设置多个条件：value > 100 AND gas_price < 50
	conditions := []models.AlertCondition{
		{
			Field:     "value",
			Operator:  models.OpGreaterThan,
			Value:     100.0,
			LogicalOp: models.LogicalAnd,
		},
		{
			Field:    "gas_price",
			Operator: models.OpLessThan,
			Value:    50.0,
		},
	}

	err = rule.SetConditions(conditions)
	require.NoError(t, err)

	err = engine.AddRule(rule)
	require.NoError(t, err)

	ctx := context.Background()

	// 测试数据1：满足两个条件
	data1 := map[string]interface{}{
		"value":     150.0,
		"gas_price": 30.0,
	}

	alert1, err := engine.EvaluateRule(ctx, rule, data1)
	require.NoError(t, err)
	require.NotNil(t, alert1)

	// 测试数据2：只满足第一个条件
	data2 := map[string]interface{}{
		"value":     150.0,
		"gas_price": 60.0,
	}

	alert2, err := engine.EvaluateRule(ctx, rule, data2)
	require.NoError(t, err)
	assert.Nil(t, alert2)

	// 测试数据3：只满足第二个条件
	data3 := map[string]interface{}{
		"value":     50.0,
		"gas_price": 30.0,
	}

	alert3, err := engine.EvaluateRule(ctx, rule, data3)
	require.NoError(t, err)
	assert.Nil(t, alert3)
}

func TestRuleEngine_Statistics(t *testing.T) {
	config := &alert.RuleEngineConfig{
		EvaluationInterval:          5 * time.Second,
		MaxConcurrentRules:          5,
		EnablePerformanceMonitoring: true,
	}

	engine := alert.NewRuleEngine(config, nil)
	require.NotNil(t, engine)

	// 获取初始统计信息
	stats := engine.GetStats()
	assert.NotNil(t, stats)
	assert.Equal(t, false, stats["is_running"])
	assert.Equal(t, int64(0), stats["total_rules"])

	// 启动引擎
	err := engine.Start()
	require.NoError(t, err)
	defer engine.Stop()

	stats = engine.GetStats()
	assert.Equal(t, true, stats["is_running"])

	// 添加规则
	rule := createTestRule("stats_rule", models.AlertTypeLargeTransfer, 100.0)
	err = engine.AddRule(rule)
	require.NoError(t, err)

	stats = engine.GetStats()
	assert.Equal(t, int64(1), stats["total_rules"])
	assert.Equal(t, int64(1), stats["active_rules"])

	// 评估规则
	data := map[string]interface{}{
		"transaction_value": 150.0,
	}

	ctx := context.Background()
	alert, err := engine.EvaluateRule(ctx, rule, data)
	require.NoError(t, err)
	require.NotNil(t, alert)

	stats = engine.GetStats()
	assert.Equal(t, int64(1), stats["triggered_rules"])
	assert.Equal(t, int64(1), stats["generated_alerts"])
}

func TestRuleEngine_ConfigUpdate(t *testing.T) {
	config := &alert.RuleEngineConfig{
		EvaluationInterval: 10 * time.Second,
		MaxConcurrentRules: 5,
	}

	engine := alert.NewRuleEngine(config, nil)
	require.NotNil(t, engine)

	// 验证初始配置
	currentConfig := engine.GetConfig()
	assert.Equal(t, 10*time.Second, currentConfig.EvaluationInterval)
	assert.Equal(t, 5, currentConfig.MaxConcurrentRules)

	// 更新配置
	newConfig := &alert.RuleEngineConfig{
		EvaluationInterval: 5 * time.Second,
		MaxConcurrentRules: 10,
	}

	err := engine.UpdateConfig(newConfig)
	require.NoError(t, err)

	// 验证配置已更新
	updatedConfig := engine.GetConfig()
	assert.Equal(t, 5*time.Second, updatedConfig.EvaluationInterval)
	assert.Equal(t, 10, updatedConfig.MaxConcurrentRules)
}

func TestRuleEvaluator_TypeSpecificEvaluators(t *testing.T) {
	evaluator := alert.NewRuleEvaluator()
	require.NotNil(t, evaluator)

	ctx := context.Background()

	// 测试 Gas 价格评估器
	gasPriceRule := &models.AlertRule{
		Name:       "gas_price_rule",
		Type:       models.AlertTypeGasPrice,
		Severity:   models.SeverityHigh,
		Status:     models.AlertStatusActive,
		Threshold:  100.0,
		Operator:   models.OpGreaterThan,
		TimeWindow: 60,
		UserID:     1,
		// 创建测试用户对象
		User: models.User{
			Username: "testuser",
			Email:    "test@example.com",
			Password: "password123",
			Role:     models.RoleUser,
			Status:   models.UserStatusActive,
			Timezone: "UTC",
			Language: "en",
		},
	}
	gasPriceRule.ID = 1
	gasPriceRule.User.ID = 1
	// 设置 Gas 价格条件
	gasPriceConditions := []models.AlertCondition{
		{
			Field:    "gas_price",
			Operator: models.OpGreaterThan,
			Value:    100.0,
		},
	}
	gasPriceRule.SetConditions(gasPriceConditions)

	gasPriceData := map[string]interface{}{
		"gas_price": 150.0,
	}

	triggered, triggerData, err := evaluator.EvaluateRule(ctx, gasPriceRule, gasPriceData)
	require.NoError(t, err)
	assert.True(t, triggered)
	assert.NotNil(t, triggerData)
	assert.Equal(t, "gas_price", triggerData.SourceType)

	// 测试网络拥堵评估器
	congestionRule := &models.AlertRule{
		Name:       "congestion_rule",
		Type:       models.AlertTypeNetworkCongestion,
		Severity:   models.SeverityMedium,
		Status:     models.AlertStatusActive,
		Threshold:  90.0,
		Operator:   models.OpGreaterThanEqual,
		TimeWindow: 60,
		UserID:     1,
		// 创建测试用户对象
		User: models.User{
			Username: "testuser2",
			Email:    "test2@example.com",
			Password: "password123",
			Role:     models.RoleUser,
			Status:   models.UserStatusActive,
			Timezone: "UTC",
			Language: "en",
		},
	}
	congestionRule.ID = 2
	congestionRule.User.ID = 2
	// 设置网络拥堵条件
	congestionConditions := []models.AlertCondition{
		{
			Field:    "gas_utilization",
			Operator: models.OpGreaterThanEqual,
			Value:    90.0,
		},
	}
	congestionRule.SetConditions(congestionConditions)

	congestionData := map[string]interface{}{
		"gas_utilization": 95.0,
		"block_number":    "12345",
	}

	triggered, triggerData, err = evaluator.EvaluateRule(ctx, congestionRule, congestionData)
	require.NoError(t, err)
	assert.True(t, triggered)
	assert.NotNil(t, triggerData)
	assert.Equal(t, "network", triggerData.SourceType)
	assert.Equal(t, 95.0, triggerData.MatchedValue)
}

// BenchmarkRuleEvaluation 规则评估性能基准测试
func BenchmarkRuleEvaluation(b *testing.B) {
	// 创建规则评估器
	evaluator := alert.NewRuleEvaluator()
	
	// 创建测试规则
	rule := createTestRule("benchmark_rule", models.AlertTypeLargeTransfer, 1000.0)
	
	// 创建测试数据
	data := map[string]interface{}{
		"value":        1500.0,
		"gas_price":    50.0,
		"block_number": "12345",
		"timestamp":    time.Now().Unix(),
	}
	
	b.ResetTimer()
	ctx := context.Background()
	for i := 0; i < b.N; i++ {
		_, _, _ = evaluator.EvaluateRule(ctx, rule, data)
	}
}

// 基准测试
func BenchmarkRuleEngine_EvaluateRule(b *testing.B) {
	config := &alert.RuleEngineConfig{
		EvaluationInterval: 10 * time.Second,
		MaxConcurrentRules: 10,
	}

	engine := alert.NewRuleEngine(config, nil)
	engine.Start()
	defer engine.Stop()

	rule := createTestRule("benchmark_rule", models.AlertTypeLargeTransfer, 100.0)
	engine.AddRule(rule)

	data := map[string]interface{}{
		"transaction_value": 150.0,
		"transaction_hash":  "0x123456789",
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := engine.EvaluateRule(ctx, rule, data)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// 辅助函数

// createTestRule 创建测试规则
func createTestRule(name string, alertType models.AlertType, threshold float64) *models.AlertRule {
	rule := &models.AlertRule{
		Name:       name,
		Type:       alertType,
		Severity:   models.SeverityMedium,
		Status:     models.AlertStatusActive,
		Threshold:  threshold,
		Operator:   models.OpGreaterThanEqual,
		TimeWindow: 60, // 设置时间窗口
		Cooldown:   0,
		UserID:     1,
		// 创建测试用户对象
		User: models.User{
			Username: "testuser",
			Email:    "test@example.com",
			Password: "password123",
			Role:     models.RoleUser,
			Status:   models.UserStatusActive,
			Timezone: "UTC",
			Language: "en",
		},
	}
	// 设置 ID 使用时间戳
	rule.ID = uint64(time.Now().UnixNano())
	rule.User.ID = 1

	// 设置默认条件
	conditions := []models.AlertCondition{
		{
			Field:    "value",
			Operator: models.OpGreaterThanEqual,
			Value:    threshold,
		},
	}
	rule.SetConditions(conditions)

	return rule
}

// createTestRuleWithConditions 创建带条件的测试规则
func createTestRuleWithConditions(name string, alertType models.AlertType, conditions []models.AlertCondition) *models.AlertRule {
	rule := createTestRule(name, alertType, 0)
	rule.SetConditions(conditions)
	return rule
}
