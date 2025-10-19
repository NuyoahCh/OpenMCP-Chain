package task

import (
	stdErrors "errors"

	xerrors "OpenMCP-Chain/internal/errors"
)

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
	Metadata    map[string]any   `json:"metadata,omitempty"`
	Status      Status           `json:"status"`
	Attempts    int              `json:"attempts"`
	MaxRetries  int              `json:"max_retries"`
	LastError   string           `json:"last_error,omitempty"`
	ErrorCode   string           `json:"error_code,omitempty"`
	Result      *ExecutionResult `json:"result,omitempty"`
	CreatedAt   int64            `json:"created_at"`
	UpdatedAt   int64            `json:"updated_at"`
}

var (
	// ErrTaskNotFound 表示指定的任务不存在。
	ErrTaskNotFound = xerrors.New(CodeTaskNotFound, "task not found")
	// ErrTaskConflict 表示任务在当前状态下无法进行所请求的操作。
	ErrTaskConflict = xerrors.New(CodeTaskConflict, "task conflict", xerrors.WithSeverity(xerrors.SeverityWarning))
	// ErrTaskCompleted 表示任务已经成功完成。
	ErrTaskCompleted = xerrors.New(CodeTaskCompleted, "task already completed", xerrors.WithSeverity(xerrors.SeverityInfo))
	// ErrTaskExhausted 表示任务的重试次数已经耗尽。
	ErrTaskExhausted = xerrors.New(CodeTaskExhausted, "task retries exhausted", xerrors.WithSeverity(xerrors.SeverityCritical))
)

const (
	CodeTaskNotFound   xerrors.Code = "TASK_NOT_FOUND"
	CodeTaskConflict   xerrors.Code = "TASK_CONFLICT"
	CodeTaskCompleted  xerrors.Code = "TASK_COMPLETED"
	CodeTaskExhausted  xerrors.Code = "TASK_RETRIES_EXHAUSTED"
	CodeTaskValidation xerrors.Code = "TASK_VALIDATION_FAILED"
	CodeTaskPublish    xerrors.Code = "TASK_PUBLISH_FAILED"
	CodeTaskProcessing xerrors.Code = "TASK_PROCESSING_FAILED"
	CodeTaskCompensate xerrors.Code = "TASK_COMPENSATION_FAILED"
)

func init() {
	xerrors.Register(CodeTaskNotFound, xerrors.Attributes{
		Message:   "task not found",
		Severity:  xerrors.SeverityInfo,
		Retryable: false,
		Alert:     false,
	})
	xerrors.Register(CodeTaskConflict, xerrors.Attributes{
		Message:   "task conflict",
		Severity:  xerrors.SeverityWarning,
		Retryable: false,
		Alert:     false,
	})
	xerrors.Register(CodeTaskCompleted, xerrors.Attributes{
		Message:   "task already completed",
		Severity:  xerrors.SeverityInfo,
		Retryable: false,
		Alert:     false,
	})
	xerrors.Register(CodeTaskExhausted, xerrors.Attributes{
		Message:   "task retries exhausted",
		Severity:  xerrors.SeverityCritical,
		Retryable: false,
		Alert:     true,
	})
	xerrors.Register(CodeTaskValidation, xerrors.Attributes{
		Message:   "task validation failed",
		Severity:  xerrors.SeverityInfo,
		Retryable: false,
		Alert:     false,
	})
	xerrors.Register(CodeTaskPublish, xerrors.Attributes{
		Message:   "failed to publish task",
		Severity:  xerrors.SeverityCritical,
		Retryable: true,
		Alert:     true,
	})
	xerrors.Register(CodeTaskProcessing, xerrors.Attributes{
		Message:   "task execution failed",
		Severity:  xerrors.SeverityWarning,
		Retryable: true,
		Alert:     true,
	})
	xerrors.Register(CodeTaskCompensate, xerrors.Attributes{
		Message:   "task compensation failed",
		Severity:  xerrors.SeverityCritical,
		Retryable: false,
		Alert:     true,
	})
}

// IsTaskError 判断错误是否为统一任务错误。
func IsTaskError(err error, target xerrors.Code) bool {
	if err == nil {
		return false
	}
	if stdErrors.Is(err, ErrTaskNotFound) {
		return target == CodeTaskNotFound
	}
	if stdErrors.Is(err, ErrTaskConflict) {
		return target == CodeTaskConflict
	}
	if stdErrors.Is(err, ErrTaskCompleted) {
		return target == CodeTaskCompleted
	}
	if stdErrors.Is(err, ErrTaskExhausted) {
		return target == CodeTaskExhausted
	}
	return false
}

func cloneMetadata(metadata map[string]any) map[string]any {
	if metadata == nil {
		return nil
	}
	cloned := make(map[string]any, len(metadata))
	for key, value := range metadata {
		cloned[key] = value
	}
	return cloned
}

// IsValidStatus 检查给定的任务状态是否为支持的枚举值。
func IsValidStatus(status Status) bool {
	switch status {
	case StatusPending, StatusRunning, StatusSucceeded, StatusFailed:
		return true
	default:
		return false
	}
}
