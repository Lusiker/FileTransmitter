package util

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
)

// CalculateFileHash calculates SHA256 hash of a file
func CalculateFileHash(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// CalculateBytesHash calculates SHA256 hash of bytes
func CalculateBytesHash(data []byte) string {
	h := sha256.New()
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}

// EnsureDir ensures a directory exists
func EnsureDir(dir string) error {
	return os.MkdirAll(dir, 0755)
}

// CleanupFiles removes files in the list
func CleanupFiles(files []string) error {
	for _, f := range files {
		if err := os.Remove(f); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

// CleanupDir removes a directory and all its contents
func CleanupDir(dir string) error {
	return os.RemoveAll(dir)
}

// GetTempFilePath returns the temp file path for a transfer
func GetTempFilePath(tempDir, sessionID, fileID string) string {
	return filepath.Join(tempDir, sessionID, fileID+".tmp")
}

// GetSessionTempDir returns the temp directory for a session
func GetSessionTempDir(tempDir, sessionID string) string {
	return filepath.Join(tempDir, sessionID)
}