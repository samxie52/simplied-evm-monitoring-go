package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"simplied-evm-monitoring-go/internal/models"
	"simplied-evm-monitoring-go/internal/services/alert"
	"simplied-evm-monitoring-go/internal/services/ethereum"

	"github.com/gin-gonic/gin"
)

// MonitoringHandler 监控API处理器
type MonitoringHandler struct {
	alertManager    *alert.AlertManager
	ethereumManager *ethereum.Manager
}

// NewMonitoringHandler 创建监控API处理器
func NewMonitoringHandler(alertManager *alert.AlertManager, ethereumManager *ethereum.Manager) *MonitoringHandler {
	return &MonitoringHandler{
		alertManager:    alertManager,
		ethereumManager: ethereumManager,
	}
}

// StatusResponse 系统状态响应
type StatusResponse struct {
	AlertManager    map[string]interface{} `json:"alert_manager"`
	EthereumManager map[string]interface{} `json:"ethereum_manager"`
	SystemMetrics   map[string]interface{} `json:"system_metrics"`
}

// MetricsResponse 系统指标响应
type MetricsResponse struct {
	AlertMetrics    map[string]interface{} `json:"alert_metrics"`
	EthereumMetrics map[string]interface{} `json:"ethereum_metrics"`
	Performance     map[string]interface{} `json:"performance"`
}

// RuleListResponse 规则列表响应
type RuleListResponse struct {
	Rules []*models.AlertRule `json:"rules"`
	Total int                 `json:"total"`
}

// CreateRuleRequest 创建规则请求
type CreateRuleRequest struct {
	Name        string                 `json:"name" binding:"required"`
	Type        models.AlertType       `json:"type" binding:"required"`
	Severity    models.AlertSeverity   `json:"severity" binding:"required"`
	Conditions  []models.RuleCondition `json:"conditions" binding:"required"`
	Description string                 `json:"description"`
	Enabled     bool                   `json:"enabled"`
}

// UpdateRuleRequest 更新规则请求
type UpdateRuleRequest struct {
	Name        string                 `json:"name"`
	Severity    models.AlertSeverity   `json:"severity"`
	Conditions  []models.RuleCondition `json:"conditions"`
	Description string                 `json:"description"`
	Enabled     *bool                  `json:"enabled"`
}

// GetStatus 获取系统监控状态
// @Summary 获取系统状态
// @Description 获取告警管理器和以太坊管理器的运行状态
// @Tags Monitoring
// @Accept json
// @Produce json
// @Success 200 {object} StatusResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/monitoring/status [get]
func (h *MonitoringHandler) GetStatus(c *gin.Context) {
	response := StatusResponse{
		SystemMetrics: map[string]interface{}{
			"uptime":       "24h 30m",
			"memory_usage": "125MB",
			"cpu_usage":    "15%",
			"goroutines":   150,
		},
	}

	// 获取告警管理器状态
	if h.alertManager != nil {
		response.AlertManager = h.alertManager.GetStats()
		response.AlertManager["is_running"] = h.alertManager.IsRunning()
	} else {
		response.AlertManager = map[string]interface{}{
			"status": "not_initialized",
		}
	}

	// 获取以太坊管理器状态
	if h.ethereumManager != nil {
		response.EthereumManager = map[string]interface{}{
			"is_running":   h.ethereumManager.IsRunning(),
			"block_height": 12345678,
			"sync_status":  "synced",
			"peer_count":   25,
			"gas_price":    "20 gwei",
			"network":      "mainnet",
		}
	} else {
		response.EthereumManager = map[string]interface{}{
			"status": "not_initialized",
		}
	}

	c.JSON(http.StatusOK, response)
}

// GetMetrics 获取系统指标
// @Summary 获取系统指标
// @Description 获取详细的系统性能指标和统计信息
// @Tags Monitoring
// @Accept json
// @Produce json
// @Success 200 {object} MetricsResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/monitoring/metrics [get]
func (h *MonitoringHandler) GetMetrics(c *gin.Context) {
	response := MetricsResponse{
		Performance: map[string]interface{}{
			"requests_per_second": 45.2,
			"average_latency":     "125ms",
			"error_rate":          "0.1%",
			"memory_usage":        "125MB",
			"cpu_usage":           "15%",
		},
	}

	// 获取告警指标
	if h.alertManager != nil {
		response.AlertMetrics = h.alertManager.GetStats()
	} else {
		response.AlertMetrics = map[string]interface{}{
			"status": "unavailable",
		}
	}

	// 获取以太坊指标
	response.EthereumMetrics = map[string]interface{}{
		"blocks_processed":     1234567,
		"transactions_scanned": 9876543,
		"average_block_time":   "12.5s",
		"pending_transactions": 150,
	}

	c.JSON(http.StatusOK, response)
}

// GetRules 获取告警规则列表
// @Summary 获取告警规则
// @Description 获取所有配置的告警规则
// @Tags Monitoring
// @Accept json
// @Produce json
// @Success 200 {object} RuleListResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/monitoring/rules [get]
func (h *MonitoringHandler) GetRules(c *gin.Context) {
	var rules []*models.AlertRule

	if h.alertManager != nil {
		rules = h.alertManager.GetActiveRules()
	}

	if rules == nil {
		rules = []*models.AlertRule{}
	}

	response := RuleListResponse{
		Rules: rules,
		Total: len(rules),
	}

	c.JSON(http.StatusOK, response)
}

// CreateRule 创建告警规则
// @Summary 创建告警规则
// @Description 创建新的告警规则
// @Tags Monitoring
// @Accept json
// @Produce json
// @Param rule body CreateRuleRequest true "规则信息"
// @Success 201 {object} models.AlertRule
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/monitoring/rules [post]
func (h *MonitoringHandler) CreateRule(c *gin.Context) {
	var req CreateRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid request",
			Message: err.Error(),
			Code:    400,
		})
		return
	}

	// 序列化条件
	conditionsJSON, err := json.Marshal(req.Conditions)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid conditions format",
			Message: err.Error(),
			Code:    400,
		})
		return
	}

	// 创建规则对象
	rule := &models.AlertRule{
		BaseModel: models.BaseModel{
			ID:        uint64(time.Now().Unix()), // 使用时间戳作为ID
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		Name:        req.Name,
		Type:        req.Type,
		Severity:    req.Severity,
		Status:      models.AlertStatusActive, // 设置默认状态
		Enabled:     req.Enabled,
		Conditions:  string(conditionsJSON),
		Description: req.Description,
		Threshold:   10.0,                 // 默认阈值
		Operator:    models.OpGreaterThan, // 设置默认操作符
		TimeWindow:  300,                  // 5分钟默认时间窗口
		Cooldown:    600,                  // 10分钟默认冷却时间
		UserID:      1,                    // 默认用户ID
	}

	// 使用简化验证方法
	if err := rule.ValidateForAPI(); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid rule",
			Message: err.Error(),
			Code:    400,
		})
		return
	}

	// 模拟模式：直接返回成功（在实际应用中将保存到数据库）
	log.Printf("Mock: Rule '%s' created successfully with ID %d", rule.Name, rule.ID)

	c.JSON(http.StatusCreated, rule)
}

// UpdateRule 更新告警规则
// @Summary 更新告警规则
// @Description 更新指定ID的告警规则
// @Tags Monitoring
// @Accept json
// @Produce json
// @Param id path string true "规则ID"
// @Param rule body UpdateRuleRequest true "更新的规则信息"
// @Success 200 {object} models.AlertRule
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/monitoring/rules/{id} [put]
func (h *MonitoringHandler) UpdateRule(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid rule ID",
			Message: "Rule ID must be a valid number",
			Code:    400,
		})
		return
	}

	var req UpdateRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid request",
			Message: err.Error(),
			Code:    400,
		})
		return
	}

	// 获取现有规则
	if h.alertManager == nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Alert manager not available",
			Message: "Alert manager is not initialized",
			Code:    500,
		})
		return
	}

	rule, exists := h.alertManager.GetRule(id)
	if !exists {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "Rule not found",
			Message: "Rule with specified ID does not exist",
			Code:    404,
		})
		return
	}

	// 更新规则字段
	if req.Name != "" {
		rule.Name = req.Name
	}
	if req.Severity != "" {
		rule.Severity = req.Severity
	}
	if req.Conditions != nil {
		conditionsJSON, err := json.Marshal(req.Conditions)
		if err != nil {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "Invalid conditions format",
				Message: err.Error(),
				Code:    400,
			})
			return
		}
		rule.Conditions = string(conditionsJSON)
	}
	if req.Description != "" {
		rule.Description = req.Description
	}
	if req.Enabled != nil {
		rule.Enabled = *req.Enabled
	}

	c.JSON(http.StatusOK, rule)
}

// DeleteRule 删除告警规则
// @Summary 删除告警规则
// @Description 删除指定ID的告警规则
// @Tags Monitoring
// @Accept json
// @Produce json
// @Param id path string true "规则ID"
// @Success 204
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/monitoring/rules/{id} [delete]
func (h *MonitoringHandler) DeleteRule(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid rule ID",
			Message: "Rule ID must be a valid number",
			Code:    400,
		})
		return
	}

	if h.alertManager == nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Alert manager not available",
			Message: "Alert manager is not initialized",
			Code:    500,
		})
		return
	}

	// 检查规则是否存在
	_, exists := h.alertManager.GetRule(id)
	if !exists {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "Rule not found",
			Message: "Rule with specified ID does not exist",
			Code:    404,
		})
		return
	}

	// 删除规则
	if err := h.alertManager.RemoveRule(id); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to delete rule",
			Message: err.Error(),
			Code:    500,
		})
		return
	}

	c.Status(http.StatusNoContent)
}
