package services

import (
	"fmt"
	"time"

	"appdeck/database"
	"appdeck/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ValidExportTables = []string{
	"apps", "categories", "plan_sets", "billing_plans",
	"global_packages", "system_config", "settings",
	"moonlight_hosts", "versions",
}

type ExportMetadata struct {
	Version    int       `json:"version"`
	ExportedAt time.Time `json:"exported_at"`
	Source     string    `json:"source"`
	Tables     []string  `json:"tables"`
}

type ExportData struct {
	Metadata       ExportMetadata                `json:"metadata"`
	Apps           []models.App                  `json:"apps,omitempty"`
	Categories     []models.Category             `json:"categories,omitempty"`
	AppCategories  []AppCategoryLink             `json:"app_categories,omitempty"`
	PlanSets       []models.PlanSet              `json:"plan_sets,omitempty"`
	BillingPlans   []models.BillingPlan          `json:"billing_plans,omitempty"`
	GlobalPackages []models.GlobalRechargePackage `json:"global_packages,omitempty"`
	SystemConfig   []models.SystemConfig         `json:"system_config,omitempty"`
	Settings       []models.Setting              `json:"settings,omitempty"`
	MoonlightHosts []MoonlightHostExport         `json:"moonlight_hosts,omitempty"`
	Versions       []models.Version              `json:"versions,omitempty"`
}

type AppCategoryLink struct {
	AppID      string `json:"app_id" gorm:"column:app_id"`
	CategoryID string `json:"category_id" gorm:"column:category_id"`
}

type MoonlightHostExport struct {
	ID      string `json:"id"`
	Host    string `json:"host"`
	Port    int    `json:"port"`
	Enabled bool   `json:"enabled"`
}

type TableResult struct {
	Created int `json:"created"`
	Updated int `json:"updated"`
}

type DataMigrationService struct {
	db *gorm.DB
}

func NewDataMigrationService() *DataMigrationService {
	return &DataMigrationService{db: database.DB}
}

func (s *DataMigrationService) ValidateTableKeys(tables []string) error {
	valid := make(map[string]bool)
	for _, t := range ValidExportTables {
		valid[t] = true
	}
	for _, t := range tables {
		if !valid[t] {
			return fmt.Errorf("invalid table key: %s", t)
		}
	}
	return nil
}

func (s *DataMigrationService) Export(tables []string) (*ExportData, error) {
	tableSet := make(map[string]bool)
	for _, t := range tables {
		tableSet[t] = true
	}

	data := &ExportData{
		Metadata: ExportMetadata{
			Version:    1,
			ExportedAt: time.Now().UTC(),
			Source:     "appdeck",
			Tables:     tables,
		},
	}

	if tableSet["categories"] {
		var categories []models.Category
		if err := s.db.Order("order_num").Find(&categories).Error; err != nil {
			return nil, fmt.Errorf("export categories: %w", err)
		}
		data.Categories = categories
	}

	if tableSet["apps"] {
		var apps []models.App
		if err := s.db.Order("order_num").Find(&apps).Error; err != nil {
			return nil, fmt.Errorf("export apps: %w", err)
		}
		data.Apps = apps

		var links []AppCategoryLink
		if err := s.db.Table("app_categories").
			Where("app_id IN (SELECT id FROM apps WHERE deleted_at IS NULL)").
			Find(&links).Error; err != nil {
			return nil, fmt.Errorf("export app_categories: %w", err)
		}
		data.AppCategories = links

		hasAppCats := false
		for _, t := range data.Metadata.Tables {
			if t == "app_categories" {
				hasAppCats = true
				break
			}
		}
		if !hasAppCats {
			data.Metadata.Tables = append(data.Metadata.Tables, "app_categories")
		}
	}

	if tableSet["plan_sets"] {
		var planSets []models.PlanSet
		if err := s.db.Find(&planSets).Error; err != nil {
			return nil, fmt.Errorf("export plan_sets: %w", err)
		}
		data.PlanSets = planSets
	}

	if tableSet["billing_plans"] {
		var plans []models.BillingPlan
		if err := s.db.Find(&plans).Error; err != nil {
			return nil, fmt.Errorf("export billing_plans: %w", err)
		}
		data.BillingPlans = plans
	}

	if tableSet["global_packages"] {
		var pkgs []models.GlobalRechargePackage
		if err := s.db.Find(&pkgs).Error; err != nil {
			return nil, fmt.Errorf("export global_packages: %w", err)
		}
		data.GlobalPackages = pkgs
	}

	if tableSet["system_config"] {
		var configs []models.SystemConfig
		if err := s.db.Find(&configs).Error; err != nil {
			return nil, fmt.Errorf("export system_config: %w", err)
		}
		data.SystemConfig = configs
	}

	if tableSet["settings"] {
		var settings []models.Setting
		if err := s.db.Where("deleted_at IS NULL").Find(&settings).Error; err != nil {
			return nil, fmt.Errorf("export settings: %w", err)
		}
		data.Settings = settings
	}

	if tableSet["moonlight_hosts"] {
		var hosts []models.MoonlightHost
		if err := s.db.Find(&hosts).Error; err != nil {
			return nil, fmt.Errorf("export moonlight_hosts: %w", err)
		}
		exports := make([]MoonlightHostExport, len(hosts))
		for i, h := range hosts {
			exports[i] = MoonlightHostExport{
				ID:      h.ID,
				Host:    h.Host,
				Port:    h.Port,
				Enabled: h.Enabled,
			}
		}
		data.MoonlightHosts = exports
	}

	if tableSet["versions"] {
		var versions []models.Version
		if err := s.db.Find(&versions).Error; err != nil {
			return nil, fmt.Errorf("export versions: %w", err)
		}
		data.Versions = versions
	}

	return data, nil
}

func (s *DataMigrationService) Import(data *ExportData) (map[string]*TableResult, error) {
	if data.Metadata.Version != 1 {
		return nil, fmt.Errorf("unsupported export format version: %d", data.Metadata.Version)
	}

	results := make(map[string]*TableResult)

	err := s.db.Transaction(func(tx *gorm.DB) error {
		if len(data.Categories) > 0 {
			r, err := upsertSlice(tx, data.Categories)
			if err != nil {
				return fmt.Errorf("import categories: %w", err)
			}
			results["categories"] = r
		}

		if len(data.SystemConfig) > 0 {
			r, err := upsertSlice(tx, data.SystemConfig)
			if err != nil {
				return fmt.Errorf("import system_config: %w", err)
			}
			results["system_config"] = r
		}

		if len(data.Settings) > 0 {
			r, err := upsertSlice(tx, data.Settings)
			if err != nil {
				return fmt.Errorf("import settings: %w", err)
			}
			results["settings"] = r
		}

		if len(data.Versions) > 0 {
			r, err := upsertSlice(tx, data.Versions)
			if err != nil {
				return fmt.Errorf("import versions: %w", err)
			}
			results["versions"] = r
		}

		if len(data.PlanSets) > 0 {
			r, err := upsertSlice(tx, data.PlanSets)
			if err != nil {
				return fmt.Errorf("import plan_sets: %w", err)
			}
			results["plan_sets"] = r
		}

		if len(data.BillingPlans) > 0 {
			r, err := upsertSlice(tx, data.BillingPlans)
			if err != nil {
				return fmt.Errorf("import billing_plans: %w", err)
			}
			results["billing_plans"] = r
		}

		if len(data.GlobalPackages) > 0 {
			r, err := upsertSlice(tx, data.GlobalPackages)
			if err != nil {
				return fmt.Errorf("import global_packages: %w", err)
			}
			results["global_packages"] = r
		}

		if len(data.Apps) > 0 {
			r, err := upsertSlice(tx, data.Apps)
			if err != nil {
				return fmt.Errorf("import apps: %w", err)
			}
			results["apps"] = r
		}

		if len(data.AppCategories) > 0 {
			appIDs := make([]string, len(data.Apps))
			for i, a := range data.Apps {
				appIDs[i] = a.ID
			}
			if len(appIDs) > 0 {
				if err := tx.Table("app_categories").Where("app_id IN ?", appIDs).Delete(nil).Error; err != nil {
					return fmt.Errorf("clear app_categories: %w", err)
				}
			}
			for _, link := range data.AppCategories {
				if err := tx.Table("app_categories").Create(&link).Error; err != nil {
					return fmt.Errorf("import app_categories: %w", err)
				}
			}
			results["app_categories"] = &TableResult{Created: len(data.AppCategories)}
		}

		if len(data.MoonlightHosts) > 0 {
			r := &TableResult{}
			for _, h := range data.MoonlightHosts {
				host := models.MoonlightHost{
					ID:      h.ID,
					Host:    h.Host,
					Port:    h.Port,
					Enabled: h.Enabled,
				}
				var existing models.MoonlightHost
				if tx.Where("id = ?", h.ID).First(&existing).Error == nil {
					if err := tx.Model(&existing).Updates(map[string]interface{}{
						"host":    h.Host,
						"port":    h.Port,
						"enabled": h.Enabled,
					}).Error; err != nil {
						return fmt.Errorf("update moonlight_host: %w", err)
					}
					r.Updated++
				} else {
					if err := tx.Create(&host).Error; err != nil {
						return fmt.Errorf("create moonlight_host: %w", err)
					}
					r.Created++
				}
			}
			results["moonlight_hosts"] = r
		}

		return nil
	})

	if err != nil {
		return nil, err
	}
	return results, nil
}

func upsertSlice[T any](tx *gorm.DB, items []T) (*TableResult, error) {
	r := &TableResult{}
	for _, item := range items {
		result := tx.Clauses(clause.OnConflict{UpdateAll: true}).Create(&item)
		if result.Error != nil {
			return nil, result.Error
		}
		if result.RowsAffected == 1 {
			r.Created++
		} else {
			r.Updated++
		}
	}
	return r, nil
}
