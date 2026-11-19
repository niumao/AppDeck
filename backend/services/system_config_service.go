package services

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"appdeck/database"
	"appdeck/models"

	"gorm.io/gorm"
)

type SystemConfigService struct{}

var systemConfigServiceInstance *SystemConfigService

// NewSystemConfigService returns the singleton system config service instance
func NewSystemConfigService() *SystemConfigService {
	if systemConfigServiceInstance == nil {
		systemConfigServiceInstance = &SystemConfigService{}
	}
	return systemConfigServiceInstance
}

// GetSystemConfigService returns the singleton instance
func GetSystemConfigService() *SystemConfigService {
	return NewSystemConfigService()
}

// GetConfigByKey retrieves a configuration by key
func (s *SystemConfigService) GetConfigByKey(key string) (*models.SystemConfig, error) {
	var config models.SystemConfig
	err := database.DB.Where("`key` = ?", key).First(&config).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil // Return nil for not found (not an error)
		}
		return nil, err
	}
	return &config, nil
}

// UpdateConfig creates or updates a configuration
func (s *SystemConfigService) UpdateConfig(key, value, description string) (*models.SystemConfig, error) {
	var config models.SystemConfig
	err := database.DB.Where("`key` = ?", key).First(&config).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Create new config
			config = models.SystemConfig{
				Key:         key,
				Value:       value,
				Description: description,
			}
			if err := database.DB.Create(&config).Error; err != nil {
				return nil, err
			}
			return &config, nil
		}
		return nil, err
	}

	// Update existing config
	config.Value = value
	if description != "" {
		config.Description = description
	}
	if err := database.DB.Save(&config).Error; err != nil {
		return nil, err
	}

	return &config, nil
}

// GetMoonlightHost retrieves the Moonlight host configuration
func (s *SystemConfigService) GetMoonlightHost() (*models.SystemConfig, error) {
	return s.GetConfigByKey("moonlight_host")
}

// UpdateMoonlightHost updates the Moonlight host configuration with validation
func (s *SystemConfigService) UpdateMoonlightHost(value string) (*models.SystemConfig, error) {
	// Validate the value
	if err := s.ValidateMoonlightHost(value); err != nil {
		return nil, err
	}

	return s.UpdateConfig("moonlight_host", value, "Moonlight server IP or hostname")
}

// ValidateMoonlightHost validates the Moonlight host value
func (s *SystemConfigService) ValidateMoonlightHost(value string) error {
	// Check non-empty
	if strings.TrimSpace(value) == "" {
		return errors.New("INVALID_HOSTNAME: value cannot be empty")
	}

	// Check max length
	if len(value) > 255 {
		return errors.New("INVALID_HOSTNAME: value exceeds 255 characters")
	}

	// Check for protocol prefix
	if strings.HasPrefix(strings.ToLower(value), "http://") ||
		strings.HasPrefix(strings.ToLower(value), "https://") {
		return errors.New("INVALID_HOSTNAME: protocol prefix not allowed")
	}

	// Check for trailing slash
	if strings.HasSuffix(value, "/") {
		return errors.New("INVALID_HOSTNAME: trailing slash not allowed")
	}

	// Check if it's a valid IPv4 or hostname
	if !s.isValidIPv4(value) && !s.isValidHostname(value) {
		return errors.New("INVALID_HOSTNAME: must be a valid IPv4 address or hostname")
	}

	return nil
}

// isValidIPv4 checks if a string is a valid IPv4 address
func (s *SystemConfigService) isValidIPv4(value string) bool {
	// IPv4 pattern: 4 groups of 1-3 digits separated by dots
	ipv4Pattern := regexp.MustCompile(`^(\d{1,3}\.){3}\d{1,3}$`)
	if !ipv4Pattern.MatchString(value) {
		return false
	}

	// Validate each octet is 0-255
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
func (s *SystemConfigService) isValidHostname(value string) bool {
	// Hostname pattern: alphanumeric, dots, and hyphens
	hostnamePattern := regexp.MustCompile(`^[a-zA-Z0-9.-]+$`)
	if !hostnamePattern.MatchString(value) {
		return false
	}

	// Additional checks: no consecutive dots, no leading/trailing dots or hyphens
	if strings.Contains(value, "..") ||
		strings.HasPrefix(value, ".") ||
		strings.HasPrefix(value, "-") ||
		strings.HasSuffix(value, ".") ||
		strings.HasSuffix(value, "-") {
		return false
	}

	return true
}

// SeedDefaultMoonlightHost ensures default moonlight_host config exists
func (s *SystemConfigService) SeedDefaultMoonlightHost() error {
	config, err := s.GetMoonlightHost()
	if err != nil {
		return fmt.Errorf("failed to check moonlight_host existence: %w", err)
	}

	// Skip if config already exists
	if config != nil {
		return nil
	}

	// Create default config
	_, err = s.UpdateConfig(
		"moonlight_host",
		"127.0.0.1",
		"Moonlight server IP or hostname",
	)
	if err != nil {
		return fmt.Errorf("failed to seed moonlight_host: %w", err)
	}

	return nil
}

// GetMoonlightPort retrieves the Moonlight port configuration
func (s *SystemConfigService) GetMoonlightPort() (*models.SystemConfig, error) {
	return s.GetConfigByKey("moonlight_port")
}

// UpdateMoonlightPort updates the Moonlight port configuration with validation
func (s *SystemConfigService) UpdateMoonlightPort(value string) (*models.SystemConfig, error) {
	// Validate the value
	if err := s.ValidateMoonlightPort(value); err != nil {
		return nil, err
	}

	return s.UpdateConfig("moonlight_port", value, "Moonlight server port (empty = auto-detect)")
}

// ValidateMoonlightPort validates the Moonlight port value
func (s *SystemConfigService) ValidateMoonlightPort(value string) error {
	// Empty string is valid (auto-detect)
	if value == "" {
		return nil
	}

	// Try to parse as integer
	port, err := strconv.Atoi(value)
	if err != nil {
		return errors.New("INVALID_PORT: value must be empty string or integer")
	}

	// Check port range 1-65535
	if port < 1 || port > 65535 {
		return errors.New("INVALID_PORT: port must be between 1 and 65535")
	}

	return nil
}

// SeedDefaultMoonlightPort ensures default moonlight_port config exists
func (s *SystemConfigService) SeedDefaultMoonlightPort() error {
	config, err := s.GetMoonlightPort()
	if err != nil {
		return fmt.Errorf("failed to check moonlight_port existence: %w", err)
	}

	// Skip if config already exists
	if config != nil {
		return nil
	}

	// Create default config (empty string = auto-detect)
	_, err = s.UpdateConfig(
		"moonlight_port",
		"",
		"Moonlight server port (empty = auto-detect)",
	)
	if err != nil {
		return fmt.Errorf("failed to seed moonlight_port: %w", err)
	}

	return nil
}
