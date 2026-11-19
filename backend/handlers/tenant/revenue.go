package tenant

import (
	"appdeck/database"
	"appdeck/models"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type RevenueHandler struct{}

func NewRevenueHandler() *RevenueHandler {
	return &RevenueHandler{}
}

// GetDevices handles GET /tenant/devices
func (h *RevenueHandler) GetDevices(c *gin.Context) {
	tenantID := c.GetString("tenant_id")

	var devices []models.RegisteredDevice
	if err := database.DB.Where("tenant_id = ?", tenantID).
		Order("device_name ASC").Find(&devices).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to retrieve devices",
			"data":    nil,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    devices,
	})
}

type deviceRevenue struct {
	DeviceID        string `json:"device_id"`
	DeviceName      string `json:"device_name"`
	MachineCode     string `json:"machine_code"`
	SessionCount    int    `json:"session_count"`
	IncomeCents     int    `json:"income_cents"`
	DurationMinutes int    `json:"duration_minutes"`
}

// GetRevenue handles GET /tenant/revenue
func (h *RevenueHandler) GetRevenue(c *gin.Context) {
	tenantID := c.GetString("tenant_id")

	// Parse date range
	dateFromStr := c.Query("date_from")
	dateToStr := c.Query("date_to")

	var dateFrom, dateTo time.Time

	if dateFromStr != "" {
		if t, err := time.Parse("2006-01-02", dateFromStr); err == nil {
			dateFrom = t
		} else {
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    400,
				"message": "date_from format must be YYYY-MM-DD",
				"data":    nil,
			})
			return
		}
	} else {
		dateFrom = time.Now().AddDate(0, 0, -30)
	}

	if dateToStr != "" {
		if t, err := time.Parse("2006-01-02", dateToStr); err == nil {
			dateTo = t
		} else {
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    400,
				"message": "date_to format must be YYYY-MM-DD",
				"data":    nil,
			})
			return
		}
	} else {
		dateTo = time.Now()
	}

	dateFrom = time.Date(dateFrom.Year(), dateFrom.Month(), dateFrom.Day(), 0, 0, 0, 0, dateFrom.Location())
	dateTo = time.Date(dateTo.Year(), dateTo.Month(), dateTo.Day(), 23, 59, 59, 999999999, dateTo.Location())

	// Optional device filter
	registeredDeviceID := c.Query("registered_device_id")

	// Get tenant's devices
	deviceQuery := database.DB.Model(&models.RegisteredDevice{}).Where("tenant_id = ?", tenantID)
	if registeredDeviceID != "" {
		deviceQuery = deviceQuery.Where("id = ?", registeredDeviceID)
	}

	var devices []models.RegisteredDevice
	if err := deviceQuery.Find(&devices).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to retrieve devices",
			"data":    nil,
		})
		return
	}

	// Collect machine codes
	machineCodes := make([]string, len(devices))
	for i, d := range devices {
		machineCodes[i] = d.MachineCode
	}

	// Query revenue per device
	var results []struct {
		MachineCode   string `gorm:"column:machine_code"`
		SessionCount  int64  `gorm:"column:session_count"`
		TotalCost     int64  `gorm:"column:total_cost"`
		TotalDuration int64  `gorm:"column:total_duration"`
	}

	if len(machineCodes) > 0 {
		database.DB.Raw(`
			SELECT uc.machine_code,
				COUNT(bs.id) AS session_count,
				COALESCE(SUM(bs.total_cost), 0) AS total_cost,
				COALESCE(SUM(bs.duration_sec), 0) AS total_duration
			FROM user_clients uc
			INNER JOIN billing_sessions bs ON bs.client_id = uc.id
				AND bs.ended_at IS NOT NULL
				AND bs.started_at >= ? AND bs.started_at <= ?
			WHERE uc.machine_code IN ?
			GROUP BY uc.machine_code
		`, dateFrom, dateTo, machineCodes).Scan(&results)
	}

	// Build response
	revenueMap := make(map[string]*struct {
		sessions int
		cost     int
		duration int
	})
	for _, r := range results {
		revenueMap[r.MachineCode] = &struct {
			sessions int
			cost     int
			duration int
		}{int(r.SessionCount), int(r.TotalCost), int(r.TotalDuration)}
	}

	var totalIncome, totalSessions, totalDuration int
	var deviceRevenues []deviceRevenue

	for _, d := range devices {
		rev := deviceRevenue{
			DeviceID:    d.ID,
			DeviceName:  d.DeviceName,
			MachineCode: d.MachineCode,
		}
		if r, ok := revenueMap[d.MachineCode]; ok {
			rev.SessionCount = r.sessions
			rev.IncomeCents = r.cost
			rev.DurationMinutes = r.duration / 60
			totalSessions += r.sessions
			totalIncome += r.cost
			totalDuration += r.duration
		}
		deviceRevenues = append(deviceRevenues, rev)
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"total_income_cents":     totalIncome,
			"total_sessions":         totalSessions,
			"total_duration_minutes": totalDuration / 60,
			"devices":                deviceRevenues,
		},
	})
}
