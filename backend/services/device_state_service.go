package services

import (
	"errors"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// DeviceState 设备状态
type DeviceState string

const (
	DeviceStateOffline  DeviceState = "offline"
	DeviceStateIdle     DeviceState = "idle"
	DeviceStateLoggedIn DeviceState = "logged_in"
	DeviceStateInUse    DeviceState = "in_use"
)

// DeviceInfo 设备信息
type DeviceInfo struct {
	HostID             string
	MachineCode        string
	TenantID           string
	DeviceName         string
	IP                 string
	InnerIPs           []string
	State              DeviceState
	BoundUserID        string
	ActiveSessionID    string
	WSConn             *websocket.Conn
	writeMu            sync.Mutex // protects WSConn.WriteJSON from concurrent writes
	ConnectedAt        time.Time
	LastPing           time.Time
	DisconnectedAt     *time.Time  // when WS disconnected, nil if connected
	GraceTimer         *time.Timer // grace period timer, nil if not running
	ExplicitDisconnect bool        // true = admin/H5 triggered, skip grace period
	MissedPongs        int
	ExpectedExpiryTime *time.Time  // when current session balance runs out
	ExpiryTimer        *time.Timer // one-shot timer firing at ExpectedExpiryTime, nil if not scheduled
	ExpiryGen          uint64      // bumped on every SetSessionExpiry call; lets a firing timer detect it's been superseded
	BatteryLevel       int         // 0-100, -1 = unknown
	ChargingState      string      // "charging", "discharging", "full", "unknown"
	CurrentAppName     string      // currently running app name (for admin visibility)
	DeviceTemp         float64     // CPU temperature in °C, -1 = unknown
}

// DeviceStateService 设备状态管理服务
type DeviceStateService struct {
	devices map[string]*DeviceInfo // key: "hostID:machineCode"
	mu      sync.RWMutex
}

var (
	deviceStateService *DeviceStateService
	deviceStateOnce    sync.Once
)

// GetDeviceStateService 获取设备状态服务单例
func GetDeviceStateService() *DeviceStateService {
	deviceStateOnce.Do(func() {
		deviceStateService = &DeviceStateService{
			devices: make(map[string]*DeviceInfo),
		}
	})
	return deviceStateService
}

// getDeviceKey 生成设备唯一键
func (s *DeviceStateService) getDeviceKey(hostID, machineCode string) string {
	return hostID + ":" + machineCode
}

// RegisterDevice 注册设备
func (s *DeviceStateService) RegisterDevice(hostID, machineCode, deviceName, ip string, conn *websocket.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := s.getDeviceKey(hostID, machineCode)
	now := time.Now()

	// Check if device already exists (reconnect case)
	if existing, exists := s.devices[key]; exists {
		// Cancel any pending grace timer
		if existing.GraceTimer != nil {
			existing.GraceTimer.Stop()
			existing.GraceTimer = nil
		}
		// NOTE: Do NOT close the old WSConn here. Closing it triggers
		// onFailure on the client, which schedules a reconnect, creating
		// an infinite reconnect loop. Instead, old goroutines detect they
		// are superseded via IsCurrentConn() on their next ping tick and
		// exit cleanly within 30 seconds.
		// Update connection and clear disconnect state
		existing.WSConn = conn
		existing.DisconnectedAt = nil
		existing.ExplicitDisconnect = false
		existing.MissedPongs = 0
		existing.LastPing = now
		existing.IP = ip
		if deviceName != "" {
			existing.DeviceName = deviceName
		}
		// Keep existing BoundUserID, ActiveSessionID, etc.
		return
	}

	// New device registration
	s.devices[key] = &DeviceInfo{
		HostID:        hostID,
		MachineCode:   machineCode,
		DeviceName:    deviceName,
		IP:            ip,
		State:         DeviceStateIdle,
		WSConn:        conn,
		ConnectedAt:   now,
		LastPing:      now,
		BatteryLevel:  -1,
		ChargingState: "unknown",
		DeviceTemp:    -1,
	}
}

// IsCurrentConn checks if the given connection is still the active one for the device.
// Used by WebSocket handler goroutines to detect when they've been superseded.
func (s *DeviceStateService) IsCurrentConn(hostID, machineCode string, conn *websocket.Conn) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := s.getDeviceKey(hostID, machineCode)
	device, exists := s.devices[key]
	if !exists {
		return false
	}
	return device.WSConn == conn
}

// GetDevice 获取设备信息
func (s *DeviceStateService) GetDevice(hostID, machineCode string) (*DeviceInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := s.getDeviceKey(hostID, machineCode)
	device, exists := s.devices[key]
	if !exists {
		return nil, errors.New("device not found")
	}
	return device, nil
}

// UpdateDeviceState 更新设备状态
func (s *DeviceStateService) UpdateDeviceState(hostID, machineCode string, state DeviceState) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := s.getDeviceKey(hostID, machineCode)
	device, exists := s.devices[key]
	if !exists {
		return errors.New("device not found")
	}

	device.State = state
	return nil
}

// BindUser 绑定用户到设备
func (s *DeviceStateService) BindUser(hostID, machineCode, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := s.getDeviceKey(hostID, machineCode)
	device, exists := s.devices[key]
	if !exists {
		return errors.New("device not found")
	}

	// 检查设备是否已绑定其他用户
	if device.BoundUserID != "" && device.BoundUserID != userID {
		return errors.New("device already bound to another user")
	}

	device.BoundUserID = userID
	return nil
}

// UnbindUser 解绑用户
func (s *DeviceStateService) UnbindUser(hostID, machineCode string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := s.getDeviceKey(hostID, machineCode)
	device, exists := s.devices[key]
	if !exists {
		return errors.New("device not found")
	}

	device.BoundUserID = ""
	device.ActiveSessionID = ""
	device.State = DeviceStateIdle
	device.ExpectedExpiryTime = nil
	device.MissedPongs = 0
	device.CurrentAppName = ""
	return nil
}

// SetActiveSession 设置设备的活跃会话ID
func (s *DeviceStateService) SetActiveSession(hostID, machineCode, sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := s.getDeviceKey(hostID, machineCode)
	device, exists := s.devices[key]
	if !exists {
		return errors.New("device not found")
	}

	device.ActiveSessionID = sessionID
	return nil
}

// UpdatePing 更新心跳时间
func (s *DeviceStateService) UpdatePing(hostID, machineCode string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := s.getDeviceKey(hostID, machineCode)
	device, exists := s.devices[key]
	if !exists {
		return errors.New("device not found")
	}

	device.LastPing = time.Now()
	return nil
}

// RemoveDevice 移除设备
func (s *DeviceStateService) RemoveDevice(hostID, machineCode string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := s.getDeviceKey(hostID, machineCode)
	delete(s.devices, key)
}

// SendMessage 发送消息到设备
func (s *DeviceStateService) SendMessage(hostID, machineCode string, msgType string, data interface{}) error {
	s.mu.RLock()
	device, exists := s.devices[s.getDeviceKey(hostID, machineCode)]
	s.mu.RUnlock()

	if !exists {
		return errors.New("device not found")
	}

	if device.WSConn == nil || device.DisconnectedAt != nil {
		return errors.New("device not connected")
	}

	message := map[string]interface{}{
		"type": msgType,
		"data": data,
	}

	device.writeMu.Lock()
	err := device.WSConn.WriteJSON(message)
	device.writeMu.Unlock()
	return err
}

// SendToAnyDevice sends a message to any one connected device.
// Returns the hostID and machineCode of the device that received the message,
// or an error if no devices are connected.
func (s *DeviceStateService) SendToAnyDevice(msgType string, data interface{}) (string, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, device := range s.devices {
		if device.WSConn == nil || device.DisconnectedAt != nil {
			continue
		}

		message := map[string]interface{}{
			"type": msgType,
			"data": data,
		}

		device.writeMu.Lock()
		err := device.WSConn.WriteJSON(message)
		device.writeMu.Unlock()
		if err != nil {
			continue
		}

		return device.HostID, device.MachineCode, nil
	}

	return "", "", errors.New("no connected devices")
}

// SendToAllDevices broadcasts a message to ALL connected devices.
// Returns the number of devices that received the message successfully.
func (s *DeviceStateService) SendToAllDevices(msgType string, data interface{}) int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sent := 0
	for _, device := range s.devices {
		if device.WSConn == nil || device.DisconnectedAt != nil {
			continue
		}

		message := map[string]interface{}{
			"type": msgType,
			"data": data,
		}

		device.writeMu.Lock()
		err := device.WSConn.WriteJSON(message)
		device.writeMu.Unlock()
		if err != nil {
			continue
		}
		sent++
	}

	return sent
}

// SendToDevicesByMachineCodes sends a message to all connected devices whose MachineCode is in the given set.
func (s *DeviceStateService) SendToDevicesByMachineCodes(machineCodes []string, msgType string, data interface{}) int {
	if len(machineCodes) == 0 {
		return 0
	}

	codeSet := make(map[string]struct{}, len(machineCodes))
	for _, mc := range machineCodes {
		codeSet[mc] = struct{}{}
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	sent := 0
	for _, device := range s.devices {
		if _, ok := codeSet[device.MachineCode]; !ok {
			continue
		}
		if device.WSConn == nil || device.DisconnectedAt != nil {
			continue
		}

		message := map[string]interface{}{
			"type": msgType,
			"data": data,
		}

		device.writeMu.Lock()
		err := device.WSConn.WriteJSON(message)
		device.writeMu.Unlock()
		if err != nil {
			continue
		}
		sent++
	}

	return sent
}

// GetDevicesByUser 获取用户绑定的所有设备
func (s *DeviceStateService) GetDevicesByUser(userID string) []*DeviceInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var devices []*DeviceInfo
	for _, device := range s.devices {
		if device.BoundUserID == userID {
			devices = append(devices, device)
		}
	}
	return devices
}

// GetAllDevices 获取所有设备
func (s *DeviceStateService) GetAllDevices() []*DeviceInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	devices := make([]*DeviceInfo, 0, len(s.devices))
	for _, device := range s.devices {
		devices = append(devices, device)
	}
	return devices
}

// UpdateDeviceRegistration updates device name and inner IPs under lock
func (s *DeviceStateService) UpdateDeviceRegistration(hostID, machineCode, deviceName string, innerIPs []string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := s.getDeviceKey(hostID, machineCode)
	device, exists := s.devices[key]
	if !exists {
		return
	}

	if deviceName != "" {
		device.DeviceName = deviceName
	}
	if len(innerIPs) > 0 {
		device.InnerIPs = innerIPs
	}
}

// MarkExplicitDisconnect flags the device so the WS defer block skips grace period
func (s *DeviceStateService) MarkExplicitDisconnect(hostID, machineCode string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := s.getDeviceKey(hostID, machineCode)
	device, exists := s.devices[key]
	if !exists {
		return
	}
	device.ExplicitDisconnect = true
}

// MarkDisconnected sets the disconnect timestamp and starts grace timer
func (s *DeviceStateService) MarkDisconnected(hostID, machineCode string, gracePeriod time.Duration, onExpire func()) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := s.getDeviceKey(hostID, machineCode)
	device, exists := s.devices[key]
	if !exists {
		return
	}

	now := time.Now()
	device.DisconnectedAt = &now

	// Start grace period timer
	device.GraceTimer = time.AfterFunc(gracePeriod, func() {
		s.mu.Lock()
		d, exists := s.devices[key]
		if !exists || d.GraceTimer == nil {
			s.mu.Unlock()
			return
		}
		d.GraceTimer = nil
		s.mu.Unlock()
		// onExpire calls RemoveDevice which needs s.mu — safe now that we released it
		if onExpire != nil {
			onExpire()
		}
	})
}

// CancelGraceTimer cancels pending session end if client reconnects
// Returns true if a grace timer was cancelled (device was reconnecting)
func (s *DeviceStateService) CancelGraceTimer(hostID, machineCode string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := s.getDeviceKey(hostID, machineCode)
	device, exists := s.devices[key]
	if !exists {
		return false
	}

	if device.GraceTimer != nil {
		device.GraceTimer.Stop()
		device.GraceTimer = nil
		device.DisconnectedAt = nil
		return true
	}

	return false
}

// IsReconnect checks if this device was recently disconnected (within grace period)
func (s *DeviceStateService) IsReconnect(hostID, machineCode string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := s.getDeviceKey(hostID, machineCode)
	device, exists := s.devices[key]
	if !exists {
		return false
	}

	return device.DisconnectedAt != nil
}

// IncrementMissedPongs increments missed pong count after sending a ping
func (s *DeviceStateService) IncrementMissedPongs(hostID, machineCode string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := s.getDeviceKey(hostID, machineCode)
	device, exists := s.devices[key]
	if !exists {
		return 0
	}
	device.MissedPongs++
	return device.MissedPongs
}

// ResetMissedPongs resets missed pong count on pong received
func (s *DeviceStateService) ResetMissedPongs(hostID, machineCode string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := s.getDeviceKey(hostID, machineCode)
	device, exists := s.devices[key]
	if !exists {
		return
	}
	device.MissedPongs = 0
}

// UpdateBatteryInfo updates the battery level and charging state for a device
func (s *DeviceStateService) UpdateBatteryInfo(hostID, machineCode string, level int, chargingState string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := s.getDeviceKey(hostID, machineCode)
	device, exists := s.devices[key]
	if !exists {
		return
	}
	device.BatteryLevel = level
	device.ChargingState = chargingState
}

// UpdateDeviceTemp updates the CPU temperature for a device
func (s *DeviceStateService) UpdateDeviceTemp(hostID, machineCode string, temp float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := s.getDeviceKey(hostID, machineCode)
	device, exists := s.devices[key]
	if !exists {
		return
	}
	device.DeviceTemp = temp
}

// SetSessionExpiry sets the expected expiry time for a device's active session and
// (re)schedules a one-shot timer that fires onExpire at that instant. Any previously
// scheduled timer for this device is cancelled first, so calling this again with a new
// expiryTime is how callers push the cutoff earlier or later as balance changes.
//
// Cancellation is tracked via ExpiryGen rather than a nil-check on ExpiryTimer: Timer.Stop()
// does not guarantee a concurrently-firing timer's callback is suppressed (per the time
// package docs), so without an identity check a stale callback that loses that race could
// find ExpiryTimer already pointing at the newly-armed timer and wrongly clear it out --
// silently killing the new timer's tick chain since nothing else would then be armed to fire it.
func (s *DeviceStateService) SetSessionExpiry(hostID, machineCode string, expiryTime time.Time, onExpire func()) {
	s.mu.Lock()
	key := s.getDeviceKey(hostID, machineCode)
	device, exists := s.devices[key]
	if !exists {
		s.mu.Unlock()
		return
	}

	if device.ExpiryTimer != nil {
		device.ExpiryTimer.Stop()
		device.ExpiryTimer = nil
	}
	device.ExpectedExpiryTime = &expiryTime
	device.ExpiryGen++
	gen := device.ExpiryGen

	delay := time.Until(expiryTime)
	if delay < 0 {
		delay = 0
	}
	device.ExpiryTimer = time.AfterFunc(delay, func() {
		s.mu.Lock()
		d, exists := s.devices[key]
		if !exists || d.ExpiryGen != gen {
			s.mu.Unlock()
			return
		}
		d.ExpiryTimer = nil
		d.ExpectedExpiryTime = nil
		s.mu.Unlock()
		if onExpire != nil {
			onExpire()
		}
	})
	s.mu.Unlock()
}

// ClearSessionExpiry cancels any pending expiry timer (e.g., on session end)
func (s *DeviceStateService) ClearSessionExpiry(hostID, machineCode string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := s.getDeviceKey(hostID, machineCode)
	device, exists := s.devices[key]
	if !exists {
		return
	}
	if device.ExpiryTimer != nil {
		device.ExpiryTimer.Stop()
		device.ExpiryTimer = nil
	}
	device.ExpectedExpiryTime = nil
	// Bump so a timer callback that was already past Stop()'s race window (see
	// SetSessionExpiry) still recognizes it's been superseded and no-ops instead of
	// firing onExpire for a session that's in the middle of being ended.
	device.ExpiryGen++
}

// SetCurrentApp sets the currently running app name for admin visibility
func (s *DeviceStateService) SetCurrentApp(hostID, machineCode, appName string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := s.getDeviceKey(hostID, machineCode)
	device, exists := s.devices[key]
	if !exists {
		return
	}
	device.CurrentAppName = appName
}

// ClearCurrentApp clears the currently running app name
func (s *DeviceStateService) ClearCurrentApp(hostID, machineCode string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := s.getDeviceKey(hostID, machineCode)
	device, exists := s.devices[key]
	if !exists {
		return
	}
	device.CurrentAppName = ""
}
