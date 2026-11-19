package admin

import (
	"appdeck/config"
	"appdeck/services"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

type AdminAuthHandler struct {
	adminService *services.AdminService
	cfg          *config.Config
}

func NewAdminAuthHandler(cfg *config.Config) *AdminAuthHandler {
	return &AdminAuthHandler{
		adminService: services.NewAdminService(),
		cfg:          cfg,
	}
}

// AdminLoginRequest 登录请求
type AdminLoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// Login 管理员登录 (ADM_LOGIN)
func (h *AdminAuthHandler) Login(c *gin.Context) {
	var req AdminLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Invalid request parameters",
			"data":    nil,
		})
		return
	}

	// 查找管理员
	admin, err := h.adminService.FindByUsername(req.Username)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    401,
			"message": "Invalid username or password",
			"data":    nil,
		})
		return
	}

	// 验证密码
	if !h.adminService.VerifyPassword(admin, req.Password) {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    401,
			"message": "Invalid username or password",
			"data":    nil,
		})
		return
	}

	// 检查状态
	if !admin.IsActive() {
		c.JSON(http.StatusForbidden, gin.H{
			"code":    403,
			"message": "Admin account is disabled",
			"data":    nil,
		})
		return
	}

	// 生成 JWT token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"admin_id": admin.ID,
		"username": admin.Username,
		"type":     "admin",
		"exp":      time.Now().Add(72 * time.Hour).Unix(),
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
			"admin": gin.H{
				"id":           admin.ID,
				"username":     admin.Username,
				"display_name": admin.DisplayName,
			},
		},
	})
}

// Logout 管理员登出 (ADM_LOGOUT)
func (h *AdminAuthHandler) Logout(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"success": true,
		},
	})
}

// GetMe 获取当前管理员信息 (ADM_ME)
func (h *AdminAuthHandler) GetMe(c *gin.Context) {
	// 从中间件获取管理员ID
	adminID, exists := c.Get("admin_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    401,
			"message": "Unauthorized",
			"data":    nil,
		})
		return
	}

	// 查找管理员
	admin, err := h.adminService.FindByID(adminID.(string))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "Admin not found",
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"id":           admin.ID,
			"username":     admin.Username,
			"display_name": admin.DisplayName,
		},
	})
}
