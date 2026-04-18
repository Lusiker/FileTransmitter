package handler

import (
	"encoding/json"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/lusiker/filetransmitter/internal/service"
	"github.com/lusiker/filetransmitter/internal/ws"
)

type DeviceHandler struct {
	clientDeviceService *service.ClientDeviceService
	hub                 *ws.Hub
	relayService        *service.RelayService // 流式转发服务
}

func NewDeviceHandler(clientDeviceService *service.ClientDeviceService, hub *ws.Hub, relayService *service.RelayService) *DeviceHandler {
	h := &DeviceHandler{
		clientDeviceService: clientDeviceService,
		hub:                 hub,
		relayService:        relayService,
	}

	// 设置 WebSocket 消息处理回调
	if relayService != nil {
		hub.SetOnMessage(h.handleWSMessage)
	}

	return h
}

func (h *DeviceHandler) GetDevices(c *gin.Context) {
	role := c.Query("role")
	var devices []*service.ClientDevice

	if role == "sender" {
		devices = h.clientDeviceService.GetSenders()
	} else if role == "receiver" {
		devices = h.clientDeviceService.GetReceivers()
	} else {
		devices = h.clientDeviceService.GetAll()
	}

	// Convert to response format
	result := make([]map[string]interface{}, len(devices))
	for i, d := range devices {
		result[i] = map[string]interface{}{
			"id":        d.ID,
			"name":      d.Name,
			"role":      d.Role,
			"is_online": true,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"devices": result,
		"count":   len(result),
	})
}

// handleWSMessage 处理 WebSocket 消息
func (h *DeviceHandler) handleWSMessage(client *ws.Client, msg *ws.Message) {
	log.Printf("[WS Handler] Message from %s: type=%s", client.DeviceID, msg.Type)

	if h.relayService == nil {
		return
	}

	// 处理流式传输相关消息
	switch msg.Type {
	case ws.MessageTypeChunkData:
		// 解析分片数据
		var chunkData ws.ChunkData
		if err := parseMessageData(msg.Data, &chunkData); err != nil {
			log.Printf("[WS Handler] Failed to parse chunk_data: %v", err)
			return
		}
		// 处理分片
		err := h.relayService.HandleChunk(&chunkData)
		if err != nil {
			log.Printf("[WS Handler] Failed to handle chunk: %v", err)
		}

	case ws.MessageTypeChunkAck:
		// 解析 ACK
		var chunkAck ws.ChunkAck
		if err := parseMessageData(msg.Data, &chunkAck); err != nil {
			log.Printf("[WS Handler] Failed to parse chunk_ack: %v", err)
			return
		}
		// 处理 ACK
		err := h.relayService.HandleAck(&chunkAck)
		if err != nil {
			log.Printf("[WS Handler] Failed to handle ack: %v", err)
		}

	case ws.MessageTypeReceiverReady:
		// 解析接收端准备状态
		var receiverReady ws.ReceiverReady
		if err := parseMessageData(msg.Data, &receiverReady); err != nil {
			log.Printf("[WS Handler] Failed to parse receiver_ready: %v", err)
			return
		}
		// 处理接收端准备
		err := h.relayService.OnReceiverReady(receiverReady.SessionID, receiverReady.Platform, receiverReady.SupportsStreaming)
		if err != nil {
			log.Printf("[WS Handler] Failed to handle receiver_ready: %v", err)
		}
	}
}

// parseMessageData 解析消息数据
func parseMessageData(data interface{}, target interface{}) error {
	// 先转为 JSON bytes，再解析
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return json.Unmarshal(jsonBytes, target)
}

func (h *DeviceHandler) GetDevice(c *gin.Context) {
	deviceID := c.Param("id")
	device := h.clientDeviceService.Get(deviceID)
	if device == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "device not found"})
		return
	}
	c.JSON(http.StatusOK, map[string]interface{}{
		"id":        device.ID,
		"name":      device.Name,
		"role":      device.Role,
		"is_online": true,
	})
}

func (h *DeviceHandler) RemoveDevice(c *gin.Context) {
	deviceID := c.Param("id")
	h.clientDeviceService.Unregister(deviceID)
	c.JSON(http.StatusOK, gin.H{"message": "device removed"})
}

var wsUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all connections
	},
}

// getServerNetworkIP 获取服务器的局域网IP地址
func getServerNetworkIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}

	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return ""
}

// isValidIP 检查IP地址是否有效
func isValidIP(ip string) bool {
	if ip == "" || ip == "::" || ip == "0.0.0.0" || ip == "::1" || ip == "127.0.0.1" {
		return false
	}
	return net.ParseIP(ip) != nil
}

// formatClientIP 格式化客户端IP
func formatClientIP(c *gin.Context) string {
	// 尝试多种方式获取真实客户端IP
	clientIP := c.ClientIP()

	// 如果 ClientIP 返回无效值，尝试从 RemoteAddr 解析
	if !isValidIP(clientIP) || clientIP == "::" || clientIP == "0.0.0.0" {
		// 从 RemoteAddr 解析 (格式: "IP:Port")
		remoteAddr := c.Request.RemoteAddr
		host, _, err := net.SplitHostPort(remoteAddr)
		if err == nil && isValidIP(host) {
			clientIP = host
		}
	}

	// 如果是localhost，替换为服务器局域网IP
	if clientIP == "::1" || clientIP == "127.0.0.1" || clientIP == "localhost" {
		serverIP := getServerNetworkIP()
		if serverIP != "" {
			return serverIP
		}
	}

	// 如果仍然无效，返回默认值
	if !isValidIP(clientIP) {
		return "unknown"
	}

	return clientIP
}

func (h *DeviceHandler) HandleWebSocket(c *gin.Context) {
	deviceID := c.Query("device_id")
	deviceName := c.Query("name")
	deviceRole := c.Query("role")

	if deviceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "device_id required"})
		return
	}

	if deviceName == "" {
		deviceName = "Unknown"
	}
	if deviceRole == "" {
		deviceRole = "sender"
	}

	// Get client IP address
	clientIP := formatClientIP(c)

	conn, err := wsUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}

	// Create client
	client := ws.NewClient("", deviceID, conn, h.hub)

	// Register in hub FIRST (this triggers onDeviceRegister callback)
	h.hub.Register(client)

	// Start client pumps BEFORE sending messages (ensure connection is ready)
	go client.WritePump()
	go client.ReadPump()

	// Small delay to ensure WritePump is ready
	time.Sleep(50 * time.Millisecond)

	// Register in client device service with IP
	h.clientDeviceService.Register(deviceID, deviceName, deviceRole, clientIP, client.ID)

	// Broadcast to others that this device is online
	h.clientDeviceService.BroadcastDeviceOnline(deviceID, deviceName, deviceRole, clientIP)

	// Send current device list to new client (excluding itself)
	h.clientDeviceService.SendDeviceListTo(deviceID)

	// Also send the client its own IP address
	h.hub.BroadcastMessageTo(deviceID, "client_ip", map[string]string{"ip": clientIP})
}

func RegisterDeviceRoutes(r *gin.RouterGroup, handler *DeviceHandler) {
	r.GET("/devices", handler.GetDevices)
	r.GET("/devices/:id", handler.GetDevice)
	r.DELETE("/devices/:id", handler.RemoveDevice)
	r.GET("/ws", handler.HandleWebSocket)
}