package handler

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lusiker/filetransmitter/internal/config"
	"github.com/lusiker/filetransmitter/internal/service"
	"github.com/lusiker/filetransmitter/internal/ws"
)

type AdminHandler struct {
	clientDeviceService *service.ClientDeviceService
	sessionService      *service.SessionService
	transferService     *service.TransferService
	relayService        *service.RelayService
	hub                 *ws.Hub
	config              *config.TransferConfig
}

func NewAdminHandler(
	clientDeviceService *service.ClientDeviceService,
	sessionService *service.SessionService,
	transferService *service.TransferService,
	relayService *service.RelayService,
	hub *ws.Hub,
	cfg *config.TransferConfig,
) *AdminHandler {
	return &AdminHandler{
		clientDeviceService: clientDeviceService,
		sessionService:      sessionService,
		transferService:     transferService,
		relayService:        relayService,
		hub:                 hub,
		config:              cfg,
	}
}

// GetDevices 获取所有连接的设备
func (h *AdminHandler) GetDevices(c *gin.Context) {
	devices := h.clientDeviceService.GetAll()

	result := make([]map[string]interface{}, len(devices))
	for i, d := range devices {
		result[i] = map[string]interface{}{
			"id":         d.ID,
			"name":       d.Name,
			"role":       d.Role,
			"ip":         d.IP,
			"is_online":  true,
			"last_seen":  time.Now().Format(time.RFC3339),
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"devices": result,
		"count":   len(result),
	})
}

// GetSessions 获取所有会话状态
func (h *AdminHandler) GetSessions(c *gin.Context) {
	sessions := h.sessionService.GetAll()

	result := make([]map[string]interface{}, len(sessions))
	for i, s := range sessions {
		files := make([]map[string]interface{}, len(s.Files))
		for j, f := range s.Files {
			files[j] = map[string]interface{}{
				"id":            f.ID,
				"name":          f.Name,
				"size":          f.Size,
				"status":        f.Status,
				"transfer_size": f.TransferSize,
				"error":         f.Error,
			}
		}

		result[i] = map[string]interface{}{
			"id":          s.ID,
			"sender_id":   s.SenderID,
			"receiver_id": s.ReceiverID,
			"state":       s.State,
			"files":       files,
			"total_size":  s.TotalSize,
			"transferred": s.Transferred,
			"created_at":  s.CreatedAt.Format(time.RFC3339),
			"updated_at":  s.UpdatedAt.Format(time.RFC3339),
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"sessions": result,
		"count":    len(result),
	})
}

// GetStorage 获取临时存储使用情况
func (h *AdminHandler) GetStorage(c *gin.Context) {
	tempDir := h.config.TempDir

	// 获取目录下的所有会话目录
	dirs, err := ioutil.ReadDir(tempDir)
	if err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusOK, gin.H{
				"temp_dir": tempDir,
				"sessions": []map[string]interface{}{},
				"total_size": 0,
				"total_count": 0,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	sessionDirs := []map[string]interface{}{}
	var totalSize int64 = 0

	for _, dir := range dirs {
		if !dir.IsDir() {
			continue
		}

		sessionID := dir.Name()
		sessionSize := h.calculateDirSize(filepath.Join(tempDir, sessionID))

		// 检查会话状态
		session := h.sessionService.Get(sessionID)
		state := "unknown"
		if session != nil {
			state = string(session.State)
		}

		// 检查是否有分片目录
		chunksDir := filepath.Join(tempDir, sessionID, "chunks")
		hasChunks := false
		if info, err := os.Stat(chunksDir); err == nil && info.IsDir() {
			hasChunks = true
		}

		sessionDirs = append(sessionDirs, map[string]interface{}{
			"session_id": sessionID,
			"size":       sessionSize,
			"size_mb":    float64(sessionSize) / (1024 * 1024),
			"state":      state,
			"has_chunks": hasChunks,
			"files_count": h.countFilesInDir(filepath.Join(tempDir, sessionID)),
		})
		totalSize += sessionSize
	}

	// 按大小排序
	sort.Slice(sessionDirs, func(i, j int) bool {
		return sessionDirs[i]["size"].(int64) > sessionDirs[j]["size"].(int64)
	})

	c.JSON(http.StatusOK, gin.H{
		"temp_dir":    tempDir,
		"sessions":    sessionDirs,
		"total_size":  totalSize,
		"total_size_mb": float64(totalSize) / (1024 * 1024),
		"total_count": len(sessionDirs),
	})
}

// CleanSession 清除指定会话的临时文件
func (h *AdminHandler) CleanSession(c *gin.Context) {
	sessionID := c.Param("session_id")

	// 检查会话状态（不允许清理正在传输的会话）
	session := h.sessionService.Get(sessionID)
	if session != nil && (session.State == "transferring" || session.State == "accepted") {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Cannot clean session that is currently transferring",
		})
		return
	}

	sessionDir := filepath.Join(h.config.TempDir, sessionID)

	// 检查目录是否存在
	if _, err := os.Stat(sessionDir); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Session directory not found",
		})
		return
	}

	// 计算删除前的大小
	sizeBefore := h.calculateDirSize(sessionDir)

	// 删除目录
	if err := os.RemoveAll(sessionDir); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to remove directory: %v", err),
		})
		return
	}

	// 清理内存中的记录
	if session != nil {
		h.sessionService.Cleanup(session.SenderID)
		h.sessionService.Cleanup(session.ReceiverID)
	}

	// 清理 transfer service 中的记录
	h.transferService.CleanupSession(sessionID)

	// 清理 relay service 中的记录
	if h.relayService != nil {
		h.relayService.CloseChannel(sessionID)
	}

	log.Printf("[Admin] Cleaned session: %s (size: %d bytes)", sessionID, sizeBefore)

	c.JSON(http.StatusOK, gin.H{
		"message":    "Session cleaned successfully",
		"session_id": sessionID,
		"size_cleaned": sizeBefore,
		"size_cleaned_mb": float64(sizeBefore) / (1024 * 1024),
	})
}

// CleanAllCompleted 清除所有已完成/取消的会话临时文件
func (h *AdminHandler) CleanAllCompleted(c *gin.Context) {
	tempDir := h.config.TempDir

	dirs, err := ioutil.ReadDir(tempDir)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var totalCleaned int64 = 0
	cleanedSessions := []string{}

	for _, dir := range dirs {
		if !dir.IsDir() {
			continue
		}

		sessionID := dir.Name()
		session := h.sessionService.Get(sessionID)

		// 只清理已完成、取消、失败的会话
		if session == nil || session.State == "completed" || session.State == "cancelled" || session.State == "failed" || session.State == "partially_completed" {
			sessionDir := filepath.Join(tempDir, sessionID)
			size := h.calculateDirSize(sessionDir)

			if err := os.RemoveAll(sessionDir); err != nil {
				log.Printf("[Admin] Failed to clean %s: %v", sessionID, err)
				continue
			}

			// 清理内存记录
			if session != nil {
				h.sessionService.Cleanup(session.SenderID)
				h.sessionService.Cleanup(session.ReceiverID)
			}
			h.transferService.CleanupSession(sessionID)
			if h.relayService != nil {
				h.relayService.CloseChannel(sessionID)
			}

			totalCleaned += size
			cleanedSessions = append(cleanedSessions, sessionID)
			log.Printf("[Admin] Cleaned: %s (%.2f MB)", sessionID, float64(size)/(1024*1024))
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message":           "Cleanup completed",
		"cleaned_sessions":  cleanedSessions,
		"cleaned_count":     len(cleanedSessions),
		"total_cleaned":     totalCleaned,
		"total_cleaned_mb":  float64(totalCleaned) / (1024 * 1024),
	})
}

// GetSystemStatus 获取系统整体状态
func (h *AdminHandler) GetSystemStatus(c *gin.Context) {
	// 设备统计
	senders := h.clientDeviceService.GetSenders()
	receivers := h.clientDeviceService.GetReceivers()

	// 会话统计
	sessions := h.sessionService.GetAll()
	var activeSessions, completedSessions, failedSessions int
	for _, s := range sessions {
		switch s.State {
		case "transferring", "accepted":
			activeSessions++
		case "completed":
			completedSessions++
		case "failed", "cancelled":
			failedSessions++
		}
	}

	// 存储统计
	tempDir := h.config.TempDir
	var storageUsed int64 = 0
	if dirs, err := ioutil.ReadDir(tempDir); err == nil {
		for _, dir := range dirs {
			if dir.IsDir() {
				storageUsed += h.calculateDirSize(filepath.Join(tempDir, dir.Name()))
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"devices": gin.H{
			"senders":  len(senders),
			"receivers": len(receivers),
			"total":    len(senders) + len(receivers),
		},
		"sessions": gin.H{
			"active":    activeSessions,
			"completed": completedSessions,
			"failed":    failedSessions,
			"total":     len(sessions),
		},
		"storage": gin.H{
			"used_bytes": storageUsed,
			"used_mb":    float64(storageUsed) / (1024 * 1024),
			"temp_dir":   tempDir,
		},
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

// 辅助函数：计算目录大小
func (h *AdminHandler) calculateDirSize(path string) int64 {
	var size int64 = 0
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // 忽略错误
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	if err != nil {
		return 0
	}
	return size
}

// 辅助函数：计算目录中的文件数
func (h *AdminHandler) countFilesInDir(path string) int {
	count := 0
	filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			count++
		}
		return nil
	})
	return count
}

// RegisterAdminRoutes 注册管理员路由
func RegisterAdminRoutes(r *gin.RouterGroup, handler *AdminHandler) {
	r.GET("/admin/status", handler.GetSystemStatus)
	r.GET("/admin/devices", handler.GetDevices)
	r.GET("/admin/sessions", handler.GetSessions)
	r.GET("/admin/storage", handler.GetStorage)
	r.DELETE("/admin/storage/:session_id", handler.CleanSession)
	r.POST("/admin/storage/clean-completed", handler.CleanAllCompleted)
}