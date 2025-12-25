package models

import (
	"time"
)

// PackageStatus 客户端包状态
type PackageStatus string

const (
	PackageStatusNone       PackageStatus = "none"
	PackageStatusGenerating PackageStatus = "generating"
	PackageStatusReady      PackageStatus = "ready"
	PackageStatusError      PackageStatus = "error"
)

// ClientPackage 客户端包版本模型
type ClientPackage struct {
	ID             int           `gorm:"column:id;primaryKey;default:1" json:"id"`
	PackageVersion int           `gorm:"column:package_version;not null;default:0" json:"packageVersion"`
	DBVersion      int           `gorm:"column:db_version;not null;default:0" json:"dbVersion"`
	GeneratedAt    *time.Time    `gorm:"column:generated_at" json:"generatedAt"`
	DBSize         int64         `gorm:"column:db_size;default:0" json:"dbSize"`
	ResourcesSize  int64         `gorm:"column:resources_size;default:0" json:"resourcesSize"`
	Status         PackageStatus `gorm:"column:status;type:enum('none','generating','ready','error');default:'none'" json:"status"`
}

// TableName 指定表名
func (ClientPackage) TableName() string {
	return "client_package"
}


