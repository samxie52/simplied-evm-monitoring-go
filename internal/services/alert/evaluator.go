package alert

import (
	"context"
	"encoding/json"
	"fmt"
	"simplied-evm-monitoring-go/internal/models"
	"simplied-evm-monitoring-go/pkg/logger"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

// RuleEvaluator 规则评估器
type RuleEvaluator struct {
	// 评估函数映射
	evaluators map[models.AlertType]AlertTypeEvaluator
}

// AlertTypeEvaluator 告警类型评估器接口
type AlertTypeEvaluator interface {
	Evaluate(ctx context.Context, rule *models.AlertRule, data map[string]interface{}) (bool, *models.AlertTriggerData, error)
}

// EvaluationContext 评估上下文
type EvaluationContext struct {
	Rule      *models.AlertRule
	Data      map[string]interface{}
	Timestamp time.Time
}

// NewRuleEvaluator 创建新的规则评估器
func NewRuleEvaluator() *RuleEvaluator {
	evaluator := &RuleEvaluator{
		evaluators: make(map[models.AlertType]AlertTypeEvaluator),
	}

	// 注册默认评估器
	evaluator.registerDefaultEvaluators()

	return evaluator
}

// registerDefaultEvaluators 注册默认评估器
func (re *RuleEvaluator) registerDefaultEvaluators() {
	// 注册各种告警类型的评估器
	re.evaluators[models.AlertTypeLargeTransfer] = &LargeTransferEvaluator{}
	re.evaluators[models.AlertTypeGasPrice] = &GasPriceEvaluator{}
	re.evaluators[models.AlertTypeBlockTime] = &BlockTimeEvaluator{}
	re.evaluators[models.AlertTypeNetworkCongestion] = &NetworkCongestionEvaluator{}
	re.evaluators[models.AlertTypeContractEvent] = &ContractEventEvaluator{}
	re.evaluators[models.AlertTypeAddressActivity] = &AddressActivityEvaluator{}
	re.evaluators[models.AlertTypeTokenTransfer] = &TokenTransferEvaluator{}
	re.evaluators[models.AlertTypeSystemHealth] = &SystemHealthEvaluator{}
	re.evaluators[models.AlertTypeCustom] = &CustomEvaluator{}
}

// RegisterEvaluator 注册自定义评估器
func (re *RuleEvaluator) RegisterEvaluator(alertType models.AlertType, evaluator AlertTypeEvaluator) {
	re.evaluators[alertType] = evaluator
}

// EvaluateRule 评估规则
func (re *RuleEvaluator) EvaluateRule(ctx context.Context, rule *models.AlertRule, data map[string]interface{}) (bool, *models.AlertTriggerData, error) {
	if rule == nil {
		return false, nil, fmt.Errorf("rule cannot be nil")
	}

	// 检查规则状态
	if rule.Status != models.AlertStatusActive {
		return false, nil, nil
	}

	// 获取对应的评估器
	evaluator, exists := re.evaluators[rule.Type]
	if !exists {
		// 使用通用评估器
		evaluator = &GenericEvaluator{}
	}

	logger.WithFields(logrus.Fields{
		"rule_id":   rule.ID,
		"rule_name": rule.Name,
		"rule_type": rule.Type,
	}).Debug("Evaluating rule")

	// 执行评估
	triggered, triggerData, err := evaluator.Evaluate(ctx, rule, data)
	if err != nil {
		return false, nil, fmt.Errorf("evaluation failed: %w", err)
	}

	if triggered && triggerData != nil {
		triggerData.Timestamp = time.Now()
	}

	return triggered, triggerData, nil
}

// GenericEvaluator 通用评估器
type GenericEvaluator struct{}

// Evaluate 通用评估逻辑
func (ge *GenericEvaluator) Evaluate(ctx context.Context, rule *models.AlertRule, data map[string]interface{}) (bool, *models.AlertTriggerData, error) {
	// 获取规则条件
	conditions, err := rule.GetConditions()
	if err != nil {
		return false, nil, fmt.Errorf("failed to get conditions: %w", err)
	}

	if len(conditions) == 0 {
		return false, nil, nil
	}

	// 评估所有条件
	result, triggerData, err := ge.evaluateConditions(conditions, data, rule)
	if err != nil {
		return false, nil, err
	}

	return result, triggerData, nil
}

// evaluateConditions 评估条件组合
func (ge *GenericEvaluator) evaluateConditions(conditions []models.AlertCondition, data map[string]interface{}, rule *models.AlertRule) (bool, *models.AlertTriggerData, error) {
	if len(conditions) == 0 {
		return false, nil, nil
	}

	// 如果只有一个条件，直接评估
	if len(conditions) == 1 {
		return ge.evaluateSingleCondition(conditions[0], data, rule)
	}

	// 多个条件需要根据逻辑操作符组合
	return ge.evaluateMultipleConditions(conditions, data, rule)
}

// evaluateSingleCondition 评估单个条件
func (ge *GenericEvaluator) evaluateSingleCondition(condition models.AlertCondition, data map[string]interface{}, rule *models.AlertRule) (bool, *models.AlertTriggerData, error) {
	// 从数据中获取字段值
	fieldValue, exists := getFieldValue(data, condition.Field)
	if !exists {
		logger.WithFields(logrus.Fields{
			"rule_id": rule.ID,
			"field":   condition.Field,
		}).Debug("Field not found in data")
		return false, nil, nil
	}

	// 评估条件
	result, err := rule.EvaluateCondition(condition, fieldValue)
	if err != nil {
		return false, nil, fmt.Errorf("failed to evaluate condition: %w", err)
	}

	var triggerData *models.AlertTriggerData
	if result {
		triggerData = &models.AlertTriggerData{
			SourceType:   string(rule.Type),
			SourceID:     fmt.Sprintf("rule_%d", rule.ID),
			MatchedValue: fieldValue,
			Context: map[string]interface{}{
				"field":     condition.Field,
				"operator":  condition.Operator,
				"threshold": condition.Value,
				"actual":    fieldValue,
			},
		}
	}

	return result, triggerData, nil
}

// evaluateMultipleConditions 评估多个条件
func (ge *GenericEvaluator) evaluateMultipleConditions(conditions []models.AlertCondition, data map[string]interface{}, rule *models.AlertRule) (bool, *models.AlertTriggerData, error) {
	results := make([]bool, len(conditions))
	var triggerData *models.AlertTriggerData

	// 评估每个条件
	for i, condition := range conditions {
		fieldValue, exists := getFieldValue(data, condition.Field)
		if !exists {
			results[i] = false
			continue
		}

		result, err := rule.EvaluateCondition(condition, fieldValue)
		if err != nil {
			return false, nil, fmt.Errorf("failed to evaluate condition %d: %w", i, err)
		}

		results[i] = result

		// 记录第一个匹配的条件数据
		if result && triggerData == nil {
			triggerData = &models.AlertTriggerData{
				SourceType:   string(rule.Type),
				SourceID:     fmt.Sprintf("rule_%d", rule.ID),
				MatchedValue: fieldValue,
				Context: map[string]interface{}{
					"field":     condition.Field,
					"operator":  condition.Operator,
					"threshold": condition.Value,
					"actual":    fieldValue,
				},
			}
		}
	}

	// 根据逻辑操作符组合结果
	finalResult := ge.combineResults(results, conditions)

	return finalResult, triggerData, nil
}

// combineResults 组合条件结果
func (ge *GenericEvaluator) combineResults(results []bool, conditions []models.AlertCondition) bool {
	if len(results) == 0 {
		return false
	}

	if len(results) == 1 {
		return results[0]
	}

	// 默认使用 AND 逻辑
	finalResult := results[0]

	for i := 1; i < len(results); i++ {
		logicalOp := models.LogicalAnd // 默认 AND
		if i-1 < len(conditions) && conditions[i-1].LogicalOp != "" {
			logicalOp = conditions[i-1].LogicalOp
		}

		switch logicalOp {
		case models.LogicalAnd:
			finalResult = finalResult && results[i]
		case models.LogicalOr:
			finalResult = finalResult || results[i]
		}
	}

	return finalResult
}

// LargeTransferEvaluator 大额转账评估器
type LargeTransferEvaluator struct{}

func (lte *LargeTransferEvaluator) Evaluate(ctx context.Context, rule *models.AlertRule, data map[string]interface{}) (bool, *models.AlertTriggerData, error) {
	// 检查是否有交易数据
	value, exists := data["transaction_value"]
	if !exists {
		return false, nil, nil
	}

	valueFloat, ok := convertToFloat64(value)
	if !ok {
		return false, nil, fmt.Errorf("invalid transaction value type")
	}

	// 检查是否超过阈值
	if valueFloat >= rule.Threshold {
		triggerData := &models.AlertTriggerData{
			SourceType:   "transaction",
			SourceID:     getStringValue(data, "transaction_hash", ""),
			MatchedValue: valueFloat,
			Context: map[string]interface{}{
				"transaction_hash": data["transaction_hash"],
				"from_address":     data["from_address"],
				"to_address":       data["to_address"],
				"value_eth":        valueFloat,
				"threshold":        rule.Threshold,
			},
		}
		return true, triggerData, nil
	}

	return false, nil, nil
}

// GasPriceEvaluator Gas价格评估器
type GasPriceEvaluator struct{}

func (gpe *GasPriceEvaluator) Evaluate(ctx context.Context, rule *models.AlertRule, data map[string]interface{}) (bool, *models.AlertTriggerData, error) {
	gasPrice, exists := data["gas_price"]
	if !exists {
		return false, nil, nil
	}

	gasPriceFloat, ok := convertToFloat64(gasPrice)
	if !ok {
		return false, nil, fmt.Errorf("invalid gas price type")
	}

	// 根据操作符检查
	triggered := false
	switch rule.Operator {
	case models.OpGreaterThan:
		triggered = gasPriceFloat > rule.Threshold
	case models.OpGreaterThanEqual:
		triggered = gasPriceFloat >= rule.Threshold
	case models.OpLessThan:
		triggered = gasPriceFloat < rule.Threshold
	case models.OpLessThanEqual:
		triggered = gasPriceFloat <= rule.Threshold
	}

	if triggered {
		triggerData := &models.AlertTriggerData{
			SourceType:   "gas_price",
			SourceID:     "current",
			MatchedValue: gasPriceFloat,
			Context: map[string]interface{}{
				"gas_price_gwei": gasPriceFloat,
				"threshold":      rule.Threshold,
				"operator":       rule.Operator,
			},
		}
		return true, triggerData, nil
	}

	return false, nil, nil
}

// BlockTimeEvaluator 出块时间评估器
type BlockTimeEvaluator struct{}

func (bte *BlockTimeEvaluator) Evaluate(ctx context.Context, rule *models.AlertRule, data map[string]interface{}) (bool, *models.AlertTriggerData, error) {
	blockTime, exists := data["block_time"]
	if !exists {
		return false, nil, nil
	}

	blockTimeFloat, ok := convertToFloat64(blockTime)
	if !ok {
		return false, nil, fmt.Errorf("invalid block time type")
	}

	if blockTimeFloat > rule.Threshold {
		triggerData := &models.AlertTriggerData{
			SourceType:   "block",
			SourceID:     getStringValue(data, "block_number", ""),
			MatchedValue: blockTimeFloat,
			Context: map[string]interface{}{
				"block_number": data["block_number"],
				"block_time":   blockTimeFloat,
				"threshold":    rule.Threshold,
			},
		}
		return true, triggerData, nil
	}

	return false, nil, nil
}

// NetworkCongestionEvaluator 网络拥堵评估器
type NetworkCongestionEvaluator struct{}

func (nce *NetworkCongestionEvaluator) Evaluate(ctx context.Context, rule *models.AlertRule, data map[string]interface{}) (bool, *models.AlertTriggerData, error) {
	utilization, exists := data["gas_utilization"]
	if !exists {
		return false, nil, nil
	}

	utilizationFloat, ok := convertToFloat64(utilization)
	if !ok {
		return false, nil, fmt.Errorf("invalid gas utilization type")
	}

	if utilizationFloat >= rule.Threshold {
		triggerData := &models.AlertTriggerData{
			SourceType:   "network",
			SourceID:     "congestion",
			MatchedValue: utilizationFloat,
			Context: map[string]interface{}{
				"gas_utilization": utilizationFloat,
				"threshold":       rule.Threshold,
				"block_number":    data["block_number"],
			},
		}
		return true, triggerData, nil
	}

	return false, nil, nil
}

// ContractEventEvaluator 合约事件评估器
type ContractEventEvaluator struct{}

func (cee *ContractEventEvaluator) Evaluate(ctx context.Context, rule *models.AlertRule, data map[string]interface{}) (bool, *models.AlertTriggerData, error) {
	// 检查是否有合约事件
	eventName, exists := data["event_name"]
	if !exists {
		return false, nil, nil
	}

	// 获取条件中的目标事件名称
	conditions, err := rule.GetConditions()
	if err != nil {
		return false, nil, err
	}

	for _, condition := range conditions {
		if condition.Field == "event_name" {
			if condition.Operator == models.OpEqual && eventName == condition.Value {
				triggerData := &models.AlertTriggerData{
					SourceType:   "contract_event",
					SourceID:     getStringValue(data, "contract_address", ""),
					MatchedValue: eventName,
					Context: map[string]interface{}{
						"contract_address": data["contract_address"],
						"event_name":       eventName,
						"event_data":       data["event_data"],
						"transaction_hash": data["transaction_hash"],
					},
				}
				return true, triggerData, nil
			}
		}
	}

	return false, nil, nil
}

// AddressActivityEvaluator 地址活动评估器
type AddressActivityEvaluator struct{}

func (aae *AddressActivityEvaluator) Evaluate(ctx context.Context, rule *models.AlertRule, data map[string]interface{}) (bool, *models.AlertTriggerData, error) {
	address, exists := data["address"]
	if !exists {
		return false, nil, nil
	}

	// 检查地址是否在监控列表中
	conditions, err := rule.GetConditions()
	if err != nil {
		return false, nil, err
	}

	for _, condition := range conditions {
		if condition.Field == "address" && condition.Operator == models.OpEqual {
			if address == condition.Value {
				triggerData := &models.AlertTriggerData{
					SourceType:   "address_activity",
					SourceID:     fmt.Sprintf("%v", address),
					MatchedValue: address,
					Context: map[string]interface{}{
						"address":          address,
						"activity_type":    data["activity_type"],
						"transaction_hash": data["transaction_hash"],
						"value":            data["value"],
					},
				}
				return true, triggerData, nil
			}
		}
	}

	return false, nil, nil
}

// TokenTransferEvaluator 代币转账评估器
type TokenTransferEvaluator struct{}

func (tte *TokenTransferEvaluator) Evaluate(ctx context.Context, rule *models.AlertRule, data map[string]interface{}) (bool, *models.AlertTriggerData, error) {
	tokenAmount, exists := data["token_amount"]
	if !exists {
		return false, nil, nil
	}

	amountFloat, ok := convertToFloat64(tokenAmount)
	if !ok {
		return false, nil, fmt.Errorf("invalid token amount type")
	}

	if amountFloat >= rule.Threshold {
		triggerData := &models.AlertTriggerData{
			SourceType:   "token_transfer",
			SourceID:     getStringValue(data, "transaction_hash", ""),
			MatchedValue: amountFloat,
			Context: map[string]interface{}{
				"token_address":    data["token_address"],
				"token_symbol":     data["token_symbol"],
				"token_amount":     amountFloat,
				"from_address":     data["from_address"],
				"to_address":       data["to_address"],
				"transaction_hash": data["transaction_hash"],
			},
		}
		return true, triggerData, nil
	}

	return false, nil, nil
}

// SystemHealthEvaluator 系统健康评估器
type SystemHealthEvaluator struct{}

func (she *SystemHealthEvaluator) Evaluate(ctx context.Context, rule *models.AlertRule, data map[string]interface{}) (bool, *models.AlertTriggerData, error) {
	healthStatus, exists := data["health_status"]
	if !exists {
		return false, nil, nil
	}

	// 检查健康状态
	if healthStatus != "healthy" {
		triggerData := &models.AlertTriggerData{
			SourceType:   "system_health",
			SourceID:     getStringValue(data, "component", "unknown"),
			MatchedValue: healthStatus,
			Context: map[string]interface{}{
				"component":     data["component"],
				"health_status": healthStatus,
				"error_message": data["error_message"],
				"timestamp":     time.Now(),
			},
		}
		return true, triggerData, nil
	}

	return false, nil, nil
}

// CustomEvaluator 自定义评估器
type CustomEvaluator struct{}

func (ce *CustomEvaluator) Evaluate(ctx context.Context, rule *models.AlertRule, data map[string]interface{}) (bool, *models.AlertTriggerData, error) {
	// 使用通用评估器
	ge := &GenericEvaluator{}
	return ge.Evaluate(ctx, rule, data)
}

// 工具函数

// getFieldValue 从数据中获取字段值，支持嵌套字段
func getFieldValue(data map[string]interface{}, field string) (interface{}, bool) {
	if strings.Contains(field, ".") {
		// 支持嵌套字段，如 "transaction.value"
		parts := strings.Split(field, ".")
		current := data
		
		for i, part := range parts {
			if i == len(parts)-1 {
				// 最后一个字段
				value, exists := current[part]
				return value, exists
			} else {
				// 中间字段
				next, exists := current[part]
				if !exists {
					return nil, false
				}
				
				nextMap, ok := next.(map[string]interface{})
				if !ok {
					return nil, false
				}
				current = nextMap
			}
		}
	}

	value, exists := data[field]
	return value, exists
}

// convertToFloat64 转换为 float64
func convertToFloat64(value interface{}) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int32:
		return float64(v), true
	case int64:
		return float64(v), true
	case uint:
		return float64(v), true
	case uint32:
		return float64(v), true
	case uint64:
		return float64(v), true
	case string:
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f, true
		}
	case json.Number:
		if f, err := v.Float64(); err == nil {
			return f, true
		}
	}
	return 0, false
}

// getStringValue 获取字符串值
func getStringValue(data map[string]interface{}, key, defaultValue string) string {
	if value, exists := data[key]; exists {
		if str, ok := value.(string); ok {
			return str
		}
		return fmt.Sprintf("%v", value)
	}
	return defaultValue
}

// compareValues 比较两个值
func compareValues(actual, expected interface{}, operator string) (bool, error) {
	// 尝试转换为数值比较
	if actualFloat, ok1 := convertToFloat64(actual); ok1 {
		if expectedFloat, ok2 := convertToFloat64(expected); ok2 {
			return compareNumbers(actualFloat, expectedFloat, operator), nil
		}
	}

	// 字符串比较
	actualStr := fmt.Sprintf("%v", actual)
	expectedStr := fmt.Sprintf("%v", expected)

	switch operator {
	case "==":
		return actualStr == expectedStr, nil
	case "!=":
		return actualStr != expectedStr, nil
	case ">", ">=", "<", "<=":
		return false, fmt.Errorf("numeric comparison not supported for non-numeric values")
	default:
		return false, fmt.Errorf("unsupported operator: %s", operator)
	}
}

// compareNumbers 比较数值
func compareNumbers(actual, expected float64, operator string) bool {
	switch operator {
	case ">":
		return actual > expected
	case ">=":
		return actual >= expected
	case "<":
		return actual < expected
	case "<=":
		return actual <= expected
	case "==":
		return actual == expected
	case "!=":
		return actual != expected
	default:
		return false
	}
}

// containsValue 检查是否包含值
func containsValue(actual, expected interface{}) (bool, error) {
	actualStr := fmt.Sprintf("%v", actual)
	expectedStr := fmt.Sprintf("%v", expected)
	
	return strings.Contains(actualStr, expectedStr), nil
}

// GetEvaluatorInfo 获取评估器信息
func (re *RuleEvaluator) GetEvaluatorInfo() map[string]interface{} {
	evaluatorTypes := make([]string, 0, len(re.evaluators))
	for alertType := range re.evaluators {
		evaluatorTypes = append(evaluatorTypes, string(alertType))
	}

	return map[string]interface{}{
		"registered_evaluators": evaluatorTypes,
		"total_evaluators":      len(re.evaluators),
		"supports_custom":       true,
	}
}
