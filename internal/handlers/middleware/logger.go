package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// Logger 日志中间件
func Logger(logger *logrus.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 记录开始时间
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		// 处理请求
		c.Next()

		// 计算延迟
		latency := time.Since(start)

		// 获取客户端IP
		clientIP := c.ClientIP()

		// 获取请求方法
		method := c.Request.Method

		// 获取状态码
		statusCode := c.Writer.Status()

		// 构建完整路径
		if raw != "" {
			path = path + "?" + raw
		}

		// 记录日志
		entry := logger.WithFields(logrus.Fields{
			"status":     statusCode,
			"latency":    latency,
			"client_ip":  clientIP,
			"method":     method,
			"path":       path,
			"user_agent": c.Request.UserAgent(),
		})

		if len(c.Errors) > 0 {
			// 如果有错误，记录错误日志
			entry.Error(c.Errors.String())
		} else {
			// 根据状态码选择日志级别
			switch {
			case statusCode >= 500:
				entry.Error("Server error")
			case statusCode >= 400:
				entry.Warn("Client error")
			default:
				entry.Info("Request completed")
			}
		}
	}
}
