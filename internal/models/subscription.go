package models

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/go-playground/validator/v10"
)

// Subscription 订阅模型
type Subscription struct {
	BaseModel

	// 订阅基本信息
	//订阅名称
	Name string `json:"name" gorm:"size:255;not null" validate:"required,min=1,max=255"`
	//订阅描述
	Description string `json:"description" gorm:"type:text"`
	//订阅类型
	Type SubscriptionType `json:"type" gorm:"type:varchar(50);index;not null" validate:"required"`
	//订阅状态
	Status SubscriptionStatus `json:"status" gorm:"type:varchar(20);index;not null;default:'active'" validate:"required"`

	// 用户关联
	UserID uint64 `json:"user_id" gorm:"index;not null" validate:"required"`

	// 订阅配置 (JSON 格式存储)
	Config string `json:"config" gorm:"type:text;not null" validate:"required"`

	// 过滤条件 (JSON 格式存储)
	Filters string `json:"filters" gorm:"type:text"`

	// 通知配置
	NotificationChannels string `json:"notification_channels" gorm:"type:text"` // JSON 数组
	NotificationTemplate string `json:"notification_template" gorm:"type:text"`

	// 频率控制
	MaxNotificationsPerHour int32     `json:"max_notifications_per_hour" validate:"min=0,max=1000"`
	NotificationCount       int32     `json:"notification_count" validate:"min=0"`
	LastNotificationReset   time.Time `json:"last_notification_reset"`

	// 统计信息
	TotalNotifications uint64     `json:"total_notifications" validate:"min=0"`
	LastTriggered      *time.Time `json:"last_triggered"`
	LastChecked        *time.Time `json:"last_checked"`

	// 有效期
	ExpiresAt *time.Time `json:"expires_at"`

	// 关联关系
	User User `json:"user,omitempty"`
}

// SubscriptionConfig 订阅配置基础结构
type SubscriptionConfig struct {
	// 通用配置
	Enabled  bool                   `json:"enabled"`
	Priority int32                  `json:"priority"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// BlockSubscriptionConfig 区块订阅配置
type BlockSubscriptionConfig struct {
	SubscriptionConfig

	// 区块过滤条件
	MinBlockNumber  *uint64  `json:"min_block_number"`
	MaxBlockNumber  *uint64  `json:"max_block_number"`
	IncludeEmpty    bool     `json:"include_empty"`    // 包含空区块
	MinTransactions *int32   `json:"min_transactions"` // 最小交易数
	MaxTransactions *int32   `json:"max_transactions"` // 最大交易数
	MinGasUsed      *uint64  `json:"min_gas_used"`     // 最小 Gas 使用量
	MaxGasUsed      *uint64  `json:"max_gas_used"`     // 最大 Gas 使用量
	Miners          []string `json:"miners,omitempty"` // 指定矿工地址
}

// TransactionSubscriptionConfig 交易订阅配置
type TransactionSubscriptionConfig struct {
	SubscriptionConfig

	// 交易过滤条件
	FromAddresses     []string            `json:"from_addresses,omitempty"`
	ToAddresses       []string            `json:"to_addresses,omitempty"`
	ContractAddresses []string            `json:"contract_addresses,omitempty"`
	MinValue          *string             `json:"min_value"`     // Wei 单位
	MaxValue          *string             `json:"max_value"`     // Wei 单位
	MinGasPrice       *string             `json:"min_gas_price"` // Wei 单位
	MaxGasPrice       *string             `json:"max_gas_price"` // Wei 单位
	TxTypes           []TransactionType   `json:"tx_types,omitempty"`
	TxStatuses        []TransactionStatus `json:"tx_statuses,omitempty"`
	ContractOnly      bool                `json:"contract_only"`    // 仅合约交易
	LargeValueOnly    bool                `json:"large_value_only"` // 仅大额交易
}

// GasPriceSubscriptionConfig Gas 价格订阅配置
type GasPriceSubscriptionConfig struct {
	SubscriptionConfig

	// Gas 价格过滤条件
	MinGasPrice          *float64 `json:"min_gas_price"`          // Gwei 单位
	MaxGasPrice          *float64 `json:"max_gas_price"`          // Gwei 单位
	PriceChangeThreshold float64  `json:"price_change_threshold"` // 价格变化阈值 (%)
	CheckInterval        int32    `json:"check_interval"`         // 检查间隔（秒）
}

// AlertSubscriptionConfig 告警订阅配置
type AlertSubscriptionConfig struct {
	SubscriptionConfig

	// 告警过滤条件
	AlertTypes      []AlertType     `json:"alert_types,omitempty"`
	Severities      []AlertSeverity `json:"severities,omitempty"`
	RuleIDs         []uint64        `json:"rule_ids,omitempty"`
	IncludeResolved bool            `json:"include_resolved"` // 包含已解决的告警
}

// SubscriptionFilter 订阅过滤器
type SubscriptionFilter struct {
	Field     string             `json:"field" validate:"required"`
	Operator  ComparisonOperator `json:"operator" validate:"required"`
	Value     interface{}        `json:"value" validate:"required"`
	LogicalOp LogicalOperator    `json:"logical_op,omitempty"`
}

// TableName 指定表名
func (Subscription) TableName() string {
	return "subscriptions"
}

// BeforeSave 保存前的钩子函数
func (s *Subscription) BeforeSave() error {
	s.UpdatedAt = time.Now()

	// 重置通知计数器（每小时）
	if time.Since(s.LastNotificationReset) >= time.Hour {
		s.NotificationCount = 0
		s.LastNotificationReset = time.Now()
	}

	return nil
}

// Validate 验证订阅数据
func (s *Subscription) Validate() error {
	validate := validator.New()
	if err := validate.Struct(s); err != nil {
		return err
	}

	// 验证订阅类型
	if !s.Type.IsValid() {
		return errors.New("invalid subscription type")
	}

	// 验证订阅状态
	if !s.Status.IsValid() {
		return errors.New("invalid subscription status")
	}

	// 验证配置 JSON 格式
	var config SubscriptionConfig
	if err := json.Unmarshal([]byte(s.Config), &config); err != nil {
		return errors.New("invalid config format")
	}

	// 验证过滤器 JSON 格式（如果存在）
	if s.Filters != "" {
		var filters []SubscriptionFilter
		if err := json.Unmarshal([]byte(s.Filters), &filters); err != nil {
			return errors.New("invalid filters format")
		}

		// 验证每个过滤器
		for _, filter := range filters {
			if err := validate.Struct(filter); err != nil {
				return errors.New("invalid filter: " + err.Error())
			}
			if !filter.Operator.IsValid() {
				return errors.New("invalid filter operator")
			}
			if filter.LogicalOp != "" && !filter.LogicalOp.IsValid() {
				return errors.New("invalid logical operator")
			}
		}
	}

	return nil
}

// GetConfig 获取解析后的配置
func (s *Subscription) GetConfig() (interface{}, error) {
	switch s.Type {
	case SubTypeBlock:
		var config BlockSubscriptionConfig
		err := json.Unmarshal([]byte(s.Config), &config)
		return &config, err
	case SubTypeTransaction:
		var config TransactionSubscriptionConfig
		err := json.Unmarshal([]byte(s.Config), &config)
		return &config, err
	case SubTypeGasPrice:
		var config GasPriceSubscriptionConfig
		err := json.Unmarshal([]byte(s.Config), &config)
		return &config, err
	case SubTypeAlert:
		var config AlertSubscriptionConfig
		err := json.Unmarshal([]byte(s.Config), &config)
		return &config, err
	default:
		var config SubscriptionConfig
		err := json.Unmarshal([]byte(s.Config), &config)
		return &config, err
	}
}

// SetConfig 设置配置
func (s *Subscription) SetConfig(config interface{}) error {
	data, err := json.Marshal(config)
	if err != nil {
		return err
	}
	s.Config = string(data)
	return nil
}

// GetFilters 获取解析后的过滤器
func (s *Subscription) GetFilters() ([]SubscriptionFilter, error) {
	var filters []SubscriptionFilter
	if s.Filters == "" {
		return filters, nil
	}
	err := json.Unmarshal([]byte(s.Filters), &filters)
	return filters, err
}

// SetFilters 设置过滤器
func (s *Subscription) SetFilters(filters []SubscriptionFilter) error {
	data, err := json.Marshal(filters)
	if err != nil {
		return err
	}
	s.Filters = string(data)
	return nil
}

// GetNotificationChannels 获取通知渠道配置
func (s *Subscription) GetNotificationChannels() ([]NotificationConfig, error) {
	var channels []NotificationConfig
	if s.NotificationChannels == "" {
		return channels, nil
	}
	err := json.Unmarshal([]byte(s.NotificationChannels), &channels)
	return channels, err
}

// SetNotificationChannels 设置通知渠道
func (s *Subscription) SetNotificationChannels(channels []NotificationConfig) error {
	data, err := json.Marshal(channels)
	if err != nil {
		return err
	}
	s.NotificationChannels = string(data)
	return nil
}

// IsActive 检查订阅是否激活
func (s *Subscription) IsActive() bool {
	if s.Status != SubStatusActive {
		return false
	}

	// 检查是否过期
	if s.ExpiresAt != nil && time.Now().After(*s.ExpiresAt) {
		return false
	}

	return true
}

// CanNotify 检查是否可以发送通知（考虑频率限制）
func (s *Subscription) CanNotify() bool {
	if !s.IsActive() {
		return false
	}

	// 检查频率限制
	if s.MaxNotificationsPerHour > 0 {
		// 重置计数器（如果需要）
		if time.Since(s.LastNotificationReset) >= time.Hour {
			s.NotificationCount = 0
			s.LastNotificationReset = time.Now()
		}

		return s.NotificationCount < s.MaxNotificationsPerHour
	}

	return true
}

// IncrementNotificationCount 增加通知计数
func (s *Subscription) IncrementNotificationCount() {
	s.NotificationCount++
	s.TotalNotifications++
	now := time.Now()
	s.LastTriggered = &now
}

// UpdateLastChecked 更新最后检查时间
func (s *Subscription) UpdateLastChecked() {
	now := time.Now()
	s.LastChecked = &now
}

// IsExpired 检查是否已过期
func (s *Subscription) IsExpired() bool {
	return s.ExpiresAt != nil && time.Now().After(*s.ExpiresAt)
}

// GetTemplate 获取通知模板
func (s *Subscription) GetTemplate() string {
	if s.NotificationTemplate != "" {
		return s.NotificationTemplate
	}

	// 使用默认模板
	if template, exists := SubscriptionTemplates[s.Type]; exists {
		return template
	}

	return "订阅通知: {{.Type}} - {{.Message}}"
}

// EvaluateFilter 评估单个过滤器
func (s *Subscription) EvaluateFilter(filter SubscriptionFilter, data map[string]interface{}) (bool, error) {
	value, exists := data[filter.Field]
	if !exists {
		return false, nil
	}

	switch filter.Operator {
	case OpGreaterThan:
		return compareValues(value, filter.Value, ">")
	case OpGreaterThanEqual:
		return compareValues(value, filter.Value, ">=")
	case OpLessThan:
		return compareValues(value, filter.Value, "<")
	case OpLessThanEqual:
		return compareValues(value, filter.Value, "<=")
	case OpEqual:
		return compareValues(value, filter.Value, "==")
	case OpNotEqual:
		return compareValues(value, filter.Value, "!=")
	case OpContains:
		return containsValue(value, filter.Value)
	case OpNotContains:
		contains, err := containsValue(value, filter.Value)
		return !contains, err
	default:
		return false, errors.New("unsupported operator")
	}
}

// EvaluateAllFilters 评估所有过滤器
func (s *Subscription) EvaluateAllFilters(data map[string]interface{}) (bool, error) {
	filters, err := s.GetFilters()
	if err != nil {
		return false, err
	}

	if len(filters) == 0 {
		return true, nil // 没有过滤器，默认匹配
	}

	result := true
	for i, filter := range filters {
		match, err := s.EvaluateFilter(filter, data)
		if err != nil {
			return false, err
		}

		if i == 0 {
			result = match
		} else {
			switch filter.LogicalOp {
			case LogicalAnd:
				result = result && match
			case LogicalOr:
				result = result || match
			default:
				result = result && match // 默认 AND
			}
		}
	}

	return result, nil
}

// ToJSON 序列化为 JSON
func (s *Subscription) ToJSON() ([]byte, error) {
	return json.Marshal(s)
}

// FromJSON 从 JSON 反序列化
func (s *Subscription) FromJSON(data []byte) error {
	return json.Unmarshal(data, s)
}

// ToCompactJSON 序列化为紧凑 JSON（排除关联数据）
func (s *Subscription) ToCompactJSON() ([]byte, error) {
	compactSub := struct {
		ID                      uint64             `json:"id"`
		Name                    string             `json:"name"`
		Type                    SubscriptionType   `json:"type"`
		Status                  SubscriptionStatus `json:"status"`
		UserID                  uint64             `json:"user_id"`
		MaxNotificationsPerHour int32              `json:"max_notifications_per_hour"`
		TotalNotifications      uint64             `json:"total_notifications"`
		LastTriggered           *time.Time         `json:"last_triggered"`
		ExpiresAt               *time.Time         `json:"expires_at"`
		CreatedAt               time.Time          `json:"created_at"`
		UpdatedAt               time.Time          `json:"updated_at"`
	}{
		ID:                      s.ID,
		Name:                    s.Name,
		Type:                    s.Type,
		Status:                  s.Status,
		UserID:                  s.UserID,
		MaxNotificationsPerHour: s.MaxNotificationsPerHour,
		TotalNotifications:      s.TotalNotifications,
		LastTriggered:           s.LastTriggered,
		ExpiresAt:               s.ExpiresAt,
		CreatedAt:               s.CreatedAt,
		UpdatedAt:               s.UpdatedAt,
	}
	return json.Marshal(compactSub)
}

// 查询参数结构

// SubscriptionQueryParams 订阅查询参数
type SubscriptionQueryParams struct {
	PaginationParams
	FilterParams

	// 订阅特定过滤条件
	Name    string             `json:"name"`
	Type    SubscriptionType   `json:"type"`
	Status  SubscriptionStatus `json:"status"`
	UserID  *uint64            `json:"user_id"`
	Expired *bool              `json:"expired"`
}

// BuildWhereClause 构建查询条件
func (q *SubscriptionQueryParams) BuildWhereClause() (string, []interface{}) {
	var conditions []string
	var args []interface{}
	argIndex := 1

	// 名称过滤
	if q.Name != "" {
		conditions = append(conditions, "name ILIKE $"+string(rune(argIndex+'0')))
		args = append(args, "%"+q.Name+"%")
		argIndex++
	}

	// 类型过滤
	if q.Type != "" {
		conditions = append(conditions, "type = $"+string(rune(argIndex+'0')))
		args = append(args, q.Type)
		argIndex++
	}

	// 状态过滤
	if q.Status != "" {
		conditions = append(conditions, "status = $"+string(rune(argIndex+'0')))
		args = append(args, q.Status)
		argIndex++
	}

	// 用户过滤
	if q.UserID != nil {
		conditions = append(conditions, "user_id = $"+string(rune(argIndex+'0')))
		args = append(args, *q.UserID)
		argIndex++
	}

	// 过期状态过滤
	if q.Expired != nil {
		if *q.Expired {
			conditions = append(conditions, "expires_at IS NOT NULL AND expires_at < NOW()")
		} else {
			conditions = append(conditions, "expires_at IS NULL OR expires_at >= NOW()")
		}
	}

	// 时间范围
	if q.StartTime != nil {
		conditions = append(conditions, "created_at >= $"+string(rune(argIndex+'0')))
		args = append(args, *q.StartTime)
		argIndex++
	}
	if q.EndTime != nil {
		conditions = append(conditions, "created_at <= $"+string(rune(argIndex+'0')))
		args = append(args, *q.EndTime)
		argIndex++
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + conditions[0]
		for i := 1; i < len(conditions); i++ {
			whereClause += " AND " + conditions[i]
		}
	}

	return whereClause, args
}

// 请求结构

// CreateSubscriptionRequest 创建订阅请求
type CreateSubscriptionRequest struct {
	Name                    string               `json:"name" validate:"required,min=1,max=255"`
	Description             string               `json:"description"`
	Type                    SubscriptionType     `json:"type" validate:"required"`
	Config                  interface{}          `json:"config" validate:"required"`
	Filters                 []SubscriptionFilter `json:"filters"`
	NotificationChannels    []NotificationConfig `json:"notification_channels"`
	NotificationTemplate    string               `json:"notification_template"`
	MaxNotificationsPerHour int32                `json:"max_notifications_per_hour" validate:"min=0,max=1000"`
	ExpiresAt               *int64               `json:"expires_at"` // Unix 时间戳
}

// ToSubscription 转换为订阅模型
func (r *CreateSubscriptionRequest) ToSubscription(userID uint64) (*Subscription, error) {
	// 序列化配置
	configJSON, err := json.Marshal(r.Config)
	if err != nil {
		return nil, err
	}

	// 序列化过滤器
	filtersJSON, err := json.Marshal(r.Filters)
	if err != nil {
		return nil, err
	}

	// 序列化通知渠道
	channelsJSON, err := json.Marshal(r.NotificationChannels)
	if err != nil {
		return nil, err
	}

	var expiresAt *time.Time
	if r.ExpiresAt != nil {
		t := time.Unix(*r.ExpiresAt, 0)
		expiresAt = &t
	}

	return &Subscription{
		Name:                    r.Name,
		Description:             r.Description,
		Type:                    r.Type,
		Status:                  SubStatusActive,
		UserID:                  userID,
		Config:                  string(configJSON),
		Filters:                 string(filtersJSON),
		NotificationChannels:    string(channelsJSON),
		NotificationTemplate:    r.NotificationTemplate,
		MaxNotificationsPerHour: r.MaxNotificationsPerHour,
		LastNotificationReset:   time.Now(),
		ExpiresAt:               expiresAt,
	}, nil
}

// Validate 验证创建请求
func (r *CreateSubscriptionRequest) Validate() error {
	validate := validator.New()
	if err := validate.Struct(r); err != nil {
		return err
	}

	// 验证订阅类型
	if !r.Type.IsValid() {
		return errors.New("invalid subscription type")
	}

	// 验证过滤器
	for _, filter := range r.Filters {
		if err := validate.Struct(filter); err != nil {
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

// UpdateSubscriptionRequest 更新订阅请求
type UpdateSubscriptionRequest struct {
	Name                    *string               `json:"name" validate:"omitempty,min=1,max=255"`
	Description             *string               `json:"description"`
	Status                  *SubscriptionStatus   `json:"status"`
	Config                  interface{}           `json:"config"`
	Filters                 *[]SubscriptionFilter `json:"filters"`
	NotificationChannels    *[]NotificationConfig `json:"notification_channels"`
	NotificationTemplate    *string               `json:"notification_template"`
	MaxNotificationsPerHour *int32                `json:"max_notifications_per_hour" validate:"omitempty,min=0,max=1000"`
	ExpiresAt               *int64                `json:"expires_at"` // Unix 时间戳
}

// ApplyToSubscription 应用更新到订阅模型
func (r *UpdateSubscriptionRequest) ApplyToSubscription(sub *Subscription) error {
	if r.Name != nil {
		sub.Name = *r.Name
	}
	if r.Description != nil {
		sub.Description = *r.Description
	}
	if r.Status != nil {
		sub.Status = *r.Status
	}
	if r.Config != nil {
		if err := sub.SetConfig(r.Config); err != nil {
			return err
		}
	}
	if r.Filters != nil {
		if err := sub.SetFilters(*r.Filters); err != nil {
			return err
		}
	}
	if r.NotificationChannels != nil {
		if err := sub.SetNotificationChannels(*r.NotificationChannels); err != nil {
			return err
		}
	}
	if r.NotificationTemplate != nil {
		sub.NotificationTemplate = *r.NotificationTemplate
	}
	if r.MaxNotificationsPerHour != nil {
		sub.MaxNotificationsPerHour = *r.MaxNotificationsPerHour
	}
	if r.ExpiresAt != nil {
		if *r.ExpiresAt == 0 {
			sub.ExpiresAt = nil
		} else {
			t := time.Unix(*r.ExpiresAt, 0)
			sub.ExpiresAt = &t
		}
	}

	return nil
}
