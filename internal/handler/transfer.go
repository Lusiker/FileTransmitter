package handler

import (
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/lusiker/filetransmitter/internal/service"
)

type TransferHandler struct {
	transferService *service.TransferService
	sessionService  *service.SessionService
}

func NewTransferHandler(transferService *service.TransferService, sessionService *service.SessionService) *TransferHandler {
	return &TransferHandler{
		transferService: transferService,
		sessionService:  sessionService,
	}
}

func (h *TransferHandler) UploadFile(c *gin.Context) {
	// 从 FormData 或 Query Param 获取 session_id（备用方式）
	sessionID := c.PostForm("session_id")
	if sessionID == "" {
		sessionID = c.Query("session_id")
	}
	if sessionID == "" {
		log.Printf("[Transfer] Upload failed: session_id required")
		c.JSON(http.StatusBadRequest, gin.H{"error": "session_id required"})
		return
	}

	// 从 FormData 或 Query Param 获取 file_id（备用方式）
	fileID := c.PostForm("file_id")
	if fileID == "" {
		fileID = c.Query("file_id")
	}
	if fileID == "" {
		log.Printf("[Transfer] Upload failed: file_id required (session: %s)", sessionID)
		c.JSON(http.StatusBadRequest, gin.H{"error": "file_id required"})
		return
	}

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		log.Printf("[Transfer] Upload failed: file required (session: %s, file_id: %s) - %v", sessionID, fileID, err)
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("file required: %v", err)})
		return
	}
	defer file.Close()

	log.Printf("[Transfer] Starting upload: session=%s, file_id=%s, filename=%s, size=%d", sessionID, fileID, header.Filename, header.Size)

	err = h.transferService.UploadFile(sessionID, fileID, file, header)
	if err != nil {
		log.Printf("[Transfer] Upload error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	log.Printf("[Transfer] Upload success: session=%s, file_id=%s", sessionID, fileID)
	c.JSON(http.StatusOK, gin.H{
		"message": "file uploaded",
		"file_id": fileID,
	})
}

// UploadFileChunk handles chunked file upload for large files
func (h *TransferHandler) UploadFileChunk(c *gin.Context) {
	sessionID := c.PostForm("session_id")
	if sessionID == "" {
		log.Printf("[Transfer] Chunk upload failed: session_id required")
		c.JSON(http.StatusBadRequest, gin.H{"error": "session_id required"})
		return
	}

	fileID := c.PostForm("file_id")
	if fileID == "" {
		log.Printf("[Transfer] Chunk upload failed: file_id required")
		c.JSON(http.StatusBadRequest, gin.H{"error": "file_id required"})
		return
	}

	chunkIndexStr := c.PostForm("chunk_index")
	totalChunksStr := c.PostForm("total_chunks")
	fileName := c.PostForm("file_name")
	fileSizeStr := c.PostForm("file_size")

	if chunkIndexStr == "" || totalChunksStr == "" {
		log.Printf("[Transfer] Chunk upload failed: chunk_index and total_chunks required")
		c.JSON(http.StatusBadRequest, gin.H{"error": "chunk_index and total_chunks required"})
		return
	}

	chunkIndex, err := strconv.Atoi(chunkIndexStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid chunk_index"})
		return
	}

	totalChunks, err := strconv.Atoi(totalChunksStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid total_chunks"})
		return
	}

	fileSize, err := strconv.ParseInt(fileSizeStr, 10, 64)
	if err != nil {
		fileSize = 0 // Unknown size, will use chunk sizes
	}

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		log.Printf("[Transfer] Chunk upload failed: file required - %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("file required: %v", err)})
		return
	}
	defer file.Close()

	log.Printf("[Transfer] Chunk upload: session=%s, file_id=%s, chunk=%d/%d, size=%d", sessionID, fileID, chunkIndex, totalChunks, header.Size)

	err = h.transferService.UploadFileChunk(sessionID, fileID, fileName, fileSize, chunkIndex, totalChunks, file, header)
	if err != nil {
		log.Printf("[Transfer] Chunk upload error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":     "chunk uploaded",
		"chunk_index": chunkIndex,
		"file_id":     fileID,
	})
}

func (h *TransferHandler) UploadFiles(c *gin.Context) {
	sessionID := c.PostForm("session_id")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "session_id required"})
		return
	}

	form, err := c.MultipartForm()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "multipart form required"})
		return
	}

	files := form.File["files"]
	if len(files) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "files required"})
		return
	}

	// Upload files concurrently
	for _, header := range files {
		file, err := header.Open()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to open %s: %v", header.Filename, err)})
			return
		}
		defer file.Close()

		// Use filename as file_id for simplicity
		err = h.transferService.UploadFile(sessionID, header.Filename, file, header)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "files uploaded",
		"count":   len(files),
	})
}

func (h *TransferHandler) DownloadFile(c *gin.Context) {
	sessionID := c.Param("session_id")
	fileID := c.Param("file_id")

	// Check for inline mode (for iPad preview)
	inline := c.Query("inline") == "true"

	err := h.transferService.DownloadFile(sessionID, fileID, c.Writer, c.Request, inline)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
}

func (h *TransferHandler) DownloadAllAsZip(c *gin.Context) {
	sessionID := c.Param("session_id")

	err := h.transferService.DownloadAllAsZip(sessionID, c.Writer)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
}

func (h *TransferHandler) GetTransferStatus(c *gin.Context) {
	sessionID := c.Param("session_id")

	session := h.sessionService.Get(sessionID)
	if session == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"session_id":  sessionID,
		"state":       session.State,
		"total_size":  session.TotalSize,
		"transferred": session.Transferred,
		"percent":     int((session.Transferred * 100) / session.TotalSize),
		"files":       session.Files,
	})
}

func (h *TransferHandler) CancelTransfer(c *gin.Context) {
	sessionID := c.Param("session_id")

	err := h.transferService.CancelTransfer(sessionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "transfer cancelled"})
}

func (h *TransferHandler) SaveFile(c *gin.Context) {
	sessionID := c.Param("session_id")
	fileID := c.Param("file_id")

	destDir := c.PostForm("dest_dir")
	if destDir == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "dest_dir required"})
		return
	}

	err := h.transferService.SaveFileTo(sessionID, fileID, destDir)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "file saved",
		"file_id": fileID,
	})
}

func RegisterTransferRoutes(r *gin.RouterGroup, handler *TransferHandler) {
	r.POST("/transfer/upload", handler.UploadFile)
	r.POST("/transfer/upload/chunk", handler.UploadFileChunk)
	r.POST("/transfer/upload/batch", handler.UploadFiles)
	r.GET("/transfer/:session_id/download/:file_id", handler.DownloadFile)
	r.GET("/transfer/:session_id/download/zip", handler.DownloadAllAsZip)
	r.GET("/transfer/:session_id/status", handler.GetTransferStatus)
	r.POST("/transfer/:session_id/cancel", handler.CancelTransfer)
	r.POST("/transfer/:session_id/save/:file_id", handler.SaveFile)
}