package protocol

import "encoding/json"

// MessageType constants
const (
	MessageTypeAnnounce = "announce"
	MessageTypeResponse = "response"
)

// DiscoveryMessage is the UDP broadcast message structure
type DiscoveryMessage struct {
	Type       string `json:"type"`
	DeviceID   string `json:"device_id"`
	DeviceName string `json:"device_name"`
	Role       string `json:"role"`
	HTTPPort   int    `json:"http_port"`
	IP         string `json:"ip"`
	Timestamp  int64  `json:"timestamp"`
}

// Encode encodes the message to JSON bytes
func (m *DiscoveryMessage) Encode() ([]byte, error) {
	return json.Marshal(m)
}

// Decode decodes JSON bytes to DiscoveryMessage
func DecodeDiscoveryMessage(data []byte) (*DiscoveryMessage, error) {
	var msg DiscoveryMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}