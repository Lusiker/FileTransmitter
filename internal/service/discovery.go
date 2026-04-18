package service

import (
	"context"
	"log"
	"net"
	"sync"
	"time"

	"github.com/lusiker/filetransmitter/internal/model"
	"github.com/lusiker/filetransmitter/internal/util"
	"github.com/lusiker/filetransmitter/pkg/protocol"
)

type DiscoveryService struct {
	port       int
	deviceID   string
	deviceName string
	role       string
	httpPort   int
	conn       *net.UDPConn
	devices    map[string]*model.Device
	mu         sync.RWMutex
	onDeviceChange func(device *model.Device, added bool)
	ctx        context.Context
	cancel     context.CancelFunc
}

type DiscoveryConfig struct {
	Port       int
	DeviceID   string
	DeviceName string
	Role       string
	HTTPPort   int
}

func NewDiscoveryService(cfg DiscoveryConfig) *DiscoveryService {
	ctx, cancel := context.WithCancel(context.Background())
	return &DiscoveryService{
		port:       cfg.Port,
		deviceID:   cfg.DeviceID,
		deviceName: cfg.DeviceName,
		role:       cfg.Role,
		httpPort:   cfg.HTTPPort,
		devices:    make(map[string]*model.Device),
		ctx:        ctx,
		cancel:     cancel,
	}
}

func (s *DiscoveryService) SetOnDeviceChange(fn func(device *model.Device, added bool)) {
	s.mu.Lock()
	s.onDeviceChange = fn
	s.mu.Unlock()
}

func (s *DiscoveryService) Start() error {
	addr := &net.UDPAddr{Port: s.port}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return err
	}
	s.conn = conn

	log.Printf("[Discovery] Service started on port %d", s.port)

	go s.broadcast()
	go s.listen()
	go s.cleanupStaleDevices()

	return nil
}

func (s *DiscoveryService) Stop() {
	s.cancel()
	if s.conn != nil {
		s.conn.Close()
	}
}

func (s *DiscoveryService) broadcast() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	broadcastAddr := &net.UDPAddr{
		IP:   net.ParseIP(util.GetBroadcastIP()),
		Port: s.port,
	}

	msg := protocol.DiscoveryMessage{
		Type:       protocol.MessageTypeAnnounce,
		DeviceID:   s.deviceID,
		DeviceName: s.deviceName,
		Role:       s.role,
		HTTPPort:   s.httpPort,
		IP:         util.GetLocalIP(),
		Timestamp:  time.Now().Unix(),
	}

	data, err := msg.Encode()
	if err != nil {
		log.Printf("[Discovery] Failed to encode message: %v", err)
		return
	}

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			msg.Timestamp = time.Now().Unix()
			data, _ = msg.Encode()
			s.conn.WriteToUDP(data, broadcastAddr)
		}
	}
}

func (s *DiscoveryService) listen() {
	buf := make([]byte, 1024)

	for {
		select {
		case <-s.ctx.Done():
			return
		default:
			n, addr, err := s.conn.ReadFromUDP(buf)
			if err != nil {
				if s.ctx.Err() != nil {
					return
				}
				log.Printf("[Discovery] Read error: %v", err)
				continue
			}

			msg, err := protocol.DecodeDiscoveryMessage(buf[:n])
			if err != nil {
				continue
			}

			// Ignore own messages
			if msg.DeviceID == s.deviceID {
				continue
			}

			s.handleMessage(msg, addr)
		}
	}
}

func (s *DiscoveryService) handleMessage(msg *protocol.DiscoveryMessage, addr *net.UDPAddr) {
	s.mu.Lock()
	defer s.mu.Unlock()

	device := &model.Device{
		ID:       msg.DeviceID,
		Name:     msg.DeviceName,
		Role:     msg.Role,
		IP:       msg.IP,
		HTTPPort: msg.HTTPPort,
		LastSeen: time.Now(),
		IsOnline: true,
	}

	_, exists := s.devices[msg.DeviceID]
	s.devices[msg.DeviceID] = device

	if s.onDeviceChange != nil {
		s.onDeviceChange(device, !exists)
	}

	if exists {
		log.Printf("[Discovery] Device updated: %s (%s) at %s:%d",
			device.Name, device.Role, device.IP, device.HTTPPort)
	} else {
		log.Printf("[Discovery] Device discovered: %s (%s) at %s:%d",
			device.Name, device.Role, device.IP, device.HTTPPort)
	}
}

func (s *DiscoveryService) cleanupStaleDevices() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.mu.Lock()
			now := time.Now()
			for id, device := range s.devices {
				if now.Sub(device.LastSeen) > 30*time.Second {
					device.IsOnline = false
					log.Printf("[Discovery] Device stale: %s", device.Name)
					if s.onDeviceChange != nil {
						s.onDeviceChange(device, false)
					}
					delete(s.devices, id)
				}
			}
			s.mu.Unlock()
		}
	}
}

func (s *DiscoveryService) GetDevices() []*model.Device {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var devices []*model.Device
	for _, d := range s.devices {
		devices = append(devices, d)
	}
	return devices
}

func (s *DiscoveryService) GetDevice(id string) *model.Device {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.devices[id]
}

func (s *DiscoveryService) RemoveDevice(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if device, exists := s.devices[id]; exists {
		delete(s.devices, id)
		log.Printf("[Discovery] Device removed: %s", device.Name)
		if s.onDeviceChange != nil {
			s.onDeviceChange(device, false)
		}
	}
}