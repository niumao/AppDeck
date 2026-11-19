package services

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
)

type CaptureResult struct {
	ImageBase64 string
	Display     string
	Width       int
	Height      int
	Error       string
}

type ScreenCaptureService struct {
	pending map[string]chan *CaptureResult
	mu      sync.Mutex
}

var (
	screenCaptureService     *ScreenCaptureService
	screenCaptureServiceOnce sync.Once
)

func GetScreenCaptureService() *ScreenCaptureService {
	screenCaptureServiceOnce.Do(func() {
		screenCaptureService = &ScreenCaptureService{
			pending: make(map[string]chan *CaptureResult),
		}
	})
	return screenCaptureService
}

func (s *ScreenCaptureService) ExecuteCapture(hostID, machineCode, display string) (*CaptureResult, error) {
	commandID := uuid.New().String()

	ch := make(chan *CaptureResult, 1)
	s.mu.Lock()
	s.pending[commandID] = ch
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.pending, commandID)
		s.mu.Unlock()
	}()

	deviceStateService := GetDeviceStateService()
	err := deviceStateService.SendMessage(hostID, machineCode, "screen_capture", map[string]interface{}{
		"command_id": commandID,
		"display":    display,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to send screen_capture to device: %w", err)
	}

	select {
	case result := <-ch:
		if result.Error != "" {
			return result, fmt.Errorf("device error: %s", result.Error)
		}
		return result, nil
	case <-time.After(15 * time.Second):
		return nil, fmt.Errorf("screen capture timed out after 15 seconds")
	}
}

func (s *ScreenCaptureService) DeliverResult(commandID string, result *CaptureResult) {
	s.mu.Lock()
	ch, exists := s.pending[commandID]
	s.mu.Unlock()

	if !exists {
		log.Printf("[screen_capture] No pending channel for command_id=%s, ignoring result", commandID)
		return
	}

	select {
	case ch <- result:
	default:
		log.Printf("[screen_capture] Channel full for command_id=%s, dropping result", commandID)
	}
}
