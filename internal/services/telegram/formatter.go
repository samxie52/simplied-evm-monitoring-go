package telegram

import (
	"fmt"
	"simplied-evm-monitoring-go/internal/models"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

// MessageFormatter 消息格式化器
type MessageFormatter struct {
	templates *MessageTemplates
	config    *FormatterConfig
}

// FormatterConfig 格式化器配置
type FormatterConfig struct {
	// 是否启用 Markdown 格式
	EnableMarkdown bool
	// 是否包含交易链接
	IncludeTransactionLinks bool
	// Etherscan 基础 URL
	EtherscanBaseURL string
	// 时区
	Timezone string
	// 最大消息长度
	MaxMessageLength int
	// 是否启用表情符号
	EnableEmojis bool
}

// NewMessageFormatter 创建新的消息格式化器
func NewMessageFormatter(config *FormatterConfig) *MessageFormatter {
	if config == nil {
		config = &FormatterConfig{
			EnableMarkdown:          true,
			IncludeTransactionLinks: true,
			EtherscanBaseURL:        "https://etherscan.io",
			Timezone:                "UTC",
			MaxMessageLength:        4096, // Telegram 消息最大长度
			EnableEmojis:            true,
		}
	}

	return &MessageFormatter{
		templates: NewMessageTemplates(),
		config:    config,
	}
}

// FormatAlert 格式化告警消息
func (f *MessageFormatter) FormatAlert(alert *models.Alert) (string, error) {
	if alert == nil {
		return "", fmt.Errorf("alert cannot be nil")
	}

	// 根据告警类型选择模板
	template, err := f.templates.GetTemplate(alert.Type)
	if err != nil {
		logrus.WithError(err).WithField("alert_type", alert.Type).Warn("Failed to get template, using default")
		template = f.templates.GetDefaultTemplate()
	}

	// 准备格式化数据
	data := f.prepareFormatData(alert)

	// 应用模板
	message, err := f.applyTemplate(template, data)
	if err != nil {
		return "", fmt.Errorf("failed to apply template: %w", err)
	}

	// 后处理
	message = f.postProcessMessage(message)

	return message, nil
}

// FormatAlertBatch 批量格式化告警消息
func (f *MessageFormatter) FormatAlertBatch(alerts []*models.Alert) ([]string, error) {
	if len(alerts) == 0 {
		return nil, fmt.Errorf("alerts list cannot be empty")
	}

	// 如果只有一个告警，直接格式化
	if len(alerts) == 1 {
		message, err := f.FormatAlert(alerts[0])
		if err != nil {
			return nil, err
		}
		return []string{message}, nil
	}

	// 多个告警，检查是否可以合并
	if f.canBatchAlerts(alerts) {
		message, err := f.formatBatchAlert(alerts)
		if err != nil {
			return nil, err
		}
		return []string{message}, nil
	}

	// 不能合并，分别格式化
	messages := make([]string, 0, len(alerts))
	for _, alert := range alerts {
		message, err := f.FormatAlert(alert)
		if err != nil {
			logrus.WithError(err).WithField("alert_id", alert.ID).Error("Failed to format alert")
			continue
		}
		messages = append(messages, message)
	}

	return messages, nil
}

// FormatRuleStatus 格式化规则状态消息
func (f *MessageFormatter) FormatRuleStatus(rule *models.AlertRule) (string, error) {
	if rule == nil {
		return "", fmt.Errorf("rule cannot be nil")
	}

	template := f.templates.GetRuleStatusTemplate()
	data := f.prepareRuleStatusData(rule)

	message, err := f.applyTemplate(template, data)
	if err != nil {
		return "", fmt.Errorf("failed to apply rule status template: %w", err)
	}

	return f.postProcessMessage(message), nil
}

// FormatSystemStatus 格式化系统状态消息
func (f *MessageFormatter) FormatSystemStatus(stats map[string]interface{}) (string, error) {
	template := f.templates.GetSystemStatusTemplate()
	
	message, err := f.applyTemplate(template, stats)
	if err != nil {
		return "", fmt.Errorf("failed to apply system status template: %w", err)
	}

	return f.postProcessMessage(message), nil
}

// prepareFormatData 准备格式化数据
func (f *MessageFormatter) prepareFormatData(alert *models.Alert) map[string]interface{} {
	data := map[string]interface{}{
		"ID":           alert.ID,
		"Title":        alert.Title,
		"Message":      alert.Message,
		"Type":         alert.Type,
		"Severity":     alert.Severity,
		"TriggerValue": alert.TriggerValue,
		"TriggerTime":  f.formatTime(alert.TriggerTime),
		"Status":       alert.Status,
		"Emoji":        f.getAlertEmoji(alert.Severity),
		"SeverityIcon": f.getSeverityIcon(alert.Severity),
	}

	// 添加触发数据
	if triggerData, err := alert.GetTriggerData(); err == nil && triggerData != nil {
		data["SourceType"] = triggerData.SourceType
		data["SourceID"] = triggerData.SourceID
		data["MatchedValue"] = triggerData.MatchedValue
		data["Context"] = triggerData.Context

		// 如果是交易类型，添加交易链接
		if f.config.IncludeTransactionLinks && triggerData.SourceType == "transaction" {
			if txHash := triggerData.SourceID; txHash != "" {
				data["TransactionLink"] = f.getTransactionLink(txHash)
			} else {
				data["TransactionLink"] = ""
			}
		} else {
			data["TransactionLink"] = ""
		}
	} else {
		// 如果没有触发数据，设置默认值
		data["SourceType"] = ""
		data["SourceID"] = ""
		data["MatchedValue"] = ""
		data["Context"] = map[string]interface{}{}
		data["TransactionLink"] = ""
	}

	// 添加规则信息
	if alert.Rule.ID != 0 {
		data["RuleName"] = alert.Rule.Name
		data["Threshold"] = alert.Rule.Threshold
		data["Operator"] = alert.Rule.Operator
	} else {
		data["RuleName"] = "Unknown"
		data["Threshold"] = "N/A"
		data["Operator"] = "N/A"
	}

	// 确保所有必要字段都有默认值
	if data["Context"] == nil {
		data["Context"] = map[string]interface{}{}
	}

	return data
}

// prepareRuleStatusData 准备规则状态数据
func (f *MessageFormatter) prepareRuleStatusData(rule *models.AlertRule) map[string]interface{} {
	data := map[string]interface{}{
		"ID":           rule.ID,
		"Name":         rule.Name,
		"Description":  rule.Description,
		"Type":         rule.Type,
		"Severity":     rule.Severity,
		"Status":       rule.Status,
		"Threshold":    rule.Threshold,
		"Operator":     rule.Operator,
		"TriggerCount": rule.TriggerCount,
		"CreatedAt":    f.formatTime(rule.CreatedAt),
		"UpdatedAt":    f.formatTime(rule.UpdatedAt),
		"Emoji":        f.getRuleStatusEmoji(rule.Status),
	}

	if rule.LastTriggered != nil {
		data["LastTriggered"] = f.formatTime(*rule.LastTriggered)
	} else {
		data["LastTriggered"] = "Never"
	}

	if rule.LastChecked != nil {
		data["LastChecked"] = f.formatTime(*rule.LastChecked)
	} else {
		data["LastChecked"] = "Never"
	}

	return data
}

// applyTemplate 应用模板
func (f *MessageFormatter) applyTemplate(template string, data map[string]interface{}) (string, error) {
	result := template

	// 简单的模板替换（实际项目中可能会使用更复杂的模板引擎）
	for key, value := range data {
		placeholder := fmt.Sprintf("{{%s}}", key)
		replacement := fmt.Sprintf("%v", value)
		result = strings.ReplaceAll(result, placeholder, replacement)
	}

	return result, nil
}

// postProcessMessage 后处理消息
func (f *MessageFormatter) postProcessMessage(message string) string {
	// 移除多余的空行
	message = strings.ReplaceAll(message, "\n\n\n", "\n\n")
	
	// 如果禁用了表情符号，移除它们
	if !f.config.EnableEmojis {
		message = f.removeEmojis(message)
	}

	// 如果禁用了 Markdown，移除格式化字符
	if !f.config.EnableMarkdown {
		message = f.removeMarkdown(message)
	}

	// 检查消息长度
	if len(message) > f.config.MaxMessageLength {
		message = f.truncateMessage(message)
	}

	return strings.TrimSpace(message)
}

// canBatchAlerts 检查是否可以批量处理告警
func (f *MessageFormatter) canBatchAlerts(alerts []*models.Alert) bool {
	if len(alerts) <= 1 {
		return false
	}

	// 检查是否为相同类型和严重级别
	firstAlert := alerts[0]
	for _, alert := range alerts[1:] {
		if alert.Type != firstAlert.Type || alert.Severity != firstAlert.Severity {
			return false
		}
	}

	// 检查时间间隔是否在合理范围内（例如 5 分钟内）
	timeWindow := 5 * time.Minute
	for _, alert := range alerts {
		if time.Since(alert.TriggerTime) > timeWindow {
			return false
		}
	}

	return true
}

// formatBatchAlert 格式化批量告警
func (f *MessageFormatter) formatBatchAlert(alerts []*models.Alert) (string, error) {
	if len(alerts) == 0 {
		return "", fmt.Errorf("alerts list cannot be empty")
	}

	template := f.templates.GetBatchTemplate()
	firstAlert := alerts[0]

	data := map[string]interface{}{
		"Count":     len(alerts),
		"Type":      firstAlert.Type,
		"Severity":  firstAlert.Severity,
		"Emoji":     f.getAlertEmoji(firstAlert.Severity),
		"Timestamp": f.formatTime(time.Now()),
	}

	// 添加告警列表
	alertList := make([]string, 0, len(alerts))
	for i, alert := range alerts {
		alertInfo := fmt.Sprintf("%d. %s (Value: %v)", i+1, alert.Title, alert.TriggerValue)
		alertList = append(alertList, alertInfo)
	}
	data["AlertList"] = strings.Join(alertList, "\n")

	message, err := f.applyTemplate(template, data)
	if err != nil {
		return "", err
	}

	return f.postProcessMessage(message), nil
}

// 辅助方法

// formatTime 格式化时间
func (f *MessageFormatter) formatTime(t time.Time) string {
	if f.config.Timezone != "UTC" {
		if loc, err := time.LoadLocation(f.config.Timezone); err == nil {
			t = t.In(loc)
		}
	}
	return t.Format("2006-01-02 15:04:05 MST")
}

// getAlertEmoji 获取告警表情符号
func (f *MessageFormatter) getAlertEmoji(severity models.AlertSeverity) string {
	if !f.config.EnableEmojis {
		return ""
	}

	switch severity {
	case models.SeverityCritical:
		return "🚨"
	case models.SeverityHigh:
		return "⚠️"
	case models.SeverityMedium:
		return "⚡"
	case models.SeverityLow:
		return "ℹ️"
	default:
		return "📢"
	}
}

// getSeverityIcon 获取严重级别图标
func (f *MessageFormatter) getSeverityIcon(severity models.AlertSeverity) string {
	switch severity {
	case models.SeverityCritical:
		return "🔴"
	case models.SeverityHigh:
		return "🟠"
	case models.SeverityMedium:
		return "🟡"
	case models.SeverityLow:
		return "🟢"
	default:
		return "⚪"
	}
}

// getRuleStatusEmoji 获取规则状态表情符号
func (f *MessageFormatter) getRuleStatusEmoji(status models.AlertStatus) string {
	if !f.config.EnableEmojis {
		return ""
	}

	switch status {
	case models.AlertStatusActive:
		return "✅"
	case models.AlertStatusInactive:
		return "⏸️"
	case models.AlertStatusPaused:
		return "⏸️"
	default:
		return "❓"
	}
}

// getTransactionLink 获取交易链接
func (f *MessageFormatter) getTransactionLink(txHash string) string {
	if !f.config.IncludeTransactionLinks || txHash == "" {
		return ""
	}
	url := fmt.Sprintf("%s/tx/%s", f.config.EtherscanBaseURL, txHash)
	return fmt.Sprintf("🔗 **View on Etherscan:** %s", url)
}

// removeEmojis 移除表情符号（简化实现）
func (f *MessageFormatter) removeEmojis(text string) string {
	// 这里是简化的实现，实际项目中可能需要更复杂的正则表达式
	emojis := []string{"🚨", "⚠️", "⚡", "ℹ️", "📢", "🔴", "🟠", "🟡", "🟢", "⚪", "✅", "⏸️", "❓"}
	for _, emoji := range emojis {
		text = strings.ReplaceAll(text, emoji, "")
	}
	return text
}

// removeMarkdown 移除 Markdown 格式
func (f *MessageFormatter) removeMarkdown(text string) string {
	// 移除粗体
	text = strings.ReplaceAll(text, "**", "")
	text = strings.ReplaceAll(text, "*", "")
	// 移除代码块
	text = strings.ReplaceAll(text, "`", "")
	// 移除链接格式
	text = strings.ReplaceAll(text, "[", "")
	text = strings.ReplaceAll(text, "]", "")
	text = strings.ReplaceAll(text, "(", "")
	text = strings.ReplaceAll(text, ")", "")
	
	return text
}

// truncateMessage 截断消息
func (f *MessageFormatter) truncateMessage(message string) string {
	if len(message) <= f.config.MaxMessageLength {
		return message
	}

	// 保留空间给省略标记
	suffix := "\n\n... (message truncated)"
	maxLen := f.config.MaxMessageLength - len(suffix)
	if maxLen <= 0 {
		maxLen = f.config.MaxMessageLength / 2
		if maxLen <= 0 {
			maxLen = 50
		}
		suffix = "..."
		maxLen = f.config.MaxMessageLength - len(suffix)
		if maxLen <= 0 {
			maxLen = 20 // 最小可用长度
			suffix = ""
		}
	}
	if maxLen >= len(message) || maxLen <= 0 {
		return message // 不需要截断或无法截断
	}
	truncated := message[:maxLen]
	
	// 尝试在单词边界截断
	if lastSpace := strings.LastIndex(truncated, " "); lastSpace > maxLen-100 && lastSpace > 0 {
		truncated = truncated[:lastSpace]
	}
	
	return truncated + suffix
}
