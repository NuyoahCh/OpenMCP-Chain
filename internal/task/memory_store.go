package task

import (
	"context"
	"sort"
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
func (m *MemoryStore) List(_ context.Context, opts ListOptions) ([]*Task, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	opts.applyDefaults()

	results := make([]*Task, 0, len(m.tasks))
	for _, task := range m.tasks {
		if !matchesListFilters(task, opts) {
	matchesStatus := func(task *Task) bool {
		if len(opts.Statuses) == 0 {
			return true
		}
		for _, status := range opts.Statuses {
			if task.Status == status {
				return true
			}
		}
		return false
	}

	hasResult := func(task *Task) bool {
		if task.Result == nil {
			return false
		}
		if task.Result.Thought != "" || task.Result.Reply != "" || task.Result.ChainID != "" || task.Result.BlockNumber != "" || task.Result.Observations != "" {
			return true
		}
		return false
	}

	results := make([]*Task, 0, len(m.tasks))
	for _, task := range m.tasks {
		if !matchesStatus(task) {
			continue
		}
		if opts.UpdatedGTE > 0 && task.UpdatedAt < opts.UpdatedGTE {
			continue
		}
		if opts.UpdatedLTE > 0 && task.UpdatedAt > opts.UpdatedLTE {
			continue
		}
		if opts.HasResult != nil && hasResult(task) != *opts.HasResult {
			continue
		}
		results = append(results, cloneTask(task))
	}

	sort.Slice(results, func(i, j int) bool {
		if opts.Order == SortByUpdatedAsc {
			if results[i].UpdatedAt == results[j].UpdatedAt {
				if results[i].CreatedAt == results[j].CreatedAt {
					return results[i].ID < results[j].ID
				}
				return results[i].CreatedAt < results[j].CreatedAt
			}
			return results[i].UpdatedAt < results[j].UpdatedAt
		}
		if results[i].UpdatedAt == results[j].UpdatedAt {
			if results[i].CreatedAt == results[j].CreatedAt {
				return results[i].ID < results[j].ID
			}
			return results[i].CreatedAt > results[j].CreatedAt
		}
		return results[i].UpdatedAt > results[j].UpdatedAt
	})

	if len(results) > opts.Limit {
		results = results[:opts.Limit]
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

func matchesListFilters(task *Task, opts ListOptions) bool {
	if len(opts.Statuses) > 0 {
		matched := false
		for _, status := range opts.Statuses {
			if task.Status == status {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	if opts.UpdatedGTE > 0 && task.UpdatedAt < opts.UpdatedGTE {
		return false
	}
	if opts.UpdatedLTE > 0 && task.UpdatedAt > opts.UpdatedLTE {
		return false
	}
	if opts.HasResult != nil && taskHasResult(task) != *opts.HasResult {
		return false
	}
	return true
}

func taskHasResult(task *Task) bool {
	if task == nil || task.Result == nil {
		return false
	}
	result := task.Result
	return result.Thought != "" || result.Reply != "" || result.ChainID != "" || result.BlockNumber != "" || result.Observations != ""
}

// Stats 统计符合过滤条件的任务数量与更新时间范围。
func (m *MemoryStore) Stats(_ context.Context, opts ListOptions) (TaskStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	opts.applyDefaults()

	stats := TaskStats{}
	for _, task := range m.tasks {
		if !matchesListFilters(task, opts) {
			continue
		}
		stats.Total++
		switch task.Status {
		case StatusPending:
			stats.Pending++
		case StatusRunning:
			stats.Running++
		case StatusSucceeded:
			stats.Succeeded++
		case StatusFailed:
			stats.Failed++
		}
		if task.UpdatedAt > stats.NewestUpdatedAt {
			stats.NewestUpdatedAt = task.UpdatedAt
		}
		if stats.OldestUpdatedAt == 0 || (task.UpdatedAt != 0 && task.UpdatedAt < stats.OldestUpdatedAt) {
			stats.OldestUpdatedAt = task.UpdatedAt
		}
	}
	if stats.Total == 0 {
		stats.OldestUpdatedAt = 0
		stats.NewestUpdatedAt = 0
	}
	return stats, nil
}

// ensure interface compliance at compile time
var _ Store = (*MemoryStore)(nil)
