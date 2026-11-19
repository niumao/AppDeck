package services

import (
	"appdeck/database"
	"appdeck/models"
	"errors"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type AdminService struct{}

func NewAdminService() *AdminService {
	return &AdminService{}
}

// FindByUsername 根据用户名查找管理员
func (s *AdminService) FindByUsername(username string) (*models.Admin, error) {
	var admin models.Admin
	err := database.DB.Where("username = ?", username).First(&admin).Error
	if err != nil {
		return nil, err
	}
	return &admin, nil
}

// FindByID 根据ID查找管理员
func (s *AdminService) FindByID(id string) (*models.Admin, error) {
	var admin models.Admin
	err := database.DB.Where("id = ?", id).First(&admin).Error
	if err != nil {
		return nil, err
	}
	return &admin, nil
}

// Create 创建管理员
func (s *AdminService) Create(username, password, displayName string) (*models.Admin, error) {
	// 检查用户名是否已存在
	var count int64
	database.DB.Model(&models.Admin{}).Where("username = ?", username).Count(&count)
	if count > 0 {
		return nil, errors.New("username already exists")
	}

	// 加密密码
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	admin := &models.Admin{
		ID:          uuid.New().String(),
		Username:    username,
		Password:    string(hashedPassword),
		DisplayName: displayName,
		Status:      1,
	}

	err = database.DB.Create(admin).Error
	if err != nil {
		return nil, err
	}

	return admin, nil
}

// VerifyPassword 验证密码
func (s *AdminService) VerifyPassword(admin *models.Admin, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(admin.Password), []byte(password))
	return err == nil
}
