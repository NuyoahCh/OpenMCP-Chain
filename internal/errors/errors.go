package errors

import (
	stdErrors "errors"
	"fmt"
	"sync"
)

// Code 表示系统内的统一错误码。
type Code string

// Severity 描述错误的严重程度，用于告警和审计。
type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityCritical Severity = "critical"
)

// Attributes 为错误码提供默认行为。
type Attributes struct {
	Message   string
	Severity  Severity
	Retryable bool
	Alert     bool
}

var (
	registryMu sync.RWMutex
	registry   = map[Code]Attributes{
		CodeUnknown: {
			Message:   "unknown error",
			Severity:  SeverityCritical,
			Retryable: false,
			Alert:     true,
		},
		CodeInvalidArgument: {
			Message:   "invalid argument",
			Severity:  SeverityInfo,
			Retryable: false,
			Alert:     false,
		},
		CodeNotFound: {
			Message:   "resource not found",
			Severity:  SeverityInfo,
			Retryable: false,
			Alert:     false,
		},
		CodeConflict: {
			Message:   "resource conflict",
			Severity:  SeverityWarning,
			Retryable: false,
			Alert:     false,
		},
		CodeAlreadyCompleted: {
			Message:   "resource already completed",
			Severity:  SeverityInfo,
			Retryable: false,
			Alert:     false,
		},
		CodeRetriesExhausted: {
			Message:   "retries exhausted",
			Severity:  SeverityWarning,
			Retryable: false,
			Alert:     true,
		},
		CodeInitializationFailure: {
			Message:   "service not initialized",
			Severity:  SeverityWarning,
			Retryable: true,
			Alert:     true,
		},
		CodeStorageFailure: {
			Message:   "storage failure",
			Severity:  SeverityCritical,
			Retryable: true,
			Alert:     true,
		},
		CodeQueueFailure: {
			Message:   "queue failure",
			Severity:  SeverityCritical,
			Retryable: true,
			Alert:     true,
		},
		CodeExecutorFailure: {
			Message:   "executor failure",
			Severity:  SeverityWarning,
			Retryable: true,
			Alert:     true,
		},
		CodeTimeout: {
			Message:   "operation timed out",
			Severity:  SeverityWarning,
			Retryable: true,
			Alert:     true,
		},
	}
)

const (
	CodeUnknown               Code = "UNKNOWN"
	CodeInvalidArgument       Code = "INVALID_ARGUMENT"
	CodeNotFound              Code = "NOT_FOUND"
	CodeConflict              Code = "CONFLICT"
	CodeAlreadyCompleted      Code = "ALREADY_COMPLETED"
	CodeRetriesExhausted      Code = "RETRIES_EXHAUSTED"
	CodeInitializationFailure Code = "INITIALIZATION_FAILURE"
	CodeStorageFailure        Code = "STORAGE_FAILURE"
	CodeQueueFailure          Code = "QUEUE_FAILURE"
	CodeExecutorFailure       Code = "EXECUTOR_FAILURE"
	CodeTimeout               Code = "TIMEOUT"
)

// Register 允许业务模块在初始化阶段注册新的错误码描述。
func Register(code Code, attr Attributes) {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry[code] = attr
}

// AttributesOf 返回错误码对应的属性。若未注册则返回 UNKNOWN 的属性。
func AttributesOf(code Code) Attributes {
	registryMu.RLock()
	attr, ok := registry[code]
	registryMu.RUnlock()
	if ok {
		return attr
	}
	registryMu.RLock()
	fallback := registry[CodeUnknown]
	registryMu.RUnlock()
	return fallback
}

// Error 是系统内统一的错误类型。
type Error struct {
	code      Code
	message   string
	cause     error
	metadata  map[string]string
	retryable *bool
	alert     *bool
	severity  *Severity
}

// Option 定义可选配置。
type Option func(*Error)

// WithMetadata 附加额外信息。
func WithMetadata(key, value string) Option {
	return func(e *Error) {
		if e.metadata == nil {
			e.metadata = make(map[string]string)
		}
		e.metadata[key] = value
	}
}

// WithRetryable 指定错误是否可重试。
func WithRetryable(retryable bool) Option {
	return func(e *Error) {
		e.retryable = &retryable
	}
}

// WithAlert 指定错误是否需要告警。
func WithAlert(alert bool) Option {
	return func(e *Error) {
		e.alert = &alert
	}
}

// WithSeverity 覆盖默认严重程度。
func WithSeverity(sev Severity) Option {
	return func(e *Error) {
		e.severity = &sev
	}
}

// New 创建一个新的错误实例。
func New(code Code, message string, opts ...Option) *Error {
	if message == "" {
		message = AttributesOf(code).Message
	}
	e := &Error{code: code, message: message}
	for _, opt := range opts {
		if opt != nil {
			opt(e)
		}
	}
	return e
}

// Wrap 在已有错误外包裹统一错误类型。
func Wrap(code Code, cause error, message string, opts ...Option) *Error {
	e := New(code, message, opts...)
	e.cause = cause
	return e
}

// Error 实现 error 接口。
func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.code, e.message, e.cause)
	}
	return fmt.Sprintf("[%s] %s", e.code, e.message)
}

// Unwrap 实现 errors.Unwrap。
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

// Is 允许通过 errors.Is 判断是否相同错误码。
func (e *Error) Is(target error) bool {
	if e == nil || target == nil {
		return false
	}
	t, ok := target.(*Error)
	if !ok {
		return false
	}
	return e.code == t.code
}

// Code 返回错误码。
func (e *Error) Code() Code {
	if e == nil {
		return CodeUnknown
	}
	return e.code
}

// Message 返回错误信息。
func (e *Error) Message() string {
	if e == nil {
		return ""
	}
	return e.message
}

// Metadata 返回附加信息。
func (e *Error) Metadata() map[string]string {
	if e == nil || len(e.metadata) == 0 {
		return nil
	}
	clone := make(map[string]string, len(e.metadata))
	for k, v := range e.metadata {
		clone[k] = v
	}
	return clone
}

// Retryable 判断是否可重试。
func (e *Error) Retryable() bool {
	if e == nil {
		return false
	}
	if e.retryable != nil {
		return *e.retryable
	}
	attr := AttributesOf(e.code)
	return attr.Retryable
}

// ShouldAlert 判断是否需要告警。
func (e *Error) ShouldAlert() bool {
	if e == nil {
		return false
	}
	if e.alert != nil {
		return *e.alert
	}
	attr := AttributesOf(e.code)
	return attr.Alert
}

// Severity 返回错误严重程度。
func (e *Error) Severity() Severity {
	if e == nil {
		return SeverityInfo
	}
	if e.severity != nil {
		return *e.severity
	}
	attr := AttributesOf(e.code)
	return attr.Severity
}

// From 尝试从 error 中解析统一错误类型。
func From(err error) (*Error, bool) {
	if err == nil {
		return nil, false
	}
	var target *Error
	if stdErrors.As(err, &target) {
		return target, true
	}
	return nil, false
}

// CodeOf 返回错误对应的错误码。
func CodeOf(err error) Code {
	if e, ok := From(err); ok {
		return e.Code()
	}
	return CodeUnknown
}

// Retryable 判断任意 error 是否可重试。
func RetryableError(err error) bool {
	if e, ok := From(err); ok {
		return e.Retryable()
	}
	return false
}

// ShouldAlert 判断是否需要触发告警。
func ShouldAlert(err error) bool {
	if e, ok := From(err); ok {
		return e.ShouldAlert()
	}
	return false
}

// SeverityOf 返回错误严重程度。
func SeverityOf(err error) Severity {
	if e, ok := From(err); ok {
		return e.Severity()
	}
	return AttributesOf(CodeUnknown).Severity
}
