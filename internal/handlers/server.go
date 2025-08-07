package handlers

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"simplied-evm-monitoring-go/internal/handlers/api"
	"simplied-evm-monitoring-go/internal/handlers/middleware"
	"simplied-evm-monitoring-go/internal/services/alert"
	"simplied-evm-monitoring-go/internal/services/ethereum"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// ServerConfig HTTP服务器配置
type ServerConfig struct {
	Port            int           `json:"port"`
	ReadTimeout     time.Duration `json:"read_timeout"`
	WriteTimeout    time.Duration `json:"write_timeout"`
	ShutdownTimeout time.Duration `json:"shutdown_timeout"`
	EnableCORS      bool          `json:"enable_cors"`
	EnableAuth      bool          `json:"enable_auth"`
	AuthToken       string        `json:"auth_token"`
}

// DefaultServerConfig 默认服务器配置
func DefaultServerConfig() *ServerConfig {
	return &ServerConfig{
		Port:            8080,
		ReadTimeout:     30 * time.Second,
		WriteTimeout:    30 * time.Second,
		ShutdownTimeout: 10 * time.Second,
		EnableCORS:      true,
		EnableAuth:      false,
		AuthToken:       "",
	}
}

// Server HTTP服务器
type Server struct {
	config          *ServerConfig
	router          *gin.Engine
	httpServer      *http.Server
	alertManager    *alert.AlertManager
	ethereumManager *ethereum.Manager
	logger          *logrus.Logger
}

// NewServer 创建新的HTTP服务器
func NewServer(config *ServerConfig, alertManager *alert.AlertManager, ethereumManager *ethereum.Manager) *Server {
	if config == nil {
		config = DefaultServerConfig()
	}

	// 设置gin模式
	gin.SetMode(gin.ReleaseMode)

	server := &Server{
		config:          config,
		router:          gin.New(),
		alertManager:    alertManager,
		ethereumManager: ethereumManager,
		logger:          logrus.New(),
	}

	server.setupMiddleware()
	server.setupRoutes()

	return server
}

// setupMiddleware 设置中间件
func (s *Server) setupMiddleware() {
	// 基础中间件
	s.router.Use(gin.Recovery())
	s.router.Use(middleware.Logger(s.logger))

	// CORS中间件
	if s.config.EnableCORS {
		s.router.Use(middleware.CORS())
	}

	// 认证中间件
	if s.config.EnableAuth {
		s.router.Use(middleware.Auth(s.config.AuthToken))
	}
}

// setupRoutes 设置路由
func (s *Server) setupRoutes() {
	// 健康检查
	s.router.GET("/health", api.NewHealthHandler(s.alertManager, s.ethereumManager).HealthCheck)

	// API v1 路由组
	v1 := s.router.Group("/api/v1")
	{
		// 告警相关API
		alertHandler := api.NewAlertHandler(s.alertManager)
		v1.GET("/alerts", alertHandler.ListAlerts)
		v1.GET("/alerts/:id", alertHandler.GetAlert)
		v1.GET("/alerts/stats", alertHandler.GetAlertStats)

		// 监控相关API
		monitoringHandler := api.NewMonitoringHandler(s.alertManager, s.ethereumManager)
		v1.GET("/monitoring/status", monitoringHandler.GetStatus)
		v1.GET("/monitoring/metrics", monitoringHandler.GetMetrics)
		v1.GET("/monitoring/rules", monitoringHandler.GetRules)
		v1.POST("/monitoring/rules", monitoringHandler.CreateRule)
		v1.PUT("/monitoring/rules/:id", monitoringHandler.UpdateRule)
		v1.DELETE("/monitoring/rules/:id", monitoringHandler.DeleteRule)
	}

	// 静态文件服务（如果需要）
	s.router.Static("/static", "./web/static")
}

// Start 启动HTTP服务器
func (s *Server) Start() error {
	s.httpServer = &http.Server{
		Addr:         fmt.Sprintf(":%d", s.config.Port),
		Handler:      s.router,
		ReadTimeout:  s.config.ReadTimeout,
		WriteTimeout: s.config.WriteTimeout,
	}

	s.logger.WithField("port", s.config.Port).Info("Starting HTTP server")

	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.WithError(err).Error("HTTP server failed to start")
		}
	}()

	return nil
}

// Stop 停止HTTP服务器
func (s *Server) Stop() error {
	if s.httpServer == nil {
		return nil
	}

	s.logger.Info("Shutting down HTTP server")

	ctx, cancel := context.WithTimeout(context.Background(), s.config.ShutdownTimeout)
	defer cancel()

	if err := s.httpServer.Shutdown(ctx); err != nil {
		s.logger.WithError(err).Error("Failed to shutdown HTTP server gracefully")
		return err
	}

	s.logger.Info("HTTP server stopped successfully")
	return nil
}

// GetRouter 获取gin路由器（用于测试）
func (s *Server) GetRouter() *gin.Engine {
	return s.router
}

// IsRunning 检查服务器是否运行中
func (s *Server) IsRunning() bool {
	return s.httpServer != nil
}
