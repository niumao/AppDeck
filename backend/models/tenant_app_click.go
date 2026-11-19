package models

import "time"

// TenantAppClick 租户维度的应用点击统计
type TenantAppClick struct {
	ID         string    `gorm:"column:id;primaryKey;size:36;collate:utf8mb4_unicode_ci" json:"id"`
	TenantID   string    `gorm:"column:tenant_id;size:36;not null;uniqueIndex:idx_tenant_app;collate:utf8mb4_unicode_ci" json:"tenantId"`
	AppID      string    `gorm:"column:app_id;size:36;not null;uniqueIndex:idx_tenant_app;collate:utf8mb4_unicode_ci" json:"appId"`
	ClickCount int       `gorm:"column:click_count;default:0" json:"clickCount"`
	CreatedAt  time.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt  time.Time `gorm:"column:updated_at" json:"updatedAt"`
}

// TableName 指定表名
func (TenantAppClick) TableName() string {
	return "tenant_app_clicks"
}
