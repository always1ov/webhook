package webui

import (
	"sync"
	"time"
)

const maxLogEntries = 200

// ExecutionLog 单条 hook 执行记录
type ExecutionLog struct {
	ID         string    `json:"id"`
	HookID     string    `json:"hookId"`
	Timestamp  time.Time `json:"timestamp"`
	Success    bool      `json:"success"`
	Status     string    `json:"status"`   // success | error | timeout | cancelled
	Output     string    `json:"output"`
	DurationMS int64     `json:"durationMs"`
	IP         string    `json:"ip"`
}

type logStore struct {
	mu    sync.RWMutex
	buf   [maxLogEntries]ExecutionLog
	head  int
	count int
}

var globalStore = &logStore{}

func (s *logStore) append(entry ExecutionLog) {
	// 截断输出防止内存膨胀
	if len(entry.Output) > 4096 {
		entry.Output = entry.Output[:4096] + "\n...(truncated)"
	}
	s.mu.Lock()
	s.buf[s.head] = entry
	s.head = (s.head + 1) % maxLogEntries
	if s.count < maxLogEntries {
		s.count++
	}
	s.mu.Unlock()
}

func (s *logStore) list() []ExecutionLog {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.count == 0 {
		return []ExecutionLog{}
	}
	result := make([]ExecutionLog, s.count)
	// 从最新到最旧倒序输出
	for i := 0; i < s.count; i++ {
		idx := (s.head - 1 - i + maxLogEntries) % maxLogEntries
		result[i] = s.buf[idx]
	}
	return result
}

// AppendLog 记录一条 hook 执行日志，供 server 包调用
func AppendLog(entry ExecutionLog) {
	globalStore.append(entry)
}

// ListLogs 返回所有执行日志（最新在前），供 API handler 调用
func ListLogs() []ExecutionLog {
	return globalStore.list()
}
