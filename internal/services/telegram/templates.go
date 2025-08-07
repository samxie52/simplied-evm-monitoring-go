package telegram

import (
	"fmt"
	"simplied-evm-monitoring-go/internal/models"
)

// MessageTemplates 消息模板管理器
type MessageTemplates struct {
	templates map[models.AlertType]string
}

// NewMessageTemplates 创建新的消息模板管理器
func NewMessageTemplates() *MessageTemplates {
	templates := &MessageTemplates{
		templates: make(map[models.AlertType]string),
	}
	templates.initializeTemplates()
	return templates
}

// initializeTemplates 初始化所有模板
func (mt *MessageTemplates) initializeTemplates() {
	// 大额转账告警模板
	mt.templates[models.AlertTypeLargeTransfer] = `{{Emoji}} **LARGE TRANSFER ALERT**

**Transaction Details:**
• **Amount:** {{TriggerValue}} ETH
• **Hash:** {{SourceID}}
• **Time:** {{TriggerTime}}

**Rule Information:**
• **Rule:** {{RuleName}}
• **Threshold:** {{Threshold}} ETH
• **Severity:** {{SeverityIcon}} {{Severity}}

{{TransactionLink}}

**Alert ID:** #{{ID}}
**Status:** {{Status}}`

	// Gas 价格告警模板
	mt.templates[models.AlertTypeGasPrice] = `{{Emoji}} **GAS PRICE ALERT**

**Gas Information:**
• **Current Price:** {{TriggerValue}} gwei
• **Threshold:** {{Threshold}} gwei
• **Network:** Ethereum Mainnet
• **Time:** {{TriggerTime}}

**Rule Information:**
• **Rule:** {{RuleName}}
• **Severity:** {{SeverityIcon}} {{Severity}}
• **Condition:** Gas price {{Operator}} {{Threshold}} gwei

**Alert ID:** #{{ID}}
**Status:** {{Status}}`

	// 网络拥堵告警模板
	mt.templates[models.AlertTypeNetworkCongestion] = `{{Emoji}} **NETWORK CONGESTION ALERT**

**Network Status:**
• **Congestion Level:** {{TriggerValue}}%
• **Threshold:** {{Threshold}}%
• **Pending Transactions:** {{Context.pending_tx_count}}
• **Average Block Time:** {{Context.avg_block_time}}s
• **Time:** {{TriggerTime}}

**Rule Information:**
• **Rule:** {{RuleName}}
• **Severity:** {{SeverityIcon}} {{Severity}}

**Alert ID:** #{{ID}}
**Status:** {{Status}}`

	// 智能合约事件告警模板
	mt.templates[models.AlertTypeContractEvent] = `{{Emoji}} **CONTRACT EVENT ALERT**

**Event Details:**
• **Contract:** {{Context.contract_address}}
• **Event:** {{Context.event_name}}
• **Value:** {{TriggerValue}}
• **Transaction:** {{SourceID}}
• **Block:** {{Context.block_number}}
• **Time:** {{TriggerTime}}

**Rule Information:**
• **Rule:** {{RuleName}}
• **Severity:** {{SeverityIcon}} {{Severity}}

{{#if TransactionLink}}
🔗 **View Transaction:** [Click Here]({{TransactionLink}})
{{/if}}

**Alert ID:** #{{ID}}
**Status:** {{Status}}`

	// 地址监控告警模板
	mt.templates[models.AlertTypeAddressActivity] = `{{Emoji}} **ADDRESS MONITORING ALERT**

**Address Activity:**
• **Address:** {{Context.address}}
• **Activity Type:** {{Context.activity_type}}
• **Amount:** {{TriggerValue}} ETH
• **Transaction:** {{SourceID}}
• **Time:** {{TriggerTime}}

**Rule Information:**
• **Rule:** {{RuleName}}
• **Threshold:** {{Threshold}} ETH
• **Severity:** {{SeverityIcon}} {{Severity}}

{{#if TransactionLink}}
🔗 **View Transaction:** [Click Here]({{TransactionLink}})
{{/if}}

**Alert ID:** #{{ID}}
**Status:** {{Status}}`

	// 价格变动告警模板
	mt.templates[models.AlertTypeTokenTransfer] = `{{Emoji}} **TOKEN TRANSFER ALERT**

**Price Information:**
• **Asset:** {{Context.asset_symbol}}
• **Current Price:** ${{TriggerValue}}
• **Change:** {{Context.price_change}}%
• **Threshold:** {{Threshold}}%
• **Time:** {{TriggerTime}}

**Rule Information:**
• **Rule:** {{RuleName}}
• **Severity:** {{SeverityIcon}} {{Severity}}
• **Condition:** Price change {{Operator}} {{Threshold}}%

**Alert ID:** #{{ID}}
**Status:** {{Status}}`

	// 系统告警模板
	mt.templates[models.AlertTypeSystemHealth] = `{{Emoji}} **SYSTEM ALERT**

**System Information:**
• **Title:** {{Title}}
• **Issue:** {{Message}}
• **Metric:** {{TriggerValue}}
• **Threshold:** {{Threshold}}
• **Time:** {{TriggerTime}}

**Rule Information:**
• **Rule:** {{RuleName}}
• **Severity:** {{SeverityIcon}} {{Severity}}

**Alert ID:** #{{ID}}
**Status:** {{Status}}`

	// 自定义告警模板
	mt.templates[models.AlertTypeCustom] = `{{Emoji}} **CUSTOM ALERT**

**Alert Information:**
• **Title:** {{Title}}
• **Message:** {{Message}}
• **Value:** {{TriggerValue}}
• **Time:** {{TriggerTime}}

**Rule Information:**
• **Rule:** {{RuleName}}
• **Severity:** {{SeverityIcon}} {{Severity}}
• **Threshold:** {{Threshold}}

**Alert ID:** #{{ID}}
**Status:** {{Status}}`
}

// GetTemplate 获取指定类型的模板
func (mt *MessageTemplates) GetTemplate(alertType models.AlertType) (string, error) {
	template, exists := mt.templates[alertType]
	if !exists {
		return "", fmt.Errorf("template not found for alert type: %s", alertType)
	}
	return template, nil
}

// GetDefaultTemplate 获取默认模板
func (mt *MessageTemplates) GetDefaultTemplate() string {
	return `{{Emoji}} **ALERT NOTIFICATION**

**Alert Information:**
• **Title:** {{Title}}
• **Type:** {{Type}}
• **Severity:** {{SeverityIcon}} {{Severity}}
• **Message:** {{Message}}
• **Value:** {{TriggerValue}}
• **Time:** {{TriggerTime}}

**Rule Information:**
• **Rule:** {{RuleName}}
• **Threshold:** {{Threshold}}

**Alert ID:** #{{ID}}
**Status:** {{Status}}`
}

// GetBatchTemplate 获取批量告警模板
func (mt *MessageTemplates) GetBatchTemplate() string {
	return `{{Emoji}} **BATCH ALERT NOTIFICATION**

**Summary:**
• **Alert Type:** {{Type}}
• **Severity:** {{Severity}}
• **Count:** {{Count}} alerts
• **Time:** {{Timestamp}}

**Alert List:**
{{AlertList}}

📊 **Batch Summary:** {{Count}} similar alerts detected in the last few minutes.`
}

// GetRuleStatusTemplate 获取规则状态模板
func (mt *MessageTemplates) GetRuleStatusTemplate() string {
	return `{{Emoji}} **RULE STATUS**

**Rule Information:**
• **Name:** {{Name}}
• **Type:** {{Type}}
• **Status:** {{Status}}
• **Severity:** {{Severity}}

**Configuration:**
• **Threshold:** {{Threshold}}
• **Operator:** {{Operator}}
• **Trigger Count:** {{TriggerCount}}

**Timestamps:**
• **Created:** {{CreatedAt}}
• **Updated:** {{UpdatedAt}}
• **Last Triggered:** {{LastTriggered}}
• **Last Checked:** {{LastChecked}}

**Rule ID:** #{{ID}}`
}

// GetSystemStatusTemplate 获取系统状态模板
func (mt *MessageTemplates) GetSystemStatusTemplate() string {
	return `📊 **SYSTEM STATUS**

**Alert Manager:**
• **Active Rules:** {{active_rules}}
• **Total Alerts:** {{total_alerts}}
• **Processed Alerts:** {{processed_alerts}}
• **Failed Alerts:** {{failed_alerts}}
• **Success Rate:** {{success_rate}}%

**Telegram Bot:**
• **Total Messages:** {{total_messages}}
• **Sent Messages:** {{sent_messages}}
• **Failed Messages:** {{failed_messages}}
• **Total Commands:** {{total_commands}}
• **Uptime:** {{uptime}}

**Performance:**
• **Average Response Time:** {{avg_response_time}}ms
• **Queue Size:** {{queue_size}}
• **Memory Usage:** {{memory_usage}}MB
• **CPU Usage:** {{cpu_usage}}%

**Last Updated:** {{timestamp}}`
}

// GetWelcomeTemplate 获取欢迎消息模板
func (mt *MessageTemplates) GetWelcomeTemplate() string {
	return `👋 **Welcome to Ethereum Alert Bot!**

**Available Commands:**
• **/start** - Start the bot
• **/help** - Show help information
• **/status** - Show system status
• **/alerts** - List active alert rules
• **/stats** - Show statistics

**Alert Types Supported:**
• 🔄 Large Transfer Monitoring
• ⛽ Gas Price Alerts
• 🌐 Network Congestion
• 📄 Smart Contract Events
• 👤 Address Monitoring
• 📈 Price Movement Alerts

**Getting Started:**
1. Use /alerts to see your active rules
2. Use /stats to check system status
3. Configure your alert rules via the web interface

For support, contact your administrator.`
}

// GetHelpTemplate 获取帮助消息模板
func (mt *MessageTemplates) GetHelpTemplate() string {
	return `ℹ️ **HELP - Ethereum Alert Bot**

**Basic Commands:**
• **/start** - Initialize the bot
• **/help** - Show this help message
• **/status** - Display system status
• **/alerts** - List your active alert rules
• **/stats** - Show detailed statistics

**Admin Commands:**
• **/admin status** - Admin system status
• **/admin rules** - Manage all rules
• **/admin users** - User management

**Alert Information:**
Each alert includes:
• Alert type and severity
• Trigger value and threshold
• Transaction details (if applicable)
• Direct links to blockchain explorers

**Severity Levels:**
• 🔴 **Critical** - Immediate attention required
• 🟠 **High** - Important but not critical
• 🟡 **Medium** - Moderate importance
• 🟢 **Low** - Informational

**Support:**
If you encounter issues, please contact your system administrator.

**Version:** 1.0.0`
}

// GetErrorTemplate 获取错误消息模板
func (mt *MessageTemplates) GetErrorTemplate() string {
	return `❌ **ERROR**

**Error Information:**
• **Type:** {{error_type}}
• **Message:** {{error_message}}
• **Time:** {{timestamp}}

**Suggested Actions:**
• Check your command syntax
• Verify your permissions
• Contact administrator if issue persists

**Error Code:** {{error_code}}`
}

// SetTemplate 设置自定义模板
func (mt *MessageTemplates) SetTemplate(alertType models.AlertType, template string) {
	mt.templates[alertType] = template
}

// GetAllTemplates 获取所有模板
func (mt *MessageTemplates) GetAllTemplates() map[models.AlertType]string {
	result := make(map[models.AlertType]string)
	for k, v := range mt.templates {
		result[k] = v
	}
	return result
}

// ValidateTemplate 验证模板格式
func (mt *MessageTemplates) ValidateTemplate(template string) error {
	// 简单的模板验证，检查是否包含基本的占位符
	requiredPlaceholders := []string{"{{Title}}", "{{Severity}}", "{{TriggerTime}}"}
	
	for _, placeholder := range requiredPlaceholders {
		if !containsString(template, placeholder) {
			return fmt.Errorf("template missing required placeholder: %s", placeholder)
		}
	}
	
	return nil
}

// containsString 检查字符串是否包含子字符串
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && 
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || 
		 containsStringAt(s, substr, 1)))
}

// containsStringAt 在指定位置开始检查是否包含子字符串
func containsStringAt(s, substr string, start int) bool {
	if start >= len(s) {
		return false
	}
	for i := start; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
