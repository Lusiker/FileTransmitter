package ws

import (
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all connections for local network
	},
}

type Client struct {
	ID       string
	DeviceID string
	conn     *websocket.Conn
	send     chan []byte
	hub      *Hub
	mu       sync.Mutex

	// 消息处理回调（由外部设置）
	onMessage func(client *Client, msg *Message)
}

func NewClient(id, deviceID string, conn *websocket.Conn, hub *Hub) *Client {
	return &Client{
		ID:       id,
		DeviceID: deviceID,
		conn:     conn,
		send:     make(chan []byte, 256),
		hub:      hub,
		onMessage: nil,
	}
}

// SetOnMessage 设置消息处理回调
func (c *Client) SetOnMessage(callback func(client *Client, msg *Message)) {
	c.onMessage = callback
}

func (c *Client) ReadPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	// 增大消息限制以支持分片数据
	c.conn.SetReadLimit(10 * 1024 * 1024) // 10MB
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("[WS] Read error: %v", err)
			}
			break
		}

		// 解析消息
		msg, err := DecodeMessage(message)
		if err != nil {
			log.Printf("[WS] Failed to decode message: %v", err)
			continue
		}

		// 调用消息处理回调
		if c.onMessage != nil {
			c.onMessage(c, msg)
		} else {
			// 默认处理：打印日志
			log.Printf("[WS] Received message from %s: type=%s", c.DeviceID, msg.Type)
		}
	}
}

func (c *Client) WritePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Batch messages
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) Send(message []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()

	select {
	case c.send <- message:
	default:
		log.Printf("[WS] Client %s send buffer full", c.ID)
	}
}