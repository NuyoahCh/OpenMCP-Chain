package task

import "errors"

// Status 表示任务在生命周期中的状态。
type Status string

const (
	StatusPending   Status = "pending"
	StatusRunning   Status = "running"
	StatusSucceeded Status = "succeeded"
	StatusFailed    Status = "failed"
)

// ExecutionResult 保存一次任务执行的结果。
type ExecutionResult struct {
	Thought      string `json:"thought"`
	Reply        string `json:"reply"`
	ChainID      string `json:"chain_id"`
	BlockNumber  string `json:"block_number"`
	Observations string `json:"observations"`
}

// Task 描述了排队执行的智能体任务。
type Task struct {
	ID          string           `json:"id"`
	Goal        string           `json:"goal"`
	ChainAction string           `json:"chain_action"`
	Address     string           `json:"address"`
	Status      Status           `json:"status"`
	Attempts    int              `json:"attempts"`
	MaxRetries  int              `json:"max_retries"`
	LastError   string           `json:"last_error,omitempty"`
	Result      *ExecutionResult `json:"result,omitempty"`
	CreatedAt   int64            `json:"created_at"`
	UpdatedAt   int64            `json:"updated_at"`
}

var (
	// ErrTaskNotFound 表示指定的任务不存在。
	ErrTaskNotFound = errors.New("task not found")
	// ErrTaskConflict 表示任务在当前状态下无法进行所请求的操作。
	ErrTaskConflict = errors.New("task conflict")
	// ErrTaskCompleted 表示任务已经成功完成。
	ErrTaskCompleted = errors.New("task already completed")
	// ErrTaskExhausted 表示任务的重试次数已经耗尽。
	ErrTaskExhausted = errors.New("task retries exhausted")
)
