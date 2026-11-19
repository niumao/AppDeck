package services

import (
	"appdeck/database"
	"appdeck/models"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type VersionService struct{}

func NewVersionService() *VersionService {
	return &VersionService{}
}

// GetCurrentVersion 获取当前数据版本
func (s *VersionService) GetCurrentVersion() (*models.Version, error) {
	var version models.Version
	err := database.DB.First(&version, 1).Error
	return &version, err
}

// IncrementVersion 版本号自增 (格式: YYMM.DD.x)
// 例如: 2512.03.1 表示 2025年12月03日的第1个版本
func (s *VersionService) IncrementVersion() error {
	now := time.Now()
	newVersionPrefix := fmt.Sprintf("%02d%02d.%02d",
		now.Year()%100, // YY: 2025 -> 25
		now.Month(),    // MM: 12
		now.Day())      // DD: 03

	var currentVersion models.Version
	if err := database.DB.First(&currentVersion, 1).Error; err != nil {
		// 如果记录不存在，创建新的
		newVersion := models.Version{
			ID:      1,
			Version: newVersionPrefix + ".1",
		}
		return database.DB.Save(&newVersion).Error
	}

	// 解析当前版本
	parts := strings.Split(currentVersion.Version, ".")
	if len(parts) != 3 {
		// 格式错误，重置为新格式
		currentVersion.Version = newVersionPrefix + ".1"
		return database.DB.Save(&currentVersion).Error
	}

	currentDatePart := parts[0] + "." + parts[1] // YYMM.DD
	newDatePart := newVersionPrefix              // YYMM.DD

	var newVersion string
	if currentDatePart == newDatePart {
		// 同一天，自增计数器
		counter, err := strconv.Atoi(parts[2])
		if err != nil {
			counter = 0
		}
		newVersion = fmt.Sprintf("%s.%d", newDatePart, counter+1)
	} else {
		// 新的一天，重置计数器为 1
		newVersion = newDatePart + ".1"
	}

	currentVersion.Version = newVersion
	return database.DB.Save(&currentVersion).Error
}

