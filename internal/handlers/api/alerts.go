package api

import (
	"net/http"
	"strconv"
	"time"

	"simplied-evm-monitoring-go/internal/models"
	"simplied-evm-monitoring-go/internal/services/alert"

	"github.com/gin-gonic/gin"
)

// AlertHandler 告警API处理器
type AlertHandler struct {
	alertManager *alert.AlertManager
}

// NewAlertHandler 创建告警API处理器
func NewAlertHandler(alertManager *alert.AlertManager) *AlertHandler {
	return &AlertHandler{
		alertManager: alertManager,
	}
}

// AlertListResponse 告警列表响应
type AlertListResponse struct {
	Alerts     []*models.Alert `json:"alerts"`
	Total      int             `json:"total"`
	Page       int             `json:"page"`
	PageSize   int             `json:"page_size"`
	TotalPages int             `json:"total_pages"`
}

// AlertStatsResponse 告警统计响应
type AlertStatsResponse struct {
	Total       int64                  `json:"total"`
	BySeverity  map[string]int64       `json:"by_severity"`
	ByType      map[string]int64       `json:"by_type"`
	ByStatus    map[string]int64       `json:"by_status"`
	Recent24h   int64                  `json:"recent_24h"`
	Performance map[string]interface{} `json:"performance"`
}

// ErrorResponse 错误响应
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Code    int    `json:"code"`
}

// ListAlerts 获取告警列表
// @Summary 获取告警列表
// @Description 分页获取告警历史记录
// @Tags Alerts
// @Accept json
// @Produce json
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页大小" default(20)
// @Param severity query string false "告警级别过滤"
// @Param type query string false "告警类型过滤"
// @Param start_time query string false "开始时间 (RFC3339格式)"
// @Param end_time query string false "结束时间 (RFC3339格式)"
// @Success 200 {object} AlertListResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/alerts [get]
func (h *AlertHandler) ListAlerts(c *gin.Context) {
	// 解析查询参数
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	severity := c.Query("severity")
	alertType := c.Query("type")
	startTimeStr := c.Query("start_time")
	endTimeStr := c.Query("end_time")

	// 验证分页参数
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	// 解析时间参数
	var startTime, endTime *time.Time
	if startTimeStr != "" {
		if t, err := time.Parse(time.RFC3339, startTimeStr); err == nil {
			startTime = &t
		} else {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "Invalid start_time format",
				Message: "start_time must be in RFC3339 format",
				Code:    400,
			})
			return
		}
	}
	if endTimeStr != "" {
		if t, err := time.Parse(time.RFC3339, endTimeStr); err == nil {
			endTime = &t
		} else {
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "Invalid end_time format",
				Message: "end_time must be in RFC3339 format",
				Code:    400,
			})
			return
		}
	}

	// 构建查询参数
	queryParams := &models.AlertQueryParams{
		PaginationParams: models.PaginationParams{
			Page:     page,
			PageSize: pageSize,
		},
		FilterParams: models.FilterParams{
			StartTime: startTime,
			EndTime:   endTime,
		},
	}

	// 使用模拟数据生成器
	allAlerts := h.generateMockAlerts(queryParams, severity, alertType, startTime, endTime)
	total := len(allAlerts)
	totalPages := (total + pageSize - 1) / pageSize

	// 分页处理
	start := (page - 1) * pageSize
	end := start + pageSize
	if end > total {
		end = total
	}
	if start > total {
		start = total
		allAlerts = []*models.Alert{}
	} else {
		allAlerts = allAlerts[start:end]
	}

	response := AlertListResponse{
		Alerts:     allAlerts,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}

	c.JSON(http.StatusOK, response)
}

// GetAlert 获取单个告警详情
// @Summary 获取告警详情
// @Description 根据ID获取告警详细信息
// @Tags Alerts
// @Accept json
// @Produce json
// @Param id path string true "告警ID"
// @Success 200 {object} models.Alert
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/v1/alerts/{id} [get]
func (h *AlertHandler) GetAlert(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid alert ID",
			Message: "Alert ID must be a valid number",
			Code:    400,
		})
		return
	}

	// 模拟获取告警（实际应该从数据库获取）
	alert := h.generateMockAlert(id)
	if alert == nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "Alert not found",
			Message: "Alert with specified ID does not exist",
			Code:    404,
		})
		return
	}

	c.JSON(http.StatusOK, alert)
}

// GetAlertStats 获取告警统计信息
// @Summary 获取告警统计
// @Description 获取告警的统计信息和性能指标
// @Tags Alerts
// @Accept json
// @Produce json
// @Success 200 {object} AlertStatsResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/alerts/stats [get]
func (h *AlertHandler) GetAlertStats(c *gin.Context) {
	// 获取告警管理器统计
	stats := h.alertManager.GetStats()

	// 构建响应
	response := AlertStatsResponse{
		Total: 150, // 模拟数据
		BySeverity: map[string]int64{
			"critical": 5,
			"high":     25,
			"medium":   80,
			"low":      40,
		},
		ByType: map[string]int64{
			"large_transfer":     60,
			"gas_price":          30,
			"network_congestion": 35,
			"contract_event":     25,
		},
		ByStatus: map[string]int64{
			"sent":      120,
			"failed":    15,
			"pending":   10,
			"duplicate": 5,
		},
		Recent24h:   45,
		Performance: stats,
	}

	c.JSON(http.StatusOK, response)
}

// generateMockAlerts 生成模拟告警数据
func (h *AlertHandler) generateMockAlerts(params *models.AlertQueryParams, severity, alertType string, startTime, endTime *time.Time) []*models.Alert {
	alerts := make([]*models.Alert, 0)

	// 生成一些模拟数据
	for i := 1; i <= 50; i++ {
		alert := &models.Alert{
			BaseModel: models.BaseModel{
				ID:        uint64(i),
				CreatedAt: time.Now().Add(-time.Duration(i) * time.Hour),
				UpdatedAt: time.Now().Add(-time.Duration(i) * time.Hour),
			},
			Type:     models.AlertTypeLargeTransfer,
			Severity: models.SeverityHigh,
			Title:    "Large Transfer Detected",
			Message:  "Large transfer of 15.5 ETH detected",
			TriggerData: `{
				"transaction_hash": "0x123...",
				"from_address": "0xabc...",
				"to_address": "0xdef...",
				"amount": "15500000000000000000"
			}`,
			Status: models.NotificationStatusSent,
		}

		// 应用过滤条件
		if severity != "" && string(alert.Severity) != severity {
			continue
		}
		if alertType != "" && string(alert.Type) != alertType {
			continue
		}
		if startTime != nil && alert.CreatedAt.Before(*startTime) {
			continue
		}
		if endTime != nil && alert.CreatedAt.After(*endTime) {
			continue
		}

		alerts = append(alerts, alert)
	}

	return alerts
}

// generateMockAlert 生成单个模拟告警
func (h *AlertHandler) generateMockAlert(id uint64) *models.Alert {
	if id > 50 {
		return nil
	}

	return &models.Alert{
		BaseModel: models.BaseModel{
			ID:        id,
			CreatedAt: time.Now().Add(-time.Duration(id) * time.Hour),
			UpdatedAt: time.Now().Add(-time.Duration(id) * time.Hour),
		},
		Type:     models.AlertTypeLargeTransfer,
		Severity: models.SeverityHigh,
		Title:    "Large Transfer Detected",
		Message:  "Large transfer of 15.5 ETH detected",
		TriggerData: `{
			"transaction_hash": "0x123...",
			"from_address": "0xabc...",
			"to_address": "0xdef...",
			"amount": "15500000000000000000"
		}`,
		Status: models.NotificationStatusSent,
	}
}
