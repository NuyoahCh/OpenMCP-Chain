package task

import (
	"context"
)

// Handler 处理来自消息队列的任务 ID。
type Handler func(ctx context.Context, taskID string) error

// Producer 负责向队列投递任务。
type Producer interface {
	Publish(ctx context.Context, taskID string) error
	Close() error
}

// Consumer 负责从队列中消费任务。
type Consumer interface {
	Consume(ctx context.Context, workerCount int, handler Handler) error
	Close() error
}

// Queue 同时具备生产者与消费者能力。
type Queue interface {
	Producer
	Consumer
}
