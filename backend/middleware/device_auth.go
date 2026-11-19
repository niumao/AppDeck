package middleware

import (
	"net/http"

	"appdeck/services"

	"github.com/gin-gonic/gin"
)

// DeviceAuthMiddleware rejects requests from unregistered devices.
// If the request carries a machine_code query parameter, the device must be
// approved in the registered_devices table. Requests without machine_code
// (e.g. H5 pages, WeChat callbacks) are not affected.
func DeviceAuthMiddleware() gin.HandlerFunc {
	deviceService := services.GetRegisteredDeviceService()

	return func(c *gin.Context) {
		machineCode := c.Query("machine_code")
		if machineCode == "" {
			c.Next()
			return
		}

		if !deviceService.IsDeviceAllowed(machineCode) {
			// Auto-register unknown devices as "pending" so they appear
			// in the admin panel for approval.
			hostID := c.Query("host_id")
			ip := c.ClientIP()
			deviceService.EnsurePendingRecord(machineCode, hostID, ip)

			c.JSON(http.StatusForbidden, gin.H{
				"code":    403,
				"message": "DEVICE_NOT_REGISTERED",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
