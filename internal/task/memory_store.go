package task

import (
	"context"
	"sync"
	"time"

	xerrors "OpenMCP-Chain/internal/errors"
)

// MemoryStore 以内存方式保存任务状态，主要用于测试。
type MemoryStore struct {
	mu    sync.RWMutex
	tasks map[string]*Task
}

// NewMemoryStore 创建 MemoryStore。
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{tasks: make(map[string]*Task)}
}

// Create 实现 Store 接口。
func (m *MemoryStore) Create(_ context.Context, task *Task) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if task == nil {
		return xerrors.New(xerrors.CodeInvalidArgument, "task 不能为空")
	}
	if task.ID == "" {
		return xerrors.New(xerrors.CodeInvalidArgument, "任务 ID 不能为空")
	}
	if _, ok := m.tasks[task.ID]; ok {
		return ErrTaskConflict
	}
	now := time.Now().Unix()
	if task.CreatedAt == 0 {
		task.CreatedAt = now
	}
	task.UpdatedAt = now
	clone := *task
	if task.Result != nil {
		resultCopy := *task.Result
		clone.Result = &resultCopy
	}
	clone.Metadata = cloneMetadata(task.Metadata)
	m.tasks[task.ID] = &clone
	return nil
}

// Get 返回任务。
func (m *MemoryStore) Get(_ context.Context, id string) (*Task, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	task, ok := m.tasks[id]
	if !ok {
		return nil, ErrTaskNotFound
	}
	clone := *task
	if task.Result != nil {
		resultCopy := *task.Result
		clone.Result = &resultCopy
	}
	clone.Metadata = cloneMetadata(task.Metadata)
	return &clone, nil
}

// Claim 将任务状态更新为运行中。
func (m *MemoryStore) Claim(_ context.Context, id string) (*Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	task, ok := m.tasks[id]
	if !ok {
		return nil, ErrTaskNotFound
	}
	switch task.Status {
	case StatusSucceeded:
		return cloneTask(task), ErrTaskCompleted
	case StatusRunning:
		return cloneTask(task), ErrTaskConflict
	}
	if task.Attempts >= task.MaxRetries {
		return cloneTask(task), ErrTaskExhausted
	}
	task.Status = StatusRunning
	task.Attempts++
	task.LastError = ""
	task.ErrorCode = ""
	task.UpdatedAt = time.Now().Unix()
	return cloneTask(task), nil
}

// MarkSucceeded 记录成功结果。
func (m *MemoryStore) MarkSucceeded(_ context.Context, id string, result ExecutionResult) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	task, ok := m.tasks[id]
	if !ok {
		return ErrTaskNotFound
	}
	task.Status = StatusSucceeded
	task.Result = &result
	task.LastError = ""
	task.ErrorCode = ""
	task.UpdatedAt = time.Now().Unix()
	return nil
}

// MarkFailed 标记任务失败。
func (m *MemoryStore) MarkFailed(_ context.Context, id string, code xerrors.Code, lastError string, _ bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	task, ok := m.tasks[id]
	if !ok {
		return ErrTaskNotFound
	}
	task.Status = StatusFailed
	task.LastError = lastError
	task.ErrorCode = string(code)
	task.UpdatedAt = time.Now().Unix()
	return nil
}

// List 返回最近任务。
func (m *MemoryStore) List(_ context.Context, limit int) ([]*Task, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if limit <= 0 {
		limit = len(m.tasks)
	}
	results := make([]*Task, 0, len(m.tasks))
	for _, task := range m.tasks {
		results = append(results, cloneTask(task))
	}
	if len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

// Close 对内存存储无需操作。
func (m *MemoryStore) Close() error {
	return nil
}

func cloneTask(task *Task) *Task {
	clone := *task
	if task.Result != nil {
		resultCopy := *task.Result
		clone.Result = &resultCopy
	}
	clone.Metadata = cloneMetadata(task.Metadata)
	return &clone
}

// ensure interface compliance at compile time
var _ Store = (*MemoryStore)(nil)
