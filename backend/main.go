package main

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"appdeck/config"
	"appdeck/database"
	"appdeck/routes"
	"appdeck/services"
	"appdeck/workers"

	"github.com/gin-gonic/gin"
)

//go:embed all:static
var staticFS embed.FS

//go:embed h5
var h5FS embed.FS

//go:embed appclient
var appClientFS embed.FS

func main() {
	cfg := config.Load()

	if err := database.InitMySQL(&cfg.Database); err != nil {
		log.Fatalf("数据库连接失败: %v", err)
	}
	defer database.Close()

	if os.Getenv("AUTO_MIGRATE") == "true" {
		if err := database.AutoMigrate(); err != nil {
			log.Fatalf("数据库迁移失败: %v", err)
		}
		log.Println("数据库表结构迁移成功")
	}

	// Startup tasks
	runStartupTasks(cfg)

	// Prepare embedded static assets
	staticSub, _ := fs.Sub(staticFS, "static")
	adminIndex, _ := fs.ReadFile(staticFS, "static/index.html")
	h5Sub, _ := fs.Sub(h5FS, "h5")
	h5TenantSub, _ := fs.Sub(h5FS, "h5/tenant")
	appClientSub, _ := fs.Sub(appClientFS, "appclient")

	// Setup router
	router := gin.Default()
	router.RedirectTrailingSlash = false
	router.MaxMultipartMemory = 256 << 20

	routes.Setup(router, cfg, &routes.StaticAssets{
		AdminFS:     staticSub,
		AdminIndex:  adminIndex,
		H5FS:        h5Sub,
		H5TenantFS:  h5TenantSub,
		AppClientFS: appClientSub,
	})

	// Graceful shutdown context
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Start background workers
	workers.StartAll(ctx, cfg)

	// Start HTTP server
	addr := fmt.Sprintf("%s:%s", cfg.Server.Host, cfg.Server.Port)
	log.Printf("服务器启动在 %s", addr)

	srv := &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  10 * time.Minute,
		WriteTimeout: 10 * time.Minute,
	}

	go func() {
		<-ctx.Done()
		log.Println("Shutting down server...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Printf("Server shutdown error: %v", err)
		}
	}()

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("服务器启动失败: %v", err)
	}

	log.Println("Server stopped")
}

func runStartupTasks(cfg *config.Config) {
	os.MkdirAll(cfg.Data.IconsDir, 0755)
	os.MkdirAll(cfg.Data.BigPicsDir, 0755)
	os.MkdirAll("./data/backups", 0755)

	screensaverService := services.GetScreensaverService()
	if err := screensaverService.InitializeDefaults(); err != nil {
		log.Printf("Warning: Failed to initialize screensaver defaults: %v", err)
	}

	sessionService := services.NewSessionService()
	if err := sessionService.CloseOrphanedSessions(); err != nil {
		log.Printf("Warning: Failed to close orphaned sessions: %v", err)
	} else {
		log.Println("Orphaned sessions cleanup completed")
	}

	appLogService := services.NewAppLogService()
	appLogService.CloseOrphanedAppLogs()

	moonlightHostService := services.GetMoonlightHostService()
	if err := moonlightHostService.MigrateFromSystemConfig(database.DB); err != nil {
		log.Printf("Warning: Failed to migrate moonlight config: %v", err)
	}

	services.NewWechatService(&cfg.WeChat)
	services.NewWeChatOAService(&cfg.WeChat)

	services.InitBackupService(&cfg.Database, "./data/backups")

	saveDataService := services.InitSaveDataService(services.NewLocalSaveStore("./data/saves"), "./data/save-backups")
	if storageCfg, err := saveDataService.GetStorageConfig(); err != nil {
		log.Printf("Warning: Failed to load save storage config, using default ./data/saves: %v", err)
	} else {
		saveDataService.SetStore(services.NewLocalSaveStore(storageCfg.LocalPath))
	}
}
