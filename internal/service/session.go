package service

import (
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/lusiker/filetransmitter/internal/model"
	"github.com/lusiker/filetransmitter/internal/ws"
)

type SessionService struct {
	sessions map[string]*model.Session
	mu       sync.RWMutex
	hub      *ws.Hub
	relay    *RelayService // 流式转发服务（可选）
}

func NewSessionService(hub *ws.Hub) *SessionService {
	return &SessionService{
		sessions: make(map[string]*model.Session),
		hub:      hub,
		relay:    nil, // 由外部设置
	}
}

// SetRelayService 设置流式转发服务
func (s *SessionService) SetRelayService(relay *RelayService) {
	s.relay = relay
}

func (s *SessionService) Create(senderID, receiverID string, files []model.FileInfo) (*model.Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sessionID := uuid.New().String()
	now := time.Now()

	session := &model.Session{
		ID:         sessionID,
		SenderID:   senderID,
		ReceiverID: receiverID,
		State:      model.SessionStatePending,
		Files:      files,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	// Initialize file statuses
	for i := range session.Files {
		session.Files[i].Status = model.FileStatusPending
	}

	session.TotalSize = session.CalculateTotalSize()
	s.sessions[sessionID] = session

	log.Printf("[Session] Created: %s (sender: %s, receiver: %s)", sessionID, senderID, receiverID)

	// 创建 Relay 通道（如果 relay 服务已设置）
	if s.relay != nil {
		s.relay.CreateChannel(sessionID, senderID, receiverID)
		// 将文件添加到队列
		for _, f := range files {
			s.relay.AddFileToQueue(sessionID, f.ID)
		}
	}

	// Notify receiver via WebSocket
	s.hub.BroadcastMessageToDevice(receiverID, ws.MessageTypeSessionCreated, session)

	return session, nil
}

func (s *SessionService) Get(sessionID string) *model.Session {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sessions[sessionID]
}

func (s *SessionService) GetAll() []*model.Session {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var sessions []*model.Session
	for _, sess := range s.sessions {
		sessions = append(sessions, sess)
	}
	return sessions
}

func (s *SessionService) Accept(sessionID, savePath string) (*model.Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	session := s.sessions[sessionID]
	if session == nil {
		return nil, ErrSessionNotFound
	}

	session.State = model.SessionStateAccepted
	session.SavePath = savePath
	session.UpdatedAt = time.Now()

	log.Printf("[Session] Accepted: %s", sessionID)

	// Notify sender
	s.hub.BroadcastMessageToDevice(session.SenderID, ws.MessageTypeSessionAccepted, session)

	return session, nil
}

func (s *SessionService) StartTransfer(sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	session := s.sessions[sessionID]
	if session == nil {
		return ErrSessionNotFound
	}

	if session.State != model.SessionStateAccepted {
		return ErrInvalidState
	}

	session.State = model.SessionStateTransferring
	session.UpdatedAt = time.Now()

	log.Printf("[Session] Transfer started: %s", sessionID)

	return nil
}

func (s *SessionService) Cancel(sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	session := s.sessions[sessionID]
	if session == nil {
		return ErrSessionNotFound
	}

	session.State = model.SessionStateCancelled
	session.UpdatedAt = time.Now()

	log.Printf("[Session] Cancelled: %s", sessionID)

	// 通知 sender 和 receiver
	s.hub.BroadcastMessageToDevices([]string{session.SenderID, session.ReceiverID}, "session_cancelled", session)

	return nil
}

// Reject allows receiver to reject a pending session
func (s *SessionService) Reject(sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	session := s.sessions[sessionID]
	if session == nil {
		return ErrSessionNotFound
	}

	if session.State != model.SessionStatePending {
		return ErrInvalidState
	}

	session.State = model.SessionStateCancelled
	session.UpdatedAt = time.Now()

	log.Printf("[Session] Rejected by receiver: %s", sessionID)

	// 通知 sender
	s.hub.BroadcastMessageToDevice(session.SenderID, "session_rejected", session)

	// 从内存中删除
	delete(s.sessions, sessionID)

	return nil
}

// Complete marks a session as completed by receiver (after download)
func (s *SessionService) CompleteByReceiver(sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	session := s.sessions[sessionID]
	if session == nil {
		return ErrSessionNotFound
	}

	// 可以对任何非pending状态标记完成
	if session.State == model.SessionStatePending {
		return ErrInvalidState
	}

	session.State = model.SessionStateCompleted
	session.UpdatedAt = time.Now()

	log.Printf("[Session] Completed by receiver: %s", sessionID)

	return nil
}

// Cleanup removes old completed/cancelled sessions from memory
func (s *SessionService) Cleanup(deviceID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for id, session := range s.sessions {
		// 删除已完成或已取消的、且该设备参与的session
		if (session.State == model.SessionStateCompleted || session.State == model.SessionStateCancelled) &&
			(session.SenderID == deviceID || session.ReceiverID == deviceID) {
			delete(s.sessions, id)
			log.Printf("[Session] Cleaned up: %s", id)
		}
	}
}

func (s *SessionService) UpdateFileProgress(sessionID, fileID string, transferred int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	session := s.sessions[sessionID]
	if session == nil {
		return
	}

	for i, f := range session.Files {
		if f.ID == fileID {
			session.Files[i].TransferSize = transferred
			session.Files[i].Status = model.FileStatusTransferring
			break
		}
	}

	session.Transferred = session.CalculateTransferred()
	session.UpdatedAt = time.Now()
}

func (s *SessionService) MarkFileComplete(sessionID, fileID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	session := s.sessions[sessionID]
	if session == nil {
		log.Printf("[Session] MarkFileComplete: session not found %s", sessionID)
		return
	}

	for i, f := range session.Files {
		if f.ID == fileID {
			session.Files[i].Status = model.FileStatusSuccess
			session.Files[i].TransferSize = session.Files[i].Size
			log.Printf("[Session] File marked complete: %s/%s, status=%s", sessionID, fileID, session.Files[i].Status)
			break
		}
	}

	session.Transferred = session.CalculateTransferred()
	session.UpdatedAt = time.Now()

	// Notify sender
	s.hub.BroadcastMessageToDevice(session.SenderID, ws.MessageTypeFileComplete, ws.FileStatusData{
		SessionID: sessionID,
		FileID:    fileID,
		Status:    "success",
	})

	// Check if all files completed
	allComplete := s.allFilesCompleted(session)
	log.Printf("[Session] Checking all files complete: %s, result=%v, files=%d", sessionID, allComplete, len(session.Files))
	if allComplete {
		session.State = model.SessionStateCompleted
		log.Printf("[Session] Session completed: %s", sessionID)
		s.hub.BroadcastMessageToDevices([]string{session.SenderID, session.ReceiverID}, ws.MessageTypeTransferComplete, session)
	}
}

func (s *SessionService) MarkFileFailed(sessionID, fileID string, errMsg string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	session := s.sessions[sessionID]
	if session == nil {
		return
	}

	for i, f := range session.Files {
		if f.ID == fileID {
			session.Files[i].Status = model.FileStatusFailed
			session.Files[i].Error = errMsg
			break
		}
	}

	session.UpdatedAt = time.Now()

	// Notify sender
	s.hub.BroadcastMessageToDevice(session.SenderID, ws.MessageTypeFileFailed, ws.FileStatusData{
		SessionID: sessionID,
		FileID:    fileID,
		Status:    "failed",
		Error:     errMsg,
	})

	// Check if any files failed and update state
	if s.hasFailedFiles(session) && s.hasCompletedFiles(session) {
		session.State = model.SessionStatePartially
	} else if s.allFilesFailed(session) {
		session.State = model.SessionStateFailed
	}
}

func (s *SessionService) GetFailedFiles(sessionID string) []model.FileInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	session := s.sessions[sessionID]
	if session == nil {
		return nil
	}

	var failed []model.FileInfo
	for _, f := range session.Files {
		if f.Status == model.FileStatusFailed {
			failed = append(failed, f)
		}
	}
	return failed
}

func (s *SessionService) RetryFiles(sessionID string, fileIDs []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	session := s.sessions[sessionID]
	if session == nil {
		return ErrSessionNotFound
	}

	for _, fileID := range fileIDs {
		for i, f := range session.Files {
			if f.ID == fileID && f.Status == model.FileStatusFailed {
				session.Files[i].Status = model.FileStatusPending
				session.Files[i].Error = ""
				session.Files[i].TransferSize = 0
			}
		}
	}

	session.State = model.SessionStateAccepted
	session.UpdatedAt = time.Now()

	return nil
}

func (s *SessionService) allFilesCompleted(session *model.Session) bool {
	for _, f := range session.Files {
		if f.Status != model.FileStatusSuccess {
			return false
		}
	}
	return true
}

func (s *SessionService) hasFailedFiles(session *model.Session) bool {
	for _, f := range session.Files {
		if f.Status == model.FileStatusFailed {
			return true
		}
	}
	return false
}

func (s *SessionService) allFilesFailed(session *model.Session) bool {
	for _, f := range session.Files {
		if f.Status != model.FileStatusFailed {
			return false
		}
	}
	return true
}

func (s *SessionService) hasCompletedFiles(session *model.Session) bool {
	for _, f := range session.Files {
		if f.Status == model.FileStatusSuccess {
			return true
		}
	}
	return false
}

var (
	ErrSessionNotFound = &SessionError{Code: 404, Message: "session not found"}
	ErrInvalidState    = &SessionError{Code: 400, Message: "invalid session state"}
)

type SessionError struct {
	Code    int
	Message string
}

func (e *SessionError) Error() string {
	return e.Message
}