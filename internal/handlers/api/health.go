package api

import (
	"net/http"
	"time"

	"simplied-evm-monitoring-go/internal/services/alert"
	"simplied-evm-monitoring-go/internal/services/ethereum"

	"github.com/gin-gonic/gin"
)

// HealthHandler 健康检查处理器
type HealthHandler struct {
	alertManager    *alert.AlertManager
	ethereumManager *ethereum.Manager
}

// NewHealthHandler 创建健康检查处理器
func NewHealthHandler(alertManager *alert.AlertManager, ethereumManager *ethereum.Manager) *HealthHandler {
	return &HealthHandler{
		alertManager:    alertManager,
		ethereumManager: ethereumManager,
	}
}

// HealthResponse 健康检查响应
type HealthResponse struct {
	Status    string                 `json:"status"`
	Timestamp time.Time              `json:"timestamp"`
	Version   string                 `json:"version"`
	Services  map[string]ServiceInfo `json:"services"`
	Uptime    time.Duration          `json:"uptime"`
}

// ServiceInfo 服务信息
type ServiceInfo struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

var startTime = time.Now()

// HealthCheck 健康检查接口
// @Summary 系统健康检查
// @Description 检查系统各个组件的健康状态
// @Tags Health
// @Accept json
// @Produce json
// @Success 200 {object} HealthResponse
// @Failure 503 {object} ErrorResponse
// @Router /health [get]
func (h *HealthHandler) HealthCheck(c *gin.Context) {
	services := make(map[string]ServiceInfo)
	overallStatus := "healthy"

	// 检查告警管理器
	if h.alertManager != nil {
		if err := h.alertManager.HealthCheck(); err != nil {
			services["alert_manager"] = ServiceInfo{
				Status:  "unhealthy",
				Message: err.Error(),
			}
			overallStatus = "unhealthy"
		} else {
			services["alert_manager"] = ServiceInfo{
				Status: "healthy",
			}
		}
	} else {
		services["alert_manager"] = ServiceInfo{
			Status:  "unavailable",
			Message: "Alert manager not initialized",
		}
		overallStatus = "degraded"
	}

	// 检查以太坊管理器
	if h.ethereumManager != nil {
		if h.ethereumManager.IsRunning() {
			services["ethereum_manager"] = ServiceInfo{
				Status: "healthy",
			}
		} else {
			services["ethereum_manager"] = ServiceInfo{
				Status:  "unhealthy",
				Message: "Ethereum manager not running",
			}
			overallStatus = "unhealthy"
		}
	} else {
		services["ethereum_manager"] = ServiceInfo{
			Status:  "unavailable",
			Message: "Ethereum manager not initialized",
		}
		overallStatus = "degraded"
	}

	response := HealthResponse{
		Status:    overallStatus,
		Timestamp: time.Now(),
		Version:   "1.0.0",
		Services:  services,
		Uptime:    time.Since(startTime),
	}

	// 根据整体状态设置HTTP状态码
	statusCode := http.StatusOK
	if overallStatus == "unhealthy" {
		statusCode = http.StatusServiceUnavailable
	} else if overallStatus == "degraded" {
		statusCode = http.StatusPartialContent
	}

	c.JSON(statusCode, response)
}
