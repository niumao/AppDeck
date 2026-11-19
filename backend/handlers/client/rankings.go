package client

import (
	"net/http"

	"appdeck/services"

	"github.com/gin-gonic/gin"
)

type RankingsHandler struct {
	appService              *services.AppService
	registeredDeviceService *services.RegisteredDeviceService
}

func NewRankingsHandler() *RankingsHandler {
	return &RankingsHandler{
		appService:              services.NewAppService(),
		registeredDeviceService: services.GetRegisteredDeviceService(),
	}
}

// GetRankings 一次性返回热门榜、飙升榜，以及（当设备所属租户可解析时的）地区榜。
// GET /api/v1/rankings
// 可选请求头 X-Machine-Code：用于解析设备所属租户以计算地区榜。
// 这里特意使用请求头而不是 query 参数：DeviceAuthMiddleware 会对携带
// machine_code query 参数的请求做设备审核状态校验，若用 query 参数会导致
// 未审核设备的热门榜/飙升榜也被一并 403。
func (h *RankingsHandler) GetRankings(c *gin.Context) {
	tenantID := ""
	if machineCode := c.GetHeader("X-Machine-Code"); machineCode != "" {
		tenantID = h.registeredDeviceService.GetTenantIDByMachineCode(machineCode)
	}

	hot, trending, regional, err := h.appService.GetAllRankings(tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"hot":      hot,
			"trending": trending,
			"regional": regional,
		},
	})
}
