package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// Auth 认证中间件
func Auth(token string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 如果没有设置token，跳过认证
		if token == "" {
			c.Next()
			return
		}

		// 从Header获取Authorization
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "Unauthorized",
				"message": "Missing authorization header",
			})
			c.Abort()
			return
		}

		// 检查Bearer token格式
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "Unauthorized",
				"message": "Invalid authorization header format",
			})
			c.Abort()
			return
		}

		// 验证token
		if parts[1] != token {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "Unauthorized",
				"message": "Invalid token",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
