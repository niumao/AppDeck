package main

import (
	"fmt"
	"log"
	"os"

	"appdeck/config"
	"appdeck/database"
	adminHandlers "appdeck/handlers/admin"
	clientHandlers "appdeck/handlers/client"
	"appdeck/middleware"

	"github.com/gin-gonic/gin"
)

func main() {
	// 加载配置
	cfg := config.Load()

	// 初始化 MySQL 数据库
	if err := database.InitMySQL(&cfg.Database); err != nil {
		log.Fatalf("数据库连接失败: %v", err)
	}
	defer database.Close()

	// 自动迁移表结构（开发环境）
	if os.Getenv("AUTO_MIGRATE") == "true" {
		if err := database.AutoMigrate(); err != nil {
			log.Fatalf("数据库迁移失败: %v", err)
		}
		log.Println("数据库表结构迁移成功")
	}

	// 创建数据目录
	os.MkdirAll(cfg.Data.IconsDir, 0755)
	os.MkdirAll(cfg.Data.BigPicsDir, 0755)

	// 初始化 Gin
	router := gin.Default()

	// CORS 中间件
	router.Use(middleware.CORS())

	// 初始化处理器
	versionHandler := clientHandlers.NewVersionHandler()
	appsHandler := clientHandlers.NewAppsHandler()
	downloadHandler := clientHandlers.NewDownloadHandler(cfg)
	statsHandler := clientHandlers.NewStatsHandler()

	adminAppsHandler := adminHandlers.NewAppsHandler()
	adminCategoriesHandler := adminHandlers.NewCategoriesHandler()
	adminPackageHandler := adminHandlers.NewPackageHandler(cfg)

	// 客户端 API 路由
	v1 := router.Group("/api/v1")
	{
		// 版本检查
		v1.GET("/version", versionHandler.GetVersion)

		// 应用数据
		v1.GET("/apps/full", appsHandler.GetFullData)

		// 图标和大图
		v1.GET("/icons/:filename", downloadHandler.ServeIcon)
		v1.GET("/bigpics/:filename", downloadHandler.ServeBigPic)

		// 点击统计
		v1.POST("/stats/clicks", statsHandler.UploadClicks)

		// 客户端包下载
		client := v1.Group("/client")
		{
			client.GET("/package-info", downloadHandler.GetPackageInfo)
			client.GET("/database", downloadHandler.DownloadDatabase)
			client.GET("/resources", downloadHandler.DownloadResources)
		}

		// 管理 API
		admin := v1.Group("/admin")
		{
			// 应用管理
			admin.GET("/apps", adminAppsHandler.GetAll)
			admin.GET("/apps/:id", adminAppsHandler.GetByID)
			admin.POST("/apps", adminAppsHandler.Create)
			admin.PUT("/apps/:id", adminAppsHandler.Update)
			admin.DELETE("/apps/:id", adminAppsHandler.Delete)
			admin.PUT("/apps/reorder", adminAppsHandler.Reorder)

			// 分类管理
			admin.GET("/categories", adminCategoriesHandler.GetAll)
			admin.GET("/categories/:id", adminCategoriesHandler.GetByID)
			admin.POST("/categories", adminCategoriesHandler.Create)
			admin.PUT("/categories/:id", adminCategoriesHandler.Update)
			admin.DELETE("/categories/:id", adminCategoriesHandler.Delete)
			admin.PUT("/categories/reorder", adminCategoriesHandler.Reorder)

			// 客户端包管理
			admin.GET("/package-status", adminPackageHandler.GetStatus)
			admin.POST("/generate-client-package", adminPackageHandler.Generate)
		}
	}

	// 静态文件服务（管理前端）
	router.Static("/admin", "./static")

	// 启动服务器
	addr := fmt.Sprintf("%s:%s", cfg.Server.Host, cfg.Server.Port)
	log.Printf("服务器启动在 %s", addr)
	if err := router.Run(addr); err != nil {
		log.Fatalf("服务器启动失败: %v", err)
	}
}


