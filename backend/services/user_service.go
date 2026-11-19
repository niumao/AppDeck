package services

import (
	"log"
	"strings"
	"time"

	"appdeck/database"
	"appdeck/models"

	"github.com/google/uuid"
)

// UserService 用户服务
type UserService struct{}

// NewUserService 创建用户服务
func NewUserService() *UserService {
	return &UserService{}
}

// FindOrCreateByOpenID 根据OpenID查找或创建用户
func (s *UserService) FindOrCreateByOpenID(openID, unionID, nickname, avatarURL string) (*models.User, bool, error) {
	var user models.User
	err := database.DB.Where("openid = ?", openID).First(&user).Error

	if err == nil {
		// 用户已存在，更新登录时间和用户信息
		now := time.Now()
		user.LastLoginAt = &now
		// Update nickname: only overwrite if provided nickname is better than existing
		// Allow real nicknames to replace placeholders, but don't replace real nicknames with placeholders
		isPlaceholder := func(n string) bool {
			return n == "" || n == "微信用户" || n == "WeChat User" || strings.HasPrefix(n, "用户")
		}
		oldNickname := user.Nickname
		if nickname != "" && (!isPlaceholder(nickname) || isPlaceholder(user.Nickname)) {
			user.Nickname = nickname
		}
		log.Printf("[NICKNAME-DEBUG] FindOrCreate existing user=%s: old=%q, incoming=%q, isPlaceholder(old)=%v, isPlaceholder(new)=%v → saved=%q",
			user.ID, oldNickname, nickname, isPlaceholder(oldNickname), isPlaceholder(nickname), user.Nickname)
		if avatarURL != "" {
			user.AvatarURL = avatarURL
		}
		if unionID != "" {
			user.UnionID = unionID
		}
		user.UpdatedAt = now
		database.DB.Save(&user)
		return &user, false, nil
	}

	// 创建新用户
	now := time.Now()
	user = models.User{
		ID:          uuid.New().String(),
		OpenID:      openID,
		UnionID:     unionID,
		Nickname:    nickname,
		AvatarURL:   avatarURL,
		Status:      1,
		CreatedAt:   now,
		UpdatedAt:   now,
		LastLoginAt: &now,
	}

	if err := database.DB.Create(&user).Error; err != nil {
		return nil, false, err
	}

	return &user, true, nil
}

// GetByID 根据ID获取用户
func (s *UserService) GetByID(id string) (*models.User, error) {
	var user models.User
	if err := database.DB.First(&user, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// GetByOpenID 根据OpenID获取用户
func (s *UserService) GetByOpenID(openID string) (*models.User, error) {
	var user models.User
	if err := database.DB.First(&user, "openid = ?", openID).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// GetAll 获取所有用户（分页）
func (s *UserService) GetAll(page, pageSize int) ([]models.User, int64, error) {
	var users []models.User
	var total int64

	if err := database.DB.Model(&models.User{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if err := database.DB.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&users).Error; err != nil {
		return nil, 0, err
	}

	return users, total, nil
}

// UpdateUserInfo 更新用户信息
func (s *UserService) UpdateUserInfo(userID, nickname, avatarURL string) error {
	updates := map[string]interface{}{
		"nickname":   nickname,
		"avatar_url": avatarURL,
		"updated_at": time.Now(),
	}
	return database.DB.Model(&models.User{}).Where("id = ?", userID).Updates(updates).Error
}

// UpdateStatus 更新用户状态
func (s *UserService) UpdateStatus(userID string, status int) error {
	return database.DB.Model(&models.User{}).Where("id = ?", userID).Update("status", status).Error
}

// UpdateAuthLevel 更新用户授权级别
func (s *UserService) UpdateAuthLevel(userID, authLevel string) error {
	updates := map[string]interface{}{
		"auth_level": authLevel,
		"updated_at": time.Now(),
	}
	return database.DB.Model(&models.User{}).Where("id = ?", userID).Updates(updates).Error
}

// Search 搜索用户
func (s *UserService) Search(keyword string, page, pageSize int) ([]models.User, int64, error) {
	var users []models.User
	var total int64

	query := database.DB.Model(&models.User{}).Where("nickname LIKE ?", "%"+keyword+"%")

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if err := query.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&users).Error; err != nil {
		return nil, 0, err
	}

	return users, total, nil
}
