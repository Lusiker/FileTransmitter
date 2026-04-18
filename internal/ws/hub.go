package ws

import (
	"log"
	"sync"

	"github.com/google/uuid"
)

type Hub struct {
	clients       map[string]*Client // clientID -> Client
	deviceClients map[string]*Client // deviceID -> Client
	register      chan *Client
	unregister    chan *Client
	broadcast     chan []byte
	mu            sync.RWMutex
	// Callbacks
	onDeviceRegister   func(deviceID, clientID string)
	onDeviceUnregister func(deviceID string)
	// 消息处理回调
	onMessage func(client *Client, msg *Message)
}

func NewHub() *Hub {
	return &Hub{
		clients:       make(map[string]*Client),
		deviceClients: make(map[string]*Client),
		register:      make(chan *Client),
		unregister:    make(chan *Client),
		broadcast:     make(chan []byte, 256),
		onMessage:     nil,
	}
}

// SetOnMessage 设置消息处理回调
func (h *Hub) SetOnMessage(callback func(client *Client, msg *Message)) {
	h.onMessage = callback
}

func (h *Hub) SetOnDeviceRegister(fn func(deviceID, clientID string)) {
	h.onDeviceRegister = fn
}

func (h *Hub) SetOnDeviceUnregister(fn func(deviceID string)) {
	h.onDeviceUnregister = fn
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			if client.ID == "" {
				client.ID = uuid.New().String()
			}
			// 设置消息处理回调
			if h.onMessage != nil {
				client.SetOnMessage(h.onMessage)
			}
			h.clients[client.ID] = client
			h.deviceClients[client.DeviceID] = client
			h.mu.Unlock()
			log.Printf("[WS] Client registered: %s (device: %s)", client.ID, client.DeviceID)
			if h.onDeviceRegister != nil {
				h.onDeviceRegister(client.DeviceID, client.ID)
			}

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client.ID]; ok {
				delete(h.clients, client.ID)
				delete(h.deviceClients, client.DeviceID)
				close(client.send)
			}
			h.mu.Unlock()
			log.Printf("[WS] Client unregistered: %s (device: %s)", client.ID, client.DeviceID)
			if h.onDeviceUnregister != nil {
				h.onDeviceUnregister(client.DeviceID)
			}

		case message := <-h.broadcast:
			h.mu.RLock()
			for _, client := range h.deviceClients {
				client.Send(message)
			}
			h.mu.RUnlock()
		}
	}
}

func (h *Hub) Register(client *Client) {
	h.register <- client
}

func (h *Hub) Unregister(client *Client) {
	h.unregister <- client
}

func (h *Hub) Broadcast(message []byte) {
	h.broadcast <- message
}

func (h *Hub) BroadcastTo(deviceID string, message []byte) {
	h.mu.RLock()
	client := h.deviceClients[deviceID]
	h.mu.RUnlock()
	if client != nil {
		client.Send(message)
	}
}

func (h *Hub) BroadcastExcept(excludeDeviceID string, message []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for deviceID, client := range h.deviceClients {
		if deviceID != excludeDeviceID {
			client.Send(message)
		}
	}
}

// Helper functions
func (h *Hub) BroadcastMessage(msgType string, data interface{}) {
	msg := Message{Type: msgType, Data: data}
	dataBytes, _ := msg.Encode()
	h.Broadcast(dataBytes)
}

func (h *Hub) BroadcastMessageTo(deviceID string, msgType string, data interface{}) {
	msg := Message{Type: msgType, Data: data}
	dataBytes, _ := msg.Encode()
	h.BroadcastTo(deviceID, dataBytes)
}

func (h *Hub) BroadcastMessageToDevice(deviceID string, msgType string, data interface{}) {
	h.BroadcastMessageTo(deviceID, msgType, data)
}

func (h *Hub) BroadcastMessageExcept(excludeDeviceID string, msgType string, data interface{}) {
	msg := Message{Type: msgType, Data: data}
	dataBytes, _ := msg.Encode()
	h.BroadcastExcept(excludeDeviceID, dataBytes)
}

func (h *Hub) BroadcastMessageToDevices(deviceIDs []string, msgType string, data interface{}) {
	msg := Message{Type: msgType, Data: data}
	dataBytes, _ := msg.Encode()
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, deviceID := range deviceIDs {
		if client := h.deviceClients[deviceID]; client != nil {
			client.Send(dataBytes)
		}
	}
}