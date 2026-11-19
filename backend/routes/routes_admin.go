package routes

import (
	"appdeck/config"
	adminHandlers "appdeck/handlers/admin"
	"appdeck/middleware"

	"github.com/gin-gonic/gin"
)

// setupAdminAPIRoutes registers admin-facing routes under the already-created
// /api/v1 group (DeviceAuthMiddleware is applied by the caller; admin routes
// additionally require AdminAuthMiddleware, applied below).
func setupAdminAPIRoutes(v1 *gin.RouterGroup, cfg *config.Config) {
	screensaverHandler := adminHandlers.NewScreensaverHandler(cfg)

	admin := v1.Group("/admin")
	{
		registerAdminAuthRoutes(admin, adminHandlers.NewAdminAuthHandler(cfg), cfg)
		registerPublicScreensaverRoutes(admin, screensaverHandler)
	}

	adminAuth := v1.Group("/admin")
	adminAuth.Use(middleware.AdminAuthMiddleware(cfg))
	{
		registerAppRoutes(adminAuth, adminHandlers.NewAppsHandler())
		registerCategoryRoutes(adminAuth, adminHandlers.NewCategoriesHandler())
		registerUserRoutes(adminAuth, adminHandlers.NewUsersHandler(cfg))
		registerPlanRoutes(adminAuth, adminHandlers.NewPlansHandler())
		registerDeviceRoutes(adminAuth, adminHandlers.NewDevicesHandler())
		registerAppLogRoutes(adminAuth, adminHandlers.NewAppLogsHandler())
		registerUserClientRoutes(adminAuth, adminHandlers.NewUserClientsHandler())
		registerSteamRoutes(adminAuth, adminHandlers.NewSteamHandler())
		registerScreensaverRoutes(adminAuth, screensaverHandler)
		registerNotebookRoutes(adminAuth, adminHandlers.NewNotebookHandler())
		registerSystemConfigRoutes(adminAuth, adminHandlers.NewSystemConfigHandler())
		registerMoonlightHostRoutes(adminAuth, adminHandlers.NewMoonlightHostsHandler())
		registerRevenueRoutes(adminAuth, adminHandlers.NewRevenueHandler())
		registerRegisteredDeviceRoutes(adminAuth, adminHandlers.NewRegisteredDevicesHandler())
		registerTenantRoutes(adminAuth, adminHandlers.NewTenantsHandler())
		registerPlanSetRoutes(adminAuth, adminHandlers.NewPlanSetsHandler())
		registerGlobalPackageRoutes(adminAuth, adminHandlers.NewGlobalPackagesHandler(cfg))
		registerRefundRoutes(adminAuth, adminHandlers.NewRefundsHandler())
		registerDataMigrationRoutes(adminAuth, adminHandlers.NewDataMigrationHandler())
		registerBackupRoutes(adminAuth, adminHandlers.NewBackupHandler())
		registerSaveStorageRoutes(adminAuth, adminHandlers.NewSaveStorageHandler())
	}
}

// registerAdminAuthRoutes handles admin login plus the session routes that
// carry their own AdminAuthMiddleware inline (they live on the unauthenticated
// "admin" group because login itself can't require auth).
func registerAdminAuthRoutes(g *gin.RouterGroup, h *adminHandlers.AdminAuthHandler, cfg *config.Config) {
	g.POST("/login", h.Login)
	g.POST("/logout", middleware.AdminAuthMiddleware(cfg), h.Logout)
	g.GET("/me", middleware.AdminAuthMiddleware(cfg), h.GetMe)
}

// registerPublicScreensaverRoutes serves screensaver video content without
// admin auth, so devices can fetch it directly.
func registerPublicScreensaverRoutes(g *gin.RouterGroup, h *adminHandlers.ScreensaverHandler) {
	g.GET("/screensaver/video", h.GetVideo)
	g.HEAD("/screensaver/video", h.GetVideo)
	g.GET("/screensaver/videos/:id/file", h.ServeVideoFile)
}

func registerAppRoutes(g *gin.RouterGroup, h *adminHandlers.AppsHandler) {
	g.GET("/apps", h.GetAll)
	g.GET("/apps/deleted", h.GetDeleted)
	g.GET("/apps/:id", h.GetByID)
	g.POST("/apps", h.Create)
	g.PUT("/apps/:id", h.Update)
	g.DELETE("/apps/:id", h.Delete)
	g.POST("/apps/batch-delist", h.BatchDelist)
	g.POST("/apps/:id/recover", h.Recover)
	g.PUT("/apps/reorder", h.Reorder)
}

func registerCategoryRoutes(g *gin.RouterGroup, h *adminHandlers.CategoriesHandler) {
	g.GET("/categories", h.GetAll)
	g.GET("/categories/:id", h.GetByID)
	g.POST("/categories", h.Create)
	g.PUT("/categories/:id", h.Update)
	g.DELETE("/categories/:id", h.Delete)
	g.PUT("/categories/reorder", h.Reorder)
}

func registerUserRoutes(g *gin.RouterGroup, h *adminHandlers.UsersHandler) {
	g.GET("/users", h.GetAll)
	g.GET("/users/:id", h.GetByID)
	g.POST("/users/:id/balance", h.AdjustBalance)
	g.PUT("/users/:id/balance", h.AdjustBalance)
	g.PUT("/users/:id/status", h.UpdateStatus)
	g.GET("/users/:id/balance-logs", h.GetBalanceLogs)
	g.POST("/users/:id/tenant-balance", h.AdjustTenantBalance)
}

func registerPlanRoutes(g *gin.RouterGroup, h *adminHandlers.PlansHandler) {
	g.GET("/plans", h.GetAll)
	g.GET("/plans/:id", h.GetByID)
	g.POST("/plans", h.Create)
	g.PUT("/plans/:id", h.Update)
	g.DELETE("/plans/:id", h.Delete)
}

func registerDeviceRoutes(g *gin.RouterGroup, h *adminHandlers.DevicesHandler) {
	g.GET("/devices", h.GetAll)
	g.POST("/devices/:id/disconnect", h.Disconnect)
	g.POST("/devices/:id/reboot", h.Reboot)
	g.POST("/devices/:id/poweroff", h.Poweroff)
	g.POST("/devices/:id/shell", h.ShellExec)
	g.GET("/devices/:id/shell-history", h.ShellHistory)
	g.POST("/devices/:id/screenshot", h.Screenshot)
}

func registerAppLogRoutes(g *gin.RouterGroup, h *adminHandlers.AppLogsHandler) {
	g.GET("/sessions/:id/logs", h.GetSessionLogs)
}

func registerUserClientRoutes(g *gin.RouterGroup, h *adminHandlers.UserClientsHandler) {
	g.GET("/user-clients", h.GetAll)
	g.POST("/user-clients", h.Create)
}

func registerSteamRoutes(g *gin.RouterGroup, h *adminHandlers.SteamHandler) {
	g.GET("/steam/search", h.Search)
	g.POST("/steam/validate", h.ValidateSteamID)
}

func registerScreensaverRoutes(g *gin.RouterGroup, h *adminHandlers.ScreensaverHandler) {
	g.POST("/screensaver/video", h.UploadVideo)
	g.GET("/screensaver/settings", h.GetSettings)
	g.PUT("/screensaver/settings", h.SetSettings)
	g.POST("/screensaver/videos", h.UploadVideoToLibrary)
	g.GET("/screensaver/videos", h.ListVideos)
	g.DELETE("/screensaver/videos/:id", h.DeleteVideoFromLibrary)
	g.GET("/screensaver/default-video", h.GetDefaultVideo)
	g.PUT("/screensaver/default-video", h.SetDefaultVideo)
	g.GET("/screensaver/tenant-configs", h.ListTenantScreensavers)
	g.PUT("/screensaver/tenant-configs/:tenant_id", h.SetTenantScreensaver)
	g.DELETE("/screensaver/tenant-configs/:tenant_id", h.DeleteTenantScreensaver)
}

func registerNotebookRoutes(g *gin.RouterGroup, h *adminHandlers.NotebookHandler) {
	g.GET("/notebook", h.GetNote)
	g.PUT("/notebook", h.SaveNote)
}

func registerSystemConfigRoutes(g *gin.RouterGroup, h *adminHandlers.SystemConfigHandler) {
	g.GET("/config/moonlight_host", h.GetMoonlightHost)
	g.PUT("/config/moonlight_host", h.UpdateMoonlightHost)
	g.GET("/config/moonlight_port", h.GetMoonlightPort)
	g.PUT("/config/moonlight_port", h.UpdateMoonlightPort)
}

func registerMoonlightHostRoutes(g *gin.RouterGroup, h *adminHandlers.MoonlightHostsHandler) {
	g.GET("/moonlight/hosts", h.GetAll)
	g.POST("/moonlight/hosts", h.Create)
	g.PUT("/moonlight/hosts/:id", h.Update)
	g.DELETE("/moonlight/hosts/:id", h.Delete)
	g.POST("/moonlight/hosts/:id/release", h.ForceRelease)
	g.POST("/moonlight/hosts/:id/pair", h.TriggerPair)
}

func registerRevenueRoutes(g *gin.RouterGroup, h *adminHandlers.RevenueHandler) {
	g.GET("/revenue/summary", h.GetRevenueSummary)
}

// registerRegisteredDeviceRoutes covers the full /registered-devices resource.
// The original handwritten route list had the assign-tenant/schedule/bypass-charge
// routes stranded after the unrelated tenants block further down the file;
// they're grouped here with the rest of the resource instead.
func registerRegisteredDeviceRoutes(g *gin.RouterGroup, h *adminHandlers.RegisteredDevicesHandler) {
	g.GET("/registered-devices", h.GetAll)
	g.POST("/registered-devices", h.Create)
	g.POST("/registered-devices/:id/approve", h.Approve)
	g.POST("/registered-devices/:id/reject", h.Reject)
	g.PUT("/registered-devices/:id", h.Update)
	g.DELETE("/registered-devices/:id", h.Delete)
	g.PUT("/registered-devices/:id/assign-tenant", h.AssignTenant)
	g.PUT("/registered-devices/:id/schedule", h.UpdateSchedule)
	g.PUT("/registered-devices/:id/bypass-charge", h.UpdateBypassCharge)
}

// registerTenantRoutes covers the full /tenants resource. UpdateBillingConfig
// was previously registered on its own at the very end of the route list,
// disconnected from the rest of the tenant routes; it's grouped here instead.
func registerTenantRoutes(g *gin.RouterGroup, h *adminHandlers.TenantsHandler) {
	g.GET("/tenants", h.GetAll)
	g.GET("/tenants/active", h.GetAllActive)
	g.POST("/tenants", h.Create)
	g.PUT("/tenants/:id", h.Update)
	g.PUT("/tenants/:id/password", h.UpdatePassword)
	g.PUT("/tenants/:id/status", h.UpdateStatus)
	g.DELETE("/tenants/:id", h.Delete)
	g.PUT("/tenants/:id/billing", h.UpdateBillingConfig)
}

func registerPlanSetRoutes(g *gin.RouterGroup, h *adminHandlers.PlanSetsHandler) {
	g.GET("/plan-sets", h.GetAll)
	g.GET("/plan-sets/active", h.GetActivePlanSets)
	g.GET("/plan-sets/deleted", h.GetDeleted)
	g.POST("/plan-sets", h.Create)
	g.GET("/plan-sets/:id", h.GetByID)
	g.PUT("/plan-sets/:id", h.Update)
	g.PUT("/plan-sets/:id/status", h.UpdateStatus)
	g.DELETE("/plan-sets/:id", h.Delete)
	g.POST("/plan-sets/:id/recover", h.Recover)
	g.POST("/plan-sets/:id/plans", h.AddPlan)
	g.PUT("/plan-sets/plans/:id", h.UpdatePlan)
	g.PUT("/plan-sets/plans/:id/status", h.UpdatePlanStatus)
	g.DELETE("/plan-sets/plans/:id", h.DeletePlan)
}

// registerGlobalPackageRoutes also covers /recharge-orders: both are served by
// GlobalPackagesHandler in the original code, so they're kept together here.
func registerGlobalPackageRoutes(g *gin.RouterGroup, h *adminHandlers.GlobalPackagesHandler) {
	g.GET("/global-packages", h.GetAll)
	g.GET("/global-packages/deleted", h.GetDeleted)
	g.POST("/global-packages", h.Create)
	g.PUT("/global-packages/:id", h.Update)
	g.PUT("/global-packages/:id/status", h.UpdateStatus)
	g.DELETE("/global-packages/:id", h.Delete)
	g.POST("/global-packages/:id/recover", h.Recover)
	g.DELETE("/global-packages/:id/permanent", h.PermanentDelete)

	g.GET("/recharge-orders", h.GetOrders)
	g.GET("/recharge-orders/:id", h.GetOrderByID)
}

func registerRefundRoutes(g *gin.RouterGroup, h *adminHandlers.RefundsHandler) {
	g.GET("/orders/:id/refund-preview", h.PreviewRefund)
	g.POST("/orders/:id/refund", h.ProcessRefund)
}

func registerDataMigrationRoutes(g *gin.RouterGroup, h *adminHandlers.DataMigrationHandler) {
	g.POST("/data/export", h.ExportData)
	g.POST("/data/import", h.ImportData)
}

func registerBackupRoutes(g *gin.RouterGroup, h *adminHandlers.BackupHandler) {
	g.GET("/backups", h.ListBackups)
	g.POST("/backups", h.CreateBackup)
	g.POST("/backups/upload", h.UploadBackup)
	g.POST("/backups/restore", h.RestoreBackup)
	g.DELETE("/backups/:filename", h.DeleteBackup)
	g.GET("/backups/:filename/download", h.DownloadBackup)
	g.GET("/backups/schedule", h.GetSchedule)
	g.PUT("/backups/schedule", h.UpdateSchedule)
}

func registerSaveStorageRoutes(g *gin.RouterGroup, h *adminHandlers.SaveStorageHandler) {
	g.GET("/save-storage", h.GetConfig)
	g.PUT("/save-storage", h.UpdateConfig)
	g.GET("/save-storage/saves", h.ListSaves)
	g.POST("/save-storage/backups", h.CreateBackup)
	g.GET("/save-storage/backups", h.ListBackups)
	g.POST("/save-storage/backups/upload", h.UploadBackup)
	g.POST("/save-storage/backups/restore", h.RestoreBackup)
	g.DELETE("/save-storage/backups/:filename", h.DeleteBackup)
	g.GET("/save-storage/backups/:filename/download", h.DownloadBackup)
}
