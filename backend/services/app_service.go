package services

import (
	"fmt"

	"appdeck/database"
	"appdeck/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type AppService struct {
	versionService *VersionService
}

func NewAppService() *AppService {
	return &AppService{
		versionService: NewVersionService(),
	}
}

// GetAll 获取所有应用（带分类ID）
func (s *AppService) GetAll() ([]models.AppWithCategories, error) {
	var apps []models.App
	if err := database.DB.Preload("Categories").Order("order_num ASC").Find(&apps).Error; err != nil {
		return nil, err
	}

	result := make([]models.AppWithCategories, len(apps))
	for i, app := range apps {
		categoryIDs := make([]string, len(app.Categories))
		for j, cat := range app.Categories {
			categoryIDs[j] = cat.ID
		}
		result[i] = models.AppWithCategories{
			App:         app,
			CategoryIDs: categoryIDs,
		}
	}
	return result, nil
}

// GetByID 根据ID获取应用
func (s *AppService) GetByID(id string) (*models.AppWithCategories, error) {
	var app models.App
	if err := database.DB.Preload("Categories").First(&app, "id = ?", id).Error; err != nil {
		return nil, err
	}

	categoryIDs := make([]string, len(app.Categories))
	for i, cat := range app.Categories {
		categoryIDs[i] = cat.ID
	}

	return &models.AppWithCategories{
		App:         app,
		CategoryIDs: categoryIDs,
	}, nil
}

// Create 创建应用
func (s *AppService) Create(app *models.App, categoryIDs []string) (*models.AppWithCategories, error) {
	// 生成 UUID
	app.ID = uuid.New().String()

	// 开始事务
	tx := database.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 创建应用
	if err := tx.Create(app).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	// 关联分类
	if len(categoryIDs) > 0 {
		if err := s.associateCategories(tx, app.ID, categoryIDs); err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	// 版本号自增
	if err := s.versionService.IncrementVersion(); err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	return &models.AppWithCategories{
		App:         *app,
		CategoryIDs: categoryIDs,
	}, nil
}

// Update 更新应用
func (s *AppService) Update(id string, updates map[string]interface{}, categoryIDs []string) (*models.AppWithCategories, error) {
	tx := database.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 更新应用基本信息
	if len(updates) > 0 {
		if err := tx.Model(&models.App{}).Where("id = ?", id).Updates(updates).Error; err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	// 更新分类关联
	if categoryIDs != nil {
		// 删除旧关联
		if err := tx.Exec("DELETE FROM app_categories WHERE app_id = ?", id).Error; err != nil {
			tx.Rollback()
			return nil, err
		}
		// 创建新关联
		if len(categoryIDs) > 0 {
			if err := s.associateCategories(tx, id, categoryIDs); err != nil {
				tx.Rollback()
				return nil, err
			}
		}
	}

	// 版本号自增
	if err := s.versionService.IncrementVersion(); err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	// 返回更新后的数据
	return s.GetByID(id)
}

// Delete 删除应用
func (s *AppService) Delete(id string) error {
	tx := database.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	var app models.App
	if err := tx.Where("id = ?", id).First(&app).Error; err != nil {
		tx.Rollback()
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("app not found")
		}
		return err
	}

	if err := tx.Delete(&app).Error; err != nil {
		tx.Rollback()
		return err
	}

	if err := s.versionService.IncrementVersion(); err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit().Error
}

// BatchDelete soft-deletes multiple apps by their IDs
func (s *AppService) BatchDelete(ids []string) error {
	if len(ids) == 0 {
		return fmt.Errorf("no app ids provided")
	}

	tx := database.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	result := tx.Where("id IN ?", ids).Delete(&models.App{})
	if result.Error != nil {
		tx.Rollback()
		return result.Error
	}

	if result.RowsAffected == 0 {
		tx.Rollback()
		return fmt.Errorf("no apps found with provided ids")
	}

	if err := s.versionService.IncrementVersion(); err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit().Error
}

// UpdateOrder 批量更新应用顺序
func (s *AppService) UpdateOrder(orders map[string]int) error {
	tx := database.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	for id, order := range orders {
		if err := tx.Model(&models.App{}).Where("id = ?", id).Update("order_num", order).Error; err != nil {
			tx.Rollback()
			return err
		}
	}

	// 版本号自增
	if err := s.versionService.IncrementVersion(); err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit().Error
}

// UpdateClickCount 更新点击统计
func (s *AppService) UpdateClickCount(appID string, count int) error {
	return database.DB.Model(&models.App{}).
		Where("id = ?", appID).
		UpdateColumn("click_count", gorm.Expr("click_count + ?", count)).
		Error
}

// UpdateTenantClickCount 更新租户维度的点击统计（upsert）。
// ID 必须每次都重新生成：MySQL 的 ON DUPLICATE KEY UPDATE 会在任意唯一键冲突时触发（不仅是
// tenant_id+app_id 这个复合索引），如果 ID 留空，第二个不同的 (tenant, app) 组合首次插入时会与
// 第一行在主键上冲突，从而错误地累加到别的租户/应用上。
func (s *AppService) UpdateTenantClickCount(tenantID, appID string, count int) error {
	row := models.TenantAppClick{
		ID:         uuid.New().String(),
		TenantID:   tenantID,
		AppID:      appID,
		ClickCount: count,
	}
	return database.DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "tenant_id"}, {Name: "app_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"click_count": gorm.Expr("click_count + ?", count),
		}),
	}).Create(&row).Error
}

// associateCategories 关联分类（内部方法）
func (s *AppService) associateCategories(tx *gorm.DB, appID string, categoryIDs []string) error {
	for _, catID := range categoryIDs {
		// 检查分类是否存在
		var count int64
		if err := tx.Model(&models.Category{}).Where("id = ?", catID).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return fmt.Errorf("category %s not found", catID)
		}

		// 创建关联
		if err := tx.Exec("INSERT INTO app_categories (app_id, category_id) VALUES (?, ?)", appID, catID).Error; err != nil {
			return err
		}
	}
	return nil
}

// FilterByCategory 根据分类筛选应用
func (s *AppService) FilterByCategory(categoryID string) ([]models.AppWithCategories, error) {
	var apps []models.App
	if err := database.DB.
		Joins("JOIN app_categories ON apps.id = app_categories.app_id").
		Where("app_categories.category_id = ?", categoryID).
		Preload("Categories").
		Find(&apps).Error; err != nil {
		return nil, err
	}

	result := make([]models.AppWithCategories, len(apps))
	for i, app := range apps {
		categoryIDs := make([]string, len(app.Categories))
		for j, cat := range app.Categories {
			categoryIDs[j] = cat.ID
		}
		result[i] = models.AppWithCategories{
			App:         app,
			CategoryIDs: categoryIDs,
		}
	}
	return result, nil
}

// GetDeleted returns all soft-deleted apps
func (s *AppService) GetDeleted() ([]models.AppWithCategories, error) {
	var apps []models.App
	if err := database.DB.Unscoped().Where("deleted_at IS NOT NULL").Preload("Categories").Order("deleted_at DESC").Find(&apps).Error; err != nil {
		return nil, err
	}

	result := make([]models.AppWithCategories, len(apps))
	for i, app := range apps {
		categoryIDs := make([]string, len(app.Categories))
		for j, cat := range app.Categories {
			categoryIDs[j] = cat.ID
		}
		result[i] = models.AppWithCategories{
			App:         app,
			CategoryIDs: categoryIDs,
		}
	}
	return result, nil
}

// Recover restores a soft-deleted app
func (s *AppService) Recover(id string) error {
	tx := database.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	result := tx.Unscoped().Model(&models.App{}).Where("id = ? AND deleted_at IS NOT NULL", id).Update("deleted_at", nil)
	if result.Error != nil {
		tx.Rollback()
		return result.Error
	}
	if result.RowsAffected == 0 {
		tx.Rollback()
		return fmt.Errorf("app not found or not deleted")
	}

	if err := s.versionService.IncrementVersion(); err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit().Error
}

// GetAllRankings 一次性返回热门榜（order_num ASC）、飙升榜（全局 click_count DESC），
// 以及地区榜（tenantID 对应的 tenant_app_clicks.click_count DESC；tenantID 为空时返回 nil）。
func (s *AppService) GetAllRankings(tenantID string) (hot, trending, regional []models.AppWithCategories, err error) {
	hot, err = s.getRankingsByOrder("order_num ASC, id ASC")
	if err != nil {
		return nil, nil, nil, err
	}

	trending, err = s.getRankingsByOrder("click_count DESC, id ASC")
	if err != nil {
		return nil, nil, nil, err
	}

	if tenantID == "" {
		return hot, trending, nil, nil
	}

	var apps []models.App
	if err = database.DB.
		Select("apps.*").
		Joins("JOIN tenant_app_clicks ON tenant_app_clicks.app_id = apps.id AND tenant_app_clicks.tenant_id = ?", tenantID).
		Preload("Categories").
		Order("tenant_app_clicks.click_count DESC, tenant_app_clicks.app_id ASC").
		Limit(8).
		Find(&apps).Error; err != nil {
		return nil, nil, nil, err
	}
	regional = appsToAppWithCategories(apps)

	return hot, trending, regional, nil
}

func (s *AppService) getRankingsByOrder(order string) ([]models.AppWithCategories, error) {
	var apps []models.App
	if err := database.DB.Preload("Categories").
		Order(order).
		Limit(8).
		Find(&apps).Error; err != nil {
		return nil, err
	}
	return appsToAppWithCategories(apps), nil
}

func appsToAppWithCategories(apps []models.App) []models.AppWithCategories {
	result := make([]models.AppWithCategories, len(apps))
	for i, app := range apps {
		categoryIDs := make([]string, len(app.Categories))
		for j, cat := range app.Categories {
			categoryIDs[j] = cat.ID
		}
		result[i] = models.AppWithCategories{
			App:         app,
			CategoryIDs: categoryIDs,
		}
	}
	return result
}

