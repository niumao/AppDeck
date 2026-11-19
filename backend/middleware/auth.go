package middleware

import (
	"net/http"
	"strings"

	"appdeck/config"
	"appdeck/services"

	"github.com/gin-gonic/gin"
)

// AuthMiddleware 认证中间件（必须登录）
func AuthMiddleware(cfg *config.Config) gin.HandlerFunc {
	authService := services.NewAuthService(cfg)

	return func(c *gin.Context) {
		// 从Header获取token
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": "未登录",
				"error":   "missing authorization header",
			})
			c.Abort()
			return
		}

		// 解析Bearer token
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": "无效的认证格式",
				"error":   "invalid authorization format",
			})
			c.Abort()
			return
		}

		tokenString := parts[1]

		// 验证token
		claims, err := authService.ValidateToken(tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": "登录已过期，请重新登录",
				"error":   err.Error(),
			})
			c.Abort()
			return
		}

		// 获取用户信息
		user, err := authService.GetUserFromToken(tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": "用户不存在",
				"error":   err.Error(),
			})
			c.Abort()
			return
		}

		// 检查用户状态
		if !user.IsActive() {
			c.JSON(http.StatusForbidden, gin.H{
				"code":    403,
				"message": "账号已被禁用",
				"error":   "user is disabled",
			})
			c.Abort()
			return
		}

		// 将用户信息存入上下文
		c.Set("userID", claims.UserID)
		c.Set("openID", claims.OpenID)
		c.Set("user", user)

		c.Next()
	}
}

// GetUserID 从上下文获取用户ID
func GetUserID(c *gin.Context) string {
	if userID, exists := c.Get("userID"); exists {
		return userID.(string)
	}
	return ""
}

// GetUser 从上下文获取用户
func GetUser(c *gin.Context) interface{} {
	if user, exists := c.Get("user"); exists {
		return user
	}
	return nil
}
