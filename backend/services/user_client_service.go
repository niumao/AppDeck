package services

import (
	"appdeck/database"
	"appdeck/models"
	"errors"
	"time"

	"github.com/google/uuid"
)

type UserClientService struct{}

func NewUserClientService() *UserClientService {
	return &UserClientService{}
}

func (s *UserClientService) GetActiveClients(userID string) ([]models.UserClient, error) {
	var clients []models.UserClient
	err := database.DB.
		Where("user_id = ? AND status = 1", userID).
		Order("last_active_at DESC").
		Limit(3).
		Find(&clients).Error
	return clients, err
}

func (s *UserClientService) GetClientByID(id string) (*models.UserClient, error) {
	var client models.UserClient
	err := database.DB.Where("id = ?", id).First(&client).Error
	if err != nil {
		return nil, err
	}
	return &client, nil
}

func (s *UserClientService) CreateOrUpdateClient(userID, name, deviceID string) (string, error) {
	var client models.UserClient
	err := database.DB.Where("user_id = ? AND device_id = ?", userID, deviceID).First(&client).Error

	now := time.Now()
	if err != nil {
		client = models.UserClient{
			ID:           uuid.New().String(),
			UserID:       userID,
			Name:         name,
			DeviceID:     deviceID,
			LastActiveAt: &now,
			Status:       1,
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		err = database.DB.Create(&client).Error
		if err != nil {
			return "", err
		}
	} else {
		err = database.DB.Model(&client).Updates(map[string]interface{}{
			"name":           name,
			"last_active_at": now,
			"status":         1,
			"updated_at":     now,
		}).Error
		if err != nil {
			return "", err
		}
	}

	var count int64
	database.DB.Model(&models.UserClient{}).
		Where("user_id = ? AND status = 1", userID).
		Count(&count)

	if count > 3 {
		var oldestClient models.UserClient
		database.DB.
			Where("user_id = ? AND status = 1 AND id != ?", userID, client.ID).
			Order("last_active_at ASC").
			First(&oldestClient)
		database.DB.Model(&oldestClient).Updates(map[string]interface{}{
			"status":     0,
			"updated_at": now,
		})

		// End billing sessions for the evicted client
		sessionService := NewSessionService()
		sessionService.EndSessionsByClientID(oldestClient.ID, "client_evicted")
	}

	return client.ID, nil
}

func (s *UserClientService) QuitClient(id, userID string) error {
	var client models.UserClient
	err := database.DB.Where("id = ? AND user_id = ?", id, userID).First(&client).Error
	if err != nil {
		return errors.New("客户端不存在或无权限")
	}

	// NOTE: Session ending is handled by the caller (handler) via EndSessionsByDevice
	// which properly handles incremental billing and balance deduction.
	// The old logic here would call EndSession which does NOT deduct balance.

	return database.DB.Model(&client).Updates(map[string]interface{}{
		"status":     0,
		"updated_at": time.Now(),
	}).Error
}

// GetAllClientsWithDetails gets all clients with user and plan details (admin)
func (s *UserClientService) GetAllClientsWithDetails() ([]models.UserClient, error) {
	var clients []models.UserClient
	err := database.DB.
		Preload("User").
		Preload("Plan").
		Order("created_at DESC").
		Find(&clients).Error
	return clients, err
}

// CreateClientAssociation creates a new client association (admin)
func (s *UserClientService) CreateClientAssociation(userID, machineID string, planID *string, steamID string) (string, error) {
	now := time.Now()
	client := models.UserClient{
		ID:           uuid.New().String(),
		UserID:       userID,
		MachineID:    machineID,
		DeviceID:     machineID,
		Name:         "Client-" + machineID[:8],
		PlanID:       planID,
		SteamID:      steamID,
		LastActiveAt: &now,
		Status:       1,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	err := database.DB.Create(&client).Error
	if err != nil {
		return "", err
	}

	return client.ID, nil
}

// CreateOrUpdateClientWithHostInfo creates or updates client with host_id and machine_code
func (s *UserClientService) CreateOrUpdateClientWithHostInfo(userID, deviceName, hostID, machineCode string) (string, error) {
	var client models.UserClient
	err := database.DB.Where("user_id = ? AND host_id = ? AND machine_code = ?", userID, hostID, machineCode).First(&client).Error

	now := time.Now()
	if err != nil {
		// Create new client
		client = models.UserClient{
			ID:           uuid.New().String(),
			UserID:       userID,
			Name:         deviceName,
			HostID:       hostID,
			MachineCode:  machineCode,
			MachineID:    machineCode, // Use machine_code as machine_id
			DeviceID:     hostID,      // Use host_id as device_id
			LastActiveAt: &now,
			Status:       1,
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		err = database.DB.Create(&client).Error
		if err != nil {
			return "", err
		}
	} else {
		// Update existing client
		err = database.DB.Model(&client).Updates(map[string]interface{}{
			"name":           deviceName,
			"last_active_at": now,
			"status":         1,
			"updated_at":     now,
		}).Error
		if err != nil {
			return "", err
		}
	}

	// Check and revoke oldest client if count > 3
	var count int64
	database.DB.Model(&models.UserClient{}).
		Where("user_id = ? AND status = 1", userID).
		Count(&count)

	if count > 3 {
		var oldestClient models.UserClient
		database.DB.
			Where("user_id = ? AND status = 1 AND id != ?", userID, client.ID).
			Order("last_active_at ASC").
			First(&oldestClient)
		database.DB.Model(&oldestClient).Updates(map[string]interface{}{
			"status":     0,
			"updated_at": now,
		})

		// End billing sessions for the evicted client
		sessionService := NewSessionService()
		sessionService.EndSessionsByClientID(oldestClient.ID, "client_evicted")

		if oldestClient.HostID != "" && oldestClient.MachineCode != "" {
			deviceStateService := GetDeviceStateService()
			deviceStateService.SetActiveSession(oldestClient.HostID, oldestClient.MachineCode, "")
			deviceStateService.SendMessage(oldestClient.HostID, oldestClient.MachineCode, "session_end", map[string]interface{}{
				"reason": "client_evicted",
			})
			deviceStateService.UnbindUser(oldestClient.HostID, oldestClient.MachineCode)
			deviceStateService.UpdateDeviceState(oldestClient.HostID, oldestClient.MachineCode, DeviceStateIdle)
			deviceStateService.SendMessage(oldestClient.HostID, oldestClient.MachineCode, "device_state_change", map[string]interface{}{
				"state": "idle",
			})
		}
	}

	return client.ID, nil
}

// DeactivateClientByHostInfo sets status=0 for the active UserClient matching host_id and machine_code
func (s *UserClientService) DeactivateClientByHostInfo(hostID, machineCode string) error {
	return database.DB.Model(&models.UserClient{}).
		Where("host_id = ? AND machine_code = ? AND status = 1", hostID, machineCode).
		Updates(map[string]interface{}{
			"status":     0,
			"updated_at": time.Now(),
		}).Error
}

// GetClientByHostInfo gets the currently active UserClient for a host_id/machine_code.
// Shared devices accumulate one UserClient row per user who has ever bound to them, so
// without the status filter and explicit ordering GORM's implicit primary-key (UUID) sort
// would return an arbitrary historical row instead of the one actually in use.
func (s *UserClientService) GetClientByHostInfo(hostID, machineCode string) (*models.UserClient, error) {
	var client models.UserClient
	err := database.DB.
		Preload("User").
		Preload("Plan").
		Where("host_id = ? AND machine_code = ? AND status = 1", hostID, machineCode).
		Order("last_active_at DESC").
		First(&client).Error
	if err != nil {
		return nil, err
	}
	return &client, nil
}
