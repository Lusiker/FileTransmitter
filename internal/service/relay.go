package service

import (
	"encoding/base64"
	"log"
	"sync"

	"github.com/lusiker/filetransmitter/internal/config"
	"github.com/lusiker/filetransmitter/internal/model"
	"github.com/lusiker/filetransmitter/internal/ws"
)

// RelayService 流式转发服务
// 将发送端的分片直接转发给接收端，减少后端存储
type RelayService struct {
	hub            *ws.Hub
	sessionService *SessionService
	transferConfig *config.TransferConfig

	// 活跃的传输通道
	channels   map[string]*RelayChannel // sessionID -> channel
	channelsMu sync.RWMutex

	// 文件队列（滑动窗口）
	fileQueues   map[string]*FileQueue // sessionID -> queue
	fileQueuesMu sync.RWMutex

	// 分片缓存（接收端离线时的 fallback）
	chunkBuffer   map[string]map[int]string // sessionID/fileID -> chunkIndex -> base64 data
	chunkBufferMu sync.RWMutex

	// 最大并发文件数
	maxConcurrentFiles int

	// 最大缓存大小（字节）
	maxBufferSize int64
}

// RelayChannel 传输通道
type RelayChannel struct {
	SessionID      string
	SenderDeviceID string
	ReceiverDeviceID string
	ReceiverReady  bool
	ReceiverPlatform string // "chrome" | "safari" | "android" | "ios"
	SupportsStreaming bool  // 接收端是否支持流式接收

	// 当前传输的文件状态
	fileStreams   map[string]*FileStream // fileID -> stream
	fileStreamsMu sync.Mutex
}

// FileStream 文件流状态
type FileStream struct {
	FileID       string
	ChunkIndex   int // 当前等待确认的分片索引
	TotalChunks  int
	Status       string // "uploading" | "paused" | "complete" | "error"

	// 已确认的分片数
	ackedChunks int
}

// NewRelayService 创建新的转发服务
func NewRelayService(hub *ws.Hub, sessionService *SessionService, cfg *config.TransferConfig) *RelayService {
	return &RelayService{
		hub:                hub,
		sessionService:     sessionService,
		transferConfig:     cfg,
		channels:           make(map[string]*RelayChannel),
		fileQueues:         make(map[string]*FileQueue),
		chunkBuffer:        make(map[string]map[int]string),
		maxConcurrentFiles: 2, // 默认最多 2 个文件并发
		maxBufferSize:      100 * 1024 * 1024, // 默认最大缓存 100MB
	}
}

// CreateChannel 创建传输通道
func (r *RelayService) CreateChannel(sessionID, senderDeviceID, receiverDeviceID string) error {
	r.channelsMu.Lock()
	defer r.channelsMu.Unlock()

	// 如果已存在，直接返回
	if _, exists := r.channels[sessionID]; exists {
		return nil
	}

	channel := &RelayChannel{
		SessionID:        sessionID,
		SenderDeviceID:   senderDeviceID,
		ReceiverDeviceID: receiverDeviceID,
		ReceiverReady:    false,
		fileStreams:      make(map[string]*FileStream),
	}

	r.channels[sessionID] = channel

	// 创建文件队列
	r.fileQueuesMu.Lock()
	r.fileQueues[sessionID] = NewFileQueue(sessionID, r.maxConcurrentFiles)
	r.fileQueuesMu.Unlock()

	log.Printf("[Relay] Channel created: %s (sender: %s, receiver: %s)", sessionID, senderDeviceID, receiverDeviceID)

	return nil
}

// OnReceiverReady 接收端准备好
func (r *RelayService) OnReceiverReady(sessionID, platform string, supportsStreaming bool) error {
	r.channelsMu.Lock()
	channel, exists := r.channels[sessionID]
	if !exists {
		r.channelsMu.Unlock()
		return ErrChannelNotFound
	}
	r.channelsMu.Unlock()

	channel.ReceiverReady = true
	channel.ReceiverPlatform = platform
	channel.SupportsStreaming = supportsStreaming

	log.Printf("[Relay] Receiver ready: %s (platform: %s, streaming: %v)", sessionID, platform, supportsStreaming)

	// 如果不支持流式接收（Safari/iOS），通知发送端使用传统存储转发
	if !supportsStreaming {
		r.notifySenderUseFallback(sessionID)
		return nil
	}

	// 通知发送端可以开始传输
	r.notifySenderStartTransfer(sessionID)

	return nil
}

// OnReceiverOffline 接收端离线
func (r *RelayService) OnReceiverOffline(sessionID string) error {
	r.channelsMu.Lock()
	channel, exists := r.channels[sessionID]
	if !exists {
		r.channelsMu.Unlock()
		return ErrChannelNotFound
	}
	r.channelsMu.Unlock()

	channel.ReceiverReady = false

	log.Printf("[Relay] Receiver offline: %s", sessionID)

	// 通知发送端暂停传输
	r.notifySenderPauseTransfer(sessionID)

	return nil
}

// HandleChunk 处理发送端上传的分片
func (r *RelayService) HandleChunk(chunk *ws.ChunkData) error {
	r.channelsMu.RLock()
	channel, exists := r.channels[chunk.SessionID]
	r.channelsMu.RUnlock()

	if !exists {
		return ErrChannelNotFound
	}

	// 检查接收端状态
	if !channel.ReceiverReady || !channel.SupportsStreaming {
		// 接收端离线或不支持流式，缓存分片
		return r.bufferChunk(chunk)
	}

	// 直接转发给接收端
	return r.sendChunkToReceiver(channel, chunk)
}

// bufferChunk 缓存分片（fallback）
func (r *RelayService) bufferChunk(chunk *ws.ChunkData) error {
	key := chunk.SessionID + "/" + chunk.FileID

	r.chunkBufferMu.Lock()
	if r.chunkBuffer[key] == nil {
		r.chunkBuffer[key] = make(map[int]string)
	}
	r.chunkBuffer[key][chunk.ChunkIndex] = chunk.Data
	r.chunkBufferMu.Unlock()

	log.Printf("[Relay] Buffered chunk: %s/%s chunk %d", chunk.SessionID, chunk.FileID, chunk.ChunkIndex)

	return nil
}

// sendChunkToReceiver 发送分片给接收端
func (r *RelayService) sendChunkToReceiver(channel *RelayChannel, chunk *ws.ChunkData) error {
	// 通过 WebSocket 发送给接收端
	msg := ws.Message{
		Type: ws.MessageTypeChunkData,
		Data: chunk,
	}

	r.hub.BroadcastMessageToDevice(channel.ReceiverDeviceID, msg.Type, msg.Data)

	log.Printf("[Relay] Sent chunk to receiver: %s/%s chunk %d", chunk.SessionID, chunk.FileID, chunk.ChunkIndex)

	return nil
}

// HandleAck 处理接收端的确认
func (r *RelayService) HandleAck(ack *ws.ChunkAck) error {
	r.channelsMu.RLock()
	channel, exists := r.channels[ack.SessionID]
	r.channelsMu.RUnlock()

	if !exists {
		return ErrChannelNotFound
	}

	if ack.Status == "ok" {
		// 更新文件流状态
		channel.fileStreamsMu.Lock()
		stream, exists := channel.fileStreams[ack.FileID]
		if exists {
			stream.ackedChunks++
			stream.ChunkIndex = ack.ChunkIndex + 1
			log.Printf("[Relay] ACK received: %s/%s chunk %d, acked: %d/%d",
				ack.SessionID, ack.FileID, ack.ChunkIndex, stream.ackedChunks, stream.TotalChunks)
		}
		channel.fileStreamsMu.Unlock()

		// 通知发送端继续下一个分片
		r.notifySenderNextChunk(ack.SessionID, ack.FileID, ack.ChunkIndex+1)

		// 检查文件是否完成
		if stream != nil && stream.ackedChunks >= stream.TotalChunks {
			r.onFileComplete(ack.SessionID, ack.FileID)
		}
	} else if ack.Status == "error" {
		// 重传该分片
		log.Printf("[Relay] ACK error, need retry: %s/%s chunk %d", ack.SessionID, ack.FileID, ack.ChunkIndex)
	}

	return nil
}

// StartFile 开始传输文件
func (r *RelayService) StartFile(sessionID, fileID string, totalChunks int) error {
	r.channelsMu.RLock()
	channel, exists := r.channels[sessionID]
	r.channelsMu.RUnlock()

	if !exists {
		return ErrChannelNotFound
	}

	// 创建文件流
	channel.fileStreamsMu.Lock()
	channel.fileStreams[fileID] = &FileStream{
		FileID:      fileID,
		ChunkIndex:  0,
		TotalChunks: totalChunks,
		Status:      "uploading",
		ackedChunks: 0,
	}
	channel.fileStreamsMu.Unlock()

	// 标记文件开始
	r.fileQueuesMu.RLock()
	queue := r.fileQueues[sessionID]
	r.fileQueuesMu.RUnlock()

	if queue != nil {
		queue.StartFile(fileID)
	}

	// 通知接收端文件即将开始
	session := r.sessionService.Get(sessionID)
	if session != nil {
		var fileInfo *model.FileInfo
		for _, f := range session.Files {
			if f.ID == fileID {
				fileInfo = &f
				break
			}
		}

		if fileInfo != nil {
			r.hub.BroadcastMessageToDevice(channel.ReceiverDeviceID, ws.MessageTypeFileReady, ws.FileReady{
				SessionID: sessionID,
				FileID:    fileID,
				FileInfo: ws.FileInfoWS{
					Name:        fileInfo.Name,
					Size:        fileInfo.Size,
					MimeType:    fileInfo.MIMEType,
					TotalChunks: totalChunks,
				},
			})
		}
	}

	log.Printf("[Relay] File started: %s/%s (%d chunks)", sessionID, fileID, totalChunks)

	return nil
}

// onFileComplete 文件传输完成
func (r *RelayService) onFileComplete(sessionID, fileID string) {
	// 标记文件完成
	r.fileQueuesMu.RLock()
	queue := r.fileQueues[sessionID]
	r.fileQueuesMu.RUnlock()

	if queue != nil {
		queue.MarkComplete(fileID)
	}

	// 通知双方文件完成
	r.channelsMu.RLock()
	channel := r.channels[sessionID]
	r.channelsMu.RUnlock()

	if channel != nil {
		r.hub.BroadcastMessageToDevices([]string{channel.SenderDeviceID, channel.ReceiverDeviceID},
			ws.MessageTypeFileEnd, ws.FileEnd{
				SessionID: sessionID,
				FileID:    fileID,
				Success:   true,
			})
	}

	// 更新 session 服务
	r.sessionService.MarkFileComplete(sessionID, fileID)

	log.Printf("[Relay] File complete: %s/%s", sessionID, fileID)

	// 检查是否还有下一个文件
	nextFile := queue.GetNextFile()
	if nextFile != "" {
		// 通知发送端传输下一个文件
		r.notifySenderStartFile(sessionID, nextFile)
	}
}

// GetNextFile 获取下一个待传输文件
func (r *RelayService) GetNextFile(sessionID string) string {
	r.fileQueuesMu.RLock()
	queue := r.fileQueues[sessionID]
	r.fileQueuesMu.RUnlock()

	if queue == nil {
		return ""
	}

	return queue.GetNextFile()
}

// AddFileToQueue 添加文件到队列
func (r *RelayService) AddFileToQueue(sessionID, fileID string) {
	r.fileQueuesMu.RLock()
	queue := r.fileQueues[sessionID]
	r.fileQueuesMu.RUnlock()

	if queue != nil {
		queue.AddFile(fileID)
	}
}

// notifySenderStartTransfer 通知发送端开始传输
func (r *RelayService) notifySenderStartTransfer(sessionID string) {
	r.channelsMu.RLock()
	channel := r.channels[sessionID]
	r.channelsMu.RUnlock()

	if channel != nil {
		r.hub.BroadcastMessageToDevice(channel.SenderDeviceID, ws.MessageTypeReceiverReady,
			ws.ReceiverReady{
				SessionID:        sessionID,
				SupportsStreaming: true,
			})
	}
}

// notifySenderUseFallback 通知发送端使用存储转发
func (r *RelayService) notifySenderUseFallback(sessionID string) {
	r.channelsMu.RLock()
	channel := r.channels[sessionID]
	r.channelsMu.RUnlock()

	if channel != nil {
		r.hub.BroadcastMessageToDevice(channel.SenderDeviceID, ws.MessageTypeReceiverReady,
			ws.ReceiverReady{
				SessionID:        sessionID,
				SupportsStreaming: false,
			})
	}
}

// notifySenderPauseTransfer 通知发送端暂停传输
func (r *RelayService) notifySenderPauseTransfer(sessionID string) {
	r.channelsMu.RLock()
	channel := r.channels[sessionID]
	r.channelsMu.RUnlock()

	if channel != nil {
		r.hub.BroadcastMessageToDevice(channel.SenderDeviceID, ws.MessageTypeReceiverOffline,
			ws.ReceiverOffline{SessionID: sessionID})
	}
}

// notifySenderNextChunk 通知发送端发送下一个分片
func (r *RelayService) notifySenderNextChunk(sessionID, fileID string, nextChunkIndex int) {
	r.channelsMu.RLock()
	channel := r.channels[sessionID]
	r.channelsMu.RUnlock()

	if channel != nil {
		r.hub.BroadcastMessageToDevice(channel.SenderDeviceID, ws.MessageTypeChunkAck,
			ws.ChunkAck{
				SessionID:  sessionID,
				FileID:     fileID,
				ChunkIndex: nextChunkIndex,
				Status:     "ok",
			})
	}
}

// notifySenderStartFile 通知发送端开始传输下一个文件
func (r *RelayService) notifySenderStartFile(sessionID, fileID string) {
	r.channelsMu.RLock()
	channel := r.channels[sessionID]
	r.channelsMu.RUnlock()

	if channel != nil {
		r.hub.BroadcastMessageToDevice(channel.SenderDeviceID, ws.MessageTypeFileStart,
			ws.FileStart{
				SessionID: sessionID,
				FileID:    fileID,
			})
	}
}

// CloseChannel 关闭传输通道
func (r *RelayService) CloseChannel(sessionID string) {
	r.channelsMu.Lock()
	delete(r.channels, sessionID)
	r.channelsMu.Unlock()

	r.fileQueuesMu.Lock()
	delete(r.fileQueues, sessionID)
	r.fileQueuesMu.Unlock()

	r.chunkBufferMu.Lock()
	for key := range r.chunkBuffer {
		if len(key) > len(sessionID) && key[:len(sessionID)] == sessionID {
			delete(r.chunkBuffer, key)
		}
	}
	r.chunkBufferMu.Unlock()

	log.Printf("[Relay] Channel closed: %s", sessionID)
}

// IsReceiverReady 检查接收端是否准备好
func (r *RelayService) IsReceiverReady(sessionID string) bool {
	r.channelsMu.RLock()
	channel, exists := r.channels[sessionID]
	r.channelsMu.RUnlock()

	if !exists {
		return false
	}

	return channel.ReceiverReady && channel.SupportsStreaming
}

// GetReceiverPlatform 获取接收端平台
func (r *RelayService) GetReceiverPlatform(sessionID string) string {
	r.channelsMu.RLock()
	channel, exists := r.channels[sessionID]
	r.channelsMu.RUnlock()

	if !exists {
		return ""
	}

	return channel.ReceiverPlatform
}

// EncodeChunkBase64 将分片数据编码为 base64
func EncodeChunkBase64(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

// DecodeChunkBase64 将 base64 数据解码
func DecodeChunkBase64(encoded string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(encoded)
}

// Relay errors
var (
	ErrChannelNotFound = &RelayError{Code: 404, Message: "relay channel not found"}
)

type RelayError struct {
	Code    int
	Message string
}

func (e *RelayError) Error() string {
	return e.Message
}