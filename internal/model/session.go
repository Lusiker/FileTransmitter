package model

import "time"

type SessionState string

const (
	SessionStatePending      SessionState = "pending"
	SessionStateAccepted     SessionState = "accepted"
	SessionStateTransferring SessionState = "transferring"
	SessionStateCompleted    SessionState = "completed"
	SessionStatePartially    SessionState = "partially_completed"
	SessionStateCancelled    SessionState = "cancelled"
	SessionStateFailed       SessionState = "failed"
)

type Session struct {
	ID          string       `json:"id"`
	SenderID    string       `json:"sender_id"`
	ReceiverID  string       `json:"receiver_id"`
	State       SessionState `json:"state"`
	Files       []FileInfo   `json:"files"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
	TotalSize   int64        `json:"total_size"`
	Transferred int64        `json:"transferred"`
	SavePath    string       `json:"save_path"` // Receiver's save location
}

func (s *Session) CalculateTotalSize() int64 {
	var total int64
	for _, f := range s.Files {
		total += f.Size
	}
	return total
}

func (s *Session) CalculateTransferred() int64 {
	var transferred int64
	for _, f := range s.Files {
		transferred += f.TransferSize
	}
	return transferred
}