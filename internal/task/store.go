package task

import "context"

// Store 抽象了任务状态的持久化接口。
type Store interface {
	Create(ctx context.Context, task *Task) error
	Get(ctx context.Context, id string) (*Task, error)
	Claim(ctx context.Context, id string) (*Task, error)
	MarkSucceeded(ctx context.Context, id string, result ExecutionResult) error
	MarkFailed(ctx context.Context, id string, lastError string, terminal bool) error
	List(ctx context.Context, limit int) ([]*Task, error)
	Close() error
}
