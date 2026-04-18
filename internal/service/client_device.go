package service

import (
	"log"
	"sync"
	"time"

	"github.com/lusiker/filetransmitter/internal/model"
	"github.com/lusiker/filetransmitter/internal/ws"
)

// ClientDevice represents a browser client connected via WebSocket
type ClientDevice struct {
	ID        string
	Name      string
	Role      string // "sender" or "receiver"
	IP        string // Client's IP address
	ConnID    string // WebSocket connection ID
	Connected time.Time
	LastSeen  time.Time
}

// ClientDeviceService manages browser clients connected to the same backend
type ClientDeviceService struct {
	clients map[string]*ClientDevice // deviceID -> ClientDevice
	hub     *ws.Hub
	mu      sync.RWMutex
}

func NewClientDeviceService(hub *ws.Hub) *ClientDeviceService {
	return &ClientDeviceService{
		clients: make(map[string]*ClientDevice),
		hub:     hub,
	}
}

// Register adds a new browser client
func (s *ClientDeviceService) Register(deviceID, name, role, ip, connID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	client := &ClientDevice{
		ID:        deviceID,
		Name:      name,
		Role:      role,
		IP:        ip,
		ConnID:    connID,
		Connected: now,
		LastSeen:  now,
	}

	_, exists := s.clients[deviceID]
	s.clients[deviceID] = client

	if exists {
		log.Printf("[ClientDevice] Updated: %s (%s) IP: %s", name, role, ip)
	} else {
		log.Printf("[ClientDevice] Registered: %s (%s) - %s IP: %s", name, role, deviceID, ip)
	}
}

// Unregister removes a browser client and cancels its pending sessions
func (s *ClientDeviceService) Unregister(deviceID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	client, exists := s.clients[deviceID]
	if !exists {
		return
	}

	delete(s.clients, deviceID)
	log.Printf("[ClientDevice] Unregistered: %s", client.Name)

	// Broadcast offline status
	s.hub.BroadcastMessage(ws.MessageTypeDeviceOffline, ws.DeviceStatusData{
		DeviceID:   client.ID,
		DeviceName: client.Name,
		Role:       client.Role,
		IsOnline:   false,
	})
}

// CancelPendingSessions cancels all pending sessions for a device (called when device disconnects)
func (s *ClientDeviceService) CancelPendingSessions(deviceID string, sessionService *SessionService) {
	s.mu.RLock()
	client := s.clients[deviceID]
	s.mu.RUnlock()

	if client == nil {
		return
	}

	// Get all sessions and cancel pending ones where this device is sender
	sessions := sessionService.GetAll()
	for _, session := range sessions {
		if session.SenderID == deviceID && session.State == model.SessionStatePending {
			sessionService.Cancel(session.ID)
			log.Printf("[ClientDevice] Auto-cancelled pending session %s (sender disconnected)", session.ID)
		}
	}
}

// Get returns a specific client
func (s *ClientDeviceService) Get(deviceID string) *ClientDevice {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.clients[deviceID]
}

// GetAll returns all registered clients
func (s *ClientDeviceService) GetAll() []*ClientDevice {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*ClientDevice
	for _, client := range s.clients {
		result = append(result, client)
	}
	return result
}

// GetSenders returns all sender devices
func (s *ClientDeviceService) GetSenders() []*ClientDevice {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*ClientDevice
	for _, client := range s.clients {
		if client.Role == "sender" {
			result = append(result, client)
		}
	}
	return result
}

// GetReceivers returns all receiver devices
func (s *ClientDeviceService) GetReceivers() []*ClientDevice {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*ClientDevice
	for _, client := range s.clients {
		if client.Role == "receiver" {
			result = append(result, client)
		}
	}
	return result
}

// BroadcastDeviceOnline notifies all other clients that a new device is online
func (s *ClientDeviceService) BroadcastDeviceOnline(deviceID, name, role, ip string) {
	log.Printf("[ClientDevice] Broadcasting device_online: %s (%s) IP: %s", name, role, ip)
	s.hub.BroadcastMessageExcept(deviceID, ws.MessageTypeDeviceOnline, ws.DeviceStatusData{
		DeviceID:   deviceID,
		DeviceName: name,
		Role:       role,
		IP:         ip,
		IsOnline:   true,
	})
}

// BroadcastDeviceOffline notifies all clients that a device went offline
func (s *ClientDeviceService) BroadcastDeviceOffline(deviceID string) {
	s.mu.RLock()
	client := s.clients[deviceID]
	s.mu.RUnlock()

	if client != nil {
		s.hub.BroadcastMessage(ws.MessageTypeDeviceOffline, ws.DeviceStatusData{
			DeviceID:   client.ID,
			DeviceName: client.Name,
			Role:       client.Role,
			IP:         client.IP,
			IsOnline:   false,
		})
	}
}

// SendDeviceListTo sends the current device list to a specific client (excluding itself)
func (s *ClientDeviceService) SendDeviceListTo(deviceID string) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var devices []ws.DeviceStatusData
	for id, client := range s.clients {
		if id != deviceID {
			devices = append(devices, ws.DeviceStatusData{
				DeviceID:   client.ID,
				DeviceName: client.Name,
				Role:       client.Role,
				IP:         client.IP,
				IsOnline:   true,
			})
		}
	}

	log.Printf("[ClientDevice] Sending device list to %s: %d devices", deviceID, len(devices))
	s.hub.BroadcastMessageTo(deviceID, "device_list", devices)
}