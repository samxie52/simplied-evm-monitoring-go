package models

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
)

// AlertRule 告警规则模型
type AlertRule struct {
	BaseModel

	// 规则基本信息
	//规则名称
	Name string `json:"name" gorm:"size:255;not null" validate:"required,min=1,max=255"`
	// 规则描述
	Description string `json:"description" gorm:"type:text"`
	// 规则类型
	Type AlertType `json:"type" gorm:"type:varchar(50);index;not null" validate:"required"`
	// 严重级别
	Severity AlertSeverity `json:"severity" gorm:"type:varchar(20);index;not null" validate:"required"`
	// 规则状态
	Status AlertStatus `json:"status" gorm:"type:varchar(20);index;not null;default:'active'" validate:"required"`

	// 规则条件 (JSON 格式存储)
	Conditions string `json:"conditions" gorm:"type:text;not null" validate:"required"`

	// 触发配置
	//阈值
	Threshold float64 `json:"threshold" validate:"min=0"`
	//比较操作符
	Operator ComparisonOperator `json:"operator" gorm:"size:10" validate:"required"`

	// 时间配置
	//时间窗口
	TimeWindow int32 `json:"time_window" validate:"min=1"` // 时间窗口（秒）
	//冷却时间
	Cooldown int32 `json:"cooldown" validate:"min=0"` // 冷却时间（秒）

	// 通知配置
	//通知渠道
	NotificationChannels string `json:"notification_channels" gorm:"type:text"` // JSON 数组
	//通知模板
	NotificationTemplate string `json:"notification_template" gorm:"type:text"`

	// 用户关联
	//用户ID
	UserID uint64 `json:"user_id" gorm:"index;not null" validate:"required"`

	// 统计信息
	//触发次数
	TriggerCount uint64 `json:"trigger_count" validate:"min=0"`
	//最后触发时间
	LastTriggered *time.Time `json:"last_triggered"`
	//最后检查时间
	LastChecked *time.Time `json:"last_checked"`

	// 关联关系
	//用户
	User User `json:"user,omitempty"`
	//告警记录
	Alerts []Alert `json:"alerts,omitempty"`
}

// Alert 告警记录模型
type Alert struct {
	BaseModel

	// 告警信息
	//规则ID
	RuleID uint64 `json:"rule_id" gorm:"index;not null" validate:"required"`
	//告警类型
	Type AlertType `json:"type" gorm:"type:varchar(50);index;not null"`
	//严重级别
	Severity AlertSeverity `json:"severity" gorm:"type:varchar(20);index;not null"`
	//标题
	Title string `json:"title" gorm:"size:255;not null" validate:"required"`
	//消息
	Message string `json:"message" gorm:"type:text;not null" validate:"required"`

	// 触发数据
	//触发值
	TriggerValue float64 `json:"trigger_value"`
	//触发数据
	TriggerData string `json:"trigger_data" gorm:"type:text"` // JSON 格式
	//触发时间
	TriggerTime time.Time `json:"trigger_time" gorm:"index"`

	// 处理状态
	//状态
	Status NotificationStatus `json:"status" gorm:"type:varchar(20);index;default:'pending'"` // pending, sent, failed
	//通知发送
	NotificationSent bool `json:"notification_sent" gorm:"default:false"`
	//发送时间
	SentAt *time.Time `json:"sent_at"`

	// 错误信息
	//错误信息
	ErrorMessage string `json:"error_message" gorm:"type:text"`
	//重试次数
	RetryCount int32 `json:"retry_count" validate:"min=0"`

	// 关联关系
	Rule AlertRule `json:"rule,omitempty"`
}

// AlertCondition 告警条件结构
type AlertCondition struct {
	//字段
	Field string `json:"field" validate:"required"`
	//操作符
	Operator ComparisonOperator `json:"operator" validate:"required"`
	//值
	Value interface{} `json:"value" validate:"required"`
	//逻辑操作符
	LogicalOp LogicalOperator `json:"logical_op,omitempty"`
}

// NotificationConfig 通知配置结构
type NotificationConfig struct {
	//通知渠道
	Channel NotificationChannel `json:"channel" validate:"required"`
	//目标地址/ID
	Target string `json:"target" validate:"required"` // 目标地址/ID
	//是否启用
	Enabled bool `json:"enabled"`
	//配置
	Config map[string]interface{} `json:"config,omitempty"` // 额外配置
}

// AlertTriggerData 告警触发数据结构
type AlertTriggerData struct {
	//源类型
	SourceType string `json:"source_type"` // block, transaction, gas_price, etc.
	//源ID
	SourceID string `json:"source_id"` // 源数据ID
	//匹配值
	MatchedValue interface{} `json:"matched_value"`
	//上下文
	Context map[string]interface{} `json:"context,omitempty"`
	//时间戳
	Timestamp time.Time `json:"timestamp"`
}

// TableName 指定表名
func (AlertRule) TableName() string {
	return "alert_rules"
}

func (Alert) TableName() string {
	return "alerts"
}

// AlertRule 相关方法

// BeforeSave 保存前的钩子函数
func (ar *AlertRule) BeforeSave() error {
	ar.UpdatedAt = time.Now()
	return nil
}

// Validate 验证告警规则
func (ar *AlertRule) Validate() error {
	validate := validator.New()
	if err := validate.Struct(ar); err != nil {
		return err
	}

	// 验证告警类型
	if !ar.Type.IsValid() {
		return errors.New("invalid alert type")
	}

	// 验证严重级别
	if !ar.Severity.IsValid() {
		return errors.New("invalid alert severity")
	}

	// 验证状态
	if !ar.Status.IsValid() {
		return errors.New("invalid alert status")
	}

	// 验证操作符
	if !ar.Operator.IsValid() {
		return errors.New("invalid comparison operator")
	}

	// 验证条件 JSON 格式
	var conditions []AlertCondition
	if err := json.Unmarshal([]byte(ar.Conditions), &conditions); err != nil {
		return errors.New("invalid conditions format")
	}

	// 验证每个条件
	for _, condition := range conditions {
		if err := validate.Struct(condition); err != nil {
			return errors.New("invalid condition: " + err.Error())
		}
		if !condition.Operator.IsValid() {
			return errors.New("invalid condition operator")
		}
		if condition.LogicalOp != "" && !condition.LogicalOp.IsValid() {
			return errors.New("invalid logical operator")
		}
	}

	return nil
}

// GetConditions 获取解析后的条件
func (ar *AlertRule) GetConditions() ([]AlertCondition, error) {
	var conditions []AlertCondition
	err := json.Unmarshal([]byte(ar.Conditions), &conditions)
	return conditions, err
}

// SetConditions 设置条件
func (ar *AlertRule) SetConditions(conditions []AlertCondition) error {
	data, err := json.Marshal(conditions)
	if err != nil {
		return err
	}
	ar.Conditions = string(data)
	return nil
}

// GetNotificationChannels 获取通知渠道配置
func (ar *AlertRule) GetNotificationChannels() ([]NotificationConfig, error) {
	var channels []NotificationConfig
	if ar.NotificationChannels == "" {
		return channels, nil
	}
	err := json.Unmarshal([]byte(ar.NotificationChannels), &channels)
	return channels, err
}

// SetNotificationChannels 设置通知渠道
func (ar *AlertRule) SetNotificationChannels(channels []NotificationConfig) error {
	data, err := json.Marshal(channels)
	if err != nil {
		return err
	}
	ar.NotificationChannels = string(data)
	return nil
}

// IsActive 检查规则是否激活
func (ar *AlertRule) IsActive() bool {
	return ar.Status == AlertStatusActive
}

// CanTrigger 检查是否可以触发（考虑冷却时间）
func (ar *AlertRule) CanTrigger() bool {
	if !ar.IsActive() {
		return false
	}

	if ar.LastTriggered == nil {
		return true
	}

	cooldownDuration := time.Duration(ar.Cooldown) * time.Second
	return time.Since(*ar.LastTriggered) >= cooldownDuration
}

// IncrementTriggerCount 增加触发次数
func (ar *AlertRule) IncrementTriggerCount() {
	ar.TriggerCount++
	now := time.Now()
	ar.LastTriggered = &now
}

// UpdateLastChecked 更新最后检查时间
func (ar *AlertRule) UpdateLastChecked() {
	now := time.Now()
	ar.LastChecked = &now
}

// GetTemplate 获取通知模板
func (ar *AlertRule) GetTemplate() string {
	if ar.NotificationTemplate != "" {
		return ar.NotificationTemplate
	}

	// 使用默认模板
	if template, exists := AlertTemplates[ar.Type]; exists {
		return template
	}

	return "告警触发: {{.Title}} - {{.Message}}"
}

// EvaluateCondition 评估单个条件
func (ar *AlertRule) EvaluateCondition(condition AlertCondition, value interface{}) (bool, error) {
	switch condition.Operator {
	case OpGreaterThan:
		return compareValues(value, condition.Value, ">")
	case OpGreaterThanEqual:
		return compareValues(value, condition.Value, ">=")
	case OpLessThan:
		return compareValues(value, condition.Value, "<")
	case OpLessThanEqual:
		return compareValues(value, condition.Value, "<=")
	case OpEqual:
		return compareValues(value, condition.Value, "==")
	case OpNotEqual:
		return compareValues(value, condition.Value, "!=")
	case OpContains:
		return containsValue(value, condition.Value)
	case OpNotContains:
		contains, err := containsValue(value, condition.Value)
		return !contains, err
	default:
		return false, errors.New("unsupported operator")
	}
}

// ToJSON 序列化为 JSON
func (ar *AlertRule) ToJSON() ([]byte, error) {
	return json.Marshal(ar)
}

// FromJSON 从 JSON 反序列化
func (ar *AlertRule) FromJSON(data []byte) error {
	return json.Unmarshal(data, ar)
}

// Alert 相关方法

// Validate 验证告警记录
func (a *Alert) Validate() error {
	validate := validator.New()
	if err := validate.Struct(a); err != nil {
		return err
	}

	// 验证告警类型
	if !a.Type.IsValid() {
		return errors.New("invalid alert type")
	}

	// 验证严重级别
	if !a.Severity.IsValid() {
		return errors.New("invalid alert severity")
	}

	// 验证状态
	if !a.Status.IsValid() {
		return errors.New("invalid notification status")
	}

	return nil
}

// GetTriggerData 获取解析后的触发数据
func (a *Alert) GetTriggerData() (*AlertTriggerData, error) {
	if a.TriggerData == "" {
		return nil, nil
	}

	var data AlertTriggerData
	err := json.Unmarshal([]byte(a.TriggerData), &data)
	return &data, err
}

// SetTriggerData 设置触发数据
func (a *Alert) SetTriggerData(data *AlertTriggerData) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}
	a.TriggerData = string(jsonData)
	return nil
}

// MarkAsSent 标记为已发送
func (a *Alert) MarkAsSent() {
	a.Status = NotificationStatusSent
	a.NotificationSent = true
	now := time.Now()
	a.SentAt = &now
}

// MarkAsFailed 标记为发送失败
func (a *Alert) MarkAsFailed(errorMsg string) {
	a.Status = NotificationStatusFailed
	a.ErrorMessage = errorMsg
	a.RetryCount++
}

// CanRetry 检查是否可以重试
func (a *Alert) CanRetry() bool {
	return a.Status == NotificationStatusFailed && a.RetryCount < 3
}

// IsResolved 检查告警是否已解决
func (a *Alert) IsResolved() bool {
	return a.Status == NotificationStatusSent
}

// GetAge 获取告警年龄（从触发到现在的时间）
func (a *Alert) GetAge() time.Duration {
	return time.Since(a.TriggerTime)
}

// ToJSON 序列化为 JSON
func (a *Alert) ToJSON() ([]byte, error) {
	return json.Marshal(a)
}

// FromJSON 从 JSON 反序列化
func (a *Alert) FromJSON(data []byte) error {
	return json.Unmarshal(data, a)
}

// 查询参数结构

// AlertRuleQueryParams 告警规则查询参数
type AlertRuleQueryParams struct {
	PaginationParams
	FilterParams

	// 告警规则特定过滤条件
	Name      string        `json:"name"`
	AlertType AlertType     `json:"alert_type"`
	Severity  AlertSeverity `json:"severity"`
	Status    AlertStatus   `json:"status"`
	UserID    *uint64       `json:"user_id"`
}

// AlertQueryParams 告警记录查询参数
type AlertQueryParams struct {
	PaginationParams
	FilterParams

	// 告警记录特定过滤条件
	RuleID           *uint64            `json:"rule_id"`
	AlertType        AlertType          `json:"alert_type"`
	Severity         AlertSeverity      `json:"severity"`
	Status           NotificationStatus `json:"status"`
	NotificationSent *bool              `json:"notification_sent"`
	MinTriggerValue  *float64           `json:"min_trigger_value"`
	MaxTriggerValue  *float64           `json:"max_trigger_value"`
}

// 请求结构

// CreateAlertRuleRequest 创建告警规则请求
type CreateAlertRuleRequest struct {
	Name                 string               `json:"name" validate:"required,min=1,max=255"`
	Description          string               `json:"description"`
	Type                 AlertType            `json:"type" validate:"required"`
	Severity             AlertSeverity        `json:"severity" validate:"required"`
	Conditions           []AlertCondition     `json:"conditions" validate:"required,min=1"`
	Threshold            float64              `json:"threshold" validate:"min=0"`
	Operator             ComparisonOperator   `json:"operator" validate:"required"`
	TimeWindow           int32                `json:"time_window" validate:"min=1"`
	Cooldown             int32                `json:"cooldown" validate:"min=0"`
	NotificationChannels []NotificationConfig `json:"notification_channels"`
	NotificationTemplate string               `json:"notification_template"`
}

// ToAlertRule 转换为告警规则模型
func (r *CreateAlertRuleRequest) ToAlertRule(userID uint64) (*AlertRule, error) {
	// 序列化条件
	conditionsJSON, err := json.Marshal(r.Conditions)
	if err != nil {
		return nil, err
	}

	// 序列化通知渠道
	channelsJSON, err := json.Marshal(r.NotificationChannels)
	if err != nil {
		return nil, err
	}

	return &AlertRule{
		Name:                 r.Name,
		Description:          r.Description,
		Type:                 r.Type,
		Severity:             r.Severity,
		Status:               AlertStatusActive,
		Conditions:           string(conditionsJSON),
		Threshold:            r.Threshold,
		Operator:             r.Operator,
		TimeWindow:           r.TimeWindow,
		Cooldown:             r.Cooldown,
		NotificationChannels: string(channelsJSON),
		NotificationTemplate: r.NotificationTemplate,
		UserID:               userID,
	}, nil
}

// Validate 验证创建请求
func (r *CreateAlertRuleRequest) Validate() error {
	validate := validator.New()
	if err := validate.Struct(r); err != nil {
		return err
	}

	// 验证条件
	for _, condition := range r.Conditions {
		if err := validate.Struct(condition); err != nil {
			return err
		}
	}

	// 验证通知渠道
	for _, channel := range r.NotificationChannels {
		if err := validate.Struct(channel); err != nil {
			return err
		}
	}

	return nil
}

// UpdateAlertRuleRequest 更新告警规则请求
type UpdateAlertRuleRequest struct {
	Name                 *string               `json:"name" validate:"omitempty,min=1,max=255"`
	Description          *string               `json:"description"`
	Severity             *AlertSeverity        `json:"severity"`
	Status               *AlertStatus          `json:"status"`
	Conditions           *[]AlertCondition     `json:"conditions"`
	Threshold            *float64              `json:"threshold" validate:"omitempty,min=0"`
	Operator             *ComparisonOperator   `json:"operator"`
	TimeWindow           *int32                `json:"time_window" validate:"omitempty,min=1"`
	Cooldown             *int32                `json:"cooldown" validate:"omitempty,min=0"`
	NotificationChannels *[]NotificationConfig `json:"notification_channels"`
	NotificationTemplate *string               `json:"notification_template"`
}

// ApplyToAlertRule 应用更新到告警规则模型
func (r *UpdateAlertRuleRequest) ApplyToAlertRule(rule *AlertRule) error {
	if r.Name != nil {
		rule.Name = *r.Name
	}
	if r.Description != nil {
		rule.Description = *r.Description
	}
	if r.Severity != nil {
		rule.Severity = *r.Severity
	}
	if r.Status != nil {
		rule.Status = *r.Status
	}
	if r.Conditions != nil {
		if err := rule.SetConditions(*r.Conditions); err != nil {
			return err
		}
	}
	if r.Threshold != nil {
		rule.Threshold = *r.Threshold
	}
	if r.Operator != nil {
		rule.Operator = *r.Operator
	}
	if r.TimeWindow != nil {
		rule.TimeWindow = *r.TimeWindow
	}
	if r.Cooldown != nil {
		rule.Cooldown = *r.Cooldown
	}
	if r.NotificationChannels != nil {
		if err := rule.SetNotificationChannels(*r.NotificationChannels); err != nil {
			return err
		}
	}
	if r.NotificationTemplate != nil {
		rule.NotificationTemplate = *r.NotificationTemplate
	}

	return nil
}

// 工具函数

// compareValues 比较两个值
func compareValues(a, b interface{}, operator string) (bool, error) {
	// 这里简化处理，实际应该根据类型进行更精确的比较
	switch operator {
	case ">":
		return compareFloat64(a, b, func(x, y float64) bool { return x > y })
	case ">=":
		return compareFloat64(a, b, func(x, y float64) bool { return x >= y })
	case "<":
		return compareFloat64(a, b, func(x, y float64) bool { return x < y })
	case "<=":
		return compareFloat64(a, b, func(x, y float64) bool { return x <= y })
	case "==":
		return a == b, nil
	case "!=":
		return a != b, nil
	default:
		return false, errors.New("unsupported comparison operator")
	}
}

// compareFloat64 比较浮点数
func compareFloat64(a, b interface{}, cmp func(float64, float64) bool) (bool, error) {
	var aFloat, bFloat float64
	var ok bool

	if aFloat, ok = a.(float64); !ok {
		return false, errors.New("value a is not float64")
	}
	if bFloat, ok = b.(float64); !ok {
		return false, errors.New("value b is not float64")
	}

	return cmp(aFloat, bFloat), nil
}

// containsValue 检查是否包含值
func containsValue(haystack, needle interface{}) (bool, error) {
	haystackStr, ok1 := haystack.(string)
	needleStr, ok2 := needle.(string)

	if ok1 && ok2 {
		// 简单的字符串包含检查
		return strings.Contains(haystackStr, needleStr), nil
	}

	return false, errors.New("unsupported types for contains operation")
}
