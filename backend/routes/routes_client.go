package routes

import (
	"appdeck/config"
	clientHandlers "appdeck/handlers/client"
	"appdeck/middleware"

	"github.com/gin-gonic/gin"
)

// setupClientAPIRoutes registers device-authed client-facing routes under the
// already-created /api/v1 group (DeviceAuthMiddleware is applied by the caller).
func setupClientAPIRoutes(v1 *gin.RouterGroup, cfg *config.Config) {
	registerVersionRoutes(v1, clientHandlers.NewVersionHandler())
	registerAppDataRoutes(v1, clientHandlers.NewAppsHandler())
	registerDownloadRoutes(v1, clientHandlers.NewDownloadHandler(cfg))
	registerStatsRoutes(v1, clientHandlers.NewStatsHandler())
	registerRankingsRoutes(v1, clientHandlers.NewRankingsHandler())
	registerClientWebSocketRoutes(v1, clientHandlers.NewWebSocketHandler(cfg))

	authHandler := clientHandlers.NewAuthHandler(cfg)
	userClientHandler := clientHandlers.NewUserClientHandler()
	registerClientAuthRoutes(v1, authHandler, cfg)
	registerUserProfileRoutes(v1, authHandler, userClientHandler, cfg)
	registerH5Routes(v1, authHandler, cfg)

	registerRechargeRoutes(v1, clientHandlers.NewRechargeHandler(cfg), cfg)

	saveDataHandler := clientHandlers.NewSaveDataHandler()
	registerSaveDataRoutes(v1, saveDataHandler)
	registerClientConfigRoutes(v1, clientHandlers.NewSystemConfigHandler(), clientHandlers.NewMoonlightHostsHandler(), saveDataHandler)
	registerClientScreensaverRoutes(v1, clientHandlers.NewScreensaverHandler(cfg))
}

func registerVersionRoutes(g *gin.RouterGroup, h *clientHandlers.VersionHandler) {
	g.GET("/version", h.GetVersion)
}

func registerAppDataRoutes(g *gin.RouterGroup, h *clientHandlers.AppsHandler) {
	g.GET("/apps/full", h.GetFullData)
}

func registerDownloadRoutes(g *gin.RouterGroup, h *clientHandlers.DownloadHandler) {
	g.GET("/icons/:filename", h.ServeIcon)
	g.GET("/bigpics/:filename", h.ServeBigPic)
}

func registerStatsRoutes(g *gin.RouterGroup, h *clientHandlers.StatsHandler) {
	g.POST("/stats/clicks", h.UploadClicks)
}

func registerRankingsRoutes(g *gin.RouterGroup, h *clientHandlers.RankingsHandler) {
	g.GET("/rankings", h.GetRankings)
}

func registerClientWebSocketRoutes(g *gin.RouterGroup, h *clientHandlers.WebSocketHandler) {
	g.GET("/ws/device", h.HandleDeviceConnection)
}

// registerClientAuthRoutes covers everything backed by AuthHandler that isn't
// the /user or /h5 groups: the promo plan lookup plus the /auth subgroup
// (QR login, WeChat OAuth, device binding).
func registerClientAuthRoutes(g *gin.RouterGroup, h *clientHandlers.AuthHandler, cfg *config.Config) {
	g.GET("/plans/promo", h.GetPromoPlan)

	auth := g.Group("/auth")
	{
		auth.GET("/qrcode", h.GetQRCode)
		auth.GET("/oa-qrcode", h.GetOAQRCode)
		auth.GET("/poll/:scene_str", h.PollStatus)
		auth.Any("/callback", h.WechatCallback)
		auth.POST("/logout", h.Logout)
		auth.GET("/me", middleware.AuthMiddleware(cfg), h.GetMe)
		auth.GET("/oauth/url", h.GetOAuthURL)
		auth.GET("/oauth/callback", h.WechatOAuthCallback)
		auth.POST("/bind-device", middleware.AuthMiddleware(cfg), h.BindDevice)
		auth.GET("/device-status", middleware.AuthMiddleware(cfg), h.GetDeviceStatus)
	}
}

// registerUserProfileRoutes covers the logged-in user's own profile/client
// info: balance and active-client management (/user/*), plus the equivalent
// /client/user-client lookup, all gated by user-JWT AuthMiddleware.
func registerUserProfileRoutes(g *gin.RouterGroup, authHandler *clientHandlers.AuthHandler, userClientHandler *clientHandlers.UserClientHandler, cfg *config.Config) {
	user := g.Group("/user")
	user.Use(middleware.AuthMiddleware(cfg))
	{
		user.GET("/balance", authHandler.GetBalance)
		user.GET("/clients/active", userClientHandler.GetActiveClients)
		user.POST("/clients/:id/quit", userClientHandler.QuitClient)
	}

	g.GET("/client/user-client", middleware.AuthMiddleware(cfg), userClientHandler.GetCurrentClient)
}

func registerH5Routes(g *gin.RouterGroup, h *clientHandlers.AuthHandler, cfg *config.Config) {
	h5 := g.Group("/h5")
	h5.Use(middleware.AuthMiddleware(cfg))
	{
		h5.GET("/check-follow", h.CheckFollow)
	}
}

func registerRechargeRoutes(g *gin.RouterGroup, h *clientHandlers.RechargeHandler, cfg *config.Config) {
	g.GET("/device/plan-info", h.GetDevicePlanInfo)

	recharge := g.Group("/recharge")
	{
		recharge.GET("/packages", h.GetPackages)
		recharge.GET("/callback", h.WechatServerVerify)
		recharge.POST("/callback", h.WechatPayCallback)
	}
	rechargeAuth := g.Group("/recharge")
	rechargeAuth.Use(middleware.AuthMiddleware(cfg))
	{
		rechargeAuth.POST("/create", h.CreateOrder)
		rechargeAuth.POST("/create-h5", h.CreateOrderJSAPI)
		rechargeAuth.GET("/poll/:order_no", h.PollOrder)
		rechargeAuth.GET("/orders", h.GetOrders)
	}
}

// registerSaveDataRoutes handles game save upload/download. Device-authed
// (not user-JWT): the upload/download can run from the background
// AppWebSocket service, which may outlive any Activity holding a user
// session, so user_id travels as an explicit param instead of coming from a
// JWT. The device already proved its identity via DeviceAuthMiddleware
// (applied to the whole v1 group), and only the backend can have told this
// device which user_id to use in the first place (via the
// save_upload_requested WS message).
func registerSaveDataRoutes(g *gin.RouterGroup, h *clientHandlers.SaveDataHandler) {
	g.POST("/client/saves/:userId", h.UploadSave)
	g.GET("/client/saves/:userId", h.DownloadSave)
}

// registerClientConfigRoutes covers the /client group: system config lookups,
// save-sync status, and moonlight host acquire/release/assignment.
func registerClientConfigRoutes(g *gin.RouterGroup, systemConfigHandler *clientHandlers.SystemConfigHandler, moonlightHostsHandler *clientHandlers.MoonlightHostsHandler, saveDataHandler *clientHandlers.SaveDataHandler) {
	client := g.Group("/client")
	{
		client.GET("/config/moonlight_host", systemConfigHandler.GetMoonlightHost)
		client.GET("/config/moonlight_port", systemConfigHandler.GetMoonlightPort)
		client.GET("/config/save_sync_enabled", saveDataHandler.GetSyncEnabled)
		client.POST("/moonlight/acquire", moonlightHostsHandler.AcquireHost)
		client.POST("/moonlight/release", moonlightHostsHandler.ReleaseHost)
		client.GET("/moonlight/assigned", moonlightHostsHandler.GetAssignedHost)
	}
}

func registerClientScreensaverRoutes(g *gin.RouterGroup, h *clientHandlers.ScreensaverHandler) {
	screensaver := g.Group("/screensaver")
	{
		screensaver.GET("/settings", h.GetSettings)
		screensaver.GET("/video", h.GetVideo)
		screensaver.HEAD("/video", h.GetVideo)
		screensaver.GET("/videos/:id", h.GetVideoByID)
		screensaver.HEAD("/videos/:id", h.GetVideoByID)
	}
}
