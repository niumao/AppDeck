package routes

import (
	"io/fs"
	"net/http"
	"path"
	"strings"

	"appdeck/config"
	clientHandlers "appdeck/handlers/client"
	tenantHandlers "appdeck/handlers/tenant"
	"appdeck/middleware"

	"github.com/gin-gonic/gin"
)

type StaticAssets struct {
	AdminFS     fs.FS
	AdminIndex  []byte
	H5FS        fs.FS
	H5TenantFS  fs.FS
	AppClientFS fs.FS
}

func Setup(router *gin.Engine, cfg *config.Config, assets *StaticAssets) {
	router.Use(middleware.CORS())
	setupAPIRoutes(router, cfg)
	setupStaticRoutes(router, assets)
}

func setupAPIRoutes(router *gin.Engine, cfg *config.Config) {
	v1 := router.Group("/api/v1")
	{
		v1.GET("/device/register", clientHandlers.NewDeviceRegisterHandler(cfg).CheckRegistration)
		v1.Use(middleware.DeviceAuthMiddleware())

		setupClientAPIRoutes(v1, cfg)
		setupAdminAPIRoutes(v1, cfg)
	}

	setupTenantRoutes(router, cfg)
}

func setupTenantRoutes(router *gin.Engine, cfg *config.Config) {
	tenantAuthHandler := tenantHandlers.NewAuthHandler(cfg)
	tenantRevenueHandler := tenantHandlers.NewRevenueHandler()

	tenantGroup := router.Group("/tenant")
	{
		tenantGroup.POST("/login", tenantAuthHandler.Login)
	}
	tenantAuthGroup := router.Group("/tenant")
	tenantAuthGroup.Use(middleware.TenantAuthMiddleware(cfg))
	{
		tenantAuthGroup.GET("/me", tenantAuthHandler.GetMe)
		tenantAuthGroup.GET("/devices", tenantRevenueHandler.GetDevices)
		tenantAuthGroup.GET("/revenue", tenantRevenueHandler.GetRevenue)
	}
}

func setupStaticRoutes(router *gin.Engine, assets *StaticAssets) {
	router.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/admin")
	})

	// Admin SPA
	serveAdminIndex := func(c *gin.Context) {
		c.Data(http.StatusOK, "text/html; charset=utf-8", assets.AdminIndex)
	}
	router.GET("/admin/*filepath", func(c *gin.Context) {
		fp := c.Param("filepath")
		if fp != "" && fp != "/" {
			cleanPath := strings.TrimPrefix(fp, "/")
			if f, err := assets.AdminFS.Open(cleanPath); err == nil {
				f.Close()
				c.FileFromFS(path.Join("/", cleanPath), http.FS(assets.AdminFS))
				return
			}
		}
		serveAdminIndex(c)
	})
	router.GET("/admin", serveAdminIndex)

	// H5 payment pages (no cache)
	noCacheMiddleware := func(c *gin.Context) {
		c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
		c.Header("Pragma", "no-cache")
		c.Header("Expires", "0")
		c.Next()
	}

	router.Group("/h5").Use(noCacheMiddleware).StaticFS("/", http.FS(assets.H5FS))
	router.GET("/h5", func(c *gin.Context) {
		target := "/h5/"
		if c.Request.URL.RawQuery != "" {
			target += "?" + c.Request.URL.RawQuery
		}
		c.Redirect(http.StatusMovedPermanently, target)
	})

	// Tenant H5 page (no cache)
	router.Group("/tenant-h5").Use(noCacheMiddleware).StaticFS("/", http.FS(assets.H5TenantFS))

	// App client pages (no cache)
	router.Group("/app").Use(noCacheMiddleware).StaticFS("/", http.FS(assets.AppClientFS))
	router.GET("/app", func(c *gin.Context) {
		target := "/app/"
		if c.Request.URL.RawQuery != "" {
			target += "?" + c.Request.URL.RawQuery
		}
		c.Redirect(http.StatusMovedPermanently, target)
	})
}
