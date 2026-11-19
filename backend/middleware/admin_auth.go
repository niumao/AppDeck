package middleware

import (
	"appdeck/config"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// AdminAuthMiddleware 管理员认证中间件
func AdminAuthMiddleware(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从 Header 获取 token
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": "Missing authorization header",
				"data":    nil,
			})
			c.Abort()
			return
		}

		// 移除 "Bearer " 前缀
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == authHeader {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": "Invalid authorization header format",
				"data":    nil,
			})
			c.Abort()
			return
		}

		// 解析 token
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			return []byte(cfg.JWT.Secret), nil
		})

		if err != nil || !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": "Invalid or expired token",
				"data":    nil,
			})
			c.Abort()
			return
		}

		// 检查 token 类型
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": "Invalid token claims",
				"data":    nil,
			})
			c.Abort()
			return
		}

		tokenType, ok := claims["type"].(string)
		if !ok || tokenType != "admin" {
			c.JSON(http.StatusForbidden, gin.H{
				"code":    403,
				"message": "Admin access required",
				"data":    nil,
			})
			c.Abort()
			return
		}

		// 保存管理员ID到上下文
		adminID, ok := claims["admin_id"].(string)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": "Invalid admin ID in token",
				"data":    nil,
			})
			c.Abort()
			return
		}

		c.Set("admin_id", adminID)
		c.Next()
	}
}
