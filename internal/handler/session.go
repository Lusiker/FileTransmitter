package handler

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/lusiker/filetransmitter/internal/model"
	"github.com/lusiker/filetransmitter/internal/service"
)

type SessionHandler struct {
	sessionService *service.SessionService
}

func NewSessionHandler(sessionService *service.SessionService) *SessionHandler {
	return &SessionHandler{
		sessionService: sessionService,
	}
}

type CreateSessionRequest struct {
	SenderID   string         `json:"sender_id" binding:"required"`
	ReceiverID string         `json:"receiver_id" binding:"required"`
	Files      []FileInfoReq  `json:"files" binding:"required"`
}

type FileInfoReq struct {
	Name     string `json:"name" binding:"required"`
	Size     int64  `json:"size" binding:"required"`
	MIMEType string `json:"mime_type"`
	Hash     string `json:"hash"`
}

type AcceptSessionRequest struct {
	SavePath string `json:"save_path" binding:"required"`
}

type RetryFilesRequest struct {
	FileIDs []string `json:"file_ids" binding:"required"`
}

func (h *SessionHandler) CreateSession(c *gin.Context) {
	var req CreateSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("[Session] CreateSession error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	log.Printf("[Session] CreateSession request: sender=%s, receiver=%s, files_count=%d", req.SenderID, req.ReceiverID, len(req.Files))

	files := make([]model.FileInfo, len(req.Files))
	for i, f := range req.Files {
		files[i] = model.FileInfo{
			ID:       uuid.New().String(),
			Name:     f.Name,
			Size:     f.Size,
			MIMEType: f.MIMEType,
			Hash:     f.Hash,
			Status:   model.FileStatusPending,
		}
	}

	session, err := h.sessionService.Create(req.SenderID, req.ReceiverID, files)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, session)
}

func (h *SessionHandler) GetSessions(c *gin.Context) {
	sessions := h.sessionService.GetAll()
	c.JSON(http.StatusOK, gin.H{
		"sessions": sessions,
		"count":    len(sessions),
	})
}

func (h *SessionHandler) GetSession(c *gin.Context) {
	sessionID := c.Param("id")
	session := h.sessionService.Get(sessionID)
	if session == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}
	c.JSON(http.StatusOK, session)
}

func (h *SessionHandler) AcceptSession(c *gin.Context) {
	sessionID := c.Param("id")

	var req AcceptSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	session, err := h.sessionService.Accept(sessionID, req.SavePath)
	if err != nil {
		if err == service.ErrSessionNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, session)
}

func (h *SessionHandler) CancelSession(c *gin.Context) {
	sessionID := c.Param("id")

	err := h.sessionService.Cancel(sessionID)
	if err != nil {
		if err == service.ErrSessionNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "session cancelled"})
}

func (h *SessionHandler) GetFailedFiles(c *gin.Context) {
	sessionID := c.Param("id")
	failed := h.sessionService.GetFailedFiles(sessionID)
	c.JSON(http.StatusOK, gin.H{
		"failed_files": failed,
		"count":        len(failed),
	})
}

func (h *SessionHandler) RetryFiles(c *gin.Context) {
	sessionID := c.Param("id")

	var req RetryFilesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.sessionService.RetryFiles(sessionID, req.FileIDs)
	if err != nil {
		if err == service.ErrSessionNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}
		return
	}

	session := h.sessionService.Get(sessionID)
	c.JSON(http.StatusOK, session)
}

func (h *SessionHandler) RejectSession(c *gin.Context) {
	sessionID := c.Param("id")

	err := h.sessionService.Reject(sessionID)
	if err != nil {
		if err == service.ErrSessionNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "session rejected"})
}

func (h *SessionHandler) CleanupSessions(c *gin.Context) {
	deviceID := c.Query("device_id")
	if deviceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "device_id required"})
		return
	}

	h.sessionService.Cleanup(deviceID)
	c.JSON(http.StatusOK, gin.H{"message": "sessions cleaned up"})
}

func RegisterSessionRoutes(r *gin.RouterGroup, handler *SessionHandler) {
	r.POST("/sessions", handler.CreateSession)
	r.GET("/sessions", handler.GetSessions)
	r.GET("/sessions/:id", handler.GetSession)
	r.POST("/sessions/:id/accept", handler.AcceptSession)
	r.POST("/sessions/:id/reject", handler.RejectSession)
	r.DELETE("/sessions/:id", handler.CancelSession)
	r.GET("/sessions/:id/failed", handler.GetFailedFiles)
	r.POST("/sessions/:id/retry", handler.RetryFiles)
	r.POST("/sessions/cleanup", handler.CleanupSessions)
}