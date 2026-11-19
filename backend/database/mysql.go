package database

import (
	"fmt"
	"log"
	"time"

	"appdeck/config"
	"appdeck/models"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

// ensureDatabase 确保数据库存在，不存在则自动创建
func ensureDatabase(cfg *config.DatabaseConfig) error {
	// Connect to MySQL without specifying a database
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/?charset=%s&parseTime=True&loc=Local",
		cfg.User,
		cfg.Password,
		cfg.Host,
		cfg.Port,
		cfg.Charset,
	)

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return fmt.Errorf("failed to connect to MySQL server: %w", err)
	}

	createSQL := fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci", cfg.DBName)
	if err := db.Exec(createSQL).Error; err != nil {
		return fmt.Errorf("failed to create database %s: %w", cfg.DBName, err)
	}

	sqlDB, _ := db.DB()
	sqlDB.Close()

	log.Printf("数据库 '%s' 已确认存在", cfg.DBName)
	return nil
}

// InitMySQL 初始化 MySQL 连接
func InitMySQL(cfg *config.DatabaseConfig) error {
	// Auto-create database if not exists
	if err := ensureDatabase(cfg); err != nil {
		return err
	}

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=%s&parseTime=True&loc=Local&maxAllowedPacket=67108864",
		cfg.User,
		cfg.Password,
		cfg.Host,
		cfg.Port,
		cfg.DBName,
		cfg.Charset,
	)

	var err error
	DB, err = gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger:                                   logger.Default.LogMode(logger.Warn),
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	// Raise server-side max_allowed_packet to 64 MB so large result sets work
	DB.Exec("SET GLOBAL max_allowed_packet = 67108864")

	sqlDB, err := DB.DB()
	if err != nil {
		return fmt.Errorf("failed to get database instance: %w", err)
	}

	// 设置连接池
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	log.Println("MySQL 数据库连接成功")
	return nil
}

// AutoMigrate 自动迁移表结构（开发环境使用）
func AutoMigrate() error {
	err := DB.AutoMigrate(
		&models.App{},
		&models.Category{},
		&models.Version{},
		&models.User{},
		&models.Admin{},
		&models.LoginSession{},
		&models.RechargeOrder{},
		&models.BalanceLog{},
		&models.BillingPlan{},
		&models.BillingSession{},
		&models.UserClient{},
		&models.Setting{},
		&models.SystemConfig{},
		&models.MoonlightHost{},
		&models.Tenant{},
		&models.RegisteredDevice{},
		&models.PlanSet{},
		&models.UserTenantBalance{},
		&models.GlobalRechargePackage{},
		&models.BillingSessionSegment{},
		&models.BackupSchedule{},
		&models.ScreensaverVideo{},
		&models.TenantScreensaver{},
		&models.SessionAppLog{},
		&models.ShellLog{},
		&models.AdminNotebook{},
		&models.TenantAppClick{},
		&models.UserSaveData{},
		&models.SaveStorageConfig{},
	)
	if err != nil {
		return err
	}

	// Seed default admin account if not exists
	if err := seedDefaultAdmin(); err != nil {
		log.Printf("Failed to seed default admin: %v", err)
		return err
	}

	// Seed default categories if not exists
	if err := seedDefaultCategories(); err != nil {
		log.Printf("Failed to seed default categories: %v", err)
		return err
	}

	// Seed default system config if not exists
	if err := seedDefaultSystemConfig(); err != nil {
		log.Printf("Failed to seed default system config: %v", err)
		return err
	}

	// Seed default plan set if not exists
	if err := seedDefaultPlanSet(); err != nil {
		log.Printf("Failed to seed default plan set: %v", err)
		return err
	}

	return nil
}

// seedDefaultAdmin 创建默认管理员账号
func seedDefaultAdmin() error {
	// Check if admin already exists
	var count int64
	if err := DB.Model(&models.Admin{}).Where("username = ?", "admin").Count(&count).Error; err != nil {
		return fmt.Errorf("failed to check admin existence: %w", err)
	}

	// Skip if admin already exists
	if count > 0 {
		log.Println("Default admin account already exists, skipping seed")
		return nil
	}

	// Hash default password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	// Create default admin
	admin := &models.Admin{
		ID:          uuid.New().String(),
		Username:    "admin",
		Password:    string(hashedPassword),
		DisplayName: "Default Admin",
		Status:      1,
	}

	if err := DB.Create(admin).Error; err != nil {
		return fmt.Errorf("failed to create default admin: %w", err)
	}

	log.Println("Default admin account created successfully (username: admin, password: admin123)")
	return nil
}

// seedDefaultCategories 创建默认分类
func seedDefaultCategories() error {
	defaultCategories := []struct {
		Name     string
		OrderNum int
	}{
		{Name: "双人对决", OrderNum: 0},
		{Name: "动作对战", OrderNum: 1},
		{Name: "体育竞技", OrderNum: 2},
		{Name: "冒险探索", OrderNum: 3},
		{Name: "角色扮演", OrderNum: 4},
		{Name: "策略模拟", OrderNum: 5},
		{Name: "枪战射击", OrderNum: 6},
		{Name: "休闲娱乐", OrderNum: 7},
		{Name: "街机游戏", OrderNum: 8},
		{Name: "特色登陆", OrderNum: 9},
	}

	for _, cat := range defaultCategories {
		// Check if category already exists
		var count int64
		if err := DB.Model(&models.Category{}).Where("name = ?", cat.Name).Count(&count).Error; err != nil {
			return fmt.Errorf("failed to check category existence for %s: %w", cat.Name, err)
		}

		// Skip if category already exists
		if count > 0 {
			log.Printf("Category '%s' already exists, skipping", cat.Name)
			continue
		}

		// Create category
		category := &models.Category{
			ID:       uuid.New().String(),
			Name:     cat.Name,
			OrderNum: cat.OrderNum,
		}

		if err := DB.Create(category).Error; err != nil {
			return fmt.Errorf("failed to create category %s: %w", cat.Name, err)
		}

		log.Printf("Category '%s' created successfully", cat.Name)
	}

	return nil
}

// seedDefaultSystemConfig 创建默认系统配置
func seedDefaultSystemConfig() error {
	// Check if moonlight_host config already exists
	var hostCount int64
	if err := DB.Model(&models.SystemConfig{}).Where("`key` = ?", "moonlight_host").Count(&hostCount).Error; err != nil {
		return fmt.Errorf("failed to check moonlight_host existence: %w", err)
	}

	// Create default moonlight_host config if not exists
	if hostCount == 0 {
		hostConfig := &models.SystemConfig{
			Key:         "moonlight_host",
			Value:       "127.0.0.1",
			Description: "Moonlight server IP or hostname",
		}

		if err := DB.Create(hostConfig).Error; err != nil {
			return fmt.Errorf("failed to create moonlight_host config: %w", err)
		}

		log.Println("System config 'moonlight_host' created successfully (default: 127.0.0.1)")
	} else {
		log.Println("System config 'moonlight_host' already exists, skipping seed")
	}

	// Check if moonlight_port config already exists
	var portCount int64
	if err := DB.Model(&models.SystemConfig{}).Where("`key` = ?", "moonlight_port").Count(&portCount).Error; err != nil {
		return fmt.Errorf("failed to check moonlight_port existence: %w", err)
	}

	// Create default moonlight_port config if not exists
	if portCount == 0 {
		portConfig := &models.SystemConfig{
			Key:         "moonlight_port",
			Value:       "",
			Description: "Moonlight server port (empty = auto-detect)",
		}

		if err := DB.Create(portConfig).Error; err != nil {
			return fmt.Errorf("failed to create moonlight_port config: %w", err)
		}

		log.Println("System config 'moonlight_port' created successfully (default: empty/auto-detect)")
	} else {
		log.Println("System config 'moonlight_port' already exists, skipping seed")
	}

	return nil
}

// seedDefaultPlanSet 创建默认计费方案集并迁移现有数据
func seedDefaultPlanSet() error {
	var count int64
	if err := DB.Model(&models.PlanSet{}).Where("is_default = 1").Count(&count).Error; err != nil {
		return fmt.Errorf("failed to check default plan set: %w", err)
	}
	if count > 0 {
		log.Println("Default plan set already exists, skipping seed")
		return nil
	}

	planSet := &models.PlanSet{
		ID:          uuid.New().String(),
		Name:        "Default",
		Description: "默认计费方案集",
		IsDefault:   1,
		Status:      1,
	}
	if err := DB.Create(planSet).Error; err != nil {
		return fmt.Errorf("failed to create default plan set: %w", err)
	}

	// Migrate existing default billing plan into this plan set
	var defaultPlan models.BillingPlan
	if DB.Where("is_default = 1 AND status = 1").First(&defaultPlan).Error == nil {
		DB.Model(&defaultPlan).Updates(map[string]interface{}{
			"plan_set_id": planSet.ID,
			"type":        "rate",
		})
		DB.Model(planSet).Updates(map[string]interface{}{
			"standard_plan_id":    defaultPlan.ID,
			"global_rate_plan_id": defaultPlan.ID,
		})
		log.Printf("Migrated default plan '%s' into default plan set", defaultPlan.Name)
	}

	log.Println("Default plan set created successfully")
	return nil
}

// Close 关闭数据库连接
func Close() error {
	sqlDB, err := DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}


