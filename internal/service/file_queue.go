package service

import (
	"sync"
)

// FileQueue 文件传输队列（滑动窗口）
// 控制同时传输的文件数量，避免内存爆炸
type FileQueue struct {
	SessionID      string
	PendingFiles   []string // 待传输的 fileID
	ActiveFiles    []string // 正在传输的 fileID（最多 MaxConcurrent 个）
	CompletedFiles []string // 已完成
	FailedFiles    []string // 失败

	MaxConcurrent int // 最大并发数（建议 2-3）

	mu sync.Mutex
}

// NewFileQueue 创建新的文件队列
func NewFileQueue(sessionID string, maxConcurrent int) *FileQueue {
	if maxConcurrent <= 0 {
		maxConcurrent = 2 // 默认最多 2 个文件并发
	}
	return &FileQueue{
		SessionID:     sessionID,
		PendingFiles:  []string{},
		ActiveFiles:   []string{},
		CompletedFiles: []string{},
		FailedFiles:   []string{},
		MaxConcurrent: maxConcurrent,
	}
}

// AddFile 添加文件到待传输队列
func (q *FileQueue) AddFile(fileID string) {
	q.mu.Lock()
	defer q.mu.Unlock()

	// 避免重复添加
	for _, id := range q.PendingFiles {
		if id == fileID {
			return
		}
	}
	for _, id := range q.ActiveFiles {
		if id == fileID {
			return
		}
	}

	q.PendingFiles = append(q.PendingFiles, fileID)
}

// GetNextFile 获取下一个可传输的文件
// 如果当前活跃文件数已达上限，返回空字符串
func (q *FileQueue) GetNextFile() string {
	q.mu.Lock()
	defer q.mu.Unlock()

	// 如果活跃文件数已达上限，不返回新文件
	if len(q.ActiveFiles) >= q.MaxConcurrent {
		return ""
	}

	// 如果没有待传输文件，返回空
	if len(q.PendingFiles) == 0 {
		return ""
	}

	// 取出第一个待传输文件
	fileID := q.PendingFiles[0]
	q.PendingFiles = q.PendingFiles[1:]
	q.ActiveFiles = append(q.ActiveFiles, fileID)

	return fileID
}

// StartFile 标记文件开始传输
func (q *FileQueue) StartFile(fileID string) {
	q.mu.Lock()
	defer q.mu.Unlock()

	// 检查是否在待传输列表中
	for i, id := range q.PendingFiles {
		if id == fileID {
			// 从待传输列表移除
			q.PendingFiles = append(q.PendingFiles[:i], q.PendingFiles[i+1:]...)
			// 加入活跃列表
			q.ActiveFiles = append(q.ActiveFiles, fileID)
			return
		}
	}
}

// MarkComplete 标记文件完成，释放窗口位置
func (q *FileQueue) MarkComplete(fileID string) {
	q.mu.Lock()
	defer q.mu.Unlock()

	// 从活跃列表移除
	for i, id := range q.ActiveFiles {
		if id == fileID {
			q.ActiveFiles = append(q.ActiveFiles[:i], q.ActiveFiles[i+1:]...)
			break
		}
	}

	// 加入已完成列表
	q.CompletedFiles = append(q.CompletedFiles, fileID)
}

// MarkFailed 标记文件失败
func (q *FileQueue) MarkFailed(fileID string) {
	q.mu.Lock()
	defer q.mu.Unlock()

	// 从活跃列表移除
	for i, id := range q.ActiveFiles {
		if id == fileID {
			q.ActiveFiles = append(q.ActiveFiles[:i], q.ActiveFiles[i+1:]...)
			break
		}
	}

	// 加入失败列表
	q.FailedFiles = append(q.FailedFiles, fileID)
}

// IsActive 检查文件是否正在传输
func (q *FileQueue) IsActive(fileID string) bool {
	q.mu.Lock()
	defer q.mu.Unlock()

	for _, id := range q.ActiveFiles {
		if id == fileID {
			return true
		}
	}
	return false
}

// IsPending 检查文件是否在待传输队列
func (q *FileQueue) IsPending(fileID string) bool {
	q.mu.Lock()
	defer q.mu.Unlock()

	for _, id := range q.PendingFiles {
		if id == fileID {
			return true
		}
	}
	return false
}

// GetStats 获取队列状态
func (q *FileQueue) GetStats() FileQueueStats {
	q.mu.Lock()
	defer q.mu.Unlock()

	return FileQueueStats{
		Pending:   len(q.PendingFiles),
		Active:    len(q.ActiveFiles),
		Completed: len(q.CompletedFiles),
		Failed:    len(q.FailedFiles),
	}
}

// FileQueueStats 队列状态统计
type FileQueueStats struct {
	Pending   int
	Active    int
	Completed int
	Failed    int
}

// HasMore 检查是否还有待传输文件
func (q *FileQueue) HasMore() bool {
	q.mu.Lock()
	defer q.mu.Unlock()

	return len(q.PendingFiles) > 0 || len(q.ActiveFiles) > 0
}

// AllComplete 检查所有文件是否完成
func (q *FileQueue) AllComplete() bool {
	q.mu.Lock()
	defer q.mu.Unlock()

	return len(q.PendingFiles) == 0 && len(q.ActiveFiles) == 0
}

// ResetFailed 重置失败文件，允许重新传输
func (q *FileQueue) ResetFailed() {
	q.mu.Lock()
	defer q.mu.Unlock()

	// 将失败文件移回待传输队列
	q.PendingFiles = append(q.PendingFiles, q.FailedFiles...)
	q.FailedFiles = []string{}
}