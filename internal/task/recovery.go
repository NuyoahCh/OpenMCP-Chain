package task

import "context"

// RecoveryHandler 定义了在任务执行失败时的补偿策略。
type RecoveryHandler interface {
	// Recover 尝试根据失败原因进行补偿或降级。
	// 返回的 ExecutionResult 将作为降级结果写入任务；若返回 nil 则继续按照失败流程处理。
	Recover(ctx context.Context, task *Task, cause error) (*ExecutionResult, error)
}
