package ws

import "encoding/json"

// Message types for WebSocket communication
const (
	MessageTypeDeviceOnline    = "device_online"
	MessageTypeDeviceOffline   = "device_offline"
	MessageTypeSessionCreated  = "session_created"
	MessageTypeSessionAccepted = "session_accepted"
	MessageTypeTransferProgress = "transfer_progress"
	MessageTypeFileComplete    = "file_complete"
	MessageTypeFileFailed      = "file_failed"
	MessageTypeTransferComplete = "transfer_complete"
	MessageTypeTransferFailed  = "transfer_failed"
	MessageTypeError           = "error"

	// 流式传输消息类型
	MessageTypeChunkData       = "chunk_data"      // 分片数据（发送端→后端→接收端）
	MessageTypeChunkAck        = "chunk_ack"       // 分片确认（接收端→后端→发送端）
	MessageTypeFileReady       = "file_ready"      // 文件准备开始传输
	MessageTypeFileStart       = "file_start"      // 文件开始传输（通知接收端）
	MessageTypeFileEnd         = "file_end"        // 文件传输结束

	// 接收端状态
	MessageTypeReceiverReady   = "receiver_ready"  // 接收端准备好接收
	MessageTypeReceiverOffline = "receiver_offline" // 接收端离线
)

// TransferPhase represents the phase of file transfer
type TransferPhase string

const (
	PhaseUploading TransferPhase = "uploading"
	PhaseMerging   TransferPhase = "merging"
	PhaseDone      TransferPhase = "done"
)

// Message represents a WebSocket message
type Message struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

// Encode encodes the message to JSON
func (m *Message) Encode() ([]byte, error) {
	return json.Marshal(m)
}

// DecodeMessage decodes JSON to Message
func DecodeMessage(data []byte) (*Message, error) {
	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

// TransferProgressData represents progress update
type TransferProgressData struct {
	SessionID string        `json:"session_id"`
	FileID    string        `json:"file_id"`
	Bytes     int64         `json:"bytes"`
	Total     int64         `json:"total"`
	Percent   int           `json:"percent"`
	Phase     TransferPhase `json:"phase"` // uploading / merging / done
}

// FileStatusData represents file status update
type FileStatusData struct {
	SessionID string `json:"session_id"`
	FileID    string `json:"file_id"`
	Status    string `json:"status"`
	Error     string `json:"error,omitempty"`
}

// DeviceStatusData represents device status
type DeviceStatusData struct {
	DeviceID   string `json:"device_id"`
	DeviceName string `json:"device_name"`
	Role       string `json:"role"`
	IP         string `json:"ip"`
	HTTPPort   int    `json:"http_port"`
	IsOnline   bool   `json:"is_online"`
}

// ErrorData represents an error message
type ErrorData struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// ==================== 流式传输数据结构 ====================

// ChunkData 分片数据（通过 WebSocket 传输）
type ChunkData struct {
	SessionID   string `json:"session_id"`
	FileID      string `json:"file_id"`
	ChunkIndex  int    `json:"chunk_index"`
	TotalChunks int    `json:"total_chunks"`
	Data        string `json:"data"` // base64 编码的分片数据
	Size        int    `json:"size"` // 分片原始大小（字节）
}

// ChunkAck 分片确认
type ChunkAck struct {
	SessionID  string `json:"session_id"`
	FileID     string `json:"file_id"`
	ChunkIndex int    `json:"chunk_index"`
	Status     string `json:"status"` // "ok" | "error" | "retry"
}

// FileReady 文件准备开始传输（通知双方）
type FileReady struct {
	SessionID string     `json:"session_id"`
	FileID    string     `json:"file_id"`
	FileInfo  FileInfoWS `json:"file_info"`
}

// FileInfoWS WebSocket 传输的文件信息
type FileInfoWS struct {
	Name       string `json:"name"`
	Size       int64  `json:"size"`
	MimeType   string `json:"mime_type"`
	TotalChunks int   `json:"total_chunks"`
}

// FileStart 文件开始传输
type FileStart struct {
	SessionID string `json:"session_id"`
	FileID    string `json:"file_id"`
}

// FileEnd 文件传输结束
type FileEnd struct {
	SessionID string `json:"session_id"`
	FileID    string `json:"file_id"`
	Success   bool   `json:"success"`
	Error     string `json:"error,omitempty"`
}

// ReceiverReady 接收端准备好
type ReceiverReady struct {
	SessionID string `json:"session_id"`
	Platform  string `json:"platform"` // "chrome" | "safari" | "android" | "ios"
	SupportsStreaming bool `json:"supports_streaming"` // 是否支持流式接收
}

// ReceiverOffline 接收端离线
type ReceiverOffline struct {
	SessionID string `json:"session_id"`
}