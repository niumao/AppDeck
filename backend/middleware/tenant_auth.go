package middleware

import (
	"appdeck/config"
	"appdeck/database"
	"appdeck/models"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func TenantAuthMiddleware(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
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
		if !ok || tokenType != "tenant" {
			c.JSON(http.StatusForbidden, gin.H{
				"code":    403,
				"message": "Tenant access required",
				"data":    nil,
			})
			c.Abort()
			return
		}

		tenantID, ok := claims["tenant_id"].(string)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": "Invalid tenant ID in token",
				"data":    nil,
			})
			c.Abort()
			return
		}

		var tenant models.Tenant
		if err := database.DB.Where("id = ?", tenantID).First(&tenant).Error; err != nil {
			c.JSON(http.StatusForbidden, gin.H{
				"code":    403,
				"message": "Account not found",
				"data":    nil,
			})
			c.Abort()
			return
		}

		if !tenant.IsActive() {
			c.JSON(http.StatusForbidden, gin.H{
				"code":    403,
				"message": "Account is disabled",
				"data":    nil,
			})
			c.Abort()
			return
		}

		c.Set("tenant_id", tenantID)
		c.Next()
	}
}
