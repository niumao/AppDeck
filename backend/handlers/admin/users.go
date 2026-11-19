package admin

import (
	"net/http"
	"strconv"
	"time"

	"appdeck/config"
	"appdeck/database"
	"appdeck/models"
	"appdeck/services"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// UsersHandler 用户管理处理器
type UsersHandler struct {
	userService     *services.UserService
	balanceService  *services.BalanceService
	rechargeService *services.RechargeService
}

// NewUsersHandler 创建用户管理处理器
func NewUsersHandler(cfg *config.Config) *UsersHandler {
	return &UsersHandler{
		userService:     services.NewUserService(),
		balanceService:  services.NewBalanceService(),
		rechargeService: services.NewRechargeService(cfg),
	}
}

// TenantBalanceInfo represents tenant balance information for a user
type TenantBalanceInfo struct {
	TenantID       string `json:"tenant_id"`
	TenantName     string `json:"tenant_name"`
	BalanceMinutes int    `json:"balance_minutes"`
}

// UserWithTenantBalance represents a user with their tenant balances
type UserWithTenantBalance struct {
	models.User
	TenantBalances []TenantBalanceInfo `json:"tenant_balances"`
}

// GetAll 获取用户列表
// GET /api/v1/admin/users
func (h *UsersHandler) GetAll(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limitStr := c.DefaultQuery("limit", "")
	if limitStr == "" {
		limitStr = c.DefaultQuery("pageSize", "20")
	}
	limit, _ := strconv.Atoi(limitStr)
	search := c.Query("search")
	if search == "" {
		search = c.Query("keyword")
	}

	var users interface{}
	var total int64
	var err error

	if search != "" {
		users, total, err = h.userService.Search(search, page, limit)
	} else {
		users, total, err = h.userService.GetAll(page, limit)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取用户列表失败",
			"data":    nil,
		})
		return
	}

	// Enrich users with tenant balances
	enrichedUsers := h.enrichUsersWithTenantBalances(users)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"users": enrichedUsers,
			"total": total,
			"page":  page,
			"limit": limit,
		},
	})
}

// enrichUsersWithTenantBalances adds tenant balance information to users
func (h *UsersHandler) enrichUsersWithTenantBalances(users interface{}) []UserWithTenantBalance {
	// Convert users to []models.User
	var userList []models.User
	switch v := users.(type) {
	case []models.User:
		userList = v
	case []*models.User:
		for _, u := range v {
			if u != nil {
				userList = append(userList, *u)
			}
		}
	default:
		// If we can't convert, return empty slice
		return []UserWithTenantBalance{}
	}

	if len(userList) == 0 {
		return []UserWithTenantBalance{}
	}

	// Collect user IDs
	userIDs := make([]string, 0, len(userList))
	for _, user := range userList {
		userIDs = append(userIDs, user.ID)
	}

	// Query user_tenant_balances for all user IDs with Tenant preloaded
	var tenantBalances []models.UserTenantBalance
	if err := database.DB.Where("user_id IN ? AND balance_minutes > 0", userIDs).
		Preload("Tenant").
		Find(&tenantBalances).Error; err != nil {
		// On error, return users without tenant balances
		result := make([]UserWithTenantBalance, len(userList))
		for i, user := range userList {
			result[i] = UserWithTenantBalance{
				User:           user,
				TenantBalances: []TenantBalanceInfo{},
			}
		}
		return result
	}

	// Group tenant balances by user ID
	balancesByUserID := make(map[string][]TenantBalanceInfo)
	for _, tb := range tenantBalances {
		tenantName := ""
		if tb.Tenant != nil {
			tenantName = tb.Tenant.DisplayName
		}

		balanceInfo := TenantBalanceInfo{
			TenantID:       tb.TenantID,
			TenantName:     tenantName,
			BalanceMinutes: tb.BalanceMinutes,
		}

		balancesByUserID[tb.UserID] = append(balancesByUserID[tb.UserID], balanceInfo)
	}

	// Build enriched response
	result := make([]UserWithTenantBalance, len(userList))
	for i, user := range userList {
		tenantBalances, exists := balancesByUserID[user.ID]
		if !exists {
			tenantBalances = []TenantBalanceInfo{}
		}

		result[i] = UserWithTenantBalance{
			User:           user,
			TenantBalances: tenantBalances,
		}
	}

	return result
}

// GetByID 获取用户详情
// GET /api/v1/admin/users/:id
func (h *UsersHandler) GetByID(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "缺少用户ID",
		})
		return
	}

	user, err := h.userService.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "用户不存在",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    user,
	})
}

// AdjustBalanceRequest 调整余额请求
type AdjustBalanceRequest struct {
	AdjustmentType string `json:"adjustment_type" binding:"required,oneof=add deduct"`
	AmountCents    int    `json:"amount_cents" binding:"required,min=1"`
	BalanceType    string `json:"balance_type" binding:"omitempty,oneof=real bonus"` // real=实际余额, bonus=赠送余额, 空=实际余额（向后兼容）
	Remark         string `json:"remark"`
}

// AdjustBalance 调整用户余额
// POST /api/v1/admin/users/:id/balance
func (h *UsersHandler) AdjustBalance(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "缺少用户ID",
		})
		return
	}

	var req AdjustBalanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "参数错误",
			"error":   err.Error(),
		})
		return
	}

	// 检查用户是否存在
	user, err := h.userService.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "用户不存在",
			"error":   err.Error(),
		})
		return
	}

	// 调整余额
	amountCents := req.AmountCents
	if req.AdjustmentType == "deduct" {
		amountCents = -amountCents
	}

	// 默认调整实际余额（向后兼容）
	balanceType := req.BalanceType
	if balanceType == "" {
		balanceType = "real"
	}

	remark := req.Remark
	if remark == "" {
		balanceTypeLabel := "实际余额"
		if balanceType == "bonus" {
			balanceTypeLabel = "赠送余额"
		}
		if req.AdjustmentType == "add" {
			remark = "管理员增加" + balanceTypeLabel
		} else {
			remark = "管理员扣减" + balanceTypeLabel
		}
	}

	if err := h.balanceService.AdminAdjustBalanceCentsWithType(id, amountCents, balanceType, "", remark); err != nil {
		// Check if it's an insufficient balance error
		if err.Error() == "余额不足" {
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    400,
				"message": "余额不足，无法扣减",
				"error":   err.Error(),
			})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{
				"code":    500,
				"message": "调整余额失败",
				"error":   err.Error(),
			})
		}
		return
	}

	// Push balance_correction to active devices so WebView resets countdown
	go h.rechargeService.PushBalanceUpdateToActiveDevices(id)

	// 获取更新后的用户信息
	user, _ = h.userService.GetByID(id)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "调整成功",
		"data": gin.H{
			"balance_cents":       user.BalanceCents,
			"bonus_balance_cents": user.BonusBalanceCents,
		},
	})
}

// UpdateStatus 更新用户状态
// PUT /api/v1/admin/users/:id/status
func (h *UsersHandler) UpdateStatus(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "缺少用户ID",
			"data":    nil,
		})
		return
	}

	var req struct {
		Status *int `json:"status" binding:"required,oneof=0 1"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "参数错误",
			"data":    nil,
		})
		return
	}

	if err := h.userService.UpdateStatus(id, *req.Status); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "更新状态失败",
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    gin.H{"status": *req.Status},
	})
}

// GetBalanceLogs 获取用户余额日志
// GET /api/v1/admin/users/:id/balance-logs
func (h *UsersHandler) GetBalanceLogs(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "缺少用户ID",
			"data":    nil,
		})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	logs, total, err := h.balanceService.GetBalanceLogs(id, page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取日志失败",
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"items": logs,
			"total": total,
		},
	})
}

// AdjustTenantBalanceRequest 调整租户余额请求
type AdjustTenantBalanceRequest struct {
	TenantID string `json:"tenant_id" binding:"required"`
	Minutes  int    `json:"minutes" binding:"required"`
	Remark   string `json:"remark"`
}

// AdjustTenantBalance 调整用户租户余额
// POST /api/v1/admin/users/:id/tenant-balance
func (h *UsersHandler) AdjustTenantBalance(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "缺少用户ID",
		})
		return
	}

	var req AdjustTenantBalanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "参数错误",
			"error":   err.Error(),
		})
		return
	}

	// 检查用户是否存在
	_, err := h.userService.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "用户不存在",
			"error":   err.Error(),
		})
		return
	}

	// 检查 minutes 是否为 0
	if req.Minutes == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "调整时长不能为 0",
		})
		return
	}

	// 开启事务
	tx := database.DB.Begin()
	if tx.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "事务开启失败",
			"error":   tx.Error.Error(),
		})
		return
	}

	tenantBalanceService := services.NewTenantBalanceService()

	// 根据 minutes 正负调整余额
	if req.Minutes > 0 {
		// 增加余额
		if err := tenantBalanceService.AddMinutes(tx, id, req.TenantID, req.Minutes, ""); err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{
				"code":    500,
				"message": "增加租户余额失败",
				"error":   err.Error(),
			})
			return
		}
	} else {
		// 扣减余额 (RemoveMinutes 需要正数)
		if err := tenantBalanceService.RemoveMinutes(tx, id, req.TenantID, -req.Minutes); err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{
				"code":    500,
				"message": "扣减租户余额失败",
				"error":   err.Error(),
			})
			return
		}
	}

	// 创建余额日志
	remark := req.Remark
	if remark == "" {
		remark = "管理员调整租户时间余额"
	}

	balanceLog := models.BalanceLog{
		ID:           uuid.New().String(),
		UserID:       id,
		Type:         "admin_adjust",
		Scope:        "tenant",
		TenantID:     &req.TenantID,
		DeltaMinutes: req.Minutes,
		Remark:       remark,
		CreatedAt:    time.Now(),
	}

	if err := tx.Create(&balanceLog).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "创建余额日志失败",
			"error":   err.Error(),
		})
		return
	}

	// 提交事务
	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "事务提交失败",
			"error":   err.Error(),
		})
		return
	}

	// Push balance_correction to active devices so WebView resets countdown
	go h.rechargeService.PushBalanceUpdateToActiveDevices(id)

	// 获取调整后的余额
	newBalance, err := tenantBalanceService.GetBalance(id, req.TenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取余额失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "调整成功",
		"data": gin.H{
			"balance_minutes": newBalance,
		},
	})
}
