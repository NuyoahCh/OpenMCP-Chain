package task

import (
	"context"

	xerrors "OpenMCP-Chain/internal/errors"
)

// Store 抽象了任务状态的持久化接口。
type Store interface {
        Create(ctx context.Context, task *Task) error
        Get(ctx context.Context, id string) (*Task, error)
        Claim(ctx context.Context, id string) (*Task, error)
        MarkSucceeded(ctx context.Context, id string, result ExecutionResult) error
        MarkFailed(ctx context.Context, id string, code xerrors.Code, lastError string, terminal bool) error
        List(ctx context.Context, opts ListOptions) ([]*Task, error)
        Close() error
}
