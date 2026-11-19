package services

import (
	"errors"
	"strings"
	"time"

	"appdeck/database"
	"appdeck/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type RegisteredDeviceService struct{}

var registeredDeviceServiceInstance *RegisteredDeviceService

// NewRegisteredDeviceService returns the singleton registered device service instance
func NewRegisteredDeviceService() *RegisteredDeviceService {
	if registeredDeviceServiceInstance == nil {
		registeredDeviceServiceInstance = &RegisteredDeviceService{}
	}
	return registeredDeviceServiceInstance
}

// GetRegisteredDeviceService returns the singleton instance
func GetRegisteredDeviceService() *RegisteredDeviceService {
	return NewRegisteredDeviceService()
}

// CheckOrRegister checks if device exists and returns status, or creates pending record if new
func (s *RegisteredDeviceService) CheckOrRegister(machineCode, hostID, ip string) (string, error) {
	var device models.RegisteredDevice
	err := database.DB.Where("machine_code = ?", machineCode).First(&device).Error

	now := time.Now()

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Device doesn't exist - auto-create as pending
			newDevice := &models.RegisteredDevice{
				ID:            uuid.New().String(),
				MachineCode:   machineCode,
				HostID:        hostID,
				DeviceName:    "",
				Status:        "pending",
				IP:            ip,
				LastAttemptAt: &now,
			}

			if err := database.DB.Create(newDevice).Error; err != nil {
				return "", err
			}

			return "pending", nil
		}
		return "", err
	}

	// Device exists - update IP and LastAttemptAt
	updates := map[string]interface{}{
		"ip":              ip,
		"last_attempt_at": now,
	}

	if err := database.DB.Model(&device).Updates(updates).Error; err != nil {
		return "", err
	}

	return device.Status, nil
}

// EnsurePendingRecord creates a "pending" record for an unknown device so it
// appears in the admin panel for approval. If the device already exists (any
// status), it only updates IP and LastAttemptAt. This is called from the
// DeviceAuthMiddleware on every 403 rejection, so it must be idempotent.
func (s *RegisteredDeviceService) EnsurePendingRecord(machineCode, hostID, ip string) {
	if machineCode == "" {
		return
	}

	now := time.Now()
	var device models.RegisteredDevice
	err := database.DB.Where("machine_code = ?", machineCode).First(&device).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		newDevice := &models.RegisteredDevice{
			ID:            uuid.New().String(),
			MachineCode:   machineCode,
			HostID:        hostID,
			Status:        "pending",
			IP:            ip,
			LastAttemptAt: &now,
		}
		database.DB.Create(newDevice)
		return
	}

	if err == nil {
		database.DB.Model(&device).Updates(map[string]interface{}{
			"ip":              ip,
			"last_attempt_at": now,
		})
	}
}

// IsDeviceAllowed checks if device is approved (for WebSocket gate)
func (s *RegisteredDeviceService) IsDeviceAllowed(machineCode string) bool {
	var device models.RegisteredDevice
	err := database.DB.Where("machine_code = ? AND status = ?", machineCode, "approved").First(&device).Error
	return err == nil
}

// GetFreePlayByMachineCode checks if the device's tenant has free_play enabled
func (s *RegisteredDeviceService) GetFreePlayByMachineCode(machineCode string) bool {
	var device models.RegisteredDevice
	err := database.DB.Preload("Tenant").Where("machine_code = ? AND status = ?", machineCode, "approved").First(&device).Error
	if err != nil {
		return false
	}
	if device.Tenant == nil {
		return false
	}
	return device.Tenant.FreePlay == 1
}

// ListAll returns all registered devices with optional filters
func (s *RegisteredDeviceService) ListAll(search, statusFilter string) ([]models.RegisteredDevice, error) {
	var devices []models.RegisteredDevice
	query := database.DB.Preload("Tenant").Model(&models.RegisteredDevice{})

	// Apply search filter
	if search != "" {
		searchPattern := "%" + search + "%"
		query = query.Where("machine_code LIKE ? OR host_id LIKE ? OR device_name LIKE ? OR ip LIKE ?",
			searchPattern, searchPattern, searchPattern, searchPattern)
	}

	// Apply status filter
	if statusFilter != "" {
		query = query.Where("status = ?", statusFilter)
	}

	err := query.Order("created_at DESC").Find(&devices).Error
	if err != nil {
		return nil, err
	}

	return devices, nil
}

// Create creates a new registered device (manual add, status="approved")
func (s *RegisteredDeviceService) Create(machineCode, hostID, deviceName, remark string) (*models.RegisteredDevice, error) {
	// Validate machine_code is not empty
	if strings.TrimSpace(machineCode) == "" {
		return nil, errors.New("INVALID_MACHINE_CODE: machine_code cannot be empty")
	}

	// Check for duplicate machine_code
	var count int64
	err := database.DB.Model(&models.RegisteredDevice{}).
		Where("machine_code = ?", machineCode).
		Count(&count).Error
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, errors.New("DUPLICATE_MACHINE_CODE")
	}

	// Create new device with status="approved"
	newDevice := &models.RegisteredDevice{
		ID:          uuid.New().String(),
		MachineCode: machineCode,
		HostID:      hostID,
		DeviceName:  deviceName,
		Status:      "approved",
		Remark:      remark,
	}

	if err := database.DB.Create(newDevice).Error; err != nil {
		return nil, err
	}

	return newDevice, nil
}

// Approve sets device status to "approved"
func (s *RegisteredDeviceService) Approve(id string) error {
	result := database.DB.Model(&models.RegisteredDevice{}).
		Where("id = ?", id).
		Update("status", "approved")

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return errors.New("DEVICE_NOT_FOUND")
	}

	return nil
}

// Reject sets device status to "rejected"
func (s *RegisteredDeviceService) Reject(id string) error {
	result := database.DB.Model(&models.RegisteredDevice{}).
		Where("id = ?", id).
		Update("status", "rejected")

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return errors.New("DEVICE_NOT_FOUND")
	}

	return nil
}

// Update updates device_name and remark
func (s *RegisteredDeviceService) Update(id string, deviceName *string, remark *string) error {
	// Find the device
	var device models.RegisteredDevice
	err := database.DB.Where("id = ?", id).First(&device).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("DEVICE_NOT_FOUND")
		}
		return err
	}

	// Prepare updates
	updates := make(map[string]interface{})

	if deviceName != nil {
		updates["device_name"] = *deviceName
	}

	if remark != nil {
		updates["remark"] = *remark
	}

	// Apply updates
	if len(updates) > 0 {
		if err := database.DB.Model(&device).Updates(updates).Error; err != nil {
			return err
		}
	}

	return nil
}

// Delete deletes a registered device
func (s *RegisteredDeviceService) Delete(id string) error {
	result := database.DB.Where("id = ?", id).Delete(&models.RegisteredDevice{})

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return errors.New("DEVICE_NOT_FOUND")
	}

	return nil
}

// GetByID retrieves a registered device by ID
func (s *RegisteredDeviceService) GetByID(id string) (*models.RegisteredDevice, error) {
	var device models.RegisteredDevice
	err := database.DB.Where("id = ?", id).First(&device).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &device, nil
}

// GetDeviceName returns the configured device_name for a machine_code, or "" if unset/unknown
func (s *RegisteredDeviceService) GetDeviceName(machineCode string) string {
	var device models.RegisteredDevice
	if err := database.DB.Select("device_name").Where("machine_code = ?", machineCode).First(&device).Error; err != nil {
		return ""
	}
	return device.DeviceName
}

// GetDeviceNameMap returns a map of machine_code -> device_name for all registered devices
func (s *RegisteredDeviceService) GetDeviceNameMap() map[string]string {
	var devices []models.RegisteredDevice
	database.DB.Select("machine_code, device_name").Find(&devices)
	result := make(map[string]string, len(devices))
	for _, d := range devices {
		if d.DeviceName != "" {
			result[d.MachineCode] = d.DeviceName
		}
	}
	return result
}

// GetTenantIDByMachineCode returns the tenant_id for a device identified by machine_code
func (s *RegisteredDeviceService) GetTenantIDByMachineCode(machineCode string) string {
	var device models.RegisteredDevice
	err := database.DB.Select("tenant_id").Where("machine_code = ?", machineCode).First(&device).Error
	if err != nil || device.TenantID == nil {
		return ""
	}
	return *device.TenantID
}

// GetMachineCodesByTenantID returns all machine_codes for devices assigned to a tenant
func (s *RegisteredDeviceService) GetMachineCodesByTenantID(tenantID string) []string {
	var devices []models.RegisteredDevice
	database.DB.Select("machine_code").Where("tenant_id = ? AND status = ?", tenantID, "approved").Find(&devices)
	codes := make([]string, 0, len(devices))
	for _, d := range devices {
		codes = append(codes, d.MachineCode)
	}
	return codes
}

// AssignTenant assigns or unassigns a tenant to a device
func (s *RegisteredDeviceService) AssignTenant(deviceID string, tenantID *string) error {
	result := database.DB.Model(&models.RegisteredDevice{}).Where("id = ?", deviceID).
		Update("tenant_id", tenantID)
	if result.RowsAffected == 0 {
		return errors.New("DEVICE_NOT_FOUND")
	}
	return result.Error
}

// UpdateSchedule updates the power schedule fields for a device and returns the updated device
func (s *RegisteredDeviceService) UpdateSchedule(id string, shutdownEnabled bool, shutdownTime string, poweronEnabled bool, poweronTime string) (*models.RegisteredDevice, error) {
	var device models.RegisteredDevice
	err := database.DB.Where("id = ?", id).First(&device).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("DEVICE_NOT_FOUND")
		}
		return nil, err
	}

	updates := map[string]interface{}{
		"shutdown_enabled": shutdownEnabled,
		"shutdown_time":    shutdownTime,
		"poweron_enabled":  poweronEnabled,
		"poweron_time":     poweronTime,
	}

	if err := database.DB.Model(&device).Updates(updates).Error; err != nil {
		return nil, err
	}

	device.ShutdownEnabled = shutdownEnabled
	device.ShutdownTime = shutdownTime
	device.PoweronEnabled = poweronEnabled
	device.PoweronTime = poweronTime

	return &device, nil
}

// UpdateBypassCharge updates the bypass charge setting for a device
func (s *RegisteredDeviceService) UpdateBypassCharge(id string, enabled bool) (*models.RegisteredDevice, error) {
	var device models.RegisteredDevice
	err := database.DB.Where("id = ?", id).First(&device).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("DEVICE_NOT_FOUND")
		}
		return nil, err
	}

	if err := database.DB.Model(&device).Update("bypass_charge_enabled", enabled).Error; err != nil {
		return nil, err
	}

	device.BypassChargeEnabled = enabled
	return &device, nil
}

// GetScheduleByMachineCode returns the power schedule for a device by machine code
func (s *RegisteredDeviceService) GetScheduleByMachineCode(machineCode string) *models.RegisteredDevice {
	var device models.RegisteredDevice
	err := database.DB.Select("shutdown_enabled, shutdown_time, poweron_enabled, poweron_time, bypass_charge_enabled").
		Where("machine_code = ?", machineCode).First(&device).Error
	if err != nil {
		return nil
	}
	return &device
}
