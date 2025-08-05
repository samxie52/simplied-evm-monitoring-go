package models

// TransactionType 交易类型枚举
type TransactionType string

const (
	TxTypeLegacy     TransactionType = "legacy"      // 传统交易
	TxTypeAccessList TransactionType = "access_list" // EIP-2930
	TxTypeDynamicFee TransactionType = "dynamic_fee" // EIP-1559
)

// String 返回字符串表示
func (t TransactionType) String() string {
	return string(t)
}

// IsValid 验证交易类型是否有效
func (t TransactionType) IsValid() bool {
	switch t {
	case TxTypeLegacy, TxTypeAccessList, TxTypeDynamicFee:
		return true
	default:
		return false
	}
}

// TransactionStatus 交易状态枚举
type TransactionStatus string

const (
	TxStatusPending TransactionStatus = "pending" // 待处理
	TxStatusSuccess TransactionStatus = "success" // 成功
	TxStatusFailed  TransactionStatus = "failed"  // 失败
	TxStatusDropped TransactionStatus = "dropped" // 被丢弃
)

// String 返回字符串表示
func (s TransactionStatus) String() string {
	return string(s)
}

// IsValid 验证交易状态是否有效
func (s TransactionStatus) IsValid() bool {
	switch s {
	case TxStatusPending, TxStatusSuccess, TxStatusFailed, TxStatusDropped:
		return true
	default:
		return false
	}
}

// AlertType 告警类型枚举
type AlertType string

const (
	AlertTypeGasPrice           AlertType = "gas_price"           // Gas 价格告警
	AlertTypeLargeTransfer      AlertType = "large_transfer"      // 大额转账告警
	AlertTypeBlockTime          AlertType = "block_time"          // 出块时间告警
	AlertTypeNetworkCongestion  AlertType = "network_congestion"  // 网络拥堵告警
	AlertTypeContractEvent      AlertType = "contract_event"      // 合约事件告警
	AlertTypeCustom             AlertType = "custom"              // 自定义告警
	AlertTypeAddressActivity    AlertType = "address_activity"    // 地址活动告警
	AlertTypeTokenTransfer      AlertType = "token_transfer"      // 代币转账告警
	AlertTypeSystemHealth       AlertType = "system_health"       // 系统健康告警
)

// String 返回字符串表示
func (a AlertType) String() string {
	return string(a)
}

// IsValid 验证告警类型是否有效
func (a AlertType) IsValid() bool {
	switch a {
	case AlertTypeGasPrice, AlertTypeLargeTransfer, AlertTypeBlockTime,
		AlertTypeNetworkCongestion, AlertTypeContractEvent, AlertTypeCustom,
		AlertTypeAddressActivity, AlertTypeTokenTransfer, AlertTypeSystemHealth:
		return true
	default:
		return false
	}
}

// AlertSeverity 告警严重级别
type AlertSeverity string

const (
	SeverityLow      AlertSeverity = "low"      // 低
	SeverityMedium   AlertSeverity = "medium"   // 中
	SeverityHigh     AlertSeverity = "high"     // 高
	SeverityCritical AlertSeverity = "critical" // 紧急
)

// String 返回字符串表示
func (s AlertSeverity) String() string {
	return string(s)
}

// IsValid 验证告警严重级别是否有效
func (s AlertSeverity) IsValid() bool {
	switch s {
	case SeverityLow, SeverityMedium, SeverityHigh, SeverityCritical:
		return true
	default:
		return false
	}
}

// GetPriority 获取优先级数值（数值越大优先级越高）
func (s AlertSeverity) GetPriority() int {
	switch s {
	case SeverityLow:
		return 1
	case SeverityMedium:
		return 2
	case SeverityHigh:
		return 3
	case SeverityCritical:
		return 4
	default:
		return 0
	}
}

// AlertStatus 告警状态
type AlertStatus string

const (
	AlertStatusActive   AlertStatus = "active"   // 激活
	AlertStatusInactive AlertStatus = "inactive" // 未激活
	AlertStatusPaused   AlertStatus = "paused"   // 暂停
	AlertStatusDeleted  AlertStatus = "deleted"  // 已删除
)

// String 返回字符串表示
func (s AlertStatus) String() string {
	return string(s)
}

// IsValid 验证告警状态是否有效
func (s AlertStatus) IsValid() bool {
	switch s {
	case AlertStatusActive, AlertStatusInactive, AlertStatusPaused, AlertStatusDeleted:
		return true
	default:
		return false
	}
}

// NotificationChannel 通知渠道
type NotificationChannel string

const (
	ChannelTelegram NotificationChannel = "telegram" // Telegram
	ChannelEmail    NotificationChannel = "email"    // 邮件
	ChannelWebhook  NotificationChannel = "webhook"  // Webhook
	ChannelSMS      NotificationChannel = "sms"      // 短信
	ChannelSlack    NotificationChannel = "slack"    // Slack
	ChannelDiscord  NotificationChannel = "discord"  // Discord
)

// String 返回字符串表示
func (c NotificationChannel) String() string {
	return string(c)
}

// IsValid 验证通知渠道是否有效
func (c NotificationChannel) IsValid() bool {
	switch c {
	case ChannelTelegram, ChannelEmail, ChannelWebhook, ChannelSMS, ChannelSlack, ChannelDiscord:
		return true
	default:
		return false
	}
}

// UserRole 用户角色枚举
type UserRole string

const (
	RoleAdmin    UserRole = "admin"    // 管理员
	RoleUser     UserRole = "user"     // 普通用户
	RoleViewer   UserRole = "viewer"   // 只读用户
	RoleOperator UserRole = "operator" // 操作员
)

// String 返回字符串表示
func (r UserRole) String() string {
	return string(r)
}

// IsValid 验证用户角色是否有效
func (r UserRole) IsValid() bool {
	switch r {
	case RoleAdmin, RoleUser, RoleViewer, RoleOperator:
		return true
	default:
		return false
	}
}

// HasPermission 检查角色是否有指定权限
func (r UserRole) HasPermission(permission string) bool {
	switch r {
	case RoleAdmin:
		return true // 管理员有所有权限
	case RoleOperator:
		// 操作员权限
		switch permission {
		case "read", "write", "alert_manage":
			return true
		default:
			return false
		}
	case RoleUser:
		// 普通用户权限
		switch permission {
		case "read", "alert_create", "subscription_manage":
			return true
		default:
			return false
		}
	case RoleViewer:
		// 只读用户权限
		return permission == "read"
	default:
		return false
	}
}

// UserStatus 用户状态
type UserStatus string

const (
	UserStatusActive    UserStatus = "active"    // 激活
	UserStatusInactive  UserStatus = "inactive"  // 未激活
	UserStatusSuspended UserStatus = "suspended" // 暂停
	UserStatusDeleted   UserStatus = "deleted"   // 已删除
)

// String 返回字符串表示
func (s UserStatus) String() string {
	return string(s)
}

// IsValid 验证用户状态是否有效
func (s UserStatus) IsValid() bool {
	switch s {
	case UserStatusActive, UserStatusInactive, UserStatusSuspended, UserStatusDeleted:
		return true
	default:
		return false
	}
}

// CanLogin 检查用户状态是否允许登录
func (s UserStatus) CanLogin() bool {
	return s == UserStatusActive
}

// SubscriptionType 订阅类型
type SubscriptionType string

const (
	SubTypeAddress    SubscriptionType = "address"    // 地址监控
	SubTypeContract   SubscriptionType = "contract"   // 合约监控
	SubTypeToken      SubscriptionType = "token"      // 代币监控
	SubTypeGasPrice   SubscriptionType = "gas_price"  // Gas 价格监控
	SubTypeNetwork    SubscriptionType = "network"    // 网络状态监控
	SubTypeBlock      SubscriptionType = "block"      // 区块监控
	SubTypeTransaction SubscriptionType = "transaction" // 交易监控
	SubTypeAlert      SubscriptionType = "alert"      // 告警监控
)

// String 返回字符串表示
func (s SubscriptionType) String() string {
	return string(s)
}

// IsValid 验证订阅类型是否有效
func (s SubscriptionType) IsValid() bool {
	switch s {
	case SubTypeAddress, SubTypeContract, SubTypeToken, SubTypeGasPrice,
		SubTypeNetwork, SubTypeBlock, SubTypeTransaction, SubTypeAlert:
		return true
	default:
		return false
	}
}

// SubscriptionStatus 订阅状态
type SubscriptionStatus string

const (
	SubStatusActive   SubscriptionStatus = "active"   // 激活
	SubStatusInactive SubscriptionStatus = "inactive" // 非激活
	SubStatusPaused   SubscriptionStatus = "paused"   // 暂停
	SubStatusExpired  SubscriptionStatus = "expired"  // 过期
)

// String 返回字符串表示
func (s SubscriptionStatus) String() string {
	return string(s)
}

// IsValid 验证订阅状态是否有效
func (s SubscriptionStatus) IsValid() bool {
	switch s {
	case SubStatusActive, SubStatusInactive, SubStatusPaused, SubStatusExpired:
		return true
	default:
		return false
	}
}

// ComparisonOperator 比较操作符
type ComparisonOperator string

const (
	OpGreaterThan       ComparisonOperator = "gt"   // 大于
	OpGreaterThanEqual  ComparisonOperator = "gte"  // 大于等于
	OpLessThan          ComparisonOperator = "lt"   // 小于
	OpLessThanEqual     ComparisonOperator = "lte"  // 小于等于
	OpEqual             ComparisonOperator = "eq"   // 等于
	OpNotEqual          ComparisonOperator = "ne"   // 不等于
	OpContains          ComparisonOperator = "contains" // 包含
	OpNotContains       ComparisonOperator = "not_contains" // 不包含
	OpStartsWith        ComparisonOperator = "starts_with" // 开始于
	OpEndsWith          ComparisonOperator = "ends_with" // 结束于
)

// String 返回字符串表示
func (o ComparisonOperator) String() string {
	return string(o)
}

// IsValid 验证比较操作符是否有效
func (o ComparisonOperator) IsValid() bool {
	switch o {
	case OpGreaterThan, OpGreaterThanEqual, OpLessThan, OpLessThanEqual,
		OpEqual, OpNotEqual, OpContains, OpNotContains, OpStartsWith, OpEndsWith:
		return true
	default:
		return false
	}
}

// GetDescription 获取操作符描述
func (o ComparisonOperator) GetDescription() string {
	switch o {
	case OpGreaterThan:
		return "大于"
	case OpGreaterThanEqual:
		return "大于等于"
	case OpLessThan:
		return "小于"
	case OpLessThanEqual:
		return "小于等于"
	case OpEqual:
		return "等于"
	case OpNotEqual:
		return "不等于"
	case OpContains:
		return "包含"
	case OpNotContains:
		return "不包含"
	case OpStartsWith:
		return "开始于"
	case OpEndsWith:
		return "结束于"
	default:
		return "未知操作符"
	}
}

// LogicalOperator 逻辑操作符
type LogicalOperator string

const (
	LogicalAnd LogicalOperator = "and" // 与
	LogicalOr  LogicalOperator = "or"  // 或
)

// String 返回字符串表示
func (l LogicalOperator) String() string {
	return string(l)
}

// IsValid 验证逻辑操作符是否有效
func (l LogicalOperator) IsValid() bool {
	switch l {
	case LogicalAnd, LogicalOr:
		return true
	default:
		return false
	}
}

// NotificationStatus 通知状态
type NotificationStatus string

const (
	NotificationStatusPending NotificationStatus = "pending" // 待发送
	NotificationStatusSent    NotificationStatus = "sent"    // 已发送
	NotificationStatusFailed  NotificationStatus = "failed"  // 发送失败
	NotificationStatusRetry   NotificationStatus = "retry"   // 重试中
)

// String 返回字符串表示
func (n NotificationStatus) String() string {
	return string(n)
}

// IsValid 验证通知状态是否有效
func (n NotificationStatus) IsValid() bool {
	switch n {
	case NotificationStatusPending, NotificationStatusSent, NotificationStatusFailed, NotificationStatusRetry:
		return true
	default:
		return false
	}
}

// IsFinal 检查是否为最终状态
func (n NotificationStatus) IsFinal() bool {
	return n == NotificationStatusSent || n == NotificationStatusFailed
}

// 常用数值常量
const (
	// 默认 Gas 限制
	DefaultGasLimit = 21000
	
	// 区块时间阈值（秒）
	NormalBlockTime = 12
	SlowBlockTime   = 20
	
	// 网络拥堵阈值（Gas 利用率百分比）
	CongestionThreshold = 90.0
	
	// 告警冷却时间（秒）
	DefaultCooldownTime = 300
	
	// 默认时间窗口（秒）
	DefaultTimeWindow = 60
)

// 预定义的告警模板
var (
	AlertTemplates = map[AlertType]string{
		AlertTypeGasPrice: "Gas 价格告警: 当前 Gas 价格为 {{.Value}} Gwei，{{.Operator}} 阈值 {{.Threshold}} Gwei",
		AlertTypeLargeTransfer: "大额转账告警: 检测到 {{.Value}} ETH 的大额转账，从 {{.From}} 到 {{.To}}",
		AlertTypeBlockTime: "出块时间告警: 当前出块时间为 {{.Value}} 秒，超过正常范围",
		AlertTypeNetworkCongestion: "网络拥堵告警: 当前网络 Gas 利用率为 {{.Value}}%，网络拥堵",
		AlertTypeContractEvent: "合约事件告警: 合约 {{.Contract}} 触发了事件 {{.Event}}",
		AlertTypeAddressActivity: "地址活动告警: 地址 {{.Address}} 发生了 {{.Activity}} 活动",
		AlertTypeTokenTransfer: "代币转账告警: 检测到 {{.Amount}} {{.Token}} 代币转账",
		AlertTypeSystemHealth: "系统健康告警: {{.Component}} 组件状态异常",
	}
)
