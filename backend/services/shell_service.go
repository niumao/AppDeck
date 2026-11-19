package services

import (
	"fmt"
	"log"
	"sync"
	"time"

	"appdeck/database"
	"appdeck/models"

	"github.com/google/uuid"
)

type ShellResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

const DefaultShellHistoryLimit = 50

type ShellService struct {
	pending map[string]chan *ShellResult
	mu      sync.Mutex
}

var (
	shellService     *ShellService
	shellServiceOnce sync.Once
)

func GetShellService() *ShellService {
	shellServiceOnce.Do(func() {
		shellService = &ShellService{
			pending: make(map[string]chan *ShellResult),
		}
	})
	return shellService
}

func (s *ShellService) ExecuteRemoteCommand(hostID, machineCode, command, adminUser string) (*models.ShellLog, error) {
	// 1. Create ShellLog record (status=pending)
	shellLog := &models.ShellLog{
		ID:          uuid.New().String(),
		MachineCode: machineCode,
		Command:     command,
		Status:      "pending",
		ExitCode:    -1,
		ExecutedBy:  adminUser,
	}
	if err := database.DB.Create(shellLog).Error; err != nil {
		return nil, fmt.Errorf("failed to create shell log: %w", err)
	}

	// Prune old entries beyond the per-device history limit
	s.pruneHistory(machineCode, DefaultShellHistoryLimit)

	// 2. Register channel in pending map
	ch := make(chan *ShellResult, 1)
	s.mu.Lock()
	s.pending[shellLog.ID] = ch
	s.mu.Unlock()

	// 7. Cleanup: always delete channel from map
	defer func() {
		s.mu.Lock()
		delete(s.pending, shellLog.ID)
		s.mu.Unlock()
	}()

	// 3. Send shell_exec WS message
	deviceStateService := GetDeviceStateService()
	err := deviceStateService.SendMessage(hostID, machineCode, "shell_exec", map[string]interface{}{
		"command_id": shellLog.ID,
		"command":    command,
	})
	if err != nil {
		shellLog.Status = "error"
		shellLog.Stderr = "Failed to send command to device: " + err.Error()
		database.DB.Save(shellLog)
		return shellLog, fmt.Errorf("failed to send command to device: %w", err)
	}

	startTime := time.Now()

	// 4. Wait on channel with 30s timeout
	select {
	case result := <-ch:
		// 5. On result: update ShellLog in DB
		durationMs := int(time.Since(startTime).Milliseconds())
		shellLog.Stdout = result.Stdout
		shellLog.Stderr = result.Stderr
		shellLog.ExitCode = result.ExitCode
		shellLog.Status = "completed"
		shellLog.DurationMs = durationMs
		database.DB.Save(shellLog)
		return shellLog, nil

	case <-time.After(30 * time.Second):
		// 6. On timeout: update ShellLog status=timeout
		durationMs := int(time.Since(startTime).Milliseconds())
		shellLog.Status = "timeout"
		shellLog.Stderr = "Command timed out after 30 seconds"
		shellLog.DurationMs = durationMs
		database.DB.Save(shellLog)
		return shellLog, fmt.Errorf("command timed out after 30 seconds")
	}
}

func (s *ShellService) DeliverResult(commandID, stdout, stderr string, exitCode int) {
	s.mu.Lock()
	ch, exists := s.pending[commandID]
	s.mu.Unlock()

	if !exists {
		log.Printf("[shell] No pending channel for command_id=%s, ignoring result", commandID)
		return
	}

	// Non-blocking send
	select {
	case ch <- &ShellResult{
		Stdout:   stdout,
		Stderr:   stderr,
		ExitCode: exitCode,
	}:
	default:
		log.Printf("[shell] Channel full for command_id=%s, dropping result", commandID)
	}
}

func (s *ShellService) GetHistory(machineCode string, limit, offset int) ([]models.ShellLog, int64, error) {
	var logs []models.ShellLog
	var total int64

	db := database.DB.Model(&models.ShellLog{}).Where("machine_code = ?", machineCode)
	db.Count(&total)

	err := db.Order("created_at DESC").Limit(limit).Offset(offset).Find(&logs).Error
	return logs, total, err
}

// pruneHistory deletes the oldest shell logs for a device, keeping only the most recent `keep` entries.
func (s *ShellService) pruneHistory(machineCode string, keep int) {
	var count int64
	database.DB.Model(&models.ShellLog{}).Where("machine_code = ?", machineCode).Count(&count)
	if count <= int64(keep) {
		return
	}

	// Find the created_at cutoff: the Nth newest entry
	var cutoff models.ShellLog
	err := database.DB.Where("machine_code = ?", machineCode).
		Order("created_at DESC").Offset(keep).Limit(1).First(&cutoff).Error
	if err != nil {
		return
	}

	result := database.DB.Where("machine_code = ? AND created_at <= ?", machineCode, cutoff.CreatedAt).
		Delete(&models.ShellLog{})
	if result.RowsAffected > 0 {
		log.Printf("[shell] Pruned %d old shell logs for device %s (limit=%d)", result.RowsAffected, machineCode, keep)
	}
}
