package task

import (
	"context"
	"errors"
	"sync"
)

// MemoryQueue 使用 channel 模拟消息队列，主要用于测试。
type MemoryQueue struct {
	ch     chan string
	mu     sync.Mutex
	closed bool
}

// NewMemoryQueue 创建一个内存队列。
func NewMemoryQueue(size int) *MemoryQueue {
	if size <= 0 {
		size = 64
	}
	return &MemoryQueue{ch: make(chan string, size)}
}

// Publish 将任务投递到队列。
func (q *MemoryQueue) Publish(ctx context.Context, taskID string) error {
	q.mu.Lock()
	closed := q.closed
	q.mu.Unlock()
	if closed {
		return errors.New("队列已关闭")
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case q.ch <- taskID:
		return nil
	}
}

// Consume 启动指定数量的工作协程消费队列中的任务。
func (q *MemoryQueue) Consume(ctx context.Context, workerCount int, handler Handler) error {
	if workerCount <= 0 {
		workerCount = 1
	}
	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case taskID, ok := <-q.ch:
					if !ok {
						return
					}
					_ = handler(ctx, taskID)
				}
			}
		}()
	}
	<-ctx.Done()
	wg.Wait()
	return ctx.Err()
}

// Close 关闭内存队列。
func (q *MemoryQueue) Close() error {
	q.mu.Lock()
	if !q.closed {
		close(q.ch)
		q.closed = true
	}
	q.mu.Unlock()
	return nil
}
