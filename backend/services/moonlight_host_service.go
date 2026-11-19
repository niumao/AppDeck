package services

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"appdeck/database"
	"appdeck/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type MoonlightHostService struct{}

var moonlightHostServiceInstance *MoonlightHostService

// NewMoonlightHostService returns the singleton moonlight host service instance
func NewMoonlightHostService() *MoonlightHostService {
	if moonlightHostServiceInstance == nil {
		moonlightHostServiceInstance = &MoonlightHostService{}
	}
	return moonlightHostServiceInstance
}

// GetMoonlightHostService returns the singleton instance
func GetMoonlightHostService() *MoonlightHostService {
	return NewMoonlightHostService()
}

// ListAll returns all moonlight hosts
func (s *MoonlightHostService) ListAll() ([]models.MoonlightHost, error) {
	var hosts []models.MoonlightHost
	err := database.DB.Order("created_at ASC").Find(&hosts).Error
	if err != nil {
		return nil, err
	}
	return hosts, nil
}

// Create creates a new moonlight host
func (s *MoonlightHostService) Create(host string, port int, sunshineUser, sunshinePass string) (*models.MoonlightHost, error) {
	// Validate host format
	if err := s.validateHost(host); err != nil {
		return nil, err
	}

	// Validate port range
	if port < 0 || port > 65535 {
		return nil, errors.New("INVALID_PORT: port must be between 0 and 65535")
	}

	// Check for duplicate host:port combination
	var count int64
	err := database.DB.Model(&models.MoonlightHost{}).
		Where("host = ? AND port = ?", host, port).
		Count(&count).Error
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, errors.New("DUPLICATE_HOST_PORT")
	}

	// Create new host
	newHost := &models.MoonlightHost{
		ID:           uuid.New().String(),
		Host:         host,
		Port:         port,
		Enabled:      true,
		Paired:       false,
		SunshineUser: sunshineUser,
		SunshinePass: sunshinePass,
	}

	if err := database.DB.Create(newHost).Error; err != nil {
		return nil, err
	}

	return newHost, nil
}

// Update updates an existing moonlight host
func (s *MoonlightHostService) Update(id string, host *string, port *int, enabled *bool, sunshineUser *string, sunshinePass *string) error {
	// Find the host
	var existingHost models.MoonlightHost
	err := database.DB.Where("id = ?", id).First(&existingHost).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("HOST_NOT_FOUND")
		}
		return err
	}

	// Check if host is occupied
	if existingHost.OccupiedByDeviceID != nil && (host != nil || port != nil) {
		return errors.New("HOST_OCCUPIED")
	}

	// Validate and apply updates
	updates := make(map[string]interface{})

	if host != nil {
		if err := s.validateHost(*host); err != nil {
			return err
		}

		// Check for duplicate host:port if changing host
		checkPort := existingHost.Port
		if port != nil {
			checkPort = *port
		}

		var count int64
		err := database.DB.Model(&models.MoonlightHost{}).
			Where("host = ? AND port = ? AND id != ?", *host, checkPort, id).
			Count(&count).Error
		if err != nil {
			return err
		}
		if count > 0 {
			return errors.New("DUPLICATE_HOST_PORT")
		}

		updates["host"] = *host
	}

	if port != nil {
		if *port < 0 || *port > 65535 {
			return errors.New("INVALID_PORT: port must be between 0 and 65535")
		}

		// Check for duplicate host:port if changing port
		checkHost := existingHost.Host
		if host != nil {
			checkHost = *host
		}

		var count int64
		err := database.DB.Model(&models.MoonlightHost{}).
			Where("host = ? AND port = ? AND id != ?", checkHost, *port, id).
			Count(&count).Error
		if err != nil {
			return err
		}
		if count > 0 {
			return errors.New("DUPLICATE_HOST_PORT")
		}

		updates["port"] = *port
	}

	if enabled != nil {
		updates["enabled"] = *enabled
	}

	if sunshineUser != nil {
		updates["sunshine_user"] = *sunshineUser
	}

	if sunshinePass != nil {
		updates["sunshine_pass"] = *sunshinePass
	}

	// Apply updates
	if len(updates) > 0 {
		if err := database.DB.Model(&existingHost).Updates(updates).Error; err != nil {
			return err
		}
	}

	return nil
}

// Delete deletes a moonlight host
func (s *MoonlightHostService) Delete(id string) error {
	// Find the host
	var host models.MoonlightHost
	err := database.DB.Where("id = ?", id).First(&host).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("HOST_NOT_FOUND")
		}
		return err
	}

	// Check if host is occupied
	if host.OccupiedByDeviceID != nil {
		return errors.New("HOST_OCCUPIED")
	}

	// Delete the host
	if err := database.DB.Delete(&host).Error; err != nil {
		return err
	}

	return nil
}

// GetByID retrieves a moonlight host by ID
func (s *MoonlightHostService) GetByID(id string) (*models.MoonlightHost, error) {
	var host models.MoonlightHost
	err := database.DB.Where("id = ?", id).First(&host).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &host, nil
}

// AcquireHost atomically acquires an available host for a device
func (s *MoonlightHostService) AcquireHost(deviceID string) (*models.MoonlightHost, error) {
	// Start transaction
	tx := database.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Check if device already has a host
	var existingHost models.MoonlightHost
	err := tx.Where("occupied_by_device_id = ?", deviceID).First(&existingHost).Error
	if err == nil {
		// Device already has a host, return it
		tx.Commit()
		return &existingHost, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		tx.Rollback()
		return nil, err
	}

	// Find an available host with row-level locking
	var host models.MoonlightHost
	err = tx.Where("enabled = ? AND occupied_by_device_id IS NULL", true).
		Order("created_at ASC").
		Clauses(clause.Locking{Strength: "UPDATE"}).
		First(&host).Error

	if err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("NO_AVAILABLE_HOSTS")
		}
		return nil, err
	}

	// Mark host as occupied
	now := time.Now()
	host.OccupiedByDeviceID = &deviceID
	host.OccupiedAt = &now

	if err := tx.Save(&host).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	return &host, nil
}

// ReleaseHost releases a host occupied by a device
func (s *MoonlightHostService) ReleaseHost(deviceID string) error {
	// Idempotent: update WHERE occupied_by_device_id = deviceID
	result := database.DB.Model(&models.MoonlightHost{}).
		Where("occupied_by_device_id = ?", deviceID).
		Updates(map[string]interface{}{
			"occupied_by_device_id": nil,
			"occupied_at":           nil,
		})

	if result.Error != nil {
		return result.Error
	}

	// Success even if no rows were affected (idempotent)
	return nil
}

// GetAssignedHost retrieves the host assigned to a device
func (s *MoonlightHostService) GetAssignedHost(deviceID string) (*models.MoonlightHost, error) {
	var host models.MoonlightHost
	err := database.DB.Where("occupied_by_device_id = ?", deviceID).First(&host).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &host, nil
}

// ForceRelease admin force releases a host by host ID
func (s *MoonlightHostService) ForceRelease(id string) error {
	result := database.DB.Model(&models.MoonlightHost{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"occupied_by_device_id": nil,
			"occupied_at":           nil,
		})

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return errors.New("HOST_NOT_FOUND")
	}

	return nil
}

// MarkPaired marks a host as paired or unpaired
func (s *MoonlightHostService) MarkPaired(id string, paired bool) error {
	result := database.DB.Model(&models.MoonlightHost{}).
		Where("id = ?", id).
		Update("paired", paired)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("HOST_NOT_FOUND")
	}
	return nil
}

// GetByIDWithCredentials retrieves a moonlight host by ID including credentials
func (s *MoonlightHostService) GetByIDWithCredentials(id string) (*models.MoonlightHost, error) {
	var host models.MoonlightHost
	err := database.DB.Where("id = ?", id).First(&host).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &host, nil
}

// ReleaseStaleHosts releases hosts that have been occupied for too long
func (s *MoonlightHostService) ReleaseStaleHosts(timeout time.Duration) error {
	cutoff := time.Now().Add(-timeout)

	result := database.DB.Model(&models.MoonlightHost{}).
		Where("occupied_at IS NOT NULL AND occupied_at < ?", cutoff).
		Updates(map[string]interface{}{
			"occupied_by_device_id": nil,
			"occupied_at":           nil,
		})

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected > 0 {
		fmt.Printf("Released %d stale moonlight hosts\n", result.RowsAffected)
	}

	return nil
}

// MigrateFromSystemConfig migrates old system_config moonlight_host/port to moonlight_hosts table
func (s *MoonlightHostService) MigrateFromSystemConfig(db *gorm.DB) error {
	// Check if moonlight_hosts already has data
	var count int64
	if err := db.Model(&models.MoonlightHost{}).Count(&count).Error; err != nil {
		return err
	}

	if count > 0 {
		// Already migrated, skip
		return nil
	}

	// Try to read old config
	var hostConfig models.SystemConfig
	err := db.Where("`key` = ?", "moonlight_host").First(&hostConfig).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// No old config, nothing to migrate
			return nil
		}
		return err
	}

	var portConfig models.SystemConfig
	err = db.Where("`key` = ?", "moonlight_port").First(&portConfig).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	// Parse port value
	port := 0
	if portConfig.Value != "" {
		parsedPort, err := strconv.Atoi(portConfig.Value)
		if err == nil && parsedPort > 0 && parsedPort <= 65535 {
			port = parsedPort
		}
	}

	// Create a new moonlight_hosts entry
	newHost := &models.MoonlightHost{
		ID:      uuid.New().String(),
		Host:    hostConfig.Value,
		Port:    port,
		Enabled: true,
	}

	if err := db.Create(newHost).Error; err != nil {
		return err
	}

	// Delete old system_config entries
	db.Where("`key` = ?", "moonlight_host").Delete(&models.SystemConfig{})
	db.Where("`key` = ?", "moonlight_port").Delete(&models.SystemConfig{})

	fmt.Println("Migrated moonlight_host/port from system_config to moonlight_hosts table")
	return nil
}

// validateHost validates the host format
func (s *MoonlightHostService) validateHost(value string) error {
	// Check non-empty
	if strings.TrimSpace(value) == "" {
		return errors.New("INVALID_HOST: value cannot be empty")
	}

	// Check max length
	if len(value) > 255 {
		return errors.New("INVALID_HOST: value exceeds 255 characters")
	}

	// Check for protocol prefix
	if strings.HasPrefix(strings.ToLower(value), "http://") ||
		strings.HasPrefix(strings.ToLower(value), "https://") {
		return errors.New("INVALID_HOST: protocol prefix not allowed")
	}

	// Check for trailing slash
	if strings.HasSuffix(value, "/") {
		return errors.New("INVALID_HOST: trailing slash not allowed")
	}

	// Check if it's a valid IPv4 or hostname
	if !s.isValidIPv4(value) && !s.isValidHostname(value) {
		return errors.New("INVALID_HOST: must be a valid IPv4 address or hostname")
	}

	return nil
}

// isValidIPv4 checks if a string is a valid IPv4 address
func (s *MoonlightHostService) isValidIPv4(value string) bool {
	ipv4Pattern := regexp.MustCompile(`^(\d{1,3}\.){3}\d{1,3}$`)
	if !ipv4Pattern.MatchString(value) {
		return false
	}

	parts := strings.Split(value, ".")
	for _, part := range parts {
		num, err := strconv.Atoi(part)
		if err != nil || num < 0 || num > 255 {
			return false
		}
	}

	return true
}

// isValidHostname checks if a string is a valid hostname
func (s *MoonlightHostService) isValidHostname(value string) bool {
	hostnamePattern := regexp.MustCompile(`^[a-zA-Z0-9.-]+$`)
	if !hostnamePattern.MatchString(value) {
		return false
	}

	if strings.Contains(value, "..") ||
		strings.HasPrefix(value, ".") ||
		strings.HasPrefix(value, "-") ||
		strings.HasSuffix(value, ".") ||
		strings.HasSuffix(value, "-") {
		return false
	}

	return true
}
