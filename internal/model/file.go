package model

type FileStatus string

const (
	FileStatusPending      FileStatus = "pending"
	FileStatusTransferring FileStatus = "transferring"
	FileStatusSuccess      FileStatus = "success"
	FileStatusFailed       FileStatus = "failed"
)

type FileInfo struct {
	ID           string     `json:"id"`
	Name         string     `json:"name"`
	Size         int64      `json:"size"`
	MIMEType     string     `json:"mime_type"`
	Hash         string     `json:"hash"`
	Status       FileStatus `json:"status"`
	Error        string     `json:"error,omitempty"`
	TransferSize int64      `json:"transfer_size"`
	TempPath     string     `json:"-"` // Internal temp storage path
}