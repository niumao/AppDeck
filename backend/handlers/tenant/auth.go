package tenant

import (
	"appdeck/config"
	"appdeck/services"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

type AuthHandler struct {
	tenantService *services.TenantService
	cfg           *config.Config
}

func NewAuthHandler(cfg *config.Config) *AuthHandler {
	return &AuthHandler{
		tenantService: services.GetTenantService(),
		cfg:           cfg,
	}
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "username and password are required",
			"data":    nil,
		})
		return
	}

	tenant, err := h.tenantService.FindByUsername(req.Username)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    401,
			"message": "Invalid username or password",
			"data":    nil,
		})
		return
	}

	if !h.tenantService.VerifyPassword(tenant, req.Password) {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    401,
			"message": "Invalid username or password",
			"data":    nil,
		})
		return
	}

	if !tenant.IsActive() {
		c.JSON(http.StatusForbidden, gin.H{
			"code":    403,
			"message": "Account is disabled",
			"data":    nil,
		})
		return
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"tenant_id": tenant.ID,
		"username":  tenant.Username,
		"type":      "tenant",
		"exp":       time.Now().Add(24 * time.Hour).Unix(),
	})

	tokenString, err := token.SignedString([]byte(h.cfg.JWT.Secret))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to generate token",
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"token": tokenString,
			"tenant": gin.H{
				"id":           tenant.ID,
				"username":     tenant.Username,
				"display_name": tenant.DisplayName,
			},
		},
	})
}

func (h *AuthHandler) GetMe(c *gin.Context) {
	tenantID := c.GetString("tenant_id")
	tenant, err := h.tenantService.FindByID(tenantID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "Tenant not found",
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    tenant,
	})
}
