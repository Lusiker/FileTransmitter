package service

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/lusiker/filetransmitter/internal/config"
	"github.com/lusiker/filetransmitter/internal/model"
	"github.com/lusiker/filetransmitter/internal/util"
	"github.com/lusiker/filetransmitter/internal/ws"
)

type TransferService struct {
	sessionService *SessionService
	cleanupService *CleanupService
	config         *config.TransferConfig
	hub            *ws.Hub
	relayService   *RelayService // 流式转发服务（可选）
	uploads        map[string]map[string]string          // sessionID -> fileID -> tempPath
	chunkUploads   map[string]map[string]*chunkUploadInfo // sessionID -> fileID -> chunk info
	mu             sync.RWMutex
}

type chunkUploadInfo struct {
	fileName     string
	fileSize     int64
	totalChunks  int
	receivedChunks map[int]string // chunkIndex -> tempChunkPath
	tempDir      string
}

func NewTransferService(sessionService *SessionService, cleanupService *CleanupService, cfg *config.TransferConfig, hub *ws.Hub) *TransferService {
	return &TransferService{
		sessionService: sessionService,
		cleanupService: cleanupService,
		config:         cfg,
		hub:            hub,
		relayService:   nil, // 初始为 nil，由外部设置
		uploads:        make(map[string]map[string]string),
		chunkUploads:   make(map[string]map[string]*chunkUploadInfo),
	}
}

// SetRelayService 设置流式转发服务
func (t *TransferService) SetRelayService(relay *RelayService) {
	t.relayService = relay
}

func (t *TransferService) UploadFile(sessionID, fileID string, file multipart.File, header *multipart.FileHeader) error {
	session := t.sessionService.Get(sessionID)
	if session == nil {
		return fmt.Errorf("session not found")
	}

	// Check session state - uploads only allowed after acceptance
	if session.State != model.SessionStateAccepted && session.State != model.SessionStateTransferring {
		return fmt.Errorf("session not accepted yet (state: %s)", session.State)
	}

	// Ensure temp directory exists
	sessionDir := util.GetSessionTempDir(t.config.TempDir, sessionID)
	if err := util.EnsureDir(sessionDir); err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}

	// Save file to temp location
	tempPath := util.GetTempFilePath(t.config.TempDir, sessionID, fileID)
	dst, err := os.Create(tempPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer dst.Close()

	// Track temp file for cleanup
	t.mu.Lock()
	if t.uploads[sessionID] == nil {
		t.uploads[sessionID] = make(map[string]string)
	}
	t.uploads[sessionID][fileID] = tempPath
	t.mu.Unlock()

	// Copy file with progress tracking
	buf := make([]byte, t.config.ChunkSize)
	transferred := int64(0)

	for {
		n, err := file.Read(buf)
		if err != nil && err != io.EOF {
			return fmt.Errorf("read error: %w", err)
		}

		if n > 0 {
			_, writeErr := dst.Write(buf[:n])
			if writeErr != nil {
				return fmt.Errorf("write error: %w", writeErr)
			}
			transferred += int64(n)

			// Update progress
			t.sessionService.UpdateFileProgress(sessionID, fileID, transferred)

			// Broadcast progress (uploading phase)
			t.broadcastProgress(sessionID, fileID, transferred, header.Size, ws.PhaseUploading)
		}

		if err == io.EOF {
			break
		}
	}

	// Verify hash if provided
	sessionFile := t.findSessionFile(session, fileID)
	if sessionFile != nil && sessionFile.Hash != "" {
		hash, err := util.CalculateFileHash(tempPath)
		if err != nil {
			t.sessionService.MarkFileFailed(sessionID, fileID, "hash calculation failed")
			return t.cleanupFailed(sessionID, fileID)
		}
		if hash != sessionFile.Hash {
			t.sessionService.MarkFileFailed(sessionID, fileID, "hash mismatch")
			return t.cleanupFailed(sessionID, fileID)
		}
	}

	// Mark file as complete
	t.sessionService.MarkFileComplete(sessionID, fileID)

	log.Printf("[Transfer] Upload complete: %s/%s", sessionID, fileID)

	return nil
}

// UploadFileChunk handles a single chunk upload for large files
func (t *TransferService) UploadFileChunk(sessionID, fileID, fileName string, fileSize int64, chunkIndex, totalChunks int, file multipart.File, header *multipart.FileHeader) error {
	session := t.sessionService.Get(sessionID)
	if session == nil {
		return fmt.Errorf("session not found")
	}

	// Check session state
	if session.State != model.SessionStateAccepted && session.State != model.SessionStateTransferring {
		return fmt.Errorf("session not accepted yet (state: %s)", session.State)
	}

	// Ensure temp directory exists
	sessionDir := util.GetSessionTempDir(t.config.TempDir, sessionID)
	chunksDir := filepath.Join(sessionDir, "chunks", fileID)
	if err := util.EnsureDir(chunksDir); err != nil {
		return fmt.Errorf("failed to create chunks dir: %w", err)
	}

	// Initialize or get chunk upload info
	t.mu.Lock()
	if t.chunkUploads[sessionID] == nil {
		t.chunkUploads[sessionID] = make(map[string]*chunkUploadInfo)
	}
	info, exists := t.chunkUploads[sessionID][fileID]
	if !exists {
		info = &chunkUploadInfo{
			fileName:       fileName,
			fileSize:       fileSize,
			totalChunks:    totalChunks,
			receivedChunks: make(map[int]string),
			tempDir:        chunksDir,
		}
		t.chunkUploads[sessionID][fileID] = info
	}
	t.mu.Unlock()

	// Save chunk to temp location
	chunkPath := filepath.Join(chunksDir, fmt.Sprintf("chunk_%d", chunkIndex))
	dst, err := os.Create(chunkPath)
	if err != nil {
		return fmt.Errorf("failed to create chunk file: %w", err)
	}

	_, err = io.Copy(dst, file)
	dst.Close()
	if err != nil {
		return fmt.Errorf("failed to write chunk: %w", err)
	}

	// Record received chunk
	t.mu.Lock()
	info.receivedChunks[chunkIndex] = chunkPath
	receivedCount := len(info.receivedChunks)
	t.mu.Unlock()

	// Update progress based on received chunks
	sessionFile := t.findSessionFile(session, fileID)
	if sessionFile != nil && sessionFile.Size > 0 {
		estimatedTransferred := (int64(receivedCount) * sessionFile.Size) / int64(totalChunks)
		t.sessionService.UpdateFileProgress(sessionID, fileID, estimatedTransferred)
		t.broadcastProgress(sessionID, fileID, estimatedTransferred, sessionFile.Size, ws.PhaseUploading)
	}

	log.Printf("[Transfer] Chunk saved: %s/%s chunk %d/%d (received: %d)", sessionID, fileID, chunkIndex, totalChunks, receivedCount)

	// Check if all chunks received
	if receivedCount == totalChunks {
		log.Printf("[Transfer] All chunks received, merging: %s/%s", sessionID, fileID)
		err = t.mergeChunks(sessionID, fileID, info, session)
		if err != nil {
			log.Printf("[Transfer] Merge failed: %v", err)
			t.sessionService.MarkFileFailed(sessionID, fileID, "merge failed")
			return t.cleanupChunkUpload(sessionID, fileID, info)
		}

		// Mark file as complete
		t.sessionService.MarkFileComplete(sessionID, fileID)
		log.Printf("[Transfer] Upload complete (chunked): %s/%s", sessionID, fileID)
	}

	return nil
}

// mergeChunks combines all chunks into the final file
func (t *TransferService) mergeChunks(sessionID, fileID string, info *chunkUploadInfo, session *model.Session) error {
	// Get the session file info for the correct file size
	sessionFile := t.findSessionFile(session, fileID)
	finalSize := info.fileSize
	if sessionFile != nil && sessionFile.Size > 0 {
		finalSize = sessionFile.Size
	}

	// Broadcast merging phase start (progress stays at 100% from upload)
	if sessionFile != nil && sessionFile.Size > 0 {
		t.broadcastProgress(sessionID, fileID, sessionFile.Size, sessionFile.Size, ws.PhaseMerging)
	}

	// Create final file
	finalPath := util.GetTempFilePath(t.config.TempDir, sessionID, fileID)
	finalFile, err := os.Create(finalPath)
	if err != nil {
		return fmt.Errorf("failed to create final file: %w", err)
	}
	defer finalFile.Close()

	// Merge chunks in order
	for i := 0; i < info.totalChunks; i++ {
		chunkPath, exists := info.receivedChunks[i]
		if !exists {
			return fmt.Errorf("missing chunk %d", i)
		}

		chunkFile, err := os.Open(chunkPath)
		if err != nil {
			return fmt.Errorf("failed to open chunk %d: %w", i, err)
		}

		_, err = io.Copy(finalFile, chunkFile)
		chunkFile.Close()
		if err != nil {
			return fmt.Errorf("failed to copy chunk %d: %w", i, err)
		}

		// Delete chunk after merging
		os.Remove(chunkPath)
	}

	// Track final file
	t.mu.Lock()
	if t.uploads[sessionID] == nil {
		t.uploads[sessionID] = make(map[string]string)
	}
	t.uploads[sessionID][fileID] = finalPath
	delete(t.chunkUploads[sessionID], fileID)
	t.mu.Unlock()

	// Clean up chunks directory
	os.RemoveAll(info.tempDir)

	// Verify file size if known
	if finalSize > 0 {
		stat, err := finalFile.Stat()
		if err == nil && stat.Size() != finalSize {
			log.Printf("[Transfer] Warning: final file size %d != expected %d", stat.Size(), finalSize)
		}
	}

	return nil
}

// cleanupChunkUpload removes all chunk files and tracking info
func (t *TransferService) cleanupChunkUpload(sessionID, fileID string, info *chunkUploadInfo) error {
	// Remove all chunk files
	for _, chunkPath := range info.receivedChunks {
		os.Remove(chunkPath)
	}
	if info.tempDir != "" {
		os.RemoveAll(info.tempDir)
	}

	// Remove tracking
	t.mu.Lock()
	if t.chunkUploads[sessionID] != nil {
		delete(t.chunkUploads[sessionID], fileID)
	}
	t.mu.Unlock()

	return fmt.Errorf("chunk upload failed")
}

func (t *TransferService) DownloadFile(sessionID, fileID string, w http.ResponseWriter, r *http.Request, inline bool) error {
	session := t.sessionService.Get(sessionID)
	if session == nil {
		return fmt.Errorf("session not found")
	}

	file := t.findSessionFile(session, fileID)
	if file == nil {
		return fmt.Errorf("file not found")
	}

	tempPath := t.getTempPath(sessionID, fileID)
	if tempPath == "" {
		return fmt.Errorf("file not uploaded")
	}

	// Set disposition header
	disposition := "attachment"
	if inline {
		disposition = "inline"
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf("%s; filename=\"%s\"", disposition, file.Name))

	// Use http.ServeFile to support Range requests (enables Safari download resume)
	// This automatically handles:
	// - Accept-Ranges: bytes
	// - Content-Range for partial content (206)
	// - Range header parsing
	http.ServeFile(w, r, tempPath)

	log.Printf("[Transfer] Download complete: %s/%s", sessionID, fileID)

	return nil
}

func (t *TransferService) CancelTransfer(sessionID string) error {
	t.mu.Lock()
	paths := t.uploads[sessionID]
	t.mu.Unlock()

	// Clean up all files
	for fileID, path := range paths {
		os.Remove(path)
		t.sessionService.MarkFileFailed(sessionID, fileID, "transfer cancelled")
	}

	// Clean up directory
	sessionDir := util.GetSessionTempDir(t.config.TempDir, sessionID)
	os.RemoveAll(sessionDir)

	// Remove tracking
	t.mu.Lock()
	delete(t.uploads, sessionID)
	t.mu.Unlock()

	return t.sessionService.Cancel(sessionID)
}

func (t *TransferService) SaveFileTo(sessionID, fileID, destDir string) error {
	session := t.sessionService.Get(sessionID)
	if session == nil {
		return fmt.Errorf("session not found")
	}

	file := t.findSessionFile(session, fileID)
	if file == nil {
		return fmt.Errorf("file not found")
	}

	tempPath := t.getTempPath(sessionID, fileID)
	if tempPath == "" {
		return fmt.Errorf("file not uploaded")
	}

	// Ensure destination directory exists
	if err := util.EnsureDir(destDir); err != nil {
		return fmt.Errorf("failed to create dest dir: %w", err)
	}

	// Move file to destination
	destPath := filepath.Join(destDir, file.Name)
	if err := os.Rename(tempPath, destPath); err != nil {
		// If rename fails (cross-device), copy instead
		if err := copyFile(tempPath, destPath); err != nil {
			return fmt.Errorf("failed to save file: %w", err)
		}
		os.Remove(tempPath)
	}

	log.Printf("[Transfer] File saved: %s -> %s", fileID, destPath)

	return nil
}

func (t *TransferService) CleanupSession(sessionID string) {
	t.mu.Lock()
	delete(t.uploads, sessionID)
	t.mu.Unlock()

	// Clean up directory
	sessionDir := util.GetSessionTempDir(t.config.TempDir, sessionID)
	os.RemoveAll(sessionDir)
}

func (t *TransferService) findSessionFile(session *model.Session, fileID string) *model.FileInfo {
	for _, f := range session.Files {
		if f.ID == fileID {
			return &f
		}
	}
	return nil
}

func (t *TransferService) getTempPath(sessionID, fileID string) string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.uploads[sessionID] == nil {
		return ""
	}
	return t.uploads[sessionID][fileID]
}

func (t *TransferService) cleanupFailed(sessionID, fileID string) error {
	t.mu.Lock()
	path := ""
	if t.uploads[sessionID] != nil {
		path = t.uploads[sessionID][fileID]
		delete(t.uploads[sessionID], fileID)
	}
	t.mu.Unlock()

	if path != "" {
		os.Remove(path)
	}

	// If no more files, clean up directory
	t.mu.RLock()
	remaining := len(t.uploads[sessionID])
	t.mu.RUnlock()

	if remaining == 0 {
		sessionDir := util.GetSessionTempDir(t.config.TempDir, sessionID)
		os.RemoveAll(sessionDir)
	}

	return fmt.Errorf("file transfer failed")
}

func (t *TransferService) broadcastProgress(sessionID, fileID string, transferred, total int64, phase ws.TransferPhase) {
	percent := int((transferred * 100) / total)
	if percent > 100 {
		percent = 100
	}

	progress := ws.TransferProgressData{
		SessionID: sessionID,
		FileID:    fileID,
		Bytes:     transferred,
		Total:     total,
		Percent:   percent,
		Phase:     phase,
	}

	session := t.sessionService.Get(sessionID)
	if session != nil {
		t.hub.BroadcastMessageToDevices([]string{session.SenderID, session.ReceiverID}, ws.MessageTypeTransferProgress, progress)
	}
}

func copyFile(src, dst string) error {
	s, err := os.Open(src)
	if err != nil {
		return err
	}
	defer s.Close()

	d, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer d.Close()

	_, err = io.Copy(d, s)
	return err
}

// IsPreviewable checks if a file type can be previewed in Safari
func IsPreviewable(mimeType string) bool {
	previewableTypes := []string{
		"application/pdf",
		"image/",
		"video/",
		"text/",
		"application/json",
	}
	for _, t := range previewableTypes {
		if strings.HasPrefix(mimeType, t) {
			return true
		}
	}
	return false
}

// DownloadAllAsZip creates a zip file containing all session files
func (t *TransferService) DownloadAllAsZip(sessionID string, w http.ResponseWriter) error {
	session := t.sessionService.Get(sessionID)
	if session == nil {
		return fmt.Errorf("session not found")
	}

	// Set headers for zip download
	zipName := fmt.Sprintf("transfer_%s.zip", sessionID[:8])
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", zipName))

	// Create zip writer
	zipWriter := zip.NewWriter(w)
	defer zipWriter.Close()

	// Add each file to the zip
	for _, file := range session.Files {
		if file.Status != model.FileStatusSuccess {
			continue
		}

		tempPath := t.getTempPath(sessionID, file.ID)
		if tempPath == "" {
			continue
		}

		// Open source file
		srcFile, err := os.Open(tempPath)
		if err != nil {
			log.Printf("[Transfer] Failed to open file for zip: %s", file.ID)
			continue
		}

		// Create entry in zip
		entry, err := zipWriter.Create(file.Name)
		if err != nil {
			srcFile.Close()
			continue
		}

		// Copy file content
		_, err = io.Copy(entry, srcFile)
		srcFile.Close()
		if err != nil {
			continue
		}
	}

	log.Printf("[Transfer] Zip download complete: %s", sessionID)
	return nil
}

// ConcurrentUpload handles multiple file uploads concurrently
func (t *TransferService) ConcurrentUpload(ctx context.Context, sessionID string, files []*multipart.FileHeader) error {
	var wg sync.WaitGroup
	errCh := make(chan error, len(files))

	for _, header := range files {
		wg.Add(1)
		go func(h *multipart.FileHeader) {
			defer wg.Done()

			file, err := h.Open()
			if err != nil {
				errCh <- fmt.Errorf("failed to open %s: %w", h.Filename, err)
				return
			}
			defer file.Close()

			if err := t.UploadFile(sessionID, h.Filename, file, h); err != nil {
				errCh <- err
			}
		}(header)
	}

	wg.Wait()
	close(errCh)

	// Return first error if any
	for err := range errCh {
		if err != nil {
			return err
		}
	}

	return nil
}