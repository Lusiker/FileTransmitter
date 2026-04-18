package service

import (
	"log"
	"os"

	"github.com/lusiker/filetransmitter/internal/util"
)

type CleanupService struct {
	tempDir string
}

func NewCleanupService(tempDir string) *CleanupService {
	return &CleanupService{tempDir: tempDir}
}

func (c *CleanupService) CleanupFailedFiles(sessionID string, fileIDs []string) error {
	for _, fileID := range fileIDs {
		tempPath := util.GetTempFilePath(c.tempDir, sessionID, fileID)
		if err := os.Remove(tempPath); err != nil {
			if !os.IsNotExist(err) {
				log.Printf("[Cleanup] Failed to remove %s: %v", tempPath, err)
			}
		} else {
			log.Printf("[Cleanup] Removed: %s", tempPath)
		}
	}

	// Check if session directory is empty
	sessionDir := util.GetSessionTempDir(c.tempDir, sessionID)
	files, err := os.ReadDir(sessionDir)
	if err == nil && len(files) == 0 {
		os.RemoveAll(sessionDir)
		log.Printf("[Cleanup] Removed empty session dir: %s", sessionDir)
	}

	return nil
}

func (c *CleanupService) CleanupSession(sessionID string) error {
	sessionDir := util.GetSessionTempDir(c.tempDir, sessionID)
	if err := os.RemoveAll(sessionDir); err != nil {
		if !os.IsNotExist(err) {
			return err
		}
	}
	log.Printf("[Cleanup] Removed session dir: %s", sessionDir)
	return nil
}

func (c *CleanupService) CleanupAll() error {
	if err := os.RemoveAll(c.tempDir); err != nil {
		if !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}