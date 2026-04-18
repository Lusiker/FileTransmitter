package model

import "time"

type Device struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Role         string    `json:"role"` // "sender" or "receiver"
	IP           string    `json:"ip"`
	HTTPPort     int       `json:"http_port"`
	LastSeen     time.Time `json:"last_seen"`
	IsOnline     bool      `json:"is_online"`
}

type DeviceRole string

const (
	RoleSender   DeviceRole = "sender"
	RoleReceiver DeviceRole = "receiver"
)