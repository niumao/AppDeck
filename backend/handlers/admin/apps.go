package admin

import (
	"net/http"
	"regexp"
	"strings"

	"appdeck/models"
	"appdeck/services"

	"github.com/gin-gonic/gin"
)

var titleIDPattern = regexp.MustCompile(`^[0-9A-Fa-f]{16}$`)

// dirToCoreMap maps ROM directory prefixes to RetroArch core names.
var dirToCoreMap = map[string]string{
	"FC":       "nestopia",
	"SFC":      "snes9x",
	"ARCADE":   "fbalpha2012_neogeo",
	"sega-md":  "genesis_plus_gx",
	"Sega-32X": "picodrive",
	"PCE":      "mednafen_pce_fast",
	"NGPC":     "mednafen_ngp",
	"NDS":      "melonds",
}

func detectCoreFromRom(romPath string) string {
	if romPath == "" {
		return ""
	}
	dir := romPath
	if i := strings.IndexByte(romPath, '/'); i >= 0 {
		dir = romPath[:i]
	}
	return dirToCoreMap[dir]
}

type AppsHandler struct {
	appService *services.AppService
}

func NewAppsHandler() *AppsHandler {
	return &AppsHandler{
		appService: services.NewAppService(),
	}
}

// GetAll 获取所有应用
// GET /api/v1/admin/apps
func (h *AppsHandler) GetAll(c *gin.Context) {
	apps, err := h.appService.GetAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取应用列表失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    apps,
	})
}

// GetByID 获取单个应用
// GET /api/v1/admin/apps/:id
func (h *AppsHandler) GetByID(c *gin.Context) {
	id := c.Param("id")
	app, err := h.appService.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "应用不存在",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    app,
	})
}

// Create 创建应用
// POST /api/v1/admin/apps
func (h *AppsHandler) Create(c *gin.Context) {
	var req struct {
		Name          string   `json:"name" binding:"required"`
		Description   string   `json:"description"`
		IconURL       string   `json:"iconUrl"`
		BigPicURL     string   `json:"bigPicUrl"`
		LaunchParam   string   `json:"launchParam"`
		LaunchType    string   `json:"launchType"`
		RetroarchCore string   `json:"retroarchCore"`
		RetroarchRom  string   `json:"retroarchRom"`
		TitleID       string   `json:"titleId"`
		CategoryIDs   []string `json:"categoryIds" binding:"required"`
		Order         *int     `json:"order"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请求参数错误",
			"error":   err.Error(),
		})
		return
	}

	// Default launch type
	launchType := req.LaunchType
	if launchType == "" {
		launchType = "moonlight"
	}

	// Validate based on launch type
	if launchType == "moonlight" && req.LaunchParam == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Moonlight apps require launch_param",
		})
		return
	}
	if launchType == "retroarch" && req.RetroarchRom == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "RetroArch apps require ROM",
		})
		return
	}
	if launchType == "eden" && req.LaunchParam == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Eden apps require launch_param",
		})
		return
	}
	if launchType == "android" && req.LaunchParam == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Android apps require package name in launch_param",
		})
		return
	}
	if launchType == "eden" && req.TitleID != "" && !titleIDPattern.MatchString(req.TitleID) {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Title ID must be a 16-character hex string",
		})
		return
	}

	app := &models.App{
		Name:          req.Name,
		Description:   req.Description,
		IconURL:       req.IconURL,
		BigPicURL:     req.BigPicURL,
		LaunchParam:   req.LaunchParam,
		LaunchType:    launchType,
		RetroarchCore: req.RetroarchCore,
		RetroarchRom:  req.RetroarchRom,
		TitleID:       req.TitleID,
	}

	if req.Order != nil {
		app.OrderNum = *req.Order
	} else {
		// Default order by launch type
		switch launchType {
		case "eden":
			app.OrderNum = 288
		case "retroarch":
			app.OrderNum = 388
		}
	}

	// Auto-detect RetroArch core from ROM path
	if launchType == "retroarch" && app.RetroarchCore == "" && app.RetroarchRom != "" {
		if detected := detectCoreFromRom(app.RetroarchRom); detected != "" {
			app.RetroarchCore = detected
		}
	}

	created, err := h.appService.Create(app, req.CategoryIDs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "创建应用失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "创建成功",
		"data":    created,
	})
}

// Update 更新应用
// PUT /api/v1/admin/apps/:id
func (h *AppsHandler) Update(c *gin.Context) {
	id := c.Param("id")

	var req struct {
		Name          *string   `json:"name"`
		Description   *string   `json:"description"`
		IconURL       *string   `json:"iconUrl"`
		BigPicURL     *string   `json:"bigPicUrl"`
		LaunchParam   *string   `json:"launchParam"`
		LaunchType    *string   `json:"launchType"`
		RetroarchCore *string   `json:"retroarchCore"`
		RetroarchRom  *string   `json:"retroarchRom"`
		TitleID       *string   `json:"titleId"`
		CategoryIDs   *[]string `json:"categoryIds"`
		Order         *int      `json:"order"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请求参数错误",
			"error":   err.Error(),
		})
		return
	}

	updates := make(map[string]interface{})
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.IconURL != nil {
		updates["icon_url"] = *req.IconURL
	}
	if req.BigPicURL != nil {
		updates["big_pic_url"] = *req.BigPicURL
	}
	if req.LaunchParam != nil {
		updates["launch_param"] = *req.LaunchParam
	}
	if req.LaunchType != nil {
		updates["launch_type"] = *req.LaunchType
	}
	if req.RetroarchCore != nil {
		updates["retroarch_core"] = *req.RetroarchCore
	}
	if req.RetroarchRom != nil {
		updates["retroarch_rom"] = *req.RetroarchRom
	}
	if req.TitleID != nil {
		updates["title_id"] = *req.TitleID
	}
	if req.Order != nil {
		updates["order_num"] = *req.Order
	}

	// Validate Title ID format when it's being set on an Eden app. LaunchType may not be
	// part of this request, so fall back to the app's current stored value.
	if req.TitleID != nil && *req.TitleID != "" {
		effectiveLaunchType := ""
		if req.LaunchType != nil {
			effectiveLaunchType = *req.LaunchType
		} else if existing, err := h.appService.GetByID(id); err == nil {
			effectiveLaunchType = existing.LaunchType
		}
		if effectiveLaunchType == "eden" && !titleIDPattern.MatchString(*req.TitleID) {
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    400,
				"message": "Title ID must be a 16-character hex string",
			})
			return
		}
	}

	// Auto-detect RetroArch core from ROM path if core is empty
	romVal, hasRom := updates["retroarch_rom"]
	coreVal, hasCore := updates["retroarch_core"]
	if hasRom {
		romStr, _ := romVal.(string)
		coreStr, _ := coreVal.(string)
		if romStr != "" && (!hasCore || coreStr == "") {
			if detected := detectCoreFromRom(romStr); detected != "" {
				updates["retroarch_core"] = detected
			}
		}
	}

	var categoryIDs []string
	if req.CategoryIDs != nil {
		categoryIDs = *req.CategoryIDs
	}

	updated, err := h.appService.Update(id, updates, categoryIDs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "更新应用失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "更新成功",
		"data":    updated,
	})
}

// Delete 删除应用
// DELETE /api/v1/admin/apps/:id
func (h *AppsHandler) Delete(c *gin.Context) {
	id := c.Param("id")

	if err := h.appService.Delete(id); err != nil {
		// Check if app was not found
		if err.Error() == "app not found" {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    404,
				"message": "应用不存在",
			})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "删除应用失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "删除成功",
		"data":    gin.H{"success": true},
	})
}

// Reorder 批量调整应用顺序
// PUT /api/v1/admin/apps/reorder
func (h *AppsHandler) Reorder(c *gin.Context) {
	var req struct {
		Orders []struct {
			ID    string `json:"id" binding:"required"`
			Order int    `json:"order" binding:"required"`
		} `json:"orders" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请求参数错误",
			"error":   err.Error(),
		})
		return
	}

	orders := make(map[string]int)
	for _, item := range req.Orders {
		orders[item.ID] = item.Order
	}

	if err := h.appService.UpdateOrder(orders); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "更新排序失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "排序更新成功",
	})
}

// GetDeleted returns all soft-deleted apps
// GET /api/v1/admin/apps/deleted
func (h *AppsHandler) GetDeleted(c *gin.Context) {
	apps, err := h.appService.GetDeleted()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取已删除应用列表失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    apps,
	})
}

// Recover restores a soft-deleted app
// POST /api/v1/admin/apps/:id/recover
func (h *AppsHandler) Recover(c *gin.Context) {
	id := c.Param("id")

	if err := h.appService.Recover(id); err != nil {
		if err.Error() == "app not found or not deleted" {
			c.JSON(http.StatusNotFound, gin.H{
				"code":    404,
				"message": "应用不存在或未被删除",
			})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "恢复应用失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "恢复成功",
		"data":    gin.H{"success": true},
	})
}

// BatchDelist batch soft-deletes apps
// POST /api/v1/admin/apps/batch-delist
func (h *AppsHandler) BatchDelist(c *gin.Context) {
	var req struct {
		IDs []string `json:"ids" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请求参数错误",
			"error":   err.Error(),
		})
		return
	}

	if err := h.appService.BatchDelete(req.IDs); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "批量下架失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "批量下架成功",
		"data":    gin.H{"success": true},
	})
}


